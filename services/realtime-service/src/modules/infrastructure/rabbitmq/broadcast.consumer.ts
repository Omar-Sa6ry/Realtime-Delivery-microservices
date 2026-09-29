import { Injectable, Logger, Optional } from '@nestjs/common';
import {
  BaseRabbitMQConsumer,
  RabbitMQConsumeContext,
  RabbitMQEventEnvelope,
  RabbitMQExchanges,
  RabbitMQQueues,
  RabbitMQService,
} from '@delivery/common';
import { EventMapper } from '../../features/events/event.mapper';

@Injectable()
export class RealtimeBroadcastConsumer extends BaseRabbitMQConsumer {
  protected readonly queue = RabbitMQQueues.REALTIME_BROADCAST;
  protected readonly exchange = RabbitMQExchanges.REALTIME;
  protected readonly routingKeys = [''];
  protected readonly logger = new Logger(RealtimeBroadcastConsumer.name);
  protected readonly prefetch = 20;

  constructor(
    rabbitmq: RabbitMQService,
    @Optional() private readonly eventMapper?: EventMapper,
  ) {
    super(rabbitmq);
  }

  protected async handleEvent(
    envelope: RabbitMQEventEnvelope,
    ctx: RabbitMQConsumeContext,
  ): Promise<void> {
    if (!this.eventMapper) {
      this.logger.debug(
        `No EventMapper available, logging only: ${envelope.eventType} (trace=${ctx.traceId ?? 'n/a'})`,
      );
      return;
    }

    try {
      const clientEvent = this.eventMapper.toClientEvent({
        eventId: envelope.eventId,
        eventType: envelope.eventType,
        traceId: ctx.traceId ?? envelope.traceId,
        timestamp: envelope.timestamp ?? Date.now(),
        payload: (envelope.payload ?? {}) as Record<string, unknown>,
      });
      this.logger.debug(
        `Broadcast mapped: ${envelope.eventType} -> ${clientEvent.type} (event=${envelope.eventId})`,
      );
    } catch (err) {
      // Unsupported/unknown event types are logged and acked — never rethrown,
      // so they cannot poison the queue with infinite redeliveries.
      this.logger.debug(
        `Skipping unsupported realtime event ${envelope.eventType}: ${(err as Error)?.message}`,
      );
    }
  }

  protected override async isDuplicate(_eventId: string): Promise<boolean> {
    // Realtime broadcast is lossy by design; upstream dedup (EventDeduplicator)
    // applies at emit time. No inbox check here to keep broadcast low-latency.
    return false;
  }
}
