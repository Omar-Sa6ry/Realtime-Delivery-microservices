# Driver Service

## Overview

The Driver Service is the microservice that owns the complete driver domain of the real-time delivery platform. It is responsible for driver registration and profile management, online/offline availability, real-time location indexing, assignment lifecycle management, and the dispatch pipeline that matches open deliveries with the nearest available driver.

The service is written in Go and follows a hexagonal (ports and adapters) architecture. The domain layer is pure business logic with no infrastructure dependencies; every external system (MongoDB, Redis, Kafka, RabbitMQ, NATS, gRPC) is reached through a port interface and implemented by an adapter.

It exposes three listeners:

| Listener | Default Port | Protocol | Purpose |
| --- | --- | --- | --- |
| GraphQL | 4008 | HTTP | Apollo Federation v2 subgraph for driver operations, plus health endpoints |
| gRPC | 50055 | gRPC | Internal service-to-service API for dispatch, validation, and lookups |
| Metrics | 9105 | HTTP | Prometheus scrape endpoint |

## Architecture

### Technology Stack

- **Language**: Go 1.25
- **Database**: MongoDB (official v2 driver) for drivers, assignments, reviews, idempotency records
- **Cache and geospatial index**: Redis (GEO search for nearby drivers, SET NX for distributed locks)
- **Message brokers**:
  - Apache Kafka (primary event backbone, segmentio/kafka-go client)
  - RabbitMQ (optional secondary transport, enabled when `RABBITMQ_URL` is set)
  - NATS (location fan-out publisher)
- **API**: GraphQL (graph-gophers/graphql-go with Apollo Federation v2 directives) and gRPC (google.golang.org/grpc with reflection)
- **Observability**: Prometheus metrics, OpenTelemetry OTLP gRPC tracing, structured logging
- **Shared code**: monorepo `packages/go` module (events, kafka, rabbitmq, logging, middleware, metrics, tracing)
- **Containerization**: multi-stage Docker build on Alpine, Docker Compose for the dev stack and the e2e stack

### Architectural Layout

