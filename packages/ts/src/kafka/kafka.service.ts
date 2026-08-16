import { Injectable, Logger, OnModuleDestroy } from '@nestjs/common';
import { Kafka, KafkaConfig, Consumer, Producer, ProducerRecord } from 'kafkajs';
import { randomUUID } from 'crypto';

export interface KafkaModuleOptions {
  clientId?: string;
  brokers?: string[];
  ssl?: KafkaConfig['ssl'];
  sasl?: KafkaConfig['sasl'];
  connectionTimeout?: number;
}

export interface KafkaEmitOptions {
  key?: string;
  partition?: number;
  traceId?: string;
  headers?: Record<string, string>;
}

/**
 * Generic Kafka event envelope — matches the Go EventEnvelope struct.
 * All events published through KafkaService are wrapped in this structure.
 */
export interface KafkaEventEnvelope<T = unknown> {
  eventId: string;
  eventType: string;
  traceId?: string;
  timestamp: number; // unix milliseconds
  payload: T;
}

@Injectable()
export class KafkaService implements OnModuleDestroy {
  private readonly logger = new Logger(KafkaService.name);
  private readonly kafka: Kafka;
  private producer: Producer;

  constructor(private readonly options: KafkaModuleOptions) {
    const kafkaConfig: KafkaConfig = {
      clientId: this.options.clientId || 'delivery-service',
      brokers: this.options.brokers && this.options.brokers.length > 0
        ? this.options.brokers
        : ['localhost:9092'],
    };

    if (this.options.ssl) kafkaConfig.ssl = this.options.ssl;
    if (this.options.sasl) kafkaConfig.sasl = this.options.sasl;
    if (this.options.connectionTimeout) kafkaConfig.connectionTimeout = this.options.connectionTimeout;

    this.kafka = new Kafka(kafkaConfig);
  }

  /** Returns the underlying kafkajs client (for consumers / admin). */
  getClient(): Kafka {
    return this.kafka;
  }

  /** Creates a consumer bound to the given group id. */
  consumer(groupId: string): Consumer {
    return this.kafka.consumer({ groupId });
  }

  private async ensureProducer(): Promise<Producer> {
    if (!this.producer) {
      this.producer = this.kafka.producer();
      await this.producer.connect();
      this.logger.log('Kafka producer connected');
    }
    return this.producer;
  }

  buildEnvelope<T>(eventType: string, payload: T, traceId?: string): KafkaEventEnvelope<T> {
    return {
      eventId: randomUUID(),
      eventType,
      traceId,
      timestamp: Date.now(),
      payload,
    };
  }

  /** Publishes an event wrapped in the standard envelope to the given topic. */
  async emit<T = unknown>(
    topic: string,
    eventType: string,
    payload: T,
    options?: KafkaEmitOptions,
  ): Promise<void> {
    const producer = await this.ensureProducer();
    const envelope = this.buildEnvelope(eventType, payload, options?.traceId);

    const record: ProducerRecord = {
      topic,
      messages: [
        {
          key: options?.key,
          partition: options?.partition,
          headers: options?.headers,
          value: JSON.stringify(envelope),
        },
      ],
    };

    await producer.send(record);
    this.logger.debug(`Event emitted to Kafka topic [${topic}]: ${eventType}`);
  }

  async onModuleDestroy() {
    if (this.producer) {
      await this.producer.disconnect();
      this.producer = undefined;
    }
  }
}