# Payment Service

The Payment Service is an event-driven, production-grade microservice engineered in Go that owns the entire payment lifecycle for the Realtime Delivery microservices platform. It orchestrates financial transactions with payment service providers (Stripe, with extensible support for PayPal, Adyen, and local providers), processes authorizations, captures funds, issues full and partial refunds, handles asynchronous webhooks, and broadcasts transactional state transitions in real time across the platform via Apache Kafka and NATS JetStream.

The service follows clean hexagonal architecture (Ports and Adapters), strictly decoupling pure domain business rules from database drivers, message brokers, and transport layers.

---

## Architecture Overview

```
                 +-----------------------------------+
                 |        Client / API Gateway       |
                 +-----------------+-----------------+
                                   |
                         +---------+---------+
                         |                   |
                         | GraphQL           | gRPC
                         v (Port 4002)       v (Port 50056)
             +---------------------------------------------------+
             |                  Payment Service                  |
             |---------------------------------------------------|
             |  Domain Services (Payment, Refund, Authorize)     |
             |  Transactional Outbox Publisher                   |
             |  Background Reconciliation Engine                 |
             |  Idempotency & Concurrency Controller             |
             |  OpenTelemetry Tracer & Prometheus Metrics        |
             +---+---------------+---------------+-----------+---+
                 |               |               |           |
                 v               v               v           v
           +-----------+   +-----------+   +-----------+ +-----------+
           | PostgreSQL|   |   Kafka   |   |   NATS    | |   Redis   |
           |  (State & |   |  (Durable |   | JetStream | |(Distributed|
           |  Outbox)  |   |  Events)  |   |(Broadcast)| |  Locks)   |
           +-----------+   +-----------+   +-----------+ +-----------+
                                                           |
                                                           v
                                                [Stripe Webhook (4012)]
```

### Core Architectural Patterns
- **Hexagonal Architecture**: Business logic in `internal/domain` has zero third-party dependencies. Repositories, providers, and message brokers are interfaced in `internal/ports` and implemented in `internal/adapters`.
- **Two-Phase Payment Processing**: Supports explicit pre-authorization hold on customer funds followed by capture upon courier pickup, or voiding if no driver is available.
- **Transactional Outbox Pattern**: State modifications and outbox event records are saved within the same atomic PostgreSQL transaction, guaranteeing at-least-once delivery to Kafka.
- **Idempotency Guarantee**: Every mutating request requires an `idempotency-key` checked against Redis and PostgreSQL to prevent duplicate charges or double refunds.
- **Self-Healing Reconciliation**: The background reconciliation worker detects stuck or interrupted payment states and automatically queries provider APIs to converge state.

---

## Directory Structure

```
services/payment-service/
|-- cmd/
|   `-- server/                 Service main entrypoint, dependency wiring, graceful shutdown
|-- internal/
|   |-- domain/                 Pure domain entities, money value objects, state machines
|   |   |-- payment.go          Payment aggregate root, status enum, state transitions
|   |   |-- refund.go           Refund aggregate root, calculation rules
|   |   `-- money.go            Immutable Money value object with currency validation
|   |-- ports/                  Interfaces for repositories, payment providers, message buses
|   |-- application/
|   |   `-- services/           Use cases (CreatePayment, Authorize, Capture, Refund, Cancel)
|   |-- adapters/
|   |   |-- postgres/           PostgreSQL persistence, transactional outbox implementation
|   |   |-- redis/              Distributed lock manager and idempotency cache
|   |   |-- providers/
|   |   |   `-- stripe/         Stripe SDK integration, webhook verification, error normalization
|   |   |-- kafka/              Kafka event publisher using confluent-kafka-go / sarama
|   |   |-- nats/               NATS JetStream realtime status publisher
|   |   |-- grpc/               gRPC server handling inter-service RPC invocations
|   |   |-- graphql/            Apollo Federation Subgraph schema and resolvers
|   |   `-- webhook/            HTTP webhook listener for Stripe asynchronous events
|   |-- workers/                Background processes (outbox publisher, reconciliation)
|   |-- config/                 Environment variables mapping and validation
|   |-- i18n/                   Localized error messages (English & Arabic)
|   `-- observability/          OpenTelemetry tracing and Prometheus metrics collectors
|-- test/
|   `-- e2e/                    End-to-end integration and lifecycle tests
|-- Dockerfile                  Multi-stage production build container
|-- docker-compose.yml          Local dependencies (PostgreSQL, Kafka, NATS, Redis)
|-- Makefile                    Standard developer targets
|-- go.mod                      Go module definition
`-- go.sum                      Go checksums
```

