import { CanActivate, Type } from '@nestjs/common';
import { createAlgorithm, RedisStore, RateLimiterConfig } from '@bts-soft/validation';
import { RedisRateLimit } from './redis-rate-limit.decorator';

jest.mock('@bts-soft/validation', () => ({
  __esModule: true,
  RedisStore: jest.fn(),
  createAlgorithm: jest.fn(),
}));

describe('RedisRateLimit decorator', () => {
  const config = {
    algorithm: 'TOKEN_BUCKET',
    limit: 5,
    windowMs: 30000,
  } as unknown as RateLimiterConfig;

  beforeEach(() => {
    (RedisStore as unknown as jest.Mock).mockClear();
    (createAlgorithm as unknown as jest.Mock).mockReset();
    (createAlgorithm as unknown as jest.Mock).mockReturnValue({ consume: jest.fn() });
  });

  const applyToMethod = () => {
    class Target {
      handle() {
        return undefined;
      }
    }
    const descriptor = Object.getOwnPropertyDescriptor(Target.prototype, 'handle')!;
    RedisRateLimit(config)(Target.prototype, 'handle', descriptor);
    return descriptor;
  };

  it('binds a single rate limiter guard to the handler', () => {
    const descriptor = applyToMethod();

    const guards = Reflect.getMetadata('__guards__', descriptor.value);

    expect(guards).toHaveLength(1);
    expect(typeof guards[0]).toBe('function');
  });

  it('binds a rate limiter guard to a class', () => {
    class Target {}

    RedisRateLimit(config)(Target);

    const guards = Reflect.getMetadata('__guards__', Target);

    expect(guards).toHaveLength(1);
    expect(typeof guards[0]).toBe('function');
  });

  it('creates a guard that wires the redis client and algorithm config', () => {
    const descriptor = applyToMethod();
    const Guard = Reflect.getMetadata('__guards__', descriptor.value)[0] as Type<CanActivate>;
    const fakeClient = { get: jest.fn() };

    const guard = new Guard({ redisClient: fakeClient }) as any;

    expect(RedisStore).toHaveBeenCalledWith(fakeClient);
    expect(createAlgorithm).toHaveBeenCalledWith(config, expect.anything());
    expect(typeof guard.canActivate).toBe('function');
  });

  it('creates independent guard classes per decoration', () => {
    const first = applyToMethod();
    const second = applyToMethod();

    const firstGuards = Reflect.getMetadata('__guards__', first.value);
    const secondGuards = Reflect.getMetadata('__guards__', second.value);

    expect(firstGuards[0]).not.toBe(secondGuards[0]);
  });
});
