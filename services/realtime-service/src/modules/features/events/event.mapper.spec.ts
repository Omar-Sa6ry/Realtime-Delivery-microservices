import { EventMapper } from './event.mapper';
import {
  MessagePriority,
  ServerMessageType,
  RealtimeNatsSubjects,
} from '@delivery-micro/shard';
import {
  eventToNatsSubject,
  eventPriorityByType,
  RealtimeEventEnvelope,
} from './event.types';

describe('EventMapper', () => {
  let mapper: EventMapper;

  const envelope = (eventType: string, payload: Record<string, unknown> = {}) =>
    ({
      eventId: 'evt-1',
      eventType,
      timestamp: Date.now(),
      payload,
    }) as RealtimeEventEnvelope;

  beforeEach(() => {
    mapper = new EventMapper();
  });

  it('maps driver.assignment.offered to ASSIGNMENT_OFFERED (CRITICAL)', () => {
    const event = mapper.toClientEvent(
      envelope('driver.assignment.offered', {
        assignmentId: 'a1',
        deliveryId: 'd1',
        driverId: 'drv1',
        expiresAt: '2026-01-01',
        distanceMeters: 1200,
        pickupAddress: { city: 'Cairo' },
        dropoffAddress: { city: 'Giza' },
        amount: '50',
        currency: 'EGP',
      }),
    );
    expect(event.type).toBe(ServerMessageType.ASSIGNMENT_OFFERED);
    expect(event.priority).toBe(MessagePriority.CRITICAL);
    expect(event.data).toMatchObject({
      assignmentId: 'a1',
      deliveryId: 'd1',
      driverId: 'drv1',
      amount: '50',
      currency: 'EGP',
    });
  });

  it('maps driver.assignment.rejected to DRIVER_SEARCH_RETRY', () => {
    const event = mapper.toClientEvent(
      envelope('driver.assignment.rejected', {
        assignmentId: 'a1',
        deliveryId: 'd1',
        driverId: 'drv1',
        reason: 'too_far',
      }),
    );
    expect(event.type).toBe(ServerMessageType.DRIVER_SEARCH_RETRY);
    expect(event.priority).toBe(MessagePriority.NORMAL);
    expect(event.data.reason).toBe('too_far');
  });

  it('maps driver.assignment.expired with OFFER_EXPIRED reason', () => {
    const event = mapper.toClientEvent(
      envelope('driver.assignment.expired', {
        assignmentId: 'a1',
        deliveryId: 'd1',
        driverId: 'drv1',
      }),
    );
    expect(event.type).toBe(ServerMessageType.DRIVER_SEARCH_RETRY);
    expect(event.data.reason).toBe('OFFER_EXPIRED');
  });

  it('maps driver.no_driver_available to DRIVER_SEARCH_RETRY', () => {
    const event = mapper.toClientEvent(
      envelope('driver.no_driver_available', {
        deliveryId: 'd1',
        customerId: 'c1',
        attemptNumber: 2,
        reason: 'none_found',
      }),
    );
    expect(event.type).toBe(ServerMessageType.DRIVER_SEARCH_RETRY);
    expect(event.data).toMatchObject({
      attemptNumber: 2,
      status: 'SEARCHING_RETRY',
    });
  });

  it('maps driver.search.retry to DRIVER_SEARCH_RETRY with retry metadata', () => {
    const event = mapper.toClientEvent(
      envelope('driver.search.retry', {
        deliveryId: 'd1',
        customerId: 'c1',
        attemptNumber: 3,
        retryInterval: 30,
        nextRetryAt: '2026-01-01T00:01:00Z',
      }),
    );
    expect(event.type).toBe(ServerMessageType.DRIVER_SEARCH_RETRY);
    expect(event.data.retryInterval).toBe(30);
  });

  it('maps delivery.created with default CREATED status', () => {
    const event = mapper.toClientEvent(
      envelope('delivery.created', { deliveryId: 'd1' }),
    );
    expect(event.type).toBe(ServerMessageType.DELIVERY_STATUS_UPDATED);
    expect(event.priority).toBe(MessagePriority.NORMAL);
    expect(event.data.status).toBe('CREATED');
  });

  it('maps delivery.driver.assigned to DRIVER_ASSIGNED (CRITICAL)', () => {
    const event = mapper.toClientEvent(
      envelope('delivery.driver.assigned', {
        deliveryId: 'd1',
        driverId: 'drv1',
        estimatedArrival: '10min',
      }),
    );
    expect(event.type).toBe(ServerMessageType.DRIVER_ASSIGNED);
    expect(event.priority).toBe(MessagePriority.CRITICAL);
    expect(event.data).toMatchObject({
      deliveryId: 'd1',
      driverId: 'drv1',
      estimatedArrival: '10min',
    });
  });

  it.each([
    ['delivery.driver.accepted', 'ACCEPTED'],
    ['delivery.picked_up', 'PICKED_UP'],
    ['delivery.in_transit', 'IN_TRANSIT'],
  ])(
    'maps %s with default status %s and version passthrough',
    (type, status) => {
      const event = mapper.toClientEvent(
        envelope(type, { deliveryId: 'd1', version: 7 }),
      );
      expect(event.type).toBe(ServerMessageType.DELIVERY_STATUS_UPDATED);
      expect(event.data.status).toBe(status);
      expect(event.data.version).toBe(7);
    },
  );

  it('maps delivery.completed to DELIVERY_COMPLETED (CRITICAL)', () => {
    const event = mapper.toClientEvent(
      envelope('delivery.completed', {
        deliveryId: 'd1',
        completedAt: '2026-01-01T00:00:00Z',
      }),
    );
    expect(event.type).toBe(ServerMessageType.DELIVERY_COMPLETED);
    expect(event.priority).toBe(MessagePriority.CRITICAL);
    expect(event.data.status).toBe('COMPLETED');
  });

  it('maps delivery.cancelled to DELIVERY_CANCELLED (CRITICAL)', () => {
    const event = mapper.toClientEvent(
      envelope('delivery.cancelled', {
        deliveryId: 'd1',
        reason: 'customer_request',
        cancelledAt: '2026-01-01T00:00:00Z',
      }),
    );
    expect(event.type).toBe(ServerMessageType.DELIVERY_CANCELLED);
    expect(event.priority).toBe(MessagePriority.CRITICAL);
    expect(event.data.reason).toBe('customer_request');
  });

  it('maps payment.completed to PAYMENT_STATUS_CHANGED', () => {
    const event = mapper.toClientEvent(
      envelope('payment.completed', { deliveryId: 'd1' }),
    );
    expect(event.type).toBe(ServerMessageType.PAYMENT_STATUS_CHANGED);
    expect(event.data.status).toBe('COMPLETED');
  });

  it('maps payment.failed to PAYMENT_STATUS_CHANGED with FAILED status', () => {
    const event = mapper.toClientEvent(
      envelope('payment.failed', { deliveryId: 'd1' }),
    );
    expect(event.type).toBe(ServerMessageType.PAYMENT_STATUS_CHANGED);
    expect(event.data.status).toBe('FAILED');
  });

  it('throws on unsupported event types', () => {
    expect(() => mapper.toClientEvent(envelope('unknown.event'))).toThrow(
      'Unsupported internal event type: unknown.event',
    );
  });
});

describe('event.types mappings', () => {
  it('maps every known event type to a NATS subject', () => {
    expect(eventToNatsSubject['delivery.created']).toBe(
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    );
    expect(eventToNatsSubject['delivery.driver.assigned']).toBe(
      RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED,
    );
    expect(eventToNatsSubject['driver.assignment.offered']).toBe(
      RealtimeNatsSubjects.DRIVER_ASSIGNMENT_OFFERED,
    );
    expect(eventToNatsSubject['payment.failed']).toBe(
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    );
  });

  it('marks critical events in eventPriorityByType', () => {
    expect(eventPriorityByType['delivery.completed']).toBe(
      MessagePriority.CRITICAL,
    );
    expect(eventPriorityByType['payment.completed']).toBe(
      MessagePriority.CRITICAL,
    );
  });
});
