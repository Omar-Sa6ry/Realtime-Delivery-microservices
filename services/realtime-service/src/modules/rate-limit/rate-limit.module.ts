import { Global, Module } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import { RateLimitStore } from '@delivery/common';
import { RealtimeRateLimiterService } from './realtime-rate-limiter.service';

@Global()
@Module({
  providers: [
    RealtimeRateLimiterService,
    {
      provide: RateLimitStore,
      useFactory: (redis: RedisService) => new RateLimitStore(redis as any),
      inject: [RedisService],
    },
  ],
  exports: [RateLimitStore, RealtimeRateLimiterService],
})
export class RateLimitModule {}