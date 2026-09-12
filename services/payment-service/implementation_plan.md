# Payment Service — Implementation Plan (v2 · Stripe · Full Detail)

## Overview

إنشاء **Payment Service** كاملة بلغة **Go** على بورت **4002**، مع Stripe كـ real payment provider، هيكل ملفات يشبه **media-service** (الأفضل هيكلاً)، واستخدام كل الـ shared packages الموجودة في `packages/go`.

**ما هو موجود بالفعل** (لن يُلمس):
- `Dockerfile` ✅
- `docker-compose.yml` ✅  
- `go.mod` (يحتاج تحديث فقط)
- `infrastructure/kubernetes/base/services/payment-depl.yaml` ✅
- `infrastructure/kubernetes/base/infrastructure/payment-db-depl.yaml` ✅
- `infrastructure/kubernetes/base/configmaps/payment-service-config.yaml` ✅
- `infrastructure/kubernetes/base/secrets/payment-secret.yaml` ✅ (Stripe keys موجودة)
- `infrastructure/kubernetes/base/kustomization.yaml` ✅ (payment مضاف)
- `infrastructure/skaffold/skaffold.yaml` ✅ (payment artifact مضاف)
- `protos/payment.proto` ✅ (7 RPCs كاملة)

---

## الـ Stripe Secrets الموجودة

من `payment-secret.yaml`:
```
STRIPE_SECRET_KEY      → sk_test_51TE2RVA...  (Stripe API Key)
STRIPE_WEBHOOK_SECRET  → whsec_de4d23...      (Webhook Signature)
STRIPE_WEBHOOK_SECRETSUCCESSURL → http://localhost:4002/payment/success
STRIPE_WEBHOOK_SECRETFAILURL    → http://localhost:4002/payment/cancel
```

---

## File Structure النهائي

مستوحى من **media-service** (الأكثر اكتمالاً):

```text
services/payment-service/
├── cmd/
│   └── server/
│       └── main.go                              # [NEW] entry point
├── internal/
│   ├── config/
│   │   └── config.go                            # [NEW] env config
│   ├── domain/
│   │   ├── payment.go                           # [NEW] Payment aggregate + state machine
│   │   ├── refund.go                            # [NEW] Refund entity
│   │   ├── attempt.go                           # [NEW] PaymentAttempt
│   │   ├── money.go                             # [NEW] Money value object (integer minor)
│   │   ├── errors.go                            # [NEW] domain errors
│   │   └── state.go                             # [NEW] status constants & transitions
│   ├── ports/
│   │   ├── repositories.go                      # [NEW] PaymentRepo, RefundRepo, etc.
│   │   ├── payment_provider.go                  # [NEW] PaymentProvider interface
│   │   ├── publisher.go                         # [NEW] EventPublisher + NATSPublisher
│   │   └── idempotency_store.go                 # [NEW] IdempotencyStore
│   ├── application/
│   │   ├── commands/
│   │   │   ├── create_payment.go                # [NEW]
│   │   │   ├── authorize_payment.go             # [NEW]
│   │   │   ├── capture_payment.go               # [NEW]
│   │   │   ├── cancel_authorization.go          # [NEW]
│   │   │   └── create_refund.go                 # [NEW]
│   │   ├── queries/
│   │   │   ├── get_payment.go                   # [NEW]
│   │   │   └── get_payment_status.go            # [NEW]
│   │   └── services/
│   │       └── payment_service.go               # [NEW] orchestrates all
│   ├── adapters/
│   │   ├── postgres/
│   │   │   ├── payment_repo.go                  # [NEW] PaymentRepository impl
│   │   │   ├── refund_repo.go                   # [NEW]
│   │   │   ├── attempt_repo.go                  # [NEW]
│   │   │   ├── outbox_repo.go                   # [NEW] FOR UPDATE SKIP LOCKED
│   │   │   ├── idempotency_repo.go              # [NEW]
│   │   │   ├── audit_repo.go                    # [NEW]
│   │   │   └── db.go                            # [NEW] DB init + migrations runner
│   │   ├── redis/
│   │   │   ├── idempotency_cache.go             # [NEW] fast-path idempotency
│   │   │   └── lock_manager.go                  # [NEW] distributed locks (TTL)
│   │   ├── kafka/
│   │   │   ├── publisher.go                     # [NEW] outbox → Kafka producer
│   │   │   └── event_publisher.go               # [NEW] EventPublisher port impl
│   │   ├── nats/
│   │   │   └── realtime_publisher.go            # [NEW] payment.status.updated
│   │   ├── grpc/
│   │   │   ├── server.go                        # [NEW] gRPC server impl (7 RPCs)
│   │   │   └── proto/                           # [NEW] generated .pb.go files
│   │   ├── providers/
│   │   │   ├── interface.go                     # [NEW] (mirrors ports/payment_provider.go)
│   │   │   ├── errors.go                        # [NEW] normalized error categories
│   │   │   └── stripe/
│   │   │       └── stripe_provider.go           # [NEW] Stripe adapter (real)
│   │   └── webhook/
│   │       └── handler.go                       # [NEW] Stripe webhook endpoint
│   ├── graphql/
│   │   ├── schema_sdl.go                        # [NEW] payment subgraph SDL
│   │   ├── resolver.go                          # [NEW] root resolver
│   │   ├── handler.go                           # [NEW] HTTP handler + health endpoints
│   │   └── dataloader.go                        # [NEW] DataLoader (N+1 prevention)
│   ├── observability/
│   │   ├── metrics.go                           # [NEW] payment-specific Prometheus metrics
│   │   └── tracer.go                            # [NEW] OpenTelemetry + Jaeger setup
│   ├── workers/
│   │   ├── pool.go                              # [NEW] WorkerPool (same as driver-service)
│   │   ├── outbox_publisher.go                  # [NEW] polls outbox → Kafka
│   │   ├── reconciliation.go                    # [NEW] UNKNOWN → provider query → fix
│   │   ├── stuck_recovery.go                    # [NEW] PROCESSING > threshold → UNKNOWN
│   │   └── cleanup.go                           # [NEW] old outbox/idempotency GC
│   └── validation/
│       └── validator.go                         # [NEW] input validation helpers
├── migrations/
│   ├── 001_create_payments.sql                  # [NEW]
│   ├── 002_create_payment_attempts.sql          # [NEW]
│   ├── 003_create_payment_transactions.sql      # [NEW]
│   ├── 004_create_refunds.sql                   # [NEW]
│   ├── 005_create_refund_attempts.sql           # [NEW]
│   ├── 006_create_idempotency_keys.sql          # [NEW]
│   ├── 007_create_payment_events_outbox.sql     # [NEW]
│   ├── 008_create_payment_audit_log.sql         # [NEW]
│   └── 009_create_processed_provider_events.sql # [NEW]
├── Dockerfile                                   # ✅ موجود
├── docker-compose.yml                           # ✅ موجود
├── go.mod                                       # [MODIFY] إضافة dependencies
└── go.sum                                       # [AUTO] بعد go mod tidy
```

