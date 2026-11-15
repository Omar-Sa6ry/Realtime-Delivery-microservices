import {
  PresenceStatus,
  BooleanResponse,
  ConnectionStatusData,
  ConnectionsByRole,
  ConnectionsCountData,
  ConnectionStatusResponse,
  ConnectionsCountResponse,
} from './connection.types';

describe('GraphQL connection types', () => {
  it('exposes the PresenceStatus enum values', () => {
    expect(PresenceStatus.ONLINE).toBe('ONLINE');
    expect(PresenceStatus.IDLE).toBe('IDLE');
    expect(PresenceStatus.OFFLINE).toBe('OFFLINE');
  });

  it('BooleanResponse carries success defaults', () => {
    const r = new BooleanResponse();
    expect(r.message).toBe('Operation executed successfully');
    expect(r.success).toBe(true);
    expect(r.statusCode).toBe(200);
    expect(typeof r.timeStamp).toBe('string');
    expect(r.data).toBeUndefined();
  });

  it('ConnectionStatusResponse carries success defaults', () => {
    const r = new ConnectionStatusResponse();
    expect(r.message).toBe('Operation executed successfully');
    expect(r.success).toBe(true);
    expect(r.statusCode).toBe(200);
    expect(r.data).toBeUndefined();
  });

  it('ConnectionsCountResponse carries success defaults', () => {
    const r = new ConnectionsCountResponse();
    expect(r.message).toBe('Operation executed successfully');
    expect(r.success).toBe(true);
    expect(r.statusCode).toBe(200);
    expect(r.data).toBeUndefined();
  });

  it('data shapes can be assembled', () => {
    const status: ConnectionStatusData = {
      isConnected: true,
      connectionCount: 2,
      lastSeen: '2026-01-01T00:00:00.000Z',
    };
    const byRole: ConnectionsByRole = { customers: 1, drivers: 1, admins: 0 };
    const counts: ConnectionsCountData = { total: 2, byRole };

    expect(status.isConnected).toBe(true);
    expect(counts.total).toBe(2);
    expect(counts.byRole.drivers).toBe(1);
  });
});
