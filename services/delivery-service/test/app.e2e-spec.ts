import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication, ValidationPipe } from '@nestjs/common';
import request from 'supertest';
import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { GraphQLModule } from '@nestjs/graphql';
import {
  ApolloFederationDriver,
  ApolloFederationDriverConfig,
} from '@nestjs/apollo';
import { JwtModule, JwtService } from '@nestjs/jwt';
import { HealthController } from '../src/health.controller';
import { DeliveryMetricsService } from '../src/common/metrics/delivery-metrics.service';
import deliveryConfig from '../src/common/config/delivery.config';
import { APP_FILTER, APP_INTERCEPTOR } from '@nestjs/core';
import { GraphQLExceptionFilter, NatsService } from '@delivery-micro/shard';
import { RedisService } from '@bts-soft/core';
import { I18nService } from 'nestjs-i18n';
import { DeliveryCommandService } from '../src/modules/delivery/services/delivery-command.service';
import { DeliveryQueryService } from '../src/modules/delivery/services/delivery-query.service';
import { DeliverySagaOrchestrator } from '../src/modules/delivery/saga/delivery-saga.orchestrator';
import { DeliveryRepository } from '../src/modules/delivery/repositories/delivery.repository';
import { DeliveryStateMachine } from '../src/modules/delivery/services/delivery.state-machine';
import { IdempotencyService } from '../src/modules/delivery/services/idempotency.service';
import { OutboxRepository } from '../src/modules/delivery/outbox/outbox.repository';
import { KafkaProducer } from '../src/modules/delivery/outbox/kafka.producer';
import { DeliveryRabbitMQPublisher } from '../src/modules/infrastructure/rabbitmq/rabbitmq.publisher';
import { DeliveryResolver } from '../src/modules/delivery/graphql/delivery.resolver';
import { DeliveryQueryResolver } from '../src/modules/delivery/graphql/delivery.query.resolver';
import { AppResolver } from '../src/modules/delivery/graphql/app.resolver';

