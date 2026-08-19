import { Injectable, Logger } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import { redisKeys, TTL } from '../../../common/common-types/constants';

@Injectable()
export class IdempotencyStore {
  private readonly logger = new Logger(IdempotencyStore.name);

  constructor(private readonly redis: RedisService) {}

  async claim(commandId: string): Promise<'new' | 'duplicate'> {
    const key = redisKeys.idempotency(commandId);
    try {
      const created = await this.redis.setNX(key, '1', TTL.IDEMPOTENCY);
      return created ? 'new' : 'duplicate';
    } catch (err) {
      this.logger.warn(
        `Idempotency check failed for ${commandId}: ${err.message}`,
      );
      return 'new';
    }
  }
}
