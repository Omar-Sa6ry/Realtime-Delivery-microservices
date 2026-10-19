import { Test, TestingModule } from '@nestjs/testing';
import { DeliverySagaOrchestrator } from './delivery-saga.orchestrator';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { PaymentConfirmationStep } from './steps/payment-confirmation.step';
import { DriverAssignmentStep } from './steps/driver-assignment.step';
import { AlertService } from '@delivery-micro/shard';
import { Delivery } from '../../entities/delivery.entity';
import { DeliveryStatus } from '../../enums/delivery-status.enum';
import { createMockDeliveryRepository, createMockPaymentConfirmationStep, createMockDriverAssignmentStep, createMockAlertService, mockDelivery } from '../../../../test-utils';

describe('DeliverySagaOrchestrator', () => {
  let orchestrator: DeliverySagaOrchestrator;
  let repository: ReturnType<typeof createMockDeliveryRepository>;
  let paymentStep: ReturnType<typeof createMockPaymentConfirmationStep>;
  let driverStep: ReturnType<typeof createMockDriverAssignmentStep>;
  let alerts: ReturnType<typeof createMockAlertService>;

  beforeEach(async () => {
    repository = createMockDeliveryRepository();
    paymentStep = createMockPaymentConfirmationStep();
    driverStep = createMockDriverAssignmentStep();
    alerts = createMockAlertService();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DeliverySagaOrchestrator,
        { provide: DeliveryRepository, useValue: repository },
        { provide: PaymentConfirmationStep, useValue: paymentStep },
        { provide: DriverAssignmentStep, useValue: driverStep },
        { provide: AlertService, useValue: alerts },
      ],
    }).compile();

    orchestrator = module.get<DeliverySagaOrchestrator>(DeliverySagaOrchestrator);
  });

  it('should be defined', () => {
    expect(orchestrator).toBeDefined();
  });

  describe('execute', () => {
    it('should execute all steps successfully', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.CREATED });
      const updatedDelivery = mockDelivery({ status: DeliveryStatus.PAYMENT_CONFIRMED });
      repository.findById.mockResolvedValue(delivery);
      paymentStep.execute.mockResolvedValue({ delivery: updatedDelivery, paymentId: 'pay-123' });
      driverStep.execute.mockResolvedValue({ delivery: updatedDelivery, paymentId: 'pay-123' });

      const result = await orchestrator.execute('test-delivery-id');

      expect(repository.findById).toHaveBeenCalledWith('test-delivery-id');
      expect(paymentStep.execute).toHaveBeenCalledWith(expect.objectContaining({ delivery }));
      expect(driverStep.execute).toHaveBeenCalledWith(expect.objectContaining({ delivery: updatedDelivery }));
      expect(result).toEqual(updatedDelivery);
    });

    it('should skip driver assignment when delivery is PENDING_PAYMENT', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.PENDING_PAYMENT });
      const updatedDelivery = mockDelivery({ status: DeliveryStatus.PENDING_PAYMENT });
      repository.findById.mockResolvedValue(delivery);
      paymentStep.execute.mockResolvedValue({ delivery: updatedDelivery, paymentId: 'pay-123' });

      const result = await orchestrator.execute('test-delivery-id');

      expect(paymentStep.execute).toHaveBeenCalled();
      expect(driverStep.execute).not.toHaveBeenCalled();
      expect(result).toEqual(updatedDelivery);
    });

    it('should throw error when delivery not found', async () => {
      repository.findById.mockResolvedValue(null);

      await expect(orchestrator.execute('non-existent')).rejects.toThrow('Delivery with ID non-existent not found for Saga execution');
    });

    it('should execute compensations in reverse order on step failure', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.CREATED });
      const updatedDelivery = mockDelivery({ status: DeliveryStatus.PAYMENT_CONFIRMED });
      repository.findById.mockResolvedValue(delivery);
      paymentStep.execute.mockResolvedValue({ delivery: updatedDelivery, paymentId: 'pay-123' });
      driverStep.execute.mockRejectedValue(new Error('Driver assignment failed'));
      paymentStep.compensate.mockResolvedValue(undefined);
      alerts.triggerAlert.mockResolvedValue(undefined);

      await expect(orchestrator.execute('test-delivery-id')).rejects.toThrow('Driver assignment failed');

      expect(driverStep.execute).toHaveBeenCalled();
      expect(paymentStep.compensate).toHaveBeenCalledWith(expect.objectContaining({ delivery: updatedDelivery }));
      expect(alerts.triggerAlert).toHaveBeenCalledWith(
        'Delivery saga failed',
        expect.stringContaining('Driver assignment failed'),
        'CRITICAL'
      );
    });

    it('should execute compensations for all completed steps on failure', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.CREATED });
      const afterPayment = mockDelivery({ status: DeliveryStatus.PAYMENT_CONFIRMED });
      repository.findById.mockResolvedValue(delivery);
      paymentStep.execute.mockResolvedValue({ delivery: afterPayment, paymentId: 'pay-123' });
      driverStep.execute.mockRejectedValue(new Error('Driver assignment failed'));
      paymentStep.compensate.mockResolvedValue(undefined);
      alerts.triggerAlert.mockResolvedValue(undefined);

      await expect(orchestrator.execute('test-delivery-id')).rejects.toThrow();

      expect(paymentStep.compensate).toHaveBeenCalledTimes(1);
      expect(alerts.triggerAlert).toHaveBeenCalledTimes(2); // Once for saga failure, once for compensation failure
    });

    it('should continue compensation even if one compensation fails', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.CREATED });
      const afterPayment = mockDelivery({ status: DeliveryStatus.PAYMENT_CONFIRMED });
      repository.findById.mockResolvedValue(delivery);
      paymentStep.execute.mockResolvedValue({ delivery: afterPayment, paymentId: 'pay-123' });
      driverStep.execute.mockRejectedValue(new Error('Driver assignment failed'));
      paymentStep.compensate.mockRejectedValue(new Error('Compensation failed'));
      alerts.triggerAlert.mockResolvedValue(undefined);

      await expect(orchestrator.execute('test-delivery-id')).rejects.toThrow('Driver assignment failed');

      expect(paymentStep.compensate).toHaveBeenCalled();
      expect(alerts.triggerAlert).toHaveBeenCalledWith(
        'Saga compensation failed',
        expect.stringContaining('could not be compensated'),
        'CRITICAL'
      );
    });

    it('should handle error in first step', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.CREATED });
      repository.findById.mockResolvedValue(delivery);
      paymentStep.execute.mockRejectedValue(new Error('Payment failed'));
      paymentStep.compensate.mockResolvedValue(undefined);
      alerts.triggerAlert.mockResolvedValue(undefined);

      await expect(orchestrator.execute('test-delivery-id')).rejects.toThrow('Payment failed');

      expect(paymentStep.compensate).toHaveBeenCalledWith(expect.objectContaining({ delivery }));
      expect(driverStep.execute).not.toHaveBeenCalled();
    });
  });
});