// Provider tokens
const DELIVERY_COMMAND_SERVICE = DeliveryCommandService;
const DELIVERY_QUERY_SERVICE = DeliveryQueryService;
const DELIVERY_SAGA_ORCHESTRATOR = DeliverySagaOrchestrator;
const DELIVERY_REPOSITORY = DeliveryRepository;
const DELIVERY_STATE_MACHINE = DeliveryStateMachine;
const IDEMPOTENCY_SERVICE = IdempotencyService;
const OUTBOX_REPOSITORY = OutboxRepository;
const KAFKA_PRODUCER = KafkaProducer;
const DELIVERY_RABBITMQ_PUBLISHER = DeliveryRabbitMQPublisher;

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      load: [deliveryConfig],
      envFilePath: ['../../config/env/.env.test'],
    }),

    JwtModule.registerAsync({
      global: true,
      imports: [ConfigModule],
      useFactory: () => ({
        secret: process.env.JWT_SECRET || 'test_secret',
        signOptions: {
          expiresIn: '36000s',
        },
      }),
    }),

    GraphQLModule.forRoot<ApolloFederationDriverConfig>({
      driver: ApolloFederationDriver,
      path: '/delivery/graphql',
      autoSchemaFile: {
        federation: 2,
      },
      context: ({ req }) => ({
        req,
        user: req?.user,
        language: req?.headers?.['accept-language'] || 'en',
      }),
      playground: true,
      debug: false,
    }),
  ],
  controllers: [HealthController],
  providers: [
    AppResolver,
    DeliveryResolver,
    DeliveryQueryResolver,
    DeliveryMetricsService,
    {
      provide: DELIVERY_COMMAND_SERVICE,
      useFactory: () => ({
        create: jest.fn(),
        executeWithIdempotency: jest.fn(),
        transition: jest.fn(),
        updatePaymentStatus: jest.fn(),
        assignDriver: jest.fn(),
        acceptDriver: jest.fn(),
        retryDriverDispatch: jest.fn(),
        handleDriverRejectedOrExpired: jest.fn(),
        handleAssignmentRejected: jest.fn(),
        handleDriverSearchTimeout: jest.fn(),
        cancel: jest.fn(),
        clearIdempotency: jest.fn(),
      }),
    },
    {
      provide: DELIVERY_QUERY_SERVICE,
      useFactory: () => ({
        getById: jest.fn(),
        listByCustomer: jest.fn(),
        getOpenDeliveries: jest.fn(),
        nextStatuses: jest.fn(),
      }),
    },
    {
      provide: DELIVERY_SAGA_ORCHESTRATOR,
      useFactory: () => ({
        execute: jest.fn(),
      }),
    },
    {
      provide: DELIVERY_REPOSITORY,
      useFactory: () => ({
        create: jest.fn(),
        save: jest.fn(),
        findById: jest.fn(),
        findByCustomerId: jest.fn(),
        findUnassignedPendingDeliveries: jest.fn(),
        clearDriverId: jest.fn(),
        findOpenDeliveries: jest.fn(),
        appendHistory: jest.fn(),
      }),
    },
    {
      provide: DELIVERY_STATE_MACHINE,
      useFactory: () => ({
        canTransition: jest.fn(),
        assertTransition: jest.fn(),
        nextStates: jest.fn(),
      }),
    },
    {
      provide: IDEMPOTENCY_SERVICE,
      useFactory: () => ({
        get: jest.fn(),
        execute: jest.fn(),
        clear: jest.fn(),
      }),
    },
    {
      provide: OUTBOX_REPOSITORY,
      useFactory: () => ({
        createEvent: jest.fn(),
        save: jest.fn(),
        claimPending: jest.fn(),
        markPublished: jest.fn(),
        markFailed: jest.fn(),
      }),
    },
    {
      provide: KAFKA_PRODUCER,
      useFactory: () => ({
        publish: jest.fn().mockResolvedValue(undefined),
      }),
    },
    {
      provide: DELIVERY_RABBITMQ_PUBLISHER,
      useFactory: () => ({
        publishOrderCreated: jest.fn().mockResolvedValue(undefined),
        publishOrderStatusChanged: jest.fn().mockResolvedValue(undefined),
        publishDriverAssigned: jest.fn().mockResolvedValue(undefined),
        publishOrderCompleted: jest.fn().mockResolvedValue(undefined),
        publishOrderCancelled: jest.fn().mockResolvedValue(undefined),
        publishDispatchRequest: jest.fn().mockResolvedValue(undefined),
      }),
    },
    {
      provide: NatsService,
      useFactory: () => ({
        emit: jest.fn().mockResolvedValue(undefined),
        publish: jest.fn().mockResolvedValue(undefined),
        subscribe: jest.fn().mockResolvedValue(undefined),
        connect: jest.fn().mockResolvedValue(undefined),
        close: jest.fn().mockResolvedValue(undefined),
      }),
    },
    {
      provide: RedisService,
      useFactory: () => ({
        get: jest.fn(),
        set: jest.fn(),
        setNX: jest.fn(),
        del: jest.fn(),
        exists: jest.fn(),
        expire: jest.fn(),
        incr: jest.fn(),
        decr: jest.fn(),
        keys: jest.fn(),
        flushAll: jest.fn(),
      }),
    },
    {
      provide: I18nService,
      useFactory: () => ({
        t: jest
          .fn()
          .mockImplementation(
            (
              key: string,
              options?: { args?: Record<string, any>; lang?: string },
            ) => {
              const args = options?.args || {};
              return Promise.resolve(
                key
                  .replace(/\./g, ' ')
                  .replace(/{(\w+)}/g, (_, k) => args[k] || ''),
              );
            },
          ),
        translate: jest.fn(),
      }),
    },
    { provide: 'USER_SERVICE', useFactory: () => ({ GetUser: jest.fn() }) },
    {
      provide: 'PAYMENT_SERVICE',
      useFactory: () => ({
        CreatePayment: jest.fn(),
        CapturePayment: jest.fn(),
        CancelAuthorization: jest.fn(),
        CreateRefund: jest.fn(),
      }),
    },
    {
      provide: 'DRIVER_SERVICE',
      useFactory: () => ({
        ReserveDriver: jest.fn(),
        ReleaseDriver: jest.fn(),
      }),
    },
    {
      provide: DeliveryMetricsService,
      useFactory: () => ({
        getContentType: jest
          .fn()
          .mockReturnValue('text/plain; version=0.0.4; charset=utf-8'),
        getMetrics: jest
          .fn()
          .mockResolvedValue(
            '# HELP test_metric Test metric\n# TYPE test_metric gauge\ntest_metric 1\n',
          ),
      }),
    },
    {
      provide: APP_FILTER,
      useClass: GraphQLExceptionFilter,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: class {
        intercept(context: any, next: any) {
          return next.handle();
        }
      },
    },
  ],
})
class TestAppModule {}

// Mock the modules that import TypeORM
jest.mock('@bts-soft/core', () => ({
  RedisService: jest.fn().mockImplementation(() => ({
    get: jest.fn(),
    set: jest.fn(),
    setNX: jest.fn(),
    del: jest.fn(),
    exists: jest.fn(),
    expire: jest.fn(),
    incr: jest.fn(),
    decr: jest.fn(),
    keys: jest.fn(),
    flushAll: jest.fn(),
  })),
}));

// Mock the enums
jest.mock('../src/modules/delivery/enums/delivery-status.enum', () => ({
  DeliveryStatus: {
    CREATED: 'CREATED',
    PENDING: 'PENDING',
    PENDING_PAYMENT: 'PENDING_PAYMENT',
    PAYMENT_CONFIRMED: 'PAYMENT_CONFIRMED',
    SEARCHING_DRIVER: 'SEARCHING_DRIVER',
    DRIVER_ASSIGNED: 'DRIVER_ASSIGNED',
    DRIVER_ACCEPTED: 'DRIVER_ACCEPTED',
    PICKUP_STARTED: 'PICKUP_STARTED',
    PICKED_UP: 'PICKED_UP',
    IN_TRANSIT: 'IN_TRANSIT',
    DELIVERED: 'DELIVERED',
    COMPLETED: 'COMPLETED',
    CANCELLED: 'CANCELLED',
    FAILED: 'FAILED',
  },
}));

jest.mock('../src/modules/delivery/enums/payment-status.enum', () => ({
  PaymentStatus: {
    PENDING: 'PENDING',
    AUTHORIZED: 'AUTHORIZED',
    CAPTURED: 'CAPTURED',
    COMPLETED: 'COMPLETED',
    FAILED: 'FAILED',
    REFUNDED: 'REFUNDED',
    CANCELLED: 'CANCELLED',
  },
}));