```
services/driver-service/
├── cmd/
│   └── server/
│       ├── main.go                    # Composition root: wires adapters, workers, servers
│       └── main_test.go               # Integration-style wiring tests (probe infra, skip if down)
├── internal/
│   ├── domain/                        # Pure domain model, no infrastructure imports
│   │   ├── driver.go                  # Driver aggregate, status machine, vehicle info
│   │   ├── assignment.go              # Assignment aggregate and status machine
│   │   ├── dispatch.go                # Dispatch policy, candidates, ranking, attempts
│   │   ├── review.go                  # Review entity and rating validation
│   │   ├── state.go                   # Generic transition validation helpers
│   │   └── errors.go                  # Typed domain error catalog with stable codes
│   ├── ports/                         # Interfaces isolating domain from infrastructure
│   │   ├── driver_repository.go
│   │   ├── assignment_repository.go
│   │   ├── dispatch_attempt_repository.go
│   │   ├── review_repository.go
│   │   ├── idempotency_store.go
│   │   ├── location_store.go
│   │   ├── lock_manager.go
│   │   ├── event_publisher.go
│   │   └── delivery_service.go
│   ├── application/
│   │   ├── commands/                  # Write use cases (CQRS command side)
│   │   │   ├── register_driver.go
│   │   │   ├── go_online.go / go_offline.go
│   │   │   ├── block_driver.go / unblock_driver.go
│   │   │   ├── reserve_driver.go / release_driver.go
│   │   │   ├── accept_assignment.go / reject_assignment.go
│   │   │   └── rate_driver.go
│   │   ├── queries/                   # Read use cases (CQRS query side)
│   │   │   ├── get_driver.go
│   │   │   ├── get_assignment.go
│   │   │   ├── get_driver_reviews.go
│   │   │   └── find_available_drivers.go
│   │   └── services/
│   │       └── dispatch_service.go    # Core dispatch orchestration
│   ├── adapters/                      # Infrastructure implementations of the ports
│   │   ├── mongodb/                   # Repositories: drivers, assignments, reviews, idempotency, dispatch attempts
│   │   ├── redis/                     # GEO store, lock manager, idempotency cache
│   │   ├── kafka/                     # Publisher, event publisher adapter, delivery.created consumer
│   │   ├── rabbitmq/                  # Publisher and dispatch consumer (optional transport)
│   │   └── grpc/                      # gRPC server, delivery-service client, generated proto stubs
│   │       └── proto/                 # Generated Go bindings for driver, user, delivery protos
│   ├── graphql/                       # GraphQL layer
│   │   ├── schema_sdl.go              # Apollo Federation v2 subgraph SDL
│   │   ├── resolver.go                # Root resolver and all field resolvers
│   │   ├── handler.go                 # HTTP handler, language and identity extraction, health
│   │   └── dataloader.go              # Batched entity loading to avoid N+1
│   ├── workers/                       # Background loops
│   │   ├── pool.go                    # Worker pool abstraction
│   │   ├── assignment_expiry.go       # Expires stale OFFERED assignments
│   │   ├── reconciliation.go          # Repairs BUSY drivers with no active assignment
│   │   └── heartbeat_monitor.go       # Monitors driver liveness
│   ├── observability/
│   │   ├── metrics.go                 # All Prometheus metrics and record helpers
│   │   └── tracing.go                 # OpenTelemetry OTLP tracer setup
│   ├── i18n/                          # English and Arabic message catalog
│   ├── config/                        # Environment-driven configuration
│   └── mocks/                         # Shared test mocks
├── test/
│   └── e2e/                           # End-to-end suite against live infrastructure
│       ├── setup_test.go              # Infra wait, config, cleanup
│       ├── driver_flow_test.go        # 19 scenarios: registration through broker round trips
│       ├── assert_test.go
│       └── stubuser/                  # Stand-in user-service gRPC server for e2e
├── src/                               # Legacy TypeScript DTO shapes kept for reference
├── Dockerfile                         # Multi-stage build, repo-root context
├── docker-compose.yml                 # Dev stack: MongoDB, Redis, Kafka, NATS, service
├── docker-compose.e2e.yml             # E2E stack with offset ports and healthchecks
└── Makefile                           # Full developer workflow
```

## Domain Model

### Driver Aggregate

A driver document lives in the `drivers` collection and carries a mutex-guarded set of invariants:

| Field | Type | Description |
| --- | --- | --- |
| `_id` | string | UUID assigned at registration |
| `userId` | string | Owning user reference (user-service) |
| `status` | enum | `OFFLINE`, `AVAILABLE`, `BUSY`, `SUSPENDED`, `BLOCKED` |
| `isActive` | bool | Admin activation flag |
| `isBlocked` | bool | Admin block flag |
| `vehicle.type` | enum | `CAR`, `MOTORCYCLE`, `TRUCK`, `BICYCLE`, `VAN` |
| `vehicle.plateNumber` | string | Plate number |
| `vehicle.capacityKg` | int64 | Carrying capacity |
| `capabilities` | string[] | Free-form capability tags |
| `serviceArea` | string | Declared service area |
| `rating` / `ratingCount` | float / int64 | Aggregated review statistics |
| `version` | int | Optimistic concurrency version |
| `createdAt` / `updatedAt` | time | Timestamps |

Driver status transitions are enforced through an explicit transition map:

```
OFFLINE    --GoOnline--------> AVAILABLE
AVAILABLE  --GoOffline-------> OFFLINE
AVAILABLE  --Reserve---------> BUSY
BUSY       --CompleteDelivery> AVAILABLE
```

Guards applied before a transition:

- `GoOnline` and `Reserve` fail if the driver is blocked, inactive, or already in the target state.
- `GoOffline` only succeeds from `AVAILABLE`.
- `SUSPENDED` and `BLOCKED` are administrative states with no outbound transitions in the map.

### Assignment Aggregate

Assignments live in the `assignments` collection. The assignment ID is deterministic: `<deliveryId>-<driverId>`, so re-offers of the same delivery to the same driver converge on one record.

