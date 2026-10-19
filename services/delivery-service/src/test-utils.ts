import { DeliveryStatus } from './modules/delivery/enums/delivery-status.enum';
import { PaymentStatus } from './modules/delivery/enums/payment-status.enum';
import { Delivery } from './modules/delivery/entities/delivery.entity';
import { DeliveryStatusHistory } from './modules/delivery/entities/delivery-status-history.entity';
import { Outbox } from './modules/delivery/entities/outbox.entity';
import { DeliverySagaState } from './modules/delivery/entities/delivery-saga-state.entity';
import { Address } from './modules/delivery/entities/address.entity';

export const mockDelivery = (overrides: Partial<Delivery> = {}): Delivery => {
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

export const createMockAddress = (overrides: Partial<Address> = {}): Address => {
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

export const mockDeliveryStatusHistory = (overrides: Partial<DeliveryStatusHistory> = {}): DeliveryStatusHistory => {
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

export const mockOutbox = (overrides: Partial<Outbox> = {}): Outbox => {
  const outbox = new Outbox();
  outbox.id = 'outbox-123';
  outbox.eventId = 'event-123';
  outbox.eventType = 'delivery.created';
  outbox.aggregateId = 'test-delivery-id';
  outbox.aggregateType = 'delivery';
  outbox.payload = { deliveryId: 'test-delivery-id' };
  outbox.status = 'PENDING' as any;
  outbox.attempts = 0;
  outbox.availableAt = new Date();
  outbox.publishedAt = null;
  outbox.lastError = null;
  outbox.createdAt = new Date();
  outbox.updatedAt = new Date();
  return { ...outbox, ...overrides };
};

export const mockSagaState = (overrides: Partial<DeliverySagaState> = {}): DeliverySagaState => {
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

export const createMockRepository = <T extends object = any>() => ({
  create: jest.fn(),
  save: jest.fn(),
  findOne: jest.fn(),
  find: jest.fn(),
  findAndCount: jest.fn(),
  update: jest.fn(),
  delete: jest.fn(),
  remove: jest.fn(),
  findBy: jest.fn(),
  findOneBy: jest.fn(),
  findOneOrFail: jest.fn(),
  createQueryBuilder: jest.fn().mockReturnValue({
    where: jest.fn().mockReturnThis(),
    andWhere: jest.fn().mockReturnThis(),
    orWhere: jest.fn().mockReturnThis(),
    orderBy: jest.fn().mockReturnThis(),
    skip: jest.fn().mockReturnThis(),
    take: jest.fn().mockReturnThis(),
    getOne: jest.fn(),
    getMany: jest.fn(),
    getCount: jest.fn(),
  }),
});

export const createMockRedisService = () => ({
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

export const createMockI18nService = () => ({
  t: jest.fn().mockImplementation((key: string, options?: { args?: Record<string, any>; lang?: string }) => {
    const args = options?.args || {};
    return Promise.resolve(key.replace(/\./g, ' ').replace(/{(\w+)}/g, (_, k) => args[k] || ''));
  }),
  translate: jest.fn(),
});

export const createMockLogger = () => ({
  log: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
  debug: jest.fn(),
  verbose: jest.fn(),
});

export const createMockKafkaService = () => ({
  emit: jest.fn().mockResolvedValue(undefined),
  send: jest.fn().mockResolvedValue(undefined),
  subscribe: jest.fn().mockResolvedValue(undefined),
  connect: jest.fn().mockResolvedValue(undefined),
  disconnect: jest.fn().mockResolvedValue(undefined),
});

export const createMockNatsService = () => ({
  emit: jest.fn().mockResolvedValue(undefined),
  publish: jest.fn().mockResolvedValue(undefined),
  subscribe: jest.fn().mockResolvedValue(undefined),
  connect: jest.fn().mockResolvedValue(undefined),
  close: jest.fn().mockResolvedValue(undefined),
});

export const createMockRabbitMQService = () => ({
  publish: jest.fn().mockResolvedValue(undefined),
  consume: jest.fn().mockResolvedValue(undefined),
  connect: jest.fn().mockResolvedValue(undefined),
  close: jest.fn().mockResolvedValue(undefined),
});

export const createMockGrpcClient = () => ({
  getService: jest.fn().mockReturnValue({}),
});

export const createMockAlertService = () => ({
  triggerAlert: jest.fn().mockResolvedValue(undefined),
});

export const createMockConfigService = () => ({
  get: jest.fn((key: string, defaultValue?: any) => defaultValue),
});

export const createMockDeliveryRepository = () => ({
  create: jest.fn(),
  save: jest.fn(),
  findById: jest.fn(),
  findByCustomerId: jest.fn(),
  findUnassignedPendingDeliveries: jest.fn(),
  clearDriverId: jest.fn(),
  findOpenDeliveries: jest.fn(),
  appendHistory: jest.fn(),
});

export const createMockDeliveryStateMachine = () => ({
  canTransition: jest.fn(),
  assertTransition: jest.fn(),
  nextStates: jest.fn(),
});

export const createMockIdempotencyService = () => ({
  get: jest.fn(),
  execute: jest.fn(),
  clear: jest.fn(),
});

export const createMockOutboxRepository = () => ({
  createEvent: jest.fn(),
  save: jest.fn(),
  claimPending: jest.fn(),
  markPublished: jest.fn(),
  markFailed: jest.fn(),
});

export const createMockKafkaProducer = () => ({
  publish: jest.fn().mockResolvedValue(undefined),
});

export const createMockDeliveryCommandService = () => ({
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

export const createMockDeliveryQueryService = () => ({
  getById: jest.fn(),
  listByCustomer: jest.fn(),
  getOpenDeliveries: jest.fn(),
  nextStatuses: jest.fn(),
});

export const createMockDeliverySagaOrchestrator = () => ({
  execute: jest.fn(),
});

export const createMockPaymentConfirmationStep = () => ({
  execute: jest.fn(),
  compensate: jest.fn(),
  name: 'PAYMENT_AUTHORIZATION',
});

export const createMockDriverAssignmentStep = () => ({
  execute: jest.fn(),
  compensate: jest.fn(),
  name: 'DRIVER_ASSIGNMENT',
});

export const createMockDriverDispatchSchedulerService = () => ({
  processUnassignedDeliveries: jest.fn(),
});

export const createMockDeliveryRabbitMQPublisher = () => ({
  publishOrderCreated: jest.fn().mockResolvedValue(undefined),
  publishOrderStatusChanged: jest.fn().mockResolvedValue(undefined),
  publishDriverAssigned: jest.fn().mockResolvedValue(undefined),
  publishOrderCompleted: jest.fn().mockResolvedValue(undefined),
  publishOrderCancelled: jest.fn().mockResolvedValue(undefined),
  publishDispatchRequest: jest.fn().mockResolvedValue(undefined),
});

export const createMockGraphQLContext = (overrides: any = {}) => ({
  req: {
    user: overrides.user || { id: 'user-123', role: 'customer' },
    headers: {
      'x-user-id': overrides.userId || 'user-123',
      'x-user-role': overrides.userRole || 'customer',
      'accept-language': overrides.language || 'en',
      ...overrides.headers,
    },
  },
  language: overrides.language || 'en',
});

export const createMockGraphQLContextForAdmin = (overrides: any = {}) => ({
  req: {
    user: overrides.user || { id: 'admin-123', role: 'admin' },
    headers: {
      'x-user-id': overrides.userId || 'admin-123',
      'x-user-role': overrides.userRole || 'admin',
      'accept-language': overrides.language || 'en',
      ...overrides.headers,
    },
  },
  language: overrides.language || 'en',
});

export const createMockCreateDeliveryInput = (overrides: any = {}) => ({
  amount: '100.00',
  currency: 'USD',
  pickupAddress: {
    line1: '123 Source St',
    line2: 'Apt 1',
    city: 'San Francisco',
    state: 'CA',
    postalCode: '94105',
    countryCode: 'US',
    latitude: 37.7749,
    longitude: -122.4194,
  },
  dropoffAddress: {
    line1: '456 Destination St',
    line2: 'Suite 100',
    city: 'San Francisco',
    state: 'CA',
    postalCode: '94105',
    countryCode: 'US',
    latitude: 37.7849,
    longitude: -122.4094,
  },
  idempotencyKey: 'idem-key-123',
  ...overrides,
});

export const createMockTransitionInput = (overrides: any = {}) => ({
  deliveryId: 'test-delivery-id',
  status: DeliveryStatus.DRIVER_ASSIGNED,
  note: 'Driver assigned',
  ...overrides,
});

export const createMockDeliverySagaContext = (overrides: any = {}) => ({
  delivery: mockDelivery(),
  paymentId: 'payment-123',
  authorizationId: 'auth-123',
  ...overrides,
});

export const createMockDeliveryMetricsService = () => ({
  getContentType: jest.fn().mockReturnValue('text/plain; version=0.0.4; charset=utf-8'),
  getMetrics: jest.fn().mockResolvedValue('# HELP test_metric Test metric\n# TYPE test_metric gauge\ntest_metric 1\n'),
});

export const mockDeliveryStatusValues = Object.values(DeliveryStatus);
export const mockPaymentStatusValues = Object.values(PaymentStatus);