// Mock the entities
jest.mock('../src/modules/delivery/entities/delivery.entity', () => {
  class MockDelivery {
    id = 'test-delivery-id';
    customerId = 'customer-123';
    driverId = null;
    status = 'CREATED';
    paymentStatus = 'PENDING';
    amount = '100.00';
    currency = 'USD';
    checkoutUrl = null;
    pickupAddress: any = { line1: '123 Source St', city: 'San Francisco' };
    dropoffAddress: any = {
      line1: '456 Destination St',
      city: 'San Francisco',
    };
    pickedUpAt = null;
    completedAt = null;
    cancelledAt = null;
    createdAt = new Date();
    updatedAt = new Date();
    statusHistory: any[] = [];
    sagaState = null;
    constructor(overrides: Partial<MockDelivery> = {}) {
      Object.assign(this, overrides);
    }
  }
  return { Delivery: MockDelivery };
});

jest.mock('../src/modules/delivery/entities/address.entity', () => {
  class MockAddress {
    line1 = '123 Source St';
    line2 = 'Apt 1';
    city = 'San Francisco';
    state = 'CA';
    postalCode = '94105';
    countryCode = 'US';
    latitude = 37.7749;
    longitude = -122.4194;
    constructor(overrides: Partial<MockAddress> = {}) {
      Object.assign(this, overrides);
    }
  }
  return { Address: MockAddress };
});

jest.mock(
  '../src/modules/delivery/entities/delivery-status-history.entity',
  () => {
    class MockDeliveryStatusHistory {
      id = 'history-123';
      deliveryId = 'test-delivery-id';
      status = 'CREATED';
      changedBy = 'user-123';
      note = 'Created';
      metadata = null;
      createdAt = new Date();
      constructor(overrides: Partial<MockDeliveryStatusHistory> = {}) {
        Object.assign(this, overrides);
      }
    }
    return { DeliveryStatusHistory: MockDeliveryStatusHistory };
  },
);

jest.mock('../src/modules/delivery/entities/outbox.entity', () => {
  class MockOutbox {
    id = 'outbox-123';
    eventId = 'event-123';
    eventType = 'delivery.created';
    aggregateId = 'test-delivery-id';
    aggregateType = 'delivery';
    payload = { deliveryId: 'test-delivery-id' };
    status = 'PENDING';
    attempts = 0;
    availableAt = new Date();
    publishedAt = null;
    lastError = null;
    createdAt = new Date();
    updatedAt = new Date();
    constructor(overrides: Partial<MockOutbox> = {}) {
      Object.assign(this, overrides);
    }
  }
  return {
    Outbox: MockOutbox,
    OutboxStatus: {
      PENDING: 'PENDING',
      PROCESSING: 'PROCESSING',
      PUBLISHED: 'PUBLISHED',
      FAILED: 'FAILED',
    },
  };
});

jest.mock('../src/modules/delivery/entities/delivery-saga-state.entity', () => {
  class MockDeliverySagaState {
    id = 'saga-123';
    deliveryId = 'test-delivery-id';
    currentStep = 'PAYMENT_AUTHORIZATION';
    status = 'IN_PROGRESS';
    compensationData = null;
    createdAt = new Date();
    updatedAt = new Date();
    constructor(overrides: Partial<MockDeliverySagaState> = {}) {
      Object.assign(this, overrides);
    }
  }
  return { DeliverySagaState: MockDeliverySagaState };
});

// Delivery entity imports and helpers
import { Delivery } from '../src/modules/delivery/entities/delivery.entity';
import { DeliveryStatus } from '../src/modules/delivery/enums/delivery-status.enum';
import { PaymentStatus } from '../src/modules/delivery/enums/payment-status.enum';
import { Address } from '../src/modules/delivery/entities/address.entity';
import { DeliveryStatusHistory } from '../src/modules/delivery/entities/delivery-status-history.entity';
import {
  Outbox,
  OutboxStatus,
} from '../src/modules/delivery/entities/outbox.entity';
import { DeliverySagaState } from '../src/modules/delivery/entities/delivery-saga-state.entity';

const mockDelivery = (overrides: Partial<Delivery> = {}): Delivery => {
  const delivery = new Delivery();
  delivery.id = 'test-delivery-id';
  delivery.customerId = 'customer-123';
  delivery.driverId = null;
  delivery.status = DeliveryStatus.CREATED;
  delivery.paymentStatus = PaymentStatus.PENDING;
  delivery.amount = '100.00';
  delivery.currency = 'USD';
  delivery.checkoutUrl = null;
  delivery.pickupAddress = createMockAddress();
  delivery.dropoffAddress = createMockAddress({ line1: '456 Destination St' });
  delivery.pickedUpAt = null;
  delivery.completedAt = null;
  delivery.cancelledAt = null;
  delivery.createdAt = new Date();
  delivery.updatedAt = new Date();
  delivery.statusHistory = [];
  delivery.sagaState = null;
  return { ...delivery, ...overrides };
};

