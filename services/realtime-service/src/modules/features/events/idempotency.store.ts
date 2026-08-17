import { Injectable, Logger } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import { redisKeys, TTL } from '../../../common/common-types/constants';

/**
 * Command idempotency store: `SET realtime:idempotency:{commandId} 1 NX EX 86400`.
 * Commands (ACCEPT/REJECT/COMPLETE) forward only once to the domain service.
 */
@Injectable()
export class IdempotencyStore {
  private readonly logger = new Logger(IdempotencyStore.name);

  constructor(private readonly redis: RedisService) {}

  /**
   * @returns 'new' when this commandId was never seen, 'duplicate' otherwise.
   */
  async claim(commandId: string): Promise<'new' | 'duplicate'> {
    const key = redisKeys.idempotency(commandId);
    try {
      const created = await this.redis.setNX(key, '1', TTL.IDEMPOTENCY);
      return created ? 'new' : 'duplicate';
    } catch (err) {
      this.logger.warn(`Idempotency check failed for ${commandId}: ${err.message}`);
      return 'new';
    }
  }
}