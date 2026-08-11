import { applyDecorators, UseGuards } from '@nestjs/common';
import { RedisRateLimiter } from '../guard/redis-rate-limiter.guard';
import { RateLimiterConfig } from '@bts-soft/validation';

export function RedisRateLimit(config: RateLimiterConfig) {
  return applyDecorators(UseGuards(RedisRateLimiter(config)));
}