const createMockAddress = (overrides: Partial<Address> = {}): Address => {
  const address = new Address();
  address.line1 = '123 Source St';
  address.line2 = 'Apt 1';
  address.city = 'San Francisco';
  address.state = 'CA';
  address.postalCode = '94105';
  address.countryCode = 'US';
  address.latitude = 37.7749;
  address.longitude = -122.4194;
  return { ...address, ...overrides };
};

const createMockDeliveryStatusHistory = (
  overrides: Partial<DeliveryStatusHistory> = {},
): DeliveryStatusHistory => {
  const history = new DeliveryStatusHistory();
  history.id = 'history-123';
  history.deliveryId = 'test-delivery-id';
  history.status = DeliveryStatus.CREATED;
  history.changedBy = 'user-123';
  history.note = 'Created';
  history.metadata = null;
  history.createdAt = new Date();
  return { ...history, ...overrides };
};

const createMockOutbox = (overrides: Partial<Outbox> = {}): Outbox => {
  const outbox = new Outbox();
  outbox.id = 'outbox-123';
  outbox.eventId = 'event-123';
  outbox.eventType = 'delivery.created';
  outbox.aggregateId = 'test-delivery-id';
  outbox.aggregateType = 'delivery';
  outbox.payload = { deliveryId: 'test-delivery-id' };
  outbox.status = OutboxStatus.PENDING;
  outbox.attempts = 0;
  outbox.availableAt = new Date();
  outbox.publishedAt = null;
  outbox.lastError = null;
  outbox.createdAt = new Date();
  outbox.updatedAt = new Date();
  return { ...outbox, ...overrides };
};

const createMockSagaState = (
  overrides: Partial<DeliverySagaState> = {},
): DeliverySagaState => {
  const sagaState = new DeliverySagaState();
  sagaState.id = 'saga-123';
  sagaState.deliveryId = 'test-delivery-id';
  sagaState.currentStep = 'PAYMENT_AUTHORIZATION';
  sagaState.status = 'IN_PROGRESS';
  sagaState.compensationData = null;
  sagaState.createdAt = new Date();
  sagaState.updatedAt = new Date();
  return { ...sagaState, ...overrides };
};

const createMockDeliveryCommandService = () => ({
  create: jest.fn(),
  executeWithIdempotency: jest.fn(),
  transition: jest.fn(),
  updatePaymentStatus: jest.fn(),
  assignDriver: jest.fn(),
  acceptDriver: jest.fn(),
  retryDriverDispatch: jest.fn(),
  handleDriverRejectedOrExpired: jest.fn(),
  handleAssignmentRejected: jest.fn(),
  handleDriverSearchTimeout: jest.fn(),
  cancel: jest.fn(),
  clearIdempotency: jest.fn(),
});

const createMockDeliveryQueryService = () => ({
  getById: jest.fn(),
  listByCustomer: jest.fn(),
  getOpenDeliveries: jest.fn(),
  nextStatuses: jest.fn(),
});

const createMockDeliverySagaOrchestrator = () => ({
  execute: jest.fn(),
});

const createMockDeliveryRepository = () => ({
  create: jest.fn(),
  save: jest.fn(),
  findById: jest.fn(),
  findByCustomerId: jest.fn(),
  findUnassignedPendingDeliveries: jest.fn(),
  clearDriverId: jest.fn(),
  findOpenDeliveries: jest.fn(),
  appendHistory: jest.fn(),
});

const createMockDeliveryStateMachine = () => ({
  canTransition: jest.fn(),
  assertTransition: jest.fn(),
  nextStates: jest.fn(),
});

const createMockIdempotencyService = () => ({
  get: jest.fn(),
  execute: jest.fn(),
  clear: jest.fn(),
});

const createMockOutboxRepository = () => ({
  createEvent: jest.fn(),
  save: jest.fn(),
  claimPending: jest.fn(),
  markPublished: jest.fn(),
  markFailed: jest.fn(),
});

const createMockKafkaProducer = () => ({
  publish: jest.fn().mockResolvedValue(undefined),
});

const createMockDeliveryRabbitMQPublisher = () => ({
  publishOrderCreated: jest.fn().mockResolvedValue(undefined),
  publishOrderStatusChanged: jest.fn().mockResolvedValue(undefined),
  publishDriverAssigned: jest.fn().mockResolvedValue(undefined),
  publishOrderCompleted: jest.fn().mockResolvedValue(undefined),
  publishOrderCancelled: jest.fn().mockResolvedValue(undefined),
  publishDispatchRequest: jest.fn().mockResolvedValue(undefined),
});

const createMockNatsService = () => ({
  emit: jest.fn().mockResolvedValue(undefined),
  publish: jest.fn().mockResolvedValue(undefined),
  subscribe: jest.fn().mockResolvedValue(undefined),
  connect: jest.fn().mockResolvedValue(undefined),
  close: jest.fn().mockResolvedValue(undefined),
});

const createMockRedisService = () => ({
  get: jest.fn(),
  set: jest.fn(),
  setNX: jest.fn(),
  del: jest.fn(),
  exists: jest.fn(),
  expire: jest.fn(),
  incr: jest.fn(),
  decr: jest.fn(),
  keys: jest.fn(),
  flushAll: jest.fn(),
});

