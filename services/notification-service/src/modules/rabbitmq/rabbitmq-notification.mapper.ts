import { Injectable } from '@nestjs/common';
import { RabbitMQEventEnvelope } from '@delivery-micro/shard';

export interface RabbitMQNotificationView {
  type: string;
  title: string;
  body: string;
  data: Record<string, unknown>;
}

@Injectable()
export class RabbitMQNotificationMapper {
  toNotification(envelope: RabbitMQEventEnvelope): RabbitMQNotificationView {
    return toNotificationView(envelope);
  }
}

export function toNotificationView(
  envelope: RabbitMQEventEnvelope,
): RabbitMQNotificationView {
  const payload =
    envelope.payload && typeof envelope.payload === 'object'
      ? (envelope.payload as Record<string, unknown>)
      : {};
  const eventType = envelope.eventType || 'unknown.event';

  const deliveryId =
    payload.deliveryId ?? payload.delivery_id ?? envelope.aggregateId ?? '';
  const titles: Record<string, string> = {
    'delivery.created': 'Order Received',
    'delivery.status.changed': 'Order Status Updated',
    'delivery.driver.assigned': 'Driver Assigned',
    'delivery.completed': 'Order Delivered!',
    'delivery.cancelled': 'Order Cancelled',
    'dispatch.request': 'New Dispatch Request',
    'payment.authorized': 'Payment Authorized',
    'payment.failed': 'Payment Failed',
    'payment.refunded': 'Payment Refunded',
  };

  const title = titles[eventType] ?? `Update: ${eventType}`;
  const body =
    typeof payload.body === 'string' && payload.body
      ? payload.body
      : typeof payload.message === 'string' && payload.message
        ? payload.message
        : `Event ${eventType} for delivery ${deliveryId || 'N/A'}.`;

  return {
    type: eventType,
    title,
    body,
    data: {
      ...payload,
      deliveryId,
      eventId: envelope.eventId,
      traceId: envelope.traceId ?? null,
    },
  };
}
