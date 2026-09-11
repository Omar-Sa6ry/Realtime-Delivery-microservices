# Payment Service — Implementation Plan

## Overview

إنشاء **Payment Service** كاملة وإنتاجية بلغة **Go** ضمن منصة Realtime Delivery microservices، مع تكاملها مع بقية السيرفيسز عبر **gRPC / Kafka / NATS**، وإنشاء كل ملفات Infrastructure وDocker المطلوبة.

السيرفيس هي **Saga Participant** (وليست Orchestrator) — Delivery Service تنسّق، وPayment تُنفّذ العمليات المالية.

---

## Open Questions

> [!IMPORTANT]
> **Payment Provider:** هل تريد استخدام mock provider فقط للآن (كما هو موضح في architecture)، أم تريد تكاملاً مع provider حقيقي (Stripe مثلاً)؟ الخطة الحالية تتضمن mock provider + interface جاهز للتوسع.

> [!NOTE]
> **NATS Version:** الـ `compose.yml` الحالي يستخدم `nats-streaming:0.17.0` لكن driver-service يستخدم `nats:2-alpine`. الخطة ستستخدم `nats:2-alpine` (JetStream) المتوافق مع بقية السيرفيسز الجديدة.

---

## Proposed Changes

---

### 1. `packages/go` — تحديث الـ shared packages

#### [MODIFY] [payment_events.go](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/packages/go/events/payment_events.go)

إضافة event types وpayloads مكتملة تتوافق مع architecture:

```go
// Event type constants (PaymentEventType)
PaymentCreated                // payment.created
PaymentAuthorizationStarted   // payment.authorization.started
PaymentAuthorized             // payment.authorized
PaymentAuthorizationFailed    // payment.authorization.failed
PaymentCaptureStarted         // payment.capture.started
PaymentCaptured               // payment.captured
PaymentCaptureFailed          // payment.capture.failed
PaymentCancelled              // payment.cancelled
PaymentRefundStarted          // payment.refund.started
PaymentRefunded               // payment.refunded
PaymentRefundFailed           // payment.refund.failed
PaymentFailed                 // payment.failed

// Full payloads: PaymentCreatedPayload, PaymentAuthorizedPayload,
//                PaymentCapturedPayload, PaymentRefundedPayload, etc.
// كل payload تحمل: PaymentID, DeliveryID, UserID, AmountMinor,
//                   Currency, Status, CorrelationID, CausationID
```

---

### 2. `protos/payment.proto` — تحديث الـ gRPC contract

#### [MODIFY] [payment.proto](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/protos/payment.proto)

إعادة كتابة كاملة بـ proper versioning وكل RPCs المطلوبة:

```proto
syntax = "proto3";
package payment.v1;
option go_package = "github.com/.../protos/payment/v1;paymentpb";

service PaymentService {
  rpc CreatePayment(CreatePaymentRequest) returns (CreatePaymentResponse);
  rpc AuthorizePayment(AuthorizePaymentRequest) returns (AuthorizePaymentResponse);
  rpc CapturePayment(CapturePaymentRequest) returns (CapturePaymentResponse);
  rpc CancelAuthorization(CancelAuthorizationRequest) returns (CancelAuthorizationResponse);
  rpc CreateRefund(CreateRefundRequest) returns (CreateRefundResponse);
  rpc GetPayment(GetPaymentRequest) returns (GetPaymentResponse);
  rpc GetPaymentStatus(GetPaymentStatusRequest) returns (GetPaymentStatusResponse);
}
// كل messages تستخدم int64 amount_minor (مش double)
// وتحمل idempotency_key, correlation_id, request_id
```

---

### 3. `services/payment-service/` — الهيكل الكامل للسيرفيس

الهيكل مشابه لـ driver-service مع اختلافات تتناسب مع طبيعة payment:

