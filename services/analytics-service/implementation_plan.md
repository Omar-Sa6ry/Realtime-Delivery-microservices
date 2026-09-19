# Analytics Service — Full Implementation Plan

> **Language:** Go | **Database:** ClickHouse | **API:** GraphQL Federation Subgraph  
> **Pattern:** Hexagonal Architecture + CQRS (Read/Write separation) + Event-Driven  
> Based on: Payment Service & Driver Service conventions

---

## Overview

The Analytics Service is the analytical bounded context of the Realtime Delivery Platform. It consumes durable Kafka events from Delivery, Driver, Payment, and Notification domains, transforms them into analytical facts, aggregates, and dimensions in ClickHouse, and exposes a GraphQL Federation subgraph.

### Port Assignments (matching existing services)
| Port | Purpose |
|---|---|
| `4009` | GraphQL HTTP |
| `50057` | gRPC (future use) |
| `9107` | Prometheus metrics |

---

## Design Patterns Applied

| Category | Pattern | Where Used |
|---|---|---|
| **Creational** | Factory Method | `NewKafkaConsumer`, `NewClickHouseWriter`, `NewLoaders`, `NewConfig` |
| **Creational** | Builder | `QueryBuilder` for ClickHouse queries |
| **Structural** | Adapter | Kafka adapter, ClickHouse adapter, Redis cache adapter |
| **Structural** | Decorator | Retry decorator, Metrics decorator on writers |
| **Structural** | Facade | `IngestionPipeline` (validates → deduplicates → transforms → writes) |
| **Behavioral** | Strategy | Event router (dispatch by event type), Retry strategy |
| **Behavioral** | Observer | Prometheus metrics hooks |
| **Behavioral** | Template Method | Base event handler → concrete domain handlers |
| **Behavioral** | Chain of Responsibility | Middleware pipeline (validate → dedup → transform → batch) |
| **Behavioral** | Command | `ProcessEventCommand` |

---

## Open Questions

> [!IMPORTANT]
> **Port 4009 Confirmation** — Is port `4009` free for analytics-service GraphQL? Assigned by comparison to existing services (payment=4002, driver=4008, search=4006).

> [!IMPORTANT]
> **ClickHouse Deployment** — Do you want ClickHouse deployed as a Kubernetes StatefulSet or use an external managed instance? The plan assumes local StatefulSet for dev/Skaffold.

> [!NOTE]
> **Notification Events** — The notification service may not publish Kafka events yet. The plan includes the consumer handler but marks it optional/configurable.

---

## Proposed Changes

---

### Component 1 — Analytics Service Core

#### [NEW] `services/analytics-service/` — Full Service Structure

