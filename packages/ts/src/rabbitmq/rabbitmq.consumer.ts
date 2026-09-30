import { Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import * as amqp from 'amqplib';
import * as client from 'prom-client';
import {
  RABBITMQ_EXCHANGE_TYPES,
  RabbitMQExchanges,
  RabbitMQHeaders,
  buildQueueArguments,
  resolvePrefetch,
} from './rabbitmq.constants';
import { RabbitMQEventEnvelope, RabbitMQService } from './rabbitmq.service';

export interface RabbitMQConsumeContext {
  msg: amqp.Message;
  attempt: number;
  traceId?: string;
}

export abstract class BaseRabbitMQConsumer implements OnModuleInit, OnModuleDestroy {
  protected abstract readonly queue: string;
  protected abstract readonly exchange: string;
  protected abstract readonly routingKeys: string[];
  protected abstract readonly logger: any;

  protected readonly prefetch: number = 10;
  protected readonly maxRetries = 3;
  protected readonly retryDelayMs = 2000;

  private channel: amqp.Channel | null = null;
  private consumerTag: string | null = null;
  private closing = false;
  protected connected = false;

  private consumedCounter: client.Counter<string>;
  private failedCounter: client.Counter<string>;
  private dlqCounter: client.Counter<string>;
  private retryCounter: client.Counter<string>;

  constructor(protected readonly rabbitmq: RabbitMQService) {
    const registry = client.register;
    const getOrCreate = <T>(name: string, create: () => T): T => {
      const existing = registry.getSingleMetric(name) as unknown as T | undefined;
      return existing ?? create();
    };
    this.consumedCounter = getOrCreate('rabbitmq_messages_consumed_total', () =>
      new client.Counter({
        name: 'rabbitmq_messages_consumed_total',
        help: 'Total RabbitMQ messages consumed and acked',
        labelNames: ['service', 'queue'],
        registers: [registry],
      }),
    );
    this.failedCounter = getOrCreate('rabbitmq_consumer_messages_failed_total', () =>
      new client.Counter({
        name: 'rabbitmq_consumer_messages_failed_total',
        help: 'Total RabbitMQ consumer processing failures',
        labelNames: ['service', 'queue', 'reason'],
        registers: [registry],
      }),
    );
    this.dlqCounter = getOrCreate('rabbitmq_messages_dlq_total', () =>
      new client.Counter({
        name: 'rabbitmq_messages_dlq_total',
        help: 'Total RabbitMQ messages routed to a DLQ',
        labelNames: ['service', 'queue'],
        registers: [registry],
      }),
    );
    this.retryCounter = getOrCreate('rabbitmq_messages_retry_total', () =>
      new client.Counter({
        name: 'rabbitmq_messages_retry_total',
        help: 'Total RabbitMQ consumer retries',
        labelNames: ['service', 'queue'],
        registers: [registry],
      }),
    );
  }

  protected abstract handleEvent(
    envelope: RabbitMQEventEnvelope,
    ctx: RabbitMQConsumeContext,
  ): Promise<void>;

  protected async isDuplicate(_eventId: string): Promise<boolean> {
    return false;
  }

  protected serviceName(): string {
    return this.constructor.name;
  }

  async onModuleInit(): Promise<void> {
    await this.startWithRetry();
  }

  async onModuleDestroy(): Promise<void> {
    this.closing = true
    if (this.channel && this.consumerTag) {
      try {
        await this.channel.cancel(this.consumerTag);
      } catch {
        // ignore — channel may already be gone
      }
    }
    try {
      await this.channel?.close();
    } catch {
      // ignore
    }
    this.channel = null;
    this.consumerTag = null;
    this.connected = false;
  }

  isConnected(): boolean {
    return this.connected;
  }

  private async startWithRetry(retries = 15, baseDelayMs = 2000): Promise<void> {
    let delay = baseDelayMs;
    for (let i = 1; i <= retries; i++) {
      if (this.closing) return;
      try {
        await this.start();
        this.connected = true;
        this.logger.log(
          `RabbitMQ consumer started: queue=<${this.queue}> exchange=<${this.exchange}> prefetch=${this.prefetch}`,
        );
        return;
      } catch (err) {
        this.connected = false;
        this.logger.warn(
          `RabbitMQ consumer failed to start (attempt ${i}/${retries}): ${(err as Error).message}`,
        );
        if (i < retries) {
          await new Promise((resolve) => setTimeout(resolve, delay));
          delay = Math.min(delay * 2, 30000);
        }
      }
    }
    this.logger.error(
      `RabbitMQ consumer failed to start after ${retries} retries: queue=<${this.queue}>`,
    );
  }

  private async start(): Promise<void> {
    const prefetch = this.prefetch || resolvePrefetch(this.queue);
    this.channel = await this.rabbitmq.createConsumerChannel(prefetch);

    const exchangeType =
      RABBITMQ_EXCHANGE_TYPES[this.exchange as RabbitMQExchanges] ?? 'topic';
    await this.channel.assertExchange(this.exchange, exchangeType as string, {
      durable: true,
    });
    await this.channel.assertQueue(this.queue, {
      durable: true,
      arguments: buildQueueArguments(this.queue),
    });
    for (const routingKey of this.routingKeys) {
      await this.channel.bindQueue(this.queue, this.exchange, routingKey);
    }

    const { consumerTag } = await this.channel.consume(
      this.queue,
      (msg) => void this.onMessage(msg),
      { noAck: false },
    );
    this.consumerTag = consumerTag;
  }

  private async onMessage(msg: amqp.Message | null): Promise<void> {
    if (!msg) return;
    if (!this.channel) return;

    const headers = (msg.properties.headers ?? {}) as Record<string, unknown>;
    const attempt = Number(headers[RabbitMQHeaders.RETRY_COUNT] ?? 0);
    const traceId =
      (headers[RabbitMQHeaders.TRACE_ID] as string | undefined) ?? undefined;

    let envelope: RabbitMQEventEnvelope;
    try {
      envelope = JSON.parse(msg.content.toString()) as RabbitMQEventEnvelope;
    } catch {
      // Poison message — never retry, dead-letter immediately.
      this.failedCounter.inc({
        service: this.serviceName(),
        queue: this.queue,
        reason: 'invalid_json',
      });
      this.deadLetter(msg, 'invalid_json');
      return;
    }

    const eventId = envelope.eventId || (msg.properties.messageId as string) || 'unknown';

    try {
      if (await this.isDuplicate(eventId)) {
        this.channel.ack(msg);
        this.logger.debug(`Duplicate event skipped: ${eventId}`);
        return;
      }

      await this.handleEvent(envelope, { msg, attempt, traceId: traceId ?? envelope.traceId });
      this.channel.ack(msg);
      this.consumedCounter.inc({ service: this.serviceName(), queue: this.queue });
    } catch (err) {
      await this.onFailure(msg, envelope, attempt, err as Error);
    }
  }

  private async onFailure(
    msg: amqp.Message,
    envelope: RabbitMQEventEnvelope,
    attempt: number,
    err: Error,
  ): Promise<void> {
    if (!this.channel) return;

    if (attempt < this.maxRetries) {
      this.retryCounter.inc({ service: this.serviceName(), queue: this.queue });
      const delay = this.retryDelayMs * Math.pow(2, attempt);
      this.logger.warn(
        `Retrying [${this.queue}] event=${envelope.eventType} attempt=${attempt + 1}/${this.maxRetries} delay=${delay}ms: ${err.message}`,
      );
      await new Promise((resolve) => setTimeout(resolve, delay));
      if (this.closing || !this.channel) return;
      try {
        const headers = {
          ...((msg.properties.headers ?? {}) as Record<string, unknown>),
          [RabbitMQHeaders.RETRY_COUNT]: attempt + 1,
        };
        this.channel.sendToQueue(this.queue, msg.content, {
          ...msg.properties,
          headers,
          persistent: true,
        });
        this.channel.ack(msg);
      } catch (republishErr) {
        this.logger.error(
          `Retry republish failed [${this.queue}]: ${(republishErr as Error).message}`,
        );
        this.channel.nack(msg, false, true);
      }
      return;
    }

    this.failedCounter.inc({
      service: this.serviceName(),
      queue: this.queue,
      reason: 'max_retries_exceeded',
    });
    this.deadLetter(msg, err.message);
  }

  private deadLetter(msg: amqp.Message, reason: string): void {
    if (!this.channel) return;
    try {
      this.channel.nack(msg, false, false);
      this.dlqCounter.inc({ service: this.serviceName(), queue: this.queue });
      this.logger.warn(`Message dead-lettered [${this.queue}]: ${reason}`);
    } catch (err) {
      this.logger.error(`Dead-letter nack failed: ${(err as Error).message}`);
    }
  }
}