const createMockDeliveryMetricsService = () => ({
  getContentType: jest
    .fn()
    .mockReturnValue('text/plain; version=0.0.4; charset=utf-8'),
  getMetrics: jest
    .fn()
    .mockResolvedValue(
      '# HELP test_metric Test metric\n# TYPE test_metric gauge\ntest_metric 1\n',
    ),
});

const createMockUserServiceClient = () => ({
  GetUser: jest.fn(),
});

const createMockPaymentServiceClient = () => ({
  CreatePayment: jest.fn(),
  CapturePayment: jest.fn(),
  CancelAuthorization: jest.fn(),
  CreateRefund: jest.fn(),
});

const createMockDriverServiceClient = () => ({
  ReserveDriver: jest.fn(),
  ReleaseDriver: jest.fn(),
});

const createMockGrpcClient = () => ({
  getService: jest.fn().mockReturnValue({}),
});

const createMockKafkaService = () => ({
  emit: jest.fn().mockResolvedValue(undefined),
  send: jest.fn().mockResolvedValue(undefined),
  subscribe: jest.fn().mockResolvedValue(undefined),
  connect: jest.fn().mockResolvedValue(undefined),
  disconnect: jest.fn().mockResolvedValue(undefined),
});

const createMockRabbitMQService = () => ({
  publish: jest.fn().mockResolvedValue(undefined),
  consume: jest.fn().mockResolvedValue(undefined),
  connect: jest.fn().mockResolvedValue(undefined),
  close: jest.fn().mockResolvedValue(undefined),
});

const createMockAlertService = () => ({
  triggerAlert: jest.fn().mockResolvedValue(undefined),
});

