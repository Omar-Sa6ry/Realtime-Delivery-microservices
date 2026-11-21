/**
 * Gateway bootstrap, REST endpoints, GraphQL plumbing & introspection.
 *
 * Boots the real `AppModule` (Apollo Gateway + guards + middlewares + rate
 * limiter) against real mock federation subgraphs on ephemeral ports.
 */
import request from 'supertest';

import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

describe('API Gateway (bootstrap, REST, GraphQL plumbing)', () => {
  let h: GatewayHarness;

  beforeAll(async () => {
    h = await createGatewayHarness();
  });

  afterAll(async () => {
    await h.close();
  });

  describe('REST: GET /ping', () => {
    it('responds with the gateway liveness envelope', async () => {
      const res = await request(h.app.getHttpServer()).get('/ping').expect(200);

      expect(res.body).toMatchObject({
        success: true,
        statusCode: 200,
        message: 'Api Gateway is running',
        data: true,
      });
    });

    it('is reachable without any authentication', async () => {
      await request(h.app.getHttpServer()).get('/ping').expect(200);
    });
  });

  describe('REST: GET /health', () => {
    it('reports ok status with system stats', async () => {
      const res = await request(h.app.getHttpServer())
        .get('/health')
        .expect(200);

      expect(res.body).toMatchObject({ status: 'ok', service: 'api-gateway' });
      expect(typeof res.body.timestamp).toBe('string');
      // HealthService.getSystemStats() → { uptime, memory, cpuUsage }
      expect(res.body.system).toBeDefined();
      expect(typeof res.body.system.uptime).toBe('number');
      expect(res.body.system.memory).toHaveProperty('heapUsedMemoryMB');
      expect(res.body.system.cpuUsage).toBeDefined();
    });
  });

  describe('REST: GET /metrics', () => {
    it('serves Prometheus metrics as text/plain', async () => {
      const res = await request(h.app.getHttpServer())
        .get('/metrics')
        .expect(200);

      expect(res.headers['content-type']).toContain('text/plain');
      // prom-client exposition format is a plain string (may be empty until
      // the first metric is pushed, but the endpoint must always answer).
      expect(typeof res.text).toBe('string');
    });

    it('accepts the prometheus format query param', async () => {
      await request(h.app.getHttpServer())
        .get('/metrics?format=prometheus')
        .expect(200);
    });
  });

  describe('GraphQL endpoint', () => {
    it('answers a cross-subgraph query through the composed supergraph', async () => {
      const res = await h
        .gql(
          `
          query Smoke {
            pingForUser { statusCode success data }
            pingForNotification { statusCode success data }
            pingForRealtime { statusCode success data }
          }
        `,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.pingForUser).toMatchObject({
        statusCode: 200,
        success: true,
        data: true,
      });
      expect(res.body.data.pingForNotification).toMatchObject({
        success: true,
        data: true,
      });
      expect(res.body.data.pingForRealtime).toMatchObject({
        success: true,
        data: true,
      });

      // each ping landed on its own subgraph
      expect(h.subgraphs.last('user')?.fields).toContain('pingForUser');
      expect(h.subgraphs.last('notification')?.fields).toContain(
        'pingForNotification',
      );
      expect(h.subgraphs.last('realtime')?.fields).toContain('pingForRealtime');
    });

    it('returns 404 for unknown REST paths', async () => {
      await request(h.app.getHttpServer()).get('/does-not-exist').expect(404);
    });
  });

  describe('introspection', () => {
    it('exposes the full composed schema (every subgraph is registered)', async () => {
      const res = await h
        .gql(
          `
          query IntrospectionQuery {
            __schema {
              queryType { name }
              mutationType { name }
              types { name kind }
            }
          }
        `,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();

      const types = new Set<string>(
        res.body.data.__schema.types.map((t: any) => t.name),
      );

      // core federation types
      for (const t of [
        'Query',
        'Mutation',
        'User',
        'Driver',
        'Delivery',
        'Payment',
      ]) {
        expect(types).toContain(t);
      }
      // one type unique to each subgraph proves all 9 composed
      for (const t of [
        'AuthPayload', // user
        'DeliveryNextStatusesResponse', // delivery
        'SelfAcceptData', // driver
        'PaymentsResponse', // payment
        'NotificationsResponse', // notification
        'ActiveConnectionsResponse', // realtime
        'UploadSessionData', // media
        'AutocompleteResponse', // search
        'PlatformOverviewResponse', // analytics
      ]) {
        expect(types).toContain(t);
      }
    });

    it('does not throttle introspection (skipIntrospection)', async () => {
      // introspection queries start with `query IntrospectionQuery` and the
      // rate limiter skips them entirely
      const introspection = `query IntrospectionQuery {
        __schema { types { name } }
      }`;
      for (let i = 0; i < 5; i += 1) {
        const res = await h.gql(introspection).expect(200);
        expect(res.body.errors).toBeUndefined();
      }
    });
  });

  describe('GraphQL validation', () => {
    it('rejects unknown fields with a validation error payload', async () => {
      // Apollo Server answers 400 for document-level validation failures
      const res = await h.gql(`{ thisFieldDoesNotExist }`).expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0].message).toContain('thisFieldDoesNotExist');
      // formatError normalizes to the gateway envelope
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: expect.any(Number),
        message: expect.any(String),
        timeStamp: expect.any(String),
      });
    });

    it('rejects a syntactically invalid document', async () => {
      const res = await h.gql(`{ pingForUser { `).expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0].message).toMatch(/syntax|parse/i);
    });

    // Delivery → User.deliveries → Delivery … is a real cycle in the composed
    // schema, so it is what the depth limit is really protecting against.
    // 8 nested selection sets exceed `depthLimit(7)`…
    it('enforces the depth limit of 7 (Delivery ⇄ User cycle)', async () => {
      const tooDeep = `{
        delivery(id: "d-1") {
          data {
            customer {
              deliveries {
                customer {
                  deliveries {
                    customer {
                      deliveries { id }
                    }
                  }
                }
              }
            }
          }
        }
      }`;

      const res = await h.gql(tooDeep).expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0].message).toContain(
        'exceeds maximum operation depth of 7',
      );
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });

    // …while the same query one level shallower sits exactly on the limit.
    it('allows queries at exactly the depth limit', async () => {
      const atLimit = `{
        delivery(id: "d-1") {
          data {
            customer {
              deliveries {
                customer {
                  deliveries { id status }
                }
              }
            }
          }
        }
      }`;

      const res = await h.gql(atLimit).expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(
        res.body.data.delivery.data.customer.deliveries[0].customer
          .deliveries[0].id,
      ).toBe('delivery-1');
      expect(
        res.body.data.delivery.data.customer.deliveries[0].customer
          .deliveries[0].status,
      ).toBeDefined();
    });
  });

  describe('GraphQL Playground', () => {
    it('serves the playground HTML on GET /graphql', async () => {
      const res = await request(h.app.getHttpServer())
        .get('/graphql')
        .set('Accept', 'text/html')
        .expect(200);

      expect(res.text).toMatch(/GraphQL Playground|graphql-playground/i);
    });
  });

  describe('correlation id middleware', () => {
    it('generates an x-correlation-id when the client does not send one', async () => {
      const res = await h.gql(`{ pingForUser { success } }`).expect(200);

      expect(res.headers['x-correlation-id']).toMatch(
        /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
      );
    });

    it('echoes back a client-supplied x-correlation-id', async () => {
      const res = await h
        .gql(`{ pingForUser { success } }`, undefined, {
          headers: { 'x-correlation-id': 'corr-test-123' },
        })
        .expect(200);

      expect(res.headers['x-correlation-id']).toBe('corr-test-123');
      // and the gateway forwards it to the subgraph
      expect(h.subgraphs.last('user')?.headers['x-correlation-id']).toBe(
        'corr-test-123',
      );
    });
  });
});