| Field | Description |
| --- | --- |
| `driverId`, `deliveryId`, `customerId` | Participants |
| `status` | `NONE`, `OFFERED`, `ACCEPTED`, `ACTIVE`, `COMPLETED`, `REJECTED`, `EXPIRED`, `CANCELLED` |
| `attemptNumber` | Incremented on every offer |
| `offeredAt`, `expiresAt` | Offer window (default TTL 30 seconds) |
| `acceptedAt`, `rejectedAt`, `completedAt` | Terminal timestamps |

Assignment status transitions:

```
NONE      --create--> OFFERED
OFFERED   --accept--> ACCEPTED      OFFERED --reject--> REJECTED     OFFERED --expire--> EXPIRED
ACCEPTED  --start---> ACTIVE
ACTIVE    --complete> COMPLETED
REJECTED  --cancel--> CANCELLED     EXPIRED --cancel--> CANCELLED
```

### Dispatch Policy

`domain.DispatchPolicy` encapsulates dispatch rules: maximum attempts (default 5), search radius (default 5 km), assignment timeout (default 30 s), and candidate pool size (default 10). Candidates are ranked by distance (ascending); the top N are selected for the offer round.

### Review Entity

Reviews live in the `reviews` collection. Rating is validated to the range 1.0 to 5.0. One review per user per delivery is enforced by the rate-driver handler, which also verifies delivery ownership and driver assignment through the delivery-service client.

### Error Catalog

All domain errors carry a stable machine-readable code plus a message: `driver_not_found`, `driver_not_available_for_assignment`, `driver_already_reserved`, `driver_already_online`, `driver_already_offline`, `assignment_not_found`, `assignment_invalid_state`, `assignment_expired`, `invalid_transition`, `invalid_argument`, `max_attempts_reached`, `driver_blocked`, `driver_not_active`, `invalid_rating`, `duplicate_review`, `driver_already_exists`, `not_delivery_owner`, `not_assigned_driver`, and `internal_error`. The gRPC layer maps these codes to appropriate gRPC status codes and localized messages.

## Dispatch Flow

The dispatch pipeline is the heart of the service:

1. **Trigger**: a `delivery.created` event arrives on Kafka (or the delivery-service calls `FindAvailableDrivers` over gRPC, or a driver self-accepts through GraphQL).
2. **Candidate search**: `DispatchService.FindAvailableDrivers` queries Redis GEO (`drivers:locations`, sorted ascending by distance) and batch-fetches the drivers from MongoDB. It filters out excluded drivers (for example, drivers that already rejected this delivery), non-`AVAILABLE` drivers, blocked drivers, inactive drivers, and mismatched vehicle types. If GEO returns nothing, it falls back to a MongoDB location query with lower ranking priority.
3. **Ranking**: `DispatchPolicy.RankCandidates` sorts candidates by distance and truncates to the candidate pool size.
4. **Reservation**: `ReserveDriver` acquires a per-driver Redis lock (`lock:driver:<id>`, SET NX), verifies the driver is still `AVAILABLE`, transitions the driver to `BUSY`, and persists an `OFFERED` assignment with a 30-second TTL. If the assignment write fails, the driver state is rolled back. On success the `driver.assignment.offered` event is published.
5. **Driver decision**: the driver accepts or rejects through GraphQL (`acceptAssignment`, `rejectAssignment`, `selfAcceptDelivery`). Accepting moves the assignment to `ACCEPTED` and publishes `driver.assignment.accepted`; rejecting moves it to `REJECTED`, releases the driver back to `AVAILABLE`, and publishes `driver.assignment rejected`.
6. **Expiry**: the assignment expiry worker runs every 15 seconds, expires offers past their TTL, releases the drivers, and publishes `driver.assignment.expired`.
7. **Reconciliation**: the reconciliation worker runs every 60 seconds and repairs any driver stuck in `BUSY` without an active assignment.
8. **Failure signaling**: when no candidate can be reserved, the consumer publishes `driver.no_driver_available` with a reason (`NO_DRIVERS_NEARBY` or `ALL_DRIVERS_BUSY`) and an attempt number.

## GraphQL API

