jest.mock('amqplib', () => ({ connect: jest.fn() }));

import { Logger } from '@nestjs/common';
import * as amqp from 'amqplib';
import * as client from 'prom-client';
import { RabbitMQExchanges, RabbitMQHeaders, RabbitMQQueues } from './rabbitmq.constants';
import {
  CircuitBreakerState,
  RabbitMQModuleOptions,
  RabbitMQService,
} from './rabbitmq.service';

const connectMock = amqp.connect as unknown as jest.Mock;

type Handler = (...args: any[]) => any;
type HandlerMap = Record<string, Handler>;

interface FakeChannel {
  channel: any;
  handlers: HandlerMap;
}

interface FakeConnection {
  connection: any;
  confirmChannel: any;
  consumerChannel: any;
  connectionHandlers: HandlerMap;
  confirmHandlers: HandlerMap;
}

function createFakeChannel(): FakeChannel {
  const handlers: HandlerMap = {};
  const channel: any = {
    publish: jest.fn().mockReturnValue(true),
    waitForConfirms: jest.fn().mockResolvedValue(undefined),
    assertExchange: jest.fn().mockResolvedValue(undefined),
    assertQueue: jest.fn().mockResolvedValue(undefined),
    bindQueue: jest.fn().mockResolvedValue(undefined),
    prefetch: jest.fn().mockResolvedValue(undefined),
    consume: jest.fn().mockResolvedValue({ consumerTag: 'ctag-0' }),
    ack: jest.fn(),
    nack: jest.fn(),
    sendToQueue: jest.fn().mockReturnValue(true),
    cancel: jest.fn().mockResolvedValue(undefined),
    close: jest.fn().mockResolvedValue(undefined),
    on: jest.fn((event: string, cb: Handler) => {
      handlers[event] = cb;
      return channel;
    }),
  };
  return { channel, handlers };
}

function createFakeConnection(): FakeConnection {
  const confirm = createFakeChannel();
  const consumer = createFakeChannel();
  const connectionHandlers: HandlerMap = {};
  const connection: any = {
    createConfirmChannel: jest.fn().mockResolvedValue(confirm.channel),
    createChannel: jest.fn().mockResolvedValue(consumer.channel),
    close: jest.fn().mockResolvedValue(undefined),
    on: jest.fn((event: string, cb: Handler) => {
      connectionHandlers[event] = cb;
    }),
  };
  return {
    connection,
    confirmChannel: confirm.channel,
    consumerChannel: consumer.channel,
    connectionHandlers,
    confirmHandlers: confirm.handlers,
  };
}

const connections: FakeConnection[] = [];

function registerConnection(): any {
  const fake = createFakeConnection();
  connections.push(fake);
  return fake.connection;
}

