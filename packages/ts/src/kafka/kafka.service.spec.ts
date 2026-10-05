jest.mock('kafkajs', () => {
  const producer = {
    connect: jest.fn(),
    disconnect: jest.fn(),
    send: jest.fn(),
  };
  const consumer = {
    connect: jest.fn(),
    disconnect: jest.fn(),
    subscribe: jest.fn(),
    run: jest.fn(),
  };
  const kafkaInstance = {
    producer: jest.fn(() => producer),
    consumer: jest.fn(() => consumer),
    admin: jest.fn(),
  };
  return {
    Kafka: jest.fn(() => kafkaInstance),
    __mocks: { producer, consumer, kafkaInstance },
  };
});

import { Logger } from '@nestjs/common';
import { KafkaService } from './kafka.service';

const kafkajs = jest.requireMock('kafkajs');
const KafkaMock = kafkajs.Kafka as jest.Mock;
const { producer, consumer, kafkaInstance } = kafkajs.__mocks;

let logSpy: jest.SpyInstance;
let debugSpy: jest.SpyInstance;
let errorSpy: jest.SpyInstance;
let warnSpy: jest.SpyInstance;

beforeAll(() => {
  logSpy = jest.spyOn(Logger.prototype, 'log').mockImplementation();
  debugSpy = jest.spyOn(Logger.prototype, 'debug').mockImplementation();
  errorSpy = jest.spyOn(Logger.prototype, 'error').mockImplementation();
  warnSpy = jest.spyOn(Logger.prototype, 'warn').mockImplementation();
});

beforeEach(() => {
  jest.clearAllMocks();

  producer.connect.mockResolvedValue(undefined);
  producer.send.mockResolvedValue(undefined);
  producer.disconnect.mockResolvedValue(undefined);
  consumer.connect.mockResolvedValue(undefined);
  consumer.subscribe.mockResolvedValue(undefined);
  consumer.run.mockResolvedValue(undefined);
  consumer.disconnect.mockResolvedValue(undefined);
});

