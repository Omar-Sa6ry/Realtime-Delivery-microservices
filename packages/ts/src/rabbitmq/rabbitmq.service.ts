import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import * as amqp from 'amqplib';
import * as client from 'prom-client';
import { randomUUID } from 'crypto';
import {
  RABBITMQ_EXCHANGE_TYPES,
  RabbitMQExchanges,
  RabbitMQHeaders,
  buildExchangeArguments,
  buildQueueArguments,
} from './rabbitmq.constants';

export interface RabbitMQModuleOptions {
  url?: string;
  serviceName?: string;
  maxReconnectAttempts?: number;
  reconnectBaseDelayMs?: number;
  circuitBreakerFailureThreshold?: number;
  circuitBreakerResetTimeoutMs?: number;
}

export interface RabbitMQPublishOptions {
  persistent?: boolean;
  priority?: number;
  traceId?: string;
  headers?: Record<string, unknown>;
  aggregateId?: string;
  aggregateType?: string;
  producer?: string;
  correlationId?: string;
  causationId?: string;
}

export interface RabbitMQEventEnvelope<T = unknown> {
  eventId: string;
  eventType: string;
  eventVersion: number;
  occurredAt: string;
  timestamp: number;
  aggregateType: string;
  aggregateId: string;
  producer: string;
  correlationId?: string;
  causationId?: string;
  traceId?: string;
  payload: T;
}

export interface RabbitMQBinding {
  exchange: string;
  exchangeType?: string;
  queue: string;
  routingKey: string;
}

export enum CircuitBreakerState {
  CLOSED = 0,
  OPEN = 1,
  HALF_OPEN = 2,
}

function deriveAggregateType(eventType: string): string {
  const dot = eventType.indexOf('.');
  return dot !== -1 ? eventType.slice(0, dot) : eventType;
}

function resolveUrl(explicit?: string): string {
  return (
    explicit ||
    process.env.RABBITMQ_URL ||
    'amqp://guest:guest@localhost:5672/'
  );
}