---

## الملفات بالتفصيل والترتيب

---

### STEP 1 — `go.mod` تحديث

#### [MODIFY] [go.mod](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/services/payment-service/go.mod)

يحتاج إضافة كل dependencies المطلوبة + استبدال module name:

```
module github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service

require:
  github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go  → shared packages
  github.com/graph-gophers/graphql-go                                 → GraphQL
  github.com/graph-gophers/dataloader/v7                              → DataLoader (N+1)
  github.com/stripe/stripe-go/v78                                     → Stripe SDK
  github.com/lib/pq  OR  github.com/jackc/pgx/v5                     → PostgreSQL driver
  github.com/redis/go-redis/v9                                         → Redis
  github.com/segmentio/kafka-go                                        → Kafka
  github.com/nats-io/nats.go                                           → NATS
  github.com/prometheus/client_golang                                  → Prometheus
  go.opentelemetry.io/otel + exporters                                → OTel/Jaeger
  google.golang.org/grpc + protobuf                                   → gRPC
  golang.org/x/sync                                                    → errgroup

replace github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go => ../../packages/go
```

---

### STEP 2 — Shared Packages المستخدمة

> [!IMPORTANT]
> لا يتم نسخ أي كود من packages. فقط import مباشر.

| Package | الاستخدام في payment-service |
|---------|-------------------------------|
| `packages/go/logging` | `logging.InitLogger()` في `main.go` — structured slog |
| `packages/go/metrics` | `metrics.StartMetricsServer()` + `metrics.UnaryServerMetricsInterceptor()` للـ gRPC |
| `packages/go/metrics` | `metrics.HTTPHandler()` للـ /metrics endpoint |
| `packages/go/kafka` | `kafka.NewProducer()` في outbox publisher + `kafka.EnsureTopics()` |
| `packages/go/nats` | `nats.Connect()` + `nats.PublishNestJS()` للـ realtime updates |
| `packages/go/snowflake` | `snowflake.Generate()` لكل الـ IDs (Payment, Refund, Attempt) |
| `packages/go/auth` | `auth.Authenticate()` في GraphQL middleware |
| `packages/go/events` | `events.EventEnvelope` + `events.PaymentEventType` في outbox |
| `packages/go/middleware` | `middleware.UnaryServerMetricsInterceptor` للـ gRPC |

#### [MODIFY] [payment_events.go](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/packages/go/events/payment_events.go)

إضافة event type constants وpayloads مكتملة (الملف الحالي ناقص):

```go
type PaymentEventType string

const (
    PaymentCreated                PaymentEventType = "payment.created"
    PaymentAuthorizationStarted   PaymentEventType = "payment.authorization.started"
    PaymentAuthorized             PaymentEventType = "payment.authorized"
    PaymentAuthorizationFailed    PaymentEventType = "payment.authorization.failed"
    PaymentCaptureStarted         PaymentEventType = "payment.capture.started"
    PaymentCaptured               PaymentEventType = "payment.captured"
    PaymentCaptureFailed          PaymentEventType = "payment.capture.failed"
    PaymentCancelled              PaymentEventType = "payment.cancelled"
    PaymentRefundStarted          PaymentEventType = "payment.refund.started"
    PaymentRefunded               PaymentEventType = "payment.refunded"
    PaymentRefundFailed           PaymentEventType = "payment.refund.failed"
    PaymentFailed                 PaymentEventType = "payment.failed"
)

// Enriched payloads (كل منها يحمل CorrelationID + CausationID)
type PaymentCreatedPayload struct { ... }
type PaymentAuthorizedPayload struct { ... }
type PaymentCapturedPayload struct { ... }
type PaymentCancelledPayload struct { ... }
type PaymentAuthorizationFailedPayload struct { ... }
type PaymentCaptureFailedPayload struct { ... }
// PaymentRefundedPayload موجودة بالفعل - ستُحدَّث فقط
```

---

### STEP 3 — `internal/config/config.go`

#### [NEW] [config.go](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/services/payment-service/internal/config/config.go)