```text
payment-service/
├── cmd/
│   └── server/
│       └── main.go                    # entry point: wires everything
├── internal/
│   ├── config/
│   │   └── config.go                  # env-based config (port 4002)
│   ├── domain/
│   │   ├── payment.go                 # Payment aggregate + state machine
│   │   ├── refund.go                  # Refund entity + state machine
│   │   ├── attempt.go                 # PaymentAttempt entity
│   │   ├── money.go                   # Money value object (integer minor units)
│   │   ├── errors.go                  # domain errors
│   │   └── state.go                   # state transition constants & validation
│   ├── ports/
│   │   ├── repositories.go            # PaymentRepository, RefundRepository, etc.
│   │   ├── payment_provider.go        # PaymentProvider interface
│   │   ├── publisher.go               # EventPublisher, NATSPublisher ports
│   │   └── idempotency_store.go       # IdempotencyStore port
│   ├── application/
│   │   ├── commands/
│   │   │   ├── create_payment.go
│   │   │   ├── authorize_payment.go
│   │   │   ├── capture_payment.go
│   │   │   ├── cancel_authorization.go
│   │   │   └── create_refund.go
│   │   ├── queries/
│   │   │   ├── get_payment.go
│   │   │   └── get_payment_status.go
│   │   └── services/
│   │       └── payment_service.go     # coordinates repos + provider + outbox
│   ├── adapters/
│   │   ├── postgres/
│   │   │   ├── payment_repo.go        # implements PaymentRepository
│   │   │   ├── refund_repo.go         # implements RefundRepository
│   │   │   ├── attempt_repo.go        # implements AttemptRepository
│   │   │   ├── outbox_repo.go         # implements OutboxRepository
│   │   │   ├── idempotency_repo.go    # implements IdempotencyStore
│   │   │   ├── audit_repo.go          # AuditLog writer
│   │   │   └── transaction.go         # DB transaction helper
│   │   ├── redis/
│   │   │   ├── idempotency_cache.go   # fast-path idempotency via Redis
│   │   │   ├── lock_manager.go        # distributed lock (TTL + owner token)
│   │   │   └── rate_limiter.go        # Redis-backed rate limiting
│   │   ├── kafka/
│   │   │   ├── publisher.go           # OutboxPublisher: reads outbox → Kafka
│   │   │   └── event_publisher.go     # EventPublisherAdapter (port impl)
│   │   ├── nats/
│   │   │   └── realtime_publisher.go  # NATS transient updates (payment.status.updated)
│   │   ├── grpc/
│   │   │   ├── server.go              # gRPC server (PaymentService impl)
│   │   │   └── proto/                 # generated pb files (gitignored or generated)
│   │   ├── providers/
│   │   │   ├── interface.go           # ProviderAdapter (same as ports/payment_provider.go)
│   │   │   ├── mock/
│   │   │   │   └── mock_provider.go   # deterministic mock: success/decline/timeout/unknown
│   │   │   └── errors.go              # normalized error categories
│   │   └── webhook/
│   │       └── handler.go             # HTTP webhook endpoint: verify → deduplicate → process
│   ├── graphql/
│   │   ├── schema.go                  # GraphQL schema (payment subgraph)
│   │   ├── resolver.go                # root resolver
│   │   ├── handler.go                 # HTTP handler + health
│   │   └── dataloader.go             # DataLoader for payment queries
│   ├── workers/
│   │   ├── pool.go                    # WorkerPool (same pattern as driver-service)
│   │   ├── outbox_publisher.go        # polls unpublished outbox → Kafka
│   │   ├── reconciliation.go          # finds UNKNOWN ops → provider lookup → repair
│   │   ├── stuck_recovery.go          # PROCESSING > threshold → UNKNOWN
│   │   └── cleanup.go                 # retention/cleanup for old outbox/idempotency
│   ├── observability/
│   │   ├── metrics.go                 # Prometheus metrics (payment_operations_total, etc.)
│   │   └── tracer.go                  # OpenTelemetry + Jaeger setup
│   └── validation/
│       └── validator.go               # input validation helpers
├── migrations/
│   ├── 001_create_payments.sql
│   ├── 002_create_payment_attempts.sql
│   ├── 003_create_payment_transactions.sql
│   ├── 004_create_refunds.sql
│   ├── 005_create_refund_attempts.sql
│   ├── 006_create_idempotency_keys.sql
│   ├── 007_create_payment_events_outbox.sql
│   ├── 008_create_payment_audit_log.sql
│   └── 009_create_processed_provider_events.sql
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

---

### 4. الملفات التفصيلية للسيرفيس

#### [NEW] `cmd/server/main.go`

يربط كل المكونات:
- PostgreSQL connection + migrations runner
- Redis client
- Kafka producer + EnsureTopics
- NATS connection
- Snowflake ID generator (من packages/go/snowflake)
- Provider adapter (mock للآن)
- All repositories
- All command/query handlers
- WorkerPool: outbox publisher, reconciliation, stuck recovery, cleanup
- gRPC server on port **50056**
- GraphQL HTTP server on port **4002**
- Metrics server on port **9106**
- Webhook HTTP handler
- Graceful shutdown on SIGTERM

#### [NEW] `internal/config/config.go`

```go
type Config struct {
    PortGraphQL          string  // 4002
    PortGRPC             string  // 50056
    PortMetrics          string  // 9106
    PortWebhook          string  // 4012
    PostgresDSN          string
    RedisHost            string
    RedisPort            string
    KafkaBrokers         string
    KafkaGroupID         string
    NATSUrl              string
    DeliveryServiceURL   string  // for future outbound gRPC if needed
    SnowflakeWorkerID    int64   // 2 (each service has unique worker ID)
    ReconcileIntervalSec int
    StuckThresholdSec    int
    OutboxBatchSize      int
    ProviderMockMode     string  // "success" | "decline" | "timeout" | "unknown"
    WebhookSecret        string
    OTELEndpoint         string
}
```

#### [NEW] `internal/domain/payment.go`

```go
// Payment aggregate
type PaymentStatus string
const (
    StatusPending     PaymentStatus = "PENDING"
    StatusAuthorized  PaymentStatus = "AUTHORIZED"
    StatusCaptured    PaymentStatus = "CAPTURED"
    StatusCancelled   PaymentStatus = "CANCELLED"
    StatusFailed      PaymentStatus = "FAILED"
)

