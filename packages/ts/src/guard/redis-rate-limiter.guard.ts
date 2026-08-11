import { Injectable, CanActivate, ExecutionContext, mixin, Type, HttpException, Inject } from '@nestjs/common';
import { createAlgorithm, RedisStore, RateLimiterConfig, RateLimiterResult } from '@bts-soft/validation';
import { GqlExecutionContext } from '@nestjs/graphql';

function resolveRequest(context: ExecutionContext) {
  const contextType = context.getType<any>();
  if (contextType === 'graphql') {
    const gqlCtx = GqlExecutionContext.create(context);
    const ctx = gqlCtx.getContext();
    return { req: ctx.req ?? ctx.request, res: ctx.res ?? ctx.response };
  }
  const http = context.switchToHttp();
  return { req: http.getRequest(), res: http.getResponse() };
}

function applyRateLimitHeaders(res: any, result: RateLimiterResult) {
  if (!res || typeof res.setHeader !== 'function') return;
  res.setHeader('X-RateLimit-Limit', String(result.limit));
  res.setHeader('X-RateLimit-Remaining', String(result.remaining));
  res.setHeader('X-RateLimit-Reset', String(result.resetAtSeconds));
  if (!result.allowed && result.retryAfterSeconds > 0) {
    res.setHeader('Retry-After', String(result.retryAfterSeconds));
  }
}

export function RedisRateLimiter(config: RateLimiterConfig): Type<CanActivate> {
  const DEFAULT_MESSAGE = 'Too many requests, please try again later.';
  const DEFAULT_STATUS = 429;
  const skipIntrospection = config.skipIntrospection !== false;

  @Injectable()
  class RedisRateLimiterGuard implements CanActivate {
    private readonly algorithm: any;

    constructor(
      @Inject('SHARED_REDIS_SERVICE') private readonly redisService: any,
    ) {
      const redisClient = (this.redisService as any).redisClient || this.redisService['redisClient'];
      const store = new RedisStore(redisClient);
      this.algorithm = createAlgorithm(config, store);
    }

    async canActivate(context: ExecutionContext): Promise<boolean> {
      const { req, res } = resolveRequest(context);
      if (skipIntrospection && req?.body?.query?.trim().startsWith('query IntrospectionQuery')) {
        return true;
      }
      
      const key = config.keyExtractor ? config.keyExtractor(req) : (req.ip || req.headers['x-forwarded-for'] || 'default-ip');
      
      const result = await this.algorithm.consume(key);
      applyRateLimitHeaders(res, result);
      
      if (!result.allowed) {
        throw new HttpException(config.message ?? DEFAULT_MESSAGE, DEFAULT_STATUS);
      }
      return true;
    }
  }

  return mixin(RedisRateLimiterGuard);
}
