import { HeartbeatService } from './heartbeat.service';
import { ConnectionRegistry } from '../connection/connection.registry';
import { ConnectionStateStore } from '../connection/connection-state.store';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import { ServerMessageType } from '@delivery-micro/shard';
import { TIMINGS } from '../../../common/common-types/constants';

describe('HeartbeatService', () => {
  let service: HeartbeatService;
  let registry: ConnectionRegistry;
  let stateStore: { refreshHeartbeat: jest.Mock };
  let metrics: { staleConnections: { inc: jest.Mock } };

  const makeSocket = (socketId: string, lastPongAt: number) =>
    ({
      data: { socketId, lastPongAt },
      close: jest.fn(),
    }) as any;

  beforeEach(() => {
    registry = new ConnectionRegistry();
    stateStore = { refreshHeartbeat: jest.fn().mockResolvedValue(undefined) };
    metrics = { staleConnections: { inc: jest.fn() } };
    service = new HeartbeatService(
      registry,
      stateStore as unknown as ConnectionStateStore,
      metrics as unknown as RealtimeMetricsService,
    );
  });

  describe('handlePing', () => {
    it('refreshes the redis TTL and updates lastPongAt', async () => {
      const socket = makeSocket('s1', 0);
      const before = Date.now();

      const result = await service.handlePing(socket);

      expect(socket.data.lastPongAt).toBeGreaterThanOrEqual(before);
      expect(stateStore.refreshHeartbeat).toHaveBeenCalledWith('s1');
      expect(result.type).toBe(ServerMessageType.PONG);
      expect(result.data.timestamp).toBeDefined();
    });
  });

  describe('checkStaleConnections', () => {
    it('closes connections that exceeded the heartbeat timeout', async () => {
      const stale = makeSocket(
        'stale',
        Date.now() - TIMINGS.STALE_CONNECTION_MS - 1000,
      );
      const fresh = makeSocket('fresh', Date.now());
      registry.register(stale);
      registry.register(fresh);

      await service.checkStaleConnections();

      expect(stale.close).toHaveBeenCalledWith(4001, 'Heartbeat timeout');
      expect(fresh.close).not.toHaveBeenCalled();
      expect(metrics.staleConnections.inc).toHaveBeenCalledTimes(1);
    });

    it('keeps all sockets when everyone is healthy', async () => {
      const fresh = makeSocket('fresh', Date.now());
      registry.register(fresh);

      await service.checkStaleConnections();

      expect(fresh.close).not.toHaveBeenCalled();
      expect(metrics.staleConnections.inc).not.toHaveBeenCalled();
    });

    it('handles an empty registry', async () => {
      await expect(service.checkStaleConnections()).resolves.toBeUndefined();
    });
  });
});