type Payment struct {
    ID                   int64           // Snowflake
    DeliveryID           string
    UserID               string
    Currency             string
    AmountMinor          int64           // e.g. 50075 = 500.75 EGP
    Status               PaymentStatus
    Provider             string
    ProviderPaymentID    string
    AuthorizedAmountMinor int64
    CapturedAmountMinor  int64
    RefundedAmountMinor  int64
    PendingRefundMinor   int64
    Version              int64           // optimistic concurrency
    CreatedAt            time.Time
    UpdatedAt            time.Time
    AuthorizedAt         *time.Time
    CapturedAt           *time.Time
    CancelledAt          *time.Time
    FailedAt             *time.Time
}

// State machine methods
func (p *Payment) CanAuthorize() bool
func (p *Payment) CanCapture() bool
func (p *Payment) CanCancel() bool
func (p *Payment) CanRefund(amountMinor int64) bool
func (p *Payment) Authorize(providerID string, amount int64) error
func (p *Payment) Capture(amount int64) error
func (p *Payment) Cancel() error
func (p *Payment) Fail() error
func (p *Payment) ReserveRefund(amount int64) error
func (p *Payment) CommitRefund(amount int64) error
func (p *Payment) ReleaseRefundReservation(amount int64) error
```

#### [NEW] `internal/domain/money.go`

```go
// Money value object — integer minor units only
type Money struct {
    AmountMinor int64
    Currency    string
}
func NewMoney(minor int64, currency string) (Money, error) // validates > 0
func (m Money) Add(other Money) (Money, error)
func (m Money) Sub(other Money) (Money, error)
func (m Money) IsZero() bool
func (m Money) LessThanOrEqual(other Money) bool
```

#### [NEW] `internal/domain/state.go`

```go
// OperationStatus for PaymentAttempt
type OperationStatus string
const (
    OpNotStarted OperationStatus = "NOT_STARTED"
    OpProcessing OperationStatus = "PROCESSING"
    OpSucceeded  OperationStatus = "SUCCEEDED"
    OpFailed     OperationStatus = "FAILED"
    OpUnknown    OperationStatus = "UNKNOWN"
)

// RefundStatus
type RefundStatus string
const (
    RefundPending  RefundStatus = "REFUND_PENDING"
    RefundDone     RefundStatus = "REFUNDED"
    RefundFailed   RefundStatus = "REFUND_FAILED"
)
```

#### [NEW] `internal/ports/payment_provider.go`

```go
type ProviderResult struct {
    ProviderTransactionID string
    ProviderPaymentID     string
    Status                OperationStatus
    RawResponse           map[string]any
}

type NormalizedError struct {
    Category ErrorCategory  // TEMPORARY|PERMANENT|DECLINED|UNKNOWN|RATE_LIMITED|TIMEOUT
    Message  string
    Retryable bool
}

