import { Injectable } from '@nestjs/common';
import { RealtimeNatsSubjects, DriverEventType } from '@delivery/common';
import { BaseKafkaEventHandler } from './base-kafka-event.handler';
import { EventDeduplicator } from '../../../features/events/event-deduplicator';
import { EventMapper } from '../../../features/events/event.mapper';
import { NatsPublisher } from '../../nats/nats.publisher';
import { RealtimeMetricsService } from '../../../../common/metrics/realtime-metrics.service';

@Injectable()
export class DriverNoDriverAvailableHandler extends BaseKafkaEventHandler<any> {
  readonly eventType = DriverEventType.NoDriverAvailable;
  readonly natsSubject = RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED;

  constructor(
    deduplicator: EventDeduplicator,
    mapper: EventMapper,
    natsPublisher: NatsPublisher,
    metrics: RealtimeMetricsService,
  ) {
    super(deduplicator, mapper, natsPublisher, metrics);
  }
}
