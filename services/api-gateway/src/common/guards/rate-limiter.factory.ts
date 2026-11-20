import { CanActivate } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import {
  RateLimiter,
  RateLimiterAlgorithm,
  RedisStore,
} from '@bts-soft/validation';

/**
 * Builds the global `RateLimiter` guard instance used as an `APP_GUARD`.
 * Extracted verbatim from the historical inline `useFactory` in `app.module.ts`.
 */
export function createRateLimiterGuard(
  redisService: RedisService,
): CanActivate {
  return new ((RateLimiter as any)(
    {
      algorithm: RateLimiterAlgorithm.TOKEN_BUCKET,
      limit: Number(process.env.RATE_LIMIT_LIMIT) || 100,
      windowMs: Number(process.env.RATE_LIMIT_WINDOW_MS) || 60_000,
      skipIntrospection: true,
      store: new RedisStore(redisService),
    },
    new RedisStore(redisService),
  ))();
}
