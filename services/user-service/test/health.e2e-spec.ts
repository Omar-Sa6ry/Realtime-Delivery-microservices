import request from 'supertest';
import {
  E2EApp,
  bootstrap,
  shutdown,
  resetState,
  gql,
  QUERIES,
} from './support/harness';

/**
 * Boot, probes and metrics: the REST surface the Kubernetes manifests and the
 * api-gateway health checks rely on, plus the `pingForUser` GraphQL query that
 * proves the federated endpoint is reachable.
 */
describe('Service boot, probes and metrics', () => {
  let ctx: E2EApp;
  let server: any;

  beforeAll(async () => {
    ctx = await bootstrap();
    server = ctx.app.getHttpServer();
  }, 60000);

  afterAll(async () => {
    await shutdown(ctx);
  }, 30000);

  beforeEach(async () => {
    await resetState(ctx);
  });

  describe('GET /user/health (liveness)', () => {
    it('returns UP with service identity, timestamp and system stats', async () => {
      const res = await request(server).get('/user/health').expect(200);

      expect(res.body.status).toBe('UP');
      expect(res.body.service).toBe('user-service');
      expect(typeof res.body.timestamp).toBe('string');
      expect(Number.isNaN(Date.parse(res.body.timestamp))).toBe(false);

      expect(res.body.system).toBeDefined();
      expect(typeof res.body.system.uptime).toBe('number');
      expect(typeof res.body.system.memory.heapUsedMemoryMB).toBe('number');
      expect(res.body.system.memory.rssMB).toBeGreaterThan(0);
      expect(res.body.system.cpuUsage).toBeDefined();
    });
  });

  describe('GET /user/ready (readiness)', () => {
    it('reports UP with database and redis checks when both are reachable', async () => {
      const res = await request(server).get('/user/ready').expect(200);

      expect(res.body.status).toBe('UP');
      expect(Number.isNaN(Date.parse(res.body.timestamp))).toBe(false);
      expect(res.body.checks.database).toEqual({ status: 'UP' });
      expect(res.body.checks.redis).toEqual({ status: 'UP' });
      expect(res.body.system.memory.rssMB).toBeGreaterThan(0);
    });

    it('reports DOWN with a 503 when the database check fails', async () => {
      // Temporarily break the DB check to exercise the failing branch.
      const original = ctx.dataSource.query.bind(ctx.dataSource);
      (ctx.dataSource as any).query = jest
        .fn()
        .mockRejectedValue(new Error('connection lost'));

      try {
        const res = await request(server).get('/user/ready').expect(503);
        expect(res.body.status).toBe('DOWN');
        expect(res.body.checks.database.status).toBe('DOWN');
        expect(res.body.checks.database.message).toBe('connection lost');
        expect(res.body.checks.redis.status).toBe('UP');
      } finally {
        (ctx.dataSource as any).query = original;
      }
    });
  });

  describe('GET /user/metrics (Prometheus)', () => {
    it('serves the prometheus exposition with application counters', async () => {
      await request(server).get('/user/health').expect(200);

      const res = await request(server).get('/user/metrics').expect(200);

      expect(res.headers['content-type']).toContain('text/plain');
      expect(res.text).toContain('# HELP');
      expect(res.text).toContain('# TYPE');
      expect(res.text).toContain('app_requests_total');
      expect(res.text).toContain('app_request_duration_seconds');
    });
  });

  describe('GraphQL pingForUser', () => {
    it('answers the ping query with the success envelope', async () => {
      const res = await gql(ctx.app, { query: QUERIES.pingForUser });

      expect(res.status).toBe(200);
      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.pingForUser).toEqual({
        success: true,
        statusCode: 200,
        message: 'User service is running',
        data: true,
      });
    });
  });

  describe('application container', () => {
    it('boots with an initialized data source and the in-memory redis', () => {
      expect(ctx.dataSource.isInitialized).toBe(true);
      expect(ctx.redis.size()).toBeGreaterThanOrEqual(0);
    });
  });
});