type PaymentProvider interface {
    Authorize(ctx context.Context, req AuthorizeRequest) (*ProviderResult, *NormalizedError)
    Capture(ctx context.Context, req CaptureRequest) (*ProviderResult, *NormalizedError)
    Void(ctx context.Context, req VoidRequest) (*ProviderResult, *NormalizedError)
    Refund(ctx context.Context, req RefundRequest) (*ProviderResult, *NormalizedError)
    GetStatus(ctx context.Context, providerPaymentID string) (*ProviderResult, *NormalizedError)
}
```

---

### 5. `migrations/` — PostgreSQL Schema

```sql
-- 001_create_payments.sql
CREATE TABLE payments (
    id                     BIGINT PRIMARY KEY,  -- Snowflake
    delivery_id            TEXT NOT NULL,
    user_id                TEXT NOT NULL,
    currency               CHAR(3) NOT NULL,
    amount_minor           BIGINT NOT NULL CHECK (amount_minor > 0),
    status                 TEXT NOT NULL DEFAULT 'PENDING',
    payment_method_type    TEXT,
    provider               TEXT NOT NULL DEFAULT 'mock',
    provider_payment_id    TEXT,
    authorized_amount_minor BIGINT NOT NULL DEFAULT 0,
    captured_amount_minor  BIGINT NOT NULL DEFAULT 0,
    refunded_amount_minor  BIGINT NOT NULL DEFAULT 0,
    pending_refund_minor   BIGINT NOT NULL DEFAULT 0,
    version                BIGINT NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    authorized_at          TIMESTAMPTZ,
    captured_at            TIMESTAMPTZ,
    cancelled_at           TIMESTAMPTZ,
    failed_at              TIMESTAMPTZ,
    UNIQUE (delivery_id),
    CONSTRAINT chk_captured_le_authorized CHECK (captured_amount_minor <= authorized_amount_minor),
    CONSTRAINT chk_refunded_le_captured   CHECK (refunded_amount_minor <= captured_amount_minor)
);
CREATE INDEX idx_payments_delivery_id  ON payments(delivery_id);
CREATE INDEX idx_payments_user_created ON payments(user_id, created_at DESC);
CREATE INDEX idx_payments_status       ON payments(status, updated_at);

-- 002-009: payment_attempts, payment_transactions, refunds,
--          refund_attempts, idempotency_keys, payment_events_outbox,
--          payment_audit_log, processed_provider_events
```

---

### 6. `workers/` — Background Workers

| Worker | Interval | الوظيفة |
|--------|----------|---------|
| `outbox_publisher` | 2s | `SELECT ... FOR UPDATE SKIP LOCKED LIMIT N` → Kafka |
| `reconciliation` | 60s | UNKNOWN ops → provider GetStatus → repair state |
| `stuck_recovery` | 30s | PROCESSING > threshold → UNKNOWN |
| `cleanup` | 1h | حذف outbox/idempotency القديمة |

كل worker يستخدم `context.Context` للـ graceful shutdown، و`WorkerPool` نفس pattern driver-service.

---

### 7. `adapters/webhook/handler.go` — Webhook Worker

```go
// HTTP POST /webhook/payment
// 1. قراءة X-Provider-Signature
// 2. HMAC-SHA256 verification
// 3. timestamp replay protection (±5 min)
// 4. INSERT INTO processed_provider_events (UNIQUE constraint → dedup)
// 5. normalize event → update payment state
// 6. outbox insert + audit log
// 7. return 200 OK
```

---

### 8. `internal/graphql/` — Payment Subgraph

```graphql
type Payment {
  id: ID!
  deliveryId: ID!
  amount: Float!         # converted from minor units for display
  currency: String!
  status: PaymentStatus!
  capturedAmount: Float
  refundedAmount: Float
  createdAt: String!
  updatedAt: String!
}

enum PaymentStatus {
  PENDING
  AUTHORIZED
  CAPTURED
  CANCELLED
  FAILED
}

type Query {
  payment(id: ID!): Payment
  paymentStatus(paymentId: ID!): PaymentStatus
}
```

لا نكشف mutations للـ capture مباشرة للـ client (يتم عبر Delivery Saga gRPC).

---

### 9. Infrastructure Files

#### [NEW] `services/payment-service/Dockerfile`

```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY packages/go ./packages/go
COPY services/payment-service ./services/payment-service
WORKDIR /app/services/payment-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" \
    -o /workspace/bin/payment-service ./cmd/server

