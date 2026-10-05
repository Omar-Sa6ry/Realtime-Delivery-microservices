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
  NoDriverAvailable = 'driver.no_driver_available',
  SearchRetry = 'driver.search.retry',
}

export interface DriverGeoPoint {
  lat: number;
  lon: number;
}

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

export interface DriverCreatedPayload extends DriverBasePayload {}
export interface DriverUpdatedPayload extends DriverBasePayload {}

export interface DriverDeletedPayload {
  driverId: string;
  deletedAt: string; // ISO 8601
}

export interface DriverActivatedPayload {
  driverId: string;
}

export interface DriverDeactivatedPayload {
  driverId: string;
}

export interface DriverAvailablePayload {
  driverId: string;
}

export interface DriverUnavailablePayload {
  driverId: string;
}

export interface DriverGeoPoint {
  lat: number;
  lon: number;
}

export interface DriverAssignmentOfferedPayload {
  assignmentId: string;
  driverId: string;
  deliveryId: string;
  expiresAt: string; // ISO 8601
  radiusKm: number;
  distanceMeters?: number;
  pickupLatitude?: number;
  pickupLongitude?: number;
  pickupAddress?: any;
  dropoffAddress?: any;
  amount?: string;
  currency?: string;
}

export interface DriverAssignmentAcceptedPayload {
  assignmentId: string;
  deliveryId?: string;
  driverId: string;
  acceptedAt: string; // ISO 8601
}

export interface DriverAssignmentRejectedPayload {
  assignmentId: string;
  deliveryId?: string;
  driverId: string;
  reason: string;
  rejectedAt?: string; // ISO 8601
}

export interface DriverAssignmentExpiredPayload {
  assignmentId: string;
  deliveryId?: string;
  driverId?: string;
  expiredAt: string; // ISO 8601
}

export interface DriverAssignmentReleasedPayload {
  assignmentId: string;
  driverId: string;
  releasedAt: string; // ISO 8601
}

export interface DriverAssignmentCompletedPayload {
  assignmentId: string;
  driverId: string;
  completedAt: string; // ISO 8601
}

export interface DriverNoDriverAvailablePayload {
  deliveryId: string;
  customerId: string;
  attemptNumber: number;
  reason: string;
  triedAt: string;
}

export interface DriverSearchRetryPayload {
  deliveryId: string;
  customerId: string;
  attemptNumber: number;
  nextRetryAt?: string;
  retryInterval: number;
}


export interface DriverEventEnvelope<T = unknown> {
  eventId: string;
  eventType: DriverEventType | string;
  traceId?: string;
  timestamp: number; // unix milliseconds
  payload: T;
}