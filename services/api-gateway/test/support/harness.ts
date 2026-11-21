import { INestApplication } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import request from 'supertest';
import { App } from 'supertest/types';
import { JwtService } from '@nestjs/jwt';

import { startMockSubgraphs, MockSubgraphContext } from './mock-subgraphs';
import {
  FakeRedisService,
  createFakeRedisClient,
  createFakeCacheManager,
} from './fake-redis';

export const JWT_SECRET = 'api-gateway-e2e-secret';

export interface GqlRequestOptions {
  token?: string;
  headers?: Record<string, string>;
  /** Expect an HTTP-level (non-200) response and return it raw. */
  raw?: boolean;
}

export interface GatewayHarness {
  app: INestApplication<App>;
  subgraphs: MockSubgraphContext;
  redis: FakeRedisService;
  jwt: JwtService;
  /** POST a GraphQL operation to the gateway's /graphql endpoint. */
  gql(
    query: string,
    variables?: Record<string, unknown>,
    options?: GqlRequestOptions,
  ): request.Test;
  /** Sign a JWT like the services do (sub / role / sessionId). */
  signToken(payload: Record<string, unknown>): string;
  close(): Promise<void>;
}

/**
 * Boots the REAL gateway AppModule (Apollo Gateway + guards + middlewares +
 * rate limiter) composing against real mock Federation subgraphs, with all
 * Redis access faked so no external process is required.
 *
 * `AppModule` reads subgraph URLs from process.env at module-decoration time,
 * so we set those env vars *before* dynamically importing the module.
 */
export async function createGatewayHarness(
  env: Record<string, string> = {},
): Promise<GatewayHarness> {
  const subgraphs = await startMockSubgraphs();

  // Point the gateway at the mock subgraphs (URLs read at import time).
  process.env.JWT_SECRET = JWT_SECRET;
  // Functional specs should never be throttled by the global rate limiter;
  // the dedicated rate-limit spec overrides these with a tiny budget.
  process.env.RATE_LIMIT_LIMIT = process.env.RATE_LIMIT_LIMIT ?? '100000';
  process.env.RATE_LIMIT_WINDOW_MS =
    process.env.RATE_LIMIT_WINDOW_MS ?? '60000';
  for (const [name, url] of Object.entries(subgraphs.urls)) {
    const envVar = {
      user: 'USER_SERVICE_URL',
      delivery: 'DELIVERY_SERVICE_URL',
      driver: 'DRIVER_SERVICE_URL',
      payment: 'PAYMENT_SUBGRAPH_URL',
      notification: 'NOTIFICATION_SERVICE_URL',
      realtime: 'REALTIME_SERVICE_URL',
      media: 'MEDIA_SUBGRAPH_URL',
      search: 'SEARCH_SERVICE_URL',
      analytics: 'ANALYTICS_SERVICE_URL',
    }[name];
    if (envVar) process.env[envVar] = url;
  }
  Object.assign(process.env, env);

  // `require` (not dynamic `import`) so Jest's CJS runtime can load it; the
  // call happens at runtime, *after* the env vars above are set, which is the
  // whole point — AppModule reads them while being decorated.

  const { AppModule } = require('../../src/app.module');

  const fakeRedis = new FakeRedisService();

  const moduleRef = await Test.createTestingModule({ imports: [AppModule] })
    .overrideProvider('REDIS_CLIENT')
    .useValue(createFakeRedisClient())
    .overrideProvider('CACHE_MANAGER')
    .useValue(createFakeCacheManager())
    .overrideProvider(require('@bts-soft/cache').RedisService)
    .useValue(fakeRedis)
    .compile();

  const app = moduleRef.createNestApplication();
  await app.init();

  const jwt = app.get(JwtService);

  const gql = (
    query: string,
    variables?: Record<string, unknown>,
    options: GqlRequestOptions = {},
  ) => {
    let req = request(app.getHttpServer()).post('/graphql');
    if (options.token)
      req = req.set('Authorization', `Bearer ${options.token}`);
    for (const [k, v] of Object.entries(options.headers ?? {}))
      req = req.set(k, v);
    return req.send(variables ? { query, variables } : { query });
  };

  const signToken = (payload: Record<string, unknown>) =>
    jwt.sign(payload, { secret: JWT_SECRET });

  return {
    app,
    subgraphs,
    redis: fakeRedis,
    jwt,
    gql,
    signToken,
    close: async () => {
      await app.close();
      await subgraphs.stop();
    },
  };
}