describe('KafkaService', () => {
  describe('constructor configuration', () => {
    it('uses default clientId and brokers when no options are given', () => {
      new KafkaService({});

      expect(KafkaMock).toHaveBeenCalledWith({
        clientId: 'delivery-service',
        brokers: ['localhost:9092'],
      });
    });

    it('uses the provided clientId and brokers', () => {
      new KafkaService({ clientId: 'tracking-svc', brokers: ['k1:9092', 'k2:9092'] });

      expect(KafkaMock).toHaveBeenCalledWith({
        clientId: 'tracking-svc',
        brokers: ['k1:9092', 'k2:9092'],
      });
    });

    it('falls back to default brokers when the brokers array is empty', () => {
      new KafkaService({ brokers: [] });

      expect(KafkaMock).toHaveBeenCalledWith({
        clientId: 'delivery-service',
        brokers: ['localhost:9092'],
      });
    });

    it('includes ssl, sasl and connectionTimeout when provided', () => {
      new KafkaService({
        clientId: 'secure-svc',
        brokers: ['k:9092'],
        ssl: true,
        sasl: { mechanism: 'plain', username: 'user', password: 'pass' },
        connectionTimeout: 3000,
      });

      expect(KafkaMock).toHaveBeenCalledWith({
        clientId: 'secure-svc',
        brokers: ['k:9092'],
        ssl: true,
        sasl: { mechanism: 'plain', username: 'user', password: 'pass' },
        connectionTimeout: 3000,
      });
    });
  });

  describe('client accessors', () => {
    it('returns the underlying kafka client', () => {
      const service = new KafkaService({});

      expect(service.getClient()).toBe(kafkaInstance);
    });

    it('creates a consumer bound to the given group id', () => {
      const service = new KafkaService({});

      expect(service.consumer('group-1')).toBe(consumer);
      expect(kafkaInstance.consumer).toHaveBeenCalledWith({ groupId: 'group-1' });
    });
  });

  describe('buildEnvelope', () => {
    let service: KafkaService;

    beforeEach(() => {
      service = new KafkaService({});
    });

    it('builds an envelope with derived defaults', () => {
      const envelope = service.buildEnvelope('user.created', { id: 7 });

      expect(envelope.eventId).toMatch(/^[0-9a-fA-F-]{36}$/);
      expect(envelope.eventType).toBe('user.created');
      expect(envelope.eventVersion).toBe(1);
      expect(new Date(envelope.occurredAt).getTime()).toBe(envelope.timestamp);
      expect(envelope.aggregateType).toBe('user');
      expect(envelope.aggregateId).toBe('');
      expect(envelope.producer).toBe('delivery-service');
      expect(envelope.correlationId).toBeUndefined();
      expect(envelope.causationId).toBeUndefined();
      expect(envelope.traceId).toBeUndefined();
      expect(envelope.payload).toEqual({ id: 7 });
    });

    it('uses the configured clientId as producer when no explicit producer is given', () => {
      const configured = new KafkaService({ clientId: 'orders-svc' });

      const envelope = configured.buildEnvelope('x.y', {});

      expect(envelope.producer).toBe('orders-svc');
    });

    it('prefers explicit options over derived values', () => {
      const envelope = service.buildEnvelope('delivery.picked_up', {}, {
        producer: 'custom-producer',
        aggregateType: 'Delivery',
        aggregateId: 'agg-1',
        key: 'key-1',
        correlationId: 'corr-1',
        causationId: 'caus-1',
        traceId: 'trace-1',
      });

      expect(envelope.producer).toBe('custom-producer');
      expect(envelope.aggregateType).toBe('Delivery');
      expect(envelope.aggregateId).toBe('agg-1');
      expect(envelope.correlationId).toBe('corr-1');
      expect(envelope.causationId).toBe('caus-1');
      expect(envelope.traceId).toBe('trace-1');
    });

    it('keeps the whole event type as aggregate type when it has no dot and falls back to the key', () => {
      const envelope = service.buildEnvelope('noDotEvent', {}, { key: 'k-42' });

      expect(envelope.aggregateType).toBe('noDotEvent');
      expect(envelope.aggregateId).toBe('k-42');
    });
  });

  describe('emit', () => {
    let service: KafkaService;

    beforeEach(() => {
      service = new KafkaService({ clientId: 'emit-svc' });
    });

    it('sends the envelope to the topic and logs success', async () => {
      await service.emit('orders.created', 'order.created', { total: 10 }, {
        key: 'order-1',
        partition: 2,
        headers: { 'x-trace': 'abc' },
      });

      expect(producer.connect).toHaveBeenCalledTimes(1);
      expect(producer.send).toHaveBeenCalledTimes(1);
      const record = producer.send.mock.calls[0][0];
      expect(record.topic).toBe('orders.created');
      expect(record.messages).toHaveLength(1);
      const message = record.messages[0];
      expect(message.key).toBe('order-1');
      expect(message.partition).toBe(2);
      expect(message.headers).toEqual({ 'x-trace': 'abc' });
      const envelope = JSON.parse(message.value);
      expect(envelope.eventType).toBe('order.created');
      expect(envelope.producer).toBe('emit-svc');
      expect(envelope.payload).toEqual({ total: 10 });
      expect(debugSpy).toHaveBeenCalledWith(
        'Event emitted to Kafka topic [orders.created]: order.created',
      );
    });

    it('connects the producer once and reuses it for subsequent emits', async () => {
      await service.emit('a.b', 'a.b', {});
      await service.emit('a.b', 'a.b', {});

      expect(kafkaInstance.producer).toHaveBeenCalledTimes(1);
      expect(producer.connect).toHaveBeenCalledTimes(1);
      expect(producer.send).toHaveBeenCalledTimes(2);
      expect(logSpy).toHaveBeenCalledWith('Kafka producer connected');
    });

    it('propagates producer connection failures', async () => {
      producer.connect.mockRejectedValue(new Error('sasl auth failed'));

      await expect(service.emit('a.b', 'a.b', {})).rejects.toThrow('sasl auth failed');

      expect(producer.send).not.toHaveBeenCalled();
      expect(debugSpy).not.toHaveBeenCalled();
    });

    it('propagates send failures', async () => {
      await service.emit('a.b', 'a.b', {});
      debugSpy.mockClear();
      producer.send.mockRejectedValue(new Error('not leader for partition'));

      await expect(service.emit('a.b', 'a.b', {})).rejects.toThrow('not leader for partition');

      expect(debugSpy).not.toHaveBeenCalled();
    });
  });

  describe('onModuleDestroy', () => {
    it('disconnects the producer and reconnects it on the next emit', async () => {
      const service = new KafkaService({});
      await service.emit('a.b', 'a.b', {});

      await service.onModuleDestroy();
      expect(producer.disconnect).toHaveBeenCalledTimes(1);

      await service.emit('a.b', 'a.b', {});
      expect(kafkaInstance.producer).toHaveBeenCalledTimes(2);
      expect(producer.connect).toHaveBeenCalledTimes(2);
    });

    it('does nothing when no producer was ever created', async () => {
      const service = new KafkaService({});

      await service.onModuleDestroy();

      expect(producer.disconnect).not.toHaveBeenCalled();
      expect(kafkaInstance.producer).not.toHaveBeenCalled();
    });
  });
});
