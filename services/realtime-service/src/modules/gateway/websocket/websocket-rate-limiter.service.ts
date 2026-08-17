import { Injectable } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import { RedisStore, SlidingWindowLogAlgorithm } from '@bts-soft/validation';
import { RATE_LIMITS } from '../../../common/common-types/constants';

export type RateAction = 'connect' | 'subscribe' | 'command';

@Injectable()
export class WebsocketRateLimiterService {
  private store: RedisStore;

  constructor(private readonly redisService: RedisService) {
    this.store = new RedisStore(this.redisService);
  }

  async check(userId: string, action: RateAction): Promise<boolean> {
    let algo: SlidingWindowLogAlgorithm;
    
    switch (action) {
      case 'connect':
        algo = new SlidingWindowLogAlgorithm(RATE_LIMITS.CONNECT_PER_MINUTE, 60_000, this.store);
        break;
      case 'subscribe':
        algo = new SlidingWindowLogAlgorithm(RATE_LIMITS.SUBSCRIBE_PER_MINUTE, 60_000, this.store);
        break;
      case 'command':
        algo = new SlidingWindowLogAlgorithm(RATE_LIMITS.COMMANDS_PER_MINUTE, 60_000, this.store);
        break;
    }

    const key = `ws:rate:${userId}:${action}`;
    const result = await algo.consume(key);
    return result.allowed;
  }
}

