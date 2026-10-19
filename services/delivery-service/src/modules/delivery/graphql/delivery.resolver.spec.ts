import { BadRequestException, ForbiddenException } from '@nestjs/common';
import { Test, TestingModule } from '@nestjs/testing';
import { I18nService } from 'nestjs-i18n';
import { DeliveryResolver } from './delivery.resolver';
import { DeliveryCommandService } from '../services/delivery-command.service';
import { DeliveryQueryService } from '../services/delivery-query.service';
import { DeliverySagaOrchestrator } from '../saga/delivery-saga.orchestrator';
import { DeliveryStatus } from '../../enums/delivery-status.enum';
import { CreateDeliveryInputDto, TransitionDeliveryInput } from './delivery.dto';
import { createMockDeliveryCommandService, createMockDeliveryQueryService, createMockDeliverySagaOrchestrator, createMockI18nService, createMockGraphQLContext, createMockGraphQLContextForAdmin, mockDelivery, createMockAddress } from '../../../../test-utils';

describe('DeliveryResolver', () => {
  let resolver: DeliveryResolver;
  let commands: ReturnType<typeof createMockDeliveryCommandService>;
  let queries: ReturnType<typeof createMockDeliveryQueryService>;
  let saga: ReturnType<typeof createMockDeliverySagaOrchestrator>;
  let i18n: ReturnType<typeof createMockI18nService>;

  beforeEach(async () => {
    commands = createMockDeliveryCommandService();
    queries = createMockDeliveryQueryService();
    saga = createMockDeliverySagaOrchestrator();
    i18n = createMockI18nService();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DeliveryResolver,
        { provide: DeliveryCommandService, useValue: commands },
        { provide: DeliveryQueryService, useValue: queries },
        { provide: DeliverySagaOrchestrator, useValue: saga },
        { provide: I18nService, useValue: i18n },
      ],
    }).compile();

    resolver = module.get<DeliveryResolver>(DeliveryResolver);
  });

  it('should be defined', () => {
    expect(resolver).toBeDefined();
  });

  describe('resolveReference', () => {
    it('should return delivery for reference', async () => {
      const delivery = mockDelivery({ id: 'ref-123' });
      queries.getById.mockResolvedValue(delivery);

      const result = await resolver.resolveReference({ __typename: 'DeliveryType', id: 'ref-123' });

      expect(queries.getById).toHaveBeenCalledWith('ref-123');
      expect(result).toEqual(expect.objectContaining({ id: 'ref-123' }));
    });
  });

  describe('createDelivery', () => {
    const createInput: CreateDeliveryInputDto = {
      amount: '100.00',
      currency: 'USD',
      pickupAddress: {
        line1: '123 Source St',
        city: 'San Francisco',
      },
      dropoffAddress: {
        line1: '456 Destination St',
        city: 'San Francisco',
      },
      idempotencyKey: 'idem-key-123',
    };

    const ctx = createMockGraphQLContext({ userId: 'user-123', userRole: 'customer' });
    const createdDelivery = mockDelivery({ customerId: 'user-123' });
    const confirmedDelivery = mockDelivery({ customerId: 'user-123', status: DeliveryStatus.PAYMENT_CONFIRMED });

    beforeEach(() => {
      i18n.t.mockResolvedValue('translated message');
      commands.create.mockResolvedValue(createdDelivery);
      saga.execute.mockResolvedValue(confirmedDelivery);
    });

    it('should create delivery and execute saga', async () => {
      const result = await resolver.createDelivery(createInput, ctx);

      expect(commands.create).toHaveBeenCalledWith(expect.objectContaining({
        customerId: 'user-123',
        amount: '100.00',
      }));
      expect(saga.execute).toHaveBeenCalledWith(createdDelivery.id);
      expect(result).toEqual(expect.objectContaining({
        success: true,
        statusCode: 201,
        data: expect.objectContaining({ id: confirmedDelivery.id }),
      }));
    });

    it('should use idempotency when key provided', async () => {
      commands.executeWithIdempotency.mockResolvedValue({
        success: true,
        statusCode: 201,
        message: 'created',
        data: confirmedDelivery,
      });

      await resolver.createDelivery(createInput, ctx);

      expect(commands.executeWithIdempotency).toHaveBeenCalledWith('idem-key-123', expect.any(Function));
    });

    it('should throw ForbiddenException for admin user', async () => {
      const adminCtx = createMockGraphQLContextForAdmin({ userId: 'admin-123' });

      await expect(resolver.createDelivery(createInput, adminCtx)).rejects.toThrow(ForbiddenException);
      expect(i18n.t).toHaveBeenCalledWith('delivery.onlyRegularUsersCanCreate', { lang: 'en' });
    });

    it('should throw BadRequestException when customerId missing', async () => {
      const noUserCtx = createMockGraphQLContext({ userId: undefined });

      await expect(resolver.createDelivery(createInput, noUserCtx)).rejects.toThrow(BadRequestException);
      expect(i18n.t).toHaveBeenCalledWith('delivery.customerIdRequired', { lang: 'en' });
    });

    it('should throw BadRequestException when saga fails', async () => {
      i18n.t.mockResolvedValueOnce('Payment authorization failed');
      saga.execute.mockRejectedValue(new Error('Payment failed'));

      await expect(resolver.createDelivery(createInput, ctx)).rejects.toThrow(BadRequestException);
    });

    it('should schedule timeout check when not pending payment', async () => {
      const setTimeoutSpy = jest.spyOn(global, 'setTimeout');
      saga.execute.mockResolvedValue(mockDelivery({ status: DeliveryStatus.PAYMENT_CONFIRMED }));

      await resolver.createDelivery(createInput, ctx);

      expect(setTimeoutSpy).toHaveBeenCalledWith(expect.any(Function), 10 * 60 * 1000);
    });

    it('should not schedule timeout check when pending payment', async () => {
      const setTimeoutSpy = jest.spyOn(global, 'setTimeout');
      saga.execute.mockResolvedValue(mockDelivery({ status: DeliveryStatus.PENDING_PAYMENT }));

      await resolver.createDelivery(createInput, ctx);

      expect(setTimeoutSpy).not.toHaveBeenCalled();
    });
  });

  describe('transitionDelivery', () => {
    const transitionInput: TransitionDeliveryInput = {
      deliveryId: 'delivery-123',
      status: DeliveryStatus.DRIVER_ASSIGNED,
      note: 'Driver assigned',
    };

    const ctx = createMockGraphQLContext({ userId: 'user-123' });
    const updatedDelivery = mockDelivery({ status: DeliveryStatus.DRIVER_ASSIGNED });

    beforeEach(() => {
      i18n.t.mockResolvedValue('translated message');
      commands.transition.mockResolvedValue(updatedDelivery);
    });

    it('should transition delivery status', async () => {
      const result = await resolver.transitionDelivery(transitionInput, ctx);

      expect(commands.transition).toHaveBeenCalledWith('delivery-123', DeliveryStatus.DRIVER_ASSIGNED, 'user-123', 'Driver assigned');
      expect(result).toEqual(expect.objectContaining({
        success: true,
        statusCode: 200,
        data: expect.objectContaining({ status: DeliveryStatus.DRIVER_ASSIGNED }),
      }));
    });

    it('should use id field when deliveryId not provided', async () => {
      const inputWithId = { ...transitionInput, deliveryId: undefined, id: 'delivery-456' };
      commands.transition.mockResolvedValue(updatedDelivery);

      await resolver.transitionDelivery(inputWithId, ctx);

      expect(commands.transition).toHaveBeenCalledWith('delivery-456', DeliveryStatus.DRIVER_ASSIGNED, 'user-123', 'Driver assigned');
    });

    it('should throw BadRequestException when no deliveryId provided', async () => {
      const noIdInput = { ...transitionInput, deliveryId: undefined, id: undefined };
      i18n.t.mockResolvedValue('Delivery ID is required');

      await expect(resolver.transitionDelivery(noIdInput, ctx)).rejects.toThrow(BadRequestException);
      expect(i18n.t).toHaveBeenCalledWith('delivery.deliveryIdRequired', { lang: 'en' });
    });
  });

  describe('cancelDelivery', () => {
    const cancelArgs = {
      deliveryIdArg: 'delivery-123',
      idArg: undefined,
      reason: 'Customer cancelled',
    };

    const ctx = createMockGraphQLContext({ userId: 'user-123' });
    const existingDelivery = mockDelivery({ customerId: 'user-123' });
    const cancelledDelivery = mockDelivery({ status: DeliveryStatus.CANCELLED, customerId: 'user-123' });

    beforeEach(() => {
      i18n.t.mockResolvedValue('translated message');
      queries.getById.mockResolvedValue(existingDelivery);
      commands.cancel.mockResolvedValue(cancelledDelivery);
    });

    it('should cancel delivery for owner', async () => {
      const result = await resolver.cancelDelivery(cancelArgs.deliveryIdArg, cancelArgs.idArg, cancelArgs.reason, ctx);

      expect(queries.getById).toHaveBeenCalledWith('delivery-123');
      expect(commands.cancel).toHaveBeenCalledWith('delivery-123', 'user-123', 'Customer cancelled');
      expect(result).toEqual(expect.objectContaining({
        success: true,
        statusCode: 200,
        data: expect.objectContaining({ status: DeliveryStatus.CANCELLED }),
      }));
    });

    it('should allow admin to cancel any delivery', async () => {
      const adminCtx = createMockGraphQLContextForAdmin({ userId: 'admin-123' });
      const otherUserDelivery = mockDelivery({ customerId: 'other-user' });
      queries.getById.mockResolvedValue(otherUserDelivery);

      await resolver.cancelDelivery('delivery-123', undefined, 'Admin cancel', adminCtx);

      expect(commands.cancel).toHaveBeenCalledWith('delivery-123', 'admin-123', 'Admin cancel');
    });

    it('should throw ForbiddenException when non-owner tries to cancel', async () => {
      const otherUserDelivery = mockDelivery({ customerId: 'other-user' });
      queries.getById.mockResolvedValue(otherUserDelivery);
      i18n.t.mockResolvedValue('Unauthorized to cancel');

      await expect(resolver.cancelDelivery('delivery-123', undefined, 'Cancel', ctx)).rejects.toThrow(ForbiddenException);
      expect(i18n.t).toHaveBeenCalledWith('delivery.unauthorizedCancel', { lang: 'en' });
    });

    it('should throw BadRequestException when no deliveryId provided', async () => {
      i18n.t.mockResolvedValue('Delivery ID is required');

      await expect(resolver.cancelDelivery(undefined, undefined, 'reason', ctx)).rejects.toThrow(BadRequestException);
      expect(i18n.t).toHaveBeenCalledWith('delivery.deliveryIdRequired', { lang: 'en' });
    });

    it('should use id field when deliveryId not provided', async () => {
      await resolver.cancelDelivery(undefined, 'delivery-456', 'reason', ctx);

      expect(commands.cancel).toHaveBeenCalledWith('delivery-456', 'user-123', 'reason');
    });
  });
});