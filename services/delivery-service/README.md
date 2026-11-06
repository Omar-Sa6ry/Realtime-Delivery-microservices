# Delivery Service

## Overview

The Delivery Service is a microservice responsible for managing the complete lifecycle of delivery orders in a real-time delivery platform. It handles order creation, driver assignment, payment processing, status tracking, and real-time notifications through multiple communication channels.

## Architecture

### Technology Stack

- **Framework**: NestJS 11 (TypeScript)
- **Database**: PostgreSQL with TypeORM
- **Message Brokers**:
  - Apache Kafka (event streaming)
  - NATS (real-time messaging)
  - RabbitMQ (reliable message delivery)
- **Caching**: Redis (BullMQ for job queues)
- **API**: GraphQL (Apollo Federation) and gRPC
- **Monitoring**: Prometheus metrics, structured logging
- **Containerization**: Docker, Docker Compose

### Core Components

```
src/
├── app.module.ts                 # Root module
├── main.ts                       # Application bootstrap
├── health.controller.ts          # Health check endpoints
├── common/
│   ├── config/                   # Configuration modules
│   ├── metrics/                  # Prometheus metrics
│   ├── translation/              # i18n support
│   └── graphql/                  # GraphQL context
└── modules/
    ├── delivery/                 # Core delivery domain
    │   ├── entities/             # TypeORM entities
    │   ├── enums/                # Domain enums
    │   ├── repositories/         # Data access layer
    │   ├── services/             # Business logic services
    │   ├── saga/                 # Saga orchestration
    │   ├── outbox/               # Outbox pattern implementation
    │   └── graphql/              # GraphQL resolvers and types
    └── infrastructure/
        ├── kafka/                # Kafka consumer/producer
        ├── nats/                 # NATS integration
        ├── grpc/                 # gRPC server
        └── rabbitmq/             # RabbitMQ publisher
```

## Domain Model

### Entities

- **Delivery**: Core aggregate root representing a delivery order
- **Address**: Value object for pickup/dropoff locations
- **DeliveryStatusHistory**: Audit trail of status changes
- **Outbox**: Reliable event publishing pattern
- **DeliverySagaState**: Saga orchestration state tracking

### Status Flow

```
CREATED → PENDING_PAYMENT → PAYMENT_CONFIRMED → DRIVER_ASSIGNED → DRIVER_ACCEPTED → PICKED_UP → IN_TRANSIT → DELIVERED → COMPLETED
                    ↓              ↓                    ↓
                 CANCELLED    CANCELLED            CANCELLED
                    ↓              ↓                    ↓
                  FAILED       FAILED               FAILED
```

### Payment Statuses

- PENDING
- AUTHORIZED
- CAPTURED
- COMPLETED
- FAILED
- REFUNDED
- CANCELLED

## Communication Patterns

### GraphQL API

- **Endpoint**: `/delivery/graphql`
- **Schema**: Apollo Federation 2 compatible
- **Authentication**: Header-based (x-user-id, x-user-role)
- **Rate Limiting**: Sliding window (60 req/min for queries, 120 req/min for mutations)

### gRPC Service

- **Port**: 50054 (configurable via PORT_DELIVERY_GRPC)
- **Proto**: `delivery.proto`
- **Package**: `delivery`

### Event Streaming (Kafka)

Topics:
- `delivery.created`
- `delivery.status.changed`
- `delivery.driver.assigned`
- `delivery.picked_up`
- `delivery.in_transit`
- `delivery.completed`
- `delivery.cancelled`

### Real-time Messaging (NATS)

Subjects:
- `delivery.status.updated`
- `delivery.driver.assignment.updated`
- `notification.user.{userId}`

### Reliable Messaging (RabbitMQ)

Exchanges:
- `orders` (order lifecycle events)
- `dispatch` (driver dispatch requests)

## Saga Orchestration

The service implements a choreography-based saga pattern for distributed transactions:

1. **Payment Authorization Step**
   - Creates payment hold/authorization via Payment Service
   - Handles checkout URLs for customer-initiated payments
   - Transitions to PENDING_PAYMENT or PAYMENT_CONFIRMED

