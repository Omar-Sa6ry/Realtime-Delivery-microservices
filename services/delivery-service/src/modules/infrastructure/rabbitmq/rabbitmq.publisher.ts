import { Injectable, Logger } from '@nestjs/common';
import {
  DispatchRoutingKeys,
  OrdersRoutingKeys,
  RabbitMQExchanges,
  RabbitMQService,
} from '@delivery/common';

@Injectable()
export class DeliveryRabbitMQPublisher {
  private readonly logger = new Logger(DeliveryRabbitMQPublisher.name);

  constructor(private readonly rabbitmq: RabbitMQService) {}

  async publishOrderCreated(delivery: any): Promise<void> {
    await this.rabbitmq.publish(
      RabbitMQExchanges.ORDERS,
      OrdersRoutingKeys.CREATED,
      'delivery.created',
      this.toOrderPayload(delivery),
      { aggregateId: delivery?.id, aggregateType: 'delivery' },
    );
  }

  async publishOrderStatusChanged(delivery: any): Promise<void> {
    const status = String(delivery?.status ?? 'UNKNOWN').toLowerCase();
    await this.rabbitmq.publish(
      RabbitMQExchanges.ORDERS,
      `order.${status}`,
      'delivery.status.changed',
      this.toOrderPayload(delivery),
      { aggregateId: delivery?.id, aggregateType: 'delivery' },
    );
  }

  async publishDriverAssigned(
    deliveryId: string,
    driverId: string,
    extra?: Record<string, unknown>,
  ): Promise<void> {
    await this.rabbitmq.publish(
      RabbitMQExchanges.ORDERS,
      OrdersRoutingKeys.DRIVER_ASSIGNED,
      'delivery.driver.assigned',
      { deliveryId, driverId, ...(extra ?? {}) },
      { aggregateId: deliveryId, aggregateType: 'delivery' },
    );
  }

  async publishOrderCompleted(delivery: any): Promise<void> {
    await this.rabbitmq.publish(
      RabbitMQExchanges.ORDERS,
      OrdersRoutingKeys.COMPLETED,
      'delivery.completed',
      this.toOrderPayload(delivery),
      { aggregateId: delivery?.id, aggregateType: 'delivery' },
    );
  }

  async publishOrderCancelled(delivery: any, reason?: string): Promise<void> {
    await this.rabbitmq.publish(
      RabbitMQExchanges.ORDERS,
      OrdersRoutingKeys.CANCELLED,
      'delivery.cancelled',
      { ...this.toOrderPayload(delivery), reason: reason ?? null },
      { aggregateId: delivery?.id, aggregateType: 'delivery' },
    );
  }

  async publishDispatchRequest(delivery: any): Promise<void> {
    await this.rabbitmq.publish(
      RabbitMQExchanges.DISPATCH,
      DispatchRoutingKeys.REQUEST,
      'dispatch.request',
      {
        deliveryId: delivery?.id,
        customerId: delivery?.customerId,
        pickup: delivery?.pickupAddress,
        dropoff: delivery?.dropoffAddress,
        amount: delivery?.amount,
        currency: delivery?.currency,
      },
      { aggregateId: delivery?.id, aggregateType: 'delivery' },
    );
  }

  private toOrderPayload(delivery: any): Record<string, unknown> {
    if (!delivery || typeof delivery !== 'object') return { delivery };
    return {
      deliveryId: delivery.id,
      customerId: delivery.customerId,
      driverId: delivery.driverId ?? null,
      status: delivery.status,
      amount: delivery.amount,
      currency: delivery.currency,
      pickup: delivery.pickupAddress,
      dropoff: delivery.dropoffAddress,
      paymentStatus: delivery.paymentStatus ?? null,
      updatedAt:
        delivery.updatedAt instanceof Date
          ? delivery.updatedAt.toISOString()
          : (delivery.updatedAt ?? new Date().toISOString()),
    };
  }
}
