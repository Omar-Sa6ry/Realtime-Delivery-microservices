import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { WebSocket } from 'ws';
import {
  ClientMessageType,
  Role,
  ServerMessageType,
  WsAuthGuard,
  WsErrorCode,
  WsGuardChain,
  RealtimeWsAdapter,
} from '@delivery-micro/shard';

import { RealtimeGateway } from '../src/modules/gateway/websocket/websocket.gateway';
import { ConnectionService } from '../src/modules/gateway/connection/connection.service';
import { SubscriptionService } from '../src/modules/features/subscription/subscription.service';
import { HeartbeatService } from '../src/modules/gateway/heartbeat/heartbeat.service';
import { LocationService } from '../src/modules/features/location/location.service';
import { CommandService } from '../src/modules/features/command/command.service';
import { RealtimeMetricsService } from '../src/common/metrics/realtime-metrics.service';

jest.setTimeout(30000);

interface TestClient {
  ws: WebSocket;
  open: () => Promise<void>;
  next: (timeout?: number) => Promise<any>;
  close: () => Promise<void>;
}

const createTestClient = (url: string, options?: WebSocket.ClientOptions) => {
  const ws = new WebSocket(url, options);
  const queue: any[] = [];
  const waiters: Array<(msg: any) => void> = [];

  ws.on('message', (data) => {
    let msg: any;
    try {
      msg = JSON.parse(data.toString());
    } catch {
      msg = data.toString();
    }
    const waiter = waiters.shift();
    if (waiter) waiter(msg);
    else queue.push(msg);
  });

  return {
    ws,
    open: () =>
      new Promise<void>((resolve, reject) => {
        ws.once('open', () => resolve());
        ws.once('error', reject);
      }),
    next: (timeout = 2000) =>
      new Promise<any>((resolve, reject) => {
        if (queue.length > 0) {
          resolve(queue.shift());
          return;
        }
        const timer = setTimeout(
          () => reject(new Error('timed out waiting for ws message')),
          timeout,
        );
        waiters.push((msg) => {
          clearTimeout(timer);
          resolve(msg);
        });
      }),
    close: () =>
      new Promise<void>((resolve) => {
        if (ws.readyState === WebSocket.CLOSED) {
          resolve();
          return;
        }
        ws.once('close', () => resolve());
        ws.close();
      }),
  } satisfies TestClient;
};

const waitFor = async (
  predicate: () => boolean,
  timeoutMs = 2000,
): Promise<void> => {
  const deadline = Date.now() + timeoutMs;
  while (!predicate()) {
    if (Date.now() > deadline) {
      throw new Error('timed out waiting for condition');
    }
    await new Promise((resolve) => setTimeout(resolve, 20));
  }
};

