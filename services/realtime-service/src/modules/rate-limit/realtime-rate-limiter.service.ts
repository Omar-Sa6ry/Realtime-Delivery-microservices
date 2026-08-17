import { Injectable } from '@nestjs/common';
import { RateLimitStore } from '@delivery/common';
import { RATE_LIMITS } from '../../common/common-types/constants';

export type RateAction = 'connect' | 'subscribe' | 'command';

/**
 * High-level rate limiter used by the WS guard chain.
 * Sliding window per (user, action).
 */
@Injectable()
export class RealtimeRateLimiterService {
  constructor(private readonly store: RateLimitStore) {}

  async check(userId: string, action: RateAction): Promise<boolean> {
    switch (action) {
      case 'connect':
        return this.store.slidingWindowCheck(
          userId,
          'connect',
          RATE_LIMITS.CONNECT_PER_MINUTE,
          60_000,
        );
      case 'subscribe':
        return this.store.slidingWindowCheck(
          userId,
          'subscribe',
          RATE_LIMITS.SUBSCRIBE_PER_MINUTE,
          60_000,
        );
      case 'command':
        return this.store.slidingWindowCheck(
          userId,
          'commands',
          RATE_LIMITS.COMMANDS_PER_MINUTE,
          60_000,
        );
    }
  }
}