```go
type Config struct {
    PortGraphQL          string  // 4002
    PortGRPC             string  // 50056
    PortMetrics          string  // 9106
    PortWebhook          string  // 4012
    PostgresDSN          string  // POSTGRES_DSN
    RedisHost            string
    RedisPort            string
    KafkaBrokers         string
    KafkaGroupID         string
    KafkaClientID        string
    NATSUrl              string
    SnowflakeWorkerID    int64   // 2
    ReconcileIntervalSec int     // 60
    StuckThresholdSec    int     // 30
    OutboxBatchSize      int     // 100
    // Stripe
    StripeSecretKey       string  // STRIPE_SECRET_KEY (from secret)
    StripeWebhookSecret   string  // STRIPE_WEBHOOK_SECRET
    StripeSuccessURL      string  // STRIPE_WEBHOOK_SECRETSUCCESSURL
    StripeFailURL         string  // STRIPE_WEBHOOK_SECRETFAILURL
    // Observability
    OTELEndpoint         string
    // Internal gRPC clients
    DeliveryServiceURL   string
}
```

---

### STEP 4 — Domain Layer

#### [NEW] `internal/domain/money.go`
```go
// Money — integer minor units ONLY (never float)
type Money struct { AmountMinor int64; Currency string }
func NewMoney(minor int64, currency string) (Money, error)  // validates > 0
func (m Money) Add(other Money) (Money, error)
func (m Money) Sub(other Money) (Money, error)
func (m Money) IsZero() bool
func (m Money) LessThanOrEqual(other Money) bool
```

#### [NEW] `internal/domain/state.go`
```go
type PaymentStatus string
const (
    StatusPending    PaymentStatus = "PENDING"
    StatusAuthorized PaymentStatus = "AUTHORIZED"
    StatusCaptured   PaymentStatus = "CAPTURED"
    StatusCancelled  PaymentStatus = "CANCELLED"
    StatusFailed     PaymentStatus = "FAILED"
)

type OperationStatus string
const (
    OpNotStarted OperationStatus = "NOT_STARTED"
    OpProcessing OperationStatus = "PROCESSING"
    OpSucceeded  OperationStatus = "SUCCEEDED"
    OpFailed     OperationStatus = "FAILED"
    OpUnknown    OperationStatus = "UNKNOWN"   // ← critical
)

type RefundStatus string
const (
    RefundNotRequested RefundStatus = "NOT_REFUNDED"
    RefundPending      RefundStatus = "REFUND_PENDING"
    RefundDone         RefundStatus = "REFUNDED"
    RefundFailed       RefundStatus = "REFUND_FAILED"
)

// AllowedTransitions map — state machine rules
```

#### [NEW] `internal/domain/payment.go`
```go
type Payment struct {
    ID                    int64          // Snowflake via packages/go/snowflake
    DeliveryID            string
    UserID                string
    Currency              string
    AmountMinor           int64
    Status                PaymentStatus
    Provider              string         // "stripe"
    ProviderPaymentID     string         // Stripe PaymentIntent ID
    AuthorizedAmountMinor int64
    CapturedAmountMinor   int64
    RefundedAmountMinor   int64
    PendingRefundMinor    int64
    Version               int64          // optimistic concurrency
    CreatedAt             time.Time
    UpdatedAt             time.Time
    AuthorizedAt          *time.Time
    CapturedAt            *time.Time
    CancelledAt           *time.Time
    FailedAt              *time.Time
}

// State machine guard methods
func (p *Payment) CanAuthorize() bool
func (p *Payment) CanCapture() bool
func (p *Payment) CanCancel() bool
func (p *Payment) CanRefund(amountMinor int64) bool

// Transition methods (return error on invalid)
func (p *Payment) Authorize(providerID string, authorizedAmount int64) error
func (p *Payment) Capture(capturedAmount int64) error
func (p *Payment) Cancel() error
func (p *Payment) Fail() error
func (p *Payment) ReserveRefund(amount int64) error
func (p *Payment) CommitRefund(amount int64) error
func (p *Payment) ReleaseRefundReservation(amount int64) error
```

#### [NEW] `internal/domain/attempt.go`
```go
type PaymentAttempt struct {
    ID                      int64            // Snowflake
    PaymentID               int64
    AttemptNumber           int
    Operation               string           // AUTHORIZE|CAPTURE|VOID|REFUND
    Status                  OperationStatus
    Provider                string
    ProviderRequestID       string
    ProviderTransactionID   string
    ProviderIdempotencyKey  string           // reused across retries!
    ErrorCode               string
    ErrorCategory           string
    SafeErrorMessage        string
    StartedAt               time.Time
    CompletedAt             *time.Time
    CreatedAt               time.Time
}
```

#### [NEW] `internal/domain/refund.go`
```go
type Refund struct {
    ID               int64         // Snowflake
    PaymentID        int64
    DeliveryID       string
    AmountMinor      int64
    Currency         string
    Status           RefundStatus
    Reason           string
    ProviderRefundID string        // Stripe refund ID
    IdempotencyKey   string
    CreatedAt        time.Time
    UpdatedAt        time.Time
    CompletedAt      *time.Time
}
```

#### [NEW] `internal/domain/errors.go`
```go
var (
    ErrPaymentNotFound        = errors.New("payment not found")
    ErrInvalidTransition      = errors.New("invalid payment state transition")
    ErrInsufficientBalance    = errors.New("refund exceeds captured balance")
    ErrDuplicateIdempotency   = errors.New("duplicate idempotency key with different payload")
    ErrProviderUnknownOutcome = errors.New("provider outcome unknown — requires reconciliation")
    ErrAmountMustBePositive   = errors.New("amount must be positive")
    ErrCurrencyMismatch       = errors.New("currency mismatch")
)
```

---

### STEP 5 — Ports

