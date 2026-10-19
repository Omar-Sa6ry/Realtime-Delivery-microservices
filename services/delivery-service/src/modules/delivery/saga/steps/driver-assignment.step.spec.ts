import { Test, TestingModule } from '@nestjs/testing';
import { of, throwError } from 'rxjs';
import { lastValueFrom } from 'rxjs';
import { DriverAssignmentStep } from './driver-assignment.step';
import { DeliveryCommandService } from '../../../services/delivery-command.service';
import { DeliverySagaContext } from '../saga-step';
import { createMockDeliveryCommandService, createMockGrpcClient, mockDelivery } from '../../../../../test-utils';

describe('DriverAssignmentStep', () => {
  let step: DriverAssignmentStep;
  let commands: ReturnType<typeof createMockDeliveryCommandService>;
  let driverServiceClient: any;

  beforeEach(async () => {
    commands = createMockDeliveryCommandService();
    driverServiceClient = {
      ReserveDriver: jest.fn(),
      ReleaseDriver: jest.fn(),
    };

    const driverClientGrpc = { getService: jest.fn().mockReturnValue(driverServiceClient) };

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DriverAssignmentStep,
        { provide: DeliveryCommandService, useValue: commands },
        { provide: 'DRIVER_SERVICE', useValue: driverClientGrpc },
      ],
    }).compile();

    step = module.get<DriverAssignmentStep>(DriverAssignmentStep);
    await step.onModuleInit();
  });

  it('should be defined', () => {
    expect(step).toBeDefined();
    expect(step.name).toBe('DRIVER_ASSIGNMENT');
  });

  describe('execute', () => {
    it('should return context unchanged when driver service not available', async () => {
      const stepWithoutDriver = await createStepWithoutDriverClient();
      const context: DeliverySagaContext = { delivery: mockDelivery() };

      const result = await stepWithoutDriver.execute(context);

      expect(result).toEqual(context);
    });

    async function createStepWithoutDriverClient() {
      const commands = createMockDeliveryCommandService();
      const driverClientGrpc = { getService: jest.fn().mockImplementation(() => { throw new Error('No service'); }) };
      
      const module = await Test.createTestingModule({
        providers: [
          DriverAssignmentStep,
          { provide: DeliveryCommandService, useValue: commands },
          { provide: 'DRIVER_SERVICE', useValue: driverClientGrpc },
        ],
      }).compile();

      const step = module.get<DriverAssignmentStep>(DriverAssignmentStep);
      await step.onModuleInit();
      return step;
    }

    it('should reserve driver via gRPC when driverId is set', async () => {
      const delivery = mockDelivery({ driverId: 'driver-123' });
      const context: DeliverySagaContext = { delivery };
      driverServiceClient.ReserveDriver.mockReturnValue(of({ reserved: true }));

      const result = await step.execute(context);

      expect(driverServiceClient.ReserveDriver).toHaveBeenCalledWith({
        driverId: 'driver-123',
        deliveryId: delivery.id,
        idempotencyKey: `reserve-${delivery.id}-driver-123`,
        correlationId: delivery.id,
      });
      expect(result).toEqual(context);
    });

    it('should throw error if ReserveDriver fails', async () => {
      const delivery = mockDelivery({ driverId: 'driver-123' });
      const context: DeliverySagaContext = { delivery };
      driverServiceClient.ReserveDriver.mockReturnValue(throwError(() => new Error('Driver not available')));

      await expect(step.execute(context)).rejects.toThrow('Driver not available');
    });

    it('should not call ReserveDriver when driverId is not set', async () => {
      const delivery = mockDelivery({ driverId: null });
      const context: DeliverySagaContext = { delivery };

      const result = await step.execute(context);

      expect(driverServiceClient.ReserveDriver).not.toHaveBeenCalled();
      expect(result).toEqual(context);
    });
  });

  describe('compensate', () => {
    it('should release driver reservation when paymentId exists', async () => {
      const context: DeliverySagaContext = {
        delivery: mockDelivery({ id: 'delivery-123', driverId: 'driver-123' }),
      };
      driverServiceClient.ReleaseDriver.mockReturnValue(of({}));

      await step.compensate(context);

      expect(driverServiceClient.ReleaseDriver).toHaveBeenCalledWith({
        driverId: 'driver-123',
        deliveryId: 'delivery-123',
        reason: 'Saga compensation',
        correlationId: 'delivery-123',
      });
    });

    it('should not call ReleaseDriver when driverId not set', async () => {
      const context: DeliverySagaContext = {
        delivery: mockDelivery({ id: 'delivery-123', driverId: null }),
      };

      await step.compensate(context);

      expect(driverServiceClient.ReleaseDriver).not.toHaveBeenCalled();
    });

    it('should not call ReleaseDriver when driver service not available', async () => {
      const stepWithoutDriver = await createStepWithoutDriverClient();
      const context: DeliverySagaContext = {
        delivery: mockDelivery({ id: 'delivery-123', driverId: 'driver-123' }),
      };

      await expect(stepWithoutDriver.compensate(context)).resolves.not.toThrow();
    });

    it('should not throw if ReleaseDriver fails', async () => {
      const context: DeliverySagaContext = {
        delivery: mockDelivery({ id: 'delivery-123', driverId: 'driver-123' }),
      };
      driverServiceClient.ReleaseDriver.mockReturnValue(throwError(() => new Error('Failed to release')));

      await expect(step.compensate(context)).resolves.not.toThrow();
    });

    async function createStepWithoutDriverClient() {
      const commands = createMockDeliveryCommandService();
      const driverClientGrpc = { getService: jest.fn().mockImplementation(() => { throw new Error('No service'); }) };
      
      const module = await Test.createTestingModule({
        providers: [
          DriverAssignmentStep,
          { provide: DeliveryCommandService, useValue: commands },
          { provide: 'DRIVER_SERVICE', useValue: driverClientGrpc },
        ],
      }).compile();

      const step = module.get<DriverAssignmentStep>(DriverAssignmentStep);
      await step.onModuleInit();
      return step;
    }
  });
});