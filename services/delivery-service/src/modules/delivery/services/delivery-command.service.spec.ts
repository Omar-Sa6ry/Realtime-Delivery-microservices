import { BadRequestException } from '@nestjs/common';
import { Test, TestingModule } from '@nestjs/testing';
import { of, throwError } from 'rxjs';
import { lastValueFrom } from 'rxjs';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { PaymentStatus } from '../enums/payment-status.enum';
import { DeliveryCommandService } from './delivery-command.service';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { DeliveryStateMachine } from './delivery.state-machine';
import { IdempotencyService } from './idempotency.service';
import { OutboxRepository } from '../../outbox/outbox.repository';
import { DeliveryRabbitMQPublisher } from '../../../infrastructure/rabbitmq/rabbitmq.publisher';
import { NatsService, RealtimeNatsSubjects, NotificationNatsSubjects, ServerMessageType, DeliveryKafkaTopics } from '@delivery-micro/shard';
import { createMockDeliveryRepository, createMockDeliveryStateMachine, createMockIdempotencyService, createMockOutboxRepository, createMockDeliveryRabbitMQPublisher, createMockNatsService, createMockGrpcClient, mockDelivery, createMockAddress } from '../../../test-utils';

describe('DeliveryCommandService', () => {
  let service: DeliveryCommandService;
  let repository: ReturnType<typeof createMockDeliveryRepository>;
  let stateMachine: ReturnType<typeof createMockDeliveryStateMachine>;
  let idempotency: ReturnType<typeof createMockIdempotencyService>;
  let outbox: ReturnType<typeof createMockOutboxRepository>;
  let nats: ReturnType<typeof createMockNatsService>;
  let rabbitmqPublisher: ReturnType<typeof createMockDeliveryRabbitMQPublisher>;
  let userServiceClient: any;
  let paymentServiceClient: any;

  beforeEach(async () => {
    repository = createMockDeliveryRepository();
    stateMachine = createMockDeliveryStateMachine();
    idempotency = createMockIdempotencyService();
    outbox = createMockOutboxRepository();
    nats = createMockNatsService();
    rabbitmqPublisher = createMockDeliveryRabbitMQPublisher();

    userServiceClient = {
      GetUser: jest.fn(),
    };

    paymentServiceClient = {
      CreatePayment: jest.fn(),
      CapturePayment: jest.fn(),
      CancelAuthorization: jest.fn(),
      CreateRefund: jest.fn(),
    };

    const userClientGrpc = { getService: jest.fn().mockReturnValue(userServiceClient) };
    const paymentClientGrpc = { getService: jest.fn().mockReturnValue(paymentServiceClient) };

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DeliveryCommandService,
        { provide: DeliveryRepository, useValue: repository },
        { provide: DeliveryStateMachine, useValue: stateMachine },
        { provide: IdempotencyService, useValue: idempotency },
        { provide: OutboxRepository, useValue: outbox },
        { provide: 'USER_SERVICE', useValue: userClientGrpc },
        { provide: 'PAYMENT_SERVICE', useValue: paymentClientGrpc },
        { provide: NatsService, useValue: nats },
        { provide: DeliveryRabbitMQPublisher, useValue: rabbitmqPublisher },
      ],
    }).compile();

    service = module.get<DeliveryCommandService>(DeliveryCommandService);
    await service.onModuleInit();
  });

  it('should be defined', () => {
    expect(service).toBeDefined();
  });

  describe('create', () => {
    it('should create delivery and publish events', async () => {
      const input = {
        customerId: 'customer-123',
        amount: '100.00',
        currency: 'USD',
        pickupAddress: createMockAddress(),
        dropoffAddress: createMockAddress({ line1: '456 Destination St' }),
      };

      const createdDelivery = mockDelivery({ ...input, status: DeliveryStatus.CREATED, paymentStatus: PaymentStatus.PENDING });
      repository.create.mockResolvedValue(createdDelivery);

      const result = await service.create(input);

      expect(repository.create).toHaveBeenCalledWith({
        customerId: input.customerId,
        amount: input.amount,
        currency: input.currency,
        pickupAddress: input.pickupAddress,
        dropoffAddress: input.dropoffAddress,
        status: DeliveryStatus.CREATED,
        paymentStatus: PaymentStatus.PENDING,
      });

      expect(nats.emit).toHaveBeenCalledWith(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED, expect.objectContaining({
        deliveryId: createdDelivery.id,
        customerId: createdDelivery.customerId,
        status: DeliveryStatus.CREATED,
      }));

      expect(rabbitmqPublisher.publishOrderCreated).toHaveBeenCalledWith(createdDelivery);
      expect(result).toEqual(createdDelivery);
    });

    it('should validate customer via gRPC', async () => {
      const input = {
        customerId: 'customer-123',
        amount: '100.00',
        pickupAddress: createMockAddress(),
        dropoffAddress: createMockAddress({ line1: '456 Destination St' }),
      };

      userServiceClient.GetUser.mockReturnValue(of({ id: 'customer-123', name: 'Test User' }));
      repository.create.mockResolvedValue(mockDelivery(input));

      await service.create(input);

      expect(userServiceClient.GetUser).toHaveBeenCalledWith({ id: 'customer-123' });
    });

    it('should throw BadRequestException when customer not found', async () => {
      const input = {
        customerId: 'invalid-customer',
        amount: '100.00',
        pickupAddress: createMockAddress(),
        dropoffAddress: createMockAddress({ line1: '456 Destination St' }),
      };

      userServiceClient.GetUser.mockReturnValue(throwError(() => ({ code: 5, details: 'not found' })));
      repository.create.mockResolvedValue(mockDelivery(input));

      await expect(service.create(input)).rejects.toThrow(BadRequestException);
      expect(repository.create).not.toHaveBeenCalled();
    });

    it('should throw BadRequestException when user service returns empty user', async () => {
      const input = {
        customerId: 'customer-123',
        amount: '100.00',
        pickupAddress: createMockAddress(),
        dropoffAddress: createMockAddress({ line1: '456 Destination St' }),
      };

      userServiceClient.GetUser.mockReturnValue(of(null));
      repository.create.mockResolvedValue(mockDelivery(input));

      await expect(service.create(input)).rejects.toThrow(BadRequestException);
    });

    it('should use default currency when not provided', async () => {
      const input = {
        customerId: 'customer-123',
        amount: '100.00',
        pickupAddress: createMockAddress(),
        dropoffAddress: createMockAddress({ line1: '456 Destination St' }),
      };

      userServiceClient.GetUser.mockReturnValue(of({ id: 'customer-123' }));
      repository.create.mockResolvedValue(mockDelivery({ ...input, currency: 'USD' }));

      const result = await service.create(input);

      expect(result.currency).toBe('USD');
    });
  });

  describe('executeWithIdempotency', () => {
    it('should delegate to idempotency service', async () => {
      const operation = jest.fn().mockResolvedValue('result');
      idempotency.execute.mockResolvedValue('result');

      const result = await service.executeWithIdempotency('test-key', operation);

      expect(idempotency.execute).toHaveBeenCalledWith('test-key', operation);
      expect(result).toBe('result');
    });
  });

  describe('transition', () => {
    it('should transition delivery status and publish events', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.CREATED });
      const updatedDelivery = mockDelivery({ status: DeliveryStatus.DRIVER_ASSIGNED });
      repository.findById.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockResolvedValue(undefined);
      repository.appendHistory.mockResolvedValue({} as any);
      repository.save.mockResolvedValue(updatedDelivery);
      outbox.save.mockResolvedValue({} as any);

      const result = await service.transition('test-id', DeliveryStatus.DRIVER_ASSIGNED, 'driver-123', 'Assigned');

      expect(repository.findById).toHaveBeenCalledWith('test-id');
      expect(stateMachine.assertTransition).toHaveBeenCalledWith(DeliveryStatus.CREATED, DeliveryStatus.DRIVER_ASSIGNED);
      expect(delivery.status).toBe(DeliveryStatus.DRIVER_ASSIGNED);
      expect(repository.appendHistory).toHaveBeenCalledWith(delivery, DeliveryStatus.DRIVER_ASSIGNED, 'driver-123', 'Assigned');
      expect(repository.save).toHaveBeenCalledWith(delivery);
      expect(outbox.save).toHaveBeenCalled();
      expect(nats.emit).toHaveBeenCalledWith(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED, expect.any(Object));
      expect(rabbitmqPublisher.publishOrderStatusChanged).toHaveBeenCalledWith(updatedDelivery);
      expect(rabbitmqPublisher.publishDriverAssigned).toHaveBeenCalledWith(updatedDelivery.id, 'driver-123');
      expect(result).toEqual(updatedDelivery);
    });

    it('should set pickedUpAt when transitioning to PICKED_UP', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.IN_TRANSIT });
      const updatedDelivery = mockDelivery({ status: DeliveryStatus.PICKED_UP });
      repository.findById.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockResolvedValue(undefined);
      repository.appendHistory.mockResolvedValue({} as any);
      repository.save.mockResolvedValue(updatedDelivery);
      outbox.save.mockResolvedValue({} as any);

      await service.transition('test-id', DeliveryStatus.PICKED_UP);

      expect(delivery.pickedUpAt).toBeInstanceOf(Date);
    });

    it('should set completedAt when transitioning to COMPLETED', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.DELIVERED });
      const updatedDelivery = mockDelivery({ status: DeliveryStatus.COMPLETED });
      repository.findById.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockResolvedValue(undefined);
      repository.appendHistory.mockResolvedValue({} as any);
      repository.save.mockResolvedValue(updatedDelivery);
      outbox.save.mockResolvedValue({} as any);

      await service.transition('test-id', DeliveryStatus.COMPLETED);

      expect(delivery.completedAt).toBeInstanceOf(Date);
    });

    it('should set cancelledAt when transitioning to CANCELLED', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.CREATED });
      const updatedDelivery = mockDelivery({ status: DeliveryStatus.CANCELLED });
      repository.findById.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockResolvedValue(undefined);
      repository.appendHistory.mockResolvedValue({} as any);
      repository.save.mockResolvedValue(updatedDelivery);
      outbox.save.mockResolvedValue({} as any);

      await service.transition('test-id', DeliveryStatus.CANCELLED);

      expect(delivery.cancelledAt).toBeInstanceOf(Date);
    });

    it('should return delivery without changes if status is same', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.CREATED });
      repository.findById.mockResolvedValue(delivery);

      const result = await service.transition('test-id', DeliveryStatus.CREATED);

      expect(stateMachine.assertTransition).not.toHaveBeenCalled();
      expect(repository.appendHistory).not.toHaveBeenCalled();
      expect(repository.save).not.toHaveBeenCalled();
      expect(result).toEqual(delivery);
    });

    it('should throw BadRequestException for invalid transition', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.CREATED });
      repository.findById.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockRejectedValue(new BadRequestException('Invalid transition'));

      await expect(service.transition('test-id', DeliveryStatus.PICKED_UP)).rejects.toThrow(BadRequestException);
    });

    it('should publish notification when transitioning to PICKED_UP', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.IN_TRANSIT });
      const updatedDelivery = mockDelivery({ status: DeliveryStatus.PICKED_UP });
      repository.findById.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockResolvedValue(undefined);
      repository.appendHistory.mockResolvedValue({} as any);
      repository.save.mockResolvedValue(updatedDelivery);
      outbox.save.mockResolvedValue({} as any);

      await service.transition('test-id', DeliveryStatus.PICKED_UP);

      expect(nats.emit).toHaveBeenCalledWith(
        `${NotificationNatsSubjects.NOTIFICATION_USER}.${updatedDelivery.customerId}`,
        expect.objectContaining({ type: 'ORDER_PICKED_UP' })
      );
    });

    it('should capture payment when transitioning to COMPLETED', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.DELIVERED, amount: '100.00' });
      const updatedDelivery = mockDelivery({ status: DeliveryStatus.COMPLETED, amount: '100.00' });
      repository.findById.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockResolvedValue(undefined);
      repository.appendHistory.mockResolvedValue({} as any);
      repository.save.mockResolvedValue(updatedDelivery);
      outbox.save.mockResolvedValue({} as any);
      paymentServiceClient.CapturePayment.mockReturnValue(of({}));

      await service.transition('test-id', DeliveryStatus.COMPLETED);

      expect(paymentServiceClient.CapturePayment).toHaveBeenCalledWith({
        delivery_id: updatedDelivery.id,
        amount_minor: 10000,
        idempotency_key: `delivery-${updatedDelivery.id}-cap`,
      });
    });

    it('should publish DELIVERY_COMPLETED notification on completion', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.DELIVERED });
      const updatedDelivery = mockDelivery({ status: DeliveryStatus.COMPLETED });
      repository.findById.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockResolvedValue(undefined);
      repository.appendHistory.mockResolvedValue({} as any);
      repository.save.mockResolvedValue(updatedDelivery);
      outbox.save.mockResolvedValue({} as any);

      await service.transition('test-id', DeliveryStatus.COMPLETED);

      expect(nats.emit).toHaveBeenCalledWith(
        `${NotificationNatsSubjects.NOTIFICATION_USER}.${updatedDelivery.customerId}`,
        expect.objectContaining({ type: 'DELIVERY_COMPLETED' })
      );
    });
  });

  describe('updatePaymentStatus', () => {
    it('should update payment status', async () => {
      const delivery = mockDelivery({ paymentStatus: PaymentStatus.PENDING });
      repository.findById.mockResolvedValue(delivery);
      repository.save.mockResolvedValue({ ...delivery, paymentStatus: PaymentStatus.AUTHORIZED });

      const result = await service.updatePaymentStatus('test-id', PaymentStatus.AUTHORIZED);

      expect(repository.findById).toHaveBeenCalledWith('test-id');
      expect(delivery.paymentStatus).toBe(PaymentStatus.AUTHORIZED);
      expect(repository.save).toHaveBeenCalledWith(delivery);
      expect(result.paymentStatus).toBe(PaymentStatus.AUTHORIZED);
    });
  });

  describe('assignDriver', () => {
    it('should assign driver and transition to DRIVER_ASSIGNED', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.PAYMENT_CONFIRMED });
      const assignedDelivery = mockDelivery({ status: DeliveryStatus.DRIVER_ASSIGNED, driverId: 'driver-123' });
      repository.findById.mockResolvedValue(delivery);
      repository.save.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockResolvedValue(undefined);
      repository.appendHistory.mockResolvedValue({} as any);
      repository.save.mockResolvedValue(assignedDelivery);
      outbox.save.mockResolvedValue({} as any);

      const result = await service.assignDriver('test-id', 'driver-123');

      expect(delivery.driverId).toBe('driver-123');
      expect(rabbitmqPublisher.publishDispatchRequest).toHaveBeenCalledWith(delivery);
      expect(result).toEqual(assignedDelivery);
    });
  });

  describe('acceptDriver', () => {
    it('should accept driver and transition to DRIVER_ACCEPTED', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.DRIVER_ASSIGNED, driverId: 'driver-123' });
      const acceptedDelivery = mockDelivery({ status: DeliveryStatus.DRIVER_ACCEPTED });
      repository.findById.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockResolvedValue(undefined);
      repository.appendHistory.mockResolvedValue({} as any);
      repository.save.mockResolvedValue(acceptedDelivery);
      outbox.save.mockResolvedValue({} as any);

      const result = await service.acceptDriver('test-id', 'driver-123');

      expect(result).toEqual(acceptedDelivery);
      expect(nats.emit).toHaveBeenCalledWith(
        `${NotificationNatsSubjects.NOTIFICATION_USER}.${acceptedDelivery.customerId}`,
        expect.objectContaining({ type: 'DRIVER_ACCEPTED' })
      );
      expect(nats.emit).toHaveBeenCalledWith(RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED, expect.any(Object));
    });

    it('should assign driverId if not set', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.PAYMENT_CONFIRMED, driverId: null });
      const acceptedDelivery = mockDelivery({ status: DeliveryStatus.DRIVER_ACCEPTED, driverId: 'driver-123' });
      repository.findById.mockResolvedValue(delivery);
      repository.save.mockResolvedValue(delivery);
      stateMachine.assertTransition.mockResolvedValue(undefined);
      repository.appendHistory.mockResolvedValue({} as any);
      repository.save.mockResolvedValue(acceptedDelivery);
      outbox.save.mockResolvedValue({} as any);

      await service.acceptDriver('test-id', 'driver-123');

      expect(delivery.driverId).toBe('driver-123');
    });
  });

  describe('retryDriverDispatch', () => {
    it('should return early if delivery is not in PAYMENT_CONFIRMED', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.DRIVER_ASSIGNED });
      repository.findById.mockResolvedValue(delivery);

      await service.retryDriverDispatch(delivery);

      expect(nats.emit).not.toHaveBeenCalled();
    });

    it('should return early if driver already assigned', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.PAYMENT_CONFIRMED, driverId: 'driver-123' });

      await service.retryDriverDispatch(delivery);

      expect(nats.emit).not.toHaveBeenCalled();
    });

    it('should handle timeout if 10 minutes elapsed', async () => {
      const delivery = mockDelivery({
        status: DeliveryStatus.PAYMENT_CONFIRMED,
        driverId: null,
        createdAt: new Date(Date.now() - 11 * 60 * 1000),
      });

      service.handleDriverSearchTimeout = jest.fn().mockResolvedValue(undefined);

      await service.retryDriverDispatch(delivery);

      expect(service.handleDriverSearchTimeout).toHaveBeenCalledWith(delivery.id);
    });

    it('should publish retry event and re-publish to outbox', async () => {
      const delivery = mockDelivery({
        status: DeliveryStatus.PAYMENT_CONFIRMED,
        driverId: null,
        createdAt: new Date(Date.now() - 5 * 60 * 1000),
      });

      await service.retryDriverDispatch(delivery);

      expect(nats.emit).toHaveBeenCalledWith(RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED, expect.objectContaining({
        type: ServerMessageType.DRIVER_SEARCH_RETRY,
        deliveryId: delivery.id,
      }));
      expect(outbox.save).toHaveBeenCalledWith(expect.objectContaining({
        eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
      }));
    });
  });

  describe('handleDriverRejectedOrExpired', () => {
    it('should return early for terminal states', async () => {
      const terminalStates = [
        DeliveryStatus.DRIVER_ACCEPTED,
        DeliveryStatus.CANCELLED,
        DeliveryStatus.FAILED,
        DeliveryStatus.COMPLETED,
      ];

      for (const status of terminalStates) {
        const delivery = mockDelivery({ status });
        repository.findById.mockResolvedValue(delivery);

        await service.handleDriverRejectedOrExpired('test-id', 'REJECTED');

        expect(repository.clearDriverId).not.toHaveBeenCalled();
        repository.findById.mockClear();
      }
    });

    it('should clear driverId and handle timeout if 10 minutes elapsed', async () => {
      const delivery = mockDelivery({
        status: DeliveryStatus.DRIVER_ASSIGNED,
        createdAt: new Date(Date.now() - 11 * 60 * 1000),
      });
      repository.findById.mockResolvedValue(delivery);
      service.handleDriverSearchTimeout = jest.fn().mockResolvedValue(undefined);

      await service.handleDriverRejectedOrExpired('test-id', 'REJECTED');

      expect(repository.clearDriverId).toHaveBeenCalledWith('test-id');
      expect(service.handleDriverSearchTimeout).toHaveBeenCalledWith('test-id');
    });

    it('should re-publish DELIVERY_CREATED event for retry', async () => {
      const delivery = mockDelivery({
        status: DeliveryStatus.DRIVER_ASSIGNED,
        createdAt: new Date(Date.now() - 5 * 60 * 1000),
      });
      repository.findById.mockResolvedValue(delivery);

      await service.handleDriverRejectedOrExpired('test-id', 'EXPIRED');

      expect(repository.clearDriverId).toHaveBeenCalledWith('test-id');
      expect(nats.emit).toHaveBeenCalledWith(RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED, expect.objectContaining({
        type: ServerMessageType.DRIVER_SEARCH_RETRY,
        reason: 'EXPIRED',
      }));
      expect(outbox.save).toHaveBeenCalledWith(expect.objectContaining({
        eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
      }));
    });
  });

  describe('handleAssignmentRejected', () => {
    it('should delegate to handleDriverRejectedOrExpired', async () => {
      service.handleDriverRejectedOrExpired = jest.fn().mockResolvedValue(undefined);

      await service.handleAssignmentRejected('test-id', 'driver-123', 'NOT_AVAILABLE');

      expect(service.handleDriverRejectedOrExpired).toHaveBeenCalledWith('test-id', 'DRIVER_REJECTED: NOT_AVAILABLE');
    });
  });

  describe('handleDriverSearchTimeout', () => {
    it('should return early for terminal states', async () => {
      const terminalStates = [
        DeliveryStatus.DRIVER_ACCEPTED,
        DeliveryStatus.CANCELLED,
        DeliveryStatus.FAILED,
        DeliveryStatus.COMPLETED,
      ];

      for (const status of terminalStates) {
        const delivery = mockDelivery({ status });
        repository.findById.mockResolvedValue(delivery);

        await service.handleDriverSearchTimeout('test-id');

        expect(service.cancel).not.toHaveBeenCalled();
        repository.findById.mockClear();
      }
    });

    it('should cancel delivery and notify customer', async () => {
      const delivery = mockDelivery({ status: DeliveryStatus.PAYMENT_CONFIRMED });
      const cancelledDelivery = mockDelivery({ status: DeliveryStatus.CANCELLED });
      repository.findById.mockResolvedValue(delivery);
      service.cancel = jest.fn().mockResolvedValue(cancelledDelivery);

      await service.handleDriverSearchTimeout('test-id');

      expect(service.cancel).toHaveBeenCalledWith('test-id', 'system', 'No driver found within 10 minutes');
      expect(nats.emit).toHaveBeenCalledWith(
        `${NotificationNatsSubjects.NOTIFICATION_USER}.${cancelledDelivery.customerId}`,
        expect.objectContaining({ type: 'DELIVERY_CANCELLED' })
      );
    });
  });

  describe('cancel', () => {
    it('should cancel authorization if payment is AUTHORIZED', async () => {
      const delivery = mockDelivery({ paymentStatus: PaymentStatus.AUTHORIZED });
      const cancelledDelivery = mockDelivery({ status: DeliveryStatus.CANCELLED });
      repository.findById.mockResolvedValue(delivery);
      service.transition = jest.fn().mockResolvedValue(cancelledDelivery);
      paymentServiceClient.CancelAuthorization.mockReturnValue(of({}));

      const result = await service.cancel('test-id', 'user-123', 'Customer requested');

      expect(paymentServiceClient.CancelAuthorization).toHaveBeenCalledWith({
        delivery_id: 'test-id',
        idempotency_key: 'delivery-test-id-void',
      });
      expect(service.updatePaymentStatus).toHaveBeenCalledWith('test-id', PaymentStatus.CANCELLED);
      expect(service.transition).toHaveBeenCalledWith('test-id', DeliveryStatus.CANCELLED, 'user-123', 'Customer requested');
      expect(result).toEqual(cancelledDelivery);
    });

    it('should refund if payment is COMPLETED', async () => {
      const delivery = mockDelivery({ paymentStatus: PaymentStatus.COMPLETED, amount: '100.00' });
      const cancelledDelivery = mockDelivery({ status: DeliveryStatus.CANCELLED });
      repository.findById.mockResolvedValue(delivery);
      service.transition = jest.fn().mockResolvedValue(cancelledDelivery);
      paymentServiceClient.CreateRefund.mockReturnValue(of({}));

      await service.cancel('test-id', 'user-123', 'Customer requested');

      expect(paymentServiceClient.CreateRefund).toHaveBeenCalledWith({
        delivery_id: 'test-id',
        amount_minor: 10000,
        reason: 'Customer requested',
        idempotency_key: 'delivery-test-id-refund',
      });
      expect(service.updatePaymentStatus).toHaveBeenCalledWith('test-id', PaymentStatus.REFUNDED);
    });

    it('should not call payment service if payment is PENDING', async () => {
      const delivery = mockDelivery({ paymentStatus: PaymentStatus.PENDING });
      const cancelledDelivery = mockDelivery({ status: DeliveryStatus.CANCELLED });
      repository.findById.mockResolvedValue(delivery);
      service.transition = jest.fn().mockResolvedValue(cancelledDelivery);

      await service.cancel('test-id');

      expect(paymentServiceClient.CancelAuthorization).not.toHaveBeenCalled();
      expect(paymentServiceClient.CreateRefund).not.toHaveBeenCalled();
    });

    it('should transition to CANCELLED', async () => {
      const delivery = mockDelivery({ paymentStatus: PaymentStatus.PENDING });
      const cancelledDelivery = mockDelivery({ status: DeliveryStatus.CANCELLED });
      repository.findById.mockResolvedValue(delivery);
      service.transition = jest.fn().mockResolvedValue(cancelledDelivery);

      const result = await service.cancel('test-id');

      expect(service.transition).toHaveBeenCalledWith('test-id', DeliveryStatus.CANCELLED, undefined, undefined);
      expect(result).toEqual(cancelledDelivery);
    });
  });

  describe('clearIdempotency', () => {
    it('should delegate to idempotency service', async () => {
      await service.clearIdempotency('test-key');

      expect(idempotency.clear).toHaveBeenCalledWith('test-key');
    });

    it('should not call idempotency if key is empty', async () => {
      await service.clearIdempotency('');

      expect(idempotency.clear).not.toHaveBeenCalled();
    });
  });
});