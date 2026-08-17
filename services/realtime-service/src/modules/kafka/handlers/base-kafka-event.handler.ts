import { Injectable, Logger } from '@nestjs/common';
import { KafkaEventEnvelope } from '@delivery/common';
import { RealtimeNatsSubjects } from '@delivery/common';
import { EventDeduplicator } from '../../events/event-deduplicator';
import { EventMapper } from '../../events/event.mapper';
import { NatsPublisher } from '../../nats/nats.publisher';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';

export interface KafkaEventHandler {
  readonly eventType: string;
  handle(envelope: KafkaEventEnvelope): Promise<void>;
}

export const REALTIME_EVENT_HANDLERS = 'REALTIME_EVENT_HANDLERS';

/**
 * Template Method pattern — base Kafka event handler:
 *  1. schema validation hook (per concrete handler)
 *  2. deduplication (Redis SET NX EX)
 *  3. map to client event (EventMapper factory)
 *  4. publish to NATS fan-out subject
 *  The template never exposes internal envelope metadata to clients.
 */
@Injectable()
export abstract class BaseKafkaEventHandler<T> implements KafkaEventHandler {
  protected readonly logger = new Logger(this.constructor.name);

  abstract readonly eventType: string;
  abstract readonly natsSubject: RealtimeNatsSubjects;

  constructor(
    protected readonly deduplicator: EventDeduplicator,
    protected readonly mapper: EventMapper,
    protected readonly natsPublisher: NatsPublisher,
    protected readonly metrics: RealtimeMetricsService,
  ) {}

  /** Hook: concrete handlers may validate the payload shape. */
  protected validate(payload: unknown): T {
    return payload as T;
  }

  async handle(envelope: KafkaEventEnvelope): Promise<void> {
    const payload = this.validate(envelope.payload);

    if (await this.deduplicator.isDuplicate(envelope.eventId)) {
      this.logger.debug(
        `Duplicate event skipped (eventId=${envelope.eventId} type=${this.eventType})`,
      );
      return;
    }

    const clientEvent = this.mapper.toClientEvent(envelope);
    const published = await this.natsPublisher.publish(this.natsSubject, clientEvent);
    if (!published) {
      this.logger.warn(`Fan-out publish failed for ${this.eventType}`);
    }

    this.metrics.kafkaProcessed.inc({ eventType: this.eventType });
  }
}