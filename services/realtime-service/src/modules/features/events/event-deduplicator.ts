import { Injectable, Logger } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import { redisKeys, TTL } from '../../../common/common-types/constants';

/**
 * Event deduplication: `SET realtime:processed:{eventId} 1 NX EX 86400`.
 * Used before forwarding critical Kafka events to NATS/WS so repeated
 * deliveries (at-least-once semantics) do not fan out twice.
 */
@Injectable()
export class EventDeduplicator {
  private readonly logger = new Logger(EventDeduplicator.name);

  constructor(private readonly redis: RedisService) {}

  /** Returns true when the event id was already processed (duplicate). */
  async isDuplicate(eventId: string): Promise<boolean> {
    const key = redisKeys.processed(eventId);
    return this.redis
      .setNX(key, '1', TTL.PROCESSED)
      .then((created) => !created)
      .catch((err) => {
        this.logger.warn(`Dedup check failed for ${eventId}: ${err.message}; treating as new`);
        return false;
      });
  }

  /** Marks an event as processed without dedup racing logic (used for non-critical). */
  async markProcessed(eventId: string): Promise<void> {
    await this.redis
      .set(redisKeys.processed(eventId), '1', TTL.PROCESSED)
      .catch((err) =>
        this.logger.warn(`Failed to mark event processed ${eventId}: ${err.message}`),
      );
  }
}