#### [NEW] `internal/ports/repositories.go`
```go
type PaymentRepository interface {
    Create(ctx, *domain.Payment) error
    FindByID(ctx, id int64) (*domain.Payment, error)
    FindByDeliveryID(ctx, deliveryID string) (*domain.Payment, error)
    UpdateConditional(ctx, *domain.Payment, expectedVersion int64) error  // optimistic lock
}

type RefundRepository interface {
    Create(ctx, *domain.Refund) error
    FindByID(ctx, id int64) (*domain.Refund, error)
    FindByPaymentID(ctx, paymentID int64) ([]*domain.Refund, error)
    UpdateStatus(ctx, id int64, status domain.RefundStatus) error
}

type AttemptRepository interface {
    Create(ctx, *domain.PaymentAttempt) error
    FindByPaymentID(ctx, paymentID int64) ([]*domain.PaymentAttempt, error)
    UpdateStatus(ctx, id int64, status domain.OperationStatus, txID string) error
}

type OutboxRepository interface {
    Insert(ctx, tx, eventType string, payload []byte) error    // within same DB tx
    FetchUnpublished(ctx, limit int) ([]*OutboxEvent, error)   // FOR UPDATE SKIP LOCKED
    MarkPublished(ctx, id int64) error
}

type IdempotencyStore interface {
    CheckOrCreate(ctx, key, operation, requestHash string) (*IdempotencyRecord, bool, error)
    Complete(ctx, key string, responsePayload []byte) error
}

type AuditLogRepository interface {
    Log(ctx, tx, paymentID int64, operation, details string) error
}
```

#### [NEW] `internal/ports/payment_provider.go`
```go
type ErrorCategory string
const (
    ErrCategoryTemporary         ErrorCategory = "TEMPORARY"
    ErrCategoryPermanent         ErrorCategory = "PERMANENT"
    ErrCategoryDeclined          ErrorCategory = "DECLINED"
    ErrCategoryAuthError         ErrorCategory = "AUTHENTICATION_ERROR"
    ErrCategoryRateLimited       ErrorCategory = "RATE_LIMITED"
    ErrCategoryTimeout           ErrorCategory = "TIMEOUT"
    ErrCategoryUnknown           ErrorCategory = "UNKNOWN"
)

type ProviderResult struct {
    ProviderTransactionID string
    ProviderPaymentID     string
    Status                domain.OperationStatus
    CheckoutURL           string      // Stripe Checkout URL if applicable
    ClientSecret          string      // Stripe PaymentIntent client_secret
}

type NormalizedError struct {
    Category  ErrorCategory
    Message   string
    Retryable bool
}

type AuthorizeRequest struct {
    PaymentID         int64
    AmountMinor       int64
    Currency          string
    IdempotencyKey    string
    Description       string
    SuccessURL        string
    CancelURL         string
}

type CaptureRequest  struct { ProviderPaymentID string; AmountMinor int64; IdempotencyKey string }
type VoidRequest     struct { ProviderPaymentID string; IdempotencyKey string }
type RefundRequest   struct { ProviderPaymentID string; AmountMinor int64; Reason string; IdempotencyKey string }

type PaymentProvider interface {
    Authorize(ctx context.Context, req AuthorizeRequest) (*ProviderResult, *NormalizedError)
    Capture(ctx context.Context, req CaptureRequest) (*ProviderResult, *NormalizedError)
    Void(ctx context.Context, req VoidRequest) (*ProviderResult, *NormalizedError)
    Refund(ctx context.Context, req RefundRequest) (*ProviderResult, *NormalizedError)
    GetStatus(ctx context.Context, providerPaymentID string) (*ProviderResult, *NormalizedError)
}
```

---

### STEP 6 — Stripe Provider Adapter

#### [NEW] `internal/adapters/providers/stripe/stripe_provider.go`

يستخدم `github.com/stripe/stripe-go/v78`:

```go
// يقوم بـ:
// Authorize → stripe.PaymentIntentParams{} → returns ClientSecret + PI ID
// Capture   → stripe.PaymentIntentCaptureParams{}
// Void      → stripe.PaymentIntentCancelParams{}
// Refund    → stripe.RefundParams{}
// GetStatus → stripe.PaymentIntentGet() → normalize to OperationStatus

// Error normalization:
// stripe.ErrorCodeCardDeclined     → DECLINED (not retryable)
// stripe.ErrorCodeRateLimitError   → RATE_LIMITED (retryable)
// stripe.ErrorCodeAPIConnectionError → TEMPORARY (retryable)
// context.DeadlineExceeded         → TIMEOUT → UNKNOWN (reconcile!)
// stripe.ErrorTypeInvalidRequest   → PERMANENT (not retryable)
```

#### [NEW] `internal/adapters/providers/errors.go`

```go
// NormalizeStripeError(err error) *NormalizedError
// Maps stripe.Error → ErrorCategory with Retryable flag
```

---

### STEP 7 — PostgreSQL Adapter

#### [NEW] `internal/adapters/postgres/db.go`

```go
// func Connect(dsn string) (*sql.DB, error)
// func RunMigrations(db *sql.DB, migrationsDir string) error
//   → reads migrations/*.sql in order, runs them in transactions
```

#### [NEW] `internal/adapters/postgres/payment_repo.go`

يُنفّذ `ports.PaymentRepository`:
- `Create`: INSERT using Snowflake ID
- `FindByID`: SELECT + scan
- `FindByDeliveryID`: للـ gRPC clients
- `UpdateConditional`: `UPDATE ... WHERE id=$1 AND version=$2 AND status=$3` — **optimistic concurrency**

#### باقي repos بنفس النمط...

---

### STEP 8 — Redis Adapter

#### [NEW] `internal/adapters/redis/idempotency_cache.go`

```go
// Redis fast-path: SET NX with TTL
// key: "idempotency:{key}:{operation}"
// value: JSON of IdempotencyRecord
// TTL: 24h
// Fall back to PostgreSQL if Redis unavailable
```

#### [NEW] `internal/adapters/redis/lock_manager.go`

