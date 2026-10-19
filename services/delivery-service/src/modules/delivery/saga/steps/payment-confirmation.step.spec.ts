import { Test, TestingModule } from '@nestjs/testing';
import { of, throwError } from 'rxjs';
import { lastValueFrom } from 'rxjs';
import { DeliveryStatus } from '../../enums/delivery-status.enum';
import { PaymentStatus } from '../../enums/payment-status.enum';
import { PaymentConfirmationStep } from './payment-confirmation.step';
import { DeliveryCommandService } from '../../../services/delivery-command.service';
import { DeliverySagaContext } from '../saga-step';
import { createMockDeliveryCommandService, createMockGrpcClient, mockDelivery } from '../../../../../test-utils';

describe('PaymentConfirmationStep', () => {
  let step: PaymentConfirmationStep;
  let commands: ReturnType<typeof createMockDeliveryCommandService>;
  let paymentServiceClient: any;

  beforeEach(async () => {
    commands = createMockDeliveryCommandService();
    paymentServiceClient = {
      CreatePayment: jest.fn(),
      CancelAuthorization: jest.fn(),
    };

    const paymentClientGrpc = { getService: jest.fn().mockReturnValue(paymentServiceClient) };

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        PaymentConfirmationStep,
        { provide: DeliveryCommandService, useValue: commands },
        { provide: 'PAYMENT_SERVICE', useValue: paymentClientGrpc },
      ],
    }).compile();

    step = module.get<PaymentConfirmationStep>(PaymentConfirmationStep);
    await step.onModuleInit();
  });

  it('should be defined', () => {
    expect(step).toBeDefined();
    expect(step.name).toBe('PAYMENT_AUTHORIZATION');
  });

  describe('execute', () => {
    it('should return context unchanged when payment service not available', async () => {
      const stepWithoutPayment = await createStepWithoutPaymentClient();
      const context: DeliverySagaContext = { delivery: mockDelivery() };

      const result = await stepWithoutPayment.execute(context);

      expect(result).toEqual(context);
    });

    async function createStepWithoutPaymentClient() {
      const commands = createMockDeliveryCommandService();
      const paymentClientGrpc = { getService: jest.fn().mockImplementation(() => { throw new Error('No service'); }) };
      
      const module = await Test.createTestingModule({
        providers: [
          PaymentConfirmationStep,
          { provide: DeliveryCommandService, useValue: commands },
          { provide: 'PAYMENT_SERVICE', useValue: paymentClientGrpc },
        ],
      }).compile();

      const step = module.get<PaymentConfirmationStep>(PaymentConfirmationStep);
      await step.onModuleInit();
      return step;
    }

    it('should throw error for invalid amount', async () => {
      const delivery = mockDelivery({ amount: 'invalid' });
      const context: DeliverySagaContext = { delivery };
      commands.updatePaymentStatus.mockResolvedValue(delivery);
      commands.cancel.mockResolvedValue(delivery);

      await expect(step.execute(context)).rejects.toThrow('Invalid delivery amount');
      expect(commands.updatePaymentStatus).toHaveBeenCalledWith(delivery.id, PaymentStatus.FAILED);
      expect(commands.cancel).toHaveBeenCalledWith(delivery.id, 'system', expect.any(String));
    });

    it('should throw error for zero amount', async () => {
      const delivery = mockDelivery({ amount: '0' });
      const context: DeliverySagaContext = { delivery };
      commands.updatePaymentStatus.mockResolvedValue(delivery);
      commands.cancel.mockResolvedValue(delivery);

      await expect(step.execute(context)).rejects.toThrow('Invalid delivery amount');
    });

    it('should throw error for negative amount', async () => {
      const delivery = mockDelivery({ amount: '-10' });
      const context: DeliverySagaContext = { delivery };
      commands.updatePaymentStatus.mockResolvedValue(delivery);
      commands.cancel.mockResolvedValue(delivery);

      await expect(step.execute(context)).rejects.toThrow('Invalid delivery amount');
    });

    it('should handle PENDING payment status with checkout URL', async () => {
      const delivery = mockDelivery({ amount: '100.00' });
      const context: DeliverySagaContext = { delivery };
      const checkoutUrl = 'https://checkout.example.com/pay/123';
      
      paymentServiceClient.CreatePayment.mockReturnValue(of({
        paymentId: { value: 'pay-123' },
        status: 'PENDING',
        paymentLink: checkoutUrl,
      }));
      commands.transition.mockResolvedValue({ ...delivery, status: DeliveryStatus.PENDING_PAYMENT, checkoutUrl });
      commands.updatePaymentStatus.mockResolvedValue({ ...delivery, paymentStatus: PaymentStatus.PENDING, checkoutUrl });

      const result = await step.execute(context);

      expect(commands.transition).toHaveBeenCalledWith(
        delivery.id,
        DeliveryStatus.PENDING_PAYMENT,
        undefined,
        'Awaiting customer checkout payment'
      );
      expect(commands.updatePaymentStatus).toHaveBeenCalledWith(delivery.id, PaymentStatus.PENDING);
      expect(result).toEqual({
        delivery: expect.objectContaining({
          status: DeliveryStatus.PENDING_PAYMENT,
          checkoutUrl,
        }),
        paymentId: 'pay-123',
      });
    });

    it('should handle PROCESSING payment status', async () => {
      const delivery = mockDelivery({ amount: '100.00' });
      const context: DeliverySagaContext = { delivery };
      const checkoutUrl = 'https://checkout.example.com/pay/123';
      
      paymentServiceClient.CreatePayment.mockReturnValue(of({
        paymentId: { value: 'pay-123' },
        status: 'PROCESSING',
        paymentLink: checkoutUrl,
      }));
      commands.transition.mockResolvedValue({ ...delivery, status: DeliveryStatus.PENDING_PAYMENT, checkoutUrl });
      commands.updatePaymentStatus.mockResolvedValue({ ...delivery, paymentStatus: PaymentStatus.PENDING, checkoutUrl });

      const result = await step.execute(context);

      expect(result.delivery.status).toBe(DeliveryStatus.PENDING_PAYMENT);
      expect(result.paymentId).toBe('pay-123');
    });

    it('should handle REQUIRES_PAYMENT status', async () => {
      const delivery = mockDelivery({ amount: '100.00' });
      const context: DeliverySagaContext = { delivery };
      
      paymentServiceClient.CreatePayment.mockReturnValue(of({
        payment_id: { value: 'pay-456' },
        status: 'REQUIRES_PAYMENT',
        checkout_url: 'https://checkout.example.com/pay/456',
      }));
      commands.transition.mockResolvedValue({ ...delivery, status: DeliveryStatus.PENDING_PAYMENT });
      commands.updatePaymentStatus.mockResolvedValue({ ...delivery, paymentStatus: PaymentStatus.PENDING });

      const result = await step.execute(context);

      expect(result.delivery.status).toBe(DeliveryStatus.PENDING_PAYMENT);
    });

    it('should handle AUTHORIZED payment status', async () => {
      const delivery = mockDelivery({ amount: '100.00' });
      const context: DeliverySagaContext = { delivery };
      
      paymentServiceClient.CreatePayment.mockReturnValue(of({
        paymentId: { value: 'pay-789' },
        status: 'AUTHORIZED',
      }));
      commands.transition.mockResolvedValue({ ...delivery, status: DeliveryStatus.PAYMENT_CONFIRMED });
      commands.updatePaymentStatus.mockResolvedValue({ ...delivery, paymentStatus: PaymentStatus.AUTHORIZED });

      const result = await step.execute(context);

      expect(commands.transition).toHaveBeenCalledWith(
        delivery.id,
        DeliveryStatus.PAYMENT_CONFIRMED,
        undefined,
        'Payment authorized & held in escrow'
      );
      expect(commands.updatePaymentStatus).toHaveBeenCalledWith(delivery.id, PaymentStatus.AUTHORIZED);
      expect(result).toEqual({
        delivery: expect.objectContaining({ status: DeliveryStatus.PAYMENT_CONFIRMED }),
        paymentId: 'pay-789',
      });
    });

    it('should handle SUCCEEDED payment status', async () => {
      const delivery = mockDelivery({ amount: '100.00' });
      const context: DeliverySagaContext = { delivery };
      
      paymentServiceClient.CreatePayment.mockReturnValue(of({
        paymentId: { value: 'pay-999' },
        status: 'SUCCEEDED',
      }));
      commands.transition.mockResolvedValue({ ...delivery, status: DeliveryStatus.PAYMENT_CONFIRMED });
      commands.updatePaymentStatus.mockResolvedValue({ ...delivery, paymentStatus: PaymentStatus.AUTHORIZED });

      const result = await step.execute(context);

      expect(result.delivery.status).toBe(DeliveryStatus.PAYMENT_CONFIRMED);
    });

    it('should throw error for FAILED payment status', async () => {
      const delivery = mockDelivery({ amount: '100.00' });
      const context: DeliverySagaContext = { delivery };
      
      paymentServiceClient.CreatePayment.mockReturnValue(of({
        paymentId: { value: 'pay-failed' },
        status: 'FAILED',
      }));
      commands.updatePaymentStatus.mockResolvedValue(delivery);
      commands.cancel.mockResolvedValue(delivery);

      await expect(step.execute(context)).rejects.toThrow('Payment authorization incomplete');
      expect(commands.updatePaymentStatus).toHaveBeenCalledWith(delivery.id, PaymentStatus.FAILED);
      expect(commands.cancel).toHaveBeenCalledWith(delivery.id, 'system', expect.any(String));
    });

    it('should handle CreatePayment error', async () => {
      const delivery = mockDelivery({ amount: '100.00' });
      const context: DeliverySagaContext = { delivery };
      
      paymentServiceClient.CreatePayment.mockReturnValue(throwError(() => new Error('Service unavailable')));
      commands.updatePaymentStatus.mockResolvedValue(delivery);
      commands.cancel.mockResolvedValue(delivery);

      await expect(step.execute(context)).rejects.toThrow('Service unavailable');
      expect(commands.updatePaymentStatus).toHaveBeenCalledWith(delivery.id, PaymentStatus.FAILED);
      expect(commands.cancel).toHaveBeenCalledWith(delivery.id, 'system', expect.any(String));
    });

    it('should calculate amount in minor units correctly', async () => {
      const delivery = mockDelivery({ amount: '100.50' });
      const context: DeliverySagaContext = { delivery };
      
      paymentServiceClient.CreatePayment.mockReturnValue(of({
        paymentId: { value: 'pay-123' },
        status: 'AUTHORIZED',
      }));
      commands.transition.mockResolvedValue(delivery);
      commands.updatePaymentStatus.mockResolvedValue(delivery);

      await step.execute(context);

      expect(paymentServiceClient.CreatePayment).toHaveBeenCalledWith(expect.objectContaining({
        amountMinor: 10050,
        amount_minor: 10050,
      }));
    });
  });

  describe('compensate', () => {
    it('should cancel payment authorization when paymentId exists', async () => {
      const context: DeliverySagaContext = {
        delivery: mockDelivery({ id: 'delivery-123' }),
        paymentId: 'pay-123',
      };
      commands.cancel.mockResolvedValue(context.delivery);
      commands.updatePaymentStatus.mockResolvedValue(context.delivery);
      paymentServiceClient.CancelAuthorization.mockReturnValue(of({}));

      await step.compensate(context);

      expect(paymentServiceClient.CancelAuthorization).toHaveBeenCalledWith({
        payment_id: 'pay-123',
        idempotency_key: 'delivery-delivery-123-void',
      });
      expect(commands.cancel).toHaveBeenCalledWith('delivery-123', 'saga', 'Saga compensation: delivery process cancelled');
      expect(commands.updatePaymentStatus).toHaveBeenCalledWith('delivery-123', PaymentStatus.CANCELLED);
    });

    it('should not call payment service when paymentId not provided', async () => {
      const context: DeliverySagaContext = {
        delivery: mockDelivery({ id: 'delivery-123' }),
      };
      commands.cancel.mockResolvedValue(context.delivery);
      commands.updatePaymentStatus.mockResolvedValue(context.delivery);

      await step.compensate(context);

      expect(paymentServiceClient.CancelAuthorization).not.toHaveBeenCalled();
      expect(commands.cancel).toHaveBeenCalledWith('delivery-123', 'saga', 'Saga compensation: delivery process cancelled');
    });

    it('should not throw if CancelAuthorization fails', async () => {
      const context: DeliverySagaContext = {
        delivery: mockDelivery({ id: 'delivery-123' }),
        paymentId: 'pay-123',
      };
      commands.cancel.mockResolvedValue(context.delivery);
      commands.updatePaymentStatus.mockResolvedValue(context.delivery);
      paymentServiceClient.CancelAuthorization.mockReturnValue(throwError(() => new Error('Failed to cancel')));

      await expect(step.compensate(context)).resolves.not.toThrow();
    });
  });
});