describe('AppController (e2e)', () => {
  let app: INestApplication;
  let commandService: any;
  let queryService: any;
  let sagaOrchestrator: any;
  let metricsService: any;
  let deliveryRepository: any;
  let stateMachine: any;
  let idempotencyService: any;
  let outboxRepository: any;
  let kafkaProducer: any;
  let rabbitmqPublisher: any;
  let natsService: any;
  let redisService: any;

  let jwtService: any;

  const testUserId = 'customer-123';
  const createAuthHeaders = (userId = testUserId, role = 'user') => {
    const normalizedRole = role === 'customer' ? 'user' : role;
    const token = jwtService?.sign({ id: userId || 'anonymous', sub: userId || 'anonymous', role: normalizedRole }, { secret: process.env.JWT_SECRET || 'test_secret' }) || 'test-token';
    return {
      authorization: `Bearer ${token}`,
      'x-user-id': userId,
      'x-user-role': normalizedRole,
      'accept-language': 'en',
    };
  };

  const adminHeaders = () => createAuthHeaders('admin-123', 'admin');
  const driverHeaders = () => createAuthHeaders('driver-123', 'driver');

  beforeEach(async () => {
    const moduleFixture: TestingModule = await Test.createTestingModule({
      imports: [TestAppModule],
    }).compile();

    app = moduleFixture.createNestApplication();
    app.useGlobalPipes(
      new ValidationPipe({
        transform: true,
        whitelist: true,
        stopAtFirstError: true,
      }),
    );
    await app.init();

    jwtService = moduleFixture.get(JwtService);
    commandService = moduleFixture.get(DELIVERY_COMMAND_SERVICE);
    queryService = moduleFixture.get(DELIVERY_QUERY_SERVICE);
    sagaOrchestrator = moduleFixture.get(DELIVERY_SAGA_ORCHESTRATOR);
    metricsService = moduleFixture.get<DeliveryMetricsService>(
      DeliveryMetricsService,
    );
    deliveryRepository = moduleFixture.get(DELIVERY_REPOSITORY);
    stateMachine = moduleFixture.get(DELIVERY_STATE_MACHINE);
    idempotencyService = moduleFixture.get(IDEMPOTENCY_SERVICE);
    outboxRepository = moduleFixture.get(OUTBOX_REPOSITORY);
    kafkaProducer = moduleFixture.get(KAFKA_PRODUCER);
    rabbitmqPublisher = moduleFixture.get(DELIVERY_RABBITMQ_PUBLISHER);
    natsService = moduleFixture.get<NatsService>(NatsService);
    redisService = moduleFixture.get<RedisService>(RedisService);
  });

  afterEach(async () => {
    await app.close();
  });

  describe('Health Endpoints', () => {
    it('/health (GET)', () => {
      return request(app.getHttpServer())
        .get('/health')
        .expect(200)
        .expect((res) => {
          expect(res.body).toHaveProperty('status', 'UP');
          expect(res.body).toHaveProperty('service', 'delivery-service');
          expect(res.body).toHaveProperty('timestamp');
        });
    });

    it('/delivery/health (GET)', () => {
      return request(app.getHttpServer())
        .get('/delivery/health')
        .expect(200)
        .expect((res) => {
          expect(res.body).toHaveProperty('status', 'UP');
          expect(res.body).toHaveProperty('service', 'delivery-service');
        });
    });

    it('/health/live (GET)', () => {
      return request(app.getHttpServer())
        .get('/health/live')
        .expect(200)
        .expect({ status: 'UP' });
    });

    it('/delivery/health/live (GET)', () => {
      return request(app.getHttpServer())
        .get('/delivery/health/live')
        .expect(200)
        .expect({ status: 'UP' });
    });

    it('/delivery/live (GET)', () => {
      return request(app.getHttpServer())
        .get('/delivery/live')
        .expect(200)
        .expect({ status: 'UP' });
    });

    it('/health/ready (GET)', () => {
      return request(app.getHttpServer())
        .get('/health/ready')
        .expect(200)
        .expect({ status: 'UP' });
    });

    it('/delivery/health/ready (GET)', () => {
      return request(app.getHttpServer())
        .get('/delivery/health/ready')
        .expect(200)
        .expect({ status: 'UP' });
    });

    it('/delivery/ready (GET)', () => {
      return request(app.getHttpServer())
        .get('/delivery/ready')
        .expect(200)
        .expect({ status: 'UP' });
    });

    it('/metrics (GET)', () => {
      return request(app.getHttpServer())
        .get('/metrics')
        .expect(200)
        .expect('Content-Type', /text\/plain/)
        .expect((res) => {
          expect(res.text).toContain('test_metric');
        });
    });

    it('/delivery/metrics (GET)', () => {
      return request(app.getHttpServer())
        .get('/delivery/metrics')
        .expect(200)
        .expect('Content-Type', /text\/plain/);
    });
  });

  describe('GraphQL Endpoints', () => {
    const graphqlEndpoint = '/delivery/graphql';
    const testDeliveryId = 'test-delivery-id';

    describe('Query: deliveryServiceInfo', () => {
      const query = `
        query {
          deliveryServiceInfo {
            success
            statusCode
            message
            data {
              name
              version
              status
            }
          }
        }
      `;

      it('should return service info', () => {
        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders())
          .send({ query })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.deliveryServiceInfo).toHaveProperty(
              'success',
              true,
            );
            expect(res.body.data.deliveryServiceInfo).toHaveProperty(
              'statusCode',
              200,
            );
            expect(res.body.data.deliveryServiceInfo.data).toHaveProperty(
              'name',
              'delivery-service',
            );
            expect(res.body.data.deliveryServiceInfo.data).toHaveProperty(
              'status',
              'ok',
            );
          });
      });
    });

    describe('Query: delivery', () => {
      const query = `
        query GetDelivery($id: String!) {
          delivery(id: $id) {
            success
            statusCode
            message
            data {
              id
              customer { id }
              driver { id }
              status
              paymentStatus
              amount
              currency
              pickupAddress { line1 city }
              dropoffAddress { line1 city }
              createdAt
              updatedAt
            }
          }
        }
      `;

      it('should return delivery for owner', () => {
        const mockDeliveryData = mockDelivery({
          id: testDeliveryId,
          customerId: testUserId,
        });
        queryService.getById.mockResolvedValue(mockDeliveryData);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders(testUserId))
          .send({ query, variables: { id: testDeliveryId } })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.delivery.success).toBe(true);
            expect(res.body.data.delivery.data.id).toBe(testDeliveryId);
            expect(res.body.data.delivery.data.customer.id).toBe(testUserId);
          });
      });

      it('should return delivery for admin', () => {
        const mockDeliveryData = mockDelivery({
          id: testDeliveryId,
          customerId: 'other-user',
        });
        queryService.getById.mockResolvedValue(mockDeliveryData);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(adminHeaders())
          .send({ query, variables: { id: testDeliveryId } })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.delivery.success).toBe(true);
          });
      });

      it('should return delivery for assigned driver', () => {
        const mockDeliveryData = mockDelivery({
          id: testDeliveryId,
          customerId: 'other-user',
          driverId: 'driver-123',
        });
        queryService.getById.mockResolvedValue(mockDeliveryData);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(driverHeaders())
          .send({ query, variables: { id: testDeliveryId } })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.delivery.success).toBe(true);
          });
      });

      it('should return error for unauthorized user', () => {
        const mockDeliveryData = mockDelivery({
          id: testDeliveryId,
          customerId: 'owner-user',
        });
        queryService.getById.mockResolvedValue(mockDeliveryData);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders('other-user'))
          .send({ query, variables: { id: testDeliveryId } })
          .expect(200)
          .expect((res) => {
            expect(res.body.errors).toBeDefined();
            expect(res.body.errors[0].message).toContain('unauthorizedAccess');
          });
      });

      it('should return error for non-existent delivery', () => {
        queryService.getById.mockRejectedValue(new Error('Not found'));

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders())
          .send({ query, variables: { id: 'non-existent' } })
          .expect(200)
          .expect((res) => {
            expect(res.body.errors).toBeDefined();
          });
      });
    });

    describe('Query: myDeliveries', () => {
      const query = `
        query MyDeliveries($page: Int, $pageSize: Int) {
          myDeliveries(page: $page, pageSize: $pageSize) {
            success
            statusCode
            message
            data {
              items {
                id
                status
                amount
              }
              paginationInfo {
                totalItems
                currentPage
                nextPage
              }
            }
          }
        }
      `;

      it('should return paginated deliveries for customer', () => {
        const mockDeliveries = [
          mockDelivery({
            id: '1',
            customerId: testUserId,
            status: DeliveryStatus.CREATED,
          }),
          mockDelivery({
            id: '2',
            customerId: testUserId,
            status: DeliveryStatus.PICKED_UP,
          }),
        ];
        queryService.listByCustomer.mockResolvedValue([mockDeliveries, 2]);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders(testUserId))
          .send({ query, variables: { page: 1, pageSize: 10 } })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.myDeliveries.success).toBe(true);
            expect(res.body.data.myDeliveries.data.items).toHaveLength(2);
            expect(
              res.body.data.myDeliveries.data.paginationInfo.totalItems,
            ).toBe(2);
          });
      });

      it('should return error when customerId missing', () => {
        queryService.listByCustomer.mockResolvedValue([[], 0]);
        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set({
            ...createAuthHeaders(''),
            'x-user-id': '',
          })
          .send({ query, variables: { page: 1, pageSize: 10 } })
          .expect(200)
          .expect((res) => {
            expect(res.body.errors).toBeDefined();
            expect(res.body.errors[0].message).toContain(
              'authenticatedCustomerRequired',
            );
          });
      });
    });

    describe('Query: deliveryNextStatuses', () => {
      const query = `
        query DeliveryNextStatuses($id: String!) {
          deliveryNextStatuses(id: $id) {
            success
            statusCode
            message
            data {
              statuses
            }
          }
        }
      `;

      it('should return next valid statuses', () => {
        const mockDeliveryData = mockDelivery({
          id: testDeliveryId,
          customerId: testUserId,
          status: DeliveryStatus.CREATED,
        });
        queryService.getById.mockResolvedValue(mockDeliveryData);
        queryService.nextStatuses.mockReturnValue([
          DeliveryStatus.PENDING_PAYMENT,
          DeliveryStatus.PAYMENT_CONFIRMED,
          DeliveryStatus.DRIVER_ASSIGNED,
          DeliveryStatus.CANCELLED,
        ]);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders(testUserId))
          .send({ query, variables: { id: testDeliveryId } })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.deliveryNextStatuses.success).toBe(true);
            expect(res.body.data.deliveryNextStatuses.data.statuses).toContain(
              DeliveryStatus.PENDING_PAYMENT,
            );
            expect(res.body.data.deliveryNextStatuses.data.statuses).toContain(
              DeliveryStatus.CANCELLED,
            );
          });
      });
    });

    describe('Query: openDeliveries', () => {
      const query = `
        query OpenDeliveries($page: Int, $pageSize: Int) {
          openDeliveries(page: $page, pageSize: $pageSize) {
            success
            statusCode
            message
            data {
              items {
                id
                status
              }
              paginationInfo {
                totalItems
                currentPage
              }
            }
          }
        }
      `;

      it('should return open deliveries for admin', () => {
        const mockDeliveries = [
          mockDelivery({
            id: '1',
            status: DeliveryStatus.PAYMENT_CONFIRMED,
            driverId: null,
          }),
          mockDelivery({
            id: '2',
            status: DeliveryStatus.DRIVER_ASSIGNED,
            driverId: null,
          }),
        ];
        queryService.getOpenDeliveries.mockResolvedValue([mockDeliveries, 2]);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(adminHeaders())
          .send({ query, variables: { page: 1, pageSize: 10 } })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.openDeliveries.success).toBe(true);
            expect(res.body.data.openDeliveries.data.items).toHaveLength(2);
          });
      });
    });
  });

  describe('GraphQL Mutations', () => {
    const graphqlEndpoint = '/delivery/graphql';
    const testDeliveryId = 'test-delivery-id';

    describe('Mutation: createDelivery', () => {
      const mutation = `
        mutation CreateDelivery($input: CreateDeliveryInputDto!) {
          createDelivery(input: $input) {
            success
            statusCode
            message
            data {
              id
              customer { id }
              status
              paymentStatus
              amount
              currency
              checkoutUrl
              pickupAddress { line1 city }
              dropoffAddress { line1 city }
            }
          }
        }
      `;

      const validInput = {
        amount: '100.00',
        currency: 'USD',
        pickupAddress: {
          line1: '123 Source St',
          city: 'San Francisco',
          state: 'CA',
          postalCode: '94105',
          countryCode: 'US',
        },
        dropoffAddress: {
          line1: '456 Destination St',
          city: 'San Francisco',
          state: 'CA',
          postalCode: '94105',
          countryCode: 'US',
        },
        idempotencyKey: 'idem-key-123',
      };

      it('should create delivery successfully', () => {
        const createdDelivery = mockDelivery({
          customerId: testUserId,
          status: DeliveryStatus.PAYMENT_CONFIRMED,
          checkoutUrl: 'https://checkout.example.com/pay/123',
        });
        commandService.create.mockResolvedValue(createdDelivery);
        sagaOrchestrator.execute.mockResolvedValue(createdDelivery);
        commandService.executeWithIdempotency.mockImplementation((key: string, fn: Function) => fn());

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders(testUserId))
          .send({ query: mutation, variables: { input: validInput } })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.createDelivery.success).toBe(true);
            expect(res.body.data.createDelivery.statusCode).toBe(201);
            expect(res.body.data.createDelivery.data.customer.id).toBe(
              testUserId,
            );
          });
      });

      it('should fail for admin user', () => {
        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(adminHeaders())
          .send({ query: mutation, variables: { input: validInput } })
          .expect(200)
          .expect((res) => {
            expect(res.body.errors).toBeDefined();
            expect(res.body.errors[0].message).toContain(
              'onlyRegularUsersCanCreate',
            );
          });
      });

      it('should fail when customerId missing', () => {
        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set({
            ...createAuthHeaders('temp-id'),
            'x-user-id': '',
          })
          .send({ query: mutation, variables: { input: validInput } })
          .expect(200)
          .expect((res) => {
            expect(res.body.errors).toBeDefined();
            expect(res.body.errors[0].message).toContain('customerIdRequired');
          });
      });

      it('should handle saga failure', () => {
        const createdDelivery = mockDelivery({ customerId: testUserId });
        commandService.create.mockResolvedValue(createdDelivery);
        sagaOrchestrator.execute.mockRejectedValue(new Error('Payment failed'));
        commandService.executeWithIdempotency.mockImplementation((key: string, fn: Function) => fn());

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders(testUserId))
          .send({ query: mutation, variables: { input: validInput } })
          .expect(200)
          .expect((res) => {
            expect(res.body.errors).toBeDefined();
            expect(res.body.errors[0].message).toContain('paymentAuthFailed');
          });
      });
    });

    describe('Mutation: transitionDelivery', () => {
      const mutation = `
        mutation TransitionDelivery($input: TransitionDeliveryInput!) {
          transitionDelivery(input: $input) {
            success
            statusCode
            message
            data {
              id
              status
              paymentStatus
            }
          }
        }
      `;

      it('should transition delivery status', () => {
        const updatedDelivery = mockDelivery({
          id: testDeliveryId,
          status: DeliveryStatus.DRIVER_ASSIGNED,
          driverId: 'driver-123',
        });
        commandService.transition.mockResolvedValue(updatedDelivery);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(adminHeaders())
          .send({
            query: mutation,
            variables: {
              input: {
                deliveryId: testDeliveryId,
                status: DeliveryStatus.DRIVER_ASSIGNED,
                note: 'Driver assigned',
              },
            },
          })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.transitionDelivery.success).toBe(true);
            expect(res.body.data.transitionDelivery.data.status).toBe(
              DeliveryStatus.DRIVER_ASSIGNED,
            );
          });
      });

      it('should fail when deliveryId not provided', () => {
        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(adminHeaders())
          .send({
            query: mutation,
            variables: {
              input: {
                status: DeliveryStatus.DRIVER_ASSIGNED,
              },
            },
          })
          .expect(200)
          .expect((res) => {
            expect(res.body.errors).toBeDefined();
            expect(res.body.errors[0].message).toContain('deliveryIdRequired');
          });
      });
    });

    describe('Mutation: cancelDelivery', () => {
      const mutation = `
        mutation CancelDelivery($deliveryId: String, $id: String, $reason: String) {
          cancelDelivery(deliveryId: $deliveryId, id: $id, reason: $reason) {
            success
            statusCode
            message
            data {
              id
              status
            }
          }
        }
      `;

      it('should cancel delivery for owner', () => {
        const existingDelivery = mockDelivery({
          id: testDeliveryId,
          customerId: testUserId,
        });
        const cancelledDelivery = mockDelivery({
          id: testDeliveryId,
          status: DeliveryStatus.CANCELLED,
        });
        queryService.getById.mockResolvedValue(existingDelivery);
        commandService.cancel.mockResolvedValue(cancelledDelivery);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders(testUserId))
          .send({
            query: mutation,
            variables: {
              deliveryId: testDeliveryId,
              reason: 'Customer cancelled',
            },
          })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.cancelDelivery.success).toBe(true);
            expect(res.body.data.cancelDelivery.data.status).toBe(
              DeliveryStatus.CANCELLED,
            );
          });
      });

      it('should allow admin to cancel any delivery', () => {
        const existingDelivery = mockDelivery({
          id: testDeliveryId,
          customerId: 'other-user',
        });
        const cancelledDelivery = mockDelivery({
          id: testDeliveryId,
          status: DeliveryStatus.CANCELLED,
        });
        queryService.getById.mockResolvedValue(existingDelivery);
        commandService.cancel.mockResolvedValue(cancelledDelivery);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(adminHeaders())
          .send({
            query: mutation,
            variables: { deliveryId: testDeliveryId, reason: 'Admin cancel' },
          })
          .expect(200)
          .expect((res) => {
            expect(res.body.data.cancelDelivery.success).toBe(true);
          });
      });

      it('should fail when non-owner tries to cancel', () => {
        const existingDelivery = mockDelivery({
          id: testDeliveryId,
          customerId: 'owner-user',
        });
        queryService.getById.mockResolvedValue(existingDelivery);

        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders('other-user'))
          .send({
            query: mutation,
            variables: {
              deliveryId: testDeliveryId,
              reason: 'Unauthorized cancel',
            },
          })
          .expect(200)
          .expect((res) => {
            expect(res.body.errors).toBeDefined();
            expect(res.body.errors[0].message).toContain('unauthorizedCancel');
          });
      });

      it('should fail when deliveryId not provided', () => {
        return request(app.getHttpServer())
          .post(graphqlEndpoint)
          .set(createAuthHeaders(testUserId))
          .send({ query: mutation, variables: { reason: 'No ID' } })
          .expect(200)
          .expect((res) => {
            expect(res.body.errors).toBeDefined();
            expect(res.body.errors[0].message).toContain('deliveryIdRequired');
          });
      });
    });
  });
});
