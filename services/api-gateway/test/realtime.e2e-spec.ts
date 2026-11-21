/**
 * Realtime service GraphQL operations — mirrors graphql-docs/realtime.graphql.
 *
 * The WebSocket layer documented in that file is wired in `main.ts` (a raw WS
 * proxy in front of the realtime service) and is therefore out of scope for
 * the AppModule-driven gateway; these tests cover the composed GraphQL surface.
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

describe('Realtime service (graphql-docs/realtime.graphql)', () => {
  let h: GatewayHarness;
  let token: string;

  beforeAll(async () => {
    h = await createGatewayHarness();
    token = h.signToken({
      sub: 'user-001',
      role: 'USER',
      sessionId: 'sess-rt',
    });
  });

  afterAll(async () => {
    await h.close();
  });

  describe('pingForRealtime (Public)', () => {
    it('answers the documented query', async () => {
      const res = await h
        .gql(`{ pingForRealtime { statusCode message success data } }`)
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.pingForRealtime).toMatchObject({
        statusCode: 200,
        message: 'ok',
        success: true,
        data: true,
      });
      expect(h.subgraphs.last('realtime')?.fields).toContain('pingForRealtime');
    });
  });

  describe('getConnectionStatus (User / Driver / Admin)', () => {
    it('reports connection state for a user', async () => {
      const res = await h
        .gql(
          `{ getConnectionStatus(userId: "user-001") {
            statusCode message success
            data { isConnected lastSeen connectionCount }
          } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.getConnectionStatus.data).toEqual({
        isConnected: true,
        lastSeen: '2026-01-01T00:00:00.000Z',
        connectionCount: 2,
      });
      expect(h.subgraphs.last('realtime')?.headers['x-user-id']).toBe(
        'user-001',
      );
    });
  });

  describe('getActiveConnections (Admin)', () => {
    it('reports totals broken down by role', async () => {
      const res = await h
        .gql(
          `{ getActiveConnections {
            statusCode message success
            data { total byRole { customers drivers admins } }
          } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.getActiveConnections.data).toEqual({
        total: 3,
        byRole: { customers: 1, drivers: 1, admins: 1 },
      });
    });
  });

  it('routes all realtime queries to the realtime subgraph', async () => {
    h.subgraphs.clear();

    await h.gql(`{ pingForRealtime { success } }`).expect(200);
    await h.gql(`{ getConnectionStatus(userId: "u") { success } }`).expect(200);
    await h.gql(`{ getActiveConnections { success } }`).expect(200);

    const fields = new Set(
      h.subgraphs.forSubgraph('realtime').flatMap((r) => r.fields),
    );
    expect(fields).toContain('pingForRealtime');
    expect(fields).toContain('getConnectionStatus');
    expect(fields).toContain('getActiveConnections');
  });
});
