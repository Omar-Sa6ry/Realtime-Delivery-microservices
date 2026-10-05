jest.mock('kafkajs', () => {
  const admin = {
    connect: jest.fn(),
    disconnect: jest.fn(),
    listTopics: jest.fn(),
    createTopics: jest.fn(),
  };
  const consumer = {
    connect: jest.fn(),
    disconnect: jest.fn(),
    subscribe: jest.fn(),
    run: jest.fn(),
  };
  const producer = {
    connect: jest.fn(),
    disconnect: jest.fn(),
    send: jest.fn(),
  };
  const kafkaInstance = {
    admin: jest.fn(() => admin),
    consumer: jest.fn(() => consumer),
    producer: jest.fn(() => producer),
  };
  return {
    Kafka: jest.fn(() => kafkaInstance),
    __mocks: { admin, consumer, producer, kafkaInstance },
  };
});

import type { Consumer, EachMessagePayload } from 'kafkajs';
import type { KafkaService } from './kafka.service';
import { BaseKafkaConsumer } from './base-kafka-consumer';

const { admin, consumer, kafkaInstance } = jest.requireMock('kafkajs').__mocks;

const TOPICS = ['orders.created', 'orders.updated'];

type MockLogger = {
  log: jest.Mock;
  warn: jest.Mock;
  error: jest.Mock;
  debug: jest.Mock;
};

const createLogger = (): MockLogger => ({
  log: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
  debug: jest.fn(),
});

class TestKafkaConsumer extends BaseKafkaConsumer {
  protected readonly logger: MockLogger;
  protected readonly consumer: Consumer;
  protected readonly kafkaService: KafkaService;
  protected readonly topics: string[];
  readonly handled: EachMessagePayload[] = [];

  constructor(deps: {
    logger: MockLogger;
    consumer: Consumer;
    kafkaService: KafkaService;
    topics: string[];
  }) {
    super();
    this.logger = deps.logger;
    this.consumer = deps.consumer;
    this.kafkaService = deps.kafkaService;
    this.topics = deps.topics;
  }

  protected async handleMessage(payload: EachMessagePayload): Promise<void> {
    const raw = payload.message.value?.toString('utf8') ?? '';
    try {
      JSON.parse(raw);
      this.handled.push(payload);
    } catch (err) {
      this.logger.error(`Failed to deserialize message: ${(err as Error).message}`);
      throw err;
    }
  }
}

const flushMicrotasks = async (ticks = 50): Promise<void> => {
  for (let i = 0; i < ticks; i++) {
    await Promise.resolve();
  }
};

const drainTimers = async (promise: Promise<void>, maxAdvances = 15): Promise<void> => {
  for (let i = 0; i < maxAdvances; i++) {
    await flushMicrotasks();
    if (jest.getTimerCount() === 0) {
      break;
    }
    await jest.advanceTimersByTimeAsync(5000);
  }
  await flushMicrotasks();
  await promise;
};

const buildPayload = (value: unknown): EachMessagePayload =>
  ({
    topic: TOPICS[0],
    partition: 0,
    message: {
      key: Buffer.from('key-1'),
      value,
      timestamp: '1700000000000000000',
      attributes: 0,
      offset: '0',
    },
  }) as unknown as EachMessagePayload;

