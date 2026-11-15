import { ConnectionService } from './connection.service';
import { ConnectionRegistry } from './connection.registry';
import { ConnectionStateStore } from './connection-state.store';
import { PresenceService } from '../../features/presence/presence.service';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import { IJwtPayload, Role } from '@delivery-micro/shard';
import type { WebSocket } from 'ws';

describe('ConnectionService', () => {
  let service: ConnectionService;
  let registry: ConnectionRegistry;
  let stateStore: {
    register: jest.Mock;
    unregister: jest.Mock;
    userConnectionCount: jest.Mock;
    userConnectionCountsByRole: jest.Mock;
    removeDriverGeoLocation: jest.Mock;
  };
  let presence: { setOnline: jest.Mock; setOffline: jest.Mock; get: jest.Mock };
  let metrics: {
    connectionCounter: { inc: jest.Mock };
    activeConnections: { set: jest.Mock };
  };

  const payload: IJwtPayload = { userId: 'u1', role: Role.DRIVER };

  const makeSocket = (socketId: string, userId = 'u1', role = Role.DRIVER) =>
    ({
      data: {
        socketId,
        nodeId: 'n1',
        userId,
        userRole: role,
        sessionId: 's',
        email: 'e@x.com',
        connectedAt: 1,
        lastPongAt: Date.now(),
        dirty: false,
      },
      close: jest.fn(),
    }) as unknown as WebSocket & { data: any };

  beforeEach(() => {
    registry = new ConnectionRegistry();
    stateStore = {
      register: jest.fn().mockResolvedValue(undefined),
      unregister: jest.fn().mockResolvedValue(undefined),
      userConnectionCount: jest.fn().mockResolvedValue(0),
      userConnectionCountsByRole: jest.fn().mockResolvedValue({
        user: 1,
        driver: 2,
        admin: 3,
      }),
      removeDriverGeoLocation: jest.fn().mockResolvedValue(undefined),
    };
    presence = {
      setOnline: jest.fn().mockResolvedValue(undefined),
      setOffline: jest.fn().mockResolvedValue(undefined),
      get: jest.fn(),
    };
    metrics = {
      connectionCounter: { inc: jest.fn() },
      activeConnections: { set: jest.fn() },
    };

    service = new ConnectionService(
      registry,
      stateStore as unknown as ConnectionStateStore,
      presence as unknown as PresenceService,
      metrics as unknown as RealtimeMetricsService,
    );
  });

  describe('register', () => {
    it('registers context, stores state, marks presence online and tracks metrics', async () => {
      const context = await service.register(
        makeSocket('raw') as any,
        'n1',
        payload,
      );

      expect(context.userId).toBe('u1');
      expect(context.userRole).toBe(Role.DRIVER);
      expect(stateStore.register).toHaveBeenCalledWith(context);
      expect(presence.setOnline).toHaveBeenCalledWith('u1');
      expect(metrics.connectionCounter.inc).toHaveBeenCalledWith({
        result: 'accepted',
      });
      expect(metrics.activeConnections.set).toHaveBeenCalledWith(1);
      expect(registry.get(context.socketId)).toBeDefined();
    });
  });

  describe('unregister', () => {
    it('unregisters state and removes registry entry', async () => {
      const context = await service.register(
        makeSocket('raw') as any,
        'n1',
        payload,
      );
      stateStore.userConnectionCount.mockResolvedValue(1);

      await service.unregister(makeSocket(context.socketId));

      expect(stateStore.unregister).toHaveBeenCalled();
      expect(presence.setOffline).not.toHaveBeenCalled();
      expect(registry.get(context.socketId)).toBeUndefined();
      expect(metrics.connectionCounter.inc).toHaveBeenCalledWith({
        result: 'closed',
      });
    });

    it('marks the user offline and removes driver geo when the last socket closes', async () => {
      const context = await service.register(
        makeSocket('raw') as any,
        'n1',
        payload,
      );
      stateStore.userConnectionCount.mockResolvedValue(0);

      await service.unregister(makeSocket(context.socketId));

      expect(presence.setOffline).toHaveBeenCalledWith('u1');
      expect(stateStore.removeDriverGeoLocation).toHaveBeenCalledWith('u1');
    });

    it('does not remove driver geo for non-driver roles', async () => {
      const context = await service.register(
        makeSocket('raw', 'u2', Role.USER) as any,
        'n1',
        { userId: 'u2', role: Role.USER },
      );
      stateStore.userConnectionCount.mockResolvedValue(0);

      await service.unregister(makeSocket(context.socketId, 'u2', Role.USER));

      expect(presence.setOffline).toHaveBeenCalledWith('u2');
      expect(stateStore.removeDriverGeoLocation).not.toHaveBeenCalled();
    });
  });

  describe('local socket queries', () => {
    it('filters sockets by user and role', async () => {
      await service.register(
        makeSocket('raw1', 'u1', Role.DRIVER) as any,
        'n1',
        {
          userId: 'u1',
          role: Role.DRIVER,
        },
      );
      await service.register(makeSocket('raw2', 'u2', Role.USER) as any, 'n1', {
        userId: 'u2',
        role: Role.USER,
      });

      expect(service.getLocalSocketsByUser('u1')).toHaveLength(1);
      expect(service.getLocalSocketsByRole(Role.USER)).toHaveLength(1);
      expect(service.getLocalSocketCount()).toBe(2);
      expect(service.getLocalSockets()).toHaveLength(2);
    });

    it('getLocalConnection returns a socket by id or undefined', async () => {
      const context = await service.register(
        makeSocket('raw') as any,
        'n1',
        payload,
      );
      expect(service.getLocalConnection(context.socketId)).toBeDefined();
      expect(service.getLocalConnection('missing')).toBeUndefined();
    });
  });

  describe('getUserConnectionStatus', () => {
    it('reports connection count and last seen', async () => {
      stateStore.userConnectionCount.mockResolvedValue(2);
      presence.get.mockResolvedValue({
        state: 'ONLINE',
        lastSeen: 1700000000000,
      });

      await expect(service.getUserConnectionStatus('u1')).resolves.toEqual({
        isConnected: true,
        lastSeen: new Date(1700000000000).toISOString(),
        connectionCount: 2,
      });
    });

    it('reports disconnected with null lastSeen when presence is missing', async () => {
      stateStore.userConnectionCount.mockResolvedValue(0);
      presence.get.mockResolvedValue(null);

      await expect(service.getUserConnectionStatus('u1')).resolves.toEqual({
        isConnected: false,
        lastSeen: null,
        connectionCount: 0,
      });
    });
  });

  it('getActiveConnectionCounts aggregates by role', async () => {
    await expect(service.getActiveConnectionCounts()).resolves.toEqual({
      total: 6,
      byRole: { customers: 1, drivers: 2, admins: 3 },
    });
  });

  it('getActiveConnectionCounts treats missing roles as zero', async () => {
    stateStore.userConnectionCountsByRole.mockResolvedValue({});
    await expect(service.getActiveConnectionCounts()).resolves.toEqual({
      total: 0,
      byRole: { customers: 0, drivers: 0, admins: 0 },
    });
  });
});