The service is an Apollo Federation v2.3 subgraph. It exposes `_service` for SDL introspection and uses `@key`, `@shareable`, and `@external` directives, so it composes cleanly behind the API gateway with the user and delivery subgraphs.

Endpoint: `POST http://localhost:4008/driver/graphql` (also served at `/graphql`).

### Request Headers

| Header | Purpose |
| --- | --- |
| `x-user-id` | Identity of the calling user; resolves `myDriverProfile`, driver-scoped mutations |
| `x-user-role` | Role check; `admin` is required for block, unblock, activate, deactivate |
| `x-lang` or `Accept-Language` | Response language: `en` or `ar` |

### Queries

| Query | Arguments | Description |
| --- | --- | --- |
| `_service` | - | Returns the subgraph SDL |
| `driverServiceInfo` | - | Service name, version, status |
| `driver` | `id` | Fetch a driver by ID |
| `myDriverProfile` | - | Profile for the authenticated user |
| `driverActiveAssignment` | `driverId` | Current active assignment for a driver |
| `driverStatus` | `driverId` | Status, active assignment presence, last seen |
| `nearbyDrivers` | `input` (latitude, longitude, radiusKm, vehicleType, limit) | Distance-sorted nearby drivers |
| `assignment` | `id` | Fetch an assignment by ID |
| `dispatchAttempts` | `deliveryId` | Full dispatch attempt history for a delivery |
| `driverReviews` | `driverId`, `page`, `limit` | Paginated reviews with average rating and totals |
| `availableDeliveries` | - | Open deliveries fetched from the delivery-service |

### Mutations

| Mutation | Arguments | Description |
| --- | --- | --- |
| `registerDriver` | `input` (userId, vehicleType, plateNumber, capacityKg, capabilities, serviceArea) | Registers a driver; promotes the user role to DRIVER via user-service gRPC |
| `updateDriverProfile` | `input` (driverId, vehicleType, plateNumber, capacityKg, serviceArea) | Updates profile fields |
| `goOnline` | `idempotencyKey` | `OFFLINE` to `AVAILABLE`; publishes `driver.available` |
| `goOffline` | `idempotencyKey` | `AVAILABLE` to `OFFLINE`; publishes `driver.unavailable` |
| `acceptAssignment` | `assignmentId`, `idempotencyKey` | Accepts an offered assignment |
| `rejectAssignment` | `assignmentId`, `reason`, `idempotencyKey` | Rejects and releases the driver |
| `selfAcceptDelivery` | `deliveryId`, `idempotencyKey` | Reserve and accept in one step for open-delivery claiming |
| `suspendDriver` | `driverId`, `reason` | Sets status to `SUSPENDED` |
| `activateDriver` | `driverId` | Admin only; sets `isActive` true |
| `deactivateDriver` | `driverId` | Admin only; sets `isActive` false and forces `AVAILABLE` drivers offline |
| `blockDriver` | `driverId`, `reason` | Admin only; sets `isBlocked` true |
| `unblockDriver` | `driverId`, `reason` | Admin only; sets `isBlocked` false |
| `rateDriver` | `input` (driverId, deliveryId, rating, comment) | Creates a review and updates the driver's aggregate rating |

Every response follows a uniform envelope: `success`, `statusCode`, `message`, `timeStamp`, `data`.

### Example Request

```bash
curl -X POST http://localhost:4008/driver/graphql \
  -H "Content-Type: application/json" \
  -H "x-user-id: user-123" \
  -H "x-lang: en" \
  -d '{"query":"mutation { goOnline(idempotencyKey: \"key-1\") { success statusCode message data { id status } } }"}'
```

## gRPC API

Service: `driver.DriverService` (reflection enabled, so `grpcurl` works out of the box).

