import { PresenceStore } from './presence.store';
import { RedisService } from '@bts-soft/cache';

describe('PresenceStore', () => {
  let store: PresenceStore;
  let redis: { set: jest.Mock; get: jest.Mock };
  let loggerError: jest.SpyInstance;

  beforeEach(() => {
    redis = { set: jest.fn(), get: jest.fn() };
    store = new PresenceStore(redis as unknown as RedisService);
    loggerError = jest
      .spyOn((store as any).logger, 'error')
      .mockImplementation();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('setState', () => {
    it('persists the state with TTL and lastSeen timestamp', async () => {
      redis.set.mockResolvedValue(undefined);
      const now = 1_000_000;
      jest.spyOn(Date, 'now').mockReturnValue(now);

      await store.setState('u1', 'ONLINE');

      expect(redis.set).toHaveBeenCalledWith(
        'ws:presence:u1',
        { state: 'ONLINE', lastSeen: now },
        90,
      );
    });

    it('logs and swallows redis failures', async () => {
      redis.set.mockRejectedValue(new Error('boom'));
      await expect(store.setState('u1', 'OFFLINE')).resolves.toBeUndefined();
      expect(loggerError).toHaveBeenCalled();
    });
  });

  describe('get', () => {
    it('returns the stored record', async () => {
      redis.get.mockResolvedValue({ state: 'IDLE', lastSeen: 123 });
      await expect(store.get('u1')).resolves.toEqual({
        state: 'IDLE',
        lastSeen: 123,
      });
    });

    it('returns null when missing or on redis failure', async () => {
      redis.get.mockResolvedValue(null);
      await expect(store.get('u1')).resolves.toBeNull();

      redis.get.mockRejectedValue(new Error('boom'));
      await expect(store.get('u1')).resolves.toBeNull();
    });
  });
});
