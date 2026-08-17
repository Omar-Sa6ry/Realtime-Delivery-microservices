// ===== Client -> Server payloads =====

export interface PingPayload {
  timestamp?: string;
}

export interface DeliverySubscriptionPayload {
  deliveryId: string;
  requestId?: string;
}

export interface LocationUpdatePayload {
  deliveryId: string;
  lat: number;
  lng: number;
  accuracy?: number;
  speed?: number;
  heading?: number;
  timestamp: string;
}

export interface AssignmentCommandPayload {
  deliveryId: string;
  commandId: string;
  requestId?: string;
  reason?: string;
}

export interface RejectAssignmentPayload extends AssignmentCommandPayload {
  reason?: string;
}

export interface AckPayload {
  messageId: string;
}

// ===== Server -> Client payloads =====

export interface ConnectedPayload {
  socketId: string;
  nodeId: string;
  timestamp: string;
}

export interface SubscribedPayload {
  deliveryId: string;
  requestId?: string;
}

export interface PongPayload {
  timestamp: string;
}

export interface DeliveryLocationUpdatedPayload {
  deliveryId: string;
  driverId?: string;
  lat: number;
  lng: number;
  accuracy?: number;
  speed?: number;
  heading?: number;
  timestamp: string;
}

export interface DeliveryStatusUpdatedPayload {
  deliveryId: string;
  status: string;
  version?: number;
}

export interface DriverAssignedPayload {
  deliveryId: string;
  driverId: string;
  estimatedArrival?: string;
}

export interface DeliveryCompletedPayload {
  deliveryId: string;
  completedAt: string;
}

export interface DeliveryCancelledPayload {
  deliveryId: string;
  reason?: string;
  cancelledAt: string;
}

export interface PaymentStatusChangedPayload {
  deliveryId: string;
  status: string;
}

export interface DriverPresenceUpdatedPayload {
  driverId: string;
  status: 'ONLINE' | 'IDLE' | 'OFFLINE';
}

export interface ErrorPayload {
  code: string;
  message: string;
  retryable: boolean;
  requestId?: string;
}