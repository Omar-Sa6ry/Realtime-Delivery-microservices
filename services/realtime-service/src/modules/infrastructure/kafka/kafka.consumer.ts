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
  BaseKafkaConsumer,
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

@Injectable()
export class KafkaConsumer extends BaseKafkaConsumer {
  protected readonly logger: any = new Logger(KafkaConsumer.name);
  protected consumer: Consumer;
  protected topics = CONSUMED_TOPICS;
  private readonly registry = new Map<string, KafkaEventHandler>();

  constructor(
    protected readonly kafkaService: KafkaService,
    private readonly config: ConfigService,
    private readonly metrics: RealtimeMetricsService,
    @Inject(REALTIME_EVENT_HANDLERS) handlers: KafkaEventHandler[],
  ) {
    super();
    this.consumer = this.kafkaService.consumer(
      this.config.get<string>('KAFKA_GROUP_ID', 'realtime-service-group'),
    );
    for (const handler of handlers) {
      this.registry.set(handler.eventType, handler);
    }
  }

  protected async handleMessage(payload: EachMessagePayload): Promise<void> {
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
        `Failed to process Kafka message (topic=${payload.topic}): ${(err as Error).message}`,
      );
      await this.routeToDlq(payload, err as Error);
    }
  }

  private async routeToDlq(payload: EachMessagePayload, reason: Error): Promise<void> {
    const dlqTopic = payload.topic.startsWith('payment')
      ? RealtimeKafkaTopics.DLQ_PAYMENT
      : RealtimeKafkaTopics.DLQ_DELIVERY;

    this.metrics.kafkaDlq.inc({ topic: payload.topic });
    try {
      await this.kafkaService.emit(
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
      this.logger.error(`Failed to write to DLQ: ${(dlqErr as Error).message}`);
    }
  }
}