```
analytics-service/
├── cmd/
│   └── server/
│       └── main.go                        ← Entry point, wires everything
├── internal/
│   ├── config/
│   │   └── config.go                      ← Config struct + Load()
│   ├── domain/
│   │   ├── errors.go                      ← Domain errors
│   │   ├── event.go                       ← EventEnvelope, EventVersion, validation
│   │   ├── fact_delivery.go               ← FactDeliveryEvent, FactDeliveryCompleted
│   │   ├── fact_driver.go                 ← FactDriverAssignment
│   │   ├── fact_payment.go                ← FactPaymentTransaction
│   │   ├── fact_notification.go           ← FactNotificationEvent
│   │   ├── fact_raw.go                    ← RawEventLanding
│   │   ├── data_quality.go                ← DataQualityIssue domain model
│   │   └── state.go                       ← Analytics state (idempotency keys, etc.)
│   ├── ports/
│   │   ├── event_writer.go                ← Interface: ClickHouseWriter
│   │   ├── event_store.go                 ← Interface: IdempotencyStore
│   │   ├── cache.go                       ← Interface: CacheRepository
│   │   └── analytics_repository.go        ← Interface: AnalyticsQueryRepository
│   ├── application/
│   │   ├── ingestion/
│   │   │   ├── pipeline.go                ← IngestionPipeline (Facade)
│   │   │   ├── validator.go               ← Envelope + schema validation
│   │   │   ├── deduplicator.go            ← Idempotency check
│   │   │   ├── router.go                  ← Event router (Strategy pattern)
│   │   │   ├── handlers/
│   │   │   │   ├── base_handler.go        ← Template Method base
│   │   │   │   ├── delivery_handler.go    ← Handle delivery.* events
│   │   │   │   ├── driver_handler.go      ← Handle driver.* events
│   │   │   │   ├── payment_handler.go     ← Handle payment.* events
│   │   │   │   └── notification_handler.go← Handle notification.* events
│   │   │   └── batch_writer.go            ← Bounded batch buffer + flush
│   │   ├── analytics/
│   │   │   ├── delivery_analytics.go      ← Delivery metrics queries
│   │   │   ├── driver_analytics.go        ← Driver metrics queries
│   │   │   ├── payment_analytics.go       ← Payment metrics queries
│   │   │   └── platform_overview.go       ← Platform-wide aggregates
│   │   ├── replay/
│   │   │   └── replay_service.go          ← Replay/rebuild from Kafka
│   │   └── reconciliation/
│   │       └── reconciliation_service.go  ← Gap detection + reconciliation
│   ├── adapters/
│   │   ├── kafka/
│   │   │   ├── consumer.go                ← Kafka multi-topic consumer
│   │   │   ├── dlq_publisher.go           ← DLQ publisher
│   │   │   └── topic_manager.go           ← EnsureTopics
│   │   ├── clickhouse/
│   │   │   ├── client.go                  ← ClickHouse connection pool
│   │   │   ├── writer.go                  ← Implements ports.EventWriter
│   │   │   ├── migrations.go              ← DDL migration runner
│   │   │   └── query_repository.go        ← Implements ports.AnalyticsQueryRepository
│   │   ├── redis/
│   │   │   └── cache.go                   ← Implements ports.CacheRepository
│   │   └── idempotency/
│   │       └── clickhouse_idempotency.go  ← Implements ports.IdempotencyStore via ClickHouse
│   ├── graphql/
│   │   ├── schema_sdl.go                  ← Full Apollo Federation SDL
│   │   ├── handler.go                     ← Gin HTTP handler (same pattern as payment)
│   │   ├── resolver.go                    ← All queries/mutations/subscriptions
│   │   └── dataloader.go                  ← DataLoader for N+1 prevention
│   ├── workers/
│   │   ├── pool.go                        ← Worker pool (reuse pattern from driver)
│   │   └── reconciliation_worker.go       ← Periodic reconciliation cron
│   ├── observability/
│   │   ├── metrics.go                     ← Prometheus metrics registry
│   │   ├── tracing.go                     ← OpenTelemetry tracer setup
│   │   └── logger.go                      ← Structured slog setup
│   ├── i18n/
│   │   ├── i18n.go                        ← Same pattern as payment-service
│   │   ├── en.go
│   │   └── ar.go
│   └── validation/
│       └── validator.go                   ← Input validation helpers
├── migrations/
│   ├── 001_raw_events.sql
│   ├── 002_fact_delivery_events.sql
│   ├── 003_fact_delivery_completed.sql
│   ├── 004_fact_driver_assignments.sql
│   ├── 005_fact_payment_transactions.sql
│   ├── 006_fact_notification_events.sql
│   ├── 007_dim_user.sql
│   ├── 008_dim_driver.sql
│   ├── 009_delivery_hourly_metrics.sql
│   ├── 010_driver_hourly_metrics.sql
│   ├── 011_payment_daily_metrics.sql
│   └── 012_data_quality_issues.sql
├── Dockerfile
├── go.mod
└── go.sum
```

---

### Component 2 — GraphQL SDL & Resolvers

#### [NEW] `services/analytics-service/internal/graphql/schema_sdl.go`

Full Apollo Federation v2 SDL including:

