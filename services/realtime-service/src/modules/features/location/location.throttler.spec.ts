import { LocationThrottler } from './location.throttler';
import { RedisService } from '@bts-soft/cache';
import { RATE_LIMITS } from '../../../common/common-types/constants';

describe('LocationThrottler', () => {
  let throttler: LocationThrottler;
  let store: Map<string, any>;
  let redis: any;

  beforeEach(() => {
    store = new Map();
    redis = {
      get: jest.fn(async (key: string) =>
        store.has(key) ? JSON.parse(JSON.stringify(store.get(key))) : null,
      ),
      set: jest.fn(async (key: string, value: any) => {
        store.set(key, value);
      }),
      del: jest.fn(async (key: string) => {
        store.delete(key);
      }),
    };
    throttler = new LocationThrottler(redis as RedisService);
  });

  it('allows up to the burst capacity immediately', async () => {
    for (let i = 0; i < RATE_LIMITS.LOCATION_BURST; i++) {
      await expect(throttler.allow('driver-1')).resolves.toBe(true);
    }
  });

  it('rejects when the bucket is exhausted', async () => {
    for (let i = 0; i < RATE_LIMITS.LOCATION_BURST; i++) {
      await throttler.allow('driver-1');
    }
    await expect(throttler.allow('driver-1')).resolves.toBe(false);
  });

  it('tracks buckets per driver independently', async () => {
    for (let i = 0; i < RATE_LIMITS.LOCATION_BURST; i++) {
      await throttler.allow('driver-1');
    }
    await expect(throttler.allow('driver-1')).resolves.toBe(false);
    await expect(throttler.allow('driver-2')).resolves.toBe(true);
  });

  it('refills tokens over time', async () => {
    let now = 1_000_000;
    jest.spyOn(Date, 'now').mockImplementation(() => now);

    for (let i = 0; i < RATE_LIMITS.LOCATION_BURST; i++) {
      await throttler.allow('driver-1');
    }
    expect(await throttler.allow('driver-1')).toBe(false);

    // window = burst/perSecond = 2s; advance far beyond full refill
    now += 5000;
    expect(await throttler.allow('driver-1')).toBe(true);
  });
});
