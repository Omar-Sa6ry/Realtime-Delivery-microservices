import { Test, TestingModule } from '@nestjs/testing';
import { DriverDispatchSchedulerService } from './driver-dispatch-scheduler.service';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { DeliveryCommandService } from './delivery-command.service';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { createMockDeliveryRepository, createMockDeliveryCommandService, mockDelivery } from '../../../test-utils';

describe('DriverDispatchSchedulerService', () => {
  let service: DriverDispatchSchedulerService;
  let repository: ReturnType<typeof createMockDeliveryRepository>;
  let commands: ReturnType<typeof createMockDeliveryCommandService>;

  beforeEach(async () => {
    repository = createMockDeliveryRepository();
    commands = createMockDeliveryCommandService();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DriverDispatchSchedulerService,
        { provide: DeliveryRepository, useValue: repository },
        { provide: DeliveryCommandService, useValue: commands },
      ],
    }).compile();

    service = module.get<DriverDispatchSchedulerService>(DriverDispatchSchedulerService);
  });

  it('should be defined', () => {
    expect(service).toBeDefined();
  });

  describe('onModuleInit', () => {
    it('should set up interval', () => {
      const setIntervalSpy = jest.spyOn(global, 'setInterval');
      
      service.onModuleInit();

      expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), expect.any(Number));
    });

    it('should use default interval when env not set', () => {
      const setIntervalSpy = jest.spyOn(global, 'setInterval');
      
      service.onModuleInit();

      expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), 30000);
    });

    it('should use environment variable for interval', () => {
      process.env.DRIVER_DISPATCH_RETRY_INTERVAL_MS = '60000';
      const setIntervalSpy = jest.spyOn(global, 'setInterval');
      
      const newService = new DriverDispatchSchedulerService(repository, commands);
      newService.onModuleInit();

      expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), 60000);
      delete process.env.DRIVER_DISPATCH_RETRY_INTERVAL_MS;
    });
  });

  describe('onModuleDestroy', () => {
    it('should clear interval', () => {
      const clearIntervalSpy = jest.spyOn(global, 'clearInterval');
      
      service.onModuleInit();
      service.onModuleDestroy();

      expect(clearIntervalSpy).toHaveBeenCalled();
    });
  });

  describe('processUnassignedDeliveries', () => {
    it('should not process if already processing', async () => {
      (service as any).isProcessing = true;

      await service.processUnassignedDeliveries();

      expect(repository.findUnassignedPendingDeliveries).not.toHaveBeenCalled();
    });

    it('should return early if no pending deliveries', async () => {
      repository.findUnassignedPendingDeliveries.mockResolvedValue([]);

      await service.processUnassignedDeliveries();

      expect(repository.findUnassignedPendingDeliveries).toHaveBeenCalled();
      expect(commands.retryDriverDispatch).not.toHaveBeenCalled();
    });

    it('should process each pending delivery', async () => {
      const deliveries = [
        mockDelivery({ id: '1', status: DeliveryStatus.PAYMENT_CONFIRMED, createdAt: new Date(Date.now() - 60000) }),
        mockDelivery({ id: '2', status: DeliveryStatus.PAYMENT_CONFIRMED, createdAt: new Date(Date.now() - 120000) }),
      ];
      repository.findUnassignedPendingDeliveries.mockResolvedValue(deliveries);
      commands.retryDriverDispatch.mockResolvedValue(undefined);

      await service.processUnassignedDeliveries();

      expect(commands.retryDriverDispatch).toHaveBeenCalledTimes(2);
      expect(commands.retryDriverDispatch).toHaveBeenCalledWith(deliveries[0]);
      expect(commands.retryDriverDispatch).toHaveBeenCalledWith(deliveries[1]);
    });

    it('should skip deliveries created less than 30 seconds ago', async () => {
      const recentDelivery = mockDelivery({ id: '1', status: DeliveryStatus.PAYMENT_CONFIRMED, createdAt: new Date(Date.now() - 10000) });
      const oldDelivery = mockDelivery({ id: '2', status: DeliveryStatus.PAYMENT_CONFIRMED, createdAt: new Date(Date.now() - 60000) });
      repository.findUnassignedPendingDeliveries.mockResolvedValue([recentDelivery, oldDelivery]);
      commands.retryDriverDispatch.mockResolvedValue(undefined);

      await service.processUnassignedDeliveries();

      expect(commands.retryDriverDispatch).toHaveBeenCalledTimes(1);
      expect(commands.retryDriverDispatch).toHaveBeenCalledWith(oldDelivery);
    });

    it('should continue processing other deliveries if one fails', async () => {
      const deliveries = [
        mockDelivery({ id: '1', status: DeliveryStatus.PAYMENT_CONFIRMED, createdAt: new Date(Date.now() - 60000) }),
        mockDelivery({ id: '2', status: DeliveryStatus.PAYMENT_CONFIRMED, createdAt: new Date(Date.now() - 120000) }),
      ];
      repository.findUnassignedPendingDeliveries.mockResolvedValue(deliveries);
      commands.retryDriverDispatch
        .mockRejectedValueOnce(new Error('Failed for delivery 1'))
        .mockResolvedValueOnce(undefined);

      await service.processUnassignedDeliveries();

      expect(commands.retryDriverDispatch).toHaveBeenCalledTimes(2);
    });

    it('should log error when retry fails', async () => {
      const delivery = mockDelivery({ id: '1', status: DeliveryStatus.PAYMENT_CONFIRMED, createdAt: new Date(Date.now() - 60000) });
      repository.findUnassignedPendingDeliveries.mockResolvedValue([delivery]);
      commands.retryDriverDispatch.mockRejectedValue(new Error('Dispatch failed'));

      await service.processUnassignedDeliveries();

      expect(commands.retryDriverDispatch).toHaveBeenCalledWith(delivery);
    });

    it('should set isProcessing to false even if error occurs', async () => {
      repository.findUnassignedPendingDeliveries.mockRejectedValue(new Error('DB error'));

      await service.processUnassignedDeliveries();

      expect((service as any).isProcessing).toBe(false);
    });
  });
});