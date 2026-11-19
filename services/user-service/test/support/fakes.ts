/**
 * Jest doubles for every outbound integration used by the user service.
 *
 * The e2e suite exercises the real resolvers, services, repositories and
 * GraphQL/REST plumbing; only the transports that would require external
 * brokers (Kafka, RabbitMQ, NATS/BullMQ, the media gRPC server and the
 * notification channels) are replaced. Each factory returns a fresh object so
 * that every `TestingModule` instance owns its own spies.
 */

export interface KafkaFake {
  emit: jest.Mock;
  consumer: jest.Mock;
  getClient: jest.Mock;
  onModuleDestroy: jest.Mock;
}

export interface NotificationFake {
  send: jest.Mock;
}

export interface OutboxFake {
  enqueueEvent: jest.Mock;
  reconcileOutbox: jest.Mock;
  onModuleInit: jest.Mock;
  onModuleDestroy: jest.Mock;
}

export interface RabbitPublisherFake {
  publishUserCreated: jest.Mock;
  publishUserUpdated: jest.Mock;
  publishUserDeleted: jest.Mock;
}

export interface MediaFake {
  resolveMediaUrl: jest.Mock;
}

/** `KafkaService` (emit / consumer / getClient) — never opens a socket. */
export function createKafkaFake(): KafkaFake {
  return {
    emit: jest.fn().mockResolvedValue(undefined),
    consumer: jest.fn().mockResolvedValue(undefined),
    getClient: jest.fn(),
    onModuleDestroy: jest.fn().mockResolvedValue(undefined),
  };
}

/** `NotificationService.send(channel, message)` used by forgetPassword. */
export function createNotificationFake(): NotificationFake {
  return {
    send: jest.fn().mockResolvedValue(undefined),
  };
}

/** `OutboxWorkerService` — BullMQ/NATS side of the transactional outbox. */
export function createOutboxFake(): OutboxFake {
  return {
    enqueueEvent: jest.fn().mockResolvedValue(undefined),
    reconcileOutbox: jest.fn().mockResolvedValue(undefined),
    onModuleInit: jest.fn().mockResolvedValue(undefined),
    onModuleDestroy: jest.fn().mockResolvedValue(undefined),
  };
}

/** `UserRabbitMQPublisher` — user.created / updated / deleted broadcasts. */
export function createRabbitPublisherFake(): RabbitPublisherFake {
  return {
    publishUserCreated: jest.fn().mockResolvedValue(undefined),
    publishUserUpdated: jest.fn().mockResolvedValue(undefined),
    publishUserDeleted: jest.fn().mockResolvedValue(undefined),
  };
}

/** `MediaGrpcService.resolveMediaUrl` used when updating the avatar. */
export function createMediaFake(): MediaFake {
  return {
    resolveMediaUrl: jest.fn().mockResolvedValue({
      url: 'https://media.test/avatars/resolved.png',
      expiresAtSeconds: 3600,
      contentType: 'image/png',
      mediaId: 'media-1',
      status: 'READY',
    }),
  };
}