```go
// Redis distributed lock for refund reservation:
// SET "lock:refund:{paymentID}" {ownerToken} NX EX 30s
// SafeRelease: only if ownerToken matches (Lua script)
// NOT used for payment state transitions (PostgreSQL handles that)
```

---

### STEP 9 — Kafka Adapter

يستخدم `packages/go/kafka` مباشرة:

#### [NEW] `internal/adapters/kafka/event_publisher.go`

```go
// Implements ports.EventPublisher
// Uses kafka.NewProducer(brokers)
// PublishEvent(ctx, topic, key, eventType, payload)
```

#### [NEW] `internal/adapters/kafka/publisher.go`

```go
// OutboxKafkaPublisher:
// NewProducer + wraps outbox polling
// Used by OutboxPublisherWorker
```

---

### STEP 10 — NATS Adapter

يستخدم `packages/go/nats` مباشرة:

#### [NEW] `internal/adapters/nats/realtime_publisher.go`

```go
// Uses pkgnats.Connect(url).PublishNestJS(pattern, data)
// Subjects:
//   "payment.status.updated" → Realtime Service → WebSocket → Client
// Pattern يتبع NestJS envelope format (PublishNestJS)
// Fire-and-forget: اگر NATS down مش مشكلة (transient only)
```

---

### STEP 11 — gRPC Server

#### [NEW] `internal/adapters/grpc/server.go`

```go
// Implements PaymentServiceServer from generated proto
// 7 RPCs:
// - CreatePayment   → cmd.CreatePaymentHandler
// - AuthorizePayment → cmd.AuthorizePaymentHandler
// - CapturePayment  → cmd.CapturePaymentHandler
// - CancelAuthorization → cmd.CancelAuthorizationHandler
// - CreateRefund    → cmd.CreateRefundHandler
// - GetPayment      → qry.GetPaymentHandler
// - GetPaymentStatus → qry.GetPaymentStatusHandler

// Interceptors:
// - metrics.UnaryServerMetricsInterceptor() من packages/go/metrics
// - middleware.UnaryServerLoggingInterceptor() لو موجود في packages/go/middleware
// - correlation ID propagation من metadata

func NewGRPCServer(paySvc *services.PaymentService) *grpc.Server
func (s *server) Start(lis net.Listener) error
func (s *server) Stop()
```

#### [NEW] `internal/adapters/grpc/proto/`

نتائج `protoc` من `protos/payment.proto`:
- `payment.pb.go`
- `payment_grpc.pb.go`

---

### STEP 12 — Webhook Handler (Stripe)

#### [NEW] `internal/adapters/webhook/handler.go`

```go
// HTTP POST /webhook/payment (port 4012)
// 1. io.ReadAll(body)
// 2. stripe.ConstructEvent(body, signatureHeader, webhookSecret)
//    → ✅ Stripe signature verification (HMAC-SHA256)
// 3. timestamp replay protection (Stripe handles this automatically)
// 4. event.Type switch:
//    - "payment_intent.succeeded"  → capture flow
//    - "payment_intent.payment_failed" → fail flow
//    - "charge.refunded"           → refund confirmation
// 5. INSERT INTO processed_provider_events (UNIQUE constraint = dedup)
// 6. normalize → update payment state in DB + audit + outbox
// 7. return 200 OK
//
// لازم تكون fast (Stripe expects < 30s response)
// Processing يتم async لو استلزم الأمر
```

---

### STEP 13 — Application Layer

#### [NEW] `internal/application/services/payment_service.go`

```go
// PaymentService orchestrates:
// - idempotency check (Redis fast-path → PostgreSQL fallback)
// - load payment
// - validate state transition
// - mark attempt PROCESSING (short transaction)
// - call provider (Stripe) — OUTSIDE transaction
// - update state + outbox + audit (short transaction)
// - idempotency complete
// - NATS realtime notification
```

#### [NEW] Command Handlers (5 files):

**`create_payment.go`**:
- Check idempotency
- Validate input (amount > 0, currency not empty)
- Create Payment record (PENDING) via Snowflake ID
- Insert outbox event `payment.created`
- Return payment_id + gateway_reference (Stripe checkout URL if applicable)

**`authorize_payment.go`**:
- Load payment → `CanAuthorize()`
- Create attempt record (PROCESSING)
- Call `stripe.Authorize()` — outside DB lock
- On success: `payment.Authorize()` + attempt SUCCEEDED + outbox `payment.authorized`
- On timeout: attempt UNKNOWN → reconciliation will fix
- On decline: attempt FAILED + payment FAILED + outbox `payment.authorization.failed`

**`capture_payment.go`**:
- Load payment → `CanCapture()`
- Create attempt PROCESSING
- `stripe.Capture()` — outside lock
- On success: `payment.Capture()` + outbox `payment.captured`
- On unknown: UNKNOWN → reconciliation

**`cancel_authorization.go`**:
- Load payment → `CanCancel()`
- `stripe.Void()`
- `payment.Cancel()` + outbox `payment.cancelled`

**`create_refund.go`**:
- Lock payment row → `ReserveRefund(amount)` → commit reservation
- `stripe.Refund()` — outside lock
- On success: `CommitRefund()` + outbox `payment.refunded`
- On failure: `ReleaseRefundReservation()` + outbox `payment.refund.failed`

---

### STEP 14 — GraphQL Subgraph

#### [NEW] `internal/graphql/schema_sdl.go`

