import { EventDeduplicator } from './event-deduplicator';
import { RedisService } from '@bts-soft/cache';

describe('EventDeduplicator', () => {
  let deduplicator: EventDeduplicator;
  let redis: { setNX: jest.Mock; set: jest.Mock };

  beforeEach(() => {
    redis = { setNX: jest.fn(), set: jest.fn() };
    deduplicator = new EventDeduplicator(redis as unknown as RedisService);
    jest.restoreAllMocks();
  });

  describe('isDuplicate', () => {
    it('returns false when the event id is new (SETNX created)', async () => {
      redis.setNX.mockResolvedValue(true);
      await expect(deduplicator.isDuplicate('evt-1')).resolves.toBe(false);
      expect(redis.setNX).toHaveBeenCalledWith(
        'realtime:processed:evt-1',
        '1',
        86400,
      );
    });

    it('returns true when the event id was already processed', async () => {
      redis.setNX.mockResolvedValue(false);
      await expect(deduplicator.isDuplicate('evt-1')).resolves.toBe(true);
    });

    it('treats redis failures as a new event', async () => {
      redis.setNX.mockRejectedValue(new Error('redis down'));
      const warn = jest.spyOn((deduplicator as any).logger, 'warn');
      await expect(deduplicator.isDuplicate('evt-1')).resolves.toBe(false);
      expect(warn).toHaveBeenCalled();
    });
  });

  describe('markProcessed', () => {
    it('stores a processed marker with TTL', async () => {
      redis.set.mockResolvedValue(undefined);
      await deduplicator.markProcessed('evt-2');
      expect(redis.set).toHaveBeenCalledWith(
        'realtime:processed:evt-2',
        '1',
        86400,
      );
    });

    it('swallows redis failures', async () => {
      redis.set.mockRejectedValue(new Error('redis down'));
      const warn = jest.spyOn((deduplicator as any).logger, 'warn');
      await expect(
        deduplicator.markProcessed('evt-2'),
      ).resolves.toBeUndefined();
      expect(warn).toHaveBeenCalled();
    });
  });
});
