# Search Service

Real-time search microservice for the Delivery platform. It keeps four OpenSearch indices (deliveries, drivers, media, users) continuously up to date by consuming domain events from Kafka and RabbitMQ, and exposes a GraphQL Federation v2.3 subgraph that powers full-text search, prefix autocomplete, and geo distance queries across the platform.

---

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Repository Layout](#repository-layout)
- [Package Reference](#package-reference)
- [GraphQL API](#graphql-api)
- [Event-Driven Indexing](#event-driven-indexing)
- [OpenSearch Indices](#opensearch-indices)
- [Caching](#caching)
- [Health, Metrics, and Tracing](#health-metrics-and-tracing)
- [Configuration Reference](#configuration-reference)
- [Running Locally](#running-locally)
- [Makefile](#makefile)
- [Testing](#testing)
- [Design Notes and Troubleshooting](#design-notes-and-troubleshooting)

---

## Overview

The search service is one microservice in a larger real-time delivery platform (orders, drivers, media, payments, notifications, analytics). Other services publish domain events; this service consumes them and maintains denormalized search documents in OpenSearch so that clients can query them with low latency through a single GraphQL endpoint.

Key properties:

- **Language / runtime**: Go 1.25, module `github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service`.
- **Search engine**: OpenSearch (opensearch-go v4 client), four indices behind read/write aliases.
- **API**: GraphQL (graphql-go) served as an Apollo Federation v2.3 subgraph on port `4007` (configurable), paths `/search/graphql` and `/graphql`.
- **Event ingestion**: Kafka consumer group (one consumer per topic) **and** RabbitMQ (four bindings into a single shared queue), both writing through the same indexing application service, giving dual-transport resilience.
- **Caching**: Redis cache-aside layer for search and autocomplete results.
- **Observability**: Prometheus-style metrics on port `9103`, OpenTelemetry OTLP/gRPC tracing (optional), structured `slog` logging, i18n (English/Arabic) error and success messages.
- **Shared code**: reuses the monorepo `packages/go` libraries for auth, constants, logging, events, and the RabbitMQ wrapper (wired via a `replace` directive in `go.mod`).

The service is designed around Clean Architecture: `domain` and `application` have no infrastructure imports; adapters live under `infrastructure` and `interfaces`; composition happens only in `cmd/server`.

## Features

- Full-text and filtered search over deliveries, drivers, media, and users with pagination.
- Cursor-based pagination (opaque base64 cursor carrying OpenSearch `search_after` sort values for stable deep paging).
- Prefix autocomplete with a dedicated suggestions path.
- Geo distance queries: nearby deliveries and nearby drivers (geo_point + geo_distance).
- Role-aware results: customer / driver / admin scoping applied at the resolver layer.
- Admin-only asynchronous reindex jobs via GraphQL mutation (`startReindex`) driving the OpenSearch Reindex API; the repository also exposes `SwitchAlias` for rebuild-then-repoint flows.
- Automatic index bootstrap at startup (concrete versioned indices + aliases), with configurable retry/delay knobs.
- Self-healing repository: missing-index (404) conditions trigger index re-creation before retrying the operation.
- Optimistic concurrency on upserts using an external version guard (`source_version`), so stale events cannot overwrite newer state.
- Dual event ingestion: Kafka topics and RabbitMQ exchanges feed the same indexing pipeline.
- Redis cache-aside for hot queries (60 s for result sets with hits, 300 s for autocomplete).
- Health endpoints for liveness and readiness (readiness probes OpenSearch and Redis).
- Docker image build from the monorepo root plus a docker compose stack (redis, kafka, nats, opensearch, and the service itself).

## Architecture

### Layering

```
cmd/server            composition root: wiring, HTTP server, startup order, graceful shutdown
internal/
  config              environment-driven configuration, validation, defaults
  domain/search       pure domain model: documents, queries, results, repository interface, errors
  application/
    search            query service (cache-aside) used by GraphQL resolvers
    indexing          write service: upsert / delete / bulk index documents
    reindex           async reindex job runner (OpenSearch Reindex API against <index>-v1)
  infrastructure/
    opensearch        client, index manager (mappings/aliases), search repository (DSL builder)
    kafka             consumer manager (one consumer per topic) + event handlers
    rabbitmq          consumer (queue + 4 exchange bindings) + index event handler
    redis             cache client, key generation
  interfaces/
    graphql           federation schema, resolvers, HTTP handler
    health            liveness / readiness handlers
  observability       tracing (OTLP) and metrics server
  i18n                English and Arabic translations
  testutil            fake repository and OpenSearch HTTP fixtures for tests
scripts/checktool     cross-platform helper used by the Makefile (coverage merge, fmt check, clean)
```

### Data flow

Write path:

```
producer services
  --> Kafka topics (18)  ----\
                              +--> indexing.Service --> OpenSearchRepository --> OpenSearch
  --> RabbitMQ exchanges --/         (upsert/delete/bulk, version guard)
      (orders/media/users/drivers,
       bound to queue.search_index)
```

Read path:

```
GraphQL client --> /search/graphql --> resolver (auth, role, i18n, input validation)
  --> application/search.Service (Redis cache-aside)
  --> opensearch.SearchRepository (DSL, cursor pagination)
  --> OpenSearch
```

Startup order in `run()`:

1. Load and validate configuration.
2. Initialize tracing (only when `OTEL_ENDPOINT` is set).
3. Build the OpenSearch client and run `IndexManager.EnsureIndices` with retries (`SEARCH_INDEX_INIT_MAX_RETRIES` / `SEARCH_INDEX_INIT_RETRY_DELAY_MS`) — consumers do not start before indices exist.
4. Connect Redis cache.
5. Build the indexing, search, and reindex services.
6. Start RabbitMQ consumers (skipped with a warning when `RABBITMQ_URL` is empty or unreachable).
7. Start the Kafka consumer manager (one goroutine per topic).
8. Start the metrics HTTP server and the GraphQL HTTP server.
9. Block on SIGINT/SIGTERM, then gracefully shut down both HTTP servers and cancel the consumer context.

## Repository Layout

```
services/search-service/
├── Dockerfile                     multi-stage build (golang:1.25-alpine -> alpine:3.20), EXPOSE 4007 9103
├── Makefile                       developer workflow (see Makefile section)
├── README.md                      this file
├── docker-compose.yml             local dependencies: redis, kafka, nats, opensearch
├── go.mod / go.sum                module definition; replace ../../packages/go
├── cmd/server/
│   ├── main.go                    composition root: config, wiring, run(), buildHandler, graceful shutdown
│   └── main_test.go               end-to-end style tests of run() and the HTTP mux; TestMain merges
│                                  subprocess GOCOVERDIR coverage (SUBPROCESS_COV_OUT)
├── internal/
│   ├── config/
│   │   ├── config.go              Config struct + Load() from environment, validation, clamps
│   │   └── config_test.go
│   ├── domain/search/
│   │   ├── result.go                documents (Delivery/Driver/Media/User, GeoPoint, GeoAddress),
│   │   │                            typed query structs, SearchResult[T], PageInfo, cursor encode/decode
│   │   ├── repository.go            SearchRepository interface (query + write + index management + health)
│   │   ├── errors.go                sentinel errors (ErrInvalidQuery, ErrInvalidCursor, ErrIndexNotFound,
│   │   │                            ErrDocumentNotFound, ErrVersionConflict, ErrSearchUnavailable,
│   │   │                            ErrUnauthorized, ErrReindexInProgress)
│   │   └── *_test.go
│   ├── application/
│   │   ├── search/service.go      cache-aside query service (Redis), used by resolvers
│   │   ├── indexing/service.go    UpsertDelivery/Driver/Media/User, Delete*, Bulk*, driver name enrichment
│   │   ├── reindex/service.go     StartReindex: async reindex job (OpenSearch Reindex API, -v1 target)
│   │   └── *_test.go
│   ├── infrastructure/
│   │   ├── opensearch/
│   │   │   ├── client.go          typed client wrapper (Do(ctx, req, opts)), TLS/insecure config
│   │   │   ├── index_manager.go   EnsureIndices: 4 concrete indices + aliases + mapping JSON
│   │   │   ├── repository.go      SearchRepository implementation: DSL, cursors, 404 auto-heal,
│   │   │   │                      external version guard, bulk, reindex/switch-alias
│   │   │   └── *_test.go          integration-style tests against httptest OpenSearch stubs
│   │   ├── kafka/
│   │   │   ├── consumer.go        ConsumerManager: one consumer per topic, group "<group>-<topic>"
│   │   │   ├── handlers.go        HandleEvent registry: 18 event types -> indexing service
│   │   │   └── *_test.go
│   │   ├── rabbitmq/
│   │   │   ├── consumer.go        4 bindings (orders/media/users/drivers exchanges) -> queue.search_index
│   │   │   ├── index_handler.go   HandleIndexEvent: same 18 event types, retry classification
│   │   │   └── *_test.go
│   │   └── redis/
│   │       ├── cache.go           Get/Set (JSON), GenerateKey (sha256), Ping
│   │       └── *_test.go
│   ├── interfaces/
│   │   ├── graphql/
│   │   │   ├── schema.go          Apollo Federation 2.3 SDL + graphql-go schema construction
│   │   │   ├── resolvers.go       9 resolvers: auth context, role scoping, i18n, input building
│   │   │   ├── server.go          HTTP handler: request parsing, context propagation, writeJSON
│   │   │   └── *_test.go
│   │   └── health/
│   │       ├── handler.go         LivenessHandler, ReadinessHandler (OpenSearch + Redis, 3 s timeout)
│   │       └── handler_test.go
│   ├── observability/
│   │   ├── tracing.go             lazy OTLP/gRPC tracer init; nil-safe no-op when endpoint empty
│   │   └── tracing_test.go        (metrics come from the shared packages/go/metrics library)
│   ├── i18n/
│   │   ├── i18n.go                translation tables (en, ar), T(lang, key), FromContext
│   │   └── i18n_test.go
│   └── testutil/
│       ├── fake_repository.go     in-memory SearchRepository for unit tests
│       ├── fake_opensearch.go     httptest server speaking OpenSearch REST for repository tests
│       └── testutil_test.go
└── scripts/checktool/
    └── main.go                    "merge" coverage profiles, "fmt" check, "clean" artifacts
```

## Package Reference

### `cmd/server`

The only `main` package. `run(ctx)` performs the full startup sequence described in Architecture and returns an error instead of calling `os.Exit`, which makes it directly testable. `buildHandler` mounts the HTTP routes:

| Route                | Handler                          |
|----------------------|----------------------------------|
| `/health/live`       | liveness probe                   |
| `/health/ready`      | readiness probe (OS + Redis)     |
| `/healthz`           | plain liveness alias             |
| `/search/graphql`    | GraphQL endpoint (canonical)     |
| `/graphql`           | GraphQL endpoint (alias)         |

`main_test.go` contains a `TestMain` that allocates a shared `GOCOVERDIR` directory. Because `go test` runs `cmd/server` in a subprocess when full end-to-end coverage is measured, `TestMain` converts that counter data to a text profile after the tests pass and prints `SUBPROCESS_COVERAGE_FILE=<path>` (or writes to `SUBPROCESS_COV_OUT` when set by the Makefile). This is how `main()` and `run()` are covered end to end.

### `internal/config`

`config.Load()` reads the environment once, validates `OPENSEARCH_URL` (every comma-separated entry must parse as a URL), clamps numeric knobs, and returns an immutable `Config`. All defaults target the docker compose network hostnames (`opensearch-srv`, `kafka-srv`, `redis-srv`, `nats-srv`). See [Configuration Reference](#configuration-reference) for the full table.

### `internal/domain/search`

Pure domain model with zero infrastructure imports.

- **Documents** — `DeliveryDocument` (delivery_id, customer_id, optional driver_id, status, pickup/dropoff `GeoAddress` with `GeoPoint`, created_at, updated_at, indexed_at, source_version), `DriverDocument` (driver_id, name, status, vehicle_type, rating, optional location, updated_at, indexed_at, source_version), `MediaDocument` (media_id, owner_id, file_name, mime_type, media_type, status, size, created_at), `UserDocument` (id, first_name, last_name, email, role, is_active, created_at). The federated `Driver { id }` entity reference exists only at the GraphQL layer.
- **Queries** — typed structs per domain in `result.go` plus `AutocompleteQuery` (prefix + target index + limit) and `GeoSearchQuery` (lat, lon, radius, filters, pagination).
- **Results** — generic `SearchResult[T]` with `PageInfo{HasNextPage, Cursor, Total}`; `AutocompleteResult` for suggestions. Cursors are base64-encoded JSON carrying OpenSearch `search_after` sort values (`{"sv": [...]}`), validated by `DecodeCursor`.
- **`SearchRepository` interface** — `SearchDeliveries`, `SearchDrivers`, `SearchMedia`, `SearchUsers`, `Autocomplete`, `NearbyDeliveries`, `NearbyDrivers`, `UpsertDelivery`, `UpsertDriver`, `UpsertMedia`, `UpsertUser`, `DeleteDelivery`, `DeleteDriver`, `DeleteMedia`, `DeleteUser`, `BulkUpsertDeliveries`, `BulkUpsertDrivers`, `BulkUpsertMedia`, `GetDocumentVersion`, `IndexExists`, `CountDocuments`, `Reindex`, `SwitchAlias`, `Ping`, `ClusterHealth`.
- **Sentinel errors** — `ErrInvalidQuery`, `ErrInvalidCursor`, `ErrIndexNotFound`, `ErrDocumentNotFound`, `ErrVersionConflict`, `ErrSearchUnavailable`, `ErrUnauthorized`, `ErrReindexInProgress` (as defined in `errors.go`). Application and infrastructure layers wrap these so resolvers can map them to stable i18n messages.

### `internal/application/search`

`Service` wraps every read operation in cache-aside logic:

1. Build a Redis key from the query (`redis.Cache.GenerateKey(domain, query)` — sha256 over the canonical query JSON).
2. On cache hit, deserialize and return.
3. Otherwise query the repository. Cache the result **only when `PageInfo.Total > 0`** for **60 seconds** — empty result sets are not cached, so newly indexed documents become searchable immediately.
4. Autocomplete results are always cached for **300 seconds**.
5. Geo queries (`NearbyDeliveries`, `NearbyDrivers`) bypass the cache entirely because radius/coordinate combinations have low hit rates.

Cache failures are deliberately non-fatal: `Get` returns false and `Set` errors are ignored, so Redis being down degrades to direct OpenSearch queries.

### `internal/application/indexing`

The single write path used by both Kafka and RabbitMQ handlers:

- `UpsertDelivery`, `UpsertDriver`, `UpsertMedia`, `UpsertUser` — map event payloads to domain documents and delegate to the repository (which applies the external version guard).
- `UpsertDriver` enriches the driver name from the users index when the payload carries a user reference, keeping driver documents denormalized.
- `Delete*` — tombstone deletes keyed by document id.
- `BulkUpsertDeliveries`, `BulkUpsertDrivers`, `BulkUpsertMedia` — batched indexing used by reindexing and high-volume event bursts; the repository chunks bulk requests and applies per-item error classification (retryable vs permanent).

### `internal/application/reindex`

`StartReindex(ctx, index)` rejects an empty/whitespace index name (`"index name is required"`), allocates a job id of the form `reindex-<index>-<unix-seconds>`, records a `ReindexJob{JobID, Index, Status: "IN_PROGRESS", StartedAt}`, and launches a background goroutine that calls the repository `Reindex` API against the versioned concrete index (`<index>-v1`). Job completion/failure is logged; the `SearchRepository` interface also exposes `SwitchAlias` so callers can build full rebuild-then-repoint flows on top. The GraphQL mutation is admin-only and returns the job snapshot immediately (`jobId`, `index`, `status`, `startedAt`, `completedAt`, `error`).

### `internal/infrastructure/opensearch`

- **`Client`** — thin wrapper around `opensearchapi` request types; centralizes address list, basic auth, and `OPENSEARCH_INSECURE` TLS skip-verify. All repository and manager operations go through `Do(ctx, req, opts)`.
- **`IndexManager.EnsureIndices`** — idempotently ensures four concrete indices with aliases:

  | Alias       | Concrete index | Mapping       |
  |-------------|----------------|---------------|
  | `deliveries`| `deliveries-v1`| delivery mapping with geo_point pickup/dropoff |
  | `drivers`   | `drivers-v1`   | driver mapping with geo_point location        |
  | `media`     | `media-v1`     | media mapping                                   |
  | `users`     | `users-v1`     | user mapping (keyword email/role, boolean is_active) |

  For each entry it checks existence, creates the index with its mapping if missing, then adds the alias. Errors are collected per index and returned as one aggregate error.
- **`SearchRepository`** — builds OpenSearch Query DSL in code (bool queries for filters, `multi_match` for text, `geo_distance` for nearby queries, `search_after`-style cursor pagination). Notable behaviors:
  - **Cursor pagination** — the cursor is base64-encoded JSON holding the OpenSearch `search_after` sort values; invalid cursors are rejected with `ErrInvalidCursor`. Page size is clamped to `SEARCH_MAX_PAGE_SIZE`, query text to `SEARCH_MAX_QUERY_LENGTH`.
  - **External version guard** — upserts use `version_type=external_gte` with the event's `source_version`, so out-of-order events cannot roll a document back; conflicts are classified as `ErrVersionConflict` and logged, not retried.
  - **404 auto-heal** — if an operation fails because the concrete index disappeared (mid-reindex window, manual deletion), the repository recreates the index from the mapping and retries the operation once.
  - **Bulk** — splits large batches, parses `errors` item-by-item, retries retryable failures, and returns classified permanent failures.

### `internal/infrastructure/kafka`

`ConsumerManager.Start(ctx)` launches one `franz-go`-style consumer per topic (18 topics, see [Event-Driven Indexing](#event-driven-indexing)) with consumer group `"<KAFKA_GROUP_ID_SEARCH>-<topic>"`, so lag and rebalances are isolated per topic. Each consumed message is unmarshalled into the shared `EventEnvelope` and dispatched through `HandleEvent`, which switches on `EventType` and calls the indexing service. `HandleEvent` never returns an error for unknown types (logged and skipped); unmarshal failures are permanent (no retry). The manager's `Start` returns nil after all goroutines are spawned; shutdown is driven by context cancellation plus `Close`.

### `internal/infrastructure/rabbitmq`

Uses the monorepo `packages/go/rabbitmq` wrapper. A single consumer declares queue `queue.search_index` and binds it to four exchanges from `packages/go/constants`:

| Exchange            | Domain events forwarded                      |
|---------------------|----------------------------------------------|
| `exchange.orders`   | all `delivery.*` events                      |
| `exchange.media`    | `media.upload.created`, `media.upload.completed`, `media.ready`, `media.deleted` |
| `exchange.users`    | `user.created`, `user.updated`, `user.deleted` |
| `exchange.drivers`  | `driver.created`, `driver.updated`, `driver.deleted` |

`HandleIndexEvent` mirrors the Kafka handler: same 18 event types, same payload mapping into `indexing.Service`. Errors are wrapped with the shared `ErrPermanent` / retryable markers so the RabbitMQ wrapper applies the configured retry policy (`RABBITMQ_MAX_ATTEMPTS`, `RABBITMQ_BASE_DELAY_MS`). When `RABBITMQ_URL` is empty the consumer is skipped entirely (Kafka alone keeps the indices fresh).

### `internal/infrastructure/redis`

`Cache` wraps a `go-redis` client:

- `Get(ctx, key, dst)` — reads JSON and unmarshals into the destination; any failure returns false (treated as miss).
- `Set(ctx, key, val, ttl)` — marshals and writes with TTL; errors are returned to the caller but the application layer ignores them.
- `GenerateKey(domain, query)` — `sha256` over a canonical serialization of domain + query, producing compact, collision-resistant keys.
- `Ping(ctx)` — used by the readiness probe.

### `internal/interfaces/graphql`

- **`schema.go`** — embeds the Apollo Federation v2.3 SDL (`@link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])`) as a string served through the `_service { sdl }` field, and builds the executable graphql-go schema. `Driver` is an entity (`@key(fields: "id")`) so the router can federate driver references; `PaginationInfo` is `@shareable`.
- **`resolvers.go`** — nine resolvers (`ResolveSearchDeliveries`, `ResolveSearchDrivers`, `ResolveSearchMedia`, `ResolveSearchUsers`, `ResolveAutocomplete`, `ResolveNearbyDeliveries`, `ResolveNearbyDrivers`, `ResolveSearchHealth`, `ResolveStartReindex`). Every resolver:
  1. Resolves the authenticated user id from the shared logging context or the `Authorization` header via `sharedauth.Authenticate`; unauthenticated requests receive the i18n `error.unauthorized` message.
  2. Reads the role from the `x-user-role` context key (set by the router / API gateway).
  3. Validates and maps GraphQL `input` into domain query structs, applying role scoping (for example, customers only see their own deliveries) and clamping pagination.
  4. Delegates to the application service and wraps the result in the response envelope with a localized success message.
- **`server.go`** — `Handler()` returns an `http.HandlerFunc` that accepts POST JSON (`query`, `variables`, `operationName`), propagates `authorization` and `x-user-role` headers plus the i18n language into `context.Context`, executes the operation, and serializes GraphQL-compliant responses (errors array with localized messages). Malformed JSON and method-not-allowed cases return proper HTTP status codes without panicking. `writeJSON` is defensive: encode errors are logged rather than allowed to panic the server.

### `internal/interfaces/health`

- `LivenessHandler` — always `200 {"status":"UP"}` (process is alive).
- `ReadinessHandler` — pings OpenSearch and Redis with a 3-second timeout; returns `200` only when both are reachable, otherwise `503` with the failing dependency named in the body.

### `internal/observability`

- `tracing.go` — `InitTracer(cfg)` lazily builds an OTLP/gRPC exporter when `OTEL_ENDPOINT` is non-empty, applies `OTEL_SAMPLING_RATE` (clamped to [0, 1]), and registers `otelhttp`-style instrumentation. When the endpoint is empty it returns a nil-safe no-op tracer; all call sites use it without nil checks because the returned `*trace.TracerProvider` wrapper absorbs the no-op case.
- Metrics are provided by the shared `packages/go/metrics` library: `metrics.StartMetricsServer(cfg.PortMetrics)` serves Prometheus-format metrics on `PORT_SEARCH_METRICS` (default `9103`), and `metrics.HTTPMetricsMiddleware` wraps the GraphQL handler so every request is instrumented. A failing metrics listener is logged and does not take down the main server.

### `internal/i18n`

Translation tables for `en` and `ar`. `T(lang, key)` falls back to English (then to the raw key) when a translation is missing. `FromContext(ctx)` extracts the language set by the GraphQL handler from the `Accept-Language` header (default `en`). Used for every user-facing error and success message in the resolvers and health payloads.

### `internal/testutil`

- `fake_repository.go` — in-memory `SearchRepository` with programmable results and recorded calls; used by application-service and resolver unit tests.
- `fake_opensearch.go` — `httptest.Server` implementations that speak enough of the OpenSearch REST API (search, index, bulk, exists, create, aliases, reindex) for repository tests to run without a real cluster. Includes helpers for closed listeners (connection errors), hanging servers (deadline behavior), and per-request scripting.

### `scripts/checktool`

Small cross-platform Go utility invoked by the Makefile so recipes stay free of shell-specific syntax:

- `checktool merge <base> <extra> <out>` — merges two coverage profiles (accepts `count` and `set` modes, e.g. `covdata textfmt` output) taking the per-block maximum so overlapping subprocess coverage never lowers the base.
- `checktool fmt [paths...]` — wraps `gofmt -l` and fails when any file needs formatting.
- `checktool clean [paths...]` — removes build and coverage artifacts (`bin/`, `*.out`, `coverage.html`).

## GraphQL API

### Endpoints

| Path              | Port  | Notes                                        |
|-------------------|-------|----------------------------------------------|
| `/search/graphql` | 4007  | canonical endpoint                           |
| `/graphql`        | 4007  | alias for clients that assume the root path  |

Both accept `POST` with a JSON body:

```json
{ "query": "...", "variables": { ... }, "operationName": "..." }
```

### Authentication and context

| Header            | Purpose                                                     |
|-------------------|-------------------------------------------------------------|
| `Authorization`   | JWT bearer token; validated via `packages/go/auth`          |
| `x-user-role`     | role injected by the gateway (`customer`, `driver`, `admin`)|
| `Accept-Language` | `en` (default) or `ar` for localized messages               |

### Queries

**searchDeliveries** — full-text + filter search over the deliveries index.

```graphql
query SearchDeliveries($input: DeliverySearchInput!) {
  searchDeliveries(input: $input) {
    success
    statusCode
    message
    timeStamp
    data {
      paginationInfo { totalItems currentPage nextPage }
      items {
        delivery_id
        customer_id
        driver_id
        status
        pickup { city country location { lat lon } }
        dropoff { city country location { lat lon } }
        created_at
        updated_at
        source_version
      }
    }
  }
}
```

Variables example:

```json
{
  "input": {
    "query": "downtown",
    "status": "in_transit",
    "customerId": "user-123",
    "page": 1,
    "pageSize": 20
  }
}
```

**searchDrivers** — driver documents with optional filters (status, vehicle_type, rating range) and the federated `driver { id }` entity reference.

**searchMedia** / **searchUsers** — analogous list queries over the media and users indices (users restricted to admin-role callers).

**autocomplete** — prefix suggestions across the searchable name fields; input is `{ "query": "ali", "limit": 10 }`. Results are cached for 300 seconds.

**nearbyDeliveries** / **searchNearbyDrivers** — geo distance queries; input is `{ "lat": 30.0444, "lon": 31.2357, "radiusKm": 5, ... }`. Backed by `geo_point` fields and `geo_distance` sorting; not cached.

**searchHealth** — lightweight health snapshot returning `status: "UP"` plus a timestamp (localized message); for dependency-level checks use the `/health/ready` HTTP endpoint.

### Mutations

**startReindex** — admin-only; kicks off an asynchronous reindex of one index through the OpenSearch Reindex API:

```graphql
mutation {
  startReindex(index: "deliveries") {
    success
    statusCode
    message
    data {
      jobId
      index
      status
      startedAt
      completedAt
      error
    }
  }
}
```

The `index` argument targets the versioned concrete index `<index>-v1` (for example `deliveries-v1`); an empty name is rejected with a validation error, and non-admin callers receive the i18n forbidden message. The job runs in the background and the mutation returns the job snapshot (`status: "IN_PROGRESS"`) immediately.

### Federation

The subgraph advertises Federation v2.3 via `_service { sdl }`. The `Driver` type is an entity keyed on `id`, allowing the Apollo router to resolve driver references contributed by other subgraphs while this service supplies search-specific fields (name, rating, location, status).

### Error shape

Resolver errors surface in the standard GraphQL `errors` array with localized messages, for example:

```json
{
  "errors": [ { "message": "غير مصرح لك بالوصول" } ]
}
```

## Event-Driven Indexing

The service consumes the same 18 event types from two transports; both handlers funnel into `application/indexing`.

| Event type                 | Index       | Action                                           |
|----------------------------|-------------|--------------------------------------------------|
| `delivery.created`         | deliveries  | upsert full document (addresses, status, version)|
| `delivery.driver.assigned` | deliveries  | upsert (driver_id + status + version)            |
| `delivery.driver.accepted` | deliveries  | upsert (status + version)                        |
| `delivery.picked_up`       | deliveries  | upsert (status + version)                        |
| `delivery.in_transit`      | deliveries  | upsert (status + version)                        |
| `delivery.completed`       | deliveries  | upsert (status + version)                        |
| `delivery.cancelled`       | deliveries  | upsert (status + version)                        |
| `delivery.deleted`         | deliveries  | delete document                                  |
| `driver.created`           | drivers     | upsert document                                  |
| `driver.updated`           | drivers     | upsert document                                  |
| `driver.deleted`           | drivers     | delete document                                  |
| `media.upload.created`     | media       | upsert (pending status)                          |
| `media.upload.completed`   | media       | upsert (processing status)                       |
| `media.ready`              | media       | upsert (ready status, full metadata)             |
| `media.deleted`            | media       | delete document                                  |
| `user.created`             | users       | upsert document                                  |
| `user.updated`             | users       | upsert document                                  |
| `user.deleted`             | users       | delete document                                  |

Guarantees:

- **Ordering** — every payload carries `source_version`; the repository upserts with `external_gte` so an older event arriving after a newer one is dropped as a version conflict instead of corrupting the document.
- **Retry classification** — transient failures (connection, 5xx) are retried with backoff; permanent failures (unmarshal, validation) are logged and skipped (RabbitMQ: `ErrPermanent` stops retries).
- **Unknown events** — logged and ignored, so new platform events never crash the consumer.
- **At-least-once** — duplicates are harmless because upserts are idempotent per `(id, source_version)`.

Kafka topics mirror the table above; the RabbitMQ bindings map the `delivery.*` events onto `exchange.orders`, and media/user/driver events onto their respective exchanges, all landing on the shared `queue.search_index`.

## OpenSearch Indices

Mappings (defined as constants in `index_manager.go`):

- **deliveries-v1** — `delivery_id` (keyword), `customer_id` (keyword), `driver_id` (keyword, nullable), `status` (keyword), `pickup`/`dropoff` (object with `city`/`country` text + `location` geo_point), `created_at`/`updated_at` (date), `source_version` (long).
- **drivers-v1** — `driver_id` (keyword), `name` (text + keyword subfield), `status` (keyword), `vehicle_type` (keyword), `rating` (float), `location` (geo_point, nullable), `updated_at` (date), `source_version` (long).
- **users-v1** — `id` (keyword), `first_name`/`last_name` (text), `email` (keyword), `role` (keyword), `is_active` (boolean), `created_at` (date).
- **media-v1** — `media_id` (keyword), `owner_id` (keyword), `file_name` (text), `mime_type`/`media_type` (keyword), `status` (keyword), `size` (long), `created_at` (date).

All reads and writes go through the alias (`deliveries`, `drivers`, `media`, `users`), so `startReindex` can build `deliveries-v2`, copy documents, and atomically repoint the alias with zero query downtime.

## Caching

| Operation                          | Cache     | TTL  | Notes                                    |
|------------------------------------|-----------|------|------------------------------------------|
| searchDeliveries/Drivers/Media/Users | Redis   | 60 s | only cached when `Total > 0`             |
| autocomplete                       | Redis     | 300 s| always cached                            |
| nearbyDeliveries / nearbyDrivers   | none      | -    | geo queries bypass cache                 |
| indexing writes                    | none      | -    | new data is visible on next uncached read |

Keys are `sha256(domain + canonical query JSON)` — deterministic across processes and immune to field-order differences. All cache failures degrade gracefully to direct OpenSearch queries.

## Health, Metrics, and Tracing

| Endpoint                    | Port  | Behavior                                          |
|-----------------------------|-------|---------------------------------------------------|
| `GET /health/live`          | 4007  | `200` while the process runs                      |
| `GET /health/ready`         | 4007  | `200` when OpenSearch **and** Redis respond within 3 s, else `503` |
| `GET /healthz`              | 4007  | alias of liveness                                 |
| `GET /search/graphql`       | 4007  | GraphQL (POST)                                    |
| `GET :9103/metrics`         | 9103  | Prometheus-format metrics                         |

Tracing exports OTLP/gRPC to `OTEL_ENDPOINT` (for example an otel-collector forwarding to Tempo). When unset, tracing is a no-op with zero overhead. `OTEL_SAMPLING_RATE` controls the parent-based sample ratio.

## Configuration Reference

All configuration is environment variables; values below are the defaults.

| Variable                          | Default                   | Description                                             |
|-----------------------------------|---------------------------|---------------------------------------------------------|
| `APP_ENV` / `NODE_ENV`            | `development`             | environment name                                        |
| `PORT_SEARCH_GRAPHQL`             | `4007`                    | GraphQL HTTP port                                       |
| `PORT_SEARCH_METRICS`             | `9103`                    | metrics HTTP port                                       |
| `LOG_LEVEL`                       | `info`                    | slog level                                              |
| `OTEL_ENDPOINT`                   | _(empty)_                 | OTLP/gRPC endpoint; empty disables tracing              |
| `OTEL_SAMPLING_RATE`              | `1.0`                     | sample ratio, clamped to [0, 1]                         |
| `OPENSEARCH_URL`                  | `http://opensearch-srv:9200` | comma-separated URLs; each validated at startup       |
| `OPENSEARCH_USERNAME`             | `admin`                   | basic-auth user                                         |
| `OPENSEARCH_PASSWORD`             | `admin`                   | basic-auth password                                     |
| `OPENSEARCH_INSECURE`             | `true`                    | skip TLS certificate verification                       |
| `KAFKA_BROKERS`                   | `kafka-srv:9092`          | comma-separated broker list                            |
| `KAFKA_GROUP_ID_SEARCH`           | `search-service`          | consumer group prefix (`<prefix>-<topic>`)              |
| `REDIS_HOST`                      | `redis-srv`               | Redis host                                              |
| `REDIS_PORT`                      | `6379`                    | Redis port                                              |
| `REDIS_PASSWORD`                  | _(empty)_                 | Redis password                                          |
| `REDIS_DB`                        | `0`                       | Redis database number                                   |
| `SEARCH_CACHE_TTL`                | `120`                     | Redis TTL seconds (service read path uses 60 s / 300 s constants) |
| `NATS_URL`                        | `nats://nats-srv:4222`    | NATS URL (reserved for future use)                      |
| `RABBITMQ_URL`                    | _(empty)_                 | AMQP URL; empty skips RabbitMQ consumers                |
| `RABBITMQ_MAX_ATTEMPTS`           | `15`                      | consumer retry attempts, min 1                          |
| `RABBITMQ_BASE_DELAY_MS`          | `1000`                    | retry backoff base in ms, min 0                         |
| `SEARCH_MAX_PAGE_SIZE`            | `100`                     | pagination clamp, min 1                                 |
| `SEARCH_MAX_QUERY_LENGTH`         | `500`                     | query text clamp, min 1                                 |
| `SEARCH_INDEX_INIT_MAX_RETRIES`   | `30`                      | startup retries waiting for OpenSearch, min 1           |
| `SEARCH_INDEX_INIT_RETRY_DELAY_MS`| `3000`                    | delay between startup retries, min 0                    |

## Running Locally

### Prerequisites

- Go 1.25+
- Docker (for the dependency stack and image builds)
- Optional: a C compiler for `make test-race`

### With docker compose (recommended)

```bash
cd services/search-service
make docker-up      # starts redis, kafka, nats, opensearch, and the service
make run            # or run the binary on the host: go run ./cmd/server
```

If a host port is already taken (for example another local Redis on 6379), override it: `HOST_PORT_REDIS=16379 make docker-up` (see the port table below).

`docker-compose.yml` defines the local dependencies with stable service/container names matching the default hostnames (`redis-srv`, `kafka-srv`, `nats-srv`, `opensearch-srv`) and also includes a `search-service` app service (container `search-service-local`) that builds and runs this binary against the stack, plus a persistent OpenSearch data volume.

Host ports are overridable via environment variables (defaults in parentheses) so the stack can coexist with other local services:

| Variable                       | Default | Maps to container port |
|--------------------------------|---------|------------------------|
| `HOST_PORT_REDIS`              | `6379`  | redis 6379             |
| `HOST_PORT_KAFKA`              | `9092`  | kafka 9092             |
| `HOST_PORT_NATS`               | `4222`  | nats 4222              |
| `HOST_PORT_NATS_MONITOR`       | `8222`  | nats monitor 8222      |
| `HOST_PORT_OPENSEARCH`         | `9200`  | opensearch 9200        |
| `HOST_PORT_OPENSEARCH_STATS`   | `9600`  | opensearch stats 9600  |
| `HOST_PORT_GRAPHQL`            | `4007`  | GraphQL 4007           |
| `HOST_PORT_METRICS`            | `9103`  | metrics 9103           |

Example when another Redis already occupies 6379:

```bash
HOST_PORT_REDIS=16379 make docker-up
```

Compose notes (verified end to end — stack boots, `/health/live` and `/health/ready` return 200, a GraphQL `searchHealth` query succeeds, and `/metrics` serves Prometheus output):

- The OpenSearch 2.x demo installer requires `OPENSEARCH_INITIAL_ADMIN_PASSWORD` even with `plugins.security.disabled=true`; the compose file sets a strong local-development password.
- The compose file sets `RABBITMQ_URL=""` for the service container because this stack has no RabbitMQ — the service skips those consumers (Kafka alone keeps the indices fresh) instead of spending minutes in dial retries. Remove that override (and add a RabbitMQ service) to exercise the dual-consumption path.

### Against your own infrastructure

Set the environment variables from the configuration table (at minimum `OPENSEARCH_URL` if your cluster is not on the compose network), then:

```bash
make run
```

`OPENSEARCH_URL` is validated at startup; an invalid URL fails fast with `invalid OPENSEARCH_URL "..."`.

### Docker image

```bash
make docker-build
# equivalent to:
# docker build -f Dockerfile -t search-service:latest ../..
```

The Dockerfile is a multi-stage build: `golang:1.25-alpine` compiles the server from the monorepo root (needed because of the `packages/go` replace directive), then a runtime stage based on `alpine:3.20` exposes ports `4007` and `9103`.

## Makefile

Run from `services/search-service`. `make help` lists everything.

| Target            | Command summary                                                       |
|-------------------|-----------------------------------------------------------------------|
| `help`            | print available targets (default goal)                                 |
| `tidy`            | `go mod tidy`                                                         |
| `download`        | `go mod download`                                                     |
| `mod-verify`      | `go mod verify`                                                       |
| `fmt`             | `gofmt -w .`                                                          |
| `fmt-check`       | fail if `gofmt -l` reports files                                      |
| `vet`             | `go vet ./...`                                                        |
| `build`           | compile `bin/search-service` from `./cmd/server`                      |
| `run`             | `go run ./cmd/server`                                                 |
| `test`            | `go test ./... -count=1`                                              |
| `test-v`          | same, verbose                                                         |
| `test-race`       | `go test ./... -race` (requires gcc)                                  |
| `coverage`        | tests with `-covermode=count`, merges subprocess GOCOVERDIR data, prints per-function totals |
| `coverage-html`   | renders `coverage.html` from the merged profile                       |
| `ci`              | `fmt-check` + `vet` + `test` + `coverage`                             |
| `clean`           | removes `bin/`, `coverprofile.out`, `subprocess.out`, `merged.out`, `coverage.html` |
| `docker-build`    | build the service image                                               |
| `docker-up`       | `docker compose up -d --build`                                        |
| `docker-down`     | `docker compose down`                                                 |

Coverage artifacts are written to the service root and are gitignored (`*.out` via the monorepo `.gitignore`, plus `coverage.html` / `bin/` locally).

## Testing

### Strategy

The suite combines three layers, all hermetic (no external cluster, broker, or Redis required):

1. **Pure unit tests** — config parsing/clamps, i18n fallbacks, domain validation, cache key generation, event payload mapping (fake repository).
2. **Integration-style adapter tests** — `httptest` servers speaking the OpenSearch REST protocol exercise the real repository and index manager, including transport failures (closed servers, hanging listeners, non-2xx responses, malformed bodies).
3. **End-to-end composition tests** — `cmd/server` tests call `run()` against fake OpenSearch/Redis/miniredis, boot the real HTTP mux, and assert routes, GraphQL execution, startup retries, metrics-server errors, and graceful shutdown. `TestMain` merges subprocess coverage so `main()` is measured too.

Notable fixtures and techniques:

- `testutil.NewFakeRepository` — programmable in-memory `SearchRepository`.
- `testutil` OpenSearch stubs — per-request scripting; special fixtures for closed TCP listeners (`newClosedOSFixture`) and hanging servers (deadline verification).
- `miniredis` — in-process Redis for cache hit/miss/TTL tests.
- Invalid ports (for example `99999`) to force `ListenAndServe` failures deterministically (Windows `SO_REUSEADDR` means a bound port does not fail a second listener, so invalid ports are used instead of pre-binding).
- Small `RABBITMQ_MAX_ATTEMPTS` / `RABBITMQ_BASE_DELAY_MS` and `SEARCH_INDEX_INIT_*` values keep retry-path tests fast.
- `OTEL_ENDPOINT=127.0.0.1:4317` in graceful-shutdown tests to cover the tracer-enabled path without an collector.

Commands:

```bash
make test           # full suite, ~2-3 minutes (retry-path tests dominate)
make test-v         # verbose
make test-race      # with race detector (needs gcc)
make coverage       # suite + merged coverage report
make coverage-html  # open coverage.html in a browser
```

### Coverage

The suite currently reports **96.6% statement coverage** on the merged profile (in-process packages plus `cmd/server` subprocess `GOCOVERDIR` data). Per-package highlights:

| Package                              | Coverage |
|--------------------------------------|----------|
| config, i18n, domain/search, redis, rabbitmq, health, testutil, application/* | 100% |
| scripts/checktool                    | 80.0%    |
| kafka                                | 99.3%    |
| graphql                              | 99.3%    |
| opensearch                           | 98.2%    |
| observability                        | 90.5%    |
| cmd/server (after merge)             | main 100%, runMain 100%, run 85.4%, buildHandler 100% |

The remaining gaps are the documented dead branches (see below) plus `checktool`'s CLI dispatch (`main`/`usage`, which terminate the process).

### How the coverage merge works

`go test -coverprofile` cannot see code executed in a separately-launched process. The `cmd/server` tests measure `main()`/`run()` by launching the real server lifecycle in-process with `GOCOVERDIR` pointed at a shared directory. `TestMain` then converts those counters with `go tool covdata textfmt` and (under `make coverage`) writes them to a deterministic path via the `SUBPROCESS_COV_OUT` environment variable. Finally `scripts/checktool merge` folds the two count-mode profiles together block-by-block using the maximum hit count, so overlapping coverage never lowers totals:

```bash
go test ./... -count=1 -covermode=count -coverprofile=coverprofile.out
go run ./scripts/checktool merge coverprofile.out "<service-root>/subprocess.out" merged.out
go tool cover -func=merged.out
```

### Documented dead branches (accepted, not bugs)

These branches exist for defensive symmetry but cannot currently be reached in tests; they are deliberately excluded from the "must cover" set and documented here:

- `main.go` success arms that require live Kafka, RabbitMQ, and OpenSearch clusters (consumer-start success paths beyond the warn-and-continue paths already covered).
- `observability` exporter-construction error arms that require an unreachable OTLP handshake failure at init time.

Everything else is covered, including error arms previously considered unreachable (OpenSearch transport errors on exists/create/alias/upsert/bulk/search, Redis client failures, GraphQL encode failures, resolver non-document sources, index-init retry exhaustion, metrics-server listen failure, RabbitMQ dial failure).

## Design Notes and Troubleshooting

- **Windows `SO_REUSEADDR`** — on Windows a process can bind the same port twice without error, so tests that need a failing listener use an invalid port (`99999`) rather than pre-occupying the port.
- **Docker port conflicts** — `make docker-up` publishes host ports that may already be in use by other projects (6379/9092/4222/9200 are common). Override with the `HOST_PORT_*` variables documented in [Running Locally](#running-locally); container-to-container traffic always uses the internal hostnames and ports, so overrides never affect the service itself.
- **RabbitMQ slow start** — with defaults (15 attempts, 1 s base delay) an unreachable broker delays startup by up to ~5 minutes of backoff. For local development set `RABBITMQ_MAX_ATTEMPTS=2` and `RABBITMQ_BASE_DELAY_MS=100`, or leave `RABBITMQ_URL` empty to skip RabbitMQ entirely (Kafka continues to feed the indices).
- **Index bootstrap** — the service does not serve queries or start consumers until `EnsureIndices` succeeds (up to `SEARCH_INDEX_INIT_MAX_RETRIES` attempts, `SEARCH_INDEX_INIT_RETRY_DELAY_MS` apart). Set both low in tests or dev loops.
- **`OPENSEARCH_URL` validation** — the value must be one or more valid absolute URLs; typos fail fast at startup rather than at the first query.
- **Empty results are never cached** — newly indexed documents become visible immediately; only non-empty result pages pay the 60 s staleness window.
- **Version guard semantics** — `source_version` must be monotonically increasing per document (the producing services use their datastore update counters). Events with a lower or equal version are dropped, making at-least-once redelivery safe.
- **Module replace** — `go.mod` replaces `packages/go` with `../../packages/go`, so standalone builds must run from the monorepo (the Dockerfile uses the monorepo root as build context for the same reason).
- **Federation** — this is a subgraph; wire it into the Apollo router with the `_service { sdl }` endpoint. Direct queries to `/search/graphql` work without a router for development.
- **i18n** — send `Accept-Language: ar` to receive Arabic messages; unknown languages fall back to English.