```graphql
# Payment subgraph - federated schema
type Payment @key(fields: "id") {
  id: ID!
  deliveryId: ID!
  userId: ID!
  amount: Float!        # display only — amountMinor / 100
  currency: String!
  status: PaymentStatus!
  capturedAmount: Float
  refundedAmount: Float
  checkoutUrl: String   # Stripe checkout URL (PENDING state)
  createdAt: String!
  updatedAt: String!
}

type Refund {
  id: ID!
  paymentId: ID!
  amount: Float!
  status: String!
  reason: String
  createdAt: String!
}

enum PaymentStatus { PENDING AUTHORIZED CAPTURED CANCELLED FAILED }

type Query {
  payment(id: ID!): Payment
  paymentStatus(paymentId: ID!): PaymentStatus
  paymentsByDelivery(deliveryId: ID!): Payment
  myPayments(page: Int, limit: Int): [Payment!]!
}
```

#### [NEW] `internal/graphql/dataloader.go`

```go
// DataLoader لمنع N+1
// Loader: BatchPaymentsByIDs
//   - يجمع كل IDs في request واحد
//   - SELECT * FROM payments WHERE id = ANY($1)
//   - يوزّع النتائج
//
// استخدام graph-gophers/dataloader/v7 (نفس driver-service)
//
// نوع الـ loader:
type Loaders struct {
    PaymentByID  *dataloader.Loader[string, *domain.Payment]
    RefundsByPaymentID *dataloader.Loader[string, []*domain.Refund]
}
func NewLoaders(paymentRepo ports.PaymentRepository) *Loaders
```

#### [NEW] `internal/graphql/handler.go`

```go
// - auth middleware: packages/go/auth.Authenticate()
// - inject Loaders in context
// - correlation ID injection
// - /payment/graphql → GraphQL endpoint
// - /graphql → same
// - /health/live → 200 OK
// - /health/ready → checks DB + Redis connectivity
// - /webhook/payment → Stripe webhook (روته من main.go)
```

---

### STEP 15 — Observability

#### [NEW] `internal/observability/metrics.go`

**Payment-specific metrics** فوق الـ shared `packages/go/metrics`:

```go
var (
    // عمليات الدفع
    PaymentOperationsTotal     *prometheus.CounterVec   // label: operation, status
    PaymentOperationDuration   *prometheus.HistogramVec // label: operation
    // Provider
    ProviderRequestsTotal      *prometheus.CounterVec   // label: provider, operation
    ProviderErrorsTotal        *prometheus.CounterVec   // label: provider, category
    ProviderLatencySeconds     *prometheus.HistogramVec // label: provider, operation
    // Unknown/Reconciliation
    UnknownOperationsTotal     *prometheus.CounterVec
    ReconciliationPending      *prometheus.GaugeVec
    // Outbox
    OutboxPendingGauge         prometheus.Gauge
    // Webhooks
    WebhookTotal               *prometheus.CounterVec   // label: event_type, status
    IdempotencyConflictsTotal  *prometheus.CounterVec
    DLQTotal                   *prometheus.CounterVec
)
```

#### [NEW] `internal/observability/tracer.go`

```go
// OpenTelemetry setup (Jaeger exporter)
// Uses OTEL_ENDPOINT env var
// Fallback: noop tracer if endpoint empty
// Spans:
//   payment.create, payment.authorize, payment.capture
//   payment.refund, provider.stripe.authorize, provider.stripe.capture
//   outbox.publish, webhook.process, reconcile.check
```

---

### STEP 16 — Workers

#### [NEW] `internal/workers/pool.go`

نفس pattern driver-service:
```go
type WorkerPool struct { wg sync.WaitGroup; sem chan struct{} }
func NewWorkerPool(n int) *WorkerPool
func (wp *WorkerPool) Submit(f func())
func (wp *WorkerPool) Wait()
```

#### [NEW] `internal/workers/outbox_publisher.go`

```go
// يشتغل كل 2 ثانية
// SELECT ... FROM payment_events_outbox
//   WHERE published_at IS NULL
//   ORDER BY occurred_at
//   FOR UPDATE SKIP LOCKED
//   LIMIT 100
// لكل record: publish to Kafka via packages/go/kafka
// On success: UPDATE published_at = NOW()
// Handles: duplicate publish (Kafka at-least-once → consumer idempotency)
```

#### [NEW] `internal/workers/reconciliation.go`

```go
// يشتغل كل 60 ثانية
// Find UNKNOWN operations
// Call stripe.GetStatus(providerPaymentID)
// On SUCCEEDED: fix local state + outbox
// On FAILED: mark FAILED + outbox
// On still unknown: retry later (exponential backoff)
// Bounded workload: max 50 per run
```

#### [NEW] `internal/workers/stuck_recovery.go`

```go
// يشتغل كل 30 ثانية
// Find attempts WHERE status=PROCESSING AND started_at < NOW() - threshold
// Move to UNKNOWN (NOT failed!)
// ReconciliationWorker will pick them up
```

#### [NEW] `internal/workers/cleanup.go`

```go
// يشتغل كل ساعة
// DELETE FROM payment_events_outbox WHERE published_at < NOW() - 7 days
// DELETE FROM idempotency_keys WHERE expires_at < NOW()
// DELETE FROM processed_provider_events WHERE created_at < NOW() - 30 days
```

---

### STEP 17 — Migrations

#### [NEW] `migrations/001_create_payments.sql`

