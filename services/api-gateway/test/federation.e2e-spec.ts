/**
 * Federation behaviour of the gateway: cross-subgraph entity resolution
 * (`_entities` round-trips) and header propagation from the client through
 * `willSendRequest` to every downstream subgraph.
 *
 * A single harness is shared by every test in this file: `AppModule` reads the
 * subgraph URLs from process.env at decoration time and is cached by `require`,
 * so a second harness would still point at the first one's (now-stopped)
 * subgraph ports.
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

describe('Federation: entity resolution and header propagation', () => {
  let h: GatewayHarness;

  beforeAll(async () => {
    h = await createGatewayHarness();
  });

  afterAll(async () => {
    await h.close();
  });

  afterEach(() => {
    h.subgraphs.clearErrors();
  });

  describe('entity resolution across subgraphs', () => {
    it('resolves Payment → Delivery and Payment → User entity references', async () => {
      h.subgraphs.clear();

      const res = await h
        .gql(
          `{ payment(id: "63359164469") {
            success
            data {
              id
              deliveryId
              delivery { id status amount currency }
              userId
              user { id email firstName role }
            }
          } }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();

      const data = res.body.data.payment.data;
      // stubs came from the payment subgraph…
      expect(data.deliveryId).toBe('delivery-1');
      expect(data.userId).toBe('customer-1');
      // …and the entity fields were filled in by the owning subgraphs
      expect(data.delivery).toMatchObject({
        id: 'delivery-1',
        status: 'PENDING_PAYMENT',
        amount: '25.50',
        currency: 'USD',
      });
      expect(data.user).toMatchObject({
        id: 'customer-1',
        email: 'omar@example.com',
        firstName: 'Omar',
        role: 'USER',
      });

      // each owning subgraph actually served an `_entities` request
      expect(
        h.subgraphs
          .forSubgraph('delivery')
          .some((r) => r.fields.includes('_entities:Delivery')),
      ).toBe(true);
      expect(
        h.subgraphs
          .forSubgraph('user')
          .some((r) => r.fields.includes('_entities:User')),
      ).toBe(true);
    });

    it('resolves Delivery → Driver and Delivery → User entity references', async () => {
      h.subgraphs.clear();

      const res = await h
        .gql(
          `{ delivery(id: "d-1") {
            success
            data {
              id
              driver { id plateNumber vehicleType rating }
              customer { id email }
            }
          } }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.delivery.data;
      expect(data.driver).toMatchObject({
        id: 'driver-1',
        plateNumber: 'ABC-123',
        vehicleType: 'MOTORCYCLE',
        rating: 4.8,
      });
      expect(data.customer).toMatchObject({
        id: 'customer-1',
        email: 'omar@example.com',
      });

      expect(
        h.subgraphs
          .forSubgraph('driver')
          .some((r) => r.fields.includes('_entities:Driver')),
      ).toBe(true);
      expect(
        h.subgraphs
          .forSubgraph('user')
          .some((r) => r.fields.includes('_entities:User')),
      ).toBe(true);
    });

    it('resolves a three-hop chain Analytics → Driver → User', async () => {
      h.subgraphs.clear();

      const res = await h
        .gql(
          `{ topDrivers(range: { from: "2026-09-12T00:00:00Z", to: "2026-10-19T23:59:59Z", granularity: DAY }) {
            success
            data {
              items {
                driverId
                driver { id plateNumber user { id email } }
              }
            }
          } }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const item = res.body.data.topDrivers.data.items[0];
      expect(item.driver).toMatchObject({
        id: 'driver-1',
        plateNumber: 'ABC-123',
        // Driver.user is itself a reference resolved by the user subgraph
        user: { id: 'driver-user-1', email: 'omar@example.com' },
      });

      expect(
        h.subgraphs
          .forSubgraph('analytics')
          .some((r) => r.fields.includes('topDrivers')),
      ).toBe(true);
      expect(
        h.subgraphs
          .forSubgraph('driver')
          .some((r) => r.fields.includes('_entities:Driver')),
      ).toBe(true);
      expect(
        h.subgraphs
          .forSubgraph('user')
          .some((r) => r.fields.includes('_entities:User')),
      ).toBe(true);
    });

    it('resolves User → Address lists through the user subgraph only', async () => {
      h.subgraphs.clear();

      const res = await h
        .gql(
          `{ myProfile { success data { id addresses { title city isDefault } } } }`,
          undefined,
          { token: h.signToken({ sub: 'me-1', role: 'USER' }) },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.myProfile.data.addresses[0]).toMatchObject({
        title: 'Home',
        city: 'Cairo',
        isDefault: true,
      });
      // no entity round-trips were needed for a single-subgraph query
      expect(
        h.subgraphs
          .forSubgraph('driver')
          .some((r) => r.fields.includes('_entities:Driver')),
      ).toBe(false);
    });

    it('serves a single query spanning several subgraphs in one round trip', async () => {
      h.subgraphs.clear();

      const res = await h
        .gql(
          `{ pingForUser { success } pingForNotification { success } pingForRealtime { success } quota { success } searchHealth { success } }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data).toMatchObject({
        pingForUser: { success: true },
        pingForNotification: { success: true },
        pingForRealtime: { success: true },
        quota: { success: true },
        searchHealth: { success: true },
      });

      for (const subgraph of [
        'user',
        'notification',
        'realtime',
        'media',
        'search',
      ]) {
        expect(h.subgraphs.forSubgraph(subgraph).length).toBeGreaterThan(0);
      }
    });

    it('propagates headers on entity (_entities) requests too', async () => {
      h.subgraphs.clear();
      const token = h.signToken({
        sub: 'customer-1',
        role: 'USER',
        sessionId: 'sess-ent',
      });

      await h
        .gql(
          `{ payment(id: "p-1") { success data { user { id email } } } }`,
          undefined,
          {
            token,
            headers: { 'x-correlation-id': 'corr-entity-1', 'x-lang': 'ar-EG' },
          },
        )
        .expect(200);

      const entityRequest = h.subgraphs
        .forSubgraph('user')
        .find((r) => r.fields.includes('_entities:User'));

      expect(entityRequest).toBeDefined();
      expect(entityRequest?.headers['x-user-id']).toBe('customer-1');
      expect(entityRequest?.headers['x-user-role']).toBe('USER');
      expect(entityRequest?.headers['x-user-session']).toBe('sess-ent');
      expect(entityRequest?.headers['x-correlation-id']).toBe('corr-entity-1');
      expect(entityRequest?.headers['x-lang']).toBe('ar-EG');
    });
  });

  describe('header propagation (willSendRequest)', () => {
    it('forwards identity, auth, correlation and language headers', async () => {
      h.subgraphs.clear();
      const token = h.signToken({
        sub: 'user-77',
        role: 'DRIVER',
        sessionId: 'sess-77',
      });

      await h
        .gql(`{ pingForUser { success } }`, undefined, {
          token,
          headers: {
            'x-correlation-id': 'corr-abc',
            'x-lang': 'ar-EG',
            'accept-language': 'ar-EG,ar;q=0.9',
          },
        })
        .expect(200);

      const req = h.subgraphs.last('user');
      expect(req?.headers).toMatchObject({
        'x-user-id': 'user-77',
        'x-user-role': 'DRIVER',
        'x-user-session': 'sess-77',
        'x-correlation-id': 'corr-abc',
        // x-lang takes precedence and is copied into accept-language too
        'x-lang': 'ar-EG',
        'accept-language': 'ar-EG',
        authorization: `Bearer ${token}`,
      });
    });

    it('falls back to accept-language when x-lang is absent', async () => {
      h.subgraphs.clear();

      await h
        .gql(`{ pingForUser { success } }`, undefined, {
          headers: { 'accept-language': 'fr-FR' },
        })
        .expect(200);

      const req = h.subgraphs.last('user');
      expect(req?.headers['x-lang']).toBe('fr-FR');
      expect(req?.headers['accept-language']).toBe('fr-FR');
    });

    it('omits identity headers for anonymous callers but keeps correlation', async () => {
      h.subgraphs.clear();

      await h
        .gql(`{ pingForUser { success } }`, undefined, {
          headers: { 'x-correlation-id': 'corr-anon' },
        })
        .expect(200);

      const req = h.subgraphs.last('user');
      expect(req?.headers['x-user-id']).toBeUndefined();
      expect(req?.headers['x-user-role']).toBeUndefined();
      expect(req?.headers['x-user-session']).toBeUndefined();
      expect(req?.headers['authorization']).toBeUndefined();
      expect(req?.headers['x-correlation-id']).toBe('corr-anon');
    });

    it('does not forward x-user-session when the token has no session', async () => {
      h.subgraphs.clear();
      const token = h.signToken({ sub: 'user-88', role: 'USER' });

      await h
        .gql(`{ pingForUser { success } }`, undefined, { token })
        .expect(200);

      const req = h.subgraphs.last('user');
      expect(req?.headers['x-user-id']).toBe('user-88');
      expect(req?.headers['x-user-session']).toBeUndefined();
    });

    it('uses `userId` / `id` claims when `sub` is absent', async () => {
      h.subgraphs.clear();
      const token = h.signToken({ userId: 'legacy-user', role: 'USER' });

      await h
        .gql(`{ pingForUser { success } }`, undefined, { token })
        .expect(200);

      expect(h.subgraphs.last('user')?.headers['x-user-id']).toBe(
        'legacy-user',
      );
    });

    it('forwards the same headers to every subgraph in one operation', async () => {
      h.subgraphs.clear();
      const token = h.signToken({
        sub: 'user-99',
        role: 'ADMIN',
        sessionId: 'sess-99',
      });

      await h
        .gql(
          `{ pingForUser { success } pingForNotification { success } quota { success } }`,
          undefined,
          {
            token,
            headers: { 'x-correlation-id': 'corr-multi' },
          },
        )
        .expect(200);

      for (const subgraph of ['user', 'notification', 'media']) {
        const req = h.subgraphs.last(subgraph);
        expect(req?.headers['x-user-id']).toBe('user-99');
        expect(req?.headers['x-user-role']).toBe('ADMIN');
        expect(req?.headers['x-correlation-id']).toBe('corr-multi');
        expect(req?.headers['authorization']).toBe(`Bearer ${token}`);
      }
    });
  });
});
