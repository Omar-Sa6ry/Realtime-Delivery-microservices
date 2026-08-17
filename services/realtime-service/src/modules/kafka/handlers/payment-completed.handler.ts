import { Injectable } from '@nestjs/common';
import { RealtimeNatsSubjects, PaymentKafkaTopics } from '@delivery/common';
import { BaseKafkaEventHandler } from './base-kafka-event.handler';
import { EventDeduplicator } from '../../events/event-deduplicator';
import { EventMapper } from '../../events/event.mapper';
import { NatsPublisher } from '../../nats/nats.publisher';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import { PaymentStatusPayload } from '../../events/event.types';

@Injectable()
export class PaymentCompletedHandler extends BaseKafkaEventHandler<PaymentStatusPayload> {
  readonly eventType = PaymentKafkaTopics.PAYMENT_COMPLETED;
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