```graphql
# Directives
directive @key(fields: String!) repeatable on OBJECT | INTERFACE
directive @shareable on OBJECT | FIELD_DEFINITION
directive @external on FIELD_DEFINITION

extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", ...)

# Scalars
scalar DateTime
scalar Decimal
scalar Long

# Enums
enum AnalyticsGranularity { HOUR DAY WEEK MONTH }

# Federation stubs
type User @key(fields: "id") { id: ID! }
type Delivery @key(fields: "id") { id: ID! }
type Driver @key(fields: "id") { id: ID! }

# Service info
type AnalyticsServiceInfo @shareable { name: String! version: String! status: String! }
type AnalyticsServiceInfoResponse { success: Boolean! statusCode: Int! message: String! timeStamp: String! data: AnalyticsServiceInfo }

# Pagination (shared @shareable)
type PaginationInfo @shareable { totalItems: Int! currentPage: Int! nextPage: Int }

# Input types
input AnalyticsRange { from: DateTime! to: DateTime! granularity: AnalyticsGranularity! }
input DeliveryAnalyticsFilter { range: AnalyticsRange! cityId: String driverId: ID }
input DriverAnalyticsFilter { range: AnalyticsRange! driverId: ID }
input PaymentAnalyticsFilter { range: AnalyticsRange! provider: String }

# Platform overview
type PlatformOverview {
  totalDeliveries: Long!
  completedDeliveries: Long!
  cancelledDeliveries: Long!
  failedDeliveries: Long!
  completionRate: Float!
  averageDeliveryDurationSeconds: Float!
  paymentCapturedAmount: Decimal!
  refundedAmount: Decimal!
  driverAcceptanceRate: Float!
  dataAsOf: DateTime!
}

# Delivery Analytics
type DeliveryMetricBucket { bucket: DateTime! total: Long! completed: Long! cancelled: Long! failed: Long! avgDurationSeconds: Float! }
type DeliveryAnalytics {
  total: Long!
  completed: Long!
  cancelled: Long!
  failed: Long!
  completionRate: Float!
  averageDurationSeconds: Float!
  p50DurationSeconds: Float!
  p95DurationSeconds: Float!
  p99DurationSeconds: Float!
  averageAssignmentTimeSeconds: Float!
  buckets: [DeliveryMetricBucket!]!
  dataAsOf: DateTime!
}

# Driver Analytics
type DriverAnalytics {
  driverId: ID
  driver: Driver
  offers: Long!
  accepted: Long!
  rejected: Long!
  expired: Long!
  acceptanceRate: Float!
  averageResponseTimeMs: Float!
  completedDeliveries: Long!
  dataAsOf: DateTime!
}
type DriverAnalyticsListData { paginationInfo: PaginationInfo! items: [DriverAnalytics!]! }

# Payment Analytics
type PaymentAnalytics {
  authorizationCount: Long!
  authorizationSuccessRate: Float!
  captureCount: Long!
  capturedAmount: Decimal!
  refundCount: Long!
  refundedAmount: Decimal!
  refundRate: Float!
  averageProviderLatencyMs: Float!
  dataAsOf: DateTime!
}

# Raw event (for admin queries)
type RawAnalyticsEvent {
  eventId: ID!
  eventType: String!
  eventVersion: Int!
  aggregateType: String!
  aggregateId: String!
  producer: String!
  occurredAt: DateTime!
  ingestedAt: DateTime!
  correlationId: String
  sourceTopic: String!
}
type RawAnalyticsEventsData { paginationInfo: PaginationInfo! items: [RawAnalyticsEvent!]! }

# Data Quality
type DataQualityIssue {
  issueId: ID!
  eventId: String!
  issueType: String!
  aggregateType: String!
  aggregateId: String!
  detectedAt: DateTime!
  severity: String!
  details: String
}
type DataQualityIssuesData { paginationInfo: PaginationInfo! items: [DataQualityIssue!]! }

# Responses
type PlatformOverviewResponse { success: Boolean! statusCode: Int! message: String! timeStamp: String! data: PlatformOverview }
type DeliveryAnalyticsResponse { success: Boolean! statusCode: Int! message: String! timeStamp: String! data: DeliveryAnalytics }
type DriverAnalyticsResponse { success: Boolean! statusCode: Int! message: String! timeStamp: String! data: DriverAnalytics }
type DriverAnalyticsListResponse { success: Boolean! statusCode: Int! message: String! timeStamp: String! data: DriverAnalyticsListData }
type PaymentAnalyticsResponse { success: Boolean! statusCode: Int! message: String! timeStamp: String! data: PaymentAnalytics }
type RawAnalyticsEventsResponse { success: Boolean! statusCode: Int! message: String! timeStamp: String! data: RawAnalyticsEventsData }
type DataQualityIssuesResponse { success: Boolean! statusCode: Int! message: String! timeStamp: String! data: DataQualityIssuesData }

type Query {
  _service: _Service!
  analyticsServiceInfo: AnalyticsServiceInfoResponse!
  
  # Platform-wide
  platformOverview(range: AnalyticsRange!): PlatformOverviewResponse!
  
  # Delivery analytics
  deliveryAnalytics(filter: DeliveryAnalyticsFilter!): DeliveryAnalyticsResponse!
  
  # Driver analytics  
  driverAnalytics(filter: DriverAnalyticsFilter!): DriverAnalyticsResponse!
  topDrivers(range: AnalyticsRange!, limit: Int): DriverAnalyticsListResponse!
  
  # Payment analytics
  paymentAnalytics(filter: PaymentAnalyticsFilter!): PaymentAnalyticsResponse!
  
  # Admin / debugging
  rawAnalyticsEvents(page: Int, limit: Int, eventType: String, from: DateTime, to: DateTime): RawAnalyticsEventsResponse!
  dataQualityIssues(page: Int, limit: Int, severity: String): DataQualityIssuesResponse!
}

type _Service { sdl: String! }
```

