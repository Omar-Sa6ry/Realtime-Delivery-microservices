# Analytics Service

Real-time analytics microservice for the Delivery platform. It consumes 34 domain event topics from Kafka, validates, de-duplicates and transforms them into typed ClickHouse fact tables, and serves the resulting metrics through an Apollo Federation v2.3 GraphQL subgraph, an internal gRPC API and a NATS live ticker. Reconciliation, data-quality scoring and dead-letter handling run continuously in the background.

---

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Repository Layout](#repository-layout)
- [Package Reference](#package-reference)
- [Event Ingestion](#event-ingestion)
- [ClickHouse Schema](#clickhouse-schema)
- [GraphQL API](#graphql-api)
- [gRPC API](#grpc-api)
- [NATS Live Ticker](#nats-live-ticker)
- [Caching](#caching)
- [Background Workers](#background-workers)
- [Health, Metrics, and Tracing](#health-metrics-and-tracing)
- [Configuration Reference](#configuration-reference)
- [Running Locally](#running-locally)
- [Makefile](#makefile)
- [Testing](#testing)
- [Data Quality and Error Handling](#data-quality-and-error-handling)
- [Design Notes and Troubleshooting](#design-notes-and-troubleshooting)

---

## Overview

The analytics service is one microservice inside a larger real-time delivery platform (orders, drivers, payments, notifications, media, search, realtime). Producer services publish domain events to Kafka; this service consumes them, keeps an append-only raw landing zone plus denormalized fact tables in ClickHouse, and answers analytical questions about deliveries, drivers and payments with sub-second latency through a single GraphQL endpoint.

Key properties:

- **Language / runtime**: Go 1.25, module `github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service`.
- **Storage**: ClickHouse (clickhouse-go v2), database `analytics`, eight tables created idempotently at startup.
- **API**: GraphQL (Apollo Federation v2.3 subgraph) on port `4009`, paths `/graphql` and `/analytics/graphql`; gRPC service `analytics.AnalyticsService` on port `50057`; Prometheus metrics on port `9107`.
- **Event ingestion**: one Kafka consumer group (`analytics-service`) with 34 topic readers, exponential-backoff retries, and a dead-letter topic (`analytics.dlq`).
- **Caching**: Redis cache-aside layer in front of every analytics query (`analytics:` key namespace).
- **Background work**: a worker pool hosting a reconciliation pass (default every 300 s) and a 10-second NATS live ticker.
- **Observability**: 15 Prometheus metrics, OpenTelemetry OTLP/gRPC tracing (optional), structured `slog` logging, i18n (English/Arabic) messages.
- **Shared code**: reuses the monorepo `packages/go` libraries for logging, metrics middleware, HTTP/gRPC tracing middleware, Kafka topic helpers and alerting (wired through a `replace` directive in `go.mod`).

The service is built around Clean Architecture: `internal/domain` and `internal/application` import no infrastructure, `internal/ports` declares the outbound interfaces, `internal/adapters` implements them, and `internal/graphql` is the only inbound transport. Composition happens exclusively in `cmd/server/main.go`.

## Features

- Consumes 34 Kafka topics covering delivery lifecycle, driver availability and assignment, payment authorization/capture/refund and notification delivery.
- Seven-stage ingestion pipeline: decode, envelope validation, schema validation, de-duplication, routing, transformation, buffered write.
- Per-event-type payload schema validation (required keys must be present, non-null and non-blank).
- Exactly-once style de-duplication through an idempotency table keyed on `event_id`.
- Micro-batched ClickHouse inserts (default 500 rows or 500 ms) with requeue-on-failure so nothing is silently dropped.
- Retry with exponential backoff plus jitter, permanent-error short-circuit, and dead-letter routing with an automated CRITICAL alert.
- Raw landing zone (`raw_events`) with a 90-day TTL and full provenance (topic, partition, offset, correlation id).
- Derived delivery lifecycle timeline with computed total, assignment, pickup and transit durations.
- Driver assignment response-time computation and terminal-result classification.
- Payment amount parsing as exact decimals (`Decimal(18, 2)`), including minor-unit (`amountMinor`) conversion and status derivation.
- Data-quality issue capture for out-of-order timestamps and negative durations.
- Apollo Federation v2.3 subgraph exposing eight queries with a uniform response envelope.
- Internal gRPC API with reflection and shared metadata/metrics/tracing interceptors.
- NATS publisher pushing a 10-second live metric ticker to the realtime service.
- Redis cache-aside with a configurable TTL and graceful degradation when Redis is unavailable.
- Reconciliation worker that grades ingestion freshness, raw-to-fact coverage, duplicates, unknown event types and open data-quality issues.
- Replay library for time-windowed reprocessing of event streams.
- Health probes, Prometheus metrics, OTLP tracing, structured logging and English/Arabic messages.
- Multi-stage Docker image plus a self-contained docker compose stack.

## Architecture

### Layering

```
cmd/server                 composition root: wiring, servers, worker pool, graceful shutdown
internal/
  config                   environment-driven configuration and defaults
  domain                   pure domain model: envelope, facts, data quality, state, errors
  ports                    outbound interfaces (writer, idempotency, cache, query repo)
  application/
    ingestion              pipeline, validator, deduplicator, router, batch writer, handlers
    analytics              four cache-aside query services used by GraphQL and gRPC
    reconciliation         freshness / coverage audit over the raw and fact tables
  adapters/
    kafka                  consumer (34 readers), topic manager, DLQ publisher
    clickhouse             client, DDL migrations, writer, metrics decorator, query repository
    idempotency            ClickHouse-backed de-duplication and checkpoint store
    redis                  cache client (namespaced keys)
    nats                   live metric publisher
    grpc                   gRPC server + generated protobuf code
  graphql                  SDL, request handler, resolvers, data loaders, health handlers
  observability            metrics, logger bootstrap, OTLP tracer
  validation               GraphQL argument parsing and sanitization
  i18n                     English and Arabic message tables
  testutil                 fixtures and mocks
```

### Write path

```
producer services
      |
      v
Kafka topics (34 + analytics.dlq)
      |
      v
kafkaadapter.Consumer  -- retry x N -->  DLQ publisher (+ CRITICAL alert)
      |
      v
bridgeToPipeline (metrics + span + event meta extraction)
      |
      v
IngestionPipeline.Process
      1 DecodeEnvelope          raw JSON -> EventEnvelope + payload map
      2 ValidateEnvelope        ids, version 1, occurredAt <= now+5m
      3 ValidateSchema          per-event required payload keys
      4 Deduplicator            SELECT count() FROM analytics_processed_events
      5 Router                  prefix "delivery" | "driver" | "payment" | "notification"
      6 Handler                 payload -> 7 fact row types (+ data quality issues)
      7 BatchWriter.Add         buffer until BATCH_SIZE rows or BATCH_FLUSH_MS elapses
      |
      v
clickhouse.MetricsWriter -> clickhouse.Writer -> ClickHouse
      |
      v
idempotency.Store.Save (only after a successful flush)
```

### Read path

```
GraphQL client --> :4009 /graphql --> LanguageMiddleware -> role gate (admin)
                                      -> DataLoaderMiddleware -> handler dispatch
                                      -> Resolver -> application/analytics service
                                      -> Redis cache-aside -> QueryRepository -> ClickHouse

gRPC client    --> :50057 analytics.AnalyticsService --> same services (scope "grpc")

NATS ticker    --> every 10s --> platform overview (scope "nats") --> analytics.realtime.metrics
```

### Startup order in `main()`

1. `observability.Init()` — installs the default `slog` logger (JSON when `NODE_ENV=production`, colored dev handler otherwise).
2. `config.Load()` — read the environment once; failure exits with code 1.
3. `observability.NewMetrics()` and `observability.NewTracer(...)` — tracer failure is a warning only; a 10% sample ratio is hard-coded in `main`.
4. ClickHouse: create the client, `Ping` (15 attempts, 5 s each), then `clickhouse.Migrate` runs the eight `CREATE TABLE IF NOT EXISTS` statements. Any failure exits.
5. Redis cache construction (`DialTimeout` 3 s).
6. Kafka: split brokers, `EnsureAnalyticsTopics(brokers, dlqTopic, 6, 1)` (34 topics + DLQ, 6 partitions, replication factor 1 — failure is a warning), then build the DLQ publisher.
7. Wrap the ClickHouse writer with `MetricsWriter`, build the idempotency store, the `BatchWriter` (started immediately with its flush ticker).
8. Build the router and register the four handlers, then build the `IngestionPipeline`.
9. Start the Kafka consumer in a goroutine, bridged into the pipeline.
10. Build the query repository and the four analytics services; build the GraphQL resolver.
11. Start the gRPC server in a goroutine (reflection enabled).
12. Connect the NATS publisher (failure is a warning; the live ticker is skipped when it fails).
13. Register the `reconciliation` and `live_ticker` workers and start the worker pool.
14. Mount HTTP routes, wrap them with tracing and RED-metrics middleware, start the main server (`4009`) and the dedicated metrics server (`9107`).
15. Block on `SIGINT`/`SIGTERM`, then shut down the HTTP servers, stop gRPC, stop the worker pool, close the consumer, cancel the ingestion context and run a final 10-second batch flush.

## Repository Layout

```
services/analytics-service/
|-- Dockerfile                        multi-stage build (golang:1.25-alpine -> alpine:3.20), EXPOSE 4009 50057 9107
|-- Makefile                          developer workflow (see Makefile section)
|-- README.md                         this file
|-- docker-compose.yml                local stack: kafka, clickhouse, redis, analytics-service
|-- go.mod / go.sum                   module definition; replace ../../packages/go
|-- coverage                          ad-hoc coverage profile from an earlier run
|-- cmd/server/
|   `-- main.go                       composition root: wiring, HTTP/gRPC servers, graceful shutdown,
|                                     bridgeToPipeline() and extractMeta()
|-- internal/
|   |-- adapters/
|   |   |-- clickhouse/
|   |   |   |-- client.go             connection config, Ping with retries, DB handle
|   |   |   |-- schema.go             8 CREATE TABLE IF NOT EXISTS statements + Migrate()
|   |   |   |-- writer.go             7 INSERT implementations behind ports.ClickHouseWriter
|   |   |   |-- metrics_writer.go     decorator recording flush metrics per table
|   |   |   `-- query_repository.go   analytics + reconciliation SQL (AnalyticsQueryRepository)
|   |   |-- grpc/
|   |   |   |-- server.go             analytics.AnalyticsService impl + interceptors
|   |   |   `-- proto/                generated analytics.pb.go / analytics_grpc.pb.go
|   |   |-- idempotency/
|   |   |   `-- clickhouse_idempotency.go  Exists/Save + checkpoint API
|   |   |-- kafka/
|   |   |   |-- consumer.go           34 readers, retry/backoff, DLQ routing, manual commits
|   |   |   |-- topic_manager.go      topic constants, AnalyticsTopics(), EnsureAnalyticsTopics()
|   |   |   `-- dlq_publisher.go      DLQMessage JSON schema + publisher
|   |   |-- nats/publisher.go         analytics.realtime.metrics publisher
|   |   `-- redis/cache.go            Get/Set/Delete with the "analytics:" prefix
|   |-- application/
|   |   |-- analytics/
|   |   |   |-- platform_overview.go  GetPlatformOverview + shared fetchCached[T]() helper
|   |   |   |-- delivery_analytics.go GetDeliveryAnalytics
|   |   |   |-- driver_analytics.go   GetDriverAnalytics + GetTopDrivers
|   |   |   `-- payment_analytics.go  GetPaymentAnalytics
|   |   |-- ingestion/
|   |   |   |-- pipeline.go           7-stage Process(), Outcome, PermanentError
|   |   |   |-- validator.go          DecodeEnvelope + envelope/schema validation rules
|   |   |   |-- deduplicator.go       IsDuplicate(eventID)
|   |   |   |-- router.go             prefix routing to handlers
|   |   |   |-- batch_writer.go       threshold + interval flush, requeue on failure
|   |   |   `-- handlers/             base, delivery, driver, payment, notification handlers
|   |   `-- reconciliation/           Check() report and freshness grading
|   |-- config/config.go              Config struct + Load() defaults
|   |-- domain/
|   |   |-- event.go                  EventEnvelope, EventVersion, validation
|   |   |-- fact_raw.go               RawEventLanding + 90-day retention constant
|   |   |-- fact_delivery.go          FactDeliveryEvent + FactDeliveryCompleted (timeline)
|   |   |-- fact_driver.go            FactDriverAssignment + AssignmentResult
|   |   |-- fact_payment.go           FactPaymentTransaction + amount parsing
|   |   |-- fact_notification.go      FactNotificationEvent + channel/status enums
|   |   |-- data_quality.go           DataQualityIssue, issue types, severities
|   |   |-- state.go                  IdempotencyRecord, IngestionCheckpoint
|   |   `-- errors.go                 24 sentinel errors
|   |-- graphql/
|   |   |-- schema_sdl.go             AnalyticsSubgraphSDL constant
|   |   |-- handler.go                HTTP handler, dispatch, auth, i18n, health handlers
|   |   |-- resolver.go               8 resolvers + response envelopes
|   |   `-- dataloader.go             Driver/Delivery stub loaders (batching)
|   |-- i18n/                         i18n.go, en.go, ar.go (18 keys per language)
|   |-- observability/
|   |   |-- metrics.go                15 Prometheus metrics + recorders
|   |   |-- logger.go                 Init() + EventAttrs() structured attributes
|   |   `-- tracing.go                OTLP/gRPC tracer, consume spans, operation spans
|   |-- ports/                        ClickHouseWriter, IdempotencyStore, CacheRepository,
|   |                                 AnalyticsQueryRepository, ReconciliationQueries
|   |-- testutil/                     fixtures (testdata.go) and mocks (mocks.go)
|   |-- validation/validator.go       GraphQL argument parsing, clamping, normalization
|   `-- workers/                      pool.go + reconciliation_worker.go
`-- (generated) coverage artifacts    coverage.out, coverage.html
```

## Package Reference

### `cmd/server`

The only `main` package. Besides the startup sequence described above it contains:

- `splitBrokers(string) []string` — comma-splitting with trimming and a `localhost:9092` fallback.
- `extractMeta(data []byte) (eventType string, occurredAt time.Time)` — pulls `eventType` and `occurredAt` out of the raw envelope before validation, so metrics and the freshness gauge work even for messages that later fail validation. `occurredAt` accepts an RFC3339 string or a Unix-milliseconds number.
- `bridgeToPipeline(pipeline, metrics, tracer)` — a `kafkaadapter.Handler` that records `analytics_events_consumed_total`, opens an `analytics.consume` span, runs `pipeline.Process`, then records processed/duplicate/failed counters, pipeline latency and the freshness gauge.

There are no test files under `cmd/server`; the composition root is exercised only manually.

### `internal/config`

`config.Load()` reads the environment once and always returns `nil` error. All values are strings except the integer knobs parsed with `getEnvInt` / `getEnvInt64`. See [Configuration Reference](#configuration-reference). `config_test.go` covers the default table, a full override table and the three env helpers (100% coverage).

### `internal/domain`

Pure model with zero infrastructure imports.

- **`EventEnvelope`** — `EventID`, `EventType`, `EventVersion`, `OccurredAt`, `Producer`, `AggregateType`, `AggregateID`, `CorrelationID`, `CausationID`, `SourceTopic`, `SourcePartition`, `SourceOffset`, `IngestedAt`. `Validate()` enforces non-blank ids, `EventVersion == 1` (`MaxSupportedEventVersion`), a non-zero `OccurredAt` and `OccurredAt <= now + 5 minutes` (`FutureTimestampTolerance`).
- **`RawEventLanding`** — verbatim landing row including `PayloadJSON`; `ExpiresAt()` adds `RawEventRetentionTTL` (90 days).
- **`FactDeliveryEvent`** — per-event row (`event_id`, `delivery_id`, `user_id`, `driver_id`, `event_type`, `event_version`, `city_id`, `zone_id`, timestamps, `correlation_id`).
- **`FactDeliveryCompleted`** — the merged lifecycle timeline with nullable `AssignedAt` … `CompletedAt` and computed `TotalDurationS`, `AssignmentDurationS`, `PickupDurationS`, `TransitDurationS`. `Validate()` rejects any timestamp before `CreatedAt` (`ErrOutOfOrderTimestamps`); `ComputeDurations()` rejects negative deltas (`ErrNegativeDuration`).
- **`FactDriverAssignment`** — `AssignmentResult` in `OFFERED|ACCEPTED|REJECTED|EXPIRED|RELEASED`, terminal results are the last four, `ComputeResponseTime()` stores the earliest terminal timestamp at or after `OfferedAt` in milliseconds.
- **`FactPaymentTransaction`** — `PaymentTransactionType` in `AUTHORIZATION|CAPTURE|REFUND|CANCEL`, `Amount` stored as an exact decimal string, `ParseDecimalAmount()` returns `ErrInvalidAmountFormat` / `ErrNegativeAmount`, `RefundExceedsCapture()` compares exact rationals.
- **`FactNotificationEvent`** — channel in `email|push|sms|in_app`, status in `created|sent|delivered|failed|retrying`, `RetryCount >= 0`.
- **`DataQualityIssue`** — eleven issue types, three severities (`ERROR`, `WARNING`, `INFO`), `ResolvedAt` marker.
- **State records** — `IdempotencyRecord` (dedupe identity stamped with `FirstSeenAt`) and `IngestionCheckpoint` with a monotonic `Advance(offset, at)` transition.
- **`errors.go`** — 24 sentinel errors shared by every layer (see [Data Quality and Error Handling](#data-quality-and-error-handling)).

### `internal/ports`

| Interface | Methods | Implementation |
|-----------|---------|----------------|
| `ClickHouseWriter` | `WriteRawEvents`, `WriteDeliveryEvents`, `WriteDeliveryCompleted`, `WriteDriverAssignments`, `WritePaymentTransactions`, `WriteNotificationEvents`, `WriteDataQualityIssues`, `Close` | `clickhouse.Writer`, wrapped by `clickhouse.MetricsWriter` |
| `IdempotencyStore` | `Exists`, `Save`, `LoadCheckpoint`, `SaveCheckpoint`, `Close` | `idempotency.Store` |
| `CacheRepository` | `Get`, `Set`, `Delete`, `Close` | `redisadapter.Cache` |
| `AnalyticsQueryRepository` | `PlatformOverview`, `DeliveryAnalytics`, `DriverAnalytics`, `TopDrivers`, `PaymentAnalytics`, `RawEvents`, `DataQualityIssues`, `Close` | `clickhouse.QueryRepository` |
| `ReconciliationQueries` | `MaxRawOccurredAt`, `CountRawEvents`, `CountFactDeliveryEvents`, `CountDataQualityByType`, `DistinctRawEventTypes`, `CountOpenIssuesBySeverity` | `clickhouse.QueryRepository` |

The ports package also owns `AnalyticsGranularity` (`HOUR|DAY|WEEK|MONTH`), `TimeRange` with `Validate()` (non-zero bounds, `to >= from`, span <= `MaxQueryRange` = 90 days, valid granularity), the filter/DTO structs, and `MaxQueryRange = 90 * 24 * time.Hour`.

### `internal/application/ingestion`

- **`IngestionPipeline.Process(ctx, topic, partition, offset, data) (Outcome, error)`** — the seven stages below. Errors from stages 1, 2, 3, 5 and 6 are wrapped as *permanent* (`IsPermanent(err) == true`) so the consumer stops retrying; stage 4 (dedup store) and stage 7 (buffer) are transient and retried.
- **`Validator`** — `DecodeEnvelope` normalizes the wire format (flexible timestamps, default version, derived `aggregateType`, `aggregateId` fallback through `deliveryId`, `driverId`, `assignmentId`, `paymentId`, `notificationId`, `id`, producer default `<aggregateType>-service`), `ValidateEnvelope` applies domain rules, `ValidateSchema` enforces per-event required keys.
- **`Deduplicator`** — one-line pass-through to `IdempotencyStore.Exists(eventID)`.
- **`Router`** — `Register(prefix, handler)` / `Route(eventType)`; the prefix is the segment before the first dot. Unknown prefixes or a handler that does not accept the type produce `ErrUnknownEventType`.
- **`BatchWriter`** — `Add` appends a `FactBatch` and flushes when the pending row count reaches `maxRows`; a ticker flushes every `flushInterval`; `Stop(ctx)` drains and performs a final flush. On a ClickHouse error the snapshot is re-queued in front of the pending queue, so rows are never dropped. Idempotency records are saved only after a successful flush (at-least-once by design).
- **`handlers`** — `Handler` interface (`Handles(eventType)`, `Handle(ctx, env, payload) (*FactBatch, error)`), `BaseHandler` extraction helpers (`GetString`, `GetTime`, `GetUint32`, `GetUint64`, `NewRawLanding`, `NewSeen`, `Qualify`) and the four concrete handlers described in [Event Ingestion](#event-ingestion).

### `internal/application/analytics`

Four services, each constructed as `(repo, cache, ttl)` with a `5 * time.Minute` fallback TTL, plus a shared generic helper:

```go
func fetchCached[T any](ctx, cache, key, ttl, load func(ctx) (T, error)) (T, error)
```

`fetchCached` reads the key, unmarshals and returns on a hit, otherwise calls `load`, marshals the result and writes it back with the TTL. `Set` failures are swallowed; a nil cache is tolerated (pure pass-through).

| Service | Method | Cache key template |
|---------|--------|--------------------|
| `PlatformOverviewService` | `GetPlatformOverview(ctx, range, scope)` | `platform:scope=%s:from=%s:to=%s:gran=%s:%s` |
| `DeliveryAnalyticsService` | `GetDeliveryAnalytics(ctx, filter, scope)` | `delivery:scope=%s:from=%s:to=%s:gran=%s:city=%s:driver=%s:%s` |
| `DriverAnalyticsService` | `GetDriverAnalytics(ctx, filter, scope)` | `driver:scope=%s:from=%s:to=%s:gran=%s:driver=%s:%s` |
| `DriverAnalyticsService` | `GetTopDrivers(ctx, range, limit, scope)` | `topdrivers:scope=%s:from=%s:to=%s:gran=%s:limit=%d:%s` |
| `PaymentAnalyticsService` | `GetPaymentAnalytics(ctx, filter, scope)` | `payment:scope=%s:from=%s:to=%s:gran=%s:provider=%s:%s` |

Times are RFC3339 UTC, the trailing `%s` is the cache version (`v1`), and `scope` is `global` (GraphQL), `grpc`, or `nats`. `GetTopDrivers` clamps `limit` to 10 when it is `<= 0` or `> 100`.

### `internal/application/reconciliation`

`NewService(queries, Config{WindowHours: 1})` with defaults `FreshWarnSeconds = 60`, `FreshDegradedSeconds = 300`, `MinCoverageRatio = 0.99`. `Check(ctx)` runs six queries over the window `[now - WindowHours, now]` and returns a `Report`:

| Field | Source |
|-------|--------|
| `FreshnessSeconds` | `now - max(occurred_at)` from `raw_events` |
| `Freshness` | `healthy` (< 60 s), `warning` (>= 60 s), `degraded` (>= 300 s) |
| `RawTotal` | `count()` of `raw_events` in the window |
| `FactTotal` | `count()` of `fact_delivery_events` in the window |
| `CoverageRatio` | `FactTotal / RawTotal` (0 when there are no raw events) |
| `GapDetected` | `CoverageRatio < 0.99` |
| `DuplicateCount` | `count()` of `duplicate_event` data-quality issues in the window |
| `UnknownTypes` | distinct `event_type` values not in the 35 known strings |
| `OpenIssues` | `severity -> count()` where `resolved_at IS NULL` |

### `internal/observability`

- **`metrics.go`** — 15 Prometheus collectors registered on the default registry (see [Health, Metrics, and Tracing](#health-metrics-and-tracing)). `GetFreshnessSeconds()` is hard-coded to return `0.0` (see [Design Notes](#design-notes-and-troubleshooting)).
- **`logger.go`** — `Init()` delegates to `packages/go/logging` (JSON handler when `NODE_ENV=production`, otherwise a colored dev handler); `EventAttrs()` builds the nine key/value pairs used for structured event logging.
- **`tracing.go`** — `NewTracer(TracerConfig{ServiceName, OTLPEndpoint, SamplingRate, EnableInsecure})` builds an OTLP/gRPC exporter only when the endpoint is non-empty; otherwise tracing is a nil-safe no-op. Helpers: `StartConsumeSpan` (span name `analytics.consume` with `messaging.system`, `messaging.destination`, `analytics.event_type`, `analytics.event_id`), `StartOperation`/`Operation.Finish`, `RecordError`, `ExtractTraceID`.

### `internal/validation`

GraphQL-layer argument handling (distinct from the Kafka wire validator):

- `ClampPage`, `ClampLimit` (default 20, max 100), `ClampTopLimit` (default 10).
- `ParseRange`, `ParseDeliveryFilter`, `ParseDriverFilter`, `ParsePaymentFilter`, `ParseLimit`, `ParsePage`, `ExtractQueryField` — read from `variables` first and fall back to regex extraction from the raw query text.
- `NormalizeEventType` — maps the 34 UPPER_SNAKE GraphQL enum values to internal dotted types (and back).
- `NormalizeProvider` — accepts only `STRIPE`, `PAYPAL`, `CASH`.
- `ValidateSeverity` — accepts `ERROR`, `WARNING`, `INFO` (case-insensitive).
- `ScopeFromRequest(r)` — reads `X-Scope`, defaults to `global`; used only to segment cache keys.

### `internal/graphql`

- **`schema_sdl.go`** — the full Apollo Federation v2.3 SDL as one string constant, served from `_service { sdl }`.
- **`handler.go`** — `LanguageMiddleware` (language precedence: `x-lang`, `X-Lang`, `?lang=`, `Accept-Language`; `ar*` -> Arabic, everything else -> English), `GraphQLHandler` (request parsing, introspection short-circuits, admin role gate, per-operation dispatch), `DataLoaderMiddleware`, `HealthLiveHandler`, `HealthReadyHandler(ping)`.
- **`resolver.go`** — eight resolvers producing the uniform `{success, statusCode, message, timeStamp, data}` envelope; `driverShape` resolves the federated `Driver` stub through the DataLoader.
- **`dataloader.go`** — `graph-gophers/dataloader/v7` batch loaders returning identity stubs (`DriverStub`, `DeliveryStub`) with the per-request cache disabled.

### `internal/i18n`

Eighteen keys per language (`en.go`, `ar.go`). `T(lang, key)` normalizes the tag, falls back to English and then to the raw key. `WithLanguage` / `FromContext` carry the language through the request context. Keys cover health messages, per-query success messages, five validation errors and three generic errors.

### `internal/testutil`

- `testdata.go` — factories for the envelope, all five fact types, data-quality issues, idempotency records and payload builders for the main event types, plus `NewTestEventEnvelopeJSON` producing the exact wire format.
- `mocks.go` — `MockIdempotencyStore`, `MockClickHouseWriter`, `MockCacheRepository`, `MockAnalyticsQueryRepository`, `MockReconciliationQueries`, each with setters to inject errors.

## Event Ingestion

### Kafka topics

Topics are ensured at startup with `EnsureAnalyticsTopics(brokers, dlqTopic, 6, 1)`: **6 partitions, replication factor 1**, plus the DLQ topic. Failure to create topics is logged as a warning (the compose broker also has `KAFKA_AUTO_CREATE_TOPICS_ENABLE=true`).

The consumer group is `analytics-service` (`KAFKA_GROUP_ID`), one `kafka-go` reader per topic with manual commits, `MinBytes 1`, `MaxBytes 10e6`, `MaxWait 500ms`, `StartOffset: FirstOffset`.

| Group | Topics consumed | Count |
|-------|-----------------|-------|
| Delivery | `delivery.created`, `delivery.driver.assigned`, `delivery.driver.accepted`, `delivery.pickup.started`, `delivery.picked_up`, `delivery.in_transit`, `delivery.completed`, `delivery.cancelled`, `delivery.failed` | 9 |
| Driver | `driver.available`, `driver.unavailable`, `driver.assignment.offered`, `driver.assignment.accepted`, `driver.assignment.rejected`, `driver.assignment.expired`, `driver.assignment.released` | 7 |
| Payment | `payment.created`, `payment.authorization.started`, `payment.authorized`, `payment.authorization.failed`, `payment.capture.started`, `payment.captured`, `payment.completed`, `payment.capture.failed`, `payment.cancelled`, `payment.refund.started`, `payment.refunded`, `payment.refund.failed`, `payment.failed` | 13 |
| Notification | `notification.created`, `notification.sent`, `notification.delivered`, `notification.failed`, `notification.retrying` | 5 |
| Dead letter (written by the service, never read by it) | `analytics.dlq` (`DLQ_TOPIC`) | 1 |
| **Consumed by readers** | 34 domain topics (`AnalyticsTopics()`) | **34** |

`delivery.deleted` has a topic constant and a schema rule, but it is deliberately absent from `AnalyticsTopics()`: it is neither ensured nor consumed, so `DeliveryHandler`'s delete branch is reachable only through tests.

Each topic name equals the event type name.

### Retry and dead-letter behaviour

1. `processWithRetry` attempts the handler up to `MAX_RETRY_ATTEMPTS + 1` times (default 6 total).
2. Between attempts it sleeps `delay + jitter` where `delay` starts at 100 ms, doubles per attempt and is capped at 1600 ms; the jitter is a random half of the current delay.
3. Retrying stops immediately for permanent errors: `kafkaadapter.ErrPermanent`, `domain.ErrUnsupportedEventVersion`, or `ingestion.IsPermanent(err)`.
4. On exhaustion or a permanent error the message is routed to `routeToDLQ`:
   - logs `routing message to DLQ` with topic, partition, offset and reason;
   - fires `automation.TriggerAlert("analytics-service", ..., "CRITICAL")` toward Alertmanager;
   - publishes a `DLQMessage` (`eventId`, `eventType`, `eventVersion`, `consumerGroup`, `originalTopic`, `partition`, `offset`, `attemptCount`, `error`, `failedAt`, `correlationId`, `payload`) with key `eventId` (falling back to `<topic>-<partition>-<offset>`);
   - copies the correlation id from the `x-trace-id` or `x-correlation-id` Kafka header.
5. The offset is committed **even when the message went to the DLQ**, so the partition never blocks. Fetch errors are logged and retried without committing.

### Pipeline stages

| # | Stage | Failure classification | Notes |
|---|-------|------------------------|-------|
| 1 | `DecodeEnvelope` | permanent (`decode:`) | sets `SourceTopic/Partition/Offset` and `IngestedAt` |
| 2 | `ValidateEnvelope` | permanent (`envelope:`) | ids, version 1, timestamp sanity |
| 3 | `ValidateSchema` | permanent (`schema:`) | per-event required payload keys |
| 4 | `Deduplicator.IsDuplicate` | transient (`deduplicate:`) | fail-closed: store errors never ack past |
| 5 | `Router.Route` | permanent (`route:`) | prefix dispatch |
| 6 | `Handler.Handle` | permanent (`transform:`) | payload to fact rows |
| 7 | `BatchWriter.Add` | transient (`buffer:`) | buffered unless a threshold flush runs |

A duplicate short-circuits at stage 4 and is reported as `OutcomeDuplicate`, which increments `analytics_events_duplicate_total` and records the processed metric with `outcome="duplicate"`.

### Envelope validation rules

Wire format keys: `eventId`, `eventType`, `eventVersion`, `occurredAt`, `timestamp`, `producer`, `aggregateType`, `aggregateId`, `correlationId`, `causationId`, `payload`.

- `occurredAt` accepts an RFC3339 string, a Unix-milliseconds number, or a numeric string; when missing or invalid the `timestamp` field is tried as a fallback; both missing produces `ErrInvalidOccurredAt`.
- `eventVersion <= 0` defaults to `1`; only version `1` is supported.
- `aggregateType` defaults to the segment of `eventType` before the first dot.
- `aggregateId` defaults to the first non-empty of `deliveryId`, `driverId`, `assignmentId`, `paymentId`, `notificationId`, `id`.
- `producer` defaults to `<aggregateType>-service`.
- `occurredAt` more than 5 minutes in the future is rejected with `ErrEventFromFuture`.

### Required payload keys per event type

| Event type(s) | Required keys |
|---------------|---------------|
| `delivery.created` | `deliveryId` |
| `delivery.driver.assigned`, `delivery.driver.accepted` | `deliveryId`, `driverId` |
| `delivery.pickup.started`, `delivery.picked_up`, `delivery.in_transit`, `delivery.completed`, `delivery.cancelled`, `delivery.failed`, `delivery.deleted` | `deliveryId` |
| `driver.available`, `driver.unavailable` | `driverId` |
| `driver.assignment.offered` | `assignmentId`, `driverId` |
| `driver.assignment.accepted`, `.rejected`, `.expired`, `.released` | `assignmentId` |
| all 13 `payment.*` types | `paymentId` |
| all 5 `notification.*` types | `notificationId` |

A missing, `null` or whitespace-only value fails with `ErrInvalidEnvelope: <key>`; an unregistered type fails with `ErrUnknownEventType`.

### Handlers

| Handler | Events | Outputs |
|---------|--------|---------|
| `DeliveryHandler` | 10 `delivery.*` types | `raw_events` + `fact_delivery_events` + `fact_delivery_completed` + idempotency. `delivery.deleted` writes only the raw landing row and the idempotency record. `MergeTimeline` folds the event into the lifecycle row; validation failures emit a data-quality issue (`out_of_order_timestamps` / WARNING, `negative_duration` / ERROR) instead of a row. |
| `DriverHandler` | 5 `driver.assignment.*` types, plus `driver.available` / `driver.unavailable` | `raw_events` + `fact_driver_assignments`. Availability events are raw-only. Response time is computed from the earliest terminal timestamp at or after `offered_at`. |
| `PaymentHandler` | 13 `payment.*` types | `raw_events` + `fact_payment_transactions`. Provider defaults to `STRIPE` and is upper-cased; `amountMinor` is converted to a 2-decimal string when `amount` is absent; status is taken from the payload or derived from the event suffix. |
| `NotificationHandler` | 5 `notification.*` types | `raw_events` + `fact_notification_events`. Channel defaults to `push`; `retryCount` is clamped at 0; `sentAt` / `deliveredAt` / `failedAt` are captured when present. |

Every handler also emits an `IdempotencyRecord` for the event.

### Batching

- Threshold: `BATCH_SIZE` rows (default 500).
- Interval: `BATCH_FLUSH_MS` milliseconds (default 500).
- Flush order is fixed: `raw_events` -> `fact_delivery_events` -> `fact_delivery_completed` -> `fact_driver_assignments` -> `fact_payment_transactions` -> `fact_notification_events` -> `analytics_data_quality_issues`.
- Each table is written inside a transaction (`BeginTx` / `PrepareContext` / per-row `ExecContext` / `Commit`); any row error rolls the whole table back.
- A failed flush re-queues the snapshot so rows survive; periodic flush failures surface as consumer lag rather than data loss.
- Idempotency rows are inserted **after** a successful flush, so a crash between the two replays the event (at-least-once).

## ClickHouse Schema

`clickhouse.Migrate` runs these statements at startup, in order, logging `clickhouse schema ready` per table. There are no versioned migrations — the DDL is idempotent.

| Table | Engine | Partition | Order by | TTL |
|-------|--------|-----------|----------|-----|
| `raw_events` | `MergeTree` | `toYYYYMM(occurred_at)` | `(aggregate_type, occurred_at, event_id)` | `occurred_at + 90 DAY` |
| `fact_delivery_events` | `MergeTree` | `toYYYYMM(occurred_at)` | `(event_type, occurred_at, delivery_id)` | none |
| `fact_delivery_completed` | `ReplacingMergeTree(ingested_at)` | `toYYYYMM(created_at)` | `(delivery_id)` | none |
| `fact_driver_assignments` | `MergeTree` | `toYYYYMM(offered_at)` | `(driver_id, offered_at, assignment_id)` | none |
| `fact_payment_transactions` | `MergeTree` | `toYYYYMM(occurred_at)` | `(provider, occurred_at, payment_id)` | none |
| `fact_notification_events` | `MergeTree` | `toYYYYMM(occurred_at)` | `(channel, occurred_at, notification_id)` | none |
| `analytics_data_quality_issues` | `MergeTree` | `toYYYYMM(detected_at)` | `(severity, detected_at, issue_id)` | none |
| `analytics_processed_events` | `MergeTree` | `toYYYYMM(first_seen_at)` | `(event_id)` | none |

### Columns

**raw_events** — `event_id String`, `event_type LowCardinality(String)`, `event_version UInt16`, `aggregate_type LowCardinality(String)`, `aggregate_id String`, `producer LowCardinality(String)`, `occurred_at DateTime64(3, 'UTC')`, `ingested_at DateTime64(3, 'UTC')`, `correlation_id String`, `causation_id String`, `source_topic LowCardinality(String)`, `source_partition UInt32`, `source_offset UInt64`, `payload_json String`.

**fact_delivery_events** — `event_id`, `delivery_id`, `user_id`, `driver_id`, `event_type LowCardinality`, `event_version UInt16`, `city_id LowCardinality`, `zone_id LowCardinality`, `occurred_at`, `ingested_at`, `correlation_id`.

**fact_delivery_completed** — `delivery_id`, `user_id`, `driver_id`, `created_at`, plus nullable `assigned_at`, `accepted_at`, `pickup_started_at`, `picked_up_at`, `in_transit_at`, `delivered_at`, `completed_at`; nullable `total_duration_seconds`, `assignment_duration_s`, `pickup_duration_s`, `transit_duration_s` (all `UInt32`); `city_id LowCardinality`, `ingested_at`.

**fact_driver_assignments** — `assignment_id`, `delivery_id`, `driver_id`, `offered_at`, nullable `accepted_at`, `rejected_at`, `expired_at`, `released_at`, nullable `response_time_ms UInt32`, `assignment_result LowCardinality`, `ingested_at`.

**fact_payment_transactions** — `event_id`, `payment_id`, `delivery_id`, `user_id`, `provider LowCardinality`, `transaction_type LowCardinality`, `status LowCardinality`, `amount Decimal(18, 2)`, `currency LowCardinality`, nullable `provider_latency_ms UInt64`, `occurred_at`, `ingested_at`.

**fact_notification_events** — `event_id`, `notification_id`, `recipient_type LowCardinality`, `channel LowCardinality`, `status LowCardinality`, `template_id LowCardinality`, nullable `sent_at`, `delivered_at`, `failed_at`, `retry_count UInt32`, `error String`, `occurred_at`, `ingested_at`.

**analytics_data_quality_issues** — `issue_id`, `event_id`, `issue_type LowCardinality`, `aggregate_type LowCardinality`, `aggregate_id`, `detected_at`, `severity LowCardinality`, `details String`, nullable `resolved_at`.

**analytics_processed_events** — `event_id`, `aggregate_type LowCardinality`, `aggregate_id`, `source_topic LowCardinality`, `source_offset Int64`, `first_seen_at`.

### Query repository behaviour

`QueryRepository` runs the read model. Bucketing uses `toStartOfHour` / `toStartOfDay` / `toStartOfWeek` / `toStartOfMonth` according to `AnalyticsGranularity`; pages clamp to `page >= 1` and `limit` in 1..100 (default 20).

| Method | Queries |
|--------|---------|
| `PlatformOverview` | counts by `event_type` over `fact_delivery_events`; `avg(total_duration_seconds)` over `fact_delivery_completed FINAL`; captured/refunded sums over `fact_payment_transactions`; accepted ratio over `fact_driver_assignments` |
| `DeliveryAnalytics` | the same counts (plus optional `city_id` / `driver_id` filters); `avg` + `quantile(0.5/0.95/0.99)` + `avg(assignment_duration_s)` over `fact_delivery_completed FINAL`; time buckets with `countIf` per status; a second bucketed query merged for `avgDurationSeconds` |
| `DriverAnalytics` | offers/accepted/rejected/expired and `avg(response_time_ms)` over `fact_driver_assignments`, plus completed deliveries per driver over `fact_delivery_completed FINAL` |
| `TopDrivers` | grouped by `driver_id`, ordered by acceptance ratio, `HAVING count() > 0`, `LIMIT ?`; a distinct count for pagination; one batched `driver_id IN (...)` query for completed deliveries (anti N+1) |
| `PaymentAnalytics` | authorization count/success rate, capture count/sum, refund count/sum and `avg(provider_latency_ms)` in one pass, with an optional `provider = ?` filter |
| `RawEvents` | `count()` + `ORDER BY occurred_at DESC LIMIT/OFFSET` over `raw_events` with optional `event_type` and `[from, to]` |
| `DataQualityIssues` | `count()` + `ORDER BY detected_at DESC` with optional `severity` |

## GraphQL API

### Endpoints

| Path | Port | Notes |
|------|------|-------|
| `/graphql` | 4009 | canonical endpoint |
| `/analytics/graphql` | 4009 | alias; both share the same handler chain |

Handler chain (outermost first): `sharedtracing.HTTPTracingMiddleware` -> `pkgmetrics.HTTPMetricsMiddleware` -> `gql.LanguageMiddleware` -> mux -> `DataLoaderMiddleware` -> `GraphQLHandler`.

Request body:

```json
{ "query": "...", "variables": { ... }, "operationName": "..." }
```

### Headers

| Header | Purpose |
|--------|---------|
| `x-user-role` | must be `admin`; any other value returns HTTP 403 with `extensions.code: FORBIDDEN`. Not required for `_service`, `__typename`, `__schema`/`__type` and `analyticsServiceInfo`, which are answered before the gate. |
| `x-lang` / `X-Lang` / `?lang=` / `Accept-Language` | `ar` (or any tag starting with `ar`) selects Arabic messages; everything else is English |
| `x-scope` | cache-key segment only (default `global`); does not filter data |
| `authorization` | not validated by this subgraph; the gateway is the enforcement point |

### Response envelope

Every query returns `success`, `statusCode`, `message` (localized), `timeStamp` (RFC3339 String) and `data`, nested under the operation name:

```json
{
  "data": {
    "platformOverview": {
      "success": true,
      "statusCode": 200,
      "message": "Platform overview retrieved successfully",
      "timeStamp": "2026-10-25T09:15:00Z",
      "data": { "totalDeliveries": 1284, "completionRate": 0.93, "dataAsOf": "2026-10-25T09:15:00Z" }
    }
  }
}
```

Validation failures return HTTP 200 with `errors[0].message` prefixed by the localized internal-error text; authorization failures return HTTP 403; malformed JSON returns HTTP 400.

### Enums

- `AnalyticsGranularity`: `HOUR`, `DAY`, `WEEK`, `MONTH`
- `DataQualitySeverity`: `ERROR`, `WARNING`, `INFO`
- `PaymentProvider`: `STRIPE`, `PAYPAL`, `CASH`
- `AnalyticsEventType`: 34 values (`DELIVERY_*` x10, `DRIVER_*` x7, `PAYMENT_*` x12, `NOTIFICATION_*` x5)

Federation entities declared by the subgraph: `User @key(fields: "id")`, `Delivery @key(fields: "id")`, `Driver @key(fields: "id")`, plus `@shareable` on `AnalyticsServiceInfo` and `PaginationInfo`.

### Queries

**`analyticsServiceInfo`** — no arguments; static `{name, version, status}` (`analytics-service`, `1.0.0`, `healthy`). Bypasses the admin gate.

**`platformOverview(range: AnalyticsRange!): PlatformOverviewResponse!`**

```graphql
query GetPlatformOverview {
  platformOverview(range: { from: "2026-09-12T00:00:00Z", to: "2026-10-19T23:59:59Z", granularity: DAY }) {
    success
    statusCode
    message
    timeStamp
    data {
      totalDeliveries completedDeliveries cancelledDeliveries failedDeliveries
      completionRate averageDeliveryDurationSeconds
      paymentCapturedAmount refundedAmount driverAcceptanceRate dataAsOf
    }
  }
}
```

**`deliveryAnalytics(filter: DeliveryAnalyticsFilter!): DeliveryAnalyticsResponse!`** — filter carries `range` plus optional `cityId` and `driverId`. Returns totals, `completionRate`, `averageDurationSeconds`, `p50DurationSeconds`, `p95DurationSeconds`, `p99DurationSeconds`, `averageAssignmentTimeSeconds`, a `buckets[]` series (`bucket`, `total`, `completed`, `cancelled`, `failed`, `avgDurationSeconds`) and `dataAsOf`.

**`driverAnalytics(filter: DriverAnalyticsFilter!): DriverAnalyticsResponse!`** — filter carries `range` plus optional `driverId`. Returns `driverId`, the federated `driver { id }` stub (loaded through the DataLoader), `offers`, `accepted`, `rejected`, `expired`, `acceptanceRate`, `averageResponseTimeMs`, `completedDeliveries`, `dataAsOf`.

**`topDrivers(range: AnalyticsRange!, limit: Int): DriverAnalyticsListResponse!`** — ordered by acceptance rate; `limit` is clamped to 10 when `<= 0` or `> 100`. Returns `paginationInfo { totalItems, currentPage, nextPage }` and `items[]`.

**`paymentAnalytics(filter: PaymentAnalyticsFilter!): PaymentAnalyticsResponse!`** — filter carries `range` plus optional `provider`. Returns `authorizationCount`, `authorizationSuccessRate`, `captureCount`, `capturedAmount`, `refundCount`, `refundedAmount`, `refundRate`, `averageProviderLatencyMs`, `dataAsOf`.

**`rawAnalyticsEvents(page: Int, limit: Int, eventType: AnalyticsEventType, from: DateTime, to: DateTime): RawAnalyticsEventsResponse!`** — paginated raw landing rows (`eventId`, `eventType`, `eventVersion`, `aggregateType`, `aggregateId`, `producer`, `occurredAt`, `ingestedAt`, `correlationId`, `sourceTopic`), ordered by `occurredAt` descending. Not cached.

**`dataQualityIssues(page: Int, limit: Int, severity: DataQualitySeverity): DataQualityIssuesResponse!`** — paginated issues (`issueId`, `eventId`, `issueType`, `aggregateType`, `aggregateId`, `detectedAt`, `severity`, `details`), ordered by `detectedAt` descending. Not cached.

Ready-to-run examples for all eight queries live in `graphql-docs/analytics.graphql` at the repository root.

### Dispatch notes

The handler does not parse GraphQL; it dispatches with `strings.Contains` in this order: `_service` -> `__typename` (unless `analyticsServiceInfo`) -> `__schema`/`__type` -> `analyticsServiceInfo` -> admin gate -> `platformOverview` -> `deliveryAnalytics` -> `driverAnalytics` (only when the query does not contain `topDrivers`) -> `topDrivers` -> `paymentAnalytics` -> `rawAnalyticsEvents` -> `dataQualityIssues` -> `operation not recognized`.

### Federation

`_service { sdl }` returns `AnalyticsSubgraphSDL`, which declares the Apollo Federation v2.3 link and the entity keys, so the Apollo router can compose this subgraph and resolve `Driver`, `Delivery` and `User` references.

## gRPC API

Service `analytics.AnalyticsService`, port `PORT_GRPC` (default `50057`), gRPC reflection enabled, source proto `../../protos/analytics.proto` (repository root).

| RPC | Request | Response |
|-----|---------|----------|
| `/analytics.AnalyticsService/GetPlatformOverview` | `from`, `to` (RFC3339 strings) | `total_deliveries`, `completed_deliveries`, `completion_rate`, `data_as_of` |
| `/analytics.AnalyticsService/GetDriverPerformance` | `driver_id` (required), `from`, `to` | `driver_id`, `offers`, `accepted`, `acceptance_rate`, `completed_deliveries` |

Behaviour:

- `from` defaults to now minus 24 hours, `to` defaults to now; unparsable values return `codes.InvalidArgument`.
- An empty `driver_id` returns `codes.InvalidArgument` with `driver_id is required`.
- Both RPCs call the same application services with `GranularityDay` and scope `"grpc"`, so they share the GraphQL cache namespace.
- Service failures surface as `codes.Internal`.
- Unary interceptors (in order): `sharedmiddleware.UnaryServerMetadataInterceptor`, `pkgmetrics.UnaryServerMetricsInterceptor`, `sharedtracing.UnaryServerTracingInterceptor`.

Example:

```bash
grpcurl -plaintext localhost:50057 list
grpcurl -plaintext -d '{"from":"2026-10-24T00:00:00Z","to":"2026-10-25T00:00:00Z"}' \
  localhost:50057 analytics.AnalyticsService/GetPlatformOverview
```

## NATS Live Ticker

When `NATS_URL` is reachable, the worker pool registers `live_ticker` with a 10-second interval:

1. reads the platform overview for the last hour with `GranularityHour` and scope `nats`;
2. computes `completionRate = completedDeliveries / totalDeliveries` (0 when there are none);
3. publishes JSON to subject **`analytics.realtime.metrics`**:

```json
{
  "timestamp": "2026-10-25T09:15:00Z",
  "activeEvents": 128,
  "completionRate": 0.93,
  "freshnessSec": 0
}
```

Publishing is fire-and-forget (no JetStream) and silently no-ops when the connection is down. Connection failure at startup is a warning; the ticker is then not registered at all. `freshnessSec` currently always reports `0` (see [Design Notes](#design-notes-and-troubleshooting)).

## Caching

| Concern | Value |
|---------|-------|
| Backend | Redis (`REDIS_HOST:REDIS_PORT`, 3 s dial timeout) |
| Key namespace | `analytics:` prefix + the service key template + `:v1` |
| TTL | `CACHE_TTL_SECONDS` (default 300 s); services fall back to 5 minutes when `ttl <= 0` |
| Read path | `Get` -> JSON unmarshal -> return on hit |
| Write path | `json.Marshal` -> `Set` with TTL; write errors ignored |
| Failure mode | any Redis error is treated as a miss; the query falls through to ClickHouse |
| Not cached | `rawAnalyticsEvents`, `dataQualityIssues` (resolver-level, uncached by design) |

Example key: `analytics:delivery:scope=global:from=2026-10-18T00:00:00Z:to=2026-10-25T00:00:00Z:gran=DAY:city=:driver=:v1`.

## Background Workers

`workers.Pool` runs named jobs on independent tickers:

- `Register(name, interval, fn)` ignores non-positive intervals and nil functions.
- `Start(ctx)` launches one goroutine per job; each logs `worker started`, ticks, and executes `fn` inside a closure with `recover()` (a panicking job logs `worker panic recovered` and keeps ticking).
- Jobs stop on context cancellation or `Stop()`; `Stop()` is idempotent and waits for all goroutines.

| Worker | Interval | Action |
|--------|----------|--------|
| `reconciliation` | `RECONCILE_INTERVAL_SEC` (default 300 s) | `reconciliation.Service.Check` over the last hour |
| `live_ticker` | 10 s | publish the NATS metric ticker (only when NATS connected) |

### Reconciliation logging

`ReconciliationWorker.Run` logs one structured line per pass with `freshness_seconds`, `freshness`, `raw_total`, `fact_total`, `coverage_ratio`, `gap_detected`, `duplicates`, `open_issues` (and `unknown_types` when non-empty), at:

- `ERROR` — `reconciliation pass: action required` when a gap was detected or freshness is `degraded`;
- `WARN` — `reconciliation pass: attention` when freshness is `warning` or unknown event types appeared;
- `INFO` — `reconciliation pass: healthy` otherwise.

A failing pass is logged and never crashes the pool.

## Health, Metrics, and Tracing

| Endpoint | Port | Behaviour |
|----------|------|-----------|
| `GET /health/live` | 4009 | `200` envelope with `{name, version, status}` while the process runs |
| `GET /health/ready` | 4009 | `200` when the ClickHouse ping succeeds, otherwise `503` with a logged error |
| `GET /metrics` | 4009 | Prometheus handler mounted on the main mux (also excluded from RED metrics) |
| `POST /graphql`, `POST /analytics/graphql` | 4009 | GraphQL |
| `GET :9107/metrics` | 9107 | dedicated metrics server (10 s read/write timeouts) |
| gRPC | 50057 | `analytics.AnalyticsService` + reflection |

Main HTTP server timeouts: read 15 s, write 15 s, idle 60 s.

### Prometheus metrics

| Name | Type | Labels | Description |
|------|------|--------|-------------|
| `analytics_events_consumed_total` | Counter | `topic` | Kafka events consumed per topic |
| `analytics_events_processed_total` | Counter | `event_type` | Events transformed and buffered |
| `analytics_events_failed_total` | Counter | `event_type` | Events that failed processing |
| `analytics_events_duplicate_total` | Counter | `topic` | Redelivered events collapsed by idempotency |
| `analytics_events_dlq_total` | Counter | `topic` | Events routed to the dead-letter topic |
| `analytics_processing_latency_ms` | Histogram | `event_type`, `outcome` | Per-event pipeline latency (1..2048 ms, exponential buckets of 2) |
| `analytics_batch_size` | Histogram | `table` | Rows per ClickHouse flush |
| `analytics_batch_flush_total` | Counter | `table`, `status` | ClickHouse flushes by table and status |
| `analytics_query_latency_ms` | Histogram | `query` | GraphQL query latency |
| `analytics_query_errors_total` | Counter | `query` | Failed GraphQL queries |
| `analytics_replay_events_total` | Counter | `topic`, `outcome` | Events reprocessed by replay jobs |
| `analytics_data_quality_issues_total` | Counter | `issue_type`, `severity` | Data-quality issues detected |
| `analytics_freshness_seconds` | Gauge | - | Now minus the latest successfully processed event time |
| `kafka_consumer_lag` | Gauge | `topic`, `partition` | Kafka consumer lag |
| `clickhouse_insert_latency` | Histogram | `table`, `status` | Insert latency in seconds (default buckets) |

`RecordDLQ`, `SetConsumerLag`, `RecordReplay` and `RecordDataQualityIssue` are implemented and tested but not yet called from production paths.

### Tracing

- Exporter: OTLP/gRPC to `OTEL_ENDPOINT`, batched, insecure when `EnableInsecure` is set; empty endpoint disables tracing entirely (nil-safe no-op).
- Sampler: parent-based trace-id ratio, hard-coded to `0.1` in `main`.
- Propagators: W3C `TraceContext` and `Baggage`.
- Spans: `analytics.consume` per Kafka message (attributes `messaging.system=kafka`, `messaging.destination`, `analytics.event_type`, `analytics.event_id`) plus HTTP server spans via `sharedtracing.HTTPTracingMiddleware`.
- Shutdown: 5 s flush timeout during process exit.

### Logging

Structured `slog` through `packages/go/logging`: JSON on stdout when `NODE_ENV=production`, otherwise a colored single-line dev handler. Level comes from `LOG_LEVEL` (`debug|info|warn|error`, default `info`). The context handler appends `traceId`, `userId`, `method` and `path` when present.

## Configuration Reference

All configuration is environment variables; values below are the defaults (`internal/config/config.go`).

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT_GRAPHQL` | `4009` | GraphQL + health HTTP port |
| `PORT_GRPC` | `50057` | gRPC port |
| `PORT_METRICS` | `9107` | Prometheus metrics HTTP port |
| `NODE_ENV` | `development` | `production` selects the JSON log handler |
| `CLICKHOUSE_HOST` | `localhost` | ClickHouse host |
| `CLICKHOUSE_PORT` | `9000` | ClickHouse native port |
| `CLICKHOUSE_DB` | `analytics` | Database name (also used by migrations and queries) |
| `CLICKHOUSE_USER` | `default` | Username |
| `CLICKHOUSE_PASSWORD` | `clickhouse` | Password |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated broker list |
| `KAFKA_GROUP_ID` | `analytics-service` | Consumer group |
| `KAFKA_CLIENT_ID` | `analytics-service` | Client id |
| `NATS_URL` | `nats://localhost:4222` | NATS URL for the live ticker |
| `SNOWFLAKE_WORKER_ID` | `5` | Reserved worker id for id generation |
| `BATCH_SIZE` | `500` | Rows per ClickHouse flush |
| `BATCH_FLUSH_MS` | `500` | Time-based flush interval in milliseconds |
| `MAX_RETRY_ATTEMPTS` | `5` | Retries after the first attempt (6 attempts total) |
| `DLQ_TOPIC` | `analytics.dlq` | Dead-letter topic |
| `CACHE_TTL_SECONDS` | `300` | Redis TTL for analytics queries |
| `RECONCILE_INTERVAL_SEC` | `300` | Reconciliation worker interval |
| `OTEL_ENDPOINT` | _(empty)_ | OTLP/gRPC endpoint; empty disables tracing |
| `LOG_LEVEL` | `info` | slog level (read by the shared logger) |

Hard-coded operational values: raw-event retention 90 days, max query range 90 days, future-timestamp tolerance 5 minutes, supported event version 1, Kafka topic partitions 6 / replication factor 1, trace sample ratio 0.1, NATS ticker 10 s, reconciliation window 1 hour, freshness thresholds 60 s (warning) and 300 s (degraded), coverage gap threshold 0.99.

## Running Locally

### Prerequisites

- Go 1.25+
- GNU Make
- Docker with the compose plugin (for the dependency stack and image builds)
- Optional: `golangci-lint`, `staticcheck`, `govulncheck`, `protoc` + plugins, `grpcurl`, `curl`

### With docker compose (recommended)

```bash
cd services/analytics-service
make docker-up        # kafka, clickhouse, redis and the service
make health           # 200 from /health/live
make ready            # 200 from /health/ready (ClickHouse ping)
make graphql-sdl      # federation smoke test
make docker-logs      # follow the service logs
```

`docker-compose.yml` starts four services:

| Service | Container | Host ports |
|---------|-----------|------------|
| `kafka-srv` | `analytics-kafka-srv` | 9092 |
| `clickhouse-srv` | `analytics-clickhouse-srv` | 8123, 9000 |
| `redis-srv` | `analytics-redis-srv` | 6379 |
| `analytics-service` | `analytics-service-local` | 4009, 50057, 9107 |

The compose file mounts `internal/` and `packages/go` into the image for source visibility; the container still runs the compiled binary, so rebuild with `make docker-up-build` after source changes. ClickHouse data lives in the `clickhouse_data` volume.

Run the binary on the host instead of the container with `make run`; the defaults in `config.Load()` already point at `localhost:9092`, `localhost:9000`, `localhost:6379` and `nats://localhost:4222`.

### Against your own infrastructure

Set the variables from the configuration table (at minimum `CLICKHOUSE_HOST`, `KAFKA_BROKERS`), then:

```bash
make run
```

Startup fails fast when ClickHouse is unreachable (15 pings x 5 s with 2 s pauses) or when migrations fail. Kafka topic creation, tracer init and the NATS publisher are all non-fatal warnings.

### Full platform stack

The root `infrastructure/docker/compose.yml` also defines this service (container `analytics-service`, network alias `analytics-srv`). Because host port 4009 is already taken by media-service, the platform stack publishes analytics GraphQL on **4019** (`4019:4009`) while gRPC (50057) and metrics (9107) keep their ports. Kubernetes manifests live in `infrastructure/kubernetes/base/services/analytics-depl.yaml` (startup/readiness/liveness probes on `/health/live` and `/health/ready`, Prometheus scrape annotations on 9107, PDB and HPA 1..5 replicas) with configuration from `analytics-service-config` and `analytics-secrets`.

### Docker image

```bash
make docker-build
# equivalent to:
# docker build -f Dockerfile -t analytics-service:latest ../..
```

Multi-stage build: `golang:1.25-alpine` compiles `./cmd/server` with `CGO_ENABLED=0 -ldflags="-w -s"` (BuildKit caches for `go/pkg/mod` and the build cache), then `alpine:3.20` with ca-certificates and tzdata runs `/analytics-service`. Ports 4009, 50057 and 9107 are exposed. The build context must be the monorepo root because `go.mod` replaces `packages/go` with `../../packages/go`.

## Makefile

Run from `services/analytics-service`. `make help` prints the full list.

| Target | Command summary |
|--------|-----------------|
| `help` | print available targets (default goal) |
| `version` | `go version` |
| `check-deps` | verify `go`, `docker`, `docker compose` |
| `deps` | `go list -m all` |
| `tidy` / `download` / `mod-verify` | `go mod tidy` / `go mod download` / `go mod verify` |
| `fmt` | `gofmt -w .` |
| `fmt-check` | fail when `gofmt -l .` reports any file (make-level conditional, shell-independent) |
| `vet` | `go vet ./...` |
| `lint` | `golangci-lint run ./...` (optional tool) |
| `staticcheck` | `staticcheck ./...` (optional tool) |
| `vuln` | `govulncheck ./...` (optional tool) |
| `build` | compile `bin/analytics-service` from `./cmd/server` |
| `rebuild` | `clean-bin` then `build` |
| `run` | `go run ./cmd/server` |
| `test` / `test-v` / `test-short` / `test-race` | `go test ./... -count=1` with variants |
| `test-cover` | `go test ./... -cover -count=1` (per-package summary) |
| `coverage` | tests with `-covermode=atomic -coverprofile=coverage.out` |
| `coverage-func` | `go tool cover -func=coverage.out` (runs `coverage` first) |
| `coverage-html` | render `coverage.html` (runs `coverage` first) |
| `bench` | `go test ./... -run=XXX -bench=. -benchmem` |
| `verify` | `fmt-check` + `vet` + `build` + `test` + `coverage` (full gate) |
| `ci` | `tidy` + `verify` |
| `proto-tools` | install `protoc-gen-go` and `protoc-gen-go-grpc` |
| `proto-check` | verify `../../protos/analytics.proto` exists |
| `proto-generate` | regenerate `internal/adapters/grpc/proto` |
| `docker-build` | build the image from the monorepo root |
| `docker-up` / `docker-up-build` | `docker compose up -d` (optionally `--build`) |
| `docker-down` / `docker-down-volumes` | `docker compose down` (optionally `-v`) |
| `docker-restart` / `docker-logs` / `docker-ps` / `docker-shell` / `docker-prune` | stack lifecycle helpers |
| `compose-check` | `docker compose config --quiet` |
| `infra-up` / `infra-down` / `infra-logs` | manage `kafka-srv clickhouse-srv redis-srv` |
| `kafka-up` / `clickhouse-up` / `redis-up` and the matching `*-logs` targets | single-service control |
| `kafka-topics` / `kafka-describe` / `kafka-dlq` | `kafka-topics.sh` / `kafka-console-consumer.sh` inside the broker container |
| `clickhouse-client` / `clickhouse-tables` / `clickhouse-query` | `clickhouse-client` in the `analytics` database; `QUERY=` is overridable |
| `redis-cli` / `redis-keys` / `redis-flush` | redis shell, `SCAN` of `analytics:*`, `FLUSHDB` |
| `health` / `ready` / `metrics` | `curl` the liveness, readiness and metrics endpoints |
| `graphql-sdl` | POST `{_service{sdl}}` as a federation smoke test |
| `grpc-list` | `grpcurl -plaintext localhost:50057 list` |
| `ports` | print the port map |
| `clean` / `clean-bin` / `clean-coverage` | `git clean -fdx` on `bin/`, `coverage.out`, `coverage.html` and the legacy `coverage` file |

Recipes are written one self-contained command per line (no shell operators such as `&&`, pipes or redirection) and all branching uses make functions (`$(if)`, `$(wildcard)`, `$(error)`) rather than shell syntax, so the same file works with any POSIX-compatible shell. The blank-line separators in `help` and `ports` use `echo.`, which relies on the `cmd.exe` shell that GNU Make for Windows selects when `sh.exe` is not on `PATH`.

## Testing

### Strategy

The suite is hermetic — no Kafka broker, ClickHouse server or Redis is required:

1. **Domain unit tests** — envelope validation (versions, future timestamps, aggregate types), fact validation, duration and response-time computation, amount parsing, data-quality rules, checkpoint transitions.
2. **Application unit tests** — pipeline stage ordering and error classification (decode/envelope/schema/route/transform/buffer), router dispatch, deduplicator, batch-writer threshold flush and requeue-on-error, validator decode/fallback rules, all four handlers, reconciliation grading boundaries, cache-aside hit/miss/error paths and limit clamping.
3. **Transport tests** — GraphQL handler coverage for every operation plus auth (403), i18n selection, introspection short-circuits, malformed JSON and unknown operations; health handlers; validation parsing and normalization; i18n fallbacks.
4. **Infrastructure tests** — metrics registration and every recorder, logger attribute layout, worker pool scheduling/panic recovery.

Fixtures come from `internal/testutil` (`NewTestEventEnvelope`, `NewTestFactDeliveryCompleted`, payload builders, `NewTestEventEnvelopeJSON`) and five mocks implementing the ports.

Commands:

```bash
make test           # full suite
make test-v         # verbose
make test-race      # race detector (requires a C compiler)
make coverage       # coverage.out
make coverage-func  # per-function totals
make coverage-html  # coverage.html
```

### Coverage

Current numbers from `make coverage` (all packages, `-covermode=atomic`):

| Package | Coverage |
|---------|----------|
| `internal/config` | 100.0% |
| `internal/domain` | 97.6% |
| `internal/i18n` | 95.8% |
| `internal/application/ingestion/handlers` | 95.3% |
| `internal/application/ingestion` | 94.4% |
| `internal/application/analytics` | 92.3% |
| `internal/validation` | 92.3% |
| `internal/workers` | 88.4% |
| `internal/application/reconciliation` | 84.1% |
| `internal/graphql` | 78.1% |
| `internal/observability` | 31.2% (metrics and logger tested; `tracing.go` is not) |
| `cmd/server`, `internal/adapters/*`, `internal/ports`, `internal/testutil` | 0.0% (no test files) |
| **Total** | **46.1%** |

`go vet ./...` passes with no findings.

## Data Quality and Error Handling

### Sentinel errors (`internal/domain/errors.go`)

| Group | Errors |
|-------|--------|
| Envelope identity | `ErrInvalidEnvelope`, `ErrMissingEventID`, `ErrMissingEventType`, `ErrMissingAggregateID`, `ErrUnknownEventType`, `ErrUnsupportedEventVersion` |
| Time | `ErrInvalidOccurredAt`, `ErrEventFromFuture` |
| Idempotency | `ErrDuplicateEvent` |
| Fact validation | `ErrMissingDeliveryID`, `ErrMissingDriverID`, `ErrMissingAssignmentID`, `ErrMissingPaymentID`, `ErrMissingNotificationID`, `ErrNegativeDuration`, `ErrOutOfOrderTimestamps`, `ErrNegativeAmount`, `ErrInvalidAmountFormat`, `ErrRefundExceedsCapture`, `ErrInvalidRetryCount` |
| Data quality | `ErrInvalidIssueSeverity`, `ErrMissingIssueType`, `ErrMissingIssueID` |
| State | `ErrInvalidCheckpoint` |

Permanent vs transient: decode, envelope, schema, route and transform failures are permanent (`ingestion.PermanentError`) and go straight to the DLQ; dedup-store and buffer failures are transient and retried.

### Data-quality issue types

`negative_duration`, `out_of_order_timestamps`, `negative_amount`, `refund_exceeds_capture`, `unknown_reference`, `future_timestamp`, `duplicate_event`, `unsupported_version`, `missing_aggregate_id`, `invalid_envelope`, `unknown_event_type`. Severities: `ERROR`, `WARNING`, `INFO`. The issue id is `<eventId>:<issueType>`.

Producers in current code: the delivery handler emits `out_of_order_timestamps` (WARNING) and `negative_duration` (ERROR); reconciliation counts `duplicate_event`. Issues land in `analytics_data_quality_issues` and are queryable through `dataQualityIssues`.

### Retry classification

| Class | Examples | Behaviour |
|-------|----------|-----------|
| Permanent | decode failure, missing required key, unknown event type, negative amount | no retry; DLQ with `attemptCount = 1`; CRITICAL alert |
| Transient | Redis/dedup store unavailable, ClickHouse flush failure, Kafka fetch error | retried with backoff, or re-queued for the next flush |

## Design Notes and Troubleshooting

- **`make fmt-check` currently fails.** `gofmt -l .` reports 37 files. Every difference is formatting-only — no semantic change: struct and map column alignment (for example `internal/domain/fact_delivery_test.go`, `internal/config/config_test.go`), one-line function literals that gofmt splits (`internal/domain/state_test.go`), a stray blank line (`internal/application/ingestion/validator.go`), the i18n tables (`internal/i18n/en.go`, `internal/i18n/ar.go`), a missing newline at end of file (`internal/i18n/i18n_test.go`), and CRLF line endings in `internal/adapters/clickhouse/schema.go` and `internal/adapters/grpc/proto/analytics_grpc.pb.go` (this machine checks out with `core.autocrlf=true`; gofmt normalizes to LF). Run `make fmt` to normalize, review the diff, then `make verify`. Because `fmt-check` is part of `verify` and `ci`, both gates fail until the tree is formatted.
- **`GetFreshnessSeconds()` always returns `0`.** The NATS payload field `freshnessSec` is therefore constant; the real value is exported by the `analytics_freshness_seconds` gauge, which is updated from the Kafka bridge. Reading the gauge instead of the helper would make the ticker accurate.
- **`delivery.deleted` is not consumed.** The constant and the schema rule exist, but the topic is absent from `AnalyticsTopics()`, so delete events are neither ensured nor read. Add `TopicDeliveryDeleted` to the slice if tombstone analytics are required.
- **`payment.completed` has no GraphQL enum counterpart.** Ingestion and reconciliation know 35 event types, but the `AnalyticsEventType` enum declares only 34 values — `PAYMENT_COMPLETED` is absent — so `rawAnalyticsEvents` cannot be filtered by `payment.completed` even though those rows exist in `raw_events`. `eventTypeEnumToInternal` is complete for the enum as declared (34 of 34).
- **`analytics_checkpoints` has no DDL.** `idempotency.Store.LoadCheckpoint`/`SaveCheckpoint` read and write that table, but `clickhouse.Migrate` does not create it. The methods are currently unused by the running service; if checkpointing is wired up, add the table to `schema.go` first.
- **Four metrics recorders are test-only**: `RecordDLQ`, `SetConsumerLag`, `RecordReplay`, `RecordDataQualityIssue`. DLQ routing currently logs and alerts instead of incrementing `analytics_events_dlq_total`.
- **No NATS in the service compose file.** `docker-compose.yml` sets `NATS_URL: nats://nats-srv:4222` but defines no NATS service, so startup logs `nats publisher unavailable (non-fatal)` and the `live_ticker` worker is skipped. The root platform stack does run `nats-srv`.
- **Idempotency is at-least-once, not exactly-once.** Dedup rows are written after a successful ClickHouse flush; a crash in between re-processes the event on redelivery. `fact_delivery_completed` is protected by `ReplacingMergeTree(ingested_at)`, but the other fact tables are plain `MergeTree` and can receive duplicate rows in that window.
- **Port conflicts in the platform stack.** media-service already publishes 4009, so the root compose maps analytics GraphQL to 4019. The service-local compose keeps 4009.
- **Readiness depends only on ClickHouse.** `/health/ready` pings ClickHouse; Kafka, Redis and NATS outages do not fail the probe (they degrade functionality instead).
- **Startup is ClickHouse-first.** An unreachable ClickHouse fails the process after ~90 seconds of pings; use `make clickhouse-up` before `make run`.
- **Module replace** — `go.mod` replaces `packages/go` with `../../packages/go`, so builds must run from inside the monorepo; the Dockerfile uses the repository root as build context for the same reason.
- **Batch visibility window** — up to `BATCH_SIZE` rows or `BATCH_FLUSH_MS` milliseconds of data can be pending in memory. Querying immediately after producing events may show slightly stale numbers; the freshness gauge reflects this.
- **90-day query cap** — `ports.MaxQueryRange` mirrors the `raw_events` TTL; ranges wider than 90 days are rejected with `range exceeds maximum of 2160h0m0s`.
- **i18n** — send `x-lang: ar` (or `Accept-Language: ar-SA`) for Arabic messages; unknown languages fall back to English.
- **Federation** — this is a subgraph; wire it into the Apollo router through `_service { sdl }`. Direct queries to `/graphql` work without a router as long as `x-user-role: admin` is set.
- **Coverage artifacts** — `coverage.out` and `coverage.html` are produced by the Makefile; `coverage.out` matches the `*.out` rule in the repository `.gitignore`. The extensionless `coverage` file in this directory is a leftover profile from an ad-hoc run and is removed by `make clean-coverage`.
