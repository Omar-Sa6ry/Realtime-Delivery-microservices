import { ConnectionStateStore } from './connection-state.store';
import { RedisService } from '@bts-soft/cache';
import { ConnectionContext } from './connection.types';
import { Role } from '@delivery-micro/shard';

describe('ConnectionStateStore', () => {
  let store: ConnectionStateStore;
  let redis: Record<string, jest.Mock>;
  let loggerError: jest.SpyInstance;

  const ctx: ConnectionContext = {
    socketId: 's1',
    nodeId: 'n1',
    userId: 'u1',
    userRole: Role.DRIVER,
    connectedAt: 1,
    lastHeartbeatAt: 1,
  };

  beforeEach(() => {
    redis = {
      set: jest.fn().mockResolvedValue(undefined),
      get: jest.fn(),
      del: jest.fn().mockResolvedValue(undefined),
      sAdd: jest.fn().mockResolvedValue(1),
      sRem: jest.fn().mockResolvedValue(1),
      sCard: jest.fn().mockResolvedValue(1),
      expire: jest.fn().mockResolvedValue(true),
      geoRemove: jest.fn().mockResolvedValue(1),
    };
    store = new ConnectionStateStore(redis as unknown as RedisService);
    loggerError = jest
      .spyOn((store as any).logger, 'error')
      .mockImplementation();
  });

  afterEach(() => jest.restoreAllMocks());

  describe('register', () => {
    it('writes connection metadata and set memberships', async () => {
      await store.register(ctx);

      expect(redis.set).toHaveBeenCalledWith('ws:connection:s1', ctx, 60);
      expect(redis.sAdd).toHaveBeenCalledWith('ws:user:u1:sockets', 's1');
      expect(redis.sAdd).toHaveBeenCalledWith('ws:role:driver:sockets', 's1');
      expect(redis.sAdd).toHaveBeenCalledWith('ws:node:n1:connections', 's1');
    });

    it('logs and swallows failures', async () => {
      redis.set.mockRejectedValue(new Error('boom'));
      await expect(store.register(ctx)).resolves.toBeUndefined();
      expect(loggerError).toHaveBeenCalled();
    });
  });

  describe('unregister', () => {
    it('removes connection metadata and set memberships', async () => {
      await store.unregister(ctx);

      expect(redis.del).toHaveBeenCalledWith('ws:connection:s1');
      expect(redis.sRem).toHaveBeenCalledWith('ws:user:u1:sockets', 's1');
      expect(redis.sRem).toHaveBeenCalledWith('ws:role:driver:sockets', 's1');
      expect(redis.sRem).toHaveBeenCalledWith('ws:node:n1:connections', 's1');
    });

    it('logs and swallows failures', async () => {
      redis.del.mockRejectedValue(new Error('boom'));
      await expect(store.unregister(ctx)).resolves.toBeUndefined();
      expect(loggerError).toHaveBeenCalled();
    });
  });

  it('refreshHeartbeat renews the connection TTL', async () => {
    await store.refreshHeartbeat('s1');
    expect(redis.expire).toHaveBeenCalledWith('ws:connection:s1', 60);
  });

  it('refreshHeartbeat logs and swallows failures', async () => {
    redis.expire.mockRejectedValue(new Error('boom'));
    await expect(store.refreshHeartbeat('s1')).resolves.toBeUndefined();
    expect(loggerError).toHaveBeenCalled();
  });

  it('userConnectionCount returns the set cardinality', async () => {
    redis.sCard.mockResolvedValue(4);
    await expect(store.userConnectionCount('u1')).resolves.toBe(4);
    expect(redis.sCard).toHaveBeenCalledWith('ws:user:u1:sockets');
  });

  it('userConnectionCount returns 0 on failure', async () => {
    redis.sCard.mockRejectedValue(new Error('boom'));
    await expect(store.userConnectionCount('u1')).resolves.toBe(0);
  });

  it('userConnectionCountsByRole counts each role set', async () => {
    redis.sCard.mockImplementation(async (key: string) => {
      if (key.includes('user')) return 2;
      if (key.includes('driver')) return 3;
      return 1;
    });

    await expect(store.userConnectionCountsByRole()).resolves.toEqual({
      user: 2,
      driver: 3,
      admin: 1,
    });
  });

  it('userConnectionCountsByRole maps failures to 0', async () => {
    redis.sCard.mockRejectedValue(new Error('boom'));
    await expect(store.userConnectionCountsByRole()).resolves.toEqual({
      user: 0,
      driver: 0,
      admin: 0,
    });
  });

  it('totalConnections sums role sets', async () => {
    redis.sCard.mockResolvedValue(2);
    await expect(store.totalConnections()).resolves.toBe(6);
  });

  it('totalConnections returns 0 on failure', async () => {
    redis.sCard.mockRejectedValue(new Error('boom'));
    await expect(store.totalConnections()).resolves.toBe(0);
  });

  it('getContext reads and parses the connection context', async () => {
    redis.get.mockResolvedValue(ctx);
    await expect(store.getContext('s1')).resolves.toEqual(ctx);
    expect(redis.get).toHaveBeenCalledWith('ws:connection:s1');
  });

  it('getContext returns null on failure', async () => {
    redis.get.mockRejectedValue(new Error('boom'));
    await expect(store.getContext('s1')).resolves.toBeNull();
  });

  it('removeDriverGeoLocation removes the driver from the geo index', async () => {
    await store.removeDriverGeoLocation('drv1');
    expect(redis.geoRemove).toHaveBeenCalledWith('drivers:locations', 'drv1');
  });

  it('removeDriverGeoLocation logs and swallows failures', async () => {
    redis.geoRemove.mockRejectedValue(new Error('boom'));
    await expect(
      store.removeDriverGeoLocation('drv1'),
    ).resolves.toBeUndefined();
    expect(loggerError).toHaveBeenCalled();
  });
});
