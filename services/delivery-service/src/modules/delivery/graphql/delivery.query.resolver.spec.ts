import { BadRequestException, ForbiddenException } from '@nestjs/common';
import { Test, TestingModule } from '@nestjs/testing';
import { I18nService } from 'nestjs-i18n';
import { DeliveryQueryResolver } from './delivery.query.resolver';
import { DeliveryQueryService } from '../services/delivery-query.service';
import { DeliveryStatus } from '../../enums/delivery-status.enum';
import { createMockDeliveryQueryService, createMockI18nService, createMockGraphQLContext, createMockGraphQLContextForAdmin, mockDelivery } from '../../../../test-utils';

describe('DeliveryQueryResolver', () => {
  let resolver: DeliveryQueryResolver;
  let queries: ReturnType<typeof createMockDeliveryQueryService>;
  let i18n: ReturnType<typeof createMockI18nService>;

  beforeEach(async () => {
    queries = createMockDeliveryQueryService();
    i18n = createMockI18nService();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DeliveryQueryResolver,
        { provide: DeliveryQueryService, useValue: queries },
        { provide: I18nService, useValue: i18n },
      ],
    }).compile();

    resolver = module.get<DeliveryQueryResolver>(DeliveryQueryResolver);
  });

  it('should be defined', () => {
    expect(resolver).toBeDefined();
  });

  describe('delivery', () => {
    const ctx = createMockGraphQLContext({ userId: 'user-123', userRole: 'customer' });
    const delivery = mockDelivery({ id: 'delivery-123', customerId: 'user-123' });

    beforeEach(() => {
      i18n.t.mockResolvedValue('translated message');
      queries.getById.mockResolvedValue(delivery);
    });

    it('should return delivery for owner', async () => {
      const result = await resolver.delivery('delivery-123', ctx);

      expect(queries.getById).toHaveBeenCalledWith('delivery-123');
      expect(result).toEqual(expect.objectContaining({
        success: true,
        statusCode: 200,
        data: expect.objectContaining({ id: 'delivery-123' }),
      }));
    });

    it('should return delivery for admin', async () => {
      const adminCtx = createMockGraphQLContextForAdmin({ userId: 'admin-123' });
      const otherDelivery = mockDelivery({ id: 'delivery-123', customerId: 'other-user' });
      queries.getById.mockResolvedValue(otherDelivery);

      const result = await resolver.delivery('delivery-123', adminCtx);

      expect(result.success).toBe(true);
    });

    it('should return delivery for assigned driver', async () => {
      const driverCtx = createMockGraphQLContext({ userId: 'driver-123', userRole: 'driver' });
      const driverDelivery = mockDelivery({ id: 'delivery-123', customerId: 'other-user', driverId: 'driver-123' });
      queries.getById.mockResolvedValue(driverDelivery);

      const result = await resolver.delivery('delivery-123', driverCtx);

      expect(result.success).toBe(true);
    });

    it('should throw ForbiddenException for unauthorized user', async () => {
      const unauthorizedCtx = createMockGraphQLContext({ userId: 'other-user', userRole: 'customer' });
      const otherDelivery = mockDelivery({ id: 'delivery-123', customerId: 'owner-user' });
      queries.getById.mockResolvedValue(otherDelivery);
      i18n.t.mockResolvedValue('Unauthorized access');

      await expect(resolver.delivery('delivery-123', unauthorizedCtx)).rejects.toThrow(ForbiddenException);
      expect(i18n.t).toHaveBeenCalledWith('delivery.unauthorizedAccess', { lang: 'en' });
    });
  });

  describe('myDeliveries', () => {
    const ctx = createMockGraphQLContext({ userId: 'user-123' });
    const deliveries = [mockDelivery({ id: '1', customerId: 'user-123' }), mockDelivery({ id: '2', customerId: 'user-123' })];

    beforeEach(() => {
      i18n.t.mockResolvedValue('translated message');
      queries.listByCustomer.mockResolvedValue([deliveries, 2]);
    });

    it('should return paginated deliveries for customer', async () => {
      const result = await resolver.myDeliveries(1, 50, ctx);

      expect(queries.listByCustomer).toHaveBeenCalledWith('user-123', 1, 50);
      expect(result).toEqual(expect.objectContaining({
        success: true,
        statusCode: 200,
        data: expect.objectContaining({
          items: expect.arrayContaining([
            expect.objectContaining({ id: '1' }),
            expect.objectContaining({ id: '2' }),
          ]),
          paginationInfo: expect.objectContaining({
            totalItems: 2,
            currentPage: 1,
            nextPage: 2,
          }),
        }),
      }));
    });

    it('should calculate nextPage correctly', async () => {
      queries.listByCustomer.mockResolvedValue([deliveries, 100]);

      const result = await resolver.myDeliveries(2, 50, ctx);

      expect(result.data.paginationInfo.nextPage).toBeNull();
    });

    it('should throw BadRequestException when customerId missing', async () => {
      const noUserCtx = createMockGraphQLContext({ userId: undefined });
      i18n.t.mockResolvedValue('Authenticated customer required');

      await expect(resolver.myDeliveries(1, 50, noUserCtx)).rejects.toThrow(BadRequestException);
      expect(i18n.t).toHaveBeenCalledWith('delivery.authenticatedCustomerRequired', { lang: 'en' });
    });
  });

  describe('deliveryNextStatuses', () => {
    const ctx = createMockGraphQLContext({ userId: 'user-123' });
    const delivery = mockDelivery({ id: 'delivery-123', customerId: 'user-123', status: DeliveryStatus.CREATED });
    const nextStatuses = [DeliveryStatus.PENDING_PAYMENT, DeliveryStatus.PAYMENT_CONFIRMED, DeliveryStatus.DRIVER_ASSIGNED, DeliveryStatus.CANCELLED];

    beforeEach(() => {
      i18n.t.mockResolvedValue('translated message');
      queries.getById.mockResolvedValue(delivery);
      queries.nextStatuses.mockReturnValue(nextStatuses);
    });

    it('should return next valid statuses', async () => {
      const result = await resolver.deliveryNextStatuses('delivery-123', ctx);

      expect(queries.getById).toHaveBeenCalledWith('delivery-123');
      expect(queries.nextStatuses).toHaveBeenCalledWith(delivery);
      expect(result).toEqual(expect.objectContaining({
        success: true,
        statusCode: 200,
        data: { statuses: nextStatuses },
      }));
    });
  });

  describe('openDeliveries', () => {
    const ctx = createMockGraphQLContext({ userId: 'user-123', userRole: 'admin' });
    const deliveries = [
      mockDelivery({ id: '1', status: DeliveryStatus.PAYMENT_CONFIRMED, driverId: null }),
      mockDelivery({ id: '2', status: DeliveryStatus.DRIVER_ASSIGNED, driverId: null }),
    ];

    beforeEach(() => {
      i18n.t.mockResolvedValue('translated message');
      queries.getOpenDeliveries.mockResolvedValue([deliveries, 2]);
    });

    it('should return open deliveries', async () => {
      const result = await resolver.openDeliveries(1, 50, ctx);

      expect(queries.getOpenDeliveries).toHaveBeenCalledWith(1, 50);
      expect(result).toEqual(expect.objectContaining({
        success: true,
        statusCode: 200,
        data: expect.objectContaining({
          items: expect.arrayContaining([
            expect.objectContaining({ id: '1', status: DeliveryStatus.PAYMENT_CONFIRMED }),
            expect.objectContaining({ id: '2', status: DeliveryStatus.DRIVER_ASSIGNED }),
          ]),
          paginationInfo: expect.objectContaining({
            totalItems: 2,
            currentPage: 1,
            nextPage: 2,
          }),
        }),
      }));
    });
  });
});