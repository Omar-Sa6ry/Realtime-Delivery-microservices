import { Test, TestingModule } from '@nestjs/testing';
import { DeliveryQueryService } from './delivery-query.service';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { DeliveryStateMachine } from './delivery.state-machine';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { mockDelivery, createMockDeliveryRepository, createMockDeliveryStateMachine } from '../../../test-utils';

describe('DeliveryQueryService', () => {
  let service: DeliveryQueryService;
  let repository: ReturnType<typeof createMockDeliveryRepository>;
  let stateMachine: ReturnType<typeof createMockDeliveryStateMachine>;

  beforeEach(async () => {
    repository = createMockDeliveryRepository();
    stateMachine = createMockDeliveryStateMachine();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DeliveryQueryService,
        { provide: DeliveryRepository, useValue: repository },
        { provide: DeliveryStateMachine, useValue: stateMachine },
      ],
    }).compile();

    service = module.get<DeliveryQueryService>(DeliveryQueryService);
  });

  it('should be defined', () => {
    expect(service).toBeDefined();
  });

  describe('getById', () => {
    it('should delegate to repository findById', async () => {
      const delivery = mockDelivery();
      repository.findById.mockResolvedValue(delivery);

      const result = await service.getById('test-id');

      expect(repository.findById).toHaveBeenCalledWith('test-id');
      expect(result).toEqual(delivery);
    });
  });

  describe('listByCustomer', () => {
    it('should delegate to repository findByCustomerId with correct pagination', async () => {
      const deliveries = [mockDelivery({ id: '1' }), mockDelivery({ id: '2' })];
      repository.findByCustomerId.mockResolvedValue([deliveries, 2]);

      const result = await service.listByCustomer('customer-123', 2, 10);

      expect(repository.findByCustomerId).toHaveBeenCalledWith('customer-123', 10, 10);
      expect(result).toEqual([deliveries, 2]);
    });

    it('should enforce minimum page of 1', async () => {
      repository.findByCustomerId.mockResolvedValue([[], 0]);

      await service.listByCustomer('customer-123', 0, 50);

      expect(repository.findByCustomerId).toHaveBeenCalledWith('customer-123', 0, 50);
    });

    it('should enforce minimum pageSize of 1', async () => {
      repository.findByCustomerId.mockResolvedValue([[], 0]);

      await service.listByCustomer('customer-123', 1, 0);

      expect(repository.findByCustomerId).toHaveBeenCalledWith('customer-123', 0, 1);
    });

    it('should enforce maximum pageSize of 100', async () => {
      repository.findByCustomerId.mockResolvedValue([[], 0]);

      await service.listByCustomer('customer-123', 1, 200);

      expect(repository.findByCustomerId).toHaveBeenCalledWith('customer-123', 0, 100);
    });

    it('should use default values when not provided', async () => {
      repository.findByCustomerId.mockResolvedValue([[], 0]);

      await service.listByCustomer('customer-123');

      expect(repository.findByCustomerId).toHaveBeenCalledWith('customer-123', 0, 50);
    });
  });

  describe('getOpenDeliveries', () => {
    it('should delegate to repository findOpenDeliveries with correct pagination', async () => {
      const deliveries = [mockDelivery({ id: '1', status: DeliveryStatus.PAYMENT_CONFIRMED })];
      repository.findOpenDeliveries.mockResolvedValue([deliveries, 1]);

      const result = await service.getOpenDeliveries(2, 25);

      expect(repository.findOpenDeliveries).toHaveBeenCalledWith(25, 25);
      expect(result).toEqual([deliveries, 1]);
    });

    it('should enforce pagination limits', async () => {
      repository.findOpenDeliveries.mockResolvedValue([[], 0]);

      await service.getOpenDeliveries(0, 0);

      expect(repository.findOpenDeliveries).toHaveBeenCalledWith(0, 1);
    });

    it('should use default values when not provided', async () => {
      repository.findOpenDeliveries.mockResolvedValue([[], 0]);

      await service.getOpenDeliveries();

      expect(repository.findOpenDeliveries).toHaveBeenCalledWith(0, 50);
    });
  });

  describe('nextStatuses', () => {
    it('should delegate to stateMachine nextStates', () => {
      const delivery = mockDelivery({ status: DeliveryStatus.CREATED });
      const nextStates = [DeliveryStatus.PENDING_PAYMENT, DeliveryStatus.PAYMENT_CONFIRMED];
      stateMachine.nextStates.mockReturnValue(nextStates);

      const result = service.nextStatuses(delivery);

      expect(stateMachine.nextStates).toHaveBeenCalledWith(DeliveryStatus.CREATED);
      expect(result).toEqual(nextStates);
    });

    it('should return empty array for terminal states', () => {
      const delivery = mockDelivery({ status: DeliveryStatus.COMPLETED });
      stateMachine.nextStates.mockReturnValue([]);

      const result = service.nextStatuses(delivery);

      expect(stateMachine.nextStates).toHaveBeenCalledWith(DeliveryStatus.COMPLETED);
      expect(result).toEqual([]);
    });
  });
});