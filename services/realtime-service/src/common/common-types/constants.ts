import { WS_DEFAULTS } from '@delivery/common';

export const redisKeys = {
  connection: (socketId: string) => `ws:connection:${socketId}`,
  userSockets: (userId: string) => `ws:user:${userId}:sockets`,
  roleSockets: (role: string) => `ws:role:${role}:sockets`,
  nodeConnections: (nodeId: string) => `ws:node:${nodeId}:connections`,
  deliverySubscribers: (deliveryId: string) =>
    `ws:delivery:${deliveryId}:subscribers`,
  socketSubscriptions: (socketId: string) =>
    `ws:socket:${socketId}:subscriptions`,
  presence: (userId: string) => `ws:presence:${userId}`,
  rate: (userId: string, action: string) => `ws:rate:${userId}:${action}`,
  locationBucket: (driverId: string) => `ws:rate:${driverId}:location`,
  idempotency: (commandId: string) => `realtime:idempotency:${commandId}`,
  processed: (eventId: string) => `realtime:processed:${eventId}`,
  authzCache: (userId: string, deliveryId: string) =>
    `realtime:authz:${userId}:${deliveryId}`,
  driverLocation: (driverId: string) => `driver:location:${driverId}`,
  driverLocationsIndex: () => `drivers:locations`,
} as const;

// ===== TTLs (seconds) =====

export const TTL = {
  CONNECTION: WS_DEFAULTS.CONNECTION_TTL_SECONDS,
  HEARTBEAT: Math.floor(WS_DEFAULTS.HEARTBEAT_TIMEOUT_MS / 1000),
  PRESENCE: WS_DEFAULTS.PRESENCE_TTL_SECONDS,
  DRIVER_LOCATION: 120,
  IDEMPOTENCY: 24 * 60 * 60,
  PROCESSED: 24 * 60 * 60,
  AUTHZ_CACHE: 30,
} as const;

export const TIMINGS = {
  STALE_CONNECTION_MS: WS_DEFAULTS.HEARTBEAT_TIMEOUT_MS,
  SLOW_CONSUMER_THRESHOLD_MS: WS_DEFAULTS.SLOW_CONSUMER_THRESHOLD_MS,
  MAX_BACKLOG: WS_DEFAULTS.MAX_BACKLOG,
  LOCATION_COALESCE_MS: 250,
  GRPC_TIMEOUT_MS: 2000,
  CIRCUIT_RESET_MS: 10_000,
  CIRCUIT_FAILURE_THRESHOLD: 5,
  STALE_CRON_MS: 20_000,
  LOCATION_MAX_AGE_MS: 60_000,
} as const;

export const RATE_LIMITS = {
  CONNECT_PER_MINUTE: 10,
  COMMANDS_PER_MINUTE: 30,
  SUBSCRIBE_PER_MINUTE: 60,
  LOCATION_PER_SECOND: 5,
  LOCATION_BURST: 10,
} as const;

// ===== Server -> client default priority for each server message =====

import { MessagePriority, ServerMessageType } from '@delivery/common';

export const SERVER_MESSAGE_PRIORITY: Partial<
  Record<ServerMessageType, MessagePriority>
> = {
  [ServerMessageType.DELIVERY_LOCATION_UPDATED]:
    MessagePriority.HIGH_FREQUENCY_LOSSY,
  [ServerMessageType.DRIVER_ASSIGNED]: MessagePriority.CRITICAL,
  [ServerMessageType.DELIVERY_COMPLETED]: MessagePriority.CRITICAL,
  [ServerMessageType.DELIVERY_CANCELLED]: MessagePriority.CRITICAL,
  [ServerMessageType.PAYMENT_STATUS_CHANGED]: MessagePriority.CRITICAL,
  [ServerMessageType.DELIVERY_STATUS_UPDATED]: MessagePriority.NORMAL,
  [ServerMessageType.DRIVER_PRESENCE_UPDATED]: MessagePriority.NORMAL,
};
