import { WebsocketRateLimiterService } from './websocket-rate-limiter.service';
import { RedisService } from '@bts-soft/cache';
import { RATE_LIMITS } from '../../../common/common-types/constants';

describe('WebsocketRateLimiterService', () => {
  let service: WebsocketRateLimiterService;
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
    service = new WebsocketRateLimiterService(redis as RedisService);
  });

  it('allows connect attempts up to the per-minute limit', async () => {
    for (let i = 0; i < RATE_LIMITS.CONNECT_PER_MINUTE; i++) {
      await expect(service.check('u1', 'connect')).resolves.toBe(true);
    }
    await expect(service.check('u1', 'connect')).resolves.toBe(false);
  });

  it('allows subscribe attempts up to the per-minute limit', async () => {
    for (let i = 0; i < RATE_LIMITS.SUBSCRIBE_PER_MINUTE; i++) {
      await expect(service.check('u1', 'subscribe')).resolves.toBe(true);
    }
    await expect(service.check('u1', 'subscribe')).resolves.toBe(false);
  });

  it('allows command attempts up to the per-minute limit', async () => {
    for (let i = 0; i < RATE_LIMITS.COMMANDS_PER_MINUTE; i++) {
      await expect(service.check('u1', 'command')).resolves.toBe(true);
    }
    await expect(service.check('u1', 'command')).resolves.toBe(false);
  });

  it('keeps buckets isolated per action and per user', async () => {
    for (let i = 0; i < RATE_LIMITS.CONNECT_PER_MINUTE; i++) {
      await service.check('u1', 'connect');
    }
    await expect(service.check('u1', 'connect')).resolves.toBe(false);
    await expect(service.check('u1', 'command')).resolves.toBe(true);
    await expect(service.check('u2', 'connect')).resolves.toBe(true);
  });

  it('refills after the sliding window passes', async () => {
    let now = 5_000_000;
    jest.spyOn(Date, 'now').mockImplementation(() => now);

    for (let i = 0; i < RATE_LIMITS.CONNECT_PER_MINUTE; i++) {
      await service.check('u1', 'connect');
    }
    expect(await service.check('u1', 'connect')).toBe(false);

    now += 61_000;
    expect(await service.check('u1', 'connect')).toBe(true);
  });
});