| RPC | Request | Response | Description |
| --- | --- | --- | --- |
| `FindAvailableDrivers` | latitude, longitude, radiusKm, vehicleType, deliveryId, correlationId | candidates (driverId, distanceMeters, vehicleType) | Ranked nearby available drivers |
| `ReserveDriver` | driverId, deliveryId, idempotencyKey, correlationId | reserved, driverId, deliveryId, assignmentId | Atomically reserve a driver |
| `ReleaseDriver` | driverId, deliveryId, reason, correlationId | released, driverId | Release a driver from an assignment (idempotent) |
| `AssignDriver` | driverId, deliveryId, correlationId | assigned, driverId, deliveryId, assignmentId | Reserve alias used by the delivery-service |
| `GetDriver` | driverId | found, driverId, userId, status, vehicleType, isBlocked | Driver lookup |
| `GetDriverStatus` | driverId | driverId, status, hasActiveAssignment, activeDeliveryId | Status lookup |
| `ValidateID` | id, idType (`driver` or `user`), context | valid, driverId, status, message | Cross-service ID validation |

The gRPC server installs three interceptors: metadata propagation, Prometheus metrics, and OpenTelemetry tracing. Error responses are localized using the `x-lang` or `accept-language` metadata key.

## Event-Driven Integration

### Kafka

Topics are ensured on startup (1 partition, replication factor 1) and events are published with a versioned envelope (`eventID`, `eventType`, `correlationID`, `payload`).

**Published topics**: `driver.created`, `driver.updated`, `driver.deleted`, `driver.available`, `driver.unavailable`, `driver.assignment.offered`, `driver.assignment.accepted`, `driver.assignment.rejected`, `driver.assignment.expired`, `driver.assignment.released`, `driver.assignment.completed`, `driver.location.updated`, `driver.no_driver_available`, `driver.search.retry`.

**Consumed topic**: `delivery.created` (group `<KAFKA_GROUP_ID>-delivery-created`). The consumer parses the pickup coordinates (supporting both flat and nested location shapes), excludes drivers that already rejected the delivery, searches for candidates within 50 km, reserves the first available driver, and publishes `driver.no_driver_available` when nothing can be reserved. Malformed envelopes are treated as permanent errors and skipped; the consumer retries with a 3-second backoff loop.

### RabbitMQ (optional)

When `RABBITMQ_URL` is set, the service connects, ensures the dispatch topology (`delivery.dispatch.direct` exchange with dispatch request/response queues, plus the drivers events exchange), and starts a dispatch consumer. RabbitMQ failures are non-fatal; Kafka remains the active transport.

### NATS

The NATS adapter publishes `driver.location.updated` messages for real-time location fan-out.

## Background Workers

All workers run inside a worker pool started at boot and stop cleanly on SIGINT/SIGTERM:

| Worker | Interval | Responsibility |
| --- | --- | --- |
| Assignment expiry | 15 s | Expire `OFFERED` assignments past their TTL, release drivers, publish expired events |
| Reconciliation | 60 s | Detect drivers in `BUSY` with no active assignment and repair them to `AVAILABLE` |
| Heartbeat monitor | 30 s | Track `AVAILABLE` drivers for liveness monitoring |
| Delivery consumer | continuous | Kafka `delivery.created` loop with retry and graceful shutdown |

## Configuration

All configuration is environment-driven with sensible defaults (see `internal/config/config.go`):

| Variable | Default | Description |
| --- | --- | --- |
| `PORT_DRIVER_GRAPHQL` | `4008` | GraphQL and health HTTP port |
| `PORT_DRIVER_GRPC` | `50055` | gRPC server port |
| `PORT_DRIVER_METRICS` | `9105` | Prometheus metrics port |
| `MONGODB_URI` | `mongodb://localhost:27017` | MongoDB connection string |
| `MONGODB_DATABASE` | `driver_db` | MongoDB database name |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated Kafka broker list |
| `KAFKA_GROUP_ID` | `driver-service` | Consumer group ID |
| `KAFKA_CLIENT_ID` | `driver-service` | Kafka client ID |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL |
| `DISPATCH_SEARCH_RADIUS_KM` | `5` | Default dispatch search radius |
| `ASSIGNMENT_TIMEOUT_SECONDS` | `20` | Assignment offer timeout |
| `LOCATION_STALE_SECONDS` | `30` | Location staleness threshold |
| `LOCK_TTL_MS` | `5000` | Driver lock TTL |
| `MAX_DISPATCH_ATTEMPTS` | `5` | Maximum dispatch attempts per delivery |
| `USER_SERVICE_URL` | `user-srv:50051` | User service gRPC address |
| `DELIVERY_SERVICE_URL` | `delivery-srv:50054` | Delivery service gRPC address |
| `OTEL_ENDPOINT` | empty | OTLP gRPC collector endpoint; tracing is skipped when unset |
| `RABBITMQ_URL` | unset | Enables the RabbitMQ adapters when present |

