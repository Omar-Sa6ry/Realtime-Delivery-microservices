import { KafkaConsumer } from './kafka.consumer';
import { BaseKafkaConsumer, KafkaService } from '@delivery-micro/shard';
import { ConfigService } from '@nestjs/config';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';

describe('KafkaConsumer', () => {
  let consumer: KafkaConsumer;
  let kafkaService: {
    consumer: jest.Mock;
    getClient: jest.Mock;
    emit: jest.Mock;
  };
  let kafkaJsConsumer: {
    connect: jest.Mock;
    subscribe: jest.Mock;
    run: jest.Mock;
    disconnect: jest.Mock;
  };
  let admin: {
    connect: jest.Mock;
    listTopics: jest.Mock;
    createTopics: jest.Mock;
    disconnect: jest.Mock;
  };
  let config: { get: jest.Mock };
  let metrics: { kafkaDlq: { inc: jest.Mock } };
  let handler: { eventType: string; handle: jest.Mock };
  let paymentHandler: { eventType: string; handle: jest.Mock };

  const payloadWith = (value: any, topic = 'delivery.created') =>
    ({
      topic,
      partition: 0,
      message: {
        key: Buffer.from('k'),
        value: Buffer.from(
          typeof value === 'string' ? value : JSON.stringify(value),
        ),
        offset: '1',
      },
    }) as any;

  beforeEach(() => {
    jest.clearAllMocks();
    jest.spyOn(console, 'error').mockImplementation();
    handler = {
      eventType: 'delivery.created',
      handle: jest.fn().mockResolvedValue(undefined),
    };
    paymentHandler = {
      eventType: 'payment.completed',
      handle: jest.fn().mockResolvedValue(undefined),
    };
    kafkaJsConsumer = {
      connect: jest.fn().mockResolvedValue(undefined),
      subscribe: jest.fn().mockResolvedValue(undefined),
      run: jest.fn().mockResolvedValue(undefined),
      disconnect: jest.fn().mockResolvedValue(undefined),
    };
    admin = {
      connect: jest.fn().mockResolvedValue(undefined),
      listTopics: jest.fn().mockResolvedValue(['delivery.created']),
      createTopics: jest.fn().mockResolvedValue(undefined),
      disconnect: jest.fn().mockResolvedValue(undefined),
    };
    kafkaService = {
      consumer: jest.fn().mockReturnValue(kafkaJsConsumer),
      getClient: jest.fn().mockReturnValue({ admin: () => admin }),
      emit: jest.fn().mockResolvedValue(undefined),
    };
    config = {
      get: jest.fn((key: string, def: string) =>
        key === 'KAFKA_GROUP_ID' ? 'test-group' : def,
      ),
    };
    metrics = { kafkaDlq: { inc: jest.fn() } };

    consumer = new KafkaConsumer(
      kafkaService as unknown as KafkaService,
      config as unknown as ConfigService,
      metrics as unknown as RealtimeMetricsService,
      [handler as any, paymentHandler as any],
    );
    jest.spyOn((consumer as any).logger, 'warn').mockImplementation();
    jest.spyOn((consumer as any).logger, 'error').mockImplementation();
    jest.spyOn((consumer as any).logger, 'debug').mockImplementation();
    jest.spyOn((consumer as any).logger, 'log').mockImplementation();
  });

  afterEach(async () => {
    await consumer.onModuleDestroy();
  });

  it('creates the kafka consumer with the configured group id', () => {
    expect(kafkaService.consumer).toHaveBeenCalledWith('test-group');
  });

  it('registers handlers by event type', async () => {
    await consumer.handleMessage(
      payloadWith({
        eventId: 'e1',
        eventType: 'delivery.created',
        payload: {},
      }),
    );
    expect(handler.handle).toHaveBeenCalledWith(
      expect.objectContaining({ eventId: 'e1', eventType: 'delivery.created' }),
    );
  });

  it('drops malformed envelopes without eventId/eventType', async () => {
    await consumer.handleMessage(payloadWith({ foo: 'bar' }));
    expect(handler.handle).not.toHaveBeenCalled();

    await consumer.handleMessage(payloadWith('not-json-at-all{'));
  });

  it('ignores event types without a registered handler', async () => {
    await consumer.handleMessage(
      payloadWith({ eventId: 'e1', eventType: 'unknown.type', payload: {} }),
    );
    expect(handler.handle).not.toHaveBeenCalled();
  });

  it('treats a null message value as an empty envelope', async () => {
    await consumer.handleMessage({
      topic: 'delivery.created',
      partition: 0,
      message: { key: Buffer.from('k'), value: null, offset: '1' },
    } as any);

    expect(handler.handle).not.toHaveBeenCalled();
    expect((consumer as any).logger.warn).toHaveBeenCalledWith(
      expect.stringContaining('Dropping malformed Kafka message'),
    );
  });

  it('falls back to a dlq key when the original message has no key', async () => {
    handler.handle.mockRejectedValue(new Error('handler exploded'));

    await consumer.handleMessage({
      topic: 'delivery.created',
      partition: 0,
      message: {
        key: null,
        value: Buffer.from(
          JSON.stringify({
            eventId: 'e1',
            eventType: 'delivery.created',
            payload: {},
          }),
        ),
        offset: '1',
      },
    } as any);

    expect(kafkaService.emit).toHaveBeenCalledWith(
      'realtime.delivery.dlq',
      'realtime.dlq',
      expect.anything(),
      { key: 'dlq' },
    );
  });

  it('routes failures on delivery topics to the delivery DLQ', async () => {
    handler.handle.mockRejectedValue(new Error('handler exploded'));

    await consumer.handleMessage(
      payloadWith(
        { eventId: 'e1', eventType: 'delivery.created', payload: {} },
        'delivery.created',
      ),
    );

    expect(metrics.kafkaDlq.inc).toHaveBeenCalledWith({
      topic: 'delivery.created',
    });
    expect(kafkaService.emit).toHaveBeenCalledWith(
      'realtime.delivery.dlq',
      'realtime.dlq',
      expect.objectContaining({
        originalTopic: 'delivery.created',
        reason: 'handler exploded',
        offset: '1',
      }),
      { key: 'k' },
    );
  });

  it('routes failures on payment topics to the payment DLQ', async () => {
    paymentHandler.handle.mockRejectedValue(new Error('payment boom'));

    await consumer.handleMessage(
      payloadWith(
        { eventId: 'e1', eventType: 'payment.completed', payload: {} },
        'payment.completed',
      ),
    );

    expect(kafkaService.emit).toHaveBeenCalledWith(
      'realtime.payment.dlq',
      'realtime.dlq',
      expect.anything(),
      expect.anything(),
    );
  });

  it('logs and swallows DLQ publish failures', async () => {
    handler.handle.mockRejectedValue(new Error('handler exploded'));
    kafkaService.emit.mockRejectedValue(new Error('dlq down'));
    const errorLog = jest.spyOn((consumer as any).logger, 'error');

    await expect(
      consumer.handleMessage(
        payloadWith(
          { eventId: 'e1', eventType: 'delivery.created', payload: {} },
          'delivery.created',
        ),
      ),
    ).resolves.toBeUndefined();

    expect(errorLog).toHaveBeenCalled();
  });

  describe('lifecycle', () => {
    const flushBackground = () =>
      new Promise((resolve) => setImmediate(resolve));

    it('starts the consumer in the background and reports connected', async () => {
      await consumer.onModuleInit();
      await flushBackground();

      expect(kafkaJsConsumer.connect).toHaveBeenCalled();
      expect(kafkaJsConsumer.run).toHaveBeenCalledWith(
        expect.objectContaining({ eachMessage: expect.any(Function) }),
      );
    });

    it('surfaces background start failures without throwing', async () => {
      jest.useFakeTimers();
      kafkaJsConsumer.connect
        .mockRejectedValueOnce(new Error('kafka down'))
        .mockResolvedValue(undefined);

      await expect(consumer.onModuleInit()).resolves.toBeUndefined();
      await jest.advanceTimersByTimeAsync(5000);

      expect(kafkaJsConsumer.connect).toHaveBeenCalledTimes(2);
      jest.useRealTimers();
    });

    it('warns without throwing when the base consumer fails to start', async () => {
      const baseSpy = jest
        .spyOn(BaseKafkaConsumer.prototype, 'onModuleInit')
        .mockRejectedValueOnce(new Error('boot failed'));
      const warn = jest.spyOn((consumer as any).logger, 'warn');

      await consumer.onModuleInit();
      await flushBackground();
      baseSpy.mockRestore();

      expect(warn).toHaveBeenCalledWith(
        expect.stringContaining(
          'Kafka consumer background start error: boot failed',
        ),
      );
    });

    it('onModuleDestroy disconnects the consumer', async () => {
      await consumer.onModuleDestroy();
      expect(kafkaJsConsumer.disconnect).toHaveBeenCalled();
      expect(consumer.isConnected()).toBe(false);
    });
  });

  it('creates missing topics when the admin reports them absent', async () => {
    admin.listTopics.mockResolvedValue([]);

    await consumer.onModuleInit();
    await new Promise((resolve) => setImmediate(resolve));

    expect(admin.createTopics).toHaveBeenCalledWith({
      topics: expect.any(Array),
    });
  });
});