---

### Component 3 — DataLoader (N+1 Prevention)

#### [NEW] `services/analytics-service/internal/graphql/dataloader.go`

```go
type Loaders struct {
    // Batch-load driver profiles by IDs (for DriverAnalytics.driver field)
    DriverByID *dataloader.Loader[string, *DriverStub]
    // Batch-load delivery info by IDs
    DeliveryByID *dataloader.Loader[string, *DeliveryStub]
}
```

The DataLoader batches calls to Driver gRPC / Delivery gRPC to resolve federation stubs.  
This prevents the N+1 problem when listing `topDrivers` and resolving the `driver` field on each item.

**Pattern:** Same as driver-service (`graph-gophers/dataloader/v7`) with `WithLoaders` / `GetLoaders` context pattern.

---

### Component 4 — Kafka Consumer & Pipeline

#### [NEW] `services/analytics-service/internal/adapters/kafka/consumer.go`

Multi-topic consumer subscribing to:
- `delivery.events` (partition key: deliveryId)
- `driver.events` (partition key: driverId/assignmentId)
- `payment.events` (partition key: paymentId)
- `notification.events` (optional)

**Pipeline flow:**
```
Kafka Message
    ↓ Decode (JSON)
    ↓ Envelope Validation (eventId, eventType, occurredAt required)
    ↓ Schema Validation (version-aware per event type)
    ↓ Deduplication (check ClickHouse idempotency table by eventId)
    ↓ Event Router (Strategy: dispatch to correct domain handler)
    ↓ Domain Handler (transform to fact/dimension)
    ↓ Batch Buffer (bounded, e.g. 500 events or 500ms flush)
    ↓ ClickHouse Batch Write
    ↓ Safe Kafka Offset Commit
```

**Failure path:**
```
error → transient? → exponential backoff (100ms, 200ms, 400ms, 800ms, 1600ms) + jitter
             ↓ permanent? → DLQ topic (analytics.dlq)
```

Uses `github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events` package event types.

---

### Component 5 — ClickHouse Schema (DDL Migrations)

#### [NEW] `services/analytics-service/migrations/`

**001_raw_events.sql**
```sql
CREATE TABLE IF NOT EXISTS raw_events (
    event_id       String,
    event_type     LowCardinality(String),
    event_version  UInt16,
    aggregate_type LowCardinality(String),
    aggregate_id   String,
    producer       LowCardinality(String),
    occurred_at    DateTime64(3, 'UTC'),
    ingested_at    DateTime64(3, 'UTC'),
    correlation_id String,
    causation_id   String,
    source_topic   LowCardinality(String),
    source_partition UInt32,
    source_offset  UInt64,
    payload_json   String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (aggregate_type, occurred_at, event_id)
TTL occurred_at + INTERVAL 90 DAY;
```

