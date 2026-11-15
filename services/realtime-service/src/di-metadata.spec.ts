/**
 * Coverage for compiler-generated `design:paramtypes` metadata ternaries.
 *
 * ts-jest runs with `isolatedModules`, so every constructor parameter typed as
 * an imported class is emitted as:
 *
 *   typeof (_a = typeof X !== "undefined" && X) === "function" ? _a : Object
 *
 * The `: Object` branch is dead when X is always a constructor (the normal
 * case). To execute the alternate branch, each target class is loaded in a
 * registry where its dependency modules export non-constructor values, while
 * the target itself is loaded through `jest.requireActual`.
 */
jest.mock('@delivery-micro/shard', () => ({
  ...jest.requireActual('@delivery-micro/shard'),
  HealthService: 'HealthService',
  MetricsService: 'MetricsService',
  KafkaService: 'KafkaService',
  WsAuthGuard: 'WsAuthGuard',
  WsGuardChain: 'WsGuardChain',
  RabbitMQService: 'RabbitMQService',
}));
jest.mock('@bts-soft/cache', () => ({
  ...jest.requireActual('@bts-soft/cache'),
  RedisService: 'RedisService',
}));
jest.mock('@nestjs/config', () => ({
  ...jest.requireActual('@nestjs/config'),
  ConfigService: 'ConfigService',
}));
jest.mock('nestjs-i18n', () => ({
  ...jest.requireActual('nestjs-i18n'),
  I18nService: 'I18nService',
}));

jest.mock('./common/metrics/realtime-metrics.service', () => ({
  ...jest.requireActual('./common/metrics/realtime-metrics.service'),
  RealtimeMetricsService: 'RealtimeMetricsService',
}));
jest.mock('./modules/gateway/connection/connection.registry', () => ({
  ...jest.requireActual('./modules/gateway/connection/connection.registry'),
  ConnectionRegistry: 'ConnectionRegistry',
}));
jest.mock('./modules/gateway/connection/connection-state.store', () => ({
  ...jest.requireActual('./modules/gateway/connection/connection-state.store'),
  ConnectionStateStore: 'ConnectionStateStore',
}));
jest.mock('./modules/gateway/connection/connection.service', () => ({
  ...jest.requireActual('./modules/gateway/connection/connection.service'),
  ConnectionService: 'ConnectionService',
}));
jest.mock('./modules/gateway/connection/socket-writer.service', () => ({
  ...jest.requireActual('./modules/gateway/connection/socket-writer.service'),
  SocketWriter: 'SocketWriter',
}));
jest.mock('./modules/features/presence/presence.store', () => ({
  ...jest.requireActual('./modules/features/presence/presence.store'),
  PresenceStore: 'PresenceStore',
}));
jest.mock('./modules/features/presence/presence.service', () => ({
  ...jest.requireActual('./modules/features/presence/presence.service'),
  PresenceService: 'PresenceService',
}));
jest.mock('./modules/features/subscription/subscription.store', () => ({
  ...jest.requireActual('./modules/features/subscription/subscription.store'),
  SubscriptionStore: 'SubscriptionStore',
}));
jest.mock('./modules/features/subscription/subscription.service', () => ({
  ...jest.requireActual('./modules/features/subscription/subscription.service'),
  SubscriptionService: 'SubscriptionService',
}));
jest.mock(
  './modules/gateway/authorization/realtime-authorization.service',
  () => ({
    ...jest.requireActual(
      './modules/gateway/authorization/realtime-authorization.service',
    ),
    RealtimeAuthorizationService: 'RealtimeAuthorizationService',
  }),
);
jest.mock('./modules/gateway/authorization/policies/delivery.policy', () => ({
  ...jest.requireActual(
    './modules/gateway/authorization/policies/delivery.policy',
  ),
  DeliveryPolicy: 'DeliveryPolicy',
}));
jest.mock('./modules/gateway/authorization/policies/driver.policy', () => ({
  ...jest.requireActual(
    './modules/gateway/authorization/policies/driver.policy',
  ),
  DriverPolicy: 'DriverPolicy',
}));
jest.mock('./modules/infrastructure/grpc/grpc.client', () => ({
  ...jest.requireActual('./modules/infrastructure/grpc/grpc.client'),
  GrpcClient: 'GrpcClient',
}));
jest.mock('./modules/features/events/idempotency.store', () => ({
  ...jest.requireActual('./modules/features/events/idempotency.store'),
  IdempotencyStore: 'IdempotencyStore',
}));
jest.mock('./modules/features/events/event-deduplicator', () => ({
  ...jest.requireActual('./modules/features/events/event-deduplicator'),
  EventDeduplicator: 'EventDeduplicator',
}));
jest.mock('./modules/features/events/event.mapper', () => ({
  ...jest.requireActual('./modules/features/events/event.mapper'),
  EventMapper: 'EventMapper',
}));
jest.mock('./modules/infrastructure/nats/nats.service', () => ({
  ...jest.requireActual('./modules/infrastructure/nats/nats.service'),
  RealtimeNatsService: 'RealtimeNatsService',
}));
jest.mock('./modules/infrastructure/nats/nats.publisher', () => ({
  ...jest.requireActual('./modules/infrastructure/nats/nats.publisher'),
  NatsPublisher: 'NatsPublisher',
}));
jest.mock('./modules/features/location/location.validator', () => ({
  ...jest.requireActual('./modules/features/location/location.validator'),
  LocationValidator: 'LocationValidator',
}));
jest.mock('./modules/features/location/location.throttler', () => ({
  ...jest.requireActual('./modules/features/location/location.throttler'),
  LocationThrottler: 'LocationThrottler',
}));
jest.mock('./modules/features/location/location.service', () => ({
  ...jest.requireActual('./modules/features/location/location.service'),
  LocationService: 'LocationService',
}));
jest.mock('./modules/features/command/command.service', () => ({
  ...jest.requireActual('./modules/features/command/command.service'),
  CommandService: 'CommandService',
}));
jest.mock('./modules/gateway/heartbeat/heartbeat.service', () => ({
  ...jest.requireActual('./modules/gateway/heartbeat/heartbeat.service'),
  HeartbeatService: 'HeartbeatService',
}));
jest.mock('./modules/infrastructure/kafka/kafka.consumer', () => ({
  ...jest.requireActual('./modules/infrastructure/kafka/kafka.consumer'),
  KafkaConsumer: 'KafkaConsumer',
}));