---

## Payment Lifecycle & State Machine

```
               [INITIATED]
                    |
           +--------+--------+
           |                 |
     (Authorize)          (Direct Capture)
           |                 |
           v                 v
     [AUTHORIZED]        [CAPTURED]
      |        |             |
 (Capture)   (Void)      (Refund)
      |        |             |
      v        v             v
  [CAPTURED] [VOIDED]    [REFUNDED / PARTIALLY_REFUNDED]
```

### State Machine Definitions
- `INITIATED`: Payment intent created; awaiting customer payment method presentation.
- `AUTHORIZED`: Funds held on customer card; ready for capture upon order fulfillment.
- `CAPTURED`: Funds successfully captured and settled.
- `VOIDED`: Pre-authorization release without charge (e.g. order cancelled prior to driver dispatch).
- `REFUNDED`: Funds returned to customer following cancellation or return.
- `FAILED`: Terminal failure due to card decline, fraud score, or communication failure.

---

## Transactional Outbox Pattern

To prevent dual-write anomalies between PostgreSQL and Kafka:
1. When a payment state transition occurs, the domain event is serialized and written to the `payment_outbox` table in the exact same SQL transaction.
2. The `OutboxWorker` reads unpublished events using `SELECT ... FOR UPDATE SKIP LOCKED` in batches of 50.
3. Events are published to the Kafka topic `payment.status.v1` and NATS subject `payment.status.updated`.
4. Upon broker acknowledgment, events are marked as published with a timestamp, ensuring at-least-once delivery with zero lost events.

---

## Stripe Webhook Integration

The webhook server listens on port `4012` at path `/webhooks/stripe`:
- **Signature Verification**: Validates `Stripe-Signature` using `STRIPE_WEBHOOK_SECRET` with tolerance checks against replay attacks.
- **Handled Events**:
  - `payment_intent.succeeded`: Updates payment status to `CAPTURED`.
  - `payment_intent.payment_failed`: Marks payment as `FAILED` with provider error code.
  - `payment_intent.canceled`: Marks authorization as `VOIDED`.
  - `charge.refunded`: Updates refund entity status and notifies customer.

---

## Environment Variables

| Variable | Description | Default | Required |
| :--- | :--- | :--- | :--- |
| `ENV` | Environment (`development`, `production`, `test`) | `development` | Yes |
| `HTTP_PORT` | GraphQL and Health HTTP server port | `4002` | No |
| `GRPC_PORT` | gRPC server listener port | `50056` | No |
| `WEBHOOK_PORT` | Webhook HTTP listener port | `4012` | No |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://payment:payment@localhost:5436/payment_db?sslmode=disable` | Yes |
| `REDIS_URL` | Redis connection URL | `redis://localhost:6379/1` | Yes |
| `KAFKA_BROKERS` | Comma-delimited list of Kafka broker endpoints | `localhost:9092` | Yes |
| `NATS_URL` | NATS JetStream server URL | `nats://localhost:4222` | Yes |
| `STRIPE_SECRET_KEY` | Stripe API private key | - | Yes |
| `STRIPE_WEBHOOK_SECRET`| Stripe webhook signing secret | - | Yes |
| `OTEL_EXPORTER_OTLP_ENDPOINT`| OpenTelemetry OTLP collector URL | `localhost:4317` | No |

---

## Local Development & Setup

### Prerequisites
- Go 1.22+
- Docker and Docker Compose
- GNU Make

### Running Infrastructure
```bash
make docker-up
```

### Running the Service
```bash
# Build the binary
make build

# Run locally
make run
```

---

## Testing

```bash
# Run all unit tests
make test

# Run tests with verbose output
make test-v

# Run end-to-end integration tests
make test-e2e

# Generate test coverage report
make coverage
```

---

## Makefile Reference

- `make help`: Display available targets and descriptions.
- `make build`: Compile the Go application to `bin/payment-service`.
- `make run`: Build and run the server locally.
- `make test`: Execute all unit tests.
- `make test-v`: Execute all unit tests with verbose logging.
- `make test-e2e`: Execute end-to-end integration test suites.
- `make test-all`: Execute both unit and e2e test suites.
- `make coverage`: Compute code coverage and write `coverage.out`.
- `make lint`: Run golangci-lint across all packages.
- `make tidy`: Tidy and verify Go module dependencies.
- `make docker-build`: Build Docker container image.
- `make docker-up`: Start PostgreSQL, Redis, Kafka, and NATS in background.
- `make docker-down`: Stop and remove local containers.
- `make clean`: Remove binaries and temporary test outputs.
