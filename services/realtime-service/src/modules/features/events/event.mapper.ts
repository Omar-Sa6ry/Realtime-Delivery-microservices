import { Injectable } from '@nestjs/common';
import {
  ClientMessageType,
  ServerMessageType,
  MessagePriority,
} from '@delivery-micro/shard';
import { RealtimeEventEnvelope, ClientEvent } from './event.types';

@Injectable()
export class EventMapper {
  private readonly mappings: Record<
    string,
    (payload: Record<string, unknown>) => ClientEvent
  > = {
    'driver.assignment.offered': (payload) => ({
      type: ServerMessageType.ASSIGNMENT_OFFERED,
      priority: MessagePriority.CRITICAL,
      data: {
        assignmentId: payload.assignmentId,
        deliveryId: payload.deliveryId,
        driverId: payload.driverId,
        expiresAt: payload.expiresAt,
        distanceMeters: payload.distanceMeters,
        pickupAddress: payload.pickupAddress,
        dropoffAddress: payload.dropoffAddress,
        amount: payload.amount,
        currency: payload.currency,
      },
    }),
    'driver.assignment.rejected': (payload) => ({
      type: ServerMessageType.DRIVER_SEARCH_RETRY,
      priority: MessagePriority.NORMAL,
      data: {
        assignmentId: payload.assignmentId,
        deliveryId: payload.deliveryId,
        driverId: payload.driverId,
        reason: payload.reason,
      },
    }),
    'driver.assignment.expired': (payload) => ({
      type: ServerMessageType.DRIVER_SEARCH_RETRY,
      priority: MessagePriority.NORMAL,
      data: {
        assignmentId: payload.assignmentId,
        deliveryId: payload.deliveryId,
        driverId: payload.driverId,
        reason: 'OFFER_EXPIRED',
      },
    }),
    'driver.no_driver_available': (payload) => ({
      type: ServerMessageType.DRIVER_SEARCH_RETRY,
      priority: MessagePriority.NORMAL,
      data: {
        deliveryId: payload.deliveryId,
        customerId: payload.customerId,
        attemptNumber: payload.attemptNumber,
        reason: payload.reason,
        status: 'SEARCHING_RETRY',
      },
    }),
    'driver.search.retry': (payload) => ({
      type: ServerMessageType.DRIVER_SEARCH_RETRY,
      priority: MessagePriority.NORMAL,
      data: {
        deliveryId: payload.deliveryId,
        customerId: payload.customerId,
        attemptNumber: payload.attemptNumber,
        retryInterval: payload.retryInterval,
        nextRetryAt: payload.nextRetryAt,
        status: 'SEARCHING_RETRY',
      },
    }),
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
