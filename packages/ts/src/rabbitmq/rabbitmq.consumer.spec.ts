jest.mock('amqplib', () => ({ connect: jest.fn() }));

import * as client from 'prom-client';
import { RabbitMQExchanges, RabbitMQHeaders, RabbitMQQueues } from './rabbitmq.constants';
import {
  BaseRabbitMQConsumer,
  RabbitMQConsumeContext,
  RabbitMQEventEnvelope,
} from './rabbitmq.consumer';
import { RabbitMQService } from './rabbitmq.service';

const noopLogger = () => ({
  log: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
  debug: jest.fn(),
});

class TestConsumer extends BaseRabbitMQConsumer {
  protected readonly queue = RabbitMQQueues.ORDERS_CREATED;
  protected readonly exchange = RabbitMQExchanges.ORDERS;
  protected readonly routingKeys = ['order.#', 'order.created'];
  protected readonly logger = noopLogger();
  protected readonly maxRetries = 2;
  protected readonly retryDelayMs = 100;
  public readonly handleEventMock = jest.fn(async () => undefined);
  public readonly isDuplicateMock = jest.fn(async () => false);

  protected async handleEvent(
    envelope: RabbitMQEventEnvelope,
    ctx: RabbitMQConsumeContext,
  ): Promise<void> {
    await this.handleEventMock(envelope, ctx);
  }

  protected async isDuplicate(eventId: string): Promise<boolean> {
    return this.isDuplicateMock(eventId);
  }
}

class ZeroPrefetchConsumer extends BaseRabbitMQConsumer {
  protected readonly queue = RabbitMQQueues.NOTIFICATIONS_SMS;
  protected readonly exchange = RabbitMQExchanges.NOTIFICATIONS;
  protected readonly routingKeys = ['notification.#'];
  protected readonly prefetch = 0;
  protected readonly logger = noopLogger();

  protected async handleEvent(): Promise<void> {
    return undefined;
  }
}

class UnknownExchangeConsumer extends BaseRabbitMQConsumer {
  protected readonly queue = 'custom.queue';
  protected readonly exchange = 'custom.unknown.exchange';
  protected readonly routingKeys = ['custom.key'];
  protected readonly logger = noopLogger();

  protected async handleEvent(): Promise<void> {
    return undefined;
  }
}

function createChannelMock() {
  return {
    assertExchange: jest.fn().mockResolvedValue(undefined),
    assertQueue: jest.fn().mockResolvedValue(undefined),
    bindQueue: jest.fn().mockResolvedValue(undefined),
    consume: jest.fn().mockResolvedValue({ consumerTag: 'ctag-42' }),
    ack: jest.fn(),
    nack: jest.fn(),
    sendToQueue: jest.fn().mockReturnValue(true),
    cancel: jest.fn().mockResolvedValue(undefined),
    close: jest.fn().mockResolvedValue(undefined),
  };
}

function createServiceMock() {
  return {
    createConsumerChannel: jest.fn().mockResolvedValue(createChannelMock()),
  };
}

function makeEnvelope(overrides: Partial<RabbitMQEventEnvelope> = {}): RabbitMQEventEnvelope {
  return {
    eventId: 'evt-1',
    eventType: 'order.created',
    eventVersion: 1,
    occurredAt: new Date(0).toISOString(),
    timestamp: 0,
    aggregateType: 'order',
    aggregateId: 'order-1',
    producer: 'orders-service',
    payload: { id: 'order-1' },
    ...overrides,
  };
}

function makeMessage(
  body: unknown,
  headers: Record<string, unknown> = {},
  properties: Record<string, unknown> = {},
) {
  const content = typeof body === 'string' ? body : JSON.stringify(body);
  return {
    content: Buffer.from(content),
    fields: {
      deliveryTag: 1,
      redelivered: false,
      exchange: RabbitMQExchanges.ORDERS,
      routingKey: 'order.created',
    },
    properties: {
      contentType: 'application/json',
      messageId: 'msg-1',
      headers,
      ...properties,
    },
  } as any;
}

const flush = async (times = 60): Promise<void> => {
  for (let i = 0; i < times; i += 1) {
    await Promise.resolve();
  }
};