```sql
CREATE TABLE payments (
    id                      BIGINT PRIMARY KEY,
    delivery_id             TEXT NOT NULL UNIQUE,
    user_id                 TEXT NOT NULL,
    currency                CHAR(3) NOT NULL,
    amount_minor            BIGINT NOT NULL CHECK (amount_minor > 0),
    status                  TEXT NOT NULL DEFAULT 'PENDING',
    payment_method_type     TEXT,
    provider                TEXT NOT NULL DEFAULT 'stripe',
    provider_payment_id     TEXT,          -- Stripe PaymentIntent ID
    authorized_amount_minor BIGINT NOT NULL DEFAULT 0,
    captured_amount_minor   BIGINT NOT NULL DEFAULT 0,
    refunded_amount_minor   BIGINT NOT NULL DEFAULT 0,
    pending_refund_minor    BIGINT NOT NULL DEFAULT 0,
    version                 BIGINT NOT NULL DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    authorized_at           TIMESTAMPTZ,
    captured_at             TIMESTAMPTZ,
    cancelled_at            TIMESTAMPTZ,
    failed_at               TIMESTAMPTZ,
    CONSTRAINT chk_captured_lte_authorized CHECK (captured_amount_minor <= authorized_amount_minor),
    CONSTRAINT chk_refunded_lte_captured   CHECK (refunded_amount_minor <= captured_amount_minor),
    CONSTRAINT chk_pending_refund_valid    CHECK (
        refunded_amount_minor + pending_refund_minor <= captured_amount_minor
    )
);
CREATE UNIQUE INDEX idx_payments_provider_id ON payments(provider, provider_payment_id)
    WHERE provider_payment_id IS NOT NULL;
CREATE INDEX idx_payments_delivery_id  ON payments(delivery_id);
CREATE INDEX idx_payments_user_created ON payments(user_id, created_at DESC);
CREATE INDEX idx_payments_status       ON payments(status, updated_at);
```

#### 002-009: باقي الجداول

| Migration | الجدول |
|-----------|--------|
| `002` | `payment_attempts` (attempt_number, operation, status, provider_idempotency_key, error_category) |
| `003` | `payment_transactions` (AUTHORIZATION\|CAPTURE\|VOID\|REFUND types) |
| `004` | `refunds` (reservation pattern: pending_refund column في payments) |
| `005` | `refund_attempts` (refund execution history) |
| `006` | `idempotency_keys` (key UNIQUE, request_hash, response_payload, expires_at) |
| `007` | `payment_events_outbox` (published_at IS NULL index partial, SKIP LOCKED) |
| `008` | `payment_audit_log` (paymentID, operation, actor, details, created_at) |
| `009` | `processed_provider_events` (UNIQUE(provider, provider_event_id) → webhook dedup) |

---

### STEP 18 — `cmd/server/main.go`

```go
// ترتيب التهيئة:
// 1. logging.InitLogger()        → packages/go/logging
// 2. config.Load()
// 3. snowflake.NewSnowflake({WorkerID: cfg.SnowflakeWorkerID}) → packages/go/snowflake
// 4. postgres.Connect(cfg.PostgresDSN)
// 5. postgres.RunMigrations(db, "migrations/")
// 6. redis.NewClient()
// 7. kafka topics ensure → packages/go/kafka.EnsureTopics()
// 8. kafka.NewProducer() → packages/go/kafka
// 9. nats.Connect()     → packages/go/nats
// 10. build all repositories
// 11. stripeProvider = stripe.NewStripeProvider(cfg.StripeSecretKey)
// 12. build application service
// 13. build command/query handlers
// 14. start metrics server → metrics.StartMetricsServer(cfg.PortMetrics)
// 15. start workers (WorkerPool):
//     - outboxPublisher.Run(ctx, 2s)
//     - reconciliation.Run(ctx, 60s)
//     - stuckRecovery.Run(ctx, 30s)
//     - cleanup.Run(ctx, 1h)
// 16. gRPC server (50056) with metrics.UnaryServerMetricsInterceptor()
// 17. HTTP mux:
//     /payment/graphql  → GraphQL (auth middleware)
//     /graphql          → same
//     /health/live      → 200 OK
//     /health/ready     → DB ping
//     /webhook/payment  → Stripe webhook
// 18. http.Server{Addr: ":4002"}
// 19. signal.Notify(SIGTERM, SIGINT) → graceful shutdown
```

---

### STEP 19 — Infrastructure المطلوب إضافته

#### Infrastructure الموجود ✅ (لا تعديل)
- `payment-depl.yaml` ✅
- `payment-db-depl.yaml` ✅
- `payment-service-config.yaml` ✅
- `payment-secret.yaml` ✅
- `kustomization.yaml` ✅
- `skaffold.yaml` ✅
- `Dockerfile` ✅
- `docker-compose.yml` ✅

#### [MODIFY] `infrastructure/docker/compose.yml`

إضافة payment-service + payment-db-srv:

```yaml
payment-db-srv:
  image: postgres:15
  ports: ["5436:5432"]
  volumes: [payment_db_data:/var/lib/postgresql/data]
  networks: [default]
  aliases: [payment-db-srv]

payment-service:
  build: { context: ../.., dockerfile: services/payment-service/Dockerfile }
  ports: ["4002:4002", "50056:50056", "9106:9106", "4012:4012"]
  networks: { default: { aliases: [payment-srv] } }
  environment:
    STRIPE_SECRET_KEY: ${STRIPE_SECRET_KEY}
    STRIPE_WEBHOOK_SECRET: ${STRIPE_WEBHOOK_SECRET}
    ...
```

#### [MODIFY] `infrastructure/docker/compose.yml` — api-gateway

```yaml
environment:
  PAYMENT_SUBGRAPH_URL: "http://payment-srv:4002/payment/graphql"
```

#### [MODIFY] `infrastructure/docker/compose.yml` — delivery-service

```yaml
environment:
  PAYMENT_SERVICE_GRPC_URL: "payment-srv:50056"
```

#### observability — لا تعديل مطلوب ✅

الـ `infrastructure/observability/prometheus/`, `grafana/`, `otel/` موجودة. يمكن إضافة Grafana dashboard لـ payment metrics لاحقاً.

#### localstack — لا تعديل مطلوب ✅

Payment Service لا تستخدم S3/DynamoDB. `01-init-media.sh` للـ media service فقط.

