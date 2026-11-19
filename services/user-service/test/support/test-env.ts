/**
 * Environment bootstrap for the end-to-end suite.
 *
 * This file is listed first in `setupFilesAfterEnv` (see test/jest-e2e.json),
 * so it runs before any test file is imported. Every value below must be in
 * place before `ConfigModule`, `JwtModule` and `TypeOrmModule` are read during
 * `TestAppModule` compilation.
 *
 * The values mirror the defaults declared by `services/user-service/Makefile`
 * (`DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME`), which are documented there
 * as the `E2E_DB_*` inputs of the e2e suite:
 *
 *   E2E_DB_HOST      -> 127.0.0.1  (IPv4 loopback; see the note below)
 *   E2E_DB_PORT      -> 5433   (docker starter-dev-postgres, container port 5432)
 *   E2E_DB_USER      -> postgres
 *   E2E_DB_PASSWORD  -> postgres
 *   E2E_DB_NAME      -> delivery_user_db_e2e   (created on demand)
 *   E2E_JWT_SECRET   -> shared secret used to sign and verify tokens
 *
 * `RoleGuard` (inside @delivery-micro/shard) verifies access tokens with
 * `process.env.JWT_SECRET`, while `JwtTokenProvider` signs them through
 * `ConfigService.get('JWT_SECRET')`. Both therefore have to observe the exact
 * same value, which is why it is written to `process.env` before the Nest
 * testing module is created.
 */

process.env.NODE_ENV = 'test';
process.env.APP_ENV = 'test';

// JWT signing/verification secret (RoleGuard reads it directly from process.env)
process.env.JWT_SECRET = process.env.E2E_JWT_SECRET || 'e2e-jwt-secret-key';

// Postgres connection used by the e2e TypeORM root module. The database itself
// is created by `ensureDatabase()` inside the harness.
// `127.0.0.1` instead of `localhost`: the dockerised Postgres publishes the
// port on the IPv4 interface only in some environments, and Node resolves
// `localhost` to `::1` first, which makes the TCP handshake hang until the
// driver times out (`read ECONNRESET` during bootstrap).
process.env.DB_HOST = process.env.E2E_DB_HOST || '127.0.0.1';
process.env.DB_PORT = process.env.E2E_DB_PORT || '5433';
process.env.DB_USERNAME = process.env.E2E_DB_USER || 'postgres';
process.env.POSTGRES_USER = process.env.E2E_DB_USER || 'postgres';
process.env.POSTGRES_PASSWORD = process.env.E2E_DB_PASSWORD || 'postgres';
process.env.DB_NAME = process.env.E2E_DB_NAME || 'delivery_user_db_e2e';

// The test harness always talks to the local dockerised Postgres; make sure a
// stale DB_HOST/DB_PORT from the developer shell cannot leak into the suite.
export const E2E_DB = {
  host: process.env.DB_HOST,
  port: Number(process.env.DB_PORT),
  user: process.env.POSTGRES_USER as string,
  password: process.env.POSTGRES_PASSWORD as string,
  database: process.env.DB_NAME as string,
};

export const E2E_JWT_SECRET = process.env.JWT_SECRET as string;
