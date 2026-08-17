import { Injectable } from '@nestjs/common';
import {
  ClientMessageType,
  ServerMessageType,
  MessagePriority,
} from '@delivery/common';
import { RealtimeEventEnvelope, ClientEvent } from './event.types';

const PRIORITY_FALLBACK: Record<ServerMessageType, MessagePriority> = {
  [ServerMessageType.DELIVERY_LOCATION_UPDATED]: MessagePriority.HIGH_FREQUENCY_LOSSY,
  [ServerMessageType.DELIVERY_STATUS_UPDATED]: MessagePriority.NORMAL,
  [ServerMessageType.DRIVER_ASSIGNED]: MessagePriority.CRITICAL,
  [ServerMessageType.DELIVERY_COMPLETED]: MessagePriority.CRITICAL,
  [ServerMessageType.DELIVERY_CANCELLED]: MessagePriority.CRITICAL,
  [ServerMessageType.PAYMENT_STATUS_CHANGED]: MessagePriority.CRITICAL,
  [ServerMessageType.DRIVER_PRESENCE_UPDATED]: MessagePriority.NORMAL,
  [ServerMessageType.CONNECTED]: MessagePriority.NORMAL,
  [ServerMessageType.SUBSCRIBED]: MessagePriority.NORMAL,
  [ServerMessageType.UNSUBSCRIBED]: MessagePriority.NORMAL,
  [ServerMessageType.PONG]: MessagePriority.NORMAL,
  [ServerMessageType.ACK]: MessagePriority.NORMAL,
  [ServerMessageType.LOCATION_UPDATE_REJECTED]: MessagePriority.NORMAL,
  [ServerMessageType.NOTIFICATION_RECEIVED]: MessagePriority.NORMAL,
  [ServerMessageType.ERROR]: MessagePriority.CRITICAL,
};

/**
 * Factory-pattern mapper: converts internal domain events
 * (NATS / Kafka) into client-facing events. Internal infrastructure
 * fields (eventId, producer, traceId, ...) are NEVER exposed.
 */
@Injectable()
export class EventMapper {
  private readonly mappings: Record<
    string,
    (payload: Record<string, unknown>) => ClientEvent
  > = {
    'delivery.created': (payload) => ({
      type: ServerMessageType.DELIVERY_STATUS_UPDATED,
      priority: MessagePriority.NORMAL,
      data: {
        deliveryId: payload.deliveryId,
        status: payload.status ?? 'CREATED',
      },
    }),
    'delivery.driver.assigned': (payload) => ({
      type: ServerMessageType.DRIVER_ASSIGNED,
      priority: MessagePriority.CRITICAL,
      data: {
        deliveryId: payload.deliveryId,
        driverId: payload.driverId,
        estimatedArrival: payload.estimatedArrival,
      },
    }),
    'delivery.driver.accepted': (payload) => ({
      type: ServerMessageType.DELIVERY_STATUS_UPDATED,
      priority: MessagePriority.NORMAL,
      data: {
        deliveryId: payload.deliveryId,
        status: payload.status ?? 'ACCEPTED',
        version: payload.version,
      },
    }),
    'delivery.picked_up': (payload) => ({
      type: ServerMessageType.DELIVERY_STATUS_UPDATED,
      priority: MessagePriority.NORMAL,
      data: {
        deliveryId: payload.deliveryId,
        status: payload.status ?? 'PICKED_UP',
        version: payload.version,
      },
    }),
    'delivery.in_transit': (payload) => ({
      type: ServerMessageType.DELIVERY_STATUS_UPDATED,
      priority: MessagePriority.NORMAL,
      data: {
        deliveryId: payload.deliveryId,
        status: payload.status ?? 'IN_TRANSIT',
        version: payload.version,
      },
    }),
    'delivery.completed': (payload) => ({
      type: ServerMessageType.DELIVERY_COMPLETED,
      priority: MessagePriority.CRITICAL,
      data: {
        deliveryId: payload.deliveryId,
        completedAt: payload.completedAt,
        status: payload.status ?? 'COMPLETED',
      },
    }),
    'delivery.cancelled': (payload) => ({
      type: ServerMessageType.DELIVERY_CANCELLED,
      priority: MessagePriority.CRITICAL,
      data: {
        deliveryId: payload.deliveryId,
        reason: payload.reason,
        cancelledAt: payload.cancelledAt,
      },
    }),
    'payment.completed': (payload) => ({
      type: ServerMessageType.PAYMENT_STATUS_CHANGED,
      priority: MessagePriority.CRITICAL,
      data: {
        deliveryId: payload.deliveryId,
        status: payload.status ?? 'COMPLETED',
      },
    }),
    'payment.failed': (payload) => ({
      type: ServerMessageType.PAYMENT_STATUS_CHANGED,
      priority: MessagePriority.CRITICAL,
      data: {
        deliveryId: payload.deliveryId,
        status: payload.status ?? 'FAILED',
      },
    }),
  };

  /** Maps a delivery/payment domain event to a client event. */
  toClientEvent(event: RealtimeEventEnvelope): ClientEvent {
    const mapper = this.mappings[event.eventType];
    if (!mapper) {
      throw new Error(`Unsupported internal event type: ${event.eventType}`);
    }
    return mapper(event.payload as Record<string, unknown>);
  }
}

export { ClientMessageType };