## Storage

### MongoDB Collections

| Collection | Contents |
| --- | --- |
| `drivers` | Driver aggregates |
| `assignments` | Assignment lifecycle records |
| `reviews` | Driver ratings and comments |
| `idempotency` | Idempotency keys for command deduplication |
| `dispatch_attempts` | Per-delivery dispatch attempt audit trail |

### Redis Keys

| Key pattern | Type | Purpose |
| --- | --- | --- |
| `drivers:locations` | GEO | Real-time driver positions for `GEOSEARCH` |
| `lock:driver:<id>` | String (SET NX) | Per-driver distributed reservation lock |
| `idempotency:<key>` | String | Short-TTL idempotency guards |

## Observability

### Metrics

The service registers Prometheus metrics under the `driver_` prefix, including:

- Dispatch: `driver_dispatch_requests_total`, `driver_dispatch_success_total`, `driver_dispatch_failure_total`, `driver_dispatch_duration_seconds`
- Assignments: `driver_assignment_total`, `driver_assignment_accept_total`, `driver_assignment_reject_total`, `driver_assignment_expire_total`
- Contention and location: `driver_reservation_conflict_total`, `driver_location_updates_total`, `driver_location_stale_total`
- Infrastructure: `driver_redis_geo_query_duration_seconds`, `driver_redis_lock_contention_total`, `driver_kafka_publish_failures_total`, `driver_kafka_consumer_lag`, `driver_nats_publish_failures_total`, `driver_grpc_request_duration_seconds`
- State gauges: `driver_driver_state`, `driver_driver_count`, `driver_assignment_state`

Scrape them from `http://localhost:9105/metrics`.

### Tracing

When `OTEL_ENDPOINT` is set, an OpenTelemetry tracer exports spans over OTLP gRPC with a 100 percent sampling rate in this service configuration. Both the gRPC server and the GraphQL HTTP server wrap requests with tracing middleware from the shared package. Tracer shutdown is handled gracefully with a 5-second deadline.

### Health Endpoints

- `GET /health/live` and `GET /health/ready` on the GraphQL port
- `GET /healthz` on both the GraphQL and metrics ports

All return a JSON payload with status, service name, and a localized message.

### Logging

Structured logging is initialized through the shared logging package with a standard-library bridge, so both `log` and `slog` output is unified.

### Internationalization

The `i18n` package ships English and Arabic catalogs covering validation messages, driver and assignment outcomes, and error messages. Language selection reads the `x-lang` header, the `lang` query parameter, or `Accept-Language` (HTTP) and the `x-lang`/`accept-language` metadata (gRPC).

## Getting Started

### Prerequisites

- Go 1.25 or newer
- Docker and Docker Compose
- Optional for full workflows: `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc`, `golangci-lint`, `staticcheck`, `grpcurl`

### Quick Start with Docker Compose

The Docker build context is the repository root (the image also compiles the shared `packages/go` module), so the compose file already points two directories up.

```bash
# Start the full stack: MongoDB, Redis, Kafka, NATS, and the service
make docker-up

# Or start only the infrastructure and run the service on the host
make infra-up
make run

# Tail logs and inspect
make docker-logs
make docker-ps
```

The compose stack exposes MongoDB on 27017, Redis on 6379, Kafka on 9092, NATS on 4222/8222, and the service on 4008/50055/9105.

### Running Locally

```bash
make deps        # download modules
make build       # build into bin/
make run         # go run ./cmd/server
```

The service pings MongoDB at startup but tolerates failures by retrying on demand, so it can boot before the database is reachable.

## Testing

### Unit Tests

Every package ships with tests (domain state machines, command handlers, resolvers, adapters, workers, config, i18n, observability).