const buildGatewayModule = async (
  allowedOrigins?: string,
): Promise<{
  app: INestApplication;
  port: number;
  authGuard: { authenticate: jest.Mock };
  connectionService: { register: jest.Mock; unregister: jest.Mock };
  subscriptionService: {
    subscribeToDelivery: jest.Mock;
    unsubscribeFromDelivery: jest.Mock;
    cleanupSocket: jest.Mock;
  };
  heartbeatService: { handlePing: jest.Mock };
  locationService: { handle: jest.Mock };
  commandService: { execute: jest.Mock };
  guardChain: { run: jest.Mock };
  metrics: {
    messagesReceived: { inc: jest.Mock };
    connectionCounter: { inc: jest.Mock };
  };
}> => {
  const authGuard = {
    authenticate: jest.fn(async () => ({
      userId: 'user-1',
      role: Role.DRIVER,
      sessionId: 'session-1',
      email: 'driver@x.com',
    })),
  };
  const connectionService = {
    register: jest.fn(async (socket: any) => {
      socket.data = {
        ...socket.data,
        socketId: 'sock-e2e',
        userId: 'user-1',
        userRole: Role.DRIVER,
        sessionId: 'session-1',
        email: 'driver@x.com',
        connectedAt: Date.now(),
        lastPongAt: Date.now(),
        dirty: false,
      };
      return {
        socketId: 'sock-e2e',
        nodeId: 'node-e2e',
        userId: 'user-1',
        userRole: Role.DRIVER,
        sessionId: 'session-1',
        email: 'driver@x.com',
        connectedAt: Date.now(),
        lastHeartbeatAt: Date.now(),
      };
    }),
    unregister: jest.fn(async () => undefined),
  };
  const subscriptionService = {
    subscribeToDelivery: jest.fn(async () => undefined),
    unsubscribeFromDelivery: jest.fn(async () => undefined),
    cleanupSocket: jest.fn(async () => undefined),
  };
  const heartbeatService = {
    handlePing: jest.fn(async () => ({ data: { timestamp: 'now' } })),
  };
  const locationService = { handle: jest.fn(async () => undefined) };
  const commandService = {
    execute: jest.fn(async () => ({ accepted: true })),
  };
  const guardChain = { run: jest.fn(async () => undefined) };
  const metrics = {
    messagesReceived: { inc: jest.fn() },
    connectionCounter: { inc: jest.fn() },
  };
  const config = {
    get: jest.fn((key: string, def?: unknown) => {
      if (key === 'WS_ALLOWED_ORIGINS') return allowedOrigins;
      if (key === 'INSTANCE_ID') return 'node-e2e';
      return def;
    }),
  };

  const moduleFixture: TestingModule = await Test.createTestingModule({
    providers: [
      RealtimeGateway,
      { provide: ConfigService, useValue: config },
      { provide: WsAuthGuard, useValue: authGuard },
      { provide: ConnectionService, useValue: connectionService },
      { provide: SubscriptionService, useValue: subscriptionService },
      { provide: HeartbeatService, useValue: heartbeatService },
      { provide: LocationService, useValue: locationService },
      { provide: CommandService, useValue: commandService },
      { provide: WsGuardChain, useValue: guardChain },
      { provide: RealtimeMetricsService, useValue: metrics },
    ],
  }).compile();

  const app = moduleFixture.createNestApplication();
  app.useWebSocketAdapter(new RealtimeWsAdapter(app));
  await app.listen(0, '127.0.0.1');
  const address = app.getHttpServer().address();
  const port = typeof address === 'object' && address ? address.port : 0;

  return {
    app,
    port,
    authGuard,
    connectionService,
    subscriptionService,
    heartbeatService,
    locationService,
    commandService,
    guardChain,
    metrics,
  };
};

describe('Realtime WebSocket (e2e)', () => {
  let app: INestApplication;
  let port: number;
  let url: string;
  let authGuard: { authenticate: jest.Mock };
  let connectionService: { register: jest.Mock; unregister: jest.Mock };
  let subscriptionService: {
    subscribeToDelivery: jest.Mock;
    unsubscribeFromDelivery: jest.Mock;
    cleanupSocket: jest.Mock;
  };
  let heartbeatService: { handlePing: jest.Mock };
  let locationService: { handle: jest.Mock };
  let guardChain: { run: jest.Mock };
  let metrics: {
    messagesReceived: { inc: jest.Mock };
    connectionCounter: { inc: jest.Mock };
  };

  beforeAll(async () => {
    const ctx = await buildGatewayModule();
    app = ctx.app;
    port = ctx.port;
    url = `ws://127.0.0.1:${port}/ws`;
    authGuard = ctx.authGuard;
    connectionService = ctx.connectionService;
    subscriptionService = ctx.subscriptionService;
    heartbeatService = ctx.heartbeatService;
    locationService = ctx.locationService;
    guardChain = ctx.guardChain;
    metrics = ctx.metrics;
  });

  afterAll(async () => {
    await app.close();
  });

  beforeEach(() => {
    jest.clearAllMocks();
  });

  const connect = async (): Promise<TestClient> => {
    const client = createTestClient(url);
    await client.open();
    return client;
  };

  it('completes the handshake and announces the registered socket', async () => {
    const client = await connect();
    const connected = await client.next();

    expect(connected.type).toBe(ServerMessageType.CONNECTED);
    expect(connected.data.socketId).toBe('sock-e2e');
    expect(connected.data.nodeId).toBe('node-e2e');
    expect(connectionService.register).toHaveBeenCalledWith(
      expect.anything(),
      'node-e2e',
      expect.objectContaining({ userId: 'user-1', role: Role.DRIVER }),
    );

    await client.close();
  });

  it('answers PING with PONG through the guard chain', async () => {
    const client = await connect();
    await client.next(); // CONNECTED

    client.ws.send(
      JSON.stringify({ type: ClientMessageType.PING, requestId: 'req-1' }),
    );
    const pong = await client.next();

    expect(pong).toMatchObject({
      type: ServerMessageType.PONG,
      requestId: 'req-1',
      data: { timestamp: 'now' },
    });
    expect(guardChain.run).toHaveBeenCalledWith(
      expect.objectContaining({
        message: expect.objectContaining({ type: ClientMessageType.PING }),
      }),
    );
    expect(heartbeatService.handlePing).toHaveBeenCalled();

    await client.close();
  });

  it('subscribes the socket to a delivery', async () => {
    const client = await connect();
    await client.next(); // CONNECTED

    client.ws.send(
      JSON.stringify({
        type: ClientMessageType.SUBSCRIBE_DELIVERY,
        requestId: 'req-2',
        data: { deliveryId: 'delivery-e2e' },
      }),
    );
    const response = await client.next();

    expect(response).toMatchObject({
      type: ServerMessageType.SUBSCRIBED,
      requestId: 'req-2',
      data: { deliveryId: 'delivery-e2e', requestId: 'req-2' },
    });
    expect(subscriptionService.subscribeToDelivery).toHaveBeenCalledWith(
      'sock-e2e',
      expect.objectContaining({ userId: 'user-1' }),
      'delivery-e2e',
    );

    await client.close();
  });

  it('rejects unknown message types with an ERROR envelope', async () => {
    const client = await connect();
    await client.next(); // CONNECTED

    client.ws.send(
      JSON.stringify({ type: 'TOTALLY_UNKNOWN', requestId: 'req-3' }),
    );
    const error = await client.next();

    expect(error.type).toBe('ERROR');
    expect(error.requestId).toBe('req-3');
    expect(error.data).toMatchObject({
      code: WsErrorCode.INVALID_MESSAGE,
      message: expect.stringContaining('Unknown message type'),
      retryable: false,
    });

    await client.close();
  });

  it('does not reply to fire-and-forget location updates', async () => {
    const client = await connect();
    await client.next(); // CONNECTED

    client.ws.send(
      JSON.stringify({
        type: ClientMessageType.LOCATION_UPDATE,
        requestId: 'req-4',
        data: {
          deliveryId: 'delivery-e2e',
          lat: 30.1,
          lng: 31.2,
          timestamp: new Date().toISOString(),
        },
      }),
    );
    client.ws.send(
      JSON.stringify({ type: ClientMessageType.PING, requestId: 'req-5' }),
    );

    const nextMessage = await client.next();
    expect(nextMessage.type).toBe(ServerMessageType.PONG);
    expect(locationService.handle).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ deliveryId: 'delivery-e2e' }),
      'req-4',
    );

    await client.close();
  });

  it('cleans up subscriptions and unregisters on disconnect', async () => {
    const client = await connect();
    await client.next(); // CONNECTED

    await client.close();

    await waitFor(
      () => subscriptionService.cleanupSocket.mock.calls.length > 0,
    );
    expect(subscriptionService.cleanupSocket).toHaveBeenCalledWith('sock-e2e');
    expect(connectionService.unregister).toHaveBeenCalled();
  });

  it('refuses the handshake when authentication fails', async () => {
    authGuard.authenticate.mockResolvedValueOnce(null);

    const client = await connect();
    const message = await client.next(300).catch(() => null as unknown as null);

    expect(message).toBeNull();
    expect(connectionService.register).not.toHaveBeenCalled();
    expect(metrics.connectionCounter.inc).toHaveBeenCalledWith({
      result: 'rejected',
    });

    await client.close();
    authGuard.authenticate.mockResolvedValue(async () => ({
      userId: 'user-1',
      role: Role.DRIVER,
      sessionId: 'session-1',
      email: 'driver@x.com',
    }));
  });
});

describe('Realtime WebSocket origin enforcement (e2e)', () => {
  let app: INestApplication;
  let url: string;

  beforeAll(async () => {
    const ctx = await buildGatewayModule('example.com');
    app = ctx.app;
    url = `ws://127.0.0.1:${ctx.port}/ws`;
  });

  afterAll(async () => {
    await app.close();
  });

  it('closes sockets that present a disallowed Origin header', async () => {
    const client = createTestClient(url, {
      headers: { origin: 'https://evil.example' },
    });

    const closeCode = await new Promise<number>((resolve) => {
      client.ws.once('close', (code) => resolve(code));
      client.ws.once('error', () => resolve(-1));
      // open() rejects on error; race with close instead
      client.ws.once('open', () => {
        /* opened, waiting for server close */
      });
    });

    expect(closeCode).toBe(4403);
  });

  it('accepts sockets with an allowed Origin header', async () => {
    const client = createTestClient(url, {
      headers: { origin: 'https://api.example.com' },
    });
    await client.open();
    const connected = await client.next();
    expect(connected.type).toBe(ServerMessageType.CONNECTED);
    await client.close();
  });
});