**002_fact_delivery_events.sql**
```sql
CREATE TABLE IF NOT EXISTS fact_delivery_events (
    event_id       String,
    delivery_id    String,
    user_id        String,
    driver_id      String,
    event_type     LowCardinality(String),
    event_version  UInt16,
    city_id        LowCardinality(String),
    zone_id        LowCardinality(String),
    occurred_at    DateTime64(3, 'UTC'),
    ingested_at    DateTime64(3, 'UTC'),
    correlation_id String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (event_type, occurred_at, delivery_id);
```

**003_fact_delivery_completed.sql**
```sql
CREATE TABLE IF NOT EXISTS fact_delivery_completed (
    delivery_id              String,
    user_id                  String,
    driver_id                String,
    created_at               DateTime64(3, 'UTC'),
    assigned_at              Nullable(DateTime64(3, 'UTC')),
    accepted_at              Nullable(DateTime64(3, 'UTC')),
    pickup_started_at        Nullable(DateTime64(3, 'UTC')),
    picked_up_at             Nullable(DateTime64(3, 'UTC')),
    in_transit_at            Nullable(DateTime64(3, 'UTC')),
    delivered_at             Nullable(DateTime64(3, 'UTC')),
    completed_at             Nullable(DateTime64(3, 'UTC')),
    total_duration_seconds   Nullable(UInt32),
    assignment_duration_s    Nullable(UInt32),
    pickup_duration_s        Nullable(UInt32),
    transit_duration_s       Nullable(UInt32),
    city_id                  LowCardinality(String),
    ingested_at              DateTime64(3, 'UTC')
) ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(created_at)
ORDER BY (delivery_id);
```

**004_fact_driver_assignments.sql**
```sql
CREATE TABLE IF NOT EXISTS fact_driver_assignments (
    assignment_id     String,
    delivery_id       String,
    driver_id         String,
    offered_at        DateTime64(3, 'UTC'),
    accepted_at       Nullable(DateTime64(3, 'UTC')),
    rejected_at       Nullable(DateTime64(3, 'UTC')),
    expired_at        Nullable(DateTime64(3, 'UTC')),
    released_at       Nullable(DateTime64(3, 'UTC')),
    response_time_ms  Nullable(UInt32),
    assignment_result LowCardinality(String),  -- ACCEPTED|REJECTED|EXPIRED|RELEASED
    ingested_at       DateTime64(3, 'UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(offered_at)
ORDER BY (driver_id, offered_at, assignment_id);
```

**005_fact_payment_transactions.sql**
```sql
CREATE TABLE IF NOT EXISTS fact_payment_transactions (
    event_id            String,
    payment_id          String,
    delivery_id         String,
    user_id             String,
    provider            LowCardinality(String),
    transaction_type    LowCardinality(String),  -- AUTHORIZATION|CAPTURE|REFUND|CANCEL
    status              LowCardinality(String),
    amount              Decimal(18, 2),
    currency            LowCardinality(String),
    provider_latency_ms Nullable(UInt64),
    occurred_at         DateTime64(3, 'UTC'),
    ingested_at         DateTime64(3, 'UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (provider, occurred_at, payment_id);
```

**009_delivery_hourly_metrics.sql** (Materialized aggregate)
```sql
CREATE MATERIALIZED VIEW IF NOT EXISTS delivery_hourly_metrics
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(hour)
ORDER BY (city_id, hour)
AS SELECT
    toStartOfHour(occurred_at) AS hour,
    city_id,
    countIf(event_type = 'delivery.created') AS total_deliveries,
    countIf(event_type = 'delivery.completed') AS completed_deliveries,
    countIf(event_type = 'delivery.cancelled') AS cancelled_deliveries,
    countIf(event_type = 'delivery.failed') AS failed_deliveries
FROM fact_delivery_events
GROUP BY hour, city_id;
```