# Runtime stage
FROM alpine:3.19 AS runtime
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /workspace/bin/payment-service /payment-service
EXPOSE 4002 50056 9106 4012
ENTRYPOINT ["/payment-service"]
```

#### [NEW] `services/payment-service/docker-compose.yml`

يتضمن:
- `payment-db-srv` (postgres:15, port 5436)
- `redis-srv` (مشترك)
- `kafka-srv` (مشترك)
- `nats-srv` (مشترك)
- `payment-service` (port 4002:4002, 50056:50056, 9106:9106, 4012:4012)

#### [MODIFY] `infrastructure/docker/compose.yml`

إضافة:
- `payment-db-srv` (postgres:15, port 5436:5432)
- `payment-service` (port 4002, 50056, 9106, 4012) مع network alias `payment-srv`
- تحديث `api-gateway` env: `PAYMENT_SERVICE_URL: "http://payment-srv:4002/payment/graphql"`
- تحديث `delivery-service` env: `PAYMENT_SERVICE_GRPC_URL: "payment-srv:50056"`

#### [NEW] `infrastructure/kubernetes/payment-service/`

```text
deployment.yaml          # 2 replicas, liveness/readiness probes
service.yaml             # ClusterIP: 4002 (GraphQL), 50056 (gRPC), 9106 (metrics)
configmap.yaml           # non-secret env vars
secret.yaml              # POSTGRES_PASSWORD, WEBHOOK_SECRET
hpa.yaml                 # CPU 70%, min 2 / max 5
pdb.yaml                 # minAvailable: 1
```

#### [NEW] `infrastructure/skaffold/` — تحديث skaffold.yaml

إضافة payment-service artifact + kubernetes manifests.

---

### 10. `packages/go` — تأكيد عدم إضافة business logic

الشيء الوحيد الذي نضيفه/نعدله:

| الملف | التغيير |
|-------|---------|
| `events/payment_events.go` | إضافة event type constants + full payloads |
| `events/events.go` (إن وُجد) | التأكد من EventEnvelope يحمل correlationId + causationId |

---

## Technology Mapping

| الاحتياج | التقنية | الدور |
|----------|---------|-------|
| Public API | GraphQL Federation (port 4002) | payment subgraph |
| Internal commands | gRPC (port 50056) | Delivery → Payment |
| Durable events | Kafka | payment.* events via outbox |
| Transient realtime | NATS | payment.status.updated |
| Local state | PostgreSQL (port 5436) | Source of truth |
| Cache/idempotency fast-path | Redis | Acceleration only |
| IDs | Twitter Snowflake | packages/go/snowflake |
| Background work | Go workers (pool) | outbox, reconciliation, stuck, cleanup |
| Webhooks | HTTP handler | provider async confirmations |
| Metrics | Prometheus (port 9106) | payment_operations_total, etc. |
| Tracing | OpenTelemetry + Jaeger | full Saga trace |
| Logs | structured (slog) | correlation ID في كل log |

---

## Ports Summary

| Port | الاستخدام |
|------|----------|
| **4002** | GraphQL HTTP (payment subgraph) |
| **50056** | gRPC server |
| **9106** | Prometheus metrics |
| **4012** | Webhook HTTP endpoint |
| **5436** | PostgreSQL (host-mapped) |

---

## Verification Plan

### Automated Tests
```bash
# Unit tests (domain layer)
cd services/payment-service && go test ./internal/domain/...

# Build check
go build ./...

# Docker build
docker build -f services/payment-service/Dockerfile -t payment-service:local .
```

### Manual Verification
1. `docker-compose -f services/payment-service/docker-compose.yml up`
2. gRPC call `CreatePayment` → `AuthorizePayment` → `CapturePayment`
3. تحقق من outbox → Kafka → events
4. تحقق من Prometheus metrics على `localhost:9106/metrics`
5. تحقق من GraphQL على `localhost:4002/payment/graphql`

---

## Shared Package Changes Summary

```diff
// packages/go/events/payment_events.go
+ type PaymentEventType string
+ const (PaymentCreated, PaymentAuthorized, PaymentCaptured, ...)
+ type PaymentCreatedPayload struct { ... CorrelationID, CausationID ... }
+ type PaymentAuthorizedPayload struct { ... }
+ type PaymentCapturedPayload struct { ... }
+ type PaymentRefundedPayload struct { ... } // extended existing
+ type PaymentAuthorizationFailedPayload struct { ... }
+ type PaymentCaptureFailedPayload struct { ... }
+ type PaymentCancelledPayload struct { ... }

// protos/payment.proto
- (minimal, incomplete)
+ (full versioned gRPC contract with all 7 RPCs)
```
