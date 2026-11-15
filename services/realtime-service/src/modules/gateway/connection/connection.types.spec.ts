import { toContext, toSocketData } from './connection.types';
import { IJwtPayload, Role } from '@delivery-micro/shard';

describe('connection.types', () => {
  const payload: IJwtPayload = {
    userId: 'u1',
    role: Role.DRIVER,
    sessionId: 'sess1',
    email: 'd@x.com',
  };

  describe('toContext', () => {
    it('builds a connection context from a JWT payload', () => {
      const now = Date.now();
      const ctx = toContext('sock1', 'node1', payload);
      expect(ctx).toMatchObject({
        socketId: 'sock1',
        nodeId: 'node1',
        userId: 'u1',
        userRole: Role.DRIVER,
        sessionId: 'sess1',
        email: 'd@x.com',
      });
      expect(ctx.connectedAt).toBeGreaterThanOrEqual(now);
      expect(ctx.lastHeartbeatAt).toBeGreaterThanOrEqual(now);
    });

    it('falls back to sub when userId is missing', () => {
      const ctx = toContext('sock1', 'node1', {
        sub: 'sub-1',
        role: Role.USER,
      });
      expect(ctx.userId).toBe('sub-1');
    });

    it('falls back to id when userId and sub are missing', () => {
      const ctx = toContext('sock1', 'node1', {
        id: 'id-1',
        role: Role.ADMIN,
      } as IJwtPayload);
      expect(ctx.userId).toBe('id-1');
    });
  });

  describe('toSocketData', () => {
    it('converts a context into socket data', () => {
      const ctx = toContext('sock1', 'node1', payload);
      const data = toSocketData(ctx);
      expect(data).toMatchObject({
        socketId: 'sock1',
        nodeId: 'node1',
        userId: 'u1',
        userRole: Role.DRIVER,
        sessionId: 'sess1',
        email: 'd@x.com',
        dirty: false,
      });
      expect(typeof data.lastPongAt).toBe('number');
    });
  });
});