---

### STEP 20 — `protos/payment.proto`

#### [MODIFY] [payment.proto](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/protos/payment.proto)

تعديل `go_package` فقط ليتوافق مع module name الصحيح:

```diff
-option go_package = "github.com/omar487/Realtime-Delivery/microservices/protos/payment/v1;paymentpb";
+option go_package = "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/protos/payment/v1;paymentpb";
```

باقي الـ proto كامل ومكتمل ✅

---

## Ordered File Creation List

| الترتيب | الملف | الحالة |
|---------|-------|--------|
| 1 | `packages/go/events/payment_events.go` | MODIFY |
| 2 | `protos/payment.proto` | MODIFY (go_package فقط) |
| 3 | `services/payment-service/go.mod` | MODIFY |
| 4 | `internal/config/config.go` | NEW |
| 5 | `internal/domain/state.go` | NEW |
| 6 | `internal/domain/money.go` | NEW |
| 7 | `internal/domain/errors.go` | NEW |
| 8 | `internal/domain/attempt.go` | NEW |
| 9 | `internal/domain/refund.go` | NEW |
| 10 | `internal/domain/payment.go` | NEW |
| 11 | `internal/ports/repositories.go` | NEW |
| 12 | `internal/ports/payment_provider.go` | NEW |
| 13 | `internal/ports/publisher.go` | NEW |
| 14 | `internal/ports/idempotency_store.go` | NEW |
| 15 | `internal/adapters/postgres/db.go` | NEW |
| 16 | `internal/adapters/postgres/payment_repo.go` | NEW |
| 17 | `internal/adapters/postgres/refund_repo.go` | NEW |
| 18 | `internal/adapters/postgres/attempt_repo.go` | NEW |
| 19 | `internal/adapters/postgres/outbox_repo.go` | NEW |
| 20 | `internal/adapters/postgres/idempotency_repo.go` | NEW |
| 21 | `internal/adapters/postgres/audit_repo.go` | NEW |
| 22 | `internal/adapters/redis/idempotency_cache.go` | NEW |
| 23 | `internal/adapters/redis/lock_manager.go` | NEW |
| 24 | `internal/adapters/providers/errors.go` | NEW |
| 25 | `internal/adapters/providers/stripe/stripe_provider.go` | NEW |
| 26 | `internal/adapters/kafka/event_publisher.go` | NEW |
| 27 | `internal/adapters/kafka/publisher.go` | NEW |
| 28 | `internal/adapters/nats/realtime_publisher.go` | NEW |
| 29 | `internal/adapters/grpc/proto/` (generated) | NEW |
| 30 | `internal/adapters/grpc/server.go` | NEW |
| 31 | `internal/adapters/webhook/handler.go` | NEW |
| 32 | `internal/application/services/payment_service.go` | NEW |
| 33 | `internal/application/commands/create_payment.go` | NEW |
| 34 | `internal/application/commands/authorize_payment.go` | NEW |
| 35 | `internal/application/commands/capture_payment.go` | NEW |
| 36 | `internal/application/commands/cancel_authorization.go` | NEW |
| 37 | `internal/application/commands/create_refund.go` | NEW |
| 38 | `internal/application/queries/get_payment.go` | NEW |
| 39 | `internal/application/queries/get_payment_status.go` | NEW |
| 40 | `internal/graphql/schema_sdl.go` | NEW |
| 41 | `internal/graphql/dataloader.go` | NEW |
| 42 | `internal/graphql/resolver.go` | NEW |
| 43 | `internal/graphql/handler.go` | NEW |
| 44 | `internal/observability/metrics.go` | NEW |
| 45 | `internal/observability/tracer.go` | NEW |
| 46 | `internal/workers/pool.go` | NEW |
| 47 | `internal/workers/outbox_publisher.go` | NEW |
| 48 | `internal/workers/reconciliation.go` | NEW |
| 49 | `internal/workers/stuck_recovery.go` | NEW |
| 50 | `internal/workers/cleanup.go` | NEW |
| 51 | `internal/validation/validator.go` | NEW |
| 52 | `migrations/001_create_payments.sql` | NEW |
| 53 | `migrations/002_create_payment_attempts.sql` | NEW |
| 54 | `migrations/003_create_payment_transactions.sql` | NEW |
| 55 | `migrations/004_create_refunds.sql` | NEW |
| 56 | `migrations/005_create_refund_attempts.sql` | NEW |
| 57 | `migrations/006_create_idempotency_keys.sql` | NEW |
| 58 | `migrations/007_create_payment_events_outbox.sql` | NEW |
| 59 | `migrations/008_create_payment_audit_log.sql` | NEW |
| 60 | `migrations/009_create_processed_provider_events.sql` | NEW |
| 61 | `cmd/server/main.go` | NEW |
| 62 | `infrastructure/docker/compose.yml` | MODIFY |

---

## Verification Plan

### بعد إنشاء الكود
```bash
# 1. Build
cd services/payment-service && go mod tidy && go build ./...

# 2. Unit tests (domain layer)
go test ./internal/domain/...

# 3. Docker build
cd ../.. && docker build -f services/payment-service/Dockerfile -t delivery/payment .
```

### Manual Verification
1. `docker-compose -f services/payment-service/docker-compose.yml up`
2. gRPC: `CreatePayment` → `AuthorizePayment` → `CapturePayment`
3. Stripe dashboard: تحقق من PaymentIntent
4. Kafka: events `payment.created`, `payment.authorized`, `payment.captured`
5. GraphQL: `localhost:4002/payment/graphql` → `{ payment(id: "...") { status } }`
6. Metrics: `localhost:9106/metrics` → payment_operations_total
7. Webhook: `stripe listen --forward-to localhost:4012/webhook/payment`