**012_data_quality_issues.sql**
```sql
CREATE TABLE IF NOT EXISTS analytics_data_quality_issues (
    issue_id       String,
    event_id       String,
    issue_type     LowCardinality(String),
    aggregate_type LowCardinality(String),
    aggregate_id   String,
    detected_at    DateTime64(3, 'UTC'),
    severity       LowCardinality(String),  -- ERROR|WARNING|INFO
    details        String,
    resolved_at    Nullable(DateTime64(3, 'UTC'))
) ENGINE = MergeTree
PARTITION BY toYYYYMM(detected_at)
ORDER BY (severity, detected_at, issue_id);
```

---

### Component 6 — Packages (Shared Go) — Additions

#### [MODIFY] `packages/go/events/` — Add Analytics Event Types

**[NEW] `packages/go/events/analytics_events.go`**
Add `AnalyticsEventType` constants for DLQ and internal analytics signals.

#### [NEW] `packages/go/events/notification_events.go`
Add notification event types and payloads:
```go
type NotificationEventType string
const (
    NotificationCreated  NotificationEventType = "notification.created"
    NotificationSent     NotificationEventType = "notification.sent"
    NotificationDelivered NotificationEventType = "notification.delivered"
    NotificationFailed   NotificationEventType = "notification.failed"
    NotificationRetrying NotificationEventType = "notification.retrying"
)
```

#### [MODIFY] `packages/go/kafka/kafka.go`
The existing `EnsureTopics` function is used by all Go services. Verify it accepts the analytics topics list (it already accepts `[]string` of topics).

---

### Component 7 — Infrastructure: Kubernetes

#### [NEW] `infrastructure/kubernetes/base/secrets/analytics-secrets.yaml`
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: analytics-secrets
type: Opaque
data:
  CLICKHOUSE_PASSWORD: Y2xpY2tob3VzZQ==   # clickhouse
  JWT_SECRET: <same as other services>
```

#### [NEW] `infrastructure/kubernetes/base/configmaps/analytics-service-config.yaml`
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: analytics-service-config
data:
  PORT_GRAPHQL: "4009"
  PORT_GRPC: "50057"
  PORT_METRICS: "9107"
  NODE_ENV: "development"
  CLICKHOUSE_HOST: "clickhouse-srv"
  CLICKHOUSE_PORT: "9000"
  CLICKHOUSE_DB: "analytics"
  CLICKHOUSE_USER: "default"
  REDIS_HOST: "redis-srv"
  REDIS_PORT: "6379"
  KAFKA_BROKERS: "kafka-srv:9092"
  KAFKA_GROUP_ID: "analytics-service"
  KAFKA_CLIENT_ID: "analytics-service"
  NATS_URL: "nats://nats-srv:4222"
  SNOWFLAKE_WORKER_ID: "5"
  BATCH_SIZE: "500"
  BATCH_FLUSH_MS: "500"
  MAX_RETRY_ATTEMPTS: "5"
  DLQ_TOPIC: "analytics.dlq"
  CACHE_TTL_SECONDS: "300"
  OTEL_ENDPOINT: ""
```

#### [NEW] `infrastructure/kubernetes/base/services/analytics-depl.yaml`
```yaml
# Deployment + PDB + HPA + Service + ServiceAccount
# Pattern: identical to payment-depl.yaml structure
# initContainers: wait-for-kafka, wait-for-clickhouse
# ports: 4009 (graphql), 50057 (grpc), 9107 (metrics)
# prometheus annotations: scrape=true, port=9107
```

#### [NEW] `infrastructure/kubernetes/base/infrastructure/clickhouse-depl.yaml`
```yaml
# ClickHouse StatefulSet
# Service: clickhouse-srv:9000 (native), :8123 (HTTP)
# PVC for data persistence
```

#### [MODIFY] `infrastructure/kubernetes/base/kustomization.yaml`
Add entries:
```yaml
- secrets/analytics-secrets.yaml
- configmaps/analytics-service-config.yaml
- services/analytics-depl.yaml
- infrastructure/clickhouse-depl.yaml
```

#### [MODIFY] `infrastructure/skaffold/skaffold.yaml`
Add analytics artifact:
```yaml
- image: delivery/analytics
  context: .
  docker:
    dockerfile: services/analytics-service/Dockerfile
```
Add port-forward for analytics GraphQL (port 4009).

---

### Component 8 — API Gateway Integration

#### [MODIFY] `services/api-gateway/src/app.module.ts`

