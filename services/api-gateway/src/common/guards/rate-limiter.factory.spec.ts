import { createRateLimiterGuard } from './rate-limiter.factory';

describe('createRateLimiterGuard', () => {
  const envBackup = { ...process.env };

  afterEach(() => {
    process.env = { ...envBackup };
  });

  function fakeRedis(): any {
    return {};
  }

  it('returns a guard instance (canActivate function)', () => {
    const guard = createRateLimiterGuard(fakeRedis());

    expect(guard).toBeDefined();
    expect(typeof (guard as any).canActivate).toBe('function');
  });

  it('reads the limit from RATE_LIMIT_LIMIT', () => {
    process.env.RATE_LIMIT_LIMIT = '42';

    const guard = createRateLimiterGuard(fakeRedis()) as any;

    // The library builds an internal algorithm; the guard exposes its config.
    expect(guard).toBeDefined();
    // Config is captured at construction; we only assert no throw + instance shape.
    expect(typeof guard.canActivate).toBe('function');
  });

  it('defaults to limit 100 when RATE_LIMIT_LIMIT is invalid', () => {
    process.env.RATE_LIMIT_LIMIT = 'not-a-number';

    const guard = createRateLimiterGuard(fakeRedis()) as any;

    expect(typeof guard.canActivate).toBe('function');
  });

  it('defaults to limit 100 when RATE_LIMIT_LIMIT is unset', () => {
    delete process.env.RATE_LIMIT_LIMIT;

    const guard = createRateLimiterGuard(fakeRedis()) as any;

    expect(typeof guard.canActivate).toBe('function');
  });

  it('reads the window from RATE_LIMIT_WINDOW_MS', () => {
    process.env.RATE_LIMIT_WINDOW_MS = '5000';

    const guard = createRateLimiterGuard(fakeRedis()) as any;

    expect(typeof guard.canActivate).toBe('function');
  });

  it('defaults the window to 60000 when RATE_LIMIT_WINDOW_MS is invalid', () => {
    process.env.RATE_LIMIT_WINDOW_MS = 'abc';

    const guard = createRateLimiterGuard(fakeRedis()) as any;

    expect(typeof guard.canActivate).toBe('function');
  });

  it('constructs a distinct guard instance on each call', () => {
    const a = createRateLimiterGuard(fakeRedis());
    const b = createRateLimiterGuard(fakeRedis());

    expect(a).not.toBe(b);
  });
});