2. **Driver Assignment Step**
   - Reserves driver via Driver Service
   - Publishes dispatch request to RabbitMQ
   - Handles driver acceptance/rejection

### Compensation Actions

- Payment cancellation/refund on failure
- Driver reservation release
- Delivery cancellation with notification

## Outbox Pattern

Reliable event publishing using the transactional outbox pattern:

- Events stored in `outbox` table within same transaction as domain changes
- Background publisher polls pending events
- Exponential backoff retry (max 5 minutes)
- Kafka as the primary event store

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| PORT_DELIVERY | HTTP port | 4003 |
| PORT_DELIVERY_GRPC | gRPC port | 50054 |
| DB_HOST | PostgreSQL host | localhost |
| DB_PORT | PostgreSQL port | 5432 |
| DB_NAME | Database name | delivery_delivery_db |
| DB_USERNAME | Database user | postgres |
| POSTGRES_PASSWORD | Database password | - |
| REDIS_HOST | Redis host | localhost |
| REDIS_PORT | Redis port | 6379 |
| NATS_URL | NATS server URL | nats://localhost:4222 |
| KAFKA_BROKERS | Kafka brokers | localhost:9092 |
| RABBITMQ_URL | RabbitMQ URL | amqp://localhost:5672 |
| USER_SERVICE_URL | User service gRPC | user-srv:50051 |
| PAYMENT_SERVICE_URL | Payment service gRPC | payment-srv:50056 |
| DRIVER_SERVICE_URL | Driver service gRPC | driver-srv:50053 |
| JWT_SECRET | JWT signing secret | - |
| JWT_EXPIRE | Token expiration | 36000s |
| APP_ENV | Environment (development/production) | development |

### Configuration Files

- `config/env/.env.development` - Development environment
- `config/env/.env.production` - Production environment

## Getting Started

### Prerequisites

- Node.js 20+
- PostgreSQL 15+
- Redis 7+
- Apache Kafka 3+
- NATS Server 2+
- RabbitMQ 3.12+
- Docker & Docker Compose (optional)

### Installation

```bash
# Install dependencies
make install
# or
npm install
```

### Development

```bash
# Start with hot reload
make start-dev
# or
npm run start:dev
```

### Production Build

```bash
make build
make start-prod
# or
npm run build && npm run start:prod
```

### Using Docker

```bash
# Build image
make docker-build

# Start all services
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

## Testing

### Unit Tests

```bash
# Run once
make test
# or
npm run test

# Watch mode
make test-watch
# or
npm run test:watch

# With coverage
make test-cov
# or
npm run test:cov
```

### E2E Tests

```bash
make test-e2e
# or
npm run test:e2e
```

### Coverage Report

```bash
make coverage-report
# or
npm run test:cov && open coverage/lcov-report/index.html
```

## Database Operations

### Migrations

```bash
# Create new migration
make migrate-create NAME=add_delivery_tracking

# Run pending migrations
make migrate-run

# Revert last migration
make migrate-revert

# Check status
make migrate-status
```

### Seeding

```bash
make seed
```

### Database Reset

```bash
make db-reset
```

## Code Quality

### Linting

```bash
# Auto-fix
make lint
# or
npm run lint

# Check only
make lint-check
```

### Formatting

```bash
# Format
make format
# or
npm run format

# Check formatting
make format-check
```

### Security Audit

```bash
make security-audit
```

## API Documentation

### GraphQL Schema

The GraphQL schema is auto-generated from TypeScript decorators. Key operations:

#### Queries

```graphql
# Get single delivery
query GetDelivery($id: ID!) {
  delivery(id: $id) {
    success
    data {
      id
      status
      paymentStatus
      amount
      currency
      pickupAddress { line1 city }
      dropoffAddress { line1 city }
      statusHistory { status createdAt }
    }
  }
}

# Get current user's deliveries
query MyDeliveries($page: Int, $pageSize: Int) {
  myDeliveries(page: $page, pageSize: $pageSize) {
    data {
      items { id status amount }
      paginationInfo { totalItems currentPage nextPage }
    }
  }
}

