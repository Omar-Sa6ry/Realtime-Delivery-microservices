import { Injectable } from '@nestjs/common';
import { RealtimeNatsSubjects, DeliveryKafkaTopics } from '@delivery/common';
import { BaseKafkaEventHandler } from './base-kafka-event.handler';
import { EventDeduplicator } from '../../events/event-deduplicator';
import { EventMapper } from '../../events/event.mapper';
import { NatsPublisher } from '../../nats/nats.publisher';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import { DeliveryCompletedPayload } from '../../events/event.types';

@Injectable()
export class DeliveryCompletedHandler extends BaseKafkaEventHandler<DeliveryCompletedPayload> {
  readonly eventType = DeliveryKafkaTopics.DELIVERY_COMPLETED;
  readonly natsSubject = RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED;

  constructor(
    deduplicator: EventDeduplicator,
    mapper: EventMapper,
    natsPublisher: NatsPublisher,
    metrics: RealtimeMetricsService,
  ) {
    super(deduplicator, mapper, natsPublisher, metrics);
  }
}