function lastConnection(): FakeConnection {
  return connections[connections.length - 1];
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

function createService(options?: RabbitMQModuleOptions): RabbitMQService {
  return new RabbitMQService(options);
}

const SERVICE_LABEL = 'delivery-service';

let logSpy: jest.SpyInstance;
let warnSpy: jest.SpyInstance;
let errorSpy: jest.SpyInstance;
let debugSpy: jest.SpyInstance;

beforeEach(() => {
  client.register.clear();
  jest.useFakeTimers();
  connections.length = 0;
  connectMock.mockReset();
  connectMock.mockImplementation(async () => registerConnection());
  logSpy = jest.spyOn(Logger.prototype, 'log').mockImplementation(() => undefined);
  warnSpy = jest.spyOn(Logger.prototype, 'warn').mockImplementation(() => undefined);
  errorSpy = jest.spyOn(Logger.prototype, 'error').mockImplementation(() => undefined);
  debugSpy = jest.spyOn(Logger.prototype, 'debug').mockImplementation(() => undefined);
});

afterEach(() => {
  jest.restoreAllMocks();
  jest.useRealTimers();
});

describe('RabbitMQService', () => {
  describe('configuration', () => {
    it('connects with the explicit url and masks credentials in logs', async () => {
      const service = createService({ url: 'amqp://user:secret@broker:5672/' });

      await service.onModuleInit();

      expect(connectMock).toHaveBeenCalledWith('amqp://user:secret@broker:5672/', {
        timeout: 10000,
      });
      expect(logSpy).toHaveBeenCalledWith('RabbitMQ connected (amqp://user:***@broker:5672/)');
      expect(service.isConnected()).toBe(true);
    });

    it('falls back to RABBITMQ_URL and RABBITMQ_CLIENT_ID environment variables', async () => {
      process.env.RABBITMQ_URL = 'amqp://env-host:5672/';
      process.env.RABBITMQ_CLIENT_ID = 'env-service';
      try {
        const service = createService();

        await service.onModuleInit();

        expect(connectMock).toHaveBeenCalledWith('amqp://env-host:5672/', { timeout: 10000 });
        expect((service as any).serviceName).toBe('env-service');
      } finally {
        delete process.env.RABBITMQ_URL;
        delete process.env.RABBITMQ_CLIENT_ID;
      }
    });

    it('uses the built-in defaults when nothing is configured', async () => {
      delete process.env.RABBITMQ_URL;
      delete process.env.RABBITMQ_CLIENT_ID;
      const service = createService();

      await service.onModuleInit();

      expect(connectMock).toHaveBeenCalledWith('amqp://guest:guest@localhost:5672/', {
        timeout: 10000,
      });
      expect((service as any).serviceName).toBe('delivery-service');
    });

    it('prefers an explicit service name over the environment', () => {
      process.env.RABBITMQ_CLIENT_ID = 'env-service';
      try {
        const service = createService({ serviceName: 'explicit-service' });

        expect((service as any).serviceName).toBe('explicit-service');
      } finally {
        delete process.env.RABBITMQ_CLIENT_ID;
      }
    });

    it('applies the configured retry and breaker options', () => {
      const service = createService({
        maxReconnectAttempts: 4,
        reconnectBaseDelayMs: 50,
        circuitBreakerFailureThreshold: 3,
        circuitBreakerResetTimeoutMs: 1000,
      });

      expect((service as any).maxReconnectAttempts).toBe(4);
      expect((service as any).reconnectBaseDelayMs).toBe(50);
      expect((service as any).failureThreshold).toBe(3);
      expect((service as any).resetTimeoutMs).toBe(1000);
      expect(service.getCircuitBreakerState()).toBe(CircuitBreakerState.CLOSED);
    });
  });

  describe('lifecycle', () => {
    it('connects on module init and exposes the connection', async () => {
      const service = createService();

      expect(service.isConnected()).toBe(false);
      expect(service.getConnection()).toBeNull();

      await service.onModuleInit();

      expect(connectMock).toHaveBeenCalledTimes(1);
      expect(service.isConnected()).toBe(true);
      const conn = lastConnection();
      expect(service.getConnection()).toBe(conn.connection);
      expect(conn.connection.createConfirmChannel).toHaveBeenCalled();
    });

    it('retries with exponential backoff until the broker is reachable', async () => {
      const service = createService({ maxReconnectAttempts: 3, reconnectBaseDelayMs: 1000 });
      connectMock.mockRejectedValueOnce(new Error('ECONNREFUSED'));

      const init = service.onModuleInit();
      await flush();
      expect(connectMock).toHaveBeenCalledTimes(1);

      await advance(999);
      expect(connectMock).toHaveBeenCalledTimes(1);

      await advance(1);
      expect(connectMock).toHaveBeenCalledTimes(2);

      await init;

      expect(service.isConnected()).toBe(true);
      expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('attempt 1/3'));
      expect(
        await metricValue((service as any).connectionErrorsCounter, {
          service: SERVICE_LABEL,
        }),
      ).toBe(1);
    });

    it('caps the backoff at 30s and reports when retries are exhausted', async () => {
      const service = createService({ maxReconnectAttempts: 3, reconnectBaseDelayMs: 20000 });
      connectMock.mockRejectedValue(new Error('ECONNREFUSED'));

      const init = service.onModuleInit();
      await flush();
      expect(connectMock).toHaveBeenCalledTimes(1);

      await advance(20000);
      expect(connectMock).toHaveBeenCalledTimes(2);

      await advance(30000);
      expect(connectMock).toHaveBeenCalledTimes(3);

      await init;

      expect(service.isConnected()).toBe(false);
      expect(errorSpy).toHaveBeenCalledWith('RabbitMQ failed to connect after maximum retries');
      expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('attempt 3/3'));
      expect(jest.getTimerCount()).toBe(0);
      expect(
        await metricValue((service as any).connectionErrorsCounter, {
          service: SERVICE_LABEL,
        }),
      ).toBe(3);
    });

    it('rejects publishes when the broker can never be reached', async () => {
      const service = createService({ maxReconnectAttempts: 1 });
      connectMock.mockRejectedValue(new Error('ECONNREFUSED'));

      await service.onModuleInit();

      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow(
        'RabbitMQ is not connected',
      );
      expect(service.isConnected()).toBe(false);
    });

    it('closes the confirm channel and connection on shutdown', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      await service.onModuleDestroy();

      expect(conn.confirmChannel.close).toHaveBeenCalled();
      expect(conn.connection.close).toHaveBeenCalled();
      expect(service.isConnected()).toBe(false);
      expect(service.getConnection()).toBeNull();
    });

    it('tolerates close errors during shutdown', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();
      conn.confirmChannel.close.mockRejectedValue(new Error('broker gone'));
      conn.connection.close.mockRejectedValue(new Error('broker gone'));

      await expect(service.onModuleDestroy()).resolves.toBeUndefined();

      expect(service.getConnection()).toBeNull();
      expect(service.isConnected()).toBe(false);
    });

    it('schedules a reconnect when the connection closes unexpectedly', async () => {
      const service = createService();
      await service.onModuleInit();
      const first = lastConnection();

      first.connectionHandlers['close']();
      await flush();

      expect(connectMock).toHaveBeenCalledTimes(2);
      expect(service.isConnected()).toBe(true);
      expect(warnSpy).toHaveBeenCalledWith('RabbitMQ connection closed — reconnecting...');
    });

    it('does not reconnect after shutdown', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      await service.onModuleDestroy();
      conn.connectionHandlers['close']();
      await flush();

      expect(connectMock).toHaveBeenCalledTimes(1);
      expect(
        await metricValue((service as any).connectionErrorsCounter, {
          service: SERVICE_LABEL,
        }),
      ).toBe(0);
    });

    it('ignores close events while a reconnect is already in flight', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      (service as any).reconnecting = true;
      conn.connectionHandlers['close']();
      await flush();

      expect(connectMock).toHaveBeenCalledTimes(1);
      expect(service.isConnected()).toBe(false);
      expect(
        await metricValue((service as any).connectionErrorsCounter, {
          service: SERVICE_LABEL,
        }),
      ).toBe(0);

      (service as any).reconnecting = false;
    });

    it('tracks connection and confirm channel error events', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      conn.connectionHandlers['error'](new Error('socket hang up'));
      conn.confirmHandlers['error'](new Error('channel died'));

      expect(errorSpy).toHaveBeenCalledWith('RabbitMQ connection error: socket hang up');
      expect(errorSpy).toHaveBeenCalledWith('RabbitMQ confirm channel error: channel died');
      expect(
        await metricValue((service as any).connectionErrorsCounter, {
          service: SERVICE_LABEL,
        }),
      ).toBe(2);
    });

    it('skips a new reconnect cycle while shutting down or already reconnecting', async () => {
      const service = createService();
      await service.onModuleInit();

      (service as any).closing = true;
      await (service as any).scheduleReconnect();
      expect(connectMock).toHaveBeenCalledTimes(1);

      (service as any).closing = false;
      (service as any).reconnecting = true;
      await (service as any).scheduleReconnect();
      expect(connectMock).toHaveBeenCalledTimes(1);
      expect(service.isConnected()).toBe(true);
      (service as any).reconnecting = false;
    });
  });

  describe('publish', () => {
    it('publishes a JSON envelope and waits for confirms', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      await service.publish('delivery.orders.topic', 'order.created', 'order.created', {
        id: 7,
      });

      expect(conn.confirmChannel.publish).toHaveBeenCalledTimes(1);
      const [exchange, routingKey, buffer, opts] = conn.confirmChannel.publish.mock.calls[0];
      expect(exchange).toBe('delivery.orders.topic');
      expect(routingKey).toBe('order.created');

      const envelope = JSON.parse(buffer.toString());
      expect(envelope).toMatchObject({
        eventType: 'order.created',
        eventVersion: 1,
        aggregateType: 'order',
        aggregateId: '',
        producer: SERVICE_LABEL,
        payload: { id: 7 },
      });
      expect(typeof envelope.eventId).toBe('string');
      expect(new Date(envelope.occurredAt).getTime()).toBe(envelope.timestamp);
      expect(opts).toMatchObject({
        contentType: 'application/json',
        persistent: true,
        messageId: envelope.eventId,
        timestamp: envelope.timestamp,
      });
      expect(opts.priority).toBeUndefined();
      expect(opts.headers[RabbitMQHeaders.TRACE_ID]).toBe(envelope.eventId);
      expect(opts.headers['x-correlation-id']).toBeUndefined();

      expect(conn.confirmChannel.waitForConfirms).toHaveBeenCalled();
      expect(
        await metricValue((service as any).publishedCounter, {
          service: SERVICE_LABEL,
          exchange: 'delivery.orders.topic',
          routing_key: 'order.created',
        }),
      ).toBe(1);
      expect(debugSpy).toHaveBeenCalledWith(expect.stringContaining('order.created'));
    });

    it('merges explicit publish options into the envelope and message', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      await service.publish('ex', 'rk', 'delivery.order.created', { id: 1 }, {
        persistent: false,
        priority: 7,
        traceId: 'trace-1',
        correlationId: 'corr-1',
        causationId: 'cause-1',
        aggregateType: 'custom-agg',
        aggregateId: 'agg-1',
        producer: 'orders-service',
        headers: { 'x-custom': 'v' },
      });

      const [exchange, routingKey, buffer, opts] = conn.confirmChannel.publish.mock.calls[0];
      expect(exchange).toBe('ex');
      expect(routingKey).toBe('rk');

      const envelope = JSON.parse(buffer.toString());
      expect(envelope).toMatchObject({
        aggregateType: 'custom-agg',
        aggregateId: 'agg-1',
        producer: 'orders-service',
        correlationId: 'corr-1',
        causationId: 'cause-1',
        traceId: 'trace-1',
      });
      expect(opts.persistent).toBe(false);
      expect(opts.priority).toBe(7);
      expect(opts.headers).toEqual({
        'x-custom': 'v',
        [RabbitMQHeaders.TRACE_ID]: 'trace-1',
        'x-correlation-id': 'corr-1',
      });
    });

    it('records and rethrows publish failures', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();
      conn.confirmChannel.waitForConfirms.mockRejectedValue(new Error('confirm timeout'));

      await expect(
        service.publish('ex', 'rk', 'evt', {}),
      ).rejects.toThrow('confirm timeout');

      expect(service.getCircuitBreakerState()).toBe(CircuitBreakerState.CLOSED);
      expect(
        await metricValue((service as any).failedCounter, {
          service: SERVICE_LABEL,
          exchange: 'ex',
          routing_key: 'rk',
          reason: 'publish_error',
        }),
      ).toBe(1);
      expect(errorSpy).toHaveBeenCalledWith(
        'RabbitMQ publish failed [ex] rk=<rk>: confirm timeout',
      );
    });

    it('records synchronous channel publish errors', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();
      conn.confirmChannel.publish.mockImplementationOnce(() => {
        throw new Error('channel closed');
      });

      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow('channel closed');

      expect(
        await metricValue((service as any).failedCounter, {
          service: SERVICE_LABEL,
          exchange: 'ex',
          routing_key: 'rk',
          reason: 'publish_error',
        }),
      ).toBe(1);
      expect(conn.confirmChannel.waitForConfirms).not.toHaveBeenCalled();
    });

    it('refuses to publish while shutting down', async () => {
      const service = createService();
      await service.onModuleInit();
      await service.onModuleDestroy();

      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow(
        'RabbitMQ service is shutting down',
      );
    });
  });

  describe('circuit breaker', () => {
    it('starts closed with a gauge reporting the closed state', async () => {
      const service = createService();

      expect(service.getCircuitBreakerState()).toBe(CircuitBreakerState.CLOSED);
      expect(
        await metricValue((service as any).breakerStateGauge, { service: SERVICE_LABEL }),
      ).toBe(CircuitBreakerState.CLOSED);
    });

    it('opens after the failure threshold and rejects subsequent publishes fail-fast', async () => {
      const service = createService({ circuitBreakerFailureThreshold: 2 });
      await service.onModuleInit();
      const conn = lastConnection();
      conn.confirmChannel.waitForConfirms.mockRejectedValue(new Error('confirm failed'));

      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow('confirm failed');
      expect(service.getCircuitBreakerState()).toBe(CircuitBreakerState.CLOSED);

      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow('confirm failed');
      expect(service.getCircuitBreakerState()).toBe(CircuitBreakerState.OPEN);
      expect(
        await metricValue((service as any).breakerStateGauge, { service: SERVICE_LABEL }),
      ).toBe(CircuitBreakerState.OPEN);
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining('RabbitMQ circuit breaker → open'),
      );

      const publishCalls = conn.confirmChannel.publish.mock.calls.length;
      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow(
        'RabbitMQ circuit breaker is OPEN — publish rejected (fail-fast)',
      );
      expect(conn.confirmChannel.publish.mock.calls.length).toBe(publishCalls);
      expect(
        await metricValue((service as any).failedCounter, {
          service: SERVICE_LABEL,
          exchange: 'ex',
          routing_key: 'rk',
          reason: 'circuit_open',
        }),
      ).toBe(1);
    });

    it('resets consecutive failures after a successful publish', async () => {
      const service = createService({ circuitBreakerFailureThreshold: 2 });
      await service.onModuleInit();
      const conn = lastConnection();

      conn.confirmChannel.waitForConfirms.mockRejectedValueOnce(new Error('boom'));
      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow('boom');
      await service.publish('ex', 'rk', 'evt', {});
      conn.confirmChannel.waitForConfirms.mockRejectedValueOnce(new Error('boom'));
      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow('boom');

      expect(service.getCircuitBreakerState()).toBe(CircuitBreakerState.CLOSED);
    });

    it('half-opens after the reset timeout and closes on a successful trial publish', async () => {
      const service = createService({
        circuitBreakerFailureThreshold: 1,
        circuitBreakerResetTimeoutMs: 30000,
      });
      await service.onModuleInit();
      const conn = lastConnection();
      conn.confirmChannel.waitForConfirms.mockRejectedValueOnce(new Error('boom'));
      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow('boom');
      expect(service.getCircuitBreakerState()).toBe(CircuitBreakerState.OPEN);

      (service as any).breakerOpenedAt = Date.now() - 30000;

      await service.publish('ex', 'rk', 'evt', {});

      expect(service.getCircuitBreakerState()).toBe(CircuitBreakerState.CLOSED);
      expect(logSpy).toHaveBeenCalledWith(
        'RabbitMQ circuit breaker → half-open (trial publish allowed)',
      );
      expect(logSpy).toHaveBeenCalledWith('RabbitMQ circuit breaker → closed');
      expect(
        await metricValue((service as any).breakerStateGauge, { service: SERVICE_LABEL }),
      ).toBe(CircuitBreakerState.CLOSED);
    });

    it('reopens when the half-open trial publish fails', async () => {
      const service = createService({ circuitBreakerFailureThreshold: 1 });
      await service.onModuleInit();
      const conn = lastConnection();
      conn.confirmChannel.waitForConfirms.mockRejectedValueOnce(new Error('boom'));
      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow('boom');
      (service as any).breakerOpenedAt = Date.now() - 30000;

      conn.confirmChannel.waitForConfirms.mockRejectedValueOnce(new Error('still broken'));
      await expect(service.publish('ex', 'rk', 'evt', {})).rejects.toThrow('still broken');

      expect(service.getCircuitBreakerState()).toBe(CircuitBreakerState.OPEN);
      expect(
        await metricValue((service as any).breakerStateGauge, { service: SERVICE_LABEL }),
      ).toBe(CircuitBreakerState.OPEN);
      expect(warnSpy).toHaveBeenCalledWith(
        expect.stringContaining('RabbitMQ circuit breaker → open'),
      );
    });
  });

  describe('buildEnvelope', () => {
    it('derives the aggregate type from the event type', async () => {
      const service = createService();

      const dotted = service.buildEnvelope('order.created', { id: 1 });
      const plain = service.buildEnvelope('custom-event', { id: 1 });

      expect(dotted.aggregateType).toBe('order');
      expect(plain.aggregateType).toBe('custom-event');
      expect(dotted.eventVersion).toBe(1);
      expect(dotted.producer).toBe(SERVICE_LABEL);
      expect(dotted.aggregateId).toBe('');
      expect(dotted.correlationId).toBeUndefined();
      expect(dotted.payload).toEqual({ id: 1 });
      expect(dotted.eventId).not.toBe(plain.eventId);
      expect(typeof dotted.timestamp).toBe('number');
    });

    it('honours explicit envelope options', async () => {
      const service = createService();

      const envelope = service.buildEnvelope('order.created', { id: 1 }, {
        aggregateType: 'agg',
        aggregateId: 'agg-1',
        producer: 'svc-x',
        correlationId: 'corr-1',
        causationId: 'cause-1',
        traceId: 'trace-1',
      });

      expect(envelope).toMatchObject({
        aggregateType: 'agg',
        aggregateId: 'agg-1',
        producer: 'svc-x',
        correlationId: 'corr-1',
        causationId: 'cause-1',
        traceId: 'trace-1',
        eventType: 'order.created',
      });
    });
  });

  describe('ensureTopology', () => {
    it('declares deduplicated exchanges, queues and bindings', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      expect(service.isTopologyAsserted()).toBe(false);

      await service.ensureTopology([
        {
          exchange: 'delivery.orders.topic',
          exchangeType: 'topic',
          queue: 'orders.created.queue',
          routingKey: 'order.#',
        },
        {
          exchange: 'delivery.orders.topic',
          queue: 'orders.status.queue',
          routingKey: 'order.status',
        },
        {
          exchange: RabbitMQExchanges.DLX_DIRECT,
          queue: RabbitMQQueues.DLQ_ORDERS,
          routingKey: 'orders.dlq',
        },
      ]);

      expect(conn.confirmChannel.assertExchange).toHaveBeenCalledTimes(2);
      expect(conn.confirmChannel.assertExchange).toHaveBeenCalledWith(
        'delivery.orders.topic',
        'topic',
        { durable: true, arguments: { 'alternate-exchange': RabbitMQExchanges.UNROUTABLE } },
      );
      expect(conn.confirmChannel.assertExchange).toHaveBeenCalledWith(
        RabbitMQExchanges.DLX_DIRECT,
        'direct',
        { durable: true, arguments: {} },
      );
      expect(conn.confirmChannel.assertQueue).toHaveBeenCalledWith('orders.created.queue', {
        durable: true,
        arguments: {
          'x-queue-type': 'quorum',
          'x-dead-letter-exchange': 'delivery.dlx.direct',
          'x-dead-letter-routing-key': 'orders.dlq',
        },
      });
      expect(conn.confirmChannel.bindQueue).toHaveBeenCalledTimes(3);
      expect(conn.confirmChannel.bindQueue).toHaveBeenNthCalledWith(
        1,
        'orders.created.queue',
        'delivery.orders.topic',
        'order.#',
      );
      expect(service.isTopologyAsserted()).toBe(true);
    });

    it('resolves exchange types from the constant map', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      await service.ensureTopology([
        {
          exchange: RabbitMQExchanges.NOTIFICATIONS,
          queue: 'notifications.email.queue',
          routingKey: 'notification.#',
        },
      ]);

      expect(conn.confirmChannel.assertExchange).toHaveBeenCalledWith(
        RabbitMQExchanges.NOTIFICATIONS,
        'fanout',
        { durable: true, arguments: { 'alternate-exchange': RabbitMQExchanges.UNROUTABLE } },
      );
    });

    it('defaults unknown exchanges to topic', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      await service.ensureTopology([
        { exchange: 'custom.exchange', queue: 'custom.queue', routingKey: 'custom.key' },
      ]);

      expect(conn.confirmChannel.assertExchange).toHaveBeenCalledWith('custom.exchange', 'topic', {
        durable: true,
        arguments: { 'alternate-exchange': RabbitMQExchanges.UNROUTABLE },
      });
    });

    it('applies DLQ and stream queue arguments', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      await service.ensureTopology([
        { exchange: 'ex', queue: RabbitMQQueues.DLQ_ORDERS, routingKey: 'k' },
        { exchange: 'ex', queue: RabbitMQQueues.ANALYTICS_EVENTS, routingKey: 'k' },
      ]);

      expect(conn.confirmChannel.assertQueue).toHaveBeenCalledWith(RabbitMQQueues.DLQ_ORDERS, {
        durable: true,
        arguments: { 'x-queue-type': 'quorum', 'x-message-ttl': 604800000 },
      });
      expect(conn.confirmChannel.assertQueue).toHaveBeenCalledWith(
        RabbitMQQueues.ANALYTICS_EVENTS,
        {
          durable: true,
          arguments: {
            'x-queue-type': 'stream',
            'x-max-age': '7D',
            'x-stream-max-segment-size-bytes': 50000000,
          },
        },
      );
    });

    it('throws when the broker is unreachable', async () => {
      const service = createService({ maxReconnectAttempts: 1 });
      connectMock.mockRejectedValue(new Error('ECONNREFUSED'));

      await expect(
        service.ensureTopology([{ exchange: 'ex', queue: 'q', routingKey: 'k' }]),
      ).rejects.toThrow('RabbitMQ is not connected');
      expect(service.isTopologyAsserted()).toBe(false);
    });

    it('clears the topology flag after a reconnect', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();
      await service.ensureTopology([{ exchange: 'ex', queue: 'q', routingKey: 'k' }]);
      expect(service.isTopologyAsserted()).toBe(true);

      conn.connectionHandlers['close']();
      await flush();

      expect(service.isTopologyAsserted()).toBe(false);
      expect(service.isConnected()).toBe(true);
    });
  });

  describe('channels', () => {
    it('creates a consumer channel with the default prefetch', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      const channel = await service.createConsumerChannel();

      expect(conn.connection.createChannel).toHaveBeenCalled();
      expect(conn.consumerChannel.prefetch).toHaveBeenCalledWith(10);
      expect(channel).toBe(conn.consumerChannel);
    });

    it('creates a consumer channel with a custom prefetch', async () => {
      const service = createService();
      await service.onModuleInit();
      const conn = lastConnection();

      await service.createConsumerChannel(50);

      expect(conn.consumerChannel.prefetch).toHaveBeenCalledWith(50);
    });

    it('returns null before connecting', () => {
      const service = createService();

      expect(service.getConnection()).toBeNull();
      expect(service.isConnected()).toBe(false);
      expect(service.isTopologyAsserted()).toBe(false);
    });
  });
});