Add the Analytics subgraph URL to the Apollo Federation supergraph config:
```typescript
{
  name: 'analytics',
  url: process.env.ANALYTICS_SERVICE_URL || 'http://analytics-srv:4009/graphql',
}
```

#### [MODIFY] `services/api-gateway/src/` (configmap)
Add `ANALYTICS_SERVICE_URL` env var to the API gateway ConfigMap.

---

### Component 9 — GraphQL Docs

#### [NEW] `graphql-docs/analytics.graphql`

Complete query/mutation/subscription documentation file:

```graphql
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Analytics Service GraphQL Operations
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Get Analytics Service Info
query GetAnalyticsServiceInfo {
  analyticsServiceInfo {
    success
    statusCode
    message
    timeStamp
    data {
      name
      version
      status
    }
  }
}

# Platform Overview (last 7 days, daily buckets)
query GetPlatformOverview {
  platformOverview(range: {
    from: "2026-09-12T00:00:00Z"
    to: "2026-09-19T23:59:59Z"
    granularity: DAY
  }) {
    success
    statusCode
    message
    timeStamp
    data {
      totalDeliveries
      completedDeliveries
      cancelledDeliveries
      failedDeliveries
      completionRate
      averageDeliveryDurationSeconds
      paymentCapturedAmount
      refundedAmount
      driverAcceptanceRate
      dataAsOf
    }
  }
}

# Delivery Analytics
query GetDeliveryAnalytics {
  deliveryAnalytics(filter: {
    range: {
      from: "2026-09-12T00:00:00Z"
      to: "2026-09-19T23:59:59Z"
      granularity: HOUR
    }
  }) {
    success
    statusCode
    message
    timeStamp
    data {
      total
      completed
      cancelled
      failed
      completionRate
      averageDurationSeconds
      p50DurationSeconds
      p95DurationSeconds
      p99DurationSeconds
      averageAssignmentTimeSeconds
      buckets {
        bucket
        total
        completed
        cancelled
        failed
        avgDurationSeconds
      }
      dataAsOf
    }
  }
}

# Driver Analytics (specific driver)
query GetDriverAnalytics($driverId: ID!) {
  driverAnalytics(filter: {
    range: {
      from: "2026-09-12T00:00:00Z"
      to: "2026-09-19T23:59:59Z"
      granularity: DAY
    }
    driverId: $driverId
  }) {
    success
    statusCode
    message
    timeStamp
    data {
      driverId
      driver {
        id
      }
      offers
      accepted
      rejected
      expired
      acceptanceRate
      averageResponseTimeMs
      completedDeliveries
      dataAsOf
    }
  }
}

# Top Drivers by acceptance rate
query GetTopDrivers {
  topDrivers(range: {
    from: "2026-09-12T00:00:00Z"
    to: "2026-09-19T23:59:59Z"
    granularity: DAY
  }, limit: 10) {
    success
    statusCode
    message
    timeStamp
    data {
      paginationInfo {
        totalItems
        currentPage
        nextPage
      }
      items {
        driverId
        driver {
          id
        }
        offers
        accepted
        acceptanceRate
        completedDeliveries
        dataAsOf
      }
    }
  }
}

# Payment Analytics
query GetPaymentAnalytics {
  paymentAnalytics(filter: {
    range: {
      from: "2026-09-12T00:00:00Z"
      to: "2026-09-19T23:59:59Z"
      granularity: DAY
    }
    provider: "stripe"
  }) {
    success
    statusCode
    message
    timeStamp
    data {
      authorizationCount
      authorizationSuccessRate
      captureCount
      capturedAmount
      refundCount
      refundedAmount
      refundRate
      averageProviderLatencyMs
      dataAsOf
    }
  }
}

# Raw Events (admin/debug)
query GetRawAnalyticsEvents {
  rawAnalyticsEvents(page: 1, limit: 20, eventType: "payment.captured") {
    success
    statusCode
    message
    timeStamp
    data {
      paginationInfo {
        totalItems
        currentPage
        nextPage
      }
      items {
        eventId
        eventType
        eventVersion
        aggregateType
        aggregateId
        producer
        occurredAt
        ingestedAt
        correlationId
        sourceTopic
      }
    }
  }
}

# Data Quality Issues
query GetDataQualityIssues {
  dataQualityIssues(page: 1, limit: 20, severity: "ERROR") {
    success
    statusCode
    message
    timeStamp
    data {
      paginationInfo {
        totalItems
        currentPage
        nextPage
      }
      items {
        issueId
        eventId
        issueType
        aggregateType
        aggregateId
        detectedAt
        severity
        details
      }
    }
  }
}
```

