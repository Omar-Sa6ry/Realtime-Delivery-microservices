export enum DriverEventType {
  Created = 'driver.created',
  Updated = 'driver.updated',
  Deleted = 'driver.deleted',
  Activated = 'driver.activated',
  Deactivated = 'driver.deactivated',
  Available = 'driver.available',
  Unavailable = 'driver.unavailable',
  AssignmentOffered = 'driver.assignment.offered',
  AssignmentAccepted = 'driver.assignment.accepted',
  AssignmentRejected = 'driver.assignment.rejected',
  AssignmentExpired = 'driver.assignment.expired',
  AssignmentReleased = 'driver.assignment.released',
  AssignmentCompleted = 'driver.assignment.completed',
}

export interface DriverGeoPoint {
  lat: number;
  lon: number;
}

// Base driver payload with common fields
export interface DriverBasePayload {
  driverId: string;
  userId?: string;
  status: 'AVAILABLE' | 'BUSY' | 'OFFLINE';
  vehicleType: 'CAR' | 'MOTORCYCLE' | 'TRUCK';
  rating: number;
  capabilities: string[];
  serviceArea: string;
  createdAt: string; // ISO 8601
  updatedAt: string; // ISO 8601
}

// Driver-specific payloads
export interface DriverCreatedPayload extends DriverBasePayload {}

// DriverUpdatedPayload is emitted when a driver profile is updated.
export interface DriverUpdatedPayload extends DriverBasePayload {}

// DriverDeletedPayload is emitted when a driver profile is permanently removed.
export interface DriverDeletedPayload {
  driverId: string;
  deletedAt: string; // ISO 8601
}

// DriverActivatedPayload is emitted when a driver is activated.
export interface DriverActivatedPayload {
  driverId: string;
}

// DriverDeactivatedPayload is emitted when a driver is deactivated.
export interface DriverDeactivatedPayload {
  driverId: string;
}

// DriverAvailablePayload is emitted when a driver becomes available.
export interface DriverAvailablePayload {
  driverId: string;
}

// DriverUnavailablePayload is emitted when a driver becomes unavailable.
export interface DriverUnavailablePayload {
  driverId: string;
}

// DriverGeoPoint holds a geographic coordinate for a driver's location.
export interface DriverGeoPoint {
  lat: number;
  lon: number;
}

// DriverAssignmentOfferedPayload is emitted when a driver is offered an assignment.
export interface DriverAssignmentOfferedPayload {
  assignmentId: string;
  driverId: string;
  deliveryId: string;
  expiresAt: string; // ISO 8601
  radiusKm: number;
  pickupLatitude?: number;
  pickupLongitude?: number;
}

// DriverAssignmentAcceptedPayload is emitted when a driver accepts an assignment.
export interface DriverAssignmentAcceptedPayload {
  assignmentId: string;
  driverId: string;
  acceptedAt: string; // ISO 8601
}

// DriverAssignmentRejectedPayload is emitted when a driver rejects an assignment.
export interface DriverAssignmentRejectedPayload {
  assignmentId: string;
  driverId: string;
  reason: string;
  rejectedAt: string; // ISO 8601
}

// DriverAssignmentExpiredPayload is emitted when a driver assignment offer expires.
export interface DriverAssignmentExpiredPayload {
  assignmentId: string;
  driverId: string;
  expiredAt: string; // ISO 8601
}

// DriverAssignmentReleasedPayload is emitted when a driver assignment is released.
export interface DriverAssignmentReleasedPayload {
  assignmentId: string;
  driverId: string;
  releasedAt: string; // ISO 8601
}

// DriverAssignmentCompletedPayload is emitted when a driver assignment is completed.
export interface DriverAssignmentCompletedPayload {
  assignmentId: string;
  driverId: string;
  completedAt: string; // ISO 8601
}

// Generic Kafka event envelope for driver events.
export interface DriverEventEnvelope<T = unknown> {
  eventId: string;
  eventType: DriverEventType | string;
  traceId?: string;
  timestamp: number; // unix milliseconds
  payload: T;
}