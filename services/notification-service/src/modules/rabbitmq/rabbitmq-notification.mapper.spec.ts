import {
  RabbitMQNotificationMapper,
  toNotificationView,
} from './rabbitmq-notification.mapper';
import { RabbitMQEventEnvelope } from '@delivery-micro/shard';

const envelope = (
  overrides: Partial<RabbitMQEventEnvelope> = {},
): RabbitMQEventEnvelope =>
  ({
    eventId: 'e-1',
    eventType: 'delivery.created',
    eventVersion: 1,
    occurredAt: new Date().toISOString(),
    timestamp: Date.now(),
    aggregateType: 'delivery',
    aggregateId: 'del-1',
    producer: 'delivery-service',
    payload: { deliveryId: 'del-1' },
    ...overrides,
  }) as RabbitMQEventEnvelope;

describe('toNotificationView', () => {
  it('maps known delivery events to friendly titles', () => {
    const cases: [string, string][] = [
      ['delivery.created', 'Order Received'],
      ['delivery.status.changed', 'Order Status Updated'],
      ['delivery.driver.assigned', 'Driver Assigned'],
      ['delivery.completed', 'Order Delivered!'],
      ['delivery.cancelled', 'Order Cancelled'],
      ['dispatch.request', 'New Dispatch Request'],
      ['payment.authorized', 'Payment Authorized'],
      ['payment.failed', 'Payment Failed'],
      ['payment.refunded', 'Payment Refunded'],
    ];

    for (const [eventType, title] of cases) {
      const view = toNotificationView(envelope({ eventType }));
      expect(view.title).toBe(title);
      expect(view.type).toBe(eventType);
    }
  });

  it('falls back to a generic title for unknown event types', () => {
    const view = toNotificationView(envelope({ eventType: 'weird.event' }));
    expect(view.title).toBe('Update: weird.event');
  });

  it('defaults eventType to unknown.event when missing', () => {
    const view = toNotificationView(envelope({ eventType: '' }));
    expect(view.type).toBe('unknown.event');
  });

  it('prefers payload.body, then payload.message, then a generated body', () => {
    const withBody = toNotificationView(
      envelope({ payload: { body: 'explicit body' } }),
    );
    expect(withBody.body).toBe('explicit body');

    const withMessage = toNotificationView(
      envelope({ payload: { message: 'explicit message' } }),
    );
    expect(withMessage.body).toBe('explicit message');

    const generated = toNotificationView(
      envelope({
        eventType: 'delivery.created',
        aggregateId: 'del-9',
        payload: {},
      }),
    );
    expect(generated.body).toBe('Event delivery.created for delivery del-9.');
  });

  it('uses N/A when no delivery id can be resolved', () => {
    const view = toNotificationView(envelope({ aggregateId: '', payload: {} }));
    expect(view.body).toContain('N/A');
  });

  it('reads delivery_id (snake_case) from the payload', () => {
    const view = toNotificationView(
      envelope({ payload: { delivery_id: 'del-snake' }, aggregateId: '' }),
    );
    expect(view.body).toContain('del-snake');
  });

  it('treats non-object payloads as empty and carries envelope metadata', () => {
    const view = toNotificationView(
      envelope({ payload: 'not-an-object' as any, traceId: 'trace-1' }),
    );

    expect(view.data).toEqual(
      expect.objectContaining({
        eventId: 'e-1',
        traceId: 'trace-1',
        deliveryId: 'del-1',
      }),
    );
  });

  it('defaults traceId to null when absent', () => {
    const view = toNotificationView(envelope());
    expect(view.data.traceId).toBeNull();
  });
});

describe('RabbitMQNotificationMapper', () => {
  it('delegates to the pure mapper function', () => {
    const mapper = new RabbitMQNotificationMapper();
    const env = envelope();
    expect(mapper.toNotification(env)).toEqual(toNotificationView(env));
  });
});
