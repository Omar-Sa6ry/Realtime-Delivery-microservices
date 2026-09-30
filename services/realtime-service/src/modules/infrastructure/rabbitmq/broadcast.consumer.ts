import { Injectable, Logger, Optional } from '@nestjs/common';
import {
  BaseRabbitMQConsumer,
  RabbitMQConsumeContext,
  RabbitMQEventEnvelope,
  RabbitMQExchanges,
  RabbitMQQueues,
  RabbitMQService,
  RealtimeNatsSubjects,
} from '@delivery/common';
import { EventMapper } from '../../features/events/event.mapper';
import { NatsPublisher } from '../nats/nats.publisher';

@Injectable()
export class RealtimeBroadcastConsumer extends BaseRabbitMQConsumer {
  protected readonly queue = RabbitMQQueues.REALTIME_BROADCAST;
  protected readonly exchange = RabbitMQExchanges.REALTIME;
  protected readonly routingKeys = [''];
  protected readonly logger = new Logger(RealtimeBroadcastConsumer.name);
  protected readonly prefetch = 20;

  constructor(
    rabbitmq: RabbitMQService,
    @Optional() private readonly mapper?: EventMapper,
    @Optional() private readonly natsPublisher?: NatsPublisher,
  ) {
    super(rabbitmq);
  }

  protected async handleEvent(
    envelope: RabbitMQEventEnvelope,
    ctx: RabbitMQConsumeContext,
  ): Promise<void> {
    if (!this.natsPublisher || !this.mapper) {
      this.logger.warn(
        `NatsPublisher/EventMapper not available — cannot broadcast: ${envelope.eventType} (trace=${ctx.traceId ?? 'n/a'})`,
      );
      return;
    }

    try {
      const clientEvent = this.mapper.toClientEvent({
        eventId: envelope.eventId,
        eventType: envelope.eventType,
        traceId: ctx.traceId ?? envelope.traceId,
        timestamp: envelope.timestamp ?? Date.now(),
        payload: (envelope.payload ?? {}) as Record<string, unknown>,
      });

      const subject = RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED;
      await this.natsPublisher.publish(subject, clientEvent);

      this.logger.debug(
        `Broadcast dispatched: ${envelope.eventType} (event=${envelope.eventId}, trace=${ctx.traceId ?? 'n/a'})`,
      );
    } catch (err) {
      this.logger.warn(
        `Broadcast mapping or publish failed for event ${envelope.eventType}: ${(err as Error)?.message} — acking to avoid DLQ storm`,
      );
    }
  }

  protected override async isDuplicate(_eventId: string): Promise<boolean> {
    // Realtime broadcast is intentionally lossy (at-most-once per connected session).
    return false;
  }
}