# Get next valid statuses
query NextStatuses($id: ID!) {
  deliveryNextStatuses(id: $id) {
    data { statuses }
  }
}

# Admin: Get open deliveries
query OpenDeliveries($page: Int) {
  openDeliveries(page: $page) {
    data { items { id status } }
  }
}
```

#### Mutations

```graphql
# Create delivery
mutation CreateDelivery($input: CreateDeliveryInputDto!) {
  createDelivery(input: $input) {
    success
    data { id status paymentStatus checkoutUrl }
  }
}

# Transition status
mutation TransitionDelivery($input: TransitionDeliveryInput!) {
  transitionDelivery(input: $input) {
    success
    data { id status }
  }
}

# Cancel delivery
mutation CancelDelivery($deliveryId: String, $reason: String) {
  cancelDelivery(deliveryId: $deliveryId, reason: $reason) {
    success
    data { id status }
  }
}
```

### Health Endpoints

| Endpoint | Description |
|----------|-------------|
| GET /health | Full health check |
| GET /delivery/health | Service health |
| GET /health/live | Liveness probe |
| GET /delivery/health/live | Liveness probe |
| GET /health/ready | Readiness probe |
| GET /delivery/health/ready | Readiness probe |
| GET /metrics | Prometheus metrics |
| GET /delivery/metrics | Prometheus metrics |

## Deployment

### Docker Production

```dockerfile
# Multi-stage build
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./
EXPOSE 4003 50054
CMD ["node", "dist/main"]
```

### Environment-Specific Configs

```bash
# Development
APP_ENV=development docker-compose up

# Production
APP_ENV=production docker-compose -f docker-compose.yml -f docker-compose.prod.yml up
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: delivery-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: delivery-service
  template:
    metadata:
      labels:
        app: delivery-service
    spec:
      containers:
      - name: delivery-service
        image: delivery-service:latest
        ports:
        - containerPort: 4003
        - containerPort: 50054
        envFrom:
        - configMapRef:
            name: delivery-service-config
        - secretRef:
            name: delivery-service-secrets
```

## Monitoring

### Metrics

Prometheus metrics exposed at `/metrics`:

- `delivery_created_total` - Counter of created deliveries
- `delivery_status_transitions_total` - Status transition counter
- `delivery_duration_seconds` - Delivery completion histogram
- `saga_execution_duration_seconds` - Saga execution timing
- `outbox_events_pending` - Pending outbox events gauge

### Logging

Structured JSON logging with correlation IDs:

```json
{
  "timestamp": "2024-01-15T10:30:00.000Z",
  "level": "info",
  "message": "Delivery created",
  "service": "delivery-service",
  "correlationId": "abc-123",
  "deliveryId": "del-456",
  "customerId": "cust-789"
}
```

### Health Checks

Kubernetes probes:
- Liveness: `/health/live`
- Readiness: `/health/ready`

## Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| Database connection failed | Check DB_HOST, DB_PORT, credentials |
| Kafka connection failed | Verify KAFKA_BROKERS, broker availability |
| NATS connection failed | Check NATS_URL, server status |
| gRPC connection failed | Verify service URLs, network policies |
| Payment authorization failed | Check Payment Service health, credentials |
| Driver assignment timeout | Check Driver Service, dispatch queue |

### Debug Mode

```bash
make start-debug
# or
npm run start:debug
```

Attach debugger to port 9229.

### Logs

```bash
# Docker logs
make docker-logs

# Application logs
tail -f logs/application.log

# Error logs
tail -f logs/error.log
```

## Contributing

### Development Workflow

1. Create feature branch from `main`
2. Implement changes with tests
3. Run `make ci-check` locally
4. Submit pull request
5. CI pipeline validates
6. Code review
7. Merge to `main`

### Commit Convention

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types: feat, fix, docs, style, refactor, test, chore

### Pre-commit Hooks

```bash
make pre-commit
```

Runs: lint, format-check, unit tests

## License

UNLICENSED - Private repository

## Support

For issues and questions:
- Create GitHub issue
- Check existing documentation
- Review troubleshooting guide