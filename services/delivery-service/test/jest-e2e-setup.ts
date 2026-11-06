// Jest setup file for e2e tests
// This file is executed before the test files to mock modules that import TypeORM

// Mock modules that import TypeORM before they are imported
jest.mock('../src/modules/delivery/services/delivery-command.service', () => ({
  DeliveryCommandService: class DeliveryCommandService {
    create = jest.fn();
    executeWithIdempotency = jest.fn();
    transition = jest.fn();
    updatePaymentStatus = jest.fn();
    assignDriver = jest.fn();
    acceptDriver = jest.fn();
    retryDriverDispatch = jest.fn();
    handleDriverRejectedOrExpired = jest.fn();
    handleAssignmentRejected = jest.fn();
    handleDriverSearchTimeout = jest.fn();
    cancel = jest.fn();
    clearIdempotency = jest.fn();
  },
}));

jest.mock('../src/modules/delivery/services/delivery-query.service', () => ({
  DeliveryQueryService: class DeliveryQueryService {
    getById = jest.fn();
    listByCustomer = jest.fn();
    getOpenDeliveries = jest.fn();
    nextStatuses = jest.fn();
  },
}));

jest.mock('../src/modules/delivery/saga/delivery-saga.orchestrator', () => ({
  DeliverySagaOrchestrator: class DeliverySagaOrchestrator {
    execute = jest.fn();
  },
}));

jest.mock('../src/modules/delivery/repositories/delivery.repository', () => ({
  DeliveryRepository: class DeliveryRepository {
    create = jest.fn();
    save = jest.fn();
    findById = jest.fn();
    findByCustomerId = jest.fn();
    findUnassignedPendingDeliveries = jest.fn();
    clearDriverId = jest.fn();
    findOpenDeliveries = jest.fn();
    appendHistory = jest.fn();
  },
}));

jest.mock('../src/modules/delivery/services/delivery.state-machine', () => ({
  DeliveryStateMachine: class DeliveryStateMachine {
    canTransition = jest.fn();
    assertTransition = jest.fn();
    nextStates = jest.fn();
  },
}));

jest.mock('../src/modules/delivery/services/idempotency.service', () => ({
  IdempotencyService: class IdempotencyService {
    get = jest.fn();
    execute = jest.fn();
    clear = jest.fn();
  },
}));

jest.mock('../src/modules/delivery/outbox/outbox.repository', () => ({
  OutboxRepository: class OutboxRepository {
    createEvent = jest.fn();
    save = jest.fn();
    claimPending = jest.fn();
    markPublished = jest.fn();
    markFailed = jest.fn();
  },
}));

jest.mock('../src/modules/delivery/outbox/kafka.producer', () => ({
  KafkaProducer: class KafkaProducer {
    publish = jest.fn().mockResolvedValue(undefined);
  },
}));

jest.mock('../src/modules/infrastructure/rabbitmq/rabbitmq.publisher', () => ({
  DeliveryRabbitMQPublisher: class DeliveryRabbitMQPublisher {
    publishOrderCreated = jest.fn().mockResolvedValue(undefined);
    publishOrderStatusChanged = jest.fn().mockResolvedValue(undefined);
    publishDriverAssigned = jest.fn().mockResolvedValue(undefined);
    publishOrderCompleted = jest.fn().mockResolvedValue(undefined);
    publishOrderCancelled = jest.fn().mockResolvedValue(undefined);
    publishDispatchRequest = jest.fn().mockResolvedValue(undefined);
  },
}));

jest.mock('@delivery-micro/shard', () => ({
  NatsService: class NatsService {
    emit = jest.fn().mockResolvedValue(undefined);
    publish = jest.fn().mockResolvedValue(undefined);
    subscribe = jest.fn().mockResolvedValue(undefined);
    connect = jest.fn().mockResolvedValue(undefined);
    close = jest.fn().mockResolvedValue(undefined);
  },
}));

jest.mock('@bts-soft/core', () => ({
  RedisService: class RedisService {
    get = jest.fn();
    set = jest.fn();
    setNX = jest.fn();
    del = jest.fn();
    exists = jest.fn();
    expire = jest.fn();
    incr = jest.fn();
    decr = jest.fn();
    keys = jest.fn();
    flushAll = jest.fn();
  },
}));
