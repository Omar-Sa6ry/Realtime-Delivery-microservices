import {
  Injectable,
  Inject,
  Logger,
  OnModuleDestroy,
  OnModuleInit,
} from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { Consumer, EachMessagePayload } from 'kafkajs';
import {
  KafkaService,
  KafkaEventEnvelope,
  DeliveryKafkaTopics,
  PaymentKafkaTopics,
  RealtimeKafkaTopics,
} from '@delivery/common';
import { KafkaEventHandler, REALTIME_EVENT_HANDLERS } from './handlers/base-kafka-event.handler';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';

const CONSUMED_TOPICS = [
  DeliveryKafkaTopics.DELIVERY_CREATED,
  DeliveryKafkaTopics.DRIVER_ASSIGNED,
  DeliveryKafkaTopics.DRIVER_ACCEPTED,
  DeliveryKafkaTopics.DELIVERY_PICKED_UP,
  DeliveryKafkaTopics.DELIVERY_IN_TRANSIT,
  DeliveryKafkaTopics.DELIVERY_COMPLETED,
  DeliveryKafkaTopics.DELIVERY_CANCELLED,
  PaymentKafkaTopics.PAYMENT_COMPLETED,
  PaymentKafkaTopics.PAYMENT_FAILED,
];

/**
 * Kafka consumer for the realtime fan-out pipeline.
 * Consumer group: realtime-service-group (scales with replicas — each event goes to one node).
 *  - deserialize envelope       (KafkaEventEnvelope)
 *  - route to handler by eventType (Strategy)
 *  - handler performs: validate schema -> dedup -> map -> NATS fan-out
 *  - failures are routed to the DLQ (realtime.delivery.dlq / realtime.payment.dlq)
 *  - offsets are committed by kafkajs after processing (at-least-once + idempotent handlers)
 */
@Injectable()
export class KafkaConsumer implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(KafkaConsumer.name);
  private consumer: Consumer;
  private readonly registry = new Map<string, KafkaEventHandler>();
  private connected = false;

  constructor(
    private readonly kafka: KafkaService,
    private readonly config: ConfigService,
    private readonly metrics: RealtimeMetricsService,
    @Inject(REALTIME_EVENT_HANDLERS) handlers: KafkaEventHandler[],
  ) {
    this.consumer = this.kafka.consumer(
      this.config.get<string>('KAFKA_GROUP_ID', 'realtime-service-group'),
    );
    for (const handler of handlers) {
      this.registry.set(handler.eventType, handler);
    }
  }

  async onModuleInit(): Promise<void> {
    try {
      await this.consumer.connect();
      await this.consumer.subscribe({ topics: CONSUMED_TOPICS, fromBeginning: false });
      await this.consumer.run({
        eachMessage: async (payload) => this.handleMessage(payload),
      });
      this.connected = true;
      this.logger.log('Kafka consumer started (realtime-service-group)');
    } catch (err) {
      this.connected = false;
      this.logger.error(`Kafka consumer failed to start: ${err.message}`);
    }
  }

  private async handleMessage(payload: EachMessagePayload): Promise<void> {
    try {
      const envelope = JSON.parse(payload.message.value?.toString() || '{}') as KafkaEventEnvelope;
      if (!envelope?.eventId || !envelope?.eventType) {
        this.logger.warn('Dropping malformed Kafka message (missing envelope fields)');
        return;
      }

      const handler = this.registry.get(envelope.eventType);
      if (!handler) {
        this.logger.debug(`No handler registered for event type ${envelope.eventType}`);
        return;
      }

      await handler.handle(envelope);
    } catch (err) {
      this.logger.error(
        `Failed to process Kafka message (topic=${payload.topic}): ${err.message}`,
      );
      await this.routeToDlq(payload, err);
    }
  }

  private async routeToDlq(payload: EachMessagePayload, reason: Error): Promise<void> {
    const dlqTopic = payload.topic.startsWith('payment')
      ? RealtimeKafkaTopics.DLQ_PAYMENT
      : RealtimeKafkaTopics.DLQ_DELIVERY;

    this.metrics.kafkaDlq.inc({ topic: payload.topic });
    try {
      await this.kafka.emit(
        dlqTopic,
        'realtime.dlq',
        {
          originalTopic: payload.topic,
          offset: payload.message.offset,
          reason: reason.message,
          value: payload.message.value?.toString(),
        },
        { key: payload.message.key?.toString() || 'dlq' },
      );
      this.logger.warn(`Message routed to DLQ [${dlqTopic}]`);
    } catch (dlqErr) {
      this.logger.error(`Failed to write to DLQ: ${dlqErr.message}`);
    }
  }

  isConnected(): boolean {
    return this.connected;
  }

  async onModuleDestroy(): Promise<void> {
    try {
      await this.consumer.disconnect();
      this.connected = false;
      this.logger.log('Kafka consumer disconnected');
    } catch (err) {
      this.logger.error(`Kafka disconnect failed: ${err.message}`);
    }
  }
}