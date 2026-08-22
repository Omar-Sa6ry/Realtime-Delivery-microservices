export enum DriverEventType {
  Created = 'driver.created',
  Updated = 'driver.updated',
  Deleted = 'driver.deleted',
}

export interface DriverGeoPoint {
  lat: number;
  lon: number;
}

export interface DriverCreatedPayload {
  driverId: string;
  name: string;
  status: 'AVAILABLE' | 'BUSY' | 'OFFLINE';
  vehicleType: 'CAR' | 'MOTORCYCLE' | 'TRUCK' | 'BICYCLE';
  rating: number;
  location?: DriverGeoPoint;
  updatedAt: string; // ISO 8601
  sourceVersion: number;
}

export interface DriverUpdatedPayload {
  driverId: string;
  name?: string;
  status?: 'AVAILABLE' | 'BUSY' | 'OFFLINE';
  vehicleType?: 'CAR' | 'MOTORCYCLE' | 'TRUCK' | 'BICYCLE';
  rating?: number;
  location?: DriverGeoPoint;
  updatedAt: string;
  sourceVersion: number;
}

export interface DriverDeletedPayload {
  driverId: string;
  deletedAt: string;
}

// Generic Kafka event envelope for driver events.
export interface DriverEventEnvelope<T = unknown> {
  eventId: string;
  eventType: DriverEventType | string;
  traceId?: string;
  timestamp: number; // unix milliseconds
  payload: T;
}
