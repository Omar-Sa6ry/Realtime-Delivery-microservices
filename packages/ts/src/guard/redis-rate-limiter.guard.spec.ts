import { HttpException } from '@nestjs/common';
import type { CanActivate, ExecutionContext, Type } from '@nestjs/common';
import { GqlExecutionContext } from '@nestjs/graphql';
import { createAlgorithm, RedisStore, RateLimiterConfig } from '@bts-soft/validation';
import { RedisRateLimiter } from './redis-rate-limiter.guard';

jest.mock('@nestjs/graphql', () => ({
  __esModule: true,
  GqlExecutionContext: { create: jest.fn() },
}));

jest.mock('@bts-soft/validation', () => ({
  __esModule: true,
  RedisStore: jest.fn(),
  createAlgorithm: jest.fn(),
  RateLimiterAlgorithm: { TOKEN_BUCKET: 'TOKEN_BUCKET' },
}));

describe('RedisRateLimiter', () => {
  let consume: jest.Mock;
  let fakeClient: any;
  let redisService: any;

  const baseConfig = {
    algorithm: 'TOKEN_BUCKET',
    limit: 3,
    windowMs: 60000,
  } as unknown as RateLimiterConfig;

  const allowedResult = (overrides: any = {}) => ({
    allowed: true,
    limit: 3,
    remaining: 2,
    retryAfterSeconds: 0,
    resetAtSeconds: 123,
    ...overrides,
  });

  const instantiate = (config: RateLimiterConfig) => {
    const Guard = RedisRateLimiter(config) as unknown as Type<CanActivate>;
    const guard = new Guard(redisService) as any;
    return { Guard, guard };
  };

  const httpContext = (req: any, res: any): ExecutionContext =>
    ({
      getType: () => 'http',
      switchToHttp: () => ({
        getRequest: () => req,
        getResponse: () => res,
      }),
    }) as unknown as ExecutionContext;

  const gqlContext = (contextValue: any): ExecutionContext =>
    ({
      getType: () => 'graphql',
      getHandler: () => () => undefined,
      getClass: () => class {},
    }) as unknown as ExecutionContext;

  const useGqlContext = (contextValue: any) => {
    (GqlExecutionContext.create as unknown as jest.Mock).mockReturnValue({
      getContext: () => contextValue,
    });
  };

  const captureRejection = async (promise: Promise<boolean>) => {
    try {
      await promise;
      return undefined;
    } catch (err) {
      return err as any;
    }
  };

  beforeEach(() => {
    fakeClient = { get: jest.fn(), set: jest.fn(), del: jest.fn() };
    redisService = { redisClient: fakeClient };
    consume = jest.fn().mockResolvedValue(allowedResult());
    (RedisStore as unknown as jest.Mock).mockClear();
    (createAlgorithm as unknown as jest.Mock).mockReset();
    (createAlgorithm as unknown as jest.Mock).mockReturnValue({ consume });
    (GqlExecutionContext.create as unknown as jest.Mock).mockReset();
  });

  it('wires redis client into store and algorithm on construction', () => {
    const { guard } = instantiate(baseConfig);

    expect(guard).toBeDefined();
    expect(RedisStore).toHaveBeenCalledWith(fakeClient);
    expect(createAlgorithm).toHaveBeenCalledWith(baseConfig, expect.anything());
  });

  it('allows request and applies rate limit headers over http', async () => {
    const { guard } = instantiate(baseConfig);
    const setHeader = jest.fn();
    const req = { ip: '10.0.0.1', headers: {} };
    const res = { setHeader };

    const result = await guard.canActivate(httpContext(req, res));

    expect(result).toBe(true);
    expect(consume).toHaveBeenCalledWith('10.0.0.1');
    expect(setHeader).toHaveBeenCalledWith('X-RateLimit-Limit', '3');
    expect(setHeader).toHaveBeenCalledWith('X-RateLimit-Remaining', '2');
    expect(setHeader).toHaveBeenCalledWith('X-RateLimit-Reset', '123');
    expect(setHeader).not.toHaveBeenCalledWith('Retry-After', expect.anything());
  });

  it('falls back to x-forwarded-for header when ip is missing', async () => {
    const { guard } = instantiate(baseConfig);
    const req = { headers: { 'x-forwarded-for': '203.0.113.7' } };

    await guard.canActivate(httpContext(req, { setHeader: jest.fn() }));

    expect(consume).toHaveBeenCalledWith('203.0.113.7');
  });

  it('uses default-ip key when neither ip nor forwarded header exists', async () => {
    const { guard } = instantiate(baseConfig);
    const req = { headers: {} };

    await guard.canActivate(httpContext(req, { setHeader: jest.fn() }));

    expect(consume).toHaveBeenCalledWith('default-ip');
  });

  it('uses custom key extractor when provided', async () => {
    const keyExtractor = jest.fn((req: any) => `user:${req.headers['x-user-id']}`);
    const { guard } = instantiate({ ...baseConfig, keyExtractor } as any);
    const req = { headers: { 'x-user-id': 'u-77' } };

    await guard.canActivate(httpContext(req, { setHeader: jest.fn() }));

    expect(keyExtractor).toHaveBeenCalledWith(req);
    expect(consume).toHaveBeenCalledWith('user:u-77');
  });

  it('throws 429 with default message and sets Retry-After when blocked', async () => {
    consume.mockResolvedValue(
      allowedResult({ allowed: false, remaining: 0, retryAfterSeconds: 45 }),
    );
    const { guard } = instantiate(baseConfig);
    const setHeader = jest.fn();

    const err = await captureRejection(
      guard.canActivate(httpContext({ ip: '1.1.1.1', headers: {} }, { setHeader })),
    );

    expect(err).toBeInstanceOf(HttpException);
    expect(err.getStatus()).toBe(429);
    expect(err.message).toBe('Too many requests, please try again later.');
    expect(setHeader).toHaveBeenCalledWith('Retry-After', '45');
  });

  it('uses configured message when rate limit is exceeded', async () => {
    consume.mockResolvedValue(
      allowedResult({ allowed: false, remaining: 0, retryAfterSeconds: 10 }),
    );
    const { guard } = instantiate({ ...baseConfig, message: 'Slow down' });

    const err = await captureRejection(
      guard.canActivate(httpContext({ ip: '1.1.1.1', headers: {} }, { setHeader: jest.fn() })),
    );

    expect(err.message).toBe('Slow down');
    expect(err.getStatus()).toBe(429);
  });

  it('omits Retry-After header when retryAfterSeconds is zero', async () => {
    consume.mockResolvedValue(
      allowedResult({ allowed: false, remaining: 0, retryAfterSeconds: 0 }),
    );
    const { guard } = instantiate(baseConfig);
    const setHeader = jest.fn();

    await captureRejection(
      guard.canActivate(httpContext({ ip: '1.1.1.1', headers: {} }, { setHeader })),
    );

    expect(setHeader).not.toHaveBeenCalledWith('Retry-After', expect.anything());
  });

  it('does not crash when response has no setHeader function', async () => {
    const { guard } = instantiate(baseConfig);

    const result = await guard.canActivate(
      httpContext({ ip: '1.1.1.1', headers: {} }, {}),
    );

    expect(result).toBe(true);
  });

  it('does not crash when response is missing entirely', async () => {
    const { guard } = instantiate(baseConfig);

    const result = await guard.canActivate(httpContext({ ip: '1.1.1.1', headers: {} }, undefined));

    expect(result).toBe(true);
  });

  it('resolves req and res from graphql context', async () => {
    const { guard } = instantiate(baseConfig);
    const setHeader = jest.fn();
    useGqlContext({ req: { ip: '8.8.8.8', headers: {} }, res: { setHeader } });

    const result = await guard.canActivate(gqlContext({}));

    expect(result).toBe(true);
    expect(consume).toHaveBeenCalledWith('8.8.8.8');
    expect(setHeader).toHaveBeenCalledWith('X-RateLimit-Limit', '3');
    expect(GqlExecutionContext.create).toHaveBeenCalled();
  });

  it('falls back to request/response keys in graphql context', async () => {
    const { guard } = instantiate(baseConfig);
    useGqlContext({ request: { ip: '7.7.7.7', headers: {} }, response: { setHeader: jest.fn() } });

    const result = await guard.canActivate(gqlContext({}));

    expect(result).toBe(true);
    expect(consume).toHaveBeenCalledWith('7.7.7.7');
  });

  it('skips introspection query by default', async () => {
    const { guard } = instantiate(baseConfig);
    const req = {
      ip: '1.1.1.1',
      headers: {},
      body: { query: 'query IntrospectionQuery { __schema { types { name } } }' },
    };

    const result = await guard.canActivate(httpContext(req, { setHeader: jest.fn() }));

    expect(result).toBe(true);
    expect(consume).not.toHaveBeenCalled();
  });

  it('trims query before checking introspection', async () => {
    const { guard } = instantiate(baseConfig);
    const req = {
      ip: '1.1.1.1',
      headers: {},
      body: { query: '   query IntrospectionQuery { __schema }   ' },
    };

    const result = await guard.canActivate(httpContext(req, { setHeader: jest.fn() }));

    expect(result).toBe(true);
    expect(consume).not.toHaveBeenCalled();
  });

  it('does not skip introspection when skipIntrospection is false', async () => {
    const { guard } = instantiate({ ...baseConfig, skipIntrospection: false });
    const req = {
      ip: '1.1.1.1',
      headers: {},
      body: { query: 'query IntrospectionQuery { __schema }' },
    };

    const result = await guard.canActivate(httpContext(req, { setHeader: jest.fn() }));

    expect(result).toBe(true);
    expect(consume).toHaveBeenCalledWith('1.1.1.1');
  });

  it('continues when request has no body', async () => {
    const { guard } = instantiate(baseConfig);
    const req = { ip: '5.5.5.5', headers: {} };

    const result = await guard.canActivate(httpContext(req, { setHeader: jest.fn() }));

    expect(result).toBe(true);
    expect(consume).toHaveBeenCalledWith('5.5.5.5');
  });

  it('skips introspection check when request is missing and still rate limits', async () => {
    const keyExtractor = jest.fn(() => 'no-req-key');
    const { guard } = instantiate({ ...baseConfig, keyExtractor } as any);
    useGqlContext({});

    const result = await guard.canActivate(gqlContext({}));

    expect(result).toBe(true);
    expect(keyExtractor).toHaveBeenCalledWith(undefined);
    expect(consume).toHaveBeenCalledWith('no-req-key');
  });
});
