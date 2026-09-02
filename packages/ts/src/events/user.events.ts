/**
 * User lifecycle events shared by services.
 */
export enum UserEventType {
  Created = 'user.created',
  Updated = 'user.updated',
  Deleted = 'user.deleted',
}

export interface UserCreatedPayload {
  userId: string;
  email: string;
  firstName: string;
  lastName: string;
  role: string;
  createdAt: string;
}

export interface UserUpdatedPayload {
  userId: string;
  email: string;
  firstName: string;
  lastName: string;
  role: string;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
  avatarMediaId?: string;
}

export interface UserDeletedPayload {
  userId: string;
  deletedAt: string;
}

export interface UserEventEnvelope<T = unknown> {
  eventId: string;
  eventType: UserEventType | string;
  traceId?: string;
  timestamp: number;
  payload: T;
}
