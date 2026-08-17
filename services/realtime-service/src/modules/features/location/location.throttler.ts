import { Injectable } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import { RedisStore, TokenBucketAlgorithm } from '@bts-soft/validation';
import { redisKeys, RATE_LIMITS } from '../../../common/common-types/constants';

/**
 * Token-bucket throttler for driver location updates (Redis-backed).
 *   max: 5 updates/sec (configurable)
 *   burst: 10
 */
@Injectable()
export class LocationThrottler {
  private store: RedisStore;
  private algo: TokenBucketAlgorithm;

  constructor(private readonly redisService: RedisService) {
    this.store = new RedisStore(this.redisService);
    const windowMs = (RATE_LIMITS.LOCATION_BURST / RATE_LIMITS.LOCATION_PER_SECOND) * 1000;
    this.algo = new TokenBucketAlgorithm(RATE_LIMITS.LOCATION_BURST, windowMs, this.store);
  }

  async allow(driverId: string): Promise<boolean> {
    const result = await this.algo.consume(redisKeys.locationBucket(driverId));
    return result.allowed;
  }
}