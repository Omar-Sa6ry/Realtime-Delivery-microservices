import { RealtimeGateway } from './websocket.gateway';
import { ConnectionService } from '../connection/connection.service';
import { SubscriptionService } from '../../features/subscription/subscription.service';
import { HeartbeatService } from '../heartbeat/heartbeat.service';
import { LocationService } from '../../features/location/location.service';
import { CommandService } from '../../features/command/command.service';
import { WsAuthGuard, WsGuardChain } from '@delivery-micro/shard';
import { ConfigService } from '@nestjs/config';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import {
  ClientMessageType,
  Role,
  ServerMessageType,
} from '@delivery-micro/shard';

describe('RealtimeGateway', () => {
  let gateway: RealtimeGateway;
  let config: { get: jest.Mock };
  let authGuard: { authenticate: jest.Mock };
  let connectionService: { register: jest.Mock; unregister: jest.Mock };
  let subscriptionService: {
    subscribeToDelivery: jest.Mock;
    unsubscribeFromDelivery: jest.Mock;
    cleanupSocket: jest.Mock;
  };
  let heartbeatService: { handlePing: jest.Mock };
  let locationService: { handle: jest.Mock };
  let commandService: { execute: jest.Mock };
  let guardChain: { run: jest.Mock };
  let metrics: {
    messagesReceived: { inc: jest.Mock };
    connectionCounter: { inc: jest.Mock };
  };

  const context = {
    socketId: 'sock-1',
    nodeId: 'node-1',
    userId: 'u1',
    userRole: Role.DRIVER,
    sessionId: 'sess-1',
    email: 'd@x.com',
    connectedAt: 1,
    lastHeartbeatAt: 1,
  };

  const makeClient = (overrides: any = {}) => ({
    data: {
      socketId: 'sock-1',
      nodeId: 'node-1',
      userId: 'u1',
      userRole: Role.DRIVER,
      sessionId: 'sess-1',
      email: 'd@x.com',
      connectedAt: 1,
      lastPongAt: Date.now(),
      dirty: false,
      ...overrides.data,
    },
    send: jest.fn(),
    close: jest.fn(),
  });

  const requestWith = (origin?: string) =>
    ({ headers: origin ? { origin } : {} }) as any;

  beforeEach(() => {
    config = {
      get: jest.fn((key: string, def?: string) => {
        if (key === 'WS_ALLOWED_ORIGINS') return '*';
        if (key === 'INSTANCE_ID') return 'node-1';
        return def;
      }),
    };
    authGuard = {
      authenticate: jest
        .fn()
        .mockResolvedValue({ userId: 'u1', role: Role.DRIVER }),
    };
    connectionService = {
      register: jest.fn().mockResolvedValue(context),
      unregister: jest.fn().mockResolvedValue(undefined),
    };
    subscriptionService = {
      subscribeToDelivery: jest.fn().mockResolvedValue(undefined),
      unsubscribeFromDelivery: jest.fn().mockResolvedValue(undefined),
      cleanupSocket: jest.fn().mockResolvedValue(undefined),
    };
    heartbeatService = {
      handlePing: jest.fn().mockResolvedValue({
        type: ServerMessageType.PONG,
        data: { timestamp: 'now' },
      }),
    };
    locationService = { handle: jest.fn().mockResolvedValue(undefined) };
    commandService = {
      execute: jest.fn().mockResolvedValue({ accepted: true }),
    };
    guardChain = { run: jest.fn().mockResolvedValue(undefined) };
    metrics = {
      messagesReceived: { inc: jest.fn() },
      connectionCounter: { inc: jest.fn() },
    };

    gateway = new RealtimeGateway(
      config as unknown as ConfigService,
      authGuard as unknown as WsAuthGuard,
      connectionService as unknown as ConnectionService,
      subscriptionService as unknown as SubscriptionService,
      heartbeatService as unknown as HeartbeatService,
      locationService as unknown as LocationService,
      commandService as unknown as CommandService,
      guardChain as unknown as WsGuardChain,
      metrics as unknown as RealtimeMetricsService,
    );
  });

  describe('handleConnection', () => {
    it('registers the socket and sends a CONNECTED message', async () => {
      const client = makeClient();

      await gateway.handleConnection(client, requestWith('https://any.com'));

      expect(authGuard.authenticate).toHaveBeenCalled();
      expect(connectionService.register).toHaveBeenCalledWith(
        client,
        'node-1',
        { userId: 'u1', role: Role.DRIVER },
      );
      expect(client.data.user).toMatchObject({
        userId: 'u1',
        role: Role.DRIVER,
      });
      const sent = JSON.parse(client.send.mock.calls[0][0]);
      expect(sent.type).toBe(ServerMessageType.CONNECTED);
      expect(sent.data.socketId).toBe('sock-1');
      expect(sent.data.nodeId).toBe('node-1');
    });

    it('falls back to a pid-based node id when INSTANCE_ID is missing', async () => {
      config.get.mockImplementation((key: string, def?: string) =>
        key === 'WS_ALLOWED_ORIGINS' ? '*' : def,
      );

      await gateway.handleConnection(makeClient(), requestWith());

      expect(connectionService.register).toHaveBeenCalledWith(
        expect.anything(),
        `realtime-${process.pid}`,
        expect.anything(),
      );
    });

    it('defaults allowed origins to wildcard when WS_ALLOWED_ORIGINS is unset', async () => {
      config.get.mockImplementation((key: string) =>
        key === 'WS_ALLOWED_ORIGINS' ? undefined : 'node-1',
      );
      gateway = new RealtimeGateway(
        config as unknown as ConfigService,
        authGuard as unknown as WsAuthGuard,
        connectionService as unknown as ConnectionService,
        subscriptionService as unknown as SubscriptionService,
        heartbeatService as unknown as HeartbeatService,
        locationService as unknown as LocationService,
        commandService as unknown as CommandService,
        guardChain as unknown as WsGuardChain,
        metrics as unknown as RealtimeMetricsService,
      );

      expect((gateway as any).allowedOrigins).toEqual(['*']);

      const client = makeClient();
      await gateway.handleConnection(client, requestWith('https://any.com'));
      expect(client.close).not.toHaveBeenCalled();
      expect(connectionService.register).toHaveBeenCalled();
    });

    it('rejects disallowed origins with 4403', async () => {
      config.get.mockImplementation((key: string) =>
        key === 'WS_ALLOWED_ORIGINS' ? 'example.com' : 'node-1',
      );
      gateway = new RealtimeGateway(
        config as unknown as ConfigService,
        authGuard as unknown as WsAuthGuard,
        connectionService as unknown as ConnectionService,
        subscriptionService as unknown as SubscriptionService,
        heartbeatService as unknown as HeartbeatService,
        locationService as unknown as LocationService,
        commandService as unknown as CommandService,
        guardChain as unknown as WsGuardChain,
        metrics as unknown as RealtimeMetricsService,
      );

      const client = makeClient();
      await gateway.handleConnection(client, requestWith('https://evil.com'));

      expect(client.close).toHaveBeenCalledWith(4403, 'FORBIDDEN');
      expect(authGuard.authenticate).not.toHaveBeenCalled();
    });

    it('allows exact, protocol and subdomain origin matches', async () => {
      config.get.mockImplementation((key: string) =>
        key === 'WS_ALLOWED_ORIGINS' ? 'example.com, app.test' : 'node-1',
      );
      gateway = new RealtimeGateway(
        config as unknown as ConfigService,
        authGuard as unknown as WsAuthGuard,
        connectionService as unknown as ConnectionService,
        subscriptionService as unknown as SubscriptionService,
        heartbeatService as unknown as HeartbeatService,
        locationService as unknown as LocationService,
        commandService as unknown as CommandService,
        guardChain as unknown as WsGuardChain,
        metrics as unknown as RealtimeMetricsService,
      );

      for (const origin of [
        'example.com',
        'https://example.com',
        'https://api.example.com',
        'https://app.test',
      ]) {
        const client = makeClient();
        await gateway.handleConnection(client, requestWith(origin));
        expect(client.close).not.toHaveBeenCalled();
      }
    });

    it('allows requests without an Origin header (non-browser clients)', async () => {
      config.get.mockImplementation((key: string) =>
        key === 'WS_ALLOWED_ORIGINS' ? 'example.com' : 'node-1',
      );
      gateway = new RealtimeGateway(
        config as unknown as ConfigService,
        authGuard as unknown as WsAuthGuard,
        connectionService as unknown as ConnectionService,
        subscriptionService as unknown as SubscriptionService,
        heartbeatService as unknown as HeartbeatService,
        locationService as unknown as LocationService,
        commandService as unknown as CommandService,
        guardChain as unknown as WsGuardChain,
        metrics as unknown as RealtimeMetricsService,
      );

      const client = makeClient();
      await gateway.handleConnection(client, requestWith());
      expect(client.close).not.toHaveBeenCalled();
      expect(connectionService.register).toHaveBeenCalled();
    });

    it('counts rejected authentications', async () => {
      authGuard.authenticate.mockResolvedValue(null);

      await gateway.handleConnection(makeClient(), requestWith());

      expect(metrics.connectionCounter.inc).toHaveBeenCalledWith({
        result: 'rejected',
      });
      expect(connectionService.register).not.toHaveBeenCalled();
    });

    it('closes the socket with 1011 when registration fails', async () => {
      connectionService.register.mockRejectedValue(new Error('redis down'));
      const errorLog = jest.spyOn((gateway as any).logger, 'error');

      const client = makeClient();
      await gateway.handleConnection(client, requestWith());

      expect(client.close).toHaveBeenCalledWith(1011, 'INTERNAL_ERROR');
      expect(errorLog).toHaveBeenCalled();
    });
  });

  describe('handleDisconnect', () => {
    it('cleans up subscriptions and unregisters the connection', async () => {
      const client = makeClient();
      await gateway.handleDisconnect(client);

      expect(subscriptionService.cleanupSocket).toHaveBeenCalledWith('sock-1');
      expect(connectionService.unregister).toHaveBeenCalledWith(client);
    });

    it('ignores sockets without registration data', async () => {
      const client = { data: undefined, close: jest.fn() } as any;
      await expect(gateway.handleDisconnect(client)).resolves.toBeUndefined();
      expect(connectionService.unregister).not.toHaveBeenCalled();
    });
  });

  describe('handlePing', () => {
    it('runs the guard chain and returns a PONG with requestId', async () => {
      const client = makeClient();
      const message = { requestId: 'req-1', type: ClientMessageType.PING };

      const result = await gateway.handlePing(client as any, message as any);

      expect(guardChain.run).toHaveBeenCalledWith({ message, socket: client });
      expect(heartbeatService.handlePing).toHaveBeenCalledWith(client);
      expect(result).toMatchObject({
        type: ServerMessageType.PONG,
        requestId: 'req-1',
      });
    });

    it('works without a message body', async () => {
      const result = await gateway.handlePing(makeClient() as any);
      expect(result.type).toBe(ServerMessageType.PONG);
    });
  });

  describe('handleSubscribe', () => {
    it('authorizes and subscribes the socket', async () => {
      const client = makeClient();
      const message = {
        requestId: 'req-1',
        type: ClientMessageType.SUBSCRIBE_DELIVERY,
        data: { deliveryId: 'd1' },
      };

      const result = await gateway.handleSubscribe(
        client as any,
        message as any,
      );

      expect(metrics.messagesReceived.inc).toHaveBeenCalledWith({
        type: ClientMessageType.SUBSCRIBE_DELIVERY,
      });
      expect(subscriptionService.subscribeToDelivery).toHaveBeenCalledWith(
        'sock-1',
        expect.objectContaining({ userId: 'u1', role: Role.DRIVER }),
        'd1',
      );
      expect(result).toMatchObject({
        type: ServerMessageType.SUBSCRIBED,
        requestId: 'req-1',
        data: { deliveryId: 'd1', requestId: 'req-1' },
      });
    });
  });

  describe('handleUnsubscribe', () => {
    it('unsubscribes the socket from the delivery', async () => {
      const client = makeClient();
      const message = {
        requestId: 'req-2',
        type: ClientMessageType.UNSUBSCRIBE_DELIVERY,
        data: { deliveryId: 'd1' },
      };

      const result = await gateway.handleUnsubscribe(
        client as any,
        message as any,
      );

      expect(subscriptionService.unsubscribeFromDelivery).toHaveBeenCalledWith(
        'sock-1',
        'd1',
      );
      expect(result).toMatchObject({
        type: ServerMessageType.UNSUBSCRIBED,
        data: { deliveryId: 'd1' },
      });
    });
  });

  describe('handleLocationUpdate', () => {
    it('delegates to the location service', async () => {
      const client = makeClient();
      const payload = { deliveryId: 'd1', lat: 1, lng: 2 };
      const message = {
        requestId: 'req-3',
        type: ClientMessageType.LOCATION_UPDATE,
        data: payload,
      };

      const result = await gateway.handleLocationUpdate(
        client as any,
        message as any,
      );

      expect(locationService.handle).toHaveBeenCalledWith(
        client,
        payload,
        'req-3',
      );
      expect(result).toBeUndefined();
    });
  });

  describe('command handlers', () => {
    const commandMessage = (type: ClientMessageType) => ({
      requestId: 'req-4',
      type,
      data: { deliveryId: 'd1', commandId: 'cmd-1' },
    });

    it.each([
      [
        'ACCEPT_ASSIGNMENT',
        (c: RealtimeGateway, m: any) =>
          c.handleAcceptAssignment(makeClient() as any, m),
      ],
      [
        'REJECT_ASSIGNMENT',
        (c: RealtimeGateway, m: any) =>
          c.handleRejectAssignment(makeClient() as any, m),
      ],
      [
        'COMPLETE_DELIVERY',
        (c: RealtimeGateway, m: any) =>
          c.handleCompleteDelivery(makeClient() as any, m),
      ],
    ])('executes %s and returns an ACK', async (kind, invoke) => {
      const message = commandMessage(kind as ClientMessageType);

      const result = await invoke(gateway, message as any);

      expect(metrics.messagesReceived.inc).toHaveBeenCalledWith({ type: kind });
      expect(commandService.execute).toHaveBeenCalledWith(
        expect.objectContaining({ userId: 'u1', role: Role.DRIVER }),
        kind,
        message.data,
      );
      expect(result).toMatchObject({
        type: ServerMessageType.ACK,
        requestId: 'req-4',
        data: { accepted: true },
      });
    });

    it('surfaces duplicate command results in the ACK payload', async () => {
      commandService.execute.mockResolvedValue({
        accepted: false,
        duplicate: true,
      });

      const result = await gateway.handleAcceptAssignment(
        makeClient() as any,
        commandMessage(ClientMessageType.ACCEPT_ASSIGNMENT) as any,
      );

      expect(result.data).toEqual({ accepted: false, duplicate: true });
    });
  });

  describe('handleAck', () => {
    it('records the metric and returns undefined', async () => {
      const result = await gateway.handleAck({
        type: ClientMessageType.ACK,
        data: { messageId: 'm1' },
      } as any);

      expect(metrics.messagesReceived.inc).toHaveBeenCalledWith({
        type: ClientMessageType.ACK,
      });
      expect(result).toBeUndefined();
    });

    it('tolerates a missing body', async () => {
      await expect(gateway.handleAck()).resolves.toBeUndefined();
    });
  });
});
