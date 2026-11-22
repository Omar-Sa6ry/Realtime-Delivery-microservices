# API Gateway

The API Gateway is the single public entry point for the Realtime Delivery platform. It is a NestJS application that composes every backend subgraph into one Apollo Federation v2 supergraph, exposes a unified GraphQL endpoint at `/graphql`, terminates JWT authentication, enforces Redis-backed rate limiting, normalises all errors into one envelope, and proxies WebSocket (realtime) and S3 (LocalStack) traffic to the correct downstream service. It also forwards identity, correlation and language headers to subgraphs so each service can authorise and localise without re-parsing tokens.

No client ever talks to a subgraph directly. Every request enters through this gateway.

---

## Table of Contents

- [Overview](#overview)
- [Tech Stack](#tech-stack)
- [Repository Layout](#repository-layout)
- [Startup Sequence](#startup-sequence)
- [Request Lifecycle](#request-lifecycle)
- [Key Components](#key-components)
  - [main.ts Bootstrap](#maints-bootstrap)
  - [AppModule](#appmodule)
  - [JwtAuthGuard](#jwtauthguard)
  - [JwtAuthMiddleware](#jwtauthmiddleware)
  - [CorrelationIdMiddleware](#correlationidmiddleware)
  - [GraphQL Context and applyUserFromToken](#graphql-context-and-applyuserfromtoken)
  - [SubgraphService and willSendRequest](#subgraphservice-and-willsendrequest)
  - [formatGatewayError](#formatgatewayerror)
  - [Rate Limiter Factory](#rate-limiter-factory)
  - [HTTP Controllers](#http-controllers)
  - [waitService.util](#waitserviceutil)
- [GraphQL API](#graphql-api)
- [HTTP Endpoints](#http-endpoints)
- [WebSocket and S3 Proxies](#websocket-and-s3-proxies)
- [Error Envelope](#error-envelope)
- [Authentication and Header Forwarding](#authentication-and-header-forwarding)
- [Rate Limiting](#rate-limiting)
- [Configuration](#configuration)
- [Docker Compose](#docker-compose)
- [Makefile](#makefile)
- [Scripts](#scripts)
- [Testing](#testing)
- [Coverage](#coverage)
- [Dependencies](#dependencies)
- [Design Decisions](#design-decisions)
- [Troubleshooting](#troubleshooting)

---

## Overview

Responsibilities of this service:

1. Compose nine GraphQL subgraphs (user, delivery, driver, payment, media, notification, realtime, search, analytics) into a single federated supergraph via `IntrospectAndCompose`, polling every 5 seconds for schema changes.
2. Expose the unified endpoint `POST /graphql` (Playground and introspection enabled) with a `graphql-depth-limit` of 7 to block deeply nested query attacks.
3. Terminate JWT authentication globally: a middleware decodes the bearer token into `req.user`, and a global guard verifies the signature for guarded operations while still allowing public ones (login, register, health, metrics).
4. Forward identity (`x-user-id`, `x-user-role`, x-user-session), correlation (`x-correlation-id`), `authorization`, and language (`x-lang`, `accept-language`) headers to every subgraph call.
5. Enforce a token-bucket rate limit backed by Redis, with introspection exempted.
6. Normalise every error (subgraph, legacy subgraph, or gateway-level) into a single `{ success, statusCode, message, timeStamp, error }` envelope.
7. Generate or propagate a correlation ID on every HTTP request for distributed tracing.
8. Proxy WebSocket traffic for `/realtime` and `/ws` to the Realtime Service, and HTTP `/localstack` traffic to LocalStack S3.
9. Wait for Redis and all nine subgraphs to become reachable before booting (with exponential backoff), and retry the whole bootstrap every 10 seconds on failure.
10. Expose liveness on a dedicated port (`PORT_LIVENESS`, default `4099`) and business endpoints `/ping`, `/health`, `/metrics` on the main port.

---

## Tech Stack

| Concern | Technology |
| --- | --- |
| Runtime | Node.js with TypeScript 5.7 |
| Framework | NestJS 11 (Express platform) |
| GraphQL | Apollo Federation v2 gateway (`ApolloGatewayDriver` + `IntrospectAndCompose`) |
| Federation compose | `@apollo/gateway` with a custom `RemoteGraphQLDataSource` |
| Depth limiting | `graphql-depth-limit` (max depth 7) |
| Auth | JWT via `@nestjs/jwt` (global guard + middleware) |
| Rate limiting | `RateLimiter` (token bucket) + `RedisStore` from `@bts-soft/validation` |
| Cache | `@bts-soft/cache` (`RedisModule`, `RedisService`) |
| Proxying | `http-proxy-middleware` (WebSocket + LocalStack S3) |
| Security headers | `helmet` (CSP disabled outside production) |
| CORS | `origin: *`, `credentials: true` |
| Shared platform SDK | `@delivery-micro/shard` (LoggingModule, MetricsModule, AutomationModule, MetricsInterceptor, HealthService, MetricsService, StructuredLogger) |
| Observability | Prometheus metrics at `/metrics` (`prom-client` via shard), structured logs |
| Liveness | Dedicated Node `http.Server` on `PORT_LIVENESS` |
| Testing | Jest 30 + ts-jest, supertest (e2e), real Apollo subgraph servers |
| Task runner | Makefile + npm scripts |

---

## Repository Layout

The service lives at `services/api-gateway` inside the monorepo:

```
Realtime Delivery microservices/
|-- config/env/                          # .env.development / .env.production (shared env files)
|-- infrastructure/docker/compose.yml    # api-gateway service definition (build, ports, env)
|-- graphql-docs/                        # Documented operations per subgraph (mirrored by the e2e SDL)
|-- services/
    |-- api-gateway/                     # This service
```

Full source tree:

```
services/api-gateway/
|-- Dockerfile                           # node:24 image, build context = repo root
|-- Makefile                             # Task runner (install/build/test/docker/clean)
|-- README.md
|-- eslint.config.mjs                    # ESLint flat config (recommended + prettier)
|-- nest-cli.json
|-- package.json                         # Scripts, jest config, 100% coverage thresholds
|-- tsconfig.json                        # isolatedModules: false (see Design Decisions)
|-- tsconfig.build.json                  # Excludes test/ and *.spec.ts
|-- src/
|   |-- main.ts                          # Bootstrap: wait, liveness, proxies, process guards
|   |-- main.spec.ts
|   |-- app.module.ts                    # Root module wiring (gateway, guards, middleware)
|   |-- app.module.spec.ts
|   |-- app.controller.ts                # GET /ping
|   |-- app.controller.spec.ts
|   |-- common/
|   |   |-- graphql/
|   |   |   |-- format-error.ts          # Unified error envelope
|   |   |   |-- graphql-context.ts       # Context factory + applyUserFromToken
|   |   |   |-- subgraph-service.ts      # RemoteGraphQLDataSource + willSendRequest
|   |   |   `-- *.spec.ts
|   |   |-- guards/
|   |   |   |-- auth.guard.ts            # JwtAuthGuard (global APP_GUARD)
|   |   |   |-- rate-limiter.factory.ts  # createRateLimiterGuard (global APP_GUARD)
|   |   |   `-- *.spec.ts
|   |   `-- middlewares/
|   |       |-- correlation-id.middleware.ts
|   |       |-- jwt-auth.middleware.ts
|   |       `-- *.spec.ts
|   |-- health/
|   |   |-- health.controller.ts         # GET /health
|   |   |-- metrics.controller.ts        # GET /metrics
|   |   `-- *.spec.ts
|   `-- utils/
|       |-- waitService.util.ts          # waitForService / waitForRedis (backoff loops)
|       `-- waitService.util.spec.ts
`-- test/
    |-- jest-e2e.json                    # E2E jest config (maxWorkers pinned in npm script)
    |-- *.e2e-spec.ts                    # 14 end-to-end suites
    `-- support/
        |-- harness.ts                   # createGatewayHarness (app + mocks + gql helper)
        |-- mock-subgraphs.ts            # Real Apollo subgraph servers with captured requests
        |-- fake-redis.ts                # In-memory RedisService / client / cache-manager fakes
        |-- sdl.ts                       # Federation v2 SDL for all nine mock subgraphs
        `-- uuid-shim.ts                 # uuid@14 (ESM-only) shim for Jest CJS
```

---

## Startup Sequence

`main.ts` runs a strictly ordered bootstrap:

1. `runIfEntryPoint()` fires only when `dist/main` is the process entry point. When `main.ts` is imported by tests, all side effects (ports, network) are skipped.
2. `registerProcessHandlers()` installs `unhandledRejection` (log) and `uncaughtException` (log + `process.exit(1)`) guards.
3. `startLivenessServer()` binds a bare `http.Server` on `PORT_LIVENESS` (default `4099`) that answers `{ status: "alive", service: "api-gateway" }` on every path. This port is independent of the main app, so orchestrators can probe liveness even while the gateway is still waiting on dependencies.
4. `runBootstrap()` calls `bootstrap()` and, on any rejection, logs and retries the entire bootstrap after 10 seconds.
5. `waitForDependencies()` blocks until:
   - Redis accepts a TCP connection (`waitForRedis`, 2s retry loop), then
   - all nine subgraphs answer a `{ __typename }` probe (`waitForService`, exponential backoff from 5s to a 30s cap, 5s per-attempt timeout).
6. `AppModule` and `NestFactory` are lazily `require`d only after dependencies are reachable, so the GraphQL gateway never attempts supergraph composition against dead services.
7. The Nest app is created with the shared `StructuredLogger`, `trust proxy` is set to `1`, shutdown hooks are enabled, `helmet` is applied (CSP only in production), and CORS is opened.
8. `setupProxies()` registers the WebSocket proxy (realtime) and the LocalStack S3 proxy.
9. The app listens on `PORT_GATEWAY` (default `4000`) on `0.0.0.0`.
10. The HTTP server's `upgrade` event is wired to the WebSocket proxy for `/realtime*` and `/ws*` URLs only.

---

## Request Lifecycle

A typical authenticated GraphQL mutation:

```
Client
  |
  |  POST /graphql  Authorization: Bearer <jwt>   x-lang: ar
  v
CorrelationIdMiddleware     -- ensures x-correlation-id exists, echoes it on the response
  v
JwtAuthMiddleware           -- verifies the token, sets req.user (invalid token => anonymous)
  v
JwtAuthGuard (APP_GUARD)    -- re-verifies; allows requests without a token through (public ops)
  v
RateLimiter Guard (APP_GUARD) -- token bucket in Redis; introspection exempt
  v
Apollo Gateway              -- parses, validates (depth <= 7), plans the query
  |                           formatGatewayError normalises any error
  v
willSendRequest             -- copies identity/correlation/lang headers onto the subgraph request
  v
Subgraph(s)                 -- user, delivery, driver, ... resolve their fields
  v
Response (or normalised error envelope) back to the client
```

MetricsInterceptor (global, from `@delivery-micro/shard`) records request duration and outcomes for Prometheus.

---

## Key Components

### main.ts Bootstrap

| Export | Purpose |
| --- | --- |
| `interopDefault(mod)` | Resolves a CommonJS module to its default export when one exists (used for `helmet`, which ships both shapes). |
| `startLivenessServer()` | Creates and returns the dedicated liveness `http.Server` on `PORT_LIVENESS`. |
| `waitForDependencies()` | Waits for Redis, then for all nine subgraphs in parallel. |
| `setupProxies(app)` | Registers the WebSocket proxy (realtime) and HTTP proxy (LocalStack); returns the ws proxy so `bootstrap` can attach the `upgrade` handler. |
| `bootstrap()` | The full startup described above. |
| `runBootstrap()` | Wraps `bootstrap()` with a 10-second retry loop on failure. |
| `registerProcessHandlers()` | Process-level `unhandledRejection` / `uncaughtException` guards. |
| `runIfEntryPoint(isEntryPoint?, actions?)` | Entry-point predicate with injectable actions, so unit tests can exercise the branch without binding ports. |

### AppModule

`src/app.module.ts` wires everything:

- `ConfigModule.forRoot` (global) reading `.env` from the service dir and `../../config/.env`.
- `JwtModule.register` with `JWT_SECRET` (fallback `super-secret-jwt-key`).
- `RedisModule` from `@bts-soft/cache`.
- `GraphQLModule.forRoot<ApolloGatewayDriverConfig>`:
  - `server`: `context: buildGraphQLContext`, `validationRules: [depthLimit(7)]`, Playground and introspection enabled, `formatError: formatGatewayError`.
  - `gateway`: `supergraphSdl: new IntrospectAndCompose({ subgraphs, pollIntervalInMs: 5000 })` and `buildService: buildSubgraphService`.
- Shared modules: `LoggingModule`, `MetricsModule`, `AutomationModule`.
- Controllers: `HealthController`, `MetricsController`, `AppController`.
- Global providers: `JwtAuthGuard` (`APP_GUARD`), `MetricsInterceptor` (`APP_INTERCEPTOR`), and the rate limiter guard (`APP_GUARD`, factory injected with `RedisService`).
- `configure(consumer)` applies `CorrelationIdMiddleware` and `JwtAuthMiddleware` to all routes.

### JwtAuthGuard

`src/common/guards/auth.guard.ts` - the global `APP_GUARD`.

- Works for both HTTP and GraphQL execution contexts (`GqlExecutionContext`).
- Requests **without** an `Authorization: Bearer` header pass through unchanged, so public operations (login, register, health, metrics, ping) keep working; `req.user` stays undefined.
- Requests **with** a token are verified with `JWT_SECRET`; on success `req.user` is set to `{ userId, role, sessionId }`; on failure it throws `UnauthorizedException("Invalid or expired JWT token")`.

### JwtAuthMiddleware

`src/common/middlewares/jwt-auth.middleware.ts` - runs before the guard on every route.

- Verifies the bearer token and sets `req.user` (same shape as the guard).
- An invalid or expired token is treated as anonymous rather than an error, so a stale token cannot break public mutations; downstream subgraphs still enforce authentication where required.

### CorrelationIdMiddleware

`src/common/middlewares/correlation-id.middleware.ts`

- Reads `x-correlation-id` from the incoming request; if absent it generates a UUID v4.
- Stores the ID on `req.headers` (so `willSendRequest` forwards it) and echoes it on the response.

### GraphQL Context and applyUserFromToken

`src/common/graphql/graphql-context.ts`

- `applyUserFromToken(req)` decodes (without verifying) the JWT payload and attaches `{ userId, role, sessionId }` to `req.user`. It never throws: a malformed token simply leaves the request anonymous. Signature verification is the subgraphs' responsibility.
- `buildGraphQLContext({ req })` is the Apollo `context` factory; it returns `{ req }`.

### SubgraphService and willSendRequest

`src/common/graphql/subgraph-service.ts`

- `willSendRequest({ request, context })` copies the following onto every subgraph request:
  - `x-user-id`, `x-user-role`, `x-user-session` (from `req.user`, falling back to decoding the token),
  - `x-correlation-id`,
  - `authorization` (the original bearer token),
  - `x-lang` and `accept-language`.
- `buildSubgraphService({ url })` returns a `RemoteGraphQLDataSource` with `willSendRequest` attached; this is the gateway's `buildService`.

### formatGatewayError

`src/common/graphql/format-error.ts` normalises three error shapes into one envelope:

1. **New-format subgraph error** - `error.extensions` carries `success: false` and a `statusCode`: the envelope is built directly from it, defaulting `timeStamp` to now.
2. **Legacy subgraph error** - `error.extensions.response.body.errors[0]`: unwrapped, `statusCode` defaults to `400`.
3. **Gateway-level error** (validation, depth limit, syntax, internal): `statusCode` comes from `extensions.statusCode` or `originalError.statusCode`, defaulting to `400`; an array message is reduced to its first entry.

### Rate Limiter Factory

`src/common/guards/rate-limiter.factory.ts` exports `createRateLimiterGuard(redisService)`, which builds the global `RateLimiter` with:

- algorithm `TOKEN_BUCKET`,
- `limit` from `RATE_LIMIT_LIMIT` (default `100`),
- `windowMs` from `RATE_LIMIT_WINDOW_MS` (default `60000`),
- `skipIntrospection: true`,
- a `RedisStore` (both as option and constructor argument).

### HTTP Controllers

| Controller | Route | Behaviour |
| --- | --- | --- |
| `AppController` | `GET /ping` | Returns a `BooleanResponse`: `{ success, statusCode, message, data: true }`. |
| `HealthController` | `GET /health` | Returns `{ status: "ok", service: "api-gateway", timestamp, system }` where `system` is `HealthService.getSystemStats()` (uptime, memory). |
| `MetricsController` | `GET /metrics` | Streams Prometheus metrics with the registry content type. |

### waitService.util

`src/utils/waitService.util.ts`

- `waitForService(url, signal?, delay = 5000, maxDelay = 30000)` - probes the URL with `POST { query: "{ __typename }" }` (5s timeout) until it answers with data; retries with 1.5x exponential backoff up to `maxDelay`. Honors an optional `AbortSignal`.
- `waitForRedis(host?, port?)` - opens a raw TCP socket (2s connect timeout) to `REDIS_HOST:REDIS_PORT`, retrying every 2 seconds until it connects.

---

## GraphQL API

The gateway is an Apollo Federation v2 **supergraph router**. It owns no schema of its own; the schema is composed from the subgraphs below. Any operation documented in `graphql-docs/*.graphql` can be executed against `/graphql`.

### Subgraphs

| Subgraph | Env var | Default URL | Service port |
| --- | --- | --- | --- |
| user | `USER_SERVICE_URL` | `http://user-srv:4001/user/graphql` | 4001 |
| payment | `PAYMENT_SUBGRAPH_URL` | `http://payment-srv:4002/payment/graphql` | 4002 |
| delivery | `DELIVERY_SERVICE_URL` | `http://delivery-srv:4003/delivery/graphql` | 4003 |
| notification | `NOTIFICATION_SERVICE_URL` | `http://notification-srv:4004/notification/graphql` | 4004 |
| media | `MEDIA_SUBGRAPH_URL` | `http://media-srv:4005/media/graphql` | 4005 |
| realtime | `REALTIME_SERVICE_URL` | `http://realtime-srv:4006/realtime/graphql` | 4006 |
| search | `SEARCH_SERVICE_URL` | `http://search-srv:4007/search/graphql` | 4007 |
| driver | `DRIVER_SERVICE_URL` | `http://driver-srv:4008/driver/graphql` | 4008 |
| analytics | `ANALYTICS_SERVICE_URL` | `http://analytics-srv:4009/analytics/graphql` | 4009 |

Composition uses `IntrospectAndCompose` with `pollIntervalInMs: 5000`, so schema changes in any subgraph are picked up within about five seconds without restarting the gateway.

### Endpoint

```
POST http://localhost:4000/graphql
Content-Type: application/json
Authorization: Bearer <jwt>      # optional for public operations
x-lang: ar                        # optional, forwarded to subgraphs

{ "query": "query { ping }", "variables": {} }
```

- Playground: enabled at `GET /graphql`.
- Introspection: enabled (and exempt from rate limiting via `skipIntrospection`).
- Depth limit: 7. Deeper queries return a formatted depth-limit error.

---

## HTTP Endpoints

| Method | Path | Port | Description |
| --- | --- | --- | --- |
| GET | `/ping` | `PORT_GATEWAY` (4000) | Liveness-style boolean check of the Nest app. |
| GET | `/health` | `PORT_GATEWAY` (4000) | Health + system stats (uptime, memory). |
| GET | `/metrics` | `PORT_GATEWAY` (4000) | Prometheus metrics. |
| POST/GET | `/graphql` | `PORT_GATEWAY` (4000) | Federated GraphQL endpoint (Playground at GET). |
| ANY | any path | `PORT_LIVENESS` (4099) | Bare liveness server: `{ status: "alive", service: "api-gateway" }`. |

---

## WebSocket and S3 Proxies

Registered in `setupProxies()`:

| Proxy | Filter | Target | Rewrite |
| --- | --- | --- | --- |
| WebSocket (realtime) | paths starting with `/realtime` or `/ws` | `http://realtime-srv:4006` | `^/realtime` -> `/ws` |
| HTTP (LocalStack S3) | path `/localstack` | `http://localstack-srv:4566` | `^/localstack` -> `` (stripped) |

The WebSocket proxy is additionally attached to the HTTP server's `upgrade` event so real socket upgrades (not just Express middleware) are forwarded for `/realtime*` and `/ws*`.

---

## Error Envelope

Every error leaves the gateway in one shape:

```json
{
  "success": false,
  "statusCode": 404,
  "message": "Delivery not found",
  "timeStamp": "2026-10-30T12:00:00.000Z",
  "error": "NotFound"
}
```

- Subgraph errors that already use the new format are re-wrapped verbatim (with `timeStamp` defaulted).
- Legacy subgraph errors nested under `extensions.response.body.errors[0]` are unwrapped (defaulting `statusCode` to 400).
- Gateway-level errors (unknown field, bad argument, depth limit, syntax) are mapped to a 400 envelope.
- Partial data of sibling fields is never masked: only the failing field carries the error.

---

## Authentication and Header Forwarding

The gateway performs authentication in two complementary layers:

| Layer | File | On missing token | On invalid token |
| --- | --- | --- | --- |
| `JwtAuthMiddleware` | `jwt-auth.middleware.ts` | request continues anonymously | request continues anonymously (subgraph enforces) |
| `JwtAuthGuard` | `auth.guard.ts` | request is allowed (public operation) | `UnauthorizedException` |

The decoded identity is attached as `req.user = { userId, role, sessionId }` (from `sub`/`userId`/`id`, `role` defaulting to `USER`, and `sessionId`).

`willSendRequest` then forwards to every subgraph:

| Header | Source |
| --- | --- |
| `x-user-id` | `req.user.userId` (or decoded token fallback) |
| `x-user-role` | `req.user.role` |
| `x-user-session` | `req.user.sessionId` (only when present) |
| `x-correlation-id` | incoming header or generated UUID |
| `authorization` | the original bearer token |
| `x-lang`, `accept-language` | incoming `x-lang` or `accept-language` |

---

## Rate Limiting

The global rate limiter guard is created by `createRateLimiterGuard`:

| Option | Value |
| --- | --- |
| Algorithm | Token bucket |
| Limit | `RATE_LIMIT_LIMIT` (default 100 requests) |
| Window | `RATE_LIMIT_WINDOW_MS` (default 60000 ms) |
| Store | Redis via `RedisStore` (shared across gateway instances) |
| Introspection | skipped (`skipIntrospection: true`) |

Because the store is Redis, the limit is enforced consistently across horizontally scaled gateway replicas.

---

## Configuration

All configuration is environment-driven.

| Variable | Default | Used by | Description |
| --- | --- | --- | --- |
| `PORT_GATEWAY` | `4000` | `main.ts` | Main HTTP/GraphQL port. |
| `PORT_LIVENESS` | `4099` | `main.ts` | Dedicated liveness server port. |
| `NODE_ENV` | (unset) | `main.ts` | Enables helmet CSP only when `production`. |
| `JWT_SECRET` | `super-secret-jwt-key` | guard, middleware, `JwtModule` | Signing secret for bearer tokens. |
| `REDIS_HOST` | `redis-srv` | `waitForRedis`, rate limiter | Redis host. |
| `REDIS_PORT` | `6379` | `waitForRedis`, rate limiter | Redis port. |
| `RATE_LIMIT_LIMIT` | `100` | rate limiter factory | Requests allowed per window. |
| `RATE_LIMIT_WINDOW_MS` | `60000` | rate limiter factory | Rate-limit window in ms. |
| `USER_SERVICE_URL` | `http://user-srv:4001/user/graphql` | gateway | User subgraph. |
| `PAYMENT_SUBGRAPH_URL` | `http://payment-srv:4002/payment/graphql` | gateway | Payment subgraph. |
| `DELIVERY_SERVICE_URL` | `http://delivery-srv:4003/delivery/graphql` | gateway | Delivery subgraph. |
| `NOTIFICATION_SERVICE_URL` | `http://notification-srv:4004/notification/graphql` | gateway | Notification subgraph. |
| `MEDIA_SUBGRAPH_URL` | `http://media-srv:4005/media/graphql` | gateway | Media subgraph. |
| `REALTIME_SERVICE_URL` | `http://realtime-srv:4006/realtime/graphql` | gateway | Realtime subgraph. |
| `SEARCH_SERVICE_URL` | `http://search-srv:4007/search/graphql` | gateway | Search subgraph. |
| `DRIVER_SERVICE_URL` | `http://driver-srv:4008/driver/graphql` | gateway | Driver subgraph. |
| `ANALYTICS_SERVICE_URL` | `http://analytics-srv:4009/analytics/graphql` | gateway | Analytics subgraph. |

Env files are loaded from `services/api-gateway/.env` and `config/.env` by `ConfigModule`; the compose stack additionally injects `config/env/.env.${APP_ENV}`.

---

## Docker Compose

The service is defined in `infrastructure/docker/compose.yml`:

```yaml
api-gateway:
  build:
    context: ../..                       # repository root
    dockerfile: services/api-gateway/Dockerfile
  container_name: api-gateway
  restart: always
  env_file:
    - ../../config/env/.env.${APP_ENV:-development}
  ports:
    - "4000:4000"
  networks:
    default:
      aliases:
        - api-gateway-srv
  environment:
    PORT_GATEWAY: ${PORT_GATEWAY:-4000}
    NODE_ENV: ${NODE_ENV:-development}
    REDIS_HOST: "redis-srv"
    REDIS_PORT: ${REDIS_PORT:-6379}
    RATE_LIMIT_LIMIT: ${RATE_LIMIT_LIMIT:-100}
    RATE_LIMIT_WINDOW_MS: ${RATE_LIMIT_WINDOW_MS:-60000}
    USER_SERVICE_URL: "http://user-srv:4001/user/graphql"
    # ... remaining subgraph URLs ...
    JWT_SECRET: ${JWT_SECRET}
  depends_on:
    - redis-srv
    - user-service
    - media-service
    - notification-service
    - realtime-service
    - search-service
    - delivery-service
  volumes:
    - ../../services/api-gateway/src:/usr/src/app/services/api-gateway/src
```

The `Dockerfile` installs with `--legacy-peer-deps`, pins `graphql@16.10.0`, runs `npm run build`, and starts `node dist/main`. Because the build context is the repository root, the image build must be invoked from a compose file (or with `-f` pointing at the repo root), not from inside the service directory.

---

## Makefile

Every command is available through `make` (default target prints the help menu). The Makefile is `cmd`-compatible on Windows: recipes only invoke `npm` subcommands, `node -e`, or `docker compose`.

| Target | Action |
| --- | --- |
| `help` | Print the categorized command list (default). |
| `install` | `npm install --legacy-peer-deps` |
| `install-ci` | `npm ci --legacy-peer-deps` (reproducible install from `package-lock.json`) |
| `start` | Build then run once |
| `start-dev` | `nest start --watch` |
| `start-debug` | `nest start --debug --watch` |
| `start-prod` | `node dist/main` |
| `format` | Prettier write |
| `lint` | ESLint with `--fix` |
| `lint-check` | ESLint without fixing |
| `format-check` | Prettier check |
| `typecheck` | `tsc -p tsconfig.json --noEmit` |
| `build` | `nest build` |
| `verify` | `format:check && lint:check && typecheck && build && test:all` |
| `audit` | `npm audit --audit-level=high`; exits non-zero while high or critical advisories are present |
| `outdated` | `npm outdated`; exits non-zero when newer versions are available |
| `prune` | `npm prune --legacy-peer-deps` |
| `test` | Unit tests |
| `test-watch` | Unit tests in watch mode |
| `test-cov` | Unit tests with coverage |
| `test-e2e` | End-to-end tests |
| `test-e2e-cov` | End-to-end tests with coverage |
| `test-all` | Unit coverage plus e2e |
| `test-debug` | Unit tests in debug mode |
| `coverage-summary` | Print the last `coverage-summary.json` totals |
| `smoke` | `GET /ping` and `GET /health` against a running gateway; non-zero exit on any failure |
| `docker-build` | `docker compose build api-gateway` |
| `docker-up` | `docker compose up -d api-gateway` |
| `docker-down` | `docker compose rm -sf api-gateway` |
| `docker-logs` | `docker compose logs -f api-gateway` |
| `docker-restart` | `docker compose restart api-gateway` |
| `docker-shell` | Open `sh` inside the running api-gateway container |
| `docker-status` | `docker compose ps api-gateway` |
| `clean` | Remove `dist`, `coverage`, `test/coverage`, temp dirs, caches |

Docker targets accept `ENV` (default `development`) to select `config/env/.env.$(ENV)`, e.g. `make docker-up ENV=production`.

The `smoke` target accepts `GATEWAY_URL` (default `http://localhost:4000`), e.g. `make smoke GATEWAY_URL=http://localhost:4100`. It succeeds only when both probes return an HTTP 2xx response, so it is suitable as a post-deploy health gate. `audit` and `outdated` intentionally exit non-zero when they find something (advisories at high level or above, or upgradable packages respectively); that is their reporting mechanism, not a broken target.

---

## Scripts

| Script | Command | Purpose |
| --- | --- | --- |
| `build` | `nest build` | Compile to `dist/`. |
| `format` | `prettier --write` | Format `src/` and `test/`. |
| `format:check` | `prettier --check` | Verify formatting. |
| `start` | `nest start` | Build and run. |
| `start:dev` | `nest start --watch` | Dev mode with reload. |
| `start:debug` | `nest start --debug --watch` | Debug mode with reload. |
| `start:prod` | `node dist/main` | Run the compiled build. |
| `lint` | `eslint ... --fix` | Lint and fix. |
| `lint:check` | `eslint ...` | Lint only. |
| `typecheck` | `tsc --noEmit` | Type-check without emitting. |
| `test` | `jest` | Unit tests. |
| `test:watch` | `jest --watch` | Unit tests, watch mode. |
| `test:cov` | `jest --coverage` | Unit tests + coverage (thresholds enforced). |
| `test:debug` | `node --inspect-brk ... jest --runInBand` | Debug unit tests. |
| `test:e2e` | `jest --config ./test/jest-e2e.json --maxWorkers=2` | E2E tests. |
| `test:e2e:cov` | same with `--coverage` | E2E tests + coverage report. |
| `test:all` | `test:cov && test:e2e` | Both suites. |
| `verify` | `format:check && lint:check && typecheck && build && test:all` | Full pipeline. |
| `clean` | `node -e ...` | Remove build output, `coverage/`, `test/coverage/`, temp dirs, caches. |

---

## Testing

Testing is split into two Jest projects:

- **Unit** (`src/**/*.spec.ts`, config in `package.json`) - 13 suites, 143 tests. Every non-spec source file has a co-located spec, and coverage thresholds are enforced at 100% for statements, branches, functions, and lines.
- **End-to-end** (`test/*.e2e-spec.ts`, config in `test/jest-e2e.json`) - 14 suites, 179 tests. These boot the **real** gateway against **real** mock Apollo subgraph servers over HTTP, exercising genuine `IntrospectAndCompose` federation, cross-subgraph queries, auth forwarding, rate limiting, and error formatting.

### Unit Suites

| Suite | Tests | Covers |
| --- | --- | --- |
| `main.spec.ts` | 32 | `interopDefault`, `startLivenessServer`, `waitForDependencies`, `setupProxies`, `bootstrap`, `runBootstrap`, `registerProcessHandlers`, `runIfEntryPoint` |
| `subgraph-service.spec.ts` | 19 | `willSendRequest` header forwarding, `buildSubgraphService` |
| `graphql-context.spec.ts` | 14 | `applyUserFromToken`, `buildGraphQLContext` |
| `auth.guard.spec.ts` | 13 | HTTP + GraphQL contexts, valid/invalid/missing tokens |
| `waitService.util.spec.ts` | 13 | `waitForService` backoff/abort, `waitForRedis` |
| `format-error.spec.ts` | 12 | All three error branches and defaults |
| `jwt-auth.middleware.spec.ts` | 11 | Header parsing, invalid token anonymity |
| `rate-limiter.factory.spec.ts` | 7 | Token bucket options, env overrides |
| `correlation-id.middleware.spec.ts` | 8 | Propagation and generation |
| `metrics.controller.spec.ts` | 4 | Prometheus stream |
| `app.module.spec.ts` | 4 | Module wiring, guard/interceptor registration |
| `app.controller.spec.ts` | 3 | `/ping` response shape |
| `health.controller.spec.ts` | 3 | `/health` response shape |
| **Total** | **143** | |

### End-to-End Suites

| Suite | Focus |
| --- | --- |
| `app.e2e-spec.ts` | `/ping`, `/health`, `/metrics`, liveness behaviour |
| `auth.e2e-spec.ts` | Login/register flows, token forwarding, role handling |
| `federation.e2e-spec.ts` | Cross-subgraph entity resolution (`@key` stubs), composed queries |
| `errors.e2e-spec.ts` | New-format, legacy, and gateway-level error envelopes |
| `rate-limit.e2e-spec.ts` | Token bucket exhaustion and recovery, introspection exemption |
| `user.e2e-spec.ts` | User subgraph operations |
| `delivery.e2e-spec.ts` | Delivery subgraph operations |
| `driver.e2e-spec.ts` | Driver subgraph operations |
| `payment.e2e-spec.ts` | Payment subgraph operations |
| `media.e2e-spec.ts` | Media subgraph operations |
| `notification.e2e-spec.ts` | Notification subgraph operations |
| `realtime.e2e-spec.ts` | Realtime subgraph operations |
| `search.e2e-spec.ts` | Search subgraph operations |
| `analytics.e2e-spec.ts` | Analytics subgraph operations |

### Test Support (`test/support/`)

| File | Role |
| --- | --- |
| `harness.ts` | `createGatewayHarness()` builds the Nest app with mock subgraph URLs, a fake Redis, and a test `JWT_SECRET`, and exposes `gql(query, variables, options)` and a `signToken` helper. |
| `mock-subgraphs.ts` | Starts real `@apollo/server` subgraph servers from the SDL in `sdl.ts`, records every incoming request (headers, variables, resolved fields), and can inject errors. |
| `fake-redis.ts` | In-memory `FakeRedisService` plus `REDIS_CLIENT` / `CACHE_MANAGER` fakes so the real rate limiter runs without a Redis process. |
| `sdl.ts` | Federation v2 SDL for all nine subgraphs, mirroring `graphql-docs/`. |
| `uuid-shim.ts` | Maps `uuid` to `crypto.randomUUID` because `uuid@14` is ESM-only and Jest runs CommonJS. |

Note: `test:e2e` pins `--maxWorkers=2`. The suites each start Apollo servers and bind ports; running at full parallelism exhausts memory, so the worker count is capped.

`make test-e2e-cov` writes its report to `test/coverage/` (HTML, `lcov`, `json-summary`), which is separate from the unit report in `coverage/`; the two runs never overwrite each other. `make clean` removes both directories.

---

## Coverage

Unit coverage is enforced at 100% across all four metrics via `coverageThreshold` in `package.json`. Current results:

| Metric | Value |
| --- | --- |
| Statements | 100% (268/268) |
| Branches | 100% (156/156) |
| Functions | 100% (45/45) |
| Lines | 100% (248/248) |

Reporters: `text`, `lcov`, `json-summary` (written to `coverage/`). `make coverage-summary` prints the totals from `coverage/coverage-summary.json`.

E2E coverage (`test:e2e:cov`) is report-only and has no thresholds, since the mock subgraphs and support files are not application source. Its artifacts go to `test/coverage/` and its `coverage-summary.json` totals only the files loaded during the end-to-end run (harness, mock subgraphs, fake Redis, SDL, uuid shim), so it never replaces the unit totals printed by `make coverage-summary`.

---

## Dependencies

| Package | Why it is here |
| --- | --- |
| `@apollo/gateway`, `@apollo/server`, `@apollo/subgraph` | Federation v2 composition, routing, and (in tests) mock subgraphs. |
| `@nestjs/apollo`, `@nestjs/graphql` | `ApolloGatewayDriver` integration. |
| `@nestjs/core`, `@nestjs/common`, `@nestjs/platform-express` | NestJS runtime. |
| `@nestjs/config` | `.env` loading. |
| `@nestjs/jwt` | Token verification in the guard and middleware. |
| `@bts-soft/cache` | `RedisModule` / `RedisService` (rate-limit store, readiness wait). |
| `@bts-soft/validation` | `RateLimiter`, `RateLimiterAlgorithm`, `RedisStore`. |
| `@delivery-micro/shard` | Logging, metrics, automation, health, and the global `MetricsInterceptor`. |
| `graphql-depth-limit` | Query depth cap (7). |
| `http-proxy-middleware` | WebSocket and LocalStack proxies. |
| `helmet` | Security headers. |
| `prom-client` | Prometheus registry (via shard `MetricsModule`). |
| `uuid` | Correlation ID generation. |

Run `make audit` to check this tree against the npm advisory database (`npm audit --audit-level=high`). The target is deliberately kept out of `verify`, because several findings originate in shared platform packages (`@delivery-micro/shard`, `@bts-soft/*`) or require major upgrades that must be coordinated across the monorepo rather than forced from this service alone.

---

## Design Decisions

1. **`isolatedModules: false`** - ts-jest's language-service emit emits plain `__metadata` decorator metadata without the transpile-mode `typeof X !== "undefined"` guards. Those dead guard branches would otherwise be counted as uncovered branches and make 100% branch coverage unreachable. The trade-off is that type errors surface at test time (slower unit runs), which is why `typecheck` is a separate, fast gate in `verify`.
2. **Two-layer auth** - the middleware is deliberately lenient (a stale token must not break public mutations), while the guard rejects invalid tokens with 401 and lets token-less requests through for public operations. Subgraphs remain the final authority.
3. **Decode-not-verify in the GraphQL context** - the gateway only *decodes* the token to populate identity headers; signature and expiry verification is delegated to subgraphs, so a single compromised gateway secret cannot forge cross-service identity silently.
4. **`IntrospectAndCompose` with a 5s poll** - schema changes propagate without gateway restarts, at the cost of a small periodic introspection load on each subgraph.
5. **Liveness on a separate port** - the bare `http.Server` on `PORT_LIVENESS` responds before Redis or any subgraph is reachable, so orchestrators never kill the container during the (potentially long) dependency wait.
6. **Lazy `require` of AppModule** - the module graph (and therefore Apollo composition) is only loaded after dependencies are reachable, preventing boot-time crashes when subgraphs start slowly.
7. **Retry-forever bootstrap** - `runBootstrap` retries every 10s on failure instead of exiting, matching the `restart: always` compose policy and giving slow infrastructure time to come up.
8. **`--maxWorkers=2` for e2e** - each suite starts real Apollo servers; unbounded parallelism causes out-of-memory crashes.
9. **ESLint flat config, non-type-checked** - `tseslint.configs.recommended` (not `recommendedTypeChecked`) keeps lint fast and consistent across services; `no-explicit-any` is off because the gateway intentionally forwards untyped payloads.

---

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| Gateway logs `[startup] Waiting for subgraphs...` forever | A subgraph is down. `waitForService` probes each URL with `{ __typename }` and backs off up to 30s. Start the missing service. |
| `[liveness] Health server listening on port 4099` but no GraphQL yet | The liveness server is up; `bootstrap` is still waiting on Redis/subgraphs. Check `REDIS_HOST` and the subgraph URLs. |
| `ts-jest[config] (WARN) TS151002` about hybrid module kind | Benign warning, silenced via `diagnostics.ignoreCodes: [151002]` in both jest configs. |
| Unit tests fail with type errors that `tsc` does not report | `isolatedModules: false` makes ts-jest type-check during transform. Run `npm run typecheck` for the authoritative check. |
| e2e runs crash with OOM at high parallelism | Keep `--maxWorkers=2` in `test:e2e` (already pinned). |
| Rate limiter rejects valid requests immediately | `RATE_LIMIT_LIMIT` / `RATE_LIMIT_WINDOW_MS` are shared across replicas via Redis; raise the limit or wait for the window to reset. |
| `make docker-build` fails to find `services/api-gateway/Dockerfile` | The build context is the repo root; build through compose (`make docker-build`) rather than from inside the service dir. |
| Public mutation returns 401 | A stale/invalid token is attached. The guard rejects invalid tokens; remove the `Authorization` header or obtain a fresh token. |
| Jest prints `A worker process has failed to exit gracefully and has been force exited` | Benign teardown warning from jest-worker when running suites in parallel. All suites still pass and the run exits with code 0 (confirmed for `test`, `test:cov`, `test:e2e`, `test:e2e:cov`, and `verify`). Diagnose with `npx jest --coverage=false --detectOpenHandles --runInBand`, which reports no open handles for this codebase. |