const advance = async (ms: number): Promise<void> => {
  await flush();
  await jest.advanceTimersByTimeAsync(ms);
  await flush();
};

async function metricValue(metric: any, labels: Record<string, unknown>): Promise<number> {
  const data = await metric.get();
  const hit = data.values.find(
    (value: any) =>
      Object.entries(labels).every(([key, expected]) => value.labels[key] === expected),
  );
  return hit ? hit.value : 0;
}

describe('BaseRabbitMQConsumer', () => {
  let channel: ReturnType<typeof createChannelMock>;
  let rabbitmq: ReturnType<typeof createServiceMock>;
  let consumer: TestConsumer;

  beforeEach(() => {
    client.register.clear();
    jest.useFakeTimers();
    channel = createChannelMock();
    rabbitmq = { createConsumerChannel: jest.fn().mockResolvedValue(channel) };
    consumer = new TestConsumer(rabbitmq as unknown as RabbitMQService);
  });

  afterEach(() => {
    jest.restoreAllMocks();
    jest.useRealTimers();
  });

  const serviceLabels = { service: 'TestConsumer', queue: RabbitMQQueues.ORDERS_CREATED };

  describe('metrics registration', () => {
    it('registers metrics once and reuses them for new instances', () => {
      const second = new TestConsumer(rabbitmq as unknown as RabbitMQService);

      expect(client.register.getSingleMetric('rabbitmq_messages_consumed_total')).toBeDefined();
      expect(client.register.getSingleMetric('rabbitmq_consumer_messages_failed_total')).toBeDefined();
      expect(client.register.getSingleMetric('rabbitmq_messages_dlq_total')).toBeDefined();
      expect(client.register.getSingleMetric('rabbitmq_messages_retry_total')).toBeDefined();
      expect((consumer as any).consumedCounter).toBe((second as any).consumedCounter);
    });

    it('starts disconnected', () => {
      expect(consumer.isConnected()).toBe(false);
    });
  });

  describe('startup', () => {
    it('declares topology and starts consuming on init', async () => {
      await consumer.onModuleInit();

      expect(rabbitmq.createConsumerChannel).toHaveBeenCalledWith(10);
      expect(channel.assertExchange).toHaveBeenCalledWith(RabbitMQExchanges.ORDERS, 'topic', {
        durable: true,
        arguments: { 'alternate-exchange': 'delivery.unroutable.fanout' },
      });
      expect(channel.assertQueue).toHaveBeenCalledWith(RabbitMQQueues.ORDERS_CREATED, {
        durable: true,
        arguments: {
          'x-queue-type': 'quorum',
          'x-dead-letter-exchange': 'delivery.dlx.direct',
          'x-dead-letter-routing-key': 'orders.dlq',
        },
      });
      expect(channel.bindQueue).toHaveBeenNthCalledWith(
        1,
        RabbitMQQueues.ORDERS_CREATED,
        RabbitMQExchanges.ORDERS,
        'order.#',
      );
      expect(channel.bindQueue).toHaveBeenNthCalledWith(
        2,
        RabbitMQQueues.ORDERS_CREATED,
        RabbitMQExchanges.ORDERS,
        'order.created',
      );
      expect(channel.consume).toHaveBeenCalledWith(
        RabbitMQQueues.ORDERS_CREATED,
        expect.any(Function),
        { noAck: false },
      );
      expect((consumer as any).consumerTag).toBe('ctag-42');
      expect(consumer.isConnected()).toBe(true);
      expect((consumer as any).logger.log).toHaveBeenCalledWith(
        expect.stringContaining(`queue=<${RabbitMQQueues.ORDERS_CREATED}>`),
      );
    });

    it('processes messages delivered through the consume callback', async () => {
      await consumer.onModuleInit();
      const handler = channel.consume.mock.calls[0][1];
      const msg = makeMessage(makeEnvelope());

      handler(msg);
      await flush();

      expect(consumer.handleEventMock).toHaveBeenCalledTimes(1);
      expect(channel.ack).toHaveBeenCalledWith(msg);
    });

    it('resolves prefetch and exchange type from the constants when prefetch is falsy', async () => {
      const zero = new ZeroPrefetchConsumer(rabbitmq as unknown as RabbitMQService);

      await zero.onModuleInit();

      expect(rabbitmq.createConsumerChannel).toHaveBeenCalledWith(20);
      expect(channel.assertExchange).toHaveBeenCalledWith(
        RabbitMQExchanges.NOTIFICATIONS,
        'fanout',
        { durable: true, arguments: { 'alternate-exchange': 'delivery.unroutable.fanout' } },
      );
      expect(channel.assertQueue).toHaveBeenCalledWith(RabbitMQQueues.NOTIFICATIONS_SMS, {
        durable: true,
        arguments: {
          'x-queue-type': 'quorum',
          'x-dead-letter-exchange': 'delivery.dlx.direct',
          'x-dead-letter-routing-key': 'notifications.dlq',
        },
      });
      expect(zero.isConnected()).toBe(true);
    });

    it('falls back to a topic exchange for unknown exchanges', async () => {
      const custom = new UnknownExchangeConsumer(rabbitmq as unknown as RabbitMQService);

      await custom.onModuleInit();

      expect(channel.assertExchange).toHaveBeenCalledWith('custom.unknown.exchange', 'topic', {
        durable: true,
        arguments: { 'alternate-exchange': 'delivery.unroutable.fanout' },
      });
      expect(channel.bindQueue).toHaveBeenCalledWith(
        'custom.queue',
        'custom.unknown.exchange',
        'custom.key',
      );
    });

    it('retries startup with exponential backoff until it succeeds', async () => {
      rabbitmq.createConsumerChannel
        .mockRejectedValueOnce(new Error('broker down'))
        .mockRejectedValueOnce(new Error('broker down'));

      const init = consumer.onModuleInit();
      await flush();
      expect(rabbitmq.createConsumerChannel).toHaveBeenCalledTimes(1);
      expect(consumer.isConnected()).toBe(false);

      await advance(2000);
      expect(rabbitmq.createConsumerChannel).toHaveBeenCalledTimes(2);

      await advance(4000);
      expect(rabbitmq.createConsumerChannel).toHaveBeenCalledTimes(3);

      await init;

      expect(consumer.isConnected()).toBe(true);
      expect((consumer as any).logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('attempt 1/15'),
      );
      expect((consumer as any).logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('attempt 2/15'),
      );
      expect((consumer as any).logger.log).toHaveBeenCalledWith(
        expect.stringContaining('RabbitMQ consumer started'),
      );
    });

    it('gives up after all startup retries', async () => {
      rabbitmq.createConsumerChannel.mockRejectedValue(new Error('broker down'));

      const init = consumer.onModuleInit();
      await advance(400000);
      await init;

      expect(rabbitmq.createConsumerChannel).toHaveBeenCalledTimes(15);
      expect(consumer.isConnected()).toBe(false);
      expect((consumer as any).logger.error).toHaveBeenCalledWith(
        expect.stringContaining('failed to start after 15 retries'),
      );
      expect(jest.getTimerCount()).toBe(0);
    });

    it('does not start when a shutdown is already in progress', async () => {
      (consumer as any).closing = true;

      await consumer.onModuleInit();

      expect(rabbitmq.createConsumerChannel).not.toHaveBeenCalled();
      expect((consumer as any).logger.log).not.toHaveBeenCalled();
      expect(consumer.isConnected()).toBe(false);
    });
  });

  describe('shutdown', () => {
    it('cancels the consumer and closes the channel', async () => {
      await consumer.onModuleInit();

      await consumer.onModuleDestroy();

      expect(channel.cancel).toHaveBeenCalledWith('ctag-42');
      expect(channel.close).toHaveBeenCalled();
      expect((consumer as any).channel).toBeNull();
      expect((consumer as any).consumerTag).toBeNull();
      expect(consumer.isConnected()).toBe(false);
    });

    it('tolerates cancel and close errors', async () => {
      await consumer.onModuleInit();
      channel.cancel.mockRejectedValue(new Error('channel gone'));
      channel.close.mockRejectedValue(new Error('channel gone'));

      await expect(consumer.onModuleDestroy()).resolves.toBeUndefined();

      expect(consumer.isConnected()).toBe(false);
    });

    it('closes a channel that never registered a consumer tag', async () => {
      await consumer.onModuleInit();
      (consumer as any).consumerTag = null;

      await consumer.onModuleDestroy();

      expect(channel.cancel).not.toHaveBeenCalled();
      expect(channel.close).toHaveBeenCalled();
    });

    it('is safe to destroy before init', async () => {
      await expect(consumer.onModuleDestroy()).resolves.toBeUndefined();

      expect(channel.cancel).not.toHaveBeenCalled();
      expect(channel.close).not.toHaveBeenCalled();
      expect(consumer.isConnected()).toBe(false);
    });
  });

  describe('message handling', () => {
    it('ignores null messages', async () => {
      await consumer.onModuleInit();

      await (consumer as any).onMessage(null);

      expect(channel.ack).not.toHaveBeenCalled();
      expect(channel.nack).not.toHaveBeenCalled();
      expect(consumer.handleEventMock).not.toHaveBeenCalled();
    });

    it('ignores messages when the channel is gone', async () => {
      await consumer.onModuleInit();
      (consumer as any).channel = null;

      await (consumer as any).onMessage(makeMessage(makeEnvelope()));

      expect(consumer.handleEventMock).not.toHaveBeenCalled();
      expect(channel.ack).not.toHaveBeenCalled();
    });

    it('dead-letters invalid JSON without retrying', async () => {
      await consumer.onModuleInit();
      const msg = makeMessage('definitely-not-json');

      await (consumer as any).onMessage(msg);

      expect(consumer.handleEventMock).not.toHaveBeenCalled();
      expect(channel.nack).toHaveBeenCalledWith(msg, false, false);
      expect(channel.sendToQueue).not.toHaveBeenCalled();
      expect(
        await metricValue((consumer as any).failedCounter, {
          service: 'TestConsumer',
          queue: RabbitMQQueues.ORDERS_CREATED,
          reason: 'invalid_json',
        }),
      ).toBe(1);
      expect(
        await metricValue((consumer as any).dlqCounter, {
          service: 'TestConsumer',
          queue: RabbitMQQueues.ORDERS_CREATED,
        }),
      ).toBe(1);
      expect((consumer as any).logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('dead-lettered'),
      );
    });

    it('logs when the dead-letter nack itself fails', async () => {
      await consumer.onModuleInit();
      channel.nack.mockImplementationOnce(() => {
        throw new Error('nack failed');
      });

      await (consumer as any).onMessage(makeMessage('not-json'));

      expect((consumer as any).logger.error).toHaveBeenCalledWith(
        'Dead-letter nack failed: nack failed',
      );
    });

    it('acks duplicates without processing them', async () => {
      consumer.isDuplicateMock.mockResolvedValueOnce(true);
      await consumer.onModuleInit();
      const msg = makeMessage(makeEnvelope());

      await (consumer as any).onMessage(msg);

      expect(consumer.isDuplicateMock).toHaveBeenCalledWith('evt-1');
      expect(consumer.handleEventMock).not.toHaveBeenCalled();
      expect(channel.ack).toHaveBeenCalledWith(msg);
      expect(channel.nack).not.toHaveBeenCalled();
      expect((consumer as any).logger.debug).toHaveBeenCalledWith(
        expect.stringContaining('evt-1'),
      );
    });

    it('uses the built-in duplicate check when subclasses do not override it', async () => {
      const custom = new UnknownExchangeConsumer(rabbitmq as unknown as RabbitMQService);
      await custom.onModuleInit();
      const msg = makeMessage(makeEnvelope());

      await (custom as any).onMessage(msg);

      expect(channel.ack).toHaveBeenCalledWith(msg);
      expect(channel.nack).not.toHaveBeenCalled();
    });

    it('processes the envelope with header context and acks on success', async () => {
      await consumer.onModuleInit();
      const msg = makeMessage(makeEnvelope(), {
        [RabbitMQHeaders.RETRY_COUNT]: 1,
        [RabbitMQHeaders.TRACE_ID]: 'trace-hdr',
      });

      await (consumer as any).onMessage(msg);

      expect(consumer.handleEventMock).toHaveBeenCalledWith(
        expect.objectContaining({ eventId: 'evt-1', eventType: 'order.created' }),
        { msg, attempt: 1, traceId: 'trace-hdr' },
      );
      expect(channel.ack).toHaveBeenCalledWith(msg);
      expect(channel.nack).not.toHaveBeenCalled();
      expect(
        await metricValue((consumer as any).consumedCounter, serviceLabels),
      ).toBe(1);
      expect(
        await metricValue((consumer as any).retryCounter, serviceLabels),
      ).toBe(0);
    });

    it('falls back to the envelope trace id when no header is present', async () => {
      await consumer.onModuleInit();
      const envelope = makeEnvelope({ traceId: 'envelope-trace' });

      await (consumer as any).onMessage(makeMessage(envelope));

      expect(consumer.handleEventMock).toHaveBeenCalledWith(
        expect.objectContaining({ traceId: 'envelope-trace' }),
        expect.objectContaining({ attempt: 0, traceId: 'envelope-trace' }),
      );
    });

    it('uses the message id as deduplication key when the envelope has none', async () => {
      await consumer.onModuleInit();

      await (consumer as any).onMessage(makeMessage(makeEnvelope({ eventId: '' })));

      expect(consumer.isDuplicateMock).toHaveBeenCalledWith('msg-1');
    });

    it('falls back to unknown when no identifier is available', async () => {
      await consumer.onModuleInit();

      await (consumer as any).onMessage(
        makeMessage(makeEnvelope({ eventId: '' }), {}, { messageId: undefined }),
      );

      expect(consumer.isDuplicateMock).toHaveBeenCalledWith('unknown');
    });

    it('retries by republishing with an incremented retry header', async () => {
      consumer.handleEventMock.mockRejectedValue(new Error('db down'));
      await consumer.onModuleInit();
      const msg = makeMessage(makeEnvelope(), {
        [RabbitMQHeaders.RETRY_COUNT]: 0,
        [RabbitMQHeaders.TRACE_ID]: 'tr-1',
      });

      const pending = (consumer as any).onMessage(msg);
      await flush();

      expect(
        await metricValue((consumer as any).retryCounter, serviceLabels),
      ).toBe(1);
      expect((consumer as any).logger.warn).toHaveBeenCalledWith(
        `Retrying [${RabbitMQQueues.ORDERS_CREATED}] event=order.created attempt=1/2 delay=100ms: db down`,
      );
      expect(channel.sendToQueue).not.toHaveBeenCalled();

      await advance(100);
      await pending;

      expect(channel.sendToQueue).toHaveBeenCalledWith(
        RabbitMQQueues.ORDERS_CREATED,
        msg.content,
        expect.objectContaining({
          persistent: true,
          headers: expect.objectContaining({
            [RabbitMQHeaders.RETRY_COUNT]: 1,
            [RabbitMQHeaders.TRACE_ID]: 'tr-1',
          }),
        }),
      );
      expect(channel.ack).toHaveBeenCalledWith(msg);
      expect(channel.nack).not.toHaveBeenCalled();
    });

    it('doubles the retry delay on subsequent attempts', async () => {
      consumer.handleEventMock.mockRejectedValue(new Error('still failing'));
      await consumer.onModuleInit();
      const msg = makeMessage(makeEnvelope(), { [RabbitMQHeaders.RETRY_COUNT]: 1 });

      const pending = (consumer as any).onMessage(msg);
      await flush();

      expect((consumer as any).logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('attempt=2/2 delay=200ms'),
      );

      await advance(200);
      await pending;

      expect(channel.sendToQueue).toHaveBeenCalledWith(
        RabbitMQQueues.ORDERS_CREATED,
        msg.content,
        expect.objectContaining({
          headers: expect.objectContaining({ [RabbitMQHeaders.RETRY_COUNT]: 2 }),
        }),
      );
      expect(channel.ack).toHaveBeenCalledWith(msg);
    });

    it('dead-letters once the retry budget is exhausted', async () => {
      consumer.handleEventMock.mockRejectedValue(new Error('fatal'));
      await consumer.onModuleInit();
      const msg = makeMessage(makeEnvelope(), { [RabbitMQHeaders.RETRY_COUNT]: 2 });

      await (consumer as any).onMessage(msg);

      expect(channel.sendToQueue).not.toHaveBeenCalled();
      expect(
        await metricValue((consumer as any).failedCounter, {
          service: 'TestConsumer',
          queue: RabbitMQQueues.ORDERS_CREATED,
          reason: 'max_retries_exceeded',
        }),
      ).toBe(1);
      expect(
        await metricValue((consumer as any).dlqCounter, {
          service: 'TestConsumer',
          queue: RabbitMQQueues.ORDERS_CREATED,
        }),
      ).toBe(1);
      expect(channel.nack).toHaveBeenCalledWith(msg, false, false);
      expect((consumer as any).logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('dead-lettered'),
      );
    });

    it('abandons the retry when shutdown starts during the delay', async () => {
      consumer.handleEventMock.mockRejectedValue(new Error('db down'));
      await consumer.onModuleInit();
      const msg = makeMessage(makeEnvelope());

      const pending = (consumer as any).onMessage(msg);
      await flush();
      (consumer as any).closing = true;

      await advance(100);
      await pending;

      expect(channel.sendToQueue).not.toHaveBeenCalled();
      expect(channel.ack).not.toHaveBeenCalled();
      expect(channel.nack).not.toHaveBeenCalled();
    });

    it('abandons the retry when the channel disappears during the delay', async () => {
      consumer.handleEventMock.mockRejectedValue(new Error('db down'));
      await consumer.onModuleInit();
      const msg = makeMessage(makeEnvelope());

      const pending = (consumer as any).onMessage(msg);
      await flush();
      (consumer as any).channel = null;

      await advance(100);
      await pending;

      expect(channel.sendToQueue).not.toHaveBeenCalled();
      expect(channel.ack).not.toHaveBeenCalled();
    });

    it('nacks for requeue when the republish fails', async () => {
      channel.sendToQueue.mockImplementationOnce(() => {
        throw new Error('channel closed');
      });
      consumer.handleEventMock.mockRejectedValue(new Error('db down'));
      await consumer.onModuleInit();
      const msg = makeMessage(makeEnvelope());

      const pending = (consumer as any).onMessage(msg);
      await flush();
      await advance(100);
      await pending;

      expect(channel.nack).toHaveBeenCalledWith(msg, false, true);
      expect((consumer as any).logger.error).toHaveBeenCalledWith(
        expect.stringContaining('Retry republish failed'),
      );
    });

    it('handles messages that carry no header block at all', async () => {
      consumer.handleEventMock.mockRejectedValue(new Error('db down'));
      await consumer.onModuleInit();
      const msg = makeMessage(makeEnvelope(), undefined as any, { headers: undefined });

      const pending = (consumer as any).onMessage(msg);
      await flush();
      await advance(100);
      await pending;

      expect(consumer.handleEventMock).toHaveBeenCalledWith(
        expect.objectContaining({ eventId: 'evt-1' }),
        { msg, attempt: 0, traceId: undefined },
      );
      expect(channel.sendToQueue).toHaveBeenCalledWith(
        RabbitMQQueues.ORDERS_CREATED,
        msg.content,
        expect.objectContaining({
          headers: { [RabbitMQHeaders.RETRY_COUNT]: 1 },
        }),
      );
      expect(channel.ack).toHaveBeenCalledWith(msg);
    });

    it('returns without side effects when the channel is lost before failure handling', async () => {
      consumer.handleEventMock.mockImplementation(async () => {
        (consumer as any).channel = null;
        throw new Error('boom');
      });
      await consumer.onModuleInit();
      const msg = makeMessage(makeEnvelope());

      await (consumer as any).onMessage(msg);

      expect(channel.ack).not.toHaveBeenCalled();
      expect(channel.nack).not.toHaveBeenCalled();
      expect(channel.sendToQueue).not.toHaveBeenCalled();
      expect(
        await metricValue((consumer as any).retryCounter, serviceLabels),
      ).toBe(0);
    });
  });
});