const TARGETS = [
  './app.resolver',
  './common/health.controller',
  './modules/features/command/command.service',
  './modules/features/events/event-deduplicator',
  './modules/features/events/idempotency.store',
  './modules/features/location/location.service',
  './modules/features/location/location.throttler',
  './modules/features/presence/presence.service',
  './modules/features/presence/presence.store',
  './modules/features/subscription/subscription.service',
  './modules/features/subscription/subscription.store',
  './modules/gateway/authorization/realtime-authorization.service',
  './modules/gateway/authorization/policies/delivery.policy',
  './modules/gateway/authorization/policies/driver.policy',
  './modules/gateway/connection/connection-state.store',
  './modules/gateway/connection/connection.service',
  './modules/gateway/connection/socket-writer.service',
  './modules/gateway/heartbeat/heartbeat.service',
  './modules/gateway/websocket/websocket-rate-limiter.service',
  './modules/gateway/websocket/websocket.gateway',
  './modules/infrastructure/grpc/grpc.client',
  './modules/infrastructure/kafka/kafka.consumer',
  './modules/infrastructure/kafka/handlers/base-kafka-event.handler',
  './modules/infrastructure/kafka/handlers/delivery-cancelled.handler',
  './modules/infrastructure/kafka/handlers/delivery-completed.handler',
  './modules/infrastructure/kafka/handlers/delivery-created.handler',
  './modules/infrastructure/kafka/handlers/delivery-in-transit.handler',
  './modules/infrastructure/kafka/handlers/delivery-picked-up.handler',
  './modules/infrastructure/kafka/handlers/driver-accepted.handler',
  './modules/infrastructure/kafka/handlers/driver-assigned.handler',
  './modules/infrastructure/kafka/handlers/driver-assignment-expired.handler',
  './modules/infrastructure/kafka/handlers/driver-assignment-offered.handler',
  './modules/infrastructure/kafka/handlers/driver-assignment-rejected.handler',
  './modules/infrastructure/kafka/handlers/driver-no-driver-available.handler',
  './modules/infrastructure/kafka/handlers/payment-completed.handler',
  './modules/infrastructure/kafka/handlers/payment-failed.handler',
  './modules/infrastructure/nats/nats.publisher',
  './modules/infrastructure/nats/nats.service',
  './modules/infrastructure/nats/nats.subscriber',
  './modules/infrastructure/rabbitmq/broadcast.consumer',
];

describe('DI metadata ternaries (design:paramtypes)', () => {
  it.each(TARGETS)('loads %s with non-constructor dependency types', (mod) => {
    expect(() => jest.requireActual(mod)).not.toThrow();
  });
});
