import { Injectable } from '@nestjs/common';
import { RealtimeNatsSubjects, DeliveryKafkaTopics } from '@delivery/common';
import { BaseKafkaEventHandler } from './base-kafka-event.handler';
import { EventDeduplicator } from '../../events/event-deduplicator';
import { EventMapper } from '../../events/event.mapper';
import { NatsPublisher } from '../../nats/nats.publisher';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import { DeliveryCreatedPayload } from '../../events/event.types';

@Injectable()
export class DeliveryCreatedHandler extends BaseKafkaEventHandler<DeliveryCreatedPayload> {
  readonly eventType = DeliveryKafkaTopics.DELIVERY_CREATED;
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