import {
  ServerMessageType,
  MessagePriority,
  MessagePriority as Priority,
} from '@delivery/common';
import { RealtimeNatsSubjects } from '@delivery/common';

export interface RealtimeEventEnvelope<T = unknown> {
  eventId: string;
  eventType: string;
  traceId?: string;
  timestamp: number;
  payload: T;
}

export interface ClientEvent {
  type: ServerMessageType;
  priority: MessagePriority;
  data: Record<string, unknown>;
}

export interface DeliveryCreatedPayload {
  deliveryId: string;
  customerId: string;
  status: string;
  createdAt: string;
}

export interface DriverAssignedPayload {
  deliveryId: string;
  driverId: string;
  estimatedArrival?: string;
}

export interface DriverAcceptedPayload {
  deliveryId: string;
  driverId: string;
  status: string;
  version?: number;
}

export interface DeliveryStatusPayload {
  deliveryId: string;
  status: string;
  version?: number;
}

export interface DeliveryCompletedPayload {
  deliveryId: string;
  completedAt?: string;
  status: string;
}

export interface DeliveryCancelledPayload {
  deliveryId: string;
  reason?: string;
  status: string;
}

export interface PaymentStatusPayload {
  paymentId?: string;
  deliveryId: string;
  status: string;
}

export interface DriverPresencePayload {
  driverId: string;
  status: 'ONLINE' | 'IDLE' | 'OFFLINE';
}

export type DomainEventPayload =
  | DeliveryCreatedPayload
  | DriverAssignedPayload
  | DriverAcceptedPayload
  | DeliveryStatusPayload
  | DeliveryCompletedPayload
  | DeliveryCancelledPayload
  | PaymentStatusPayload
  | DriverPresencePayload
  | Record<string, unknown>;

export const eventToNatsSubject: Partial<Record<string, RealtimeNatsSubjects>> =
  {
    'delivery.created': RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    'delivery.driver.assigned': RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED,
    'delivery.driver.accepted': RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    'delivery.picked_up': RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    'delivery.in_transit': RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    'delivery.completed': RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    'delivery.cancelled': RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    'payment.completed': RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    'payment.failed': RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
  };

export const eventPriorityByType: Partial<Record<string, Priority>> = {
  'delivery.driver.assigned': Priority.CRITICAL,
  'delivery.completed': Priority.CRITICAL,
  'delivery.cancelled': Priority.CRITICAL,
  'payment.completed': Priority.CRITICAL,
  'payment.failed': Priority.CRITICAL,
};
