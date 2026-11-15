import { IdempotencyStore } from './idempotency.store';
import { RedisService } from '@bts-soft/cache';

describe('IdempotencyStore', () => {
  let store: IdempotencyStore;
  let redis: { setNX: jest.Mock };

  beforeEach(() => {
    redis = { setNX: jest.fn() };
    store = new IdempotencyStore(redis as unknown as RedisService);
    jest.restoreAllMocks();
  });

  it('claims a new command id', async () => {
    redis.setNX.mockResolvedValue(true);
    await expect(store.claim('cmd-12345678')).resolves.toBe('new');
    expect(redis.setNX).toHaveBeenCalledWith(
      'realtime:idempotency:cmd-12345678',
      '1',
      86400,
    );
  });

  it('reports a duplicate command id', async () => {
    redis.setNX.mockResolvedValue(false);
    await expect(store.claim('cmd-12345678')).resolves.toBe('duplicate');
  });

  it('falls back to new on redis failure (fail-open)', async () => {
    redis.setNX.mockRejectedValue(new Error('redis down'));
    const warn = jest.spyOn((store as any).logger, 'warn');
    await expect(store.claim('cmd-12345678')).resolves.toBe('new');
    expect(warn).toHaveBeenCalled();
  });
});