```bash
make test        # full suite
make test-v      # verbose
make test-race   # race detector (POSIX only; needs cgo)
make test-cov    # coverage profile plus HTML report
make test-pkg PKG=./internal/domain   # single package
```

### Integration Tests

`cmd/server/main_test.go` contains wiring tests that probe the live infrastructure (MongoDB on 27018, Kafka on 9094, Redis on 6380) and skip gracefully when it is unreachable, so a stopped Docker daemon never fails the build:

```bash
make test-cmd
```

### End-to-End Tests

The e2e suite in `test/e2e` runs against real infrastructure. The dedicated compose file uses offset host ports (27018, 6380, 9094, 4223, 5673) so it can run side by side with the dev stack, and every container ships a healthcheck.

```bash
make e2e-up                 # start infrastructure and wait for health
make test-e2e               # DRIVER_E2E=1 go test ./test/e2e/... -count=1 -v
make e2e-down               # stop
make e2e-clean              # stop and delete volumes
```

The 19 scenarios cover driver registration (including auth and duplicate rejection), online/offline toggling, block/unblock (including forbidden non-admin access), suspend/activate/deactivate, profile updates, rating, the full assignment flow, open-delivery listing, the gRPC API, service introspection, the health endpoint, and Kafka, NATS, RabbitMQ, MongoDB, and Redis round trips. A stub user-service (`test/e2e/stubuser`) answers `GetUser` and `UpdateUserRole` so the registration flow completes without a live user-service.

All endpoints are overridable through `DRIVER_E2E_*` environment variables (`DRIVER_E2E_GRAPHQL`, `DRIVER_E2E_GRPC`, `DRIVER_E2E_MONGO`, `DRIVER_E2E_REDIS`, `DRIVER_E2E_NATS`, `DRIVER_E2E_KAFKA`, `DRIVER_E2E_RABBITMQ`, `DRIVER_E2E_JWT_SECRET`).

## Makefile Reference

Run `make help` for the full list. Highlights:

| Target | Description |
| --- | --- |
| `deps`, `deps-tidy`, `deps-update` | Module dependency management |
| `build`, `build-linux`, `build-all` | Local and cross-platform builds |
| `run`, `start` | Run the service |
| `fmt`, `fmt-check`, `vet`, `lint`, `staticcheck` | Code quality |
| `test`, `test-race`, `test-cov`, `test-e2e` | Testing |
| `docker-build`, `docker-up`, `docker-down` | Docker workflow |
| `infra-up`, `infra-down` | Infrastructure only |
| `e2e-up`, `e2e-down`, `e2e-clean` | E2E infrastructure |
| `proto-gen`, `proto-gen-all`, `proto-install` | Protobuf regeneration from `../../protos` |
| `mongo-shell`, `redis-cli`, `kafka-topics` | Operations helpers |
| `health`, `metrics`, `grpc-list` | Smoke checks |
| `ci-check`, `pre-commit`, `tidy-check` | Local CI gates |
| `clean`, `clean-logs`, `coverage-clean` | Cleanup |

The Makefile detects the host platform: on Windows it uses `cmd.exe` syntax and skips the race detector (it requires cgo); on Linux and macOS it uses POSIX commands.

## Docker

The Dockerfile is a two-stage build:

1. **Builder** (`golang:1.25-alpine`): copies `packages/go` and the service module files first for layer caching, runs `go mod download`, then compiles a static binary with `CGO_ENABLED=0` and stripped symbols (`-ldflags="-w -s"`). Go build and module caches are mounted for build cache reuse.
2. **Runtime** (`alpine:3.20`): installs CA certificates and timezone data, copies the binary to `/driver-service`, exposes 4008, 50055, and 9105, and runs the binary directly as the entrypoint.

Because the image needs the shared monorepo module, the build context must be the repository root (`../..` from this service directory).

## Graceful Shutdown

On SIGINT or SIGTERM the service: stops the gRPC server gracefully, cancels the worker context (halting all background loops and consumers), then shuts down the GraphQL and metrics HTTP servers with a 10-second deadline, and finally flushes the tracer and disconnects MongoDB with 5-second deadlines.