---

### Component 10 — Proto File (Optional gRPC)

#### [NEW] `protos/analytics.proto`
```proto
syntax = "proto3";
package analytics;
option go_package = "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/protos/analytics";

// Future: gRPC for AI service to query analytics
service AnalyticsService {
  rpc GetPlatformOverview(PlatformOverviewRequest) returns (PlatformOverviewResponse);
  rpc GetDriverPerformance(DriverPerformanceRequest) returns (DriverPerformanceResponse);
}

message PlatformOverviewRequest { string from = 1; string to = 2; }
message PlatformOverviewResponse {
  int64  total_deliveries      = 1;
  int64  completed_deliveries  = 2;
  double completion_rate       = 3;
  string data_as_of            = 4;
}
message DriverPerformanceRequest { string driver_id = 1; string from = 2; string to = 3; }
message DriverPerformanceResponse {
  string driver_id           = 1;
  int64  offers              = 2;
  int64  accepted            = 3;
  double acceptance_rate     = 4;
  int64  completed_deliveries = 5;
}
```

---

## Verification Plan

### Build Verification
```bash
cd services/analytics-service && go build ./...
```

### Docker Build
```bash
docker build -f services/analytics-service/Dockerfile -t delivery/analytics .
```

### Skaffold Integration
```bash
skaffold -f infrastructure/skaffold/skaffold.yaml dev --cache-artifacts=true
```

### Manual Verification

1. **Health check:** `GET http://localhost:4009/health/live` → `{"success":true,...}`
2. **GraphQL Info query:** Run `GetAnalyticsServiceInfo` — should return `analyticsServiceInfo` response
3. **Kafka consumption:** Publish a test `payment.captured` event and verify `fact_payment_transactions` in ClickHouse
4. **Idempotency:** Publish same event twice — verify only one row inserted
5. **DLQ:** Publish malformed event — verify it lands in `analytics.dlq` topic
6. **Federation:** Confirm API Gateway resolves `analyticsServiceInfo` query through federation
7. **DataLoader N+1:** Run `topDrivers` and confirm only 1 batch call to Driver gRPC (not N separate calls)

### Automated Tests
```bash
cd services/analytics-service && go test ./...
```

Key test cases:
- Duplicate event → one row in ClickHouse
- Malformed event → DLQ
- Late event → correct `occurred_at` stored
- ClickHouse unavailable → bounded retry, Kafka lag increases, no silent ack
- Consumer restart → resumes from last committed offset

---

## Implementation Order (Phases)

| Phase | Tasks |
|---|---|
| **1 — Foundation** | `go.mod`, `config.go`, `main.go`, `Dockerfile`, health endpoints, i18n |
| **2 — ClickHouse** | Connection, migrations runner, all DDL files |
| **3 — Kafka Consumer** | Multi-topic consumer, envelope validation, event router |
| **4 — Domain Handlers** | delivery/driver/payment/notification handlers, batch writer |
| **5 — Idempotency** | ClickHouse-based dedup, DLQ publisher, retry/backoff |
| **6 — GraphQL API** | SDL, handler.go, resolver.go, dataloader.go |
| **7 — Packages** | analytics_events.go, notification_events.go additions |
| **8 — Infrastructure** | K8s secret, configmap, deployment, ClickHouse depl, kustomization updates, skaffold |
| **9 — API Gateway** | Add analytics subgraph to federation |
| **10 — GraphQL Docs** | analytics.graphql full documentation |
| **11 — Workers** | reconciliation_worker.go, worker pool |
| **12 — Observability** | Prometheus metrics, OTEL tracing, structured logs |
| **13 — Tests** | Unit, integration, failure injection tests |
