export enum DeliveryEventType {
  Created        = 'delivery.created',
  DriverAssigned = 'delivery.driver.assigned',
  DriverAccepted = 'delivery.driver.accepted',
  PickedUp       = 'delivery.picked_up',
  InTransit      = 'delivery.in_transit',
  Completed      = 'delivery.completed',
  Cancelled      = 'delivery.cancelled',
  Deleted        = 'delivery.deleted',
}

export interface DeliveryLocation {
  lat: number;
  lon: number;
}

export interface DeliveryAddress {
  city: string;
  country: string;
  location: DeliveryLocation;
}

export interface DeliveryCreatedPayload {
  deliveryId: string;
  customerId: string;
  driverId?: string;
  status: string;
  pickup: DeliveryAddress;
  dropoff: DeliveryAddress;
  createdAt: string; // ISO 8601
  updatedAt: string;
  sourceVersion: number;
}

export interface DeliveryUpdatedPayload {
  deliveryId: string;
  customerId: string;
  driverId?: string;
  status: string;
  updatedAt: string;
  sourceVersion: number;
}

export interface DeliveryDriverAssignedPayload {
  deliveryId: string;
  driverId: string;
  assignedAt: string;
  sourceVersion: number;
}

export interface DeliveryDeletedPayload {
  deliveryId: string;
  deletedAt: string;
}

// Generic Kafka event envelope for delivery events.
export interface DeliveryEventEnvelope<T = unknown> {
  eventId: string;
  eventType: DeliveryEventType | string;
  traceId?: string;
  timestamp: number; // unix milliseconds
  payload: T;
}
