import { Injectable } from '@nestjs/common';
import { RateLimitStore } from '@delivery/common';
import { redisKeys, RATE_LIMITS } from '../../common/common-types/constants';

/**
 * Token-bucket throttler for driver location updates (Redis-backed).
 *   max: 5 updates/sec (configurable)
 *   burst: 10
 */
@Injectable()
export class LocationThrottler {
  constructor(private readonly store: RateLimitStore) {}

  async allow(driverId: string): Promise<boolean> {
    return this.store.tokenBucketCheck(
      redisKeys.locationBucket(driverId),
      RATE_LIMITS.LOCATION_BURST,
      RATE_LIMITS.LOCATION_PER_SECOND,
      1,
    );
  }
}