@Injectable()
export class RabbitMQService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(RabbitMQService.name);

  private readonly url: string;
  private readonly serviceName: string;
  private readonly maxReconnectAttempts: number;
  private readonly reconnectBaseDelayMs: number;
  private readonly failureThreshold: number;
  private readonly resetTimeoutMs: number;

  private connection: amqp.ChannelModel | null = null;
  private confirmChannel: amqp.ConfirmChannel | null = null;
  private closing = false;
  private reconnecting = false;
  private topologyAsserted = false;

  private breakerState: CircuitBreakerState = CircuitBreakerState.CLOSED;
  private consecutiveFailures = 0;
  private breakerOpenedAt = 0;
  private halfOpenSuccesses = 0;

  private publishedCounter: client.Counter<string>;
  private failedCounter: client.Counter<string>;
  private connectionErrorsCounter: client.Counter<string>;
  private breakerStateGauge: client.Gauge<string>;

  constructor(options?: RabbitMQModuleOptions) {
    this.url = resolveUrl(options?.url);
    this.serviceName =
      options?.serviceName || process.env.RABBITMQ_CLIENT_ID || 'delivery-service';
    this.maxReconnectAttempts = options?.maxReconnectAttempts ?? 15;
    this.reconnectBaseDelayMs = options?.reconnectBaseDelayMs ?? 1000;
    this.failureThreshold = options?.circuitBreakerFailureThreshold ?? 5;
    this.resetTimeoutMs = options?.circuitBreakerResetTimeoutMs ?? 30000;
    this.initMetrics();
  }

  // ---------------------------------------------------------------- metrics

  private initMetrics(): void {
    const registry = client.register;
    const getOrCreate = <T>(name: string, create: () => T): T => {
      const existing = registry.getSingleMetric(name) as unknown as T | undefined;
      return existing ?? create();
    };

    this.publishedCounter = getOrCreate('rabbitmq_messages_published_total', () =>
      new client.Counter({
        name: 'rabbitmq_messages_published_total',
        help: 'Total RabbitMQ messages published with confirm',
        labelNames: ['service', 'exchange', 'routing_key'],
        registers: [registry],
      }),
    );

    this.failedCounter = getOrCreate('rabbitmq_messages_failed_total', () =>
      new client.Counter({
        name: 'rabbitmq_messages_failed_total',
        help: 'Total RabbitMQ publish failures',
        labelNames: ['service', 'exchange', 'routing_key', 'reason'],
        registers: [registry],
      }),
    );

    this.connectionErrorsCounter = getOrCreate(
      'rabbitmq_connection_errors_total',
      () =>
        new client.Counter({
          name: 'rabbitmq_connection_errors_total',
          help: 'Total RabbitMQ connection/channel errors',
          labelNames: ['service'],
          registers: [registry],
        }),
    );

    this.breakerStateGauge = getOrCreate('rabbitmq_circuit_breaker_state', () =>
      new client.Gauge({
        name: 'rabbitmq_circuit_breaker_state',
        help: 'RabbitMQ circuit breaker state (0=closed, 1=open, 2=half-open)',
        labelNames: ['service'],
        registers: [registry],
      }),
    );
    this.breakerStateGauge.set({ service: this.serviceName }, CircuitBreakerState.CLOSED);
  }

  // -------------------------------------------------------------- lifecycle

  async onModuleInit(): Promise<void> {
    await this.connectWithRetry();
  }

  async onModuleDestroy(): Promise<void> {
    this.closing = true;
    try {
      await this.confirmChannel?.close();
    } catch {
      // ignore — broker may already be gone
    }
    try {
      await this.connection?.close();
    } catch {
      // ignore
    }
    this.confirmChannel = null;
    this.connection = null;
  }

  private async connectWithRetry(): Promise<void> {
    let delay = this.reconnectBaseDelayMs;
    for (let attempt = 1; attempt <= this.maxReconnectAttempts; attempt++) {
      try {
        await this.connectOnce();
        this.logger.log(`RabbitMQ connected (${this.safeUrl()})`);
        return;
      } catch (err) {
        this.connectionErrorsCounter.inc({ service: this.serviceName });
        this.logger.warn(
          `RabbitMQ connection failed (attempt ${attempt}/${this.maxReconnectAttempts}): ${(err as Error).message}`,
        );
        if (attempt < this.maxReconnectAttempts) {
          await new Promise((resolve) => setTimeout(resolve, delay));
          delay = Math.min(delay * 2, 30000);
        }
      }
    }
    this.logger.error('RabbitMQ failed to connect after maximum retries');
  }

  private async connectOnce(): Promise<void> {
    this.connection = await amqp.connect(this.url, {
      // Fail fast on blocked publishes instead of hanging forever.
      timeout: 10000,
    });
    this.confirmChannel = await this.connection.createConfirmChannel();
    this.topologyAsserted = false;

    this.connection.on('close', () => {
      this.connection = null;
      this.confirmChannel = null;
      if (!this.closing && !this.reconnecting) {
        this.connectionErrorsCounter.inc({ service: this.serviceName });
        this.logger.warn('RabbitMQ connection closed — reconnecting...');
        void this.scheduleReconnect();
      }
    });
    this.connection.on('error', (err) => {
      this.connectionErrorsCounter.inc({ service: this.serviceName });
      this.logger.error(`RabbitMQ connection error: ${(err as Error)?.message}`);
    });
    this.confirmChannel.on('error', (err) => {
      this.connectionErrorsCounter.inc({ service: this.serviceName });
      this.logger.error(`RabbitMQ confirm channel error: ${(err as Error)?.message}`);
    });
  }

  private async scheduleReconnect(): Promise<void> {
    if (this.closing || this.reconnecting) return;
    this.reconnecting = true;
    try {
      await this.connectWithRetry();
    } finally {
      this.reconnecting = false;
    }
  }

  private async ensureConnected(): Promise<void> {
    if (this.confirmChannel && this.connection) return;
    if (this.closing) throw new Error('RabbitMQ service is shutting down');
    await this.connectWithRetry();
    if (!this.confirmChannel || !this.connection) {
      throw new Error('RabbitMQ is not connected');
    }
  }

  // ---------------------------------------------------------- circuit breaker

  getCircuitBreakerState(): CircuitBreakerState {
    return this.breakerState;
  }

  isConnected(): boolean {
    return !!this.connection && !!this.confirmChannel;
  }

  private guardCircuitBreaker(exchange: string, routingKey: string): void {
    if (this.breakerState === CircuitBreakerState.OPEN) {
      if (Date.now() - this.breakerOpenedAt >= this.resetTimeoutMs) {
        this.breakerState = CircuitBreakerState.HALF_OPEN;
        this.halfOpenSuccesses = 0;
        this.breakerStateGauge.set({ service: this.serviceName }, CircuitBreakerState.HALF_OPEN);
        this.logger.log('RabbitMQ circuit breaker → half-open (trial publish allowed)');
      } else {
        this.failedCounter.inc({
          service: this.serviceName,
          exchange,
          routing_key: routingKey,
          reason: 'circuit_open',
        });
        throw new Error('RabbitMQ circuit breaker is OPEN — publish rejected (fail-fast)');
      }
    }
  }

  private recordSuccess(): void {
    if (this.breakerState === CircuitBreakerState.HALF_OPEN) {
      this.halfOpenSuccesses += 1;
      if (this.halfOpenSuccesses >= 1) {
        this.breakerState = CircuitBreakerState.CLOSED;
        this.consecutiveFailures = 0;
        this.breakerStateGauge.set({ service: this.serviceName }, CircuitBreakerState.CLOSED);
        this.logger.log('RabbitMQ circuit breaker → closed');
      }
    } else {
      this.consecutiveFailures = 0;
    }
  }

  private recordFailure(): void {
    this.consecutiveFailures += 1;
    if (
      this.breakerState === CircuitBreakerState.HALF_OPEN ||
      (this.breakerState === CircuitBreakerState.CLOSED &&
        this.consecutiveFailures >= this.failureThreshold)
    ) {
      this.breakerState = CircuitBreakerState.OPEN;
      this.breakerOpenedAt = Date.now();
      this.breakerStateGauge.set({ service: this.serviceName }, CircuitBreakerState.OPEN);
      this.logger.warn(
        `RabbitMQ circuit breaker → open after ${this.consecutiveFailures} consecutive failures`,
      );
    }
  }

  // ---------------------------------------------------------------- envelope

  buildEnvelope<T>(
    eventType: string,
    payload: T,
    options?: RabbitMQPublishOptions,
  ): RabbitMQEventEnvelope<T> {
    const now = new Date();
    return {
      eventId: randomUUID(),
      eventType,
      eventVersion: 1,
      occurredAt: now.toISOString(),
      timestamp: now.getTime(),
      aggregateType: options?.aggregateType || deriveAggregateType(eventType),
      aggregateId: options?.aggregateId || '',
      producer: options?.producer || this.serviceName,
      correlationId: options?.correlationId,
      causationId: options?.causationId,
      traceId: options?.traceId,
      payload,
    };
  }

  // ----------------------------------------------------------------- publish

  async publish<T = unknown>(
    exchange: string,
    routingKey: string,
    eventType: string,
    payload: T,
    options?: RabbitMQPublishOptions,
  ): Promise<void> {
    this.guardCircuitBreaker(exchange, routingKey);
    await this.ensureConnected();

    const envelope = this.buildEnvelope(eventType, payload, options);
    const headers: Record<string, unknown> = {
      ...(options?.headers ?? {}),
      [RabbitMQHeaders.TRACE_ID]: options?.traceId ?? envelope.traceId ?? envelope.eventId,
      ...(options?.correlationId && { 'x-correlation-id': options.correlationId }),
    };

    try {
      const channel = this.confirmChannel;
      channel.publish(
        exchange,
        routingKey,
        Buffer.from(JSON.stringify(envelope)),
        {
          contentType: 'application/json',
          persistent: options?.persistent ?? true,
          messageId: envelope.eventId,
          timestamp: envelope.timestamp,
          ...(options?.priority !== undefined && { priority: options.priority }),
          headers,
        },
      );
      await channel.waitForConfirms();
      this.recordSuccess();
      this.publishedCounter.inc({
        service: this.serviceName,
        exchange,
        routing_key: routingKey,
      });
      this.logger.debug(`Published to [${exchange}] rk=<${routingKey}>: ${eventType}`);
    } catch (err) {
      this.recordFailure();
      this.failedCounter.inc({
        service: this.serviceName,
        exchange,
        routing_key: routingKey,
        reason: 'publish_error',
      });
      this.logger.error(
        `RabbitMQ publish failed [${exchange}] rk=<${routingKey}>: ${(err as Error).message}`,
      );
      throw err;
    }
  }

  /**
   * Lazily asserts exchanges/queues/bindings (idempotent). The broker
   * already loads the base topology from `definitions.json`, so this is
   * only needed for queues/exchanges a service introduces.
   */
  async ensureTopology(bindings: RabbitMQBinding[]): Promise<void> {
    await this.ensureConnected();
    const channel = this.confirmChannel;

    const exchanges = new Map<string, string>();
    for (const b of bindings) {
      if (!exchanges.has(b.exchange)) {
        exchanges.set(
          b.exchange,
          b.exchangeType ??
            RABBITMQ_EXCHANGE_TYPES[b.exchange as RabbitMQExchanges] ??
            'topic',
        );
      }
    }
    for (const [exchange, type] of exchanges) {
      const args = buildExchangeArguments(exchange);
      await channel.assertExchange(exchange, type as string, { durable: true, arguments: args });
    }
    for (const b of bindings) {
      await channel.assertQueue(b.queue, {
        durable: true,
        arguments: buildQueueArguments(b.queue),
      });
      await channel.bindQueue(b.queue, b.exchange, b.routingKey);
    }
    this.topologyAsserted = true;
  }

  isTopologyAsserted(): boolean {
    return this.topologyAsserted;
  }

  /**
   * Creates a dedicated channel for a consumer (prefetch + manual ack).
   * The channel is bound to the shared connection and must be closed by
   * the caller on shutdown.
   */
  async createConsumerChannel(prefetch = 10): Promise<amqp.Channel> {
    await this.ensureConnected();
    const channel = await this.connection.createChannel();
    await channel.prefetch(prefetch);
    return channel;
  }

  /** Exposes the raw connection (for health checks / advanced usage). */
  getConnection(): amqp.ChannelModel | null {
    return this.connection;
  }

  private safeUrl(): string {
    return this.url.replace(/:\/\/([^:]+):[^@]+@/, '://$1:***@');
  }
}
