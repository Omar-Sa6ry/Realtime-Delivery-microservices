# Realtime Delivery Microservices Platform - Comprehensive Technical Blueprint

---

## Table of Contents
1. [Executive Summary & High-Level Architecture](#1-executive-summary--high-level-architecture)
2. [Global Architecture & Network Topology](#2-global-architecture--network-topology)
3. [Complete Directory & Subsystem Inventory](#3-complete-directory--subsystem-inventory)
4. [In-Depth Microservice Specifications](#4-in-depth-microservice-specifications)
   - [4.1 API Gateway (`api-gateway`)](#41-api-gateway-api-gateway)
   - [4.2 User & Identity Service (`user-service`)](#42-user--identity-service-user-service)
   - [4.3 Delivery Fulfillment & Saga Service (`delivery-service`)](#43-delivery-fulfillment--saga-service-delivery-service)
   - [4.4 Driver Lifecycle & Geo-Location Service (`driver-service`)](#44-driver-lifecycle--geo-location-service-driver-service)
   - [4.5 Payment Processing & Reconciliation Service (`payment-service`)](#45-payment-processing--reconciliation-service-payment-service)
   - [4.6 Media Ingestion, Security & Processing Service (`media-service`)](#46-media-ingestion-security--processing-service-media-service)
   - [4.7 Notification Engine (`notification-service`)](#47-notification-engine-notification-service)
   - [4.8 Realtime WebSocket & Presence Service (`realtime-service`)](#48-realtime-websocket--presence-service-realtime-service)
   - [4.9 Faceted Search & Indexing Engine (`search-service`)](#49-faceted-search--indexing-engine-search-service)
   - [4.10 OLAP Analytics & Business Intelligence (`analytics-service`)](#410-olap-analytics--business-intelligence-analytics-service)
5. [Shared Monorepo Libraries (`packages/`)](#5-shared-monorepo-libraries-packages)
   - [5.1 Go Shared Kernel (`packages/go`)](#51-go-shared-kernel-packagesgo)
   - [5.2 TypeScript Shared Foundation (`packages/ts`)](#52-typescript-shared-foundation-packagests)
6. [gRPC Contracts & Protocol Buffers Specification (`protos/`)](#6-grpc-contracts--protocol-buffers-specification-protos)
7. [GraphQL Federation & Supergraph Schema Architecture](#7-graphql-federation--supergraph-schema-architecture)
8. [Distributed Transaction Orchestration (Delivery Saga)](#8-distributed-transaction-orchestration-delivery-saga)
9. [Event-Driven Messaging & Multi-Broker Strategy](#9-event-driven-messaging--multi-broker-strategy)
10. [Infrastructure, Containerization & Kubernetes Deployments](#10-infrastructure-containerization--kubernetes-deployments)
11. [Distributed Observability, Telemetry & Alerting](#11-distributed-observability-telemetry--alerting)
12. [Security, Identity, RBAC & Idempotency](#12-security-identity-rbac--idempotency)
13. [Frontend Applications (`media-frontend`)](#13-frontend-applications-media-frontend)
14. [Local Development, Tooling & Operational Runbook](#14-local-development-tooling--operational-runbook)

---

## 1. Executive Summary & High-Level Architecture

The **Realtime Delivery Microservices Platform** is an enterprise-grade, distributed backend ecosystem engineered to support on-demand logistics, real-time driver dispatching, geo-location tracking, payment processing, media asset pipelines, and analytical data aggregation.

The codebase implements a modern polyglot microservice architecture:
- **Go (Golang 1.22+)**: Powers latency-critical, compute-heavy, and high-concurrency microservices (`driver-service`, `payment-service`, `media-service`, `search-service`, `analytics-service`). Built on Hexagonal Architecture (Ports and Adapters) with dependency inversion.
- **TypeScript / Node.js (NestJS 10+)**: Powers domain-rich services, GraphQL federation, and stateful WebSocket gateways (`api-gateway`, `user-service`, `delivery-service`, `notification-service`, `realtime-service`). Built using domain-driven modularity.

### Primary Architectural Patterns
1. **Apollo Federation v2 Supergraph**: Distributed subgraphs are resolved independently by backend services and unified under a single edge gateway. Clients interact with one schema while subgraphs evolve independently.
2. **Transactional Outbox Pattern**: Implemented in both Go and TypeScript services with dedicated background workers, guaranteeing at-least-once delivery of domain events without distributed two-phase commits (2PC).
3. **Orchestrated Saga Pattern**: The delivery fulfillment process coordinates multiple autonomous services (`delivery-service`, `payment-service`, `driver-service`) through explicit forward steps and automated compensation rollbacks.
4. **Multi-Broker Message Partitioning**:
   - **Apache Kafka**: Persistent, ordered log for domain event publishing, search reindexing, media workflows, and OLAP analytics ingestion.
   - **RabbitMQ**: AMQP transactional queues with Dead Letter Exchanges (DLX) and exponential backoff retry mechanisms for asynchronous jobs (notifications, delivery state dispatch).
   - **NATS JetStream**: High-frequency pub/sub for driver GPS location pings, low-latency WebSocket broadcasting, and driver presence tracking.
5. **Polyglot Persistence Layer**: Each service owns its private datastore:
   - PostgreSQL (ACID relational data for users, deliveries, payments, notifications).
   - MongoDB (Geospatial 2dsphere indexing for drivers).
   - ClickHouse (High-throughput columnar analytical storage).
   - OpenSearch (Faceted full-text search and geo-distance queries).
   - Redis (Distributed lock manager, rate limiting, and session caching).
   - AWS S3 / LocalStack (Object storage for raw and processed media).
   - DynamoDB (Multipart chunk and media upload session state).

---

## 2. Global Architecture & Network Topology

```
                                  +-----------------------+
                                  |   Web & Mobile Clients|
                                  +-----------+-----------+
                                              |
                   HTTPS / GraphQL (Port 4000)|  WebSocket (ws://, Port 4006)
                                              v
+-----------------------------------------------------------------------------------------+
|                                      INGRESS / EDGE                                     |
|                                                                                         |
|       +------------------------------------+    +------------------------------------+  |
|       |     api-gateway (NestJS / Apollo)  |    |  realtime-service (NestJS / WS)    |  |
|       |     GraphQL Federation Gateway     |    |  Presence, Tracking, Subscriptions |  |
|       +-----------------+------------------+    +-----------------+------------------+  |
+-------------------------|-----------------------------------------|---------------------+
                          |                                         |
     Apollo Federation v2 | Subgraphs                               | NATS JetStream & Redis
                          |                                         |
+-------------------------v-----------------------------------------v---------------------+
|                                   BACKEND SERVICES                                      |
|                                                                                         |
|  +--------------------+  +--------------------+  +-------------------+  +------------+  |
|  | user-service       |  | delivery-service   |  | driver-service    |  | payment-   |  |
|  | (NestJS / Postgres)|  | (NestJS / Postgres)|  | (Go / MongoDB)    |  | service    |  |
|  +---------+----------+  +---------+----------+  +---------+---------+  | (Go / Post)|  |
|            |                       |                       |            +-----+------+  |
|            +-----------+-----------+-----------+-----------+                  |         |
|                        |                       |                              |         |
|                        v                       v                              |         |
|              +-------------------+   +--------------------+                   |         |
|              | media-service     |   | search-service     |                   |         |
|              | (Go / S3 / Dynamo)|   | (Go / OpenSearch)  |                   |         |
|              +---------+---------+   +---------+----------+                   |         |
|                        |                       |                              |         |
|                        v                       v                              v         |
|              +-------------------+   +--------------------+            +-------------+  |
|              | notification-svc  |   | analytics-service  |            | Stripe /    |  |
|              | (NestJS / Postgres|   | (Go / ClickHouse)  |            | Providers   |  |
|              +-------------------+   +--------------------+            +-------------+  |
|                                                                                         |
+-----------------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------------+
|                               EVENT BROKERS & STORAGE                                   |
|                                                                                         |
|   +--------------------+    +--------------------+    +-----------------------------+   |
|   |   Apache Kafka     |    |      RabbitMQ      |    |       NATS JetStream        |   |
|   |  Persistent Logs   |    |  Task Queues & DLX |    | Low-latency Telemetry / WS  |   |
|   +--------------------+    +--------------------+    +-----------------------------+   |
|                                                                                         |
|   +--------------------+    +--------------------+    +-----------------------------+   |
|   |     PostgreSQL     |    |      MongoDB       |    |         ClickHouse          |   |
|   | Users, Deliveries, |    | Drivers, Geofence, |    | Time-series Events, OLAP,   |   |
|   | Payments, Notifs   |    | Realtime Locations |    | Metrics, Performance logs   |   |
|   +--------------------+    +--------------------+    +-----------------------------+   |
|                                                                                         |
|   +--------------------+    +--------------------+    +-----------------------------+   |
|   |     OpenSearch     |    |  Redis (Cluster)   |    |    AWS S3 / LocalStack      |   |
|   |  Faceted Search,   |    | Caching, Locks,    |    |   Media Uploads, Chunks,    |   |
|   |  Geo queries       |    | Rate Limiting, WS  |    |   Transcoded Assets, Avatars|   |
|   +--------------------+    +--------------------+    +-----------------------------+   |
+-----------------------------------------------------------------------------------------+
```

### Complete Service Port Matrix

| Service | GraphQL Subgraph Port | gRPC Server Port | Metrics Port | Auxiliary Ports / Notes |
| :--- | :--- | :--- | :--- | :--- |
| `api-gateway` | 4000 (Gateway Core) | - | 4000 (`/metrics`) | 4099 (Dedicated Liveness Probe) |
| `user-service` | 4001 | 50051 | 9101 | - |
| `payment-service` | 4002 | 50056 | 9106 | 4012 (Stripe Webhook Listener) |
| `delivery-service` | 4003 | 50053 | 9104 | - |
| `notification-service` | 4004 | 50054 | 9108 | - |
| `media-service` | 4005 | 50052 | 9102 | - |
| `realtime-service` | 4006 | - | 9109 | 4006 (`/ws` WebSocket Gateway) |
| `search-service` | 4007 | - | 9103 | - |
| `driver-service` | 4008 | 50055 | 9105 | - |
| `analytics-service` | 4009 | 50057 | 9107 | - |
| `prometheus-srv` | - | - | 9090 | Prometheus UI |
| `alertmanager-srv` | - | - | 9093 | Alertmanager API |
| `alertmanager-discord-srv` | - | - | 8080 (Mapped: 8081)| Discord Webhook Relay |
| `grafana-srv` | - | - | 3000 | Grafana Dashboards |
| `loki-srv` | - | - | 3100 | Log Ingestion API |
| `tempo-srv` | - | - | 3200 | Distributed Tracing UI & OTLP |
| `rabbitmq-srv` | - | - | 15692 (Prometheus) | 5672 (AMQP), 15672 (Management) |
| `localstack-srv` | - | - | - | 4566 (AWS S3 & DynamoDB Gateway) |

---

## 3. Complete Directory & Subsystem Inventory

```
.
├── .github/
│   └── workflows/
│       └── common-unit-tests.yml        # CI matrix running unit tests for Go and TS
├── .qodo/                               # AI code review, test generation and static analysis config
├── .trunk/                              # Trunk meta-linter orchestrator (hadolint, eslint, markdownlint)
├── api-docs/                            # Complete GraphQL schema specifications for all services
│   ├── analytics.graphql
│   ├── apiGW.graphql
│   ├── auth.graphql
│   ├── delivery.graphql
│   ├── driver.graphql
│   ├── media.graphql
│   ├── notification.graphql
│   ├── observability.md
│   ├── payment.graphql
│   ├── realtime.graphql
│   ├── search.graphql
│   └── user.graphql
├── config/
│   └── env/
│       ├── .env.development            # Local development environment configuration
│       ├── .env.production             # Production environment baseline
│       └── .env.test                   # Isolated test environment parameters
├── graphql-docs/                        # Supplemental GraphQL operation documents
├── infrastructure/
│   ├── commands.txt                     # Operational cheatsheet for building, running, and debugging
│   ├── docker/
│   │   ├── compose.infra.yml            # Storage, broker, and third-party containers
│   │   ├── compose.integration.yml      # Integration test container harness
│   │   └── compose.yml                  # Full application microservices stack
│   ├── kubernetes/
│   │   ├── base/
│   │   │   ├── configmaps/              # Environment-specific ConfigMaps
│   │   │   ├── infrastructure/          # Stateful manifests (Postgres, Mongo, ClickHouse, etc.)
│   │   │   ├── observability/           # Manifests for Prometheus, Grafana, Loki, Tempo, OTel
│   │   │   ├── secrets/                 # Sealed and base secrets
│   │   │   ├── services/                # Deployments, Services, and HPAs for all 10 microservices
│   │   │   ├── ingress-srv.yaml         # Ingress-NGINX routing rules
│   │   │   └── kustomization.yaml       # Kustomize entrypoint for base manifests
│   │   └── overlays/
│   │       ├── development/             # Dev overrides (single replicas, NodePorts)
│   │       ├── test/                    # Test overrides
│   │       └── production/              # Production overrides (multi-replica, HPA, resource quotas)
│   ├── localstack/
│   │   └── init/
│   │       └── 01-init-media.sh         # Auto-creates S3 buckets and DynamoDB tables on startup
│   ├── observability/
│   │   ├── alertmanager/                # Alertmanager configuration template and entrypoint
│   │   ├── discord-relay/               # Go microservice translating Prometheus alerts to Discord
│   │   ├── grafana/                     # Grafana provisioning, datasources, and dashboard JSONs
│   │   ├── otel/                        # OpenTelemetry Collector pipeline definition
│   │   ├── prometheus/                  # Prometheus scraping rules and alert thresholds
│   │   └── tempo/                       # Grafana Tempo trace configuration
│   ├── rabbitmq/
│   │   ├── definitions.json             # Pre-configured exchanges, queues, and bindings
│   │   ├── enabled_plugins              # Active plugins (management, prometheus)
│   │   └── rabbitmq.conf                # Node clustering and security config
│   └── skaffold/
│       ├── port-forwards.yaml           # Local forwarding definitions for Kubernetes pods
│       └── skaffold.yaml                # Continuous Kubernetes dev workflow configuration
├── media-frontend/                      # Independent React/Vite web application for media uploads
├── packages/
│   ├── go/                              # Shared Go modules
│   │   ├── auth/                        # Token verification, JWT validation, claims extraction
│   │   ├── automation/                  # Test harness utilities
│   │   ├── constants/                   # Shared error strings, HTTP status codes, and constants
│   │   ├── env/                         # Type-safe configuration loader with validation
│   │   ├── events/                      # CloudEvents envelope schemas
│   │   ├── kafka/                       # Kafka consumer group and producer abstractions
│   │   ├── logging/                     # Zap structured JSON logging with correlation tracing
│   │   ├── metrics/                     # Prometheus counters, gauges, histograms
│   │   ├── middleware/                  # HTTP & gRPC interceptors for panics, tracing, and auth
│   │   ├── nats/                        # NATS connection management and JetStream helpers
│   │   ├── rabbitmq/                    # AMQP publisher confirmation and consumer channel pooling
│   │   ├── ratelimiter/                 # Redis-backed sliding window rate limiters
│   │   ├── snowflake/                   # Monotonic 64-bit unique ID generation
│   │   ├── tracing/                     # OpenTelemetry Go SDK initialization and exporter setup
│   │   ├── users/                       # Shared user data structures
│   │   └── websocket/                   # Realtime WS frame models
│   └── ts/                              # Shared TypeScript package (`@delivery-micro/shard`)
│       ├── src/
│       │   ├── automation/              # Test utilities
│       │   ├── constants/               # Roles, Permissions, Status enums, Queue names
│       │   ├── decorators/              # `@CurrentUser()`, `@Roles()`, `@CorrelationId()`
│       │   ├── dtos/                    # Common DTOs with `class-validator` rules
│       │   ├── events/                  # Domain event payload interfaces
│       │   ├── filters/                 # Global exception filters for HTTP & GraphQL
│       │   ├── graphql/                 # Federation directives, custom scalars, plugins
│       │   ├── guard/                   # JWT authentication and RBAC guards
│       │   ├── interceptors/            # Logging, timing, and trace propagation interceptors
│       │   ├── interfaces/              # Common repository and domain contracts
│       │   ├── kafka/                   # NestJS Kafka microservice transport wrapper
│       │   ├── logging/                 # Structured Pino/Winston logging
│       │   ├── metrics/                 # Prometheus NestJS metrics exporter
│       │   ├── modules/                 # Shared dynamic modules
│       │   ├── nats/                    # NATS transport provider
│       │   ├── rabbitmq/                # AMQP broker abstractions
│       │   └── websocket/               # WS message protocols and frame validation
├── protos/                              # Canonical Protocol Buffer definitions (.proto)
│   ├── analytics.proto
│   ├── delivery.proto
│   ├── driver.proto
│   ├── media.proto
│   ├── notification.proto
│   ├── payment.proto
│   └── user.proto
├── scripts/
│   ├── sync-shard.ps1                   # Syncs packages across microservice node_modules
│   ├── test-cov.ps1                     # Aggregates test coverage reports across the platform
│   ├── test-e2e.ps1                     # Executes end-to-end integration test suites
│   └── test-unit.ps1                    # Runs unit tests for all Go and TypeScript projects
├── services/                            # All 10 backend microservices
├── Makefile                             # Top-level GNU Makefile for orchestration
└── Taskfile.yml                         # Task runner definitions for local development
```

---

## 4. In-Depth Microservice Specifications

### 4.1 API Gateway (`api-gateway`)
- **Technology**: TypeScript / Node.js 20+, NestJS 10, Apollo Gateway (@apollo/gateway, @apollo/server).
- **Port**: 4000 (GraphQL Federation Edge), 4099 (Dedicated Liveness Probe).
- **Core Role**: Acts as the single entry point for all client requests. Unifies the 8 GraphQL subgraphs (`user`, `payment`, `delivery`, `notification`, `media`, `realtime`, `search`, `analytics`) into a single federated schema.
- **Key Modules & Internal Implementation**:
  - `src/common/graphql/subgraph-service.ts`: Dynamically discovers and builds remote subgraph connections using Apollo's `RemoteGraphQLDataSource`. Intercepts every outgoing subgraph request to forward headers:
    - `x-user-id`, `x-user-role`, `x-user-email` (derived from validated JWT).
    - `x-correlation-id` (for distributed trace tracking).
    - `accept-language` / `x-lang` (for internationalized error strings).
  - `src/common/guards/auth.guard.ts`: Validates incoming Bearer JWT tokens before delegating downstream. Populates the GraphQL execution context with verified user claims.
  - `src/common/guards/rate-limiter.factory.ts`: Sliding-window distributed rate limiter backed by Redis. Protects upstream APIs against brute force and distributed denial of service.
  - `src/common/middlewares/correlation-id.middleware.ts`: Generates a UUIDv4 correlation ID if missing and attaches it to request context, response headers, and OpenTelemetry trace spans.
  - `src/common/graphql/format-error.ts`: Standardizes error formatting across all subgraphs into a uniform structure containing `message`, `statusCode`, `errorDetails`, and `timestamp`.
  - `src/utils/waitService.util.ts`: Resilient startup probe that polls all downstream subgraph health endpoints before initiating supergraph construction, preventing startup race conditions.

### 4.2 User & Identity Service (`user-service`)
- **Technology**: TypeScript, NestJS, TypeORM, PostgreSQL, gRPC.
- **Ports**: 4001 (GraphQL Subgraph), 50051 (gRPC Server), 9101 (Metrics).
- **Core Role**: Manages authentication, user registration, role-based access control, cryptographic password hashing, user profiles, and delivery addresses.
- **Data Models (`src/common/database/entities/`)**:
  - `User`: Primary entity storing `id` (Snowflake), `email`, `passwordHash` (Argon2/Bcrypt), `firstName`, `lastName`, `role` (`ADMIN`, `USER`, `DRIVER`), `isActive`, and `avatarMediaId`.
  - `Address`: User saved addresses storing structured lines, city, postal code, and geospatial coordinates (`latitude`, `longitude`).
  - `Outbox`: Stores domain events (`UserRegisteredEvent`, `UserProfileUpdatedEvent`) in the same database transaction as the business entity.
- **gRPC Controller (`src/modules/user/user-grpc.controller.ts`)**:
  - Implements `user.proto`:
    - `GetUser(id)`: Returns user details by ID.
    - `GetUserByEmail(email)`: Returns user profile by email address.
    - `ValidateToken(token)`: Validates JWT signature and expiration, returning user ID and role.
    - `GetUserPermissions(user_id)`: Resolves full permission array based on the user's role mapping.
    - `UpdateUserRole(user_id, role)`: Updates user role (e.g. promoting user to driver upon onboarding).
- **GraphQL Subgraph**: Exposes `@key(fields: "id")` directive on type `User`, allowing other subgraphs (e.g. `delivery-service`, `driver-service`) to extend the `User` entity seamlessly.

### 4.3 Delivery Fulfillment & Saga Service (`delivery-service`)
- **Technology**: TypeScript, NestJS, TypeORM, PostgreSQL, RabbitMQ, Kafka.
- **Ports**: 4003 (GraphQL Subgraph), 50053 (gRPC Server), 9104 (Metrics).
- **Core Role**: Core logistics engine. Coordinates the full lifecycle of deliveries, order assignment, price calculations, and distributed transaction orchestration.
- **State Machine Architecture (`src/modules/delivery/services/delivery.state-machine.ts`)**:
  - Governs valid state transitions for deliveries:
    - `CREATED` -> `PENDING_PAYMENT`, `PAYMENT_CONFIRMED`, `DRIVER_ASSIGNED`, `CANCELLED`
    - `PENDING_PAYMENT` -> `PAYMENT_CONFIRMED`, `CANCELLED`, `FAILED`
    - `PAYMENT_CONFIRMED` -> `DRIVER_ASSIGNED`, `CANCELLED`
    - `DRIVER_ASSIGNED` -> `DRIVER_ACCEPTED`, `CANCELLED`
    - `DRIVER_ACCEPTED` -> `PICKED_UP`, `CANCELLED`
    - `PICKED_UP` -> `IN_TRANSIT`, `CANCELLED`
    - `IN_TRANSIT` -> `DELIVERED`, `FAILED`
    - `DELIVERED` -> `COMPLETED`
    - Terminal states: `COMPLETED`, `CANCELLED`, `FAILED`.
- **Saga Orchestrator (`src/modules/delivery/saga/`)**:
  - `delivery-saga.orchestrator.ts`: Executes distributed transactions across service boundaries.
  - `steps/payment-confirmation.step.ts`: Invokes `payment-service` to authorize payment. If subsequent steps fail, issues a compensating `RefundPayment` request.
  - `steps/driver-assignment.step.ts`: Queries `driver-service` via gRPC to find available drivers within proximity, reserve a candidate, and dispatch notifications. If no driver accepts before timeout, releases reservation and triggers compensation.
- **Idempotency Service (`src/modules/delivery/services/idempotency.service.ts`)**:
  - Caches idempotency keys in Redis with SHA-256 request payload hashing. Prevents duplicate delivery orders upon client network timeouts.

### 4.4 Driver Lifecycle & Geo-Location Service (`driver-service`)
- **Technology**: Go (Golang 1.22+), Hexagonal Architecture, MongoDB (GeoJSON / 2dsphere), Redis, gRPC.
- **Ports**: 4008 (GraphQL Subgraph), 50055 (gRPC Server), 9105 (Metrics).
- **Core Role**: Manages driver registration, vehicle specifications, real-time availability, geographical proximity searches, and driver assignments.
- **Hexagonal Architecture Structure**:
  - `internal/domain`: Pure entities (`Driver`, `Vehicle`, `Location`, `Assignment`) free from external framework dependencies.
  - `internal/ports`: Inbound (Use Cases, Queries) and Outbound (Repositories, Message Publishers, External RPC clients) interfaces.
  - `internal/adapters/mongodb`: Implements driver repository using MongoDB's `$nearSphere` geospatial queries against 2dsphere indexes to locate candidates within radius $R$.
  - `internal/adapters/redis`: In-memory driver location cache and distributed mutexes using Redis `SETNX`.
  - `internal/adapters/grpc`: Implements `driver.proto`:
    - `FindAvailableDrivers`: Discovers available drivers near pickup latitude/longitude.
    - `ReserveDriver`: Atomically reserves a driver for a specific delivery with an expiration lease.
    - `ReleaseDriver`: Releases an active reservation if delivery is cancelled or timed out.
    - `AssignDriver`: Confirms assignment once accepted.
- **Background Workers (`internal/workers/`)**:
  - `assignment_expiry.go`: Ticks every few seconds to identify pending assignments exceeding the acceptance window, releasing the driver back to the pool.
  - `heartbeat_monitor.go`: Audits driver heartbeat timestamps in Redis; marks drivers as `OFFLINE` if no heartbeat is received within 60 seconds.
  - `reconciliation.go`: Synchronizes state consistency between MongoDB and Redis cache.

### 4.5 Payment Processing & Reconciliation Service (`payment-service`)
- **Technology**: Go (Golang 1.22+), Hexagonal Architecture, PostgreSQL, Stripe SDK, Redis, gRPC.
- **Ports**: 4002 (GraphQL Subgraph), 50056 (gRPC Server), 9106 (Metrics), 4012 (Stripe Webhook Listener).
- **Core Role**: Manages financial transactions, payment authorization, fund captures, cancellations, automated refunds, and external payment gateway webhooks.
- **Key Components**:
  - `internal/adapters/providers/stripe`: Encapsulates Stripe API integration (`PaymentIntent`, `Refund`, `Customer`).
  - `internal/adapters/grpc`: Implements `payment.proto`:
    - `CreatePayment`: Generates payment intent with amount and currency.
    - `AuthorizePayment`: Holds funds on customer payment method.
    - `CapturePayment`: Captures pre-authorized funds once delivery is picked up.
    - `CancelAuthorization`: Releases authorization hold if delivery is cancelled early.
    - `CreateRefund`: Issues full or partial refunds back to original payment methods.
  - `internal/adapters/webhook`: Dedicated HTTP server listening on port 4012. Validates Stripe HMAC signatures (`Stripe-Signature`), decodes events (`payment_intent.succeeded`, `payment_intent.payment_failed`), updates database records, and writes to Outbox.
- **Background Workers (`internal/workers/`)**:
  - `outbox_publisher.go`: Reads unpublished payment outbox records and writes to Kafka and RabbitMQ with transactional guarantees.
  - `reconciliation.go`: Periodically compares internal database transaction states against Stripe's remote API to resolve discrepancies.
  - `stuck_recovery.go`: Identifies payments lingering in `AUTHORIZING` or `CAPTURING` states due to network timeouts and triggers automated resolution.

### 4.6 Media Ingestion, Security & Processing Service (`media-service`)
- **Technology**: Go (Golang 1.22+), AWS S3 / LocalStack, Amazon DynamoDB, ClamAV, Redis, gRPC.
- **Ports**: 4005 (GraphQL Subgraph), 50052 (gRPC Server), 9102 (Metrics).
- **Core Role**: Handles secure media uploads (avatars, delivery proof photos, vehicle documents), multipart chunk management, malware scanning, and asynchronous transcoding.
- **Upload Flow & Key Components**:
  - Client initiates upload via GraphQL `createUploadSession` mutation.
  - Service writes upload session metadata to DynamoDB and returns presigned S3 PUT URLs.
  - Client streams file chunks directly to S3 storage.
  - Client issues `completeUpload` mutation.
- **Worker Pipeline (`internal/workers/`)**:
  - `scan`: ClamAV integration worker. Streams byte chunks from S3 through ClamAV daemon (`clamd`). If infected, immediately purges object from S3 and flags status `INFECTED`.
  - `compression`: Converts uploaded images to optimized WebP formats.
  - `image`: Generates standard thumbnails (`thumbnail` 150x150, `medium` 600x600, `large` 1200x1200).
  - `video`: Transcodes video files and extracts duration metadata.
  - `metadata`: Extracts EXIF, dimensions, and MIME types.
  - `outbox`: Dispatches `MediaUploadedEvent` and `MediaProcessedEvent` to Kafka.
  - `reconciliation`: Audits S3 bucket contents against DynamoDB records to remove abandoned or incomplete multipart uploads.

### 4.7 Notification Engine (`notification-service`)
- **Technology**: TypeScript, NestJS, TypeORM, PostgreSQL, RabbitMQ, Kafka, gRPC.
- **Ports**: 4004 (GraphQL Subgraph), 50054 (gRPC Server), 9108 (Metrics).
- **Core Role**: Multi-channel notification engine delivering transactional messages via Push Notifications (FCM/APNS), SMS (Twilio), Email (SendGrid/SMTP), and in-app notifications.
- **Key Modules & Entities**:
  - `Notification`: Main notification record storing recipient `userId`, category, priority (`HIGH`, `DEFAULT`, `LOW`), subject, body, and payload.
  - `NotificationDelivery`: Tracks individual channel attempts, gateway response IDs, error logs, and delivery timestamps.
  - `NotificationPreference`: User-configured channel preferences and quiet hours.
  - `NotificationTemplate`: Dynamic templates rendered using Mustache/Handlebars.
  - `NotificationInbox`: Read/unread in-app notification center.
  - `src/modules/workers`: Batch workers that consume messages from RabbitMQ queues, apply recipient preference filters, and route payloads to the appropriate third-party SDK.

### 4.8 Realtime WebSocket & Presence Service (`realtime-service`)
- **Technology**: TypeScript, NestJS, WebSockets (`ws`), Redis, NATS JetStream.
- **Ports**: 4006 (HTTP & WebSocket server `/ws`), 9109 (Metrics).
- **Core Role**: Bi-directional communication gateway for live location streaming, active delivery tracking maps, and user/driver presence monitoring.
- **Architecture & Protocols**:
  - Client connects via `ws://<host>:4006/ws` passing JWT token in query parameter or connection handshake headers.
  - `src/modules/gateway/websocket/websocket.gateway.ts`: Implements `OnGatewayConnection` and `OnGatewayDisconnect`. Enforces `WsAuthGuard` to validate user identity and permissions.
  - `src/modules/features/location`: Ingests high-frequency GPS coordinate pings from drivers, validates latitude/longitude boundaries, updates driver state in Redis, and pushes location events to NATS JetStream.
  - `src/modules/features/presence`: Tracks active connections in Redis sets with sliding TTLs. Broadcasts online/offline presence changes to authorized subscribers.
  - `src/modules/features/subscription`: Manages room subscriptions. When a user tracks delivery `X`, they join room `delivery:tracking:X`. Inbound driver coordinates are broadcast directly to room participants.

### 4.9 Faceted Search & Indexing Engine (`search-service`)
- **Technology**: Go (Golang 1.22+), OpenSearch, Redis, Apache Kafka, RabbitMQ.
- **Ports**: 4007 (GraphQL Subgraph), 9103 (Metrics).
- **Core Role**: Provides sub-second faceted full-text search, autocomplete, and geospatial searches across deliveries, drivers, and catalog items.
- **Key Modules**:
  - `internal/application/search`: Builds OpenSearch Query DSL requests featuring fuzzy matching, multi-field boosting, term aggregations, and geo-distance filters.
  - `internal/application/indexing`: Subscribes to Kafka domain topics (`delivery.created`, `delivery.updated`, `driver.status_changed`) and performs near-real-time upserts into OpenSearch indices.
  - `internal/application/reindex`: Zero-downtime reindexing engine that provisions new indices with updated mappings and swaps alias pointers atomically.
  - `internal/infrastructure/opensearch`: Manages index mappings, cluster settings, and connection pools.

### 4.10 OLAP Analytics & Business Intelligence (`analytics-service`)
- **Technology**: Go (Golang 1.22+), ClickHouse, Apache Kafka, NATS, gRPC.
- **Ports**: 4009 (GraphQL Subgraph), 50057 (gRPC Server), 9107 (Metrics).
- **Core Role**: Columnar analytical processing engine ingesting platform-wide telemetry, computing business KPIs, driver acceptance rates, and platform financial overviews.
- **Key Modules**:
  - `internal/adapters/clickhouse`: Executes batched asynchronous inserts into ClickHouse `MergeTree` and `SummingMergeTree` tables for write throughput.
  - `internal/adapters/kafka`: High-throughput consumer groups reading all platform domain events. Employs `internal/adapters/idempotency` to eliminate duplicate events.
  - `internal/adapters/grpc`: Implements `analytics.proto`:
    - `GetPlatformOverview(from, to)`: Computes total deliveries, completed deliveries, cancellation ratios, and completion percentage over time windows.
    - `GetDriverPerformance(driver_id, from, to)`: Computes total ride offers, accepted offers, driver acceptance rates, and average fulfillment duration.
  - `internal/workers/reconciliation_worker.go`: Schedules periodic background calculations to refresh materialized views and aggregated rollup tables.

---

## 5. Shared Monorepo Libraries (`packages/`)

To prevent duplication and guarantee consistency across microservices, shared functionality is extracted into two dedicated libraries:

### 5.1 Go Shared Kernel (`packages/go`)
- `auth`: Cryptographic verification of JWT tokens (RS256/HS256), extracting `Subject`, `Email`, and `Role` claims into Go `context.Context`.
- `constants`: Centralized error identifiers, HTTP/gRPC standard codes, and platform-wide constants.
- `env`: Reflection-based environment parser that enforces required environment variables with validation tags and fallbacks.
- `events`: Implements CloudEvents v1.0 standard schema wrappers, encapsulating payload data, correlation IDs, timestamps, and schema versions.
- `kafka`: Wrapper around `segmentio/kafka-go` configuring production-ready consumer groups, offset auto-commit strategies, and retry publishers.
- `logging`: Production Zap logger emitting structured JSON logs with automatic injection of trace IDs and correlation IDs.
- `metrics`: Standard Prometheus metrics collector configuring HTTP request duration histograms, active connection gauges, and error counters.
- `middleware`: HTTP and gRPC interceptors for panics (`recover`), correlation ID extraction, access logging, and tracing.
- `nats`: Resilient NATS connection manager with automatic reconnects and JetStream stream/consumer initialization helpers.
- `rabbitmq`: Robust AMQP client featuring channel pooling, topology declaration, publisher confirms, and reconnect loops.
- `ratelimiter`: Redis-backed sliding window rate limiter implementation.
- `snowflake`: High-performance 64-bit unique ID generator based on the Twitter Snowflake algorithm (timestamp, worker ID, sequence number).
- `tracing`: OpenTelemetry TracerProvider configuration supporting OTLP gRPC export to Tempo / OTel Collector.
- `websocket`: Shared WebSocket frame structures, message types, and serialization utilities.

### 5.2 TypeScript Shared Foundation (`packages/ts` - `@delivery-micro/shard`)
- `constants`:
  - `Role`: `ADMIN`, `USER`, `DRIVER`.
  - `Permission`: `CREATE_DELIVERY`, `VIEW_DELIVERY`, `UPDATE_DELIVERY_STATUS`, `CANCEL_DELIVERY`, `ASSIGN_DELIVERY_DRIVER`, `UPDATE_USER`, `DELETE_USER`, etc.
  - `DeliveryStatus`: `CREATED`, `PENDING`, `PENDING_PAYMENT`, `PAYMENT_CONFIRMED`, `SEARCHING_DRIVER`, `DRIVER_ASSIGNED`, `DRIVER_ACCEPTED`, `PICKUP_STARTED`, `PICKED_UP`, `IN_TRANSIT`, `DELIVERED`, `COMPLETED`, `CANCELLED`, `FAILED`.
  - `PaymentStatus`: `PENDING`, `AUTHORIZED`, `CAPTURED`, `COMPLETED`, `FAILED`, `REFUNDED`, `CANCELLED`.
  - `HeaderKeys`: `x-user-id`, `x-user-role`, `x-correlation-id`, `x-lang`, `accept-language`.
- `decorators`:
  - `@CurrentUser()`: Extracts the authenticated user object from GraphQL/HTTP context.
  - `@Roles(...roles)`: Enforces role-based access control metadata on routes and resolvers.
  - `@CorrelationId()`: Extracts the distributed correlation ID.
- `dtos`: Base DTOs with validation rules via `class-validator` (`IsUUID`, `IsNotEmpty`, `IsEmail`, `Min`, `Max`).
- `filters`: Global exception filters intercepting `HttpException`, `RpcException`, and GraphQL errors to emit uniform error envelopes.
- `guard`:
  - `AuthGuard`: Validates JWT tokens and injects user claims into context.
  - `RolesGuard`: Verifies that authenticated user roles satisfy endpoint requirements.
  - `WsAuthGuard`: Handshake guard authenticating WebSocket connections.
- `interceptors`:
  - `LoggingInterceptor`: Measures execution duration and logs requests/responses with correlation IDs.
  - `TracingInterceptor`: Creates and propagates OpenTelemetry trace spans across NestJS handlers.

---

## 6. gRPC Contracts & Protocol Buffers Specification (`protos/`)

The platform utilizes Protocol Buffers (proto3) as the single source of truth for synchronous inter-service communication:

### 1. `analytics.proto`
```protobuf
syntax = "proto3";
package analytics;

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

### 2. `delivery.proto`
```protobuf
syntax = "proto3";
package delivery;

service DeliveryService {
  rpc IsParticipant (ParticipantRequest) returns (ParticipantResponse);
  rpc GetDelivery (GetDeliveryRequest) returns (GetDeliveryResponse);
  rpc RefundPayment (RefundPaymentRequest) returns (RefundPaymentResponse);
  rpc GetOpenDeliveries (GetOpenDeliveriesRequest) returns (GetOpenDeliveriesResponse);
}

message ParticipantRequest { string userId = 1; string deliveryId = 2; }
message ParticipantResponse { bool isParticipant = 1; }

message GetDeliveryRequest { string deliveryId = 1; }
message GetDeliveryResponse {
  bool found = 1;
  string deliveryId = 2;
  string customerId = 3;
  string driverId = 4;
  string status = 5;
  double amount = 6;
  string currency = 7;
}

message RefundPaymentRequest { string deliveryId = 1; string reason = 2; }
message RefundPaymentResponse { bool accepted = 1; string deliveryId = 2; string refundId = 3; }
```

### 3. `driver.proto`
```protobuf
syntax = "proto3";
package driver;

service DriverService {
  rpc FindAvailableDrivers (FindAvailableDriversRequest) returns (FindAvailableDriversResponse);
  rpc ReserveDriver        (ReserveDriverRequest)        returns (ReserveDriverResponse);
  rpc ReleaseDriver        (ReleaseDriverRequest)        returns (ReleaseDriverResponse);
  rpc GetDriver            (GetDriverRequest)            returns (GetDriverResponse);
  rpc GetDriverStatus      (GetDriverStatusRequest)      returns (GetDriverStatusResponse);
  rpc AssignDriver         (AssignDriverRequest)         returns (AssignDriverResponse);
  rpc ValidateID           (ValidateIDRequest)           returns (ValidateIDResponse);
}

enum VehicleType {
  VEHICLE_TYPE_UNKNOWN = 0;
  CAR = 1;
  MOTORCYCLE = 2;
  TRUCK = 3;
  BICYCLE = 4;
  VAN = 5;
}

message FindAvailableDriversRequest {
  double latitude = 1;
  double longitude = 2;
  double radiusKm = 3;
  VehicleType vehicleType = 4;
  string deliveryId = 5;
  string correlationId = 6;
}

message DriverCandidate {
  string driverId = 1;
  double distanceMeters = 2;
  VehicleType vehicleType = 3;
  double latitude = 4;
  double longitude = 5;
}

message FindAvailableDriversResponse { repeated DriverCandidate candidates = 1; }
```

### 4. `media.proto`
```protobuf
syntax = "proto3";
package media;

service MediaService {
  rpc ResolveMediaUrl (ResolveMediaUrlRequest) returns (ResolveMediaUrlResponse);
  rpc GetMedia (GetMediaRequest) returns (GetMediaResponse);
}

message ResolveMediaUrlRequest {
  string media_id = 1;
  string requester_id = 2;
  string version_type = 3;
  int32  expiry_seconds = 4;
}

message ResolveMediaUrlResponse {
  string url = 1;
  int64  expires_at_seconds = 2;
  string content_type = 3;
  string media_id = 4;
  string status = 5;
}
```

### 5. `notification.proto`
```protobuf
syntax = "proto3";
package notification;

service NotificationService {
  rpc SendNotification(SendNotificationRequest) returns (SendNotificationResponse);
}

message SendNotificationRequest {
  string user_id = 1;
  string type = 2;
  string title = 3;
  string body = 4;
  string data = 5;
  string priority = 6;
}

message SendNotificationResponse {
  string notification_id = 1;
  bool success = 2;
}
```

### 6. `payment.proto`
```protobuf
syntax = "proto3";
package payment.v1;

service PaymentService {
  rpc CreatePayment(CreatePaymentRequest) returns (CreatePaymentResponse);
  rpc AuthorizePayment(AuthorizePaymentRequest) returns (AuthorizePaymentResponse);
  rpc CapturePayment(CapturePaymentRequest) returns (CapturePaymentResponse);
  rpc CancelAuthorization(CancelAuthorizationRequest) returns (CancelAuthorizationResponse);
  rpc CreateRefund(CreateRefundRequest) returns (CreateRefundResponse);
  rpc GetPayment(GetPaymentRequest) returns (GetPaymentResponse);
  rpc GetPaymentStatus(GetPaymentStatusRequest) returns (GetPaymentStatusResponse);
}

message CreatePaymentRequest {
  string delivery_id = 1;
  string user_id = 2;
  int64 amount_minor = 3;
  string currency = 4;
  string idempotency_key = 5;
}

message CreatePaymentResponse {
  string payment_id = 1;
  string status = 2;
  string payment_link = 3;
  string gateway_reference = 4;
}
```

### 7. `user.proto`
```protobuf
syntax = "proto3";
package user;

service UserService {
  rpc GetUser (GetUserRequest) returns (GetUserResponse);
  rpc GetUserByEmail (GetUserByEmailRequest) returns (GetUserResponse);
  rpc ValidateToken (ValidateTokenRequest) returns (ValidateTokenResponse);
  rpc GetUserPermissions (GetUserPermissionsRequest) returns (GetUserPermissionsResponse);
  rpc UpdateUserRole (UpdateUserRoleRequest) returns (UpdateUserRoleResponse);
}

message GetUserRequest { string id = 1; }
message GetUserResponse {
  string id = 1;
  string email = 2;
  string role = 3;
  string first_name = 4;
  string last_name = 5;
  bool is_active = 6;
}
```

---

## 7. GraphQL Federation & Supergraph Schema Architecture

The platform uses Apollo Federation v2 to assemble a single supergraph from isolated subgraphs:

```
               [Client HTTP Query / Mutation]
                             |
                             v
              [api-gateway (Federated Gateway)]
                             |
    +------------+-----------+-----------+------------+
    |            |           |           |            |
    v            v           v           v            v
[user-svc] [delivery-svc] [driver-svc] [payment-svc] [search-svc] ...
```

### Subgraph Extension Example
- `user-service` declares the canonical entity:
  ```graphql
  type User @key(fields: "id") {
    id: ID!
    email: String!
    firstName: String!
    lastName: String!
  }
  ```
- `delivery-service` extends `User` to attach active deliveries:
  ```graphql
  extend type User @key(fields: "id") {
    id: ID! @external
    deliveries: [Delivery!]!
  }
  ```
- `payment-service` extends `Delivery` to attach transactions:
  ```graphql
  extend type Delivery @key(fields: "id") {
    id: ID! @external
    payment: Payment
  }
  ```
The Gateway plans query execution, resolving `User` details from `user-service`, `Delivery` records from `delivery-service`, and `Payment` details from `payment-service` in a single GraphQL request without over-fetching.

---

## 8. Distributed Transaction Orchestration (Delivery Saga)

Fulfilling an on-demand delivery requires coordinating operations across multiple databases without distributed locking. The platform implements the **Orchestrated Saga Pattern**:

```
+-----------------------------------------------------------------------------------------+
|                               DELIVERY SAGA ORCHESTRATION                               |
+-----------------------------------------------------------------------------------------+

  1. Client: Mutation createDelivery(input, idempotencyKey)
     │
     ▼
  2. delivery-service:
     - Verify idempotency in Redis.
     - Persist Delivery entity (Status: CREATED).
     - Persist DeliverySagaState (Step: PAYMENT_PENDING).
     - Write event DeliveryCreated to Outbox in same DB transaction.
     │
     ▼
  3. payment-confirmation.step:
     - Invoke payment-service via gRPC AuthorizePayment.
     - Hold funds on customer card.
     │
     ├───► [Payment Authorization Failed]
     │     │
     │     ▼
     │     - Transition Delivery to FAILED.
     │     - Terminate Saga.
     │
     └───► [Payment Authorization Succeeded]
           │
           ▼
  4. driver-assignment.step:
     - Transition Delivery to SEARCHING_DRIVER.
     - Query driver-service via gRPC FindAvailableDrivers (pickup lat/lon, radius 5km).
     - If candidate found: ReserveDriver with 60-second lease.
     - Emit DriverOfferAssigned via RabbitMQ & Push Notification.
     │
     ├───► [Driver Rejects / Timeout Expired / No Drivers Available]
     │     │
     │     ▼
     │   EXECUTE SAGA COMPENSATIONS:
     │     1. Invoke payment-service CreateRefund / CancelAuthorization.
     │     2. Invoke driver-service ReleaseDriver (if reserved).
     │     3. Transition Delivery to CANCELLED / FAILED.
     │     4. Emit DeliveryCancelled event to notify customer.
     │
     └───► [Driver Accepts Assignment via WebSocket/API]
           │
           ▼
  5. SAGA SUCCESS:
     - Transition Delivery to DRIVER_ASSIGNED -> DRIVER_ACCEPTED.
     - Capture Payment upon package pickup (PICKED_UP).
     - Realtime tracking activated via realtime-service.
```

---

## 9. Event-Driven Messaging & Multi-Broker Strategy

The platform segregates messaging duties across three specialized brokers to align throughput, latency, and delivery guarantees:

| Broker | Primary Use Cases | Delivery Guarantees | Key Topics / Exchanges |
| :--- | :--- | :--- | :--- |
| **Apache Kafka** | Domain event audit log, Search indexing, Analytics ingestion, Media asset pipelines | At-least-once, Ordered per partition, Persistent retention | `delivery.events`, `payment.events`, `driver.events`, `media.events`, `user.events` |
| **RabbitMQ** | Transactional task processing, Delivery dispatching, Notification routing, DLX retries | At-least-once, Acknowledgements, Dead-letter handling | `delivery.exchange`, `notification.exchange`, `delivery.dlx`, `notification.dlx` |
| **NATS JetStream** | Real-time driver GPS telemetry, WebSocket broadcasting, Heartbeat monitoring | Ultra-low latency, High frequency, Ephemeral / Persistent streams | `driver.location.*`, `realtime.tracking.*`, `presence.heartbeat` |

### Transactional Outbox Pattern
To prevent dual-write anomalies where a database write succeeds but message broker publication fails, every state-modifying service records an event row inside the same database transaction:
1. Business entity updated and Outbox record inserted atomically.
2. Background worker (`outbox_publisher`) reads uncommitted events using row-level locking (`SELECT ... FOR UPDATE SKIP LOCKED`).
3. Message published to Kafka/RabbitMQ.
4. Outbox record marked `PUBLISHED` or purged.

---

## 10. Infrastructure, Containerization & Kubernetes Deployments

### 10.1 Docker Compose Architecture (`infrastructure/docker/`)
- `compose.infra.yml`: Launches all foundational persistence and message brokers:
  - PostgreSQL instances (`user-db`, `delivery-db`, `payment-db`, `notification-db`).
  - MongoDB (`driver-db`) with 2dsphere indexing.
  - ClickHouse (`clickhouse`) for OLAP analytics.
  - OpenSearch & OpenSearch Dashboards (`opensearch`, `opensearch-dashboards`).
  - Apache Kafka & Zookeeper (`kafka`, `zookeeper`).
  - RabbitMQ with management plugin (`rabbitmq`).
  - NATS with JetStream enabled (`nats`).
  - Redis cluster instance (`redis`).
  - LocalStack (`localstack`) for AWS S3 and DynamoDB emulation.
  - ClamAV (`clamav`) antivirus daemon.
- `compose.yml`: Spins up all 10 application microservices, injecting environment variables and establishing network links to `compose.infra.yml`.
- `compose.integration.yml`: Dedicated isolated container harness for running E2E and integration tests in CI pipelines.

### 10.2 Kubernetes & Kustomize Architecture (`infrastructure/kubernetes/`)
Managed declaratively via Kustomize:
- `base/infrastructure/`: Deployments, StatefulSets, PersistentVolumeClaims, and Services for all datastores and brokers.
- `base/services/`: Deployments, ClusterIP Services, and Horizontal Pod Autoscalers (HPA) for application services.
- `base/observability/`: Deployments for Prometheus, Grafana, OpenTelemetry Collector, Loki, Tempo, Alertmanager, and Discord Relay.
- `base/ingress-srv.yaml`: NGINX Ingress rules directing traffic to `api-gateway-srv:4000` and WebSocket connections (`/ws`) to `realtime-srv:4006`.
- `overlays/`:
  - `development`: Minimal resource requests, 1 replica, NodePort services for direct host access.
  - `test`: Configurations for ephemeral test environments.
  - `production`: Multi-replica deployments (3-10 replicas), PodDisruptionBudgets, resource limits/requests, and external secret bindings.

---

## 11. Distributed Observability, Telemetry & Alerting

### 1. Distributed Tracing (OpenTelemetry & Tempo)
- Trace context (`traceparent`, `tracestate`) is injected into all outbound HTTP, gRPC, Kafka, and RabbitMQ headers.
- OpenTelemetry Collector receives OTLP traces on port 4317 (gRPC) and 4318 (HTTP), batch-processes them, and exports to Grafana Tempo.
- Complete visual spans show the exact latency of Gateway -> Subgraph -> gRPC -> Database -> Message Broker.

### 2. Prometheus Metrics & Grafana Dashboards
- Every service exposes a `/metrics` Prometheus endpoint.
- Metrics track request rates (`app_requests_total`), error counts (`app_errors_total`), duration histograms (`app_request_duration_seconds_bucket`), active WebSocket clients, and Outbox lag.
- Grafana includes pre-provisioned dashboards for platform health, service overviews, RabbitMQ queue depths, and database connection pools.

### 3. Log Aggregation (Grafana Loki)
- Services log structured JSON containing `level`, `timestamp`, `service`, `correlation_id`, `trace_id`, and `message`.
- Loki ingests and indexes logs by service and severity, enabling correlation-based filtering across services.

### 4. Alertmanager & Discord Relay
- `alerts.yml` defines rules:
  - `ScrapeTargetDown`: Triggers if any service instance is unreachable for 2 minutes.
  - `HighRequestErrorRate`: Triggers if service error rate exceeds 5% over 10 minutes.
  - `HighRequestLatencyP95`: Triggers if 95th percentile latency exceeds 2 seconds.
  - `MediaServiceUploadFailures`: Triggers on upload failure spikes.
- When an alert fires, Alertmanager dispatches a webhook to `discord-relay` (`infrastructure/observability/discord-relay`), a custom Go microservice that formats the alert into a rich Discord embed and posts it to the configured operational Discord channel.

---

## 12. Security, Identity, RBAC & Idempotency

1. **Authentication & JWT Validation**:
   - `user-service` signs JWT tokens using a private secret with configurable expiration (`JWT_EXPIRE`).
   - The Gateway validates tokens and extracts claims before proxying downstream.
   - Downstream services verify forwarded user headers or re-validate tokens via the gRPC `user.ValidateToken` endpoint.

2. **Role-Based Access Control (RBAC)**:
   - System roles: `ADMIN`, `USER`, `DRIVER`.
   - Granular permissions map roles to specific operations (`VIEW_DELIVERY`, `UPDATE_DELIVERY_STATUS`, `RECHARGE_WALLET`, etc.).
   - Enforced in NestJS via `@Roles()` decorators and `RolesGuard`, and in Go services via gRPC interceptors.

3. **Data Protection & Sanitization**:
   - Password hashes generated using Argon2/Bcrypt with unique salts.
   - All file uploads scanned in real-time by ClamAV before becoming accessible.
   - S3 presigned URLs expire automatically (default TTL: 900 seconds).

4. **Idempotency Guarantees**:
   - State-changing operations (payment capture, delivery creation, refunds) require an `idempotency_key` header or parameter.
   - Idempotency middleware stores hashed keys and responses in Redis. If a duplicate request arrives while processing, subsequent requests receive identical responses without re-executing side effects.

---

## 13. Frontend Applications (`media-frontend`)

Located at `media-frontend/`, this application provides a reference frontend for the media lifecycle:
- **Technology**: React 18, TypeScript, Vite, TailwindCSS.
- **Features**:
  - Direct integration with `media-service` GraphQL API.
  - Multipart chunked upload simulator with real-time progress bars.
  - Real-time status polling (`SCANNING` -> `PROCESSING` -> `READY`).
  - Image preview and thumbnail resolution switcher.
  - Antivirus test suite with EICAR test string uploads to verify ClamAV detection and automated purging.

---

## 14. Local Development, Tooling & Operational Runbook

### Prerequisites
- Docker & Docker Compose (v24+)
- Go (v1.22+)
- Node.js (v20+) & pnpm / npm
- Task (`go-task`) or GNU Make

### Quick Start Guide

1. **Validate Environment**:
   ```bash
   task env-check
   # Verifies existence of config/env/.env.development
   ```

2. **Start Infrastructure Containers**:
   ```bash
   task infra-up
   # Starts Postgres, Mongo, ClickHouse, OpenSearch, Kafka, RabbitMQ, NATS, Redis, LocalStack, ClamAV
   ```

3. **Check Infrastructure Status**:
   ```bash
   task infra-logs
   ```

4. **Compile Protocol Buffers**:
   ```bash
   # Compiles Go and TypeScript stubs from protos/
   task generate-protos
   ```

5. **Launch Microservices**:
   ```bash
   # Start all microservices in background containers
   docker compose -f infrastructure/docker/compose.yml up -d --build
   ```

6. **Execute Automated Tests**:
   ```powershell
   # Run all Go and TypeScript unit tests
   ./scripts/test-unit.ps1

   # Run end-to-end integration tests
   ./scripts/test-e2e.ps1

   # Generate consolidated test coverage reports
   ./scripts/test-cov.ps1
   ```

7. **Useful Development URLs**:
   - **GraphQL Gateway Interface**: `http://localhost:4000/graphql`
   - **RabbitMQ Management Dashboard**: `http://localhost:15672` (User: `delivery` / Pass: `delivery-dev-password`)
   - **OpenSearch Dashboards**: `http://localhost:5601`
   - **Grafana Observability Portal**: `http://localhost:3000` (User: `admin` / Pass: `admin`)
   - **Prometheus Metrics Browser**: `http://localhost:9090`
   - **Media Frontend Test UI**: `http://localhost:5173`
