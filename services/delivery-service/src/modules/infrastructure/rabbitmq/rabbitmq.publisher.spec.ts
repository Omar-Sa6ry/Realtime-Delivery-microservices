import { Test, TestingModule } from '@nestjs/testing';
import { DeliveryRabbitMQPublisher } from './rabbitmq.publisher';
import { RabbitMQService, RabbitMQExchanges, OrdersRoutingKeys, DispatchRoutingKeys } from '@delivery-micro/shard';
import { createMockRabbitMQService } from '../../../test-utils';

describe('DeliveryRabbitMQPublisher', () => {
  let publisher: DeliveryRabbitMQPublisher;
  let rabbitmq: ReturnType<typeof createMockRabbitMQService>;

  beforeEach(async () => {
    rabbitmq = createMockRabbitMQService();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DeliveryRabbitMQPublisher,
        { provide: RabbitMQService, useValue: rabbitmq },
      ],
    }).compile();

    publisher = module.get<DeliveryRabbitMQPublisher>(DeliveryRabbitMQPublisher);
  });

  it('should be defined', () => {
    expect(publisher).toBeDefined();
  });

  describe('publishOrderCreated', () => {
    it('should publish order created event', async () => {
      const delivery = {
        id: 'delivery-123',
        customerId: 'customer-123',
        driverId: null,
        status: 'CREATED',
        amount: '100.00',
        currency: 'USD',
        pickupAddress: { line1: '123 St', city: 'SF' },
        dropoffAddress: { line1: '456 Ave', city: 'LA' },
        paymentStatus: 'PENDING',
        updatedAt: new Date(),
      };

      await publisher.publishOrderCreated(delivery);

      expect(rabbitmq.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.ORDERS,
        OrdersRoutingKeys.CREATED,
        'delivery.created',
        expect.objectContaining({
          deliveryId: 'delivery-123',
          customerId: 'customer-123',
          status: 'CREATED',
        }),
        { aggregateId: 'delivery-123', aggregateType: 'delivery' }
      );
    });
  });

  describe('publishOrderStatusChanged', () => {
    it('should publish order status changed event', async () => {
      const delivery = {
        id: 'delivery-123',
        customerId: 'customer-123',
        driverId: 'driver-123',
        status: 'DRIVER_ASSIGNED',
        amount: '100.00',
        currency: 'USD',
        pickupAddress: { line1: '123 St' },
        dropoffAddress: { line1: '456 Ave' },
        paymentStatus: 'AUTHORIZED',
        updatedAt: new Date(),
      };

      await publisher.publishOrderStatusChanged(delivery);

      expect(rabbitmq.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.ORDERS,
        'order.driver_assigned',
        'delivery.status.changed',
        expect.objectContaining({
          deliveryId: 'delivery-123',
          status: 'DRIVER_ASSIGNED',
        }),
        { aggregateId: 'delivery-123', aggregateType: 'delivery' }
      );
    });

    it('should lowercase status in routing key', async () => {
      const delivery = { id: 'delivery-123', status: 'PICKED_UP' };

      await publisher.publishOrderStatusChanged(delivery);

      expect(rabbitmq.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.ORDERS,
        'order.picked_up',
        'delivery.status.changed',
        expect.any(Object),
        expect.any(Object)
      );
    });
  });

  describe('publishDriverAssigned', () => {
    it('should publish driver assigned event', async () => {
      await publisher.publishDriverAssigned('delivery-123', 'driver-123', { extra: 'data' });

      expect(rabbitmq.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.ORDERS,
        OrdersRoutingKeys.DRIVER_ASSIGNED,
        'delivery.driver.assigned',
        { deliveryId: 'delivery-123', driverId: 'driver-123', extra: 'data' },
        { aggregateId: 'delivery-123', aggregateType: 'delivery' }
      );
    });

    it('should work without extra data', async () => {
      await publisher.publishDriverAssigned('delivery-123', 'driver-123');

      expect(rabbitmq.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.ORDERS,
        OrdersRoutingKeys.DRIVER_ASSIGNED,
        'delivery.driver.assigned',
        { deliveryId: 'delivery-123', driverId: 'driver-123' },
        { aggregateId: 'delivery-123', aggregateType: 'delivery' }
      );
    });
  });

  describe('publishOrderCompleted', () => {
    it('should publish order completed event', async () => {
      const delivery = {
        id: 'delivery-123',
        customerId: 'customer-123',
        driverId: 'driver-123',
        status: 'COMPLETED',
        amount: '100.00',
        currency: 'USD',
        pickupAddress: { line1: '123 St' },
        dropoffAddress: { line1: '456 Ave' },
        paymentStatus: 'COMPLETED',
        updatedAt: new Date(),
      };

      await publisher.publishOrderCompleted(delivery);

      expect(rabbitmq.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.ORDERS,
        OrdersRoutingKeys.COMPLETED,
        'delivery.completed',
        expect.objectContaining({
          deliveryId: 'delivery-123',
          status: 'COMPLETED',
        }),
        { aggregateId: 'delivery-123', aggregateType: 'delivery' }
      );
    });
  });

  describe('publishOrderCancelled', () => {
    it('should publish order cancelled event with reason', async () => {
      const delivery = {
        id: 'delivery-123',
        customerId: 'customer-123',
        status: 'CANCELLED',
        amount: '100.00',
        currency: 'USD',
      };

      await publisher.publishOrderCancelled(delivery, 'Customer cancelled');

      expect(rabbitmq.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.ORDERS,
        OrdersRoutingKeys.CANCELLED,
        'delivery.cancelled',
        expect.objectContaining({
          deliveryId: 'delivery-123',
          reason: 'Customer cancelled',
        }),
        { aggregateId: 'delivery-123', aggregateType: 'delivery' }
      );
    });

    it('should use null reason when not provided', async () => {
      const delivery = { id: 'delivery-123', status: 'CANCELLED' };

      await publisher.publishOrderCancelled(delivery);

      expect(rabbitmq.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.ORDERS,
        OrdersRoutingKeys.CANCELLED,
        'delivery.cancelled',
        expect.objectContaining({ reason: null }),
        expect.any(Object)
      );
    });
  });

  describe('publishDispatchRequest', () => {
    it('should publish dispatch request', async () => {
      const delivery = {
        id: 'delivery-123',
        customerId: 'customer-123',
        pickupAddress: { line1: '123 St', city: 'SF' },
        dropoffAddress: { line1: '456 Ave', city: 'LA' },
        amount: '100.00',
        currency: 'USD',
      };

      await publisher.publishDispatchRequest(delivery);

      expect(rabbitmq.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.DISPATCH,
        DispatchRoutingKeys.REQUEST,
        'dispatch.request',
        {
          deliveryId: 'delivery-123',
          customerId: 'customer-123',
          pickup: { line1: '123 St', city: 'SF' },
          dropoff: { line1: '456 Ave', city: 'LA' },
          amount: '100.00',
          currency: 'USD',
        },
        { aggregateId: 'delivery-123', aggregateType: 'delivery' }
      );
    });
  });

  describe('toOrderPayload', () => {
    it('should transform delivery to order payload', async () => {
      const delivery = {
        id: 'delivery-123',
        customerId: 'customer-123',
        driverId: 'driver-123',
        status: 'PICKED_UP',
        amount: '100.00',
        currency: 'USD',
        pickupAddress: { line1: '123 St' },
        dropoffAddress: { line1: '456 Ave' },
        paymentStatus: 'AUTHORIZED',
        updatedAt: new Date('2024-01-01T10:00:00Z'),
      };

      // Access private method through any
      const payload = (publisher as any).toOrderPayload(delivery);

      expect(payload).toEqual({
        deliveryId: 'delivery-123',
        customerId: 'customer-123',
        driverId: 'driver-123',
        status: 'PICKED_UP',
        amount: '100.00',
        currency: 'USD',
        pickup: { line1: '123 St' },
        dropoff: { line1: '456 Ave' },
        paymentStatus: 'AUTHORIZED',
        updatedAt: '2024-01-01T10:00:00.000Z',
      });
    });

    it('should handle missing updatedAt', async () => {
      const delivery = { id: 'delivery-123', updatedAt: undefined };
      const payload = (publisher as any).toOrderPayload(delivery);
      expect(payload.updatedAt).toBeDefined();
    });

    it('should handle non-object delivery', async () => {
      const payload = (publisher as any).toOrderPayload('string-delivery');
      expect(payload).toEqual({ delivery: 'string-delivery' });
    });

    it('should handle null delivery', async () => {
      const payload = (publisher as any).toOrderPayload(null);
      expect(payload).toEqual({ delivery: null });
    });
  });
});