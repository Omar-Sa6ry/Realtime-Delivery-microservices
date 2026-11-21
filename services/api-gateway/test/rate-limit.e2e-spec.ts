import { INestApplication } from '@nestjs/common';
import request from 'supertest';
import { createGatewayHarness, GatewayHarness } from './support/harness';

/**
 * E2E: rate limiting on the API gateway.
 *
 * The global `RateLimiter` guard (from `@bts-soft/validation`) only intercepts
 * REST requests — GraphQL operations bypass Nest's guard pipeline entirely (see
 * the dedicated test below).
 *
 * Implementation reality (verified against the installed library source):
 *  - `RateLimiterGuard.onModuleInit()` builds the limiter via
 *    `createAlgorithm({ ...config, ipExtractionOptions })` **without** the
 *    `store`, so `config.store` (the configured `RedisStore`) is dropped and the
 *    algorithm always falls back to `new InMemoryStore()`. The bucket therefore
 *    lives in the gateway process and is *never* written to Redis.
 *  - The key is derived from `x-forwarded-for` → `x-real-ip` → `req.ip`
 *    (`ip-extractor.util.js`), so distinct `X-Forwarded-For` values get
 *    independent budgets.
 *  - The window is a fixed (non-sliding) window: when `RATE_LIMIT_WINDOW_MS`
 *    elapses, `bucket.resetWindow()` empties the bucket and the budget is
 *    available again.
 */
jest.setTimeout(120_000);

describe('Rate limiting (e2e)', () => {
  let app: INestApplication;
  let h: GatewayHarness;
  let seq = 0;

  const IP = () => `10.0.${Math.floor(seq / 200)}.${(seq++ % 200) + 1}`;

  beforeAll(async () => {
    h = await createGatewayHarness({
      RATE_LIMIT_LIMIT: '3',
      RATE_LIMIT_WINDOW_MS: '1000',
    });
    app = h.app;
  });

  afterAll(async () => {
    await h?.close();
  });

  it('sets X-RateLimit-* headers and counts down on every request', async () => {
    const ip = IP();
    for (let i = 0; i < 3; i++) {
      const res = await request(app.getHttpServer())
        .get('/ping')
        .set('x-forwarded-for', ip)
        .expect(200);

      expect(res.headers['x-ratelimit-limit']).toBe('3');
      expect(res.headers['x-ratelimit-remaining']).toBe(String(2 - i));
      expect(res.headers['x-ratelimit-reset']).toBeDefined();
    }
  });

  it('answers 429 with a Retry-After header once the window budget is spent', async () => {
    const ip = IP();
    for (let i = 0; i < 3; i++) {
      await request(app.getHttpServer())
        .get('/ping')
        .set('x-forwarded-for', ip)
        .expect(200);
    }

    const res = await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ip)
      .expect(429);

    expect(res.headers['retry-after']).toBeDefined();
    expect(res.body).toMatchObject({
      statusCode: 429,
    });
    expect(res.body.message).toMatch(/too many requests|rate limit/i);
  });

  it('shares one bucket across REST routes for the same client', async () => {
    const ip = IP();
    await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ip)
      .expect(200);
    await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ip)
      .expect(200);
    await request(app.getHttpServer())
      .get('/health')
      .set('x-forwarded-for', ip)
      .expect(200);

    // fourth request in the window → rejected, on a different route too
    const res = await request(app.getHttpServer())
      .get('/health')
      .set('x-forwarded-for', ip)
      .expect(429);
    expect(res.body).toMatchObject({ statusCode: 429 });
  });

  it('keys buckets by client IP: distinct X-Forwarded-For values get independent budgets', async () => {
    const ipA = IP();
    const ipB = IP();

    await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ipA)
      .expect(200);
    await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ipA)
      .expect(200);

    // ipB is untouched by ipA's consumption
    await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ipB)
      .expect(200);

    // ipA is now over its budget
    await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ipA)
      .expect(200);
    await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ipA)
      .expect(429);
  });

  it('recovers the budget once the fixed window has elapsed', async () => {
    const ip = IP();

    for (let i = 0; i < 3; i++) {
      await request(app.getHttpServer())
        .get('/ping')
        .set('x-forwarded-for', ip)
        .expect(200);
    }
    await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ip)
      .expect(429);

    // RATE_LIMIT_WINDOW_MS = 1000 for this suite — wait it out.
    await new Promise((r) => setTimeout(r, 1100));

    const res = await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ip)
      .expect(200);
    expect(res.headers['x-ratelimit-remaining']).toBe('2');
  });

  it('does not throttle GraphQL operations (guards do not run on the gateway pipeline)', async () => {
    // Far beyond RATE_LIMIT_LIMIT=3; the bucket is never touched for GraphQL.
    for (let i = 0; i < 10; i++) {
      const res = await request(app.getHttpServer())
        .post('/graphql')
        .send({ query: '{ pingForUser { statusCode success data } }' })
        .expect(200);
      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.pingForUser).toMatchObject({
        statusCode: 200,
        success: true,
        data: true,
      });
    }
  });

  it('keeps the bucket in process memory — it never writes to Redis', async () => {
    const ip = IP();
    for (let i = 0; i < 3; i++) {
      await request(app.getHttpServer())
        .get('/ping')
        .set('x-forwarded-for', ip)
        .expect(200);
    }
    await request(app.getHttpServer())
      .get('/ping')
      .set('x-forwarded-for', ip)
      .expect(429);

    // The configured RedisStore is never passed to the algorithm (createAlgorithm
    // drops it), so nothing has been persisted in Redis.
    expect(h.redis.size()).toBe(0);
  });
});
