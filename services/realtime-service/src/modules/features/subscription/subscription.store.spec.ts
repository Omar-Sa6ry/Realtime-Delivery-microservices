import { SubscriptionStore } from './subscription.store';
import { RedisService } from '@bts-soft/cache';

describe('SubscriptionStore', () => {
  let store: SubscriptionStore;
  let redis: {
    sAdd: jest.Mock;
    sRem: jest.Mock;
    sMembers: jest.Mock;
    sIsMember: jest.Mock;
    del: jest.Mock;
  };

  beforeEach(() => {
    redis = {
      sAdd: jest.fn().mockResolvedValue(1),
      sRem: jest.fn().mockResolvedValue(1),
      sMembers: jest.fn().mockResolvedValue([]),
      sIsMember: jest.fn().mockResolvedValue(false),
      del: jest.fn().mockResolvedValue(undefined),
    };
    store = new SubscriptionStore(redis as unknown as RedisService);
  });

  it('addSubscriber maintains both indexes', async () => {
    await store.addSubscriber('d1', 's1');
    expect(redis.sAdd).toHaveBeenCalledWith('ws:delivery:d1:subscribers', 's1');
    expect(redis.sAdd).toHaveBeenCalledWith('ws:socket:s1:subscriptions', 'd1');
  });

  it('removeSubscriber removes from both indexes', async () => {
    await store.removeSubscriber('d1', 's1');
    expect(redis.sRem).toHaveBeenCalledWith('ws:delivery:d1:subscribers', 's1');
    expect(redis.sRem).toHaveBeenCalledWith('ws:socket:s1:subscriptions', 'd1');
  });

  it('getDeliverySubscribers returns socket ids', async () => {
    redis.sMembers.mockResolvedValue(['s1', 's2']);
    await expect(store.getDeliverySubscribers('d1')).resolves.toEqual([
      's1',
      's2',
    ]);
    expect(redis.sMembers).toHaveBeenCalledWith('ws:delivery:d1:subscribers');
  });

  it('isSubscribed checks set membership', async () => {
    redis.sIsMember.mockResolvedValue(true);
    await expect(store.isSubscribed('d1', 's1')).resolves.toBe(true);
    expect(redis.sIsMember).toHaveBeenCalledWith(
      'ws:delivery:d1:subscribers',
      's1',
    );
  });

  it('removeSocketFromAllSubscriptions cleans every delivery room and the reverse index', async () => {
    redis.sMembers.mockResolvedValue(['d1', 'd2']);

    const removed = await store.removeSocketFromAllSubscriptions('s1');

    expect(removed).toEqual(['d1', 'd2']);
    expect(redis.sRem).toHaveBeenCalledWith('ws:delivery:d1:subscribers', 's1');
    expect(redis.sRem).toHaveBeenCalledWith('ws:delivery:d2:subscribers', 's1');
    expect(redis.del).toHaveBeenCalledWith('ws:socket:s1:subscriptions');
  });

  it('handles sockets without subscriptions', async () => {
    redis.sMembers.mockResolvedValue([]);
    await expect(store.removeSocketFromAllSubscriptions('s1')).resolves.toEqual(
      [],
    );
    expect(redis.del).toHaveBeenCalledWith('ws:socket:s1:subscriptions');
  });
});
