import { Injectable } from '@nestjs/common';
import { RealtimeNatsSubjects, DeliveryKafkaTopics } from '@delivery/common';
import { BaseKafkaEventHandler } from './base-kafka-event.handler';
import { EventDeduplicator } from '../../../features/events/event-deduplicator';
import { EventMapper } from '../../../features/events/event.mapper';
import { NatsPublisher } from '../../nats/nats.publisher';
import { RealtimeMetricsService } from '../../../../common/metrics/realtime-metrics.service';
import { DeliveryStatusPayload } from '../../../features/events/event.types';

@Injectable()
export class DeliveryPickedUpHandler extends BaseKafkaEventHandler<DeliveryStatusPayload> {
  readonly eventType = DeliveryKafkaTopics.DELIVERY_PICKED_UP;
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