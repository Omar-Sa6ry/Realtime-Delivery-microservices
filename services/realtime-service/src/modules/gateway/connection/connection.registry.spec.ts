import { ConnectionRegistry } from './connection.registry';
import { IJwtPayload, Role } from '@delivery-micro/shard';
import type { WebSocket } from 'ws';

describe('ConnectionRegistry', () => {
  let registry: ConnectionRegistry;

  const fakeSocket = () =>
    ({ data: undefined, send: jest.fn(), close: jest.fn() }) as any;

  beforeEach(() => {
    registry = new ConnectionRegistry();
  });

  it('registers and retrieves sockets by id', () => {
    const socket = fakeSocket();
    socket.data = { socketId: 's1' };
    registry.register(socket);

    expect(registry.size).toBe(1);
    expect(registry.has('s1')).toBe(true);
    expect(registry.get('s1')).toBe(socket);
  });

  it('values returns all sockets', () => {
    const a = fakeSocket();
    a.data = { socketId: 'a' };
    const b = fakeSocket();
    b.data = { socketId: 'b' };
    registry.register(a);
    registry.register(b);

    expect(registry.values()).toEqual([a, b]);
  });

  it('removes sockets', () => {
    const socket = fakeSocket();
    socket.data = { socketId: 's1' };
    registry.register(socket);
    registry.remove('s1');

    expect(registry.size).toBe(0);
    expect(registry.get('s1')).toBeUndefined();
    expect(registry.has('s1')).toBe(false);
  });

  it('buildContext creates a context with a generated socket id', () => {
    const payload: IJwtPayload = { userId: 'u1', role: Role.USER };
    const ctx = registry.buildContext('node1', payload);

    expect(ctx.socketId).toBeTruthy();
    expect(ctx.nodeId).toBe('node1');
    expect(ctx.userId).toBe('u1');
  });

  it('generates unique socket ids', () => {
    const payload: IJwtPayload = { userId: 'u1', role: Role.USER };
    const ids = new Set([
      registry.buildContext('n', payload).socketId,
      registry.buildContext('n', payload).socketId,
      registry.buildContext('n', payload).socketId,
    ]);
    expect(ids.size).toBe(3);
  });

  it('attach sets socket data and registers the socket', () => {
    const socket = fakeSocket() as WebSocket;
    const ctx = registry.buildContext('node1', {
      userId: 'u1',
      role: Role.DRIVER,
    });

    registry.attach(socket, ctx);

    const attached = socket as WebSocket & { data: any };
    expect(attached.data.socketId).toBe(ctx.socketId);
    expect(attached.data.userId).toBe('u1');
    expect(registry.get(ctx.socketId)).toBeDefined();
  });
});