describe('BaseKafkaConsumer', () => {
  let logger: MockLogger;
  let kafkaServiceMock: { getClient: jest.Mock };
  let instance: TestKafkaConsumer;

  beforeEach(() => {
    jest.useFakeTimers();
    jest.clearAllMocks();

    admin.connect.mockResolvedValue(undefined);
    admin.listTopics.mockResolvedValue([...TOPICS]);
    admin.createTopics.mockResolvedValue(undefined);
    admin.disconnect.mockResolvedValue(undefined);
    consumer.connect.mockResolvedValue(undefined);
    consumer.subscribe.mockResolvedValue(undefined);
    consumer.run.mockResolvedValue(undefined);
    consumer.disconnect.mockResolvedValue(undefined);

    logger = createLogger();
    kafkaServiceMock = { getClient: jest.fn(() => kafkaInstance) };
    instance = new TestKafkaConsumer({
      logger,
      consumer: consumer as unknown as Consumer,
      kafkaService: kafkaServiceMock as unknown as KafkaService,
      topics: [...TOPICS],
    });
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  describe('connection lifecycle', () => {
    it('starts disconnected', () => {
      expect(instance.isConnected()).toBe(false);
    });

    it('creates missing topics, subscribes to all topics and starts running', async () => {
      admin.listTopics.mockResolvedValue([TOPICS[0]]);

      await instance.onModuleInit();

      expect(kafkaServiceMock.getClient).toHaveBeenCalledTimes(1);
      expect(admin.connect).toHaveBeenCalledTimes(1);
      expect(admin.listTopics).toHaveBeenCalledTimes(1);
      expect(admin.createTopics).toHaveBeenCalledWith({ topics: [{ topic: TOPICS[1] }] });
      expect(admin.disconnect).toHaveBeenCalledTimes(1);
      expect(consumer.connect).toHaveBeenCalledTimes(1);
      expect(consumer.subscribe).toHaveBeenCalledTimes(2);
      expect(consumer.subscribe).toHaveBeenNthCalledWith(1, {
        topic: TOPICS[0],
        fromBeginning: false,
      });
      expect(consumer.subscribe).toHaveBeenNthCalledWith(2, {
        topic: TOPICS[1],
        fromBeginning: false,
      });
      expect(consumer.run).toHaveBeenCalledTimes(1);
      expect(logger.log).toHaveBeenCalledWith('Created missing Kafka topics: orders.updated');
      expect(logger.log).toHaveBeenCalledWith('Kafka consumer started successfully');
      expect(instance.isConnected()).toBe(true);
      expect(jest.getTimerCount()).toBe(0);
    });

    it('does not create topics when they already exist', async () => {
      await instance.onModuleInit();

      expect(admin.createTopics).not.toHaveBeenCalled();
      expect(logger.log).not.toHaveBeenCalledWith(
        expect.stringContaining('Created missing Kafka topics'),
      );
      expect(logger.log).toHaveBeenCalledWith('Kafka consumer started successfully');
      expect(instance.isConnected()).toBe(true);
    });

    it('retries with a backoff delay after a failed start attempt', async () => {
      let attempts = 0;
      consumer.connect.mockImplementation(() => {
        attempts += 1;
        return attempts === 1
          ? Promise.reject(new Error('broker unreachable'))
          : Promise.resolve();
      });

      const promise = (instance as unknown as {
        startConsumerWithRetry(retries?: number, delayMs?: number): Promise<void>;
      }).startConsumerWithRetry(2, 5000);
      await drainTimers(promise);

      expect(logger.warn).toHaveBeenCalledTimes(1);
      expect(logger.warn).toHaveBeenCalledWith(
        'Kafka consumer failed to start (attempt 1/2): broker unreachable',
      );
      expect(consumer.connect).toHaveBeenCalledTimes(2);
      expect(consumer.run).toHaveBeenCalledTimes(1);
      expect(logger.log).toHaveBeenCalledWith('Kafka consumer started successfully');
      expect(instance.isConnected()).toBe(true);
      expect(jest.getTimerCount()).toBe(0);
    });

    it('logs an error after exhausting all retries', async () => {
      admin.connect.mockRejectedValue(new Error('broker down'));

      await drainTimers(instance.onModuleInit());

      expect(logger.warn).toHaveBeenCalledTimes(10);
      expect(logger.warn).toHaveBeenLastCalledWith(
        'Kafka consumer failed to start (attempt 10/10): broker down',
      );
      expect(logger.error).toHaveBeenCalledWith(
        'Kafka consumer failed to start after maximum retries',
      );
      expect(consumer.connect).not.toHaveBeenCalled();
      expect(instance.isConnected()).toBe(false);
      expect(jest.getTimerCount()).toBe(0);
    });
  });

  describe('message handling', () => {
    let eachMessage: (payload: EachMessagePayload) => Promise<void>;

    beforeEach(async () => {
      await instance.onModuleInit();
      eachMessage = consumer.run.mock.calls[0][0].eachMessage;
    });

    it('acknowledges a valid payload by dispatching it to handleMessage', async () => {
      const payload = buildPayload(Buffer.from(JSON.stringify({ id: 1, status: 'new' })));

      await expect(eachMessage(payload)).resolves.toBeUndefined();

      expect(instance.handled).toHaveLength(1);
      expect(instance.handled[0]).toBe(payload);
      expect(logger.error).not.toHaveBeenCalled();
    });

    it('rejects and logs when the payload cannot be deserialized', async () => {
      const payload = buildPayload(Buffer.from('not-a-json'));

      await expect(eachMessage(payload)).rejects.toThrow();

      expect(logger.error).toHaveBeenCalledWith(
        expect.stringContaining('Failed to deserialize message'),
      );
      expect(instance.handled).toHaveLength(0);
    });

    it('rejects when the message value is missing', async () => {
      const payload = buildPayload(null);

      await expect(eachMessage(payload)).rejects.toThrow();

      expect(logger.error).toHaveBeenCalledWith(
        expect.stringContaining('Failed to deserialize message'),
      );
      expect(instance.handled).toHaveLength(0);
    });

    it('propagates errors raised by handleMessage to the consumer run loop', async () => {
      const invalid = buildPayload(Buffer.from('{broken'));
      const valid = buildPayload(Buffer.from(JSON.stringify({ id: 2 })));

      await expect(eachMessage(invalid)).rejects.toThrow();
      await expect(eachMessage(valid)).resolves.toBeUndefined();

      expect(instance.handled).toHaveLength(1);
    });
  });

  describe('onModuleDestroy', () => {
    it('disconnects the consumer and marks it as disconnected', async () => {
      await instance.onModuleInit();

      await instance.onModuleDestroy();

      expect(consumer.disconnect).toHaveBeenCalledTimes(1);
      expect(logger.log).toHaveBeenCalledWith('Kafka consumer disconnected');
      expect(instance.isConnected()).toBe(false);
    });

    it('logs an error when disconnect fails', async () => {
      await instance.onModuleInit();
      consumer.disconnect.mockRejectedValue(new Error('disconnect failed'));

      await instance.onModuleDestroy();

      expect(logger.error).toHaveBeenCalledWith('Kafka disconnect failed: disconnect failed');
    });
  });
});
