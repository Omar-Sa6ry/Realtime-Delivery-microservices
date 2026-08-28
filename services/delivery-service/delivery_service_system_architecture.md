# Delivery Service — System Architecture Specification

**Project:** Realtime Delivery Platform  
**Service:** Delivery Service  
**Primary Language:** NestJS / TypeScript  
**Client API:** GraphQL Federation through API Gateway  
**Database:** PostgreSQL  
**Messaging:** Kafka + Transactional Outbox  
**Synchronous Internal Communication:** gRPC  
**Low-Latency Internal Messaging:** NATS where required  
**Cache / Coordination:** Redis  
**Background Jobs:** BullMQ where operationally useful  
**Patterns:** CQRS, State Machine, Saga Orchestration, Transactional Outbox, Idempotency, Optimistic/Conditional Concurrency, Retry, DLQ, Reconciliation  
**Infrastructure:** Docker, Docker Compose, Kubernetes, Skaffold  
**Observability:** OpenTelemetry, Prometheus, Grafana, Jaeger, Structured Logging  

---

# 1. Executive Summary

The Delivery Service is the **core business service** of the Realtime Delivery Platform. It owns the lifecycle and transactional state of a delivery from creation until completion, cancellation, or failure.

The service is intentionally designed as a serious distributed-systems component rather than a CRUD service. It demonstrates:

- GraphQL Federation
- PostgreSQL transactions
- CQRS logical separation
- Delivery state machine
- Transactional Outbox
- Kafka durable domain events
- Saga orchestration
- gRPC service-to-service calls
- Idempotency
- Optimistic concurrency
- Redis caching and ephemeral coordination
- Retry and timeout policies
- Dead-letter handling
- Reconciliation jobs
- Distributed tracing
- Horizontal scaling
- Kubernetes deployment

The Delivery Service is the **Saga orchestrator** for the delivery workflow. It does not own payment, driver, notification, realtime, user, media, or search data.

---

# 2. Relationship to the Overall System

The overall platform is a distributed real-time delivery platform. Its architectural principles are:

```text
Client -> GraphQL Federation -> Services
Service-to-Service synchronous -> gRPC
Durable business events -> Kafka
Low-latency transient messaging -> NATS
Browser realtime -> WebSocket via Realtime Service
Background jobs -> BullMQ
Transactional data -> PostgreSQL / MongoDB depending on service ownership
Ephemeral state / locks / cache -> Redis
Analytics -> ClickHouse
Files -> Object Storage
```

The project explicitly avoids e-commerce functionality. The core business entity is **Delivery**.

The existing architecture defines the Delivery Service as the core domain service and the Saga owner. The Driver & Dispatch Service owns driver operational data and proximity; Payment owns payment data; Notification owns notification delivery; Realtime owns WebSocket connections; Search owns OpenSearch projections; Analytics owns ClickHouse data.

---

# 3. Delivery Service Responsibilities

## 3.1 Owns

The Delivery Service owns:

- Delivery creation
- Delivery lifecycle
- Delivery state machine
- Pickup and dropoff information
- Delivery pricing snapshot
- Delivery payment status reference
- Assigned driver reference
- Delivery cancellation rules
- Delivery completion rules
- Delivery status history
- Delivery domain events
- Delivery Saga state
- Saga compensation state
- Idempotency records for delivery commands
- Transactional Outbox
- Delivery read projections if CQRS is introduced
- Delivery-specific reconciliation

## 3.2 Does Not Own

The service must NOT own:

- User authentication
- User credentials
- Driver profile
- Driver availability source of truth
- Driver location source of truth
- Payment provider credentials
- Payment transactions as source of truth
- Notification channel delivery
- WebSocket connections
- Media processing
- Search indexes
- Analytics warehouse

Never access another service's database directly.

---

# 4. High-Level Architecture

```text
                                      CLIENT
                                         |
                                      GraphQL
                                         |
                                         v
                              +----------------------+
                              |    API Gateway       |
                              | NestJS Federation    |
                              +----------+-----------+
                                         |
                                  Delivery Subgraph
                                         |
                                         v
                              +----------------------+
                              |   Delivery Service   |
                              |      NestJS          |
                              +----------+-----------+
                                         |
              +--------------------------+---------------------------+
              |                          |                           |
              v                          v                           v
        PostgreSQL                    Redis                       gRPC
              |                          |                           |
       +------+-------+          +-------+-------+          +--------+--------+
       |              |          |               |          |                 |
       v              v          v               v          v                 v
 Deliveries       Outbox     Cache / Idempotency   Locks   Driver/Dispatch  Payment
       |              |          |               |          Service           Service
       |              |          +---------------+                |
       |              |                                           |
       +--------------+                                           |
              |                                                   |
              v                                                   |
       Outbox Publisher                                            |
              |                                                   |
              v                                                   |
            Kafka <------------------------------------------------+
              |
       +------+---------+----------------+
       |                |                |
       v                v                v
 Notification       Analytics        Search
 Service            Service          Service
       |
     BullMQ
       |
     Workers

Realtime events:
Delivery Service -> NATS -> Realtime Service -> WebSocket -> Client
```

---

# 5. API Boundary

The only client-facing business API is GraphQL through Federation.

```text
Client
  |
  | GraphQL
  v
API Gateway
  |
  | Federation
  v
Delivery Subgraph
  |
  v
Delivery Application Layer
```

The Delivery Service does **not** expose REST endpoints for business operations.

Internal service APIs use gRPC where synchronous communication is required.

---

# 6. API Gateway Responsibilities

The Gateway handles platform-wide concerns before the request reaches Delivery:

```text
Request
  |
  v
Rate Limiting
  |
  v
JWT Authentication
  |
  v
GraphQL Parsing
  |
  v
Depth / Complexity Validation
  |
  v
Authorization Context
  |
  v
Federation Routing
  |
  v
Delivery Subgraph
```

The Gateway may provide:

- JWT validation
- Rate limiting
- Correlation ID
- Trace propagation
- Query complexity protection
- Federation routing
- Error normalization

It must not contain Delivery business logic.

---

# 7. Authentication and Authorization

Authentication is centralized at the Gateway, but **authorization remains domain-aware**.

Example:

```text
Gateway
  |
  | validates JWT
  v
Delivery Service
  |
  +-- customer can access own delivery
  +-- driver can update assigned delivery
  +-- admin can access administrative queries
```

The Delivery Service trusts the authenticated identity context propagated by the Gateway, but must still enforce resource ownership and role/business authorization.

Example context:

```text
userId
role
correlationId
traceId
```

Never trust a client-provided `userId` field as authorization proof.

---

# 8. Delivery Domain Model

The central aggregate is:

```text
Delivery
 |
 +-- Customer
 +-- Pickup Address
 +-- Dropoff Address
 +-- Driver Reference
 +-- Payment Reference
 +-- Status
 +-- Pricing Snapshot
 +-- Status History
 +-- Saga State
```

The Delivery Service stores references to external aggregates rather than copying their source-of-truth data unnecessarily.

---

# 9. Delivery State Machine

The service must reject invalid state transitions.

Recommended lifecycle:

```text
REQUESTED
    |
    v
SEARCHING_DRIVER
    |
    v
DRIVER_ASSIGNED
    |
    v
DRIVER_ACCEPTED
    |
    v
PICKUP_STARTED
    |
    v
PICKED_UP
    |
    v
IN_TRANSIT
    |
    v
DELIVERED
    |
    v
COMPLETED
```

Failure / terminal states:

```text
CANCELLED
PAYMENT_FAILED
DRIVER_REJECTED
DRIVER_UNAVAILABLE
DELIVERY_FAILED
```

A transition must be validated centrally, not duplicated across resolvers.

---

# 10. State Transition Rules

| Current | Command/Event | Next | Allowed Actor |
|---|---|---|---|
| REQUESTED | StartDriverSearch | SEARCHING_DRIVER | System |
| SEARCHING_DRIVER | DriverAssigned | DRIVER_ASSIGNED | System |
| DRIVER_ASSIGNED | AcceptDelivery | DRIVER_ACCEPTED | Driver |
| DRIVER_ASSIGNED | RejectDelivery | DRIVER_REJECTED / SEARCHING_DRIVER | Driver |
| DRIVER_ACCEPTED | StartPickup | PICKUP_STARTED | Driver |
| PICKUP_STARTED | MarkPickedUp | PICKED_UP | Driver |
| PICKED_UP | StartTransit | IN_TRANSIT | Driver |
| IN_TRANSIT | MarkDelivered | DELIVERED | Driver |
| DELIVERED | CompleteDelivery | COMPLETED | System/Driver |
| REQUESTED | CancelDelivery | CANCELLED | Customer/Admin |
| SEARCHING_DRIVER | CancelDelivery | CANCELLED | Customer/Admin |
| DRIVER_ASSIGNED | CancelDelivery | CANCELLED | Policy dependent |
| Any eligible state | PaymentFailed | PAYMENT_FAILED | System |

The exact cancellation policy should be represented as a domain policy, not hardcoded inside GraphQL resolvers.

---

# 11. CQRS Design

CQRS is used as a **logical separation** inside the service. It does not require separate microservices or databases in the initial implementation.

```text
                 Delivery Service
                       |
             +---------+---------+
             |                   |
          COMMANDS             QUERIES
             |                   |
             v                   v
       Write Model           Read Model
             |                   |
             +---------+---------+
                       |
                  PostgreSQL
```

## Commands

```text
CreateDelivery
CancelDelivery
StartDriverSearch
AssignDriver
AcceptDelivery
RejectDelivery
StartPickup
MarkPickedUp
StartTransit
MarkDelivered
CompleteDelivery
MarkPaymentCompleted
MarkPaymentFailed
FailDelivery
```

## Queries

```text
GetDelivery
GetActiveDelivery
GetDeliveries
GetDeliveryHistory
GetDeliveryStatus
GetDeliveryStatusHistory
GetDriverActiveDelivery
GetAllDeliveries [ADMIN]
```

A future phase may add a denormalized read projection or read replica. Do not introduce full event sourcing just to claim CQRS.

---

# 12. PostgreSQL Database

PostgreSQL is the source of truth for Delivery transactional data.

Recommended tables:

```text
users are NOT stored here as an owned table.

users
  -> User Service owns this data.

Delivery Service tables:

 deliveries
 delivery_addresses
 delivery_status_history
 saga_instances
 saga_steps
 idempotency_keys
 outbox_events
```

---

# 13. deliveries Table

Suggested schema:

```text
id                  UUID PK
customer_id         UUID NOT NULL
assigned_driver_id  UUID NULL
status              VARCHAR NOT NULL
payment_status      VARCHAR NOT NULL
payment_id          UUID NULL
currency            VARCHAR NOT NULL
base_price          NUMERIC NOT NULL
final_price         NUMERIC NULL
pickup_address_id   UUID NOT NULL
dropoff_address_id  UUID NOT NULL
proof_media_id      UUID NULL
version             BIGINT NOT NULL DEFAULT 1
created_at          TIMESTAMP NOT NULL
updated_at          TIMESTAMP NOT NULL
cancelled_at        TIMESTAMP NULL
completed_at        TIMESTAMP NULL
```

Important indexes:

```text
(customer_id, created_at DESC)
(customer_id, status)
(status)
(assigned_driver_id, status)
(created_at)
```

Use UUIDs for public identifiers and a version column for optimistic concurrency.

---

# 14. delivery_addresses Table

```text
id              UUID PK
delivery_id     UUID NOT NULL
type            PICKUP | DROPOFF
street          TEXT
city            TEXT
country         TEXT
postal_code     TEXT
latitude        DECIMAL
longitude       DECIMAL
created_at      TIMESTAMP
```

If the platform later integrates geocoding, store the resulting coordinates as a snapshot for the delivery.

The Delivery Service should not call external geocoding providers inside the database transaction.

---

# 15. delivery_status_history Table

```text
id              UUID PK
delivery_id     UUID NOT NULL
from_status     VARCHAR NULL
to_status       VARCHAR NOT NULL
changed_by      UUID NULL
actor_type      CUSTOMER | DRIVER | SYSTEM | ADMIN
reason          TEXT NULL
metadata        JSONB NULL
created_at      TIMESTAMP NOT NULL
```

This table provides an audit trail and supports status-history queries.

---

# 16. saga_instances Table

The Saga state should be persisted because a distributed workflow cannot depend only on process memory.

```text
id                  UUID PK
delivery_id         UUID UNIQUE
state               RUNNING | COMPLETED | COMPENSATING | FAILED
current_step        VARCHAR
version             BIGINT
last_error          TEXT NULL
started_at          TIMESTAMP
updated_at          TIMESTAMP
completed_at        TIMESTAMP NULL
```

---

# 17. saga_steps Table

```text
id                  UUID PK
saga_id             UUID NOT NULL
step_name           VARCHAR NOT NULL
status              PENDING | RUNNING | COMPLETED | FAILED | COMPENSATED
attempt_count       INTEGER
request_id          UUID NULL
response_metadata   JSONB NULL
error               TEXT NULL
started_at          TIMESTAMP NULL
completed_at        TIMESTAMP NULL
```

This makes Saga execution observable and recoverable.

---

# 18. Idempotency Table

For critical client commands:

```text
id                  UUID PK
idempotency_key     VARCHAR UNIQUE
user_id             UUID NOT NULL
operation           VARCHAR NOT NULL
request_hash        VARCHAR NOT NULL
status              PROCESSING | COMPLETED | FAILED
response_payload    JSONB NULL
created_at          TIMESTAMP
expires_at          TIMESTAMP NULL
```

Do not rely only on Redis for critical idempotency. PostgreSQL provides durable protection.

Redis may provide a fast first layer.

---

# 19. Transactional Outbox

The Delivery Service must never perform:

```text
DB COMMIT
   |
   v
Kafka publish
```

as two unrelated operations.

Use:

```text
BEGIN TRANSACTION
    |
    +-- Update delivery
    |
    +-- Insert status history
    |
    +-- Insert outbox event
    |
COMMIT
    |
    v
Outbox Publisher
    |
    v
Kafka
```

This guarantees that a committed domain change has a durable event waiting to be published.

---

# 20. outbox_events Table

```text
id              UUID PK
event_id        UUID UNIQUE
aggregate_id    UUID NOT NULL
aggregate_type  VARCHAR NOT NULL
event_type      VARCHAR NOT NULL
version         INTEGER NOT NULL
payload         JSONB NOT NULL
headers         JSONB NULL
published       BOOLEAN NOT NULL DEFAULT FALSE
attempts        INTEGER NOT NULL DEFAULT 0
created_at      TIMESTAMP NOT NULL
published_at    TIMESTAMP NULL
last_error      TEXT NULL
```

The publisher should claim rows safely using PostgreSQL locking patterns such as `FOR UPDATE SKIP LOCKED`.

---

# 21. Delivery Creation Flow

```text
STEP 1
Client
  |
  | createDelivery(input, idempotencyKey)
  v
GraphQL Gateway
  |
  v
Delivery Subgraph
  |
  v
Delivery Command Handler
```

Then:

```text
BEGIN TRANSACTION

1. Validate command
2. Check idempotency record
3. Validate customer context
4. Validate pickup/dropoff data
5. Calculate/store price snapshot
6. Create delivery with REQUESTED
7. Create status history
8. Create saga instance
9. Create outbox event delivery.created
10. Create outbox event delivery.search_requested

COMMIT
```

Then:

```text
Outbox Publisher
      |
      v
Kafka
      |
      +--> Notification Service
      +--> Analytics Service
      +--> Search Service
      +--> Delivery Saga consumers / workflow infrastructure
```

The GraphQL request returns after the transaction commits; it does not wait for driver assignment or payment.

---

# 22. Driver Assignment Flow

The Delivery Service does not own driver proximity.

```text
Delivery Service
      |
      | gRPC FindAvailableDriver
      v
Driver & Dispatch Service
      |
      +-- Redis GEO search
      +-- availability filtering
      +-- candidate ranking
      |
      v
Driver candidate(s)
```

Then:

```text
Delivery Service
      |
      | gRPC AssignDriver
      v
Driver & Dispatch Service
      |
      +-- acquire driver distributed lock
      +-- verify AVAILABLE
      +-- reserve driver
      +-- create assignment
      +-- notify driver through NATS/Realtime path
      v
Assignment Result
```

If the driver rejects:

```text
Driver Rejected
      |
      v
Delivery Service
      |
      +-- update state
      +-- try next candidate
      |
      +-- or cancel after policy limit
```

---

# 23. Payment Flow

Payment is a separate service.

The Delivery Service requests payment synchronously when the Saga requires an immediate decision, or reacts to durable payment events depending on the provider flow.

```text
Delivery Service
      |
      | gRPC Authorize/Capture Payment
      v
Payment Service
      |
      v
Payment Provider
```

Payment result:

```text
payment.completed
payment.failed
payment.refunded
```

These are consumed by the Delivery Saga.

Never store payment provider credentials in Delivery Service.

---

# 24. Delivery Saga

The Delivery Service is the Saga Orchestrator.

```text
                    DELIVERY SAGA
                         |
                         v
                 Create Delivery
                         |
                         v
                 Search Driver
                         |
                         v
                 Assign Driver
                         |
                         v
                 Driver Accepts
                         |
                         v
                  Process Payment
                         |
                         v
                    Start Pickup
                         |
                         v
                    In Transit
                         |
                         v
                  Complete Delivery
```

Each step must have:

- command
- timeout
- retry policy
- success event
- failure event
- compensation action
- idempotency strategy
- persisted state

---

# 25. Saga Failure Handling

## Driver Assignment Failure

```text
Assign Driver
     |
     X
Failure
     |
     +--> retry candidate
     |
     +--> next candidate
     |
     +--> no candidates
             |
             v
        Cancel Delivery
```

## Payment Failure

```text
Payment Failed
     |
     +--> Release Driver
     |
     +--> Cancel Delivery
     |
     +--> Notify Customer
```

## Delivery Failure After Payment

```text
Delivery Failed
     |
     +--> Refund Payment
     |
     +--> Release Driver
     |
     +--> Mark Delivery Failed
```

Compensation is not a database rollback. It is a new business operation.

---

# 26. Saga State Machine

```text
PENDING
  |
  v
DRIVER_SEARCH
  |
  v
DRIVER_ASSIGNED
  |
  v
WAITING_DRIVER_ACCEPTANCE
  |
  v
PAYMENT_PROCESSING
  |
  v
READY_FOR_PICKUP
  |
  v
IN_PROGRESS
  |
  v
COMPLETING
  |
  v
COMPLETED
```

Failure path:

```text
ANY STEP
   |
   v
COMPENSATING
   |
   v
FAILED / CANCELLED
```

---

# 27. Kafka Events Produced by Delivery Service

Recommended domain events:

```text
delivery.created
delivery.driver.search.started
delivery.driver.assigned
delivery.driver.accepted
delivery.driver.rejected
delivery.pickup.started
delivery.picked_up
delivery.in_transit
delivery.delivered
delivery.completed
delivery.cancelled
delivery.failed
delivery.payment.required
```

Events must describe facts that happened, not commands that should happen.

Good:

```text
delivery.created
```

Bad as a domain event:

```text
create.delivery
```

---

# 28. Kafka Event Envelope

All events should use a consistent envelope:

```json
{
  "eventId": "uuid",
  "eventType": "delivery.created",
  "version": 1,
  "aggregateId": "delivery-id",
  "aggregateType": "Delivery",
  "producer": "delivery-service",
  "occurredAt": "2026-08-22T12:00:00Z",
  "correlationId": "uuid",
  "traceId": "uuid",
  "payload": {}
}
```

Event schemas must be versioned.

---

# 29. Kafka Topic Strategy

Use a small number of durable topics rather than creating a topic for every tiny operation unless scale or ownership requires it.

Example:

```text
delivery.events.v1
payment.events.v1
driver.events.v1
```

The event type is carried in the envelope.

Possible consumer groups:

```text
notification-service
analytics-service
search-service
delivery-saga
```

Kafka partitioning should preserve ordering for the same delivery.

Recommended partition key:

```text
deliveryId
```

This ensures events for one delivery are routed consistently to one partition.

---

# 30. Kafka Consumer: Delivery Service

The Delivery Service may consume external events required to advance its Saga.

Examples:

```text
payment.completed
    -> MarkPaymentCompleted
    -> Advance Saga

payment.failed
    -> MarkPaymentFailed
    -> Start Compensation

driver.accepted
    -> MarkDriverAccepted
    -> Advance Saga

driver.rejected
    -> MarkDriverRejected
    -> Search another driver / cancel
```

Every consumer must be idempotent.

---

# 31. Idempotent Event Consumption

Use:

```text
Redis fast-path
       |
       v
PostgreSQL durable record
```

or a durable event-consumer table.

Example:

```text
event_id = abc
consumer = delivery-saga
```

If already processed, ignore the duplicate.

Never assume Kafka delivers a business event exactly once.

---

# 32. Redis Usage

Redis is **not** the Delivery transactional source of truth.

Use Redis for:

- Short-lived cache
- Idempotency fast-path
- Rate limiting where applicable
- Saga locks when needed
- Temporary coordination
- Request deduplication
- Hot delivery read cache

Do not store the canonical delivery status only in Redis.

Canonical state:

```text
PostgreSQL
```

---

# 33. Redis Cache Strategy

Possible keys:

```text
delivery:{deliveryId}
delivery:active:{customerId}
delivery:status:{deliveryId}
```

Use TTLs.

Example:

```text
delivery:{id} -> 30-120 seconds
```

For status updates, invalidate or update the cache after the PostgreSQL transaction succeeds.

Cache-aside pattern:

```text
Query
 |
v
Redis
 |
 +-- HIT -> return
 |
 +-- MISS
      |
      v
 PostgreSQL
      |
      v
 Redis SET
```

Never cache authorization decisions without carefully considering invalidation.

---

# 34. Optimistic Concurrency

Delivery state transitions are vulnerable to race conditions.

Example:

```text
Driver A -> AcceptDelivery
Driver B -> AcceptDelivery
```

Only one operation should succeed.

Use a version field:

```sql
UPDATE deliveries
SET status = 'DRIVER_ACCEPTED', version = version + 1
WHERE id = :id
  AND status = 'DRIVER_ASSIGNED'
  AND version = :version;
```

If affected rows = 0, the transition lost the race and must fail safely.

---

# 35. PostgreSQL Transaction Boundary

A delivery command should generally follow:

```text
BEGIN
 |
 +-- Load delivery
 +-- Validate state
 +-- Validate authorization
 +-- Apply state transition
 +-- Write status history
 +-- Update saga state if applicable
 +-- Write outbox event(s)
 |
COMMIT
```

Do not call remote services while holding the database transaction open.

Bad:

```text
BEGIN
  UPDATE delivery
  gRPC Payment
  gRPC Driver
COMMIT
```

Good:

```text
BEGIN
  Update local state
  Write outbox / saga command state
COMMIT

Then execute remote operation
```

This prevents long transactions and database connection starvation.

---

# 36. gRPC Responsibilities

Use gRPC for synchronous calls where the caller needs an immediate answer.

Delivery may call:

```text
DriverService.FindAvailableDrivers
DriverService.AssignDriver
DriverService.ReleaseDriver
DriverService.GetDriverAssignment

PaymentService.AuthorizePayment
PaymentService.CapturePayment
PaymentService.RefundPayment
```

Exact contracts must be defined in `.proto` files.

---

# 37. Example Delivery gRPC Contract

If another service needs Delivery information, define an explicit contract:

```proto
service DeliveryService {
  rpc GetDelivery(GetDeliveryRequest) returns (GetDeliveryResponse);
  rpc GetDeliveryStatus(GetDeliveryStatusRequest) returns (GetDeliveryStatusResponse);
}
```

Do not expose internal database models directly through protobuf.

---

# 38. NATS Usage

NATS is not the durable business-event backbone.

Use NATS when the operation needs low latency and transient delivery is acceptable, especially for realtime coordination.

Example:

```text
Delivery Service
      |
      | NATS
      v
Realtime Service
      |
      v
WebSocket
      |
      v
Customer
```

Examples:

```text
delivery.status.changed
 delivery.driver.assigned
 delivery.tracking.started
```

For durable business facts consumed by multiple independent consumers, use Kafka.

---

# 39. NATS JetStream Decision

JetStream provides persistence on top of NATS.

For this project:

```text
Kafka = primary durable business event platform
NATS Core = low-latency transient messaging
JetStream = optional/future infrastructure
```

Do not introduce JetStream merely to duplicate Kafka's role.

If a future requirement needs NATS-native persistence, replay, or work queues, JetStream can be introduced with a clearly defined responsibility.

---

# 40. BullMQ Usage

BullMQ is not required for the core synchronous Delivery command path.

Use BullMQ for operational background work such as:

```text
stuck-saga-recheck
reconciliation
expired-delivery cleanup
periodic consistency checks
non-critical asynchronous calculations
```

Kafka remains the durable event stream.

BullMQ remains the background job execution mechanism.

---

# 41. Retry Policy

Retries must be bounded.

Example:

```text
Attempt 1 -> 250ms + jitter
Attempt 2 -> 500ms + jitter
Attempt 3 -> 1s + jitter
Attempt 4 -> 2s + jitter
Attempt 5 -> fail
```

Use exponential backoff and jitter.

Do not retry:

- invalid input
- authorization failures
- invalid state transitions
- deterministic business rule failures

Retry:

- transient network errors
- temporary service unavailable
- connection reset
- retryable provider failures

---

# 42. Timeouts

Every remote operation must have a timeout.

Example conceptual policy:

```text
gRPC Driver lookup      -> short timeout
gRPC Driver assignment  -> short timeout
gRPC Payment authorize  -> provider-aware timeout
Kafka publish           -> bounded timeout
Redis                   -> very short timeout
```

Never allow an unbounded remote request to hold a worker forever.

---

# 43. Circuit Breaker

A circuit breaker may be used around unstable synchronous dependencies.

```text
CLOSED
  |
  | repeated failures
  v
OPEN
  |
  | cooldown
  v
HALF_OPEN
  |
  +-- success -> CLOSED
  +-- failure -> OPEN
```

The circuit breaker should protect the Delivery Service from cascading failures.

---

# 44. Dead Letter Queue

Kafka consumer failures should eventually reach a DLQ strategy.

Example:

```text
delivery.events.v1
       |
       v
Delivery Consumer
       |
       +-- success
       |
       +-- retry
       |
       +-- retry
       |
       +-- permanent failure
                |
                v
        delivery.events.dlq
```

DLQ records should contain:

- original event
- event ID
- consumer group
- error
- stack trace/reference
- retry count
- timestamp

DLQ does not mean silently dropping the event.

---

# 45. Reconciliation

Distributed systems can become stuck even when every individual component is technically correct.

Use a reconciliation process to find:

```text
REQUESTED too long
SEARCHING_DRIVER too long
DRIVER_ASSIGNED without acceptance
PAYMENT_PROCESSING too long
SAGA RUNNING too long
OUTBOX events not published
```

Example:

```text
Cron / BullMQ
     |
     v
Reconciliation Worker
     |
     +--> find stuck deliveries
     +--> inspect saga state
     +--> retry safe step
     +--> emit alert
```

Reconciliation operations must themselves be idempotent.

---

# 46. Cancellation

Cancellation is a domain operation, not simply:

```text
DELETE FROM deliveries
```

Instead:

```text
CancelDelivery
   |
   +-- validate current state
   +-- validate actor
   +-- execute compensation if needed
   +-- transition to CANCELLED
   +-- write status history
   +-- publish delivery.cancelled
```

Delivery records should normally remain for audit/history.

---

# 47. Completion Flow

```text
Driver
  |
  | complete delivery
  v
Delivery Service
  |
  +-- validate driver ownership
  +-- validate state == DELIVERED / IN_TRANSIT according to policy
  +-- validate proof if required
  +-- update status
  +-- write status history
  +-- write outbox event
  v
COMPLETED
```

Completion must be idempotent.

A duplicate completion request must not create duplicate completion side effects.

---

# 48. Proof of Delivery

The Delivery Service should reference Media Service data rather than storing file bytes.

```text
Driver
  |
  v
Media Service
  |
  v
Object Storage
  |
  v
mediaId
  |
  v
Delivery Service
```

Example:

```text
proof_media_id = UUID
```

Media Service owns the actual media metadata and object storage workflow.

---

# 49. Realtime Integration

The Delivery Service should not own browser WebSocket connections.

Instead:

```text
Delivery Service
      |
      | delivery.status.changed
      v
NATS
      |
      v
Realtime Service
      |
      v
WebSocket
      |
      v
Customer / Driver
```

The Realtime Service owns connection state and fan-out.

---

# 50. Notification Integration

Delivery Service should not send email/SMS/push directly.

It publishes durable events:

```text
delivery.created
delivery.driver.assigned
delivery.driver.accepted
delivery.in_transit
delivery.completed
delivery.cancelled
delivery.failed
```

Notification Service consumes them and uses BullMQ for channel-specific delivery.

```text
Kafka
  |
  v
Notification Service
  |
  v
BullMQ
  |
  +--> Email Worker
  +--> Push Worker
  +--> In-App Worker
```

---

# 51. Search Integration

Delivery Service is the source of truth.

Search Service maintains a read projection in OpenSearch.

```text
Delivery PostgreSQL
      |
      v
Outbox
      |
      v
Kafka
      |
      v
Search Service
      |
      v
OpenSearch
```

Search is eventually consistent.

Never write directly from Delivery Service to OpenSearch.

---

# 52. Analytics Integration

Analytics receives events asynchronously:

```text
Delivery Service
      |
      v
Kafka
      |
      v
Analytics Service
      |
      v
ClickHouse
```

This keeps analytical workloads away from PostgreSQL transactions.

---

# 53. GraphQL Schema — Commands

Recommended Federation operations:

```graphql
mutation createDelivery(input: CreateDeliveryInput!): Delivery!
mutation cancelDelivery(id: ID!, reason: String): Delivery!
mutation acceptDelivery(id: ID!): Delivery!
mutation rejectDelivery(id: ID!, reason: String): Delivery!
mutation startPickup(id: ID!): Delivery!
mutation markPickedUp(id: ID!): Delivery!
mutation startTransit(id: ID!): Delivery!
mutation markDelivered(id: ID!): Delivery!
mutation completeDelivery(id: ID!, proofMediaId: ID): Delivery!
```

Do not expose internal Saga commands directly to clients.

---

# 54. GraphQL Schema — Queries

```graphql
query delivery(id: ID!): Delivery
query activeDelivery: Delivery
query deliveries(page: Int, limit: Int, status: DeliveryStatus): DeliveryConnection!
query deliveryHistory(page: Int, limit: Int): DeliveryConnection!
query deliveryStatus(id: ID!): DeliveryStatus!
query deliveryStatusHistory(id: ID!): [DeliveryStatusHistory!]!
```

Administrative operations must be protected by authorization policies.

---

# 55. GraphQL Delivery Type

Conceptually:

```graphql
type Delivery {
  id: ID!
  customerId: ID!
  driverId: ID
  status: DeliveryStatus!
  paymentStatus: PaymentStatus!
  price: Money!
  pickup: DeliveryAddress!
  dropoff: DeliveryAddress!
  proofMediaId: ID
  createdAt: DateTime!
  updatedAt: DateTime!
}
```

Avoid exposing internal database columns such as `version`, `outbox_id`, or internal Saga implementation details unless there is a clear client requirement.

---

# 56. Pagination

For normal delivery history:

```text
cursor-based pagination
```

is preferable for large datasets.

For example:

```graphql
query deliveries(after: String, first: Int)
```

For administrative screens, offset pagination may be acceptable initially.

Do not load unlimited delivery history.

---

# 57. Authorization Matrix

| Operation | Customer | Driver | Admin |
|---|---:|---:|---:|
| Create delivery | Yes | Policy dependent | Yes |
| View own delivery | Yes | Assigned only | Yes |
| Cancel own delivery | Yes if eligible | No | Yes |
| Accept assignment | No | Assigned driver | Admin override only |
| Reject assignment | No | Assigned driver | Admin override only |
| Start pickup | No | Assigned driver | Controlled |
| Mark picked up | No | Assigned driver | Controlled |
| Start transit | No | Assigned driver | Controlled |
| Complete delivery | No | Assigned driver | Controlled |
| View all deliveries | No | No | Yes |

Authorization must be evaluated using authenticated identity and domain state.

---

# 58. Service Folder Structure

Recommended NestJS structure:

```text
apps/delivery/
├── src/
│   ├── main.ts
│   │
│   ├── app.module.ts
│   │
│   ├── config/
│   │   ├── configuration.ts
│   │   └── validation.ts
│   │
│   ├── presentation/
│   │   ├── graphql/
│   │   │   ├── delivery.resolver.ts
│   │   │   ├── delivery.types.ts
│   │   │   ├── delivery.inputs.ts
│   │   │   └── delivery.schema.ts
│   │   │
│   │   └── grpc/
│   │       └── delivery.controller.ts
│   │
│   ├── application/
│   │   ├── commands/
│   │   │   ├── create-delivery/
│   │   │   ├── cancel-delivery/
│   │   │   ├── assign-driver/
│   │   │   ├── accept-delivery/
│   │   │   ├── reject-delivery/
│   │   │   ├── start-pickup/
│   │   │   ├── mark-picked-up/
│   │   │   ├── start-transit/
│   │   │   ├── mark-delivered/
│   │   │   └── complete-delivery/
│   │   │
│   │   ├── queries/
│   │   │   ├── get-delivery/
│   │   │   ├── get-active-delivery/
│   │   │   ├── list-deliveries/
│   │   │   └── get-status-history/
│   │   │
│   │   ├── saga/
│   │   │   ├── delivery-saga.orchestrator.ts
│   │   │   ├── saga-state.service.ts
│   │   │   └── compensation.service.ts
│   │   │
│   │   └── services/
│   │       ├── pricing.service.ts
│   │       └── authorization.service.ts
│   │
│   ├── domain/
│   │   ├── entities/
│   │   │   └── delivery.entity.ts
│   │   ├── value-objects/
│   │   │   ├── money.vo.ts
│   │   │   ├── address.vo.ts
│   │   │   └── delivery-status.vo.ts
│   │   ├── state-machine/
│   │   │   └── delivery-state-machine.ts
│   │   ├── events/
│   │   └── errors/
│   │
│   ├── infrastructure/
│   │   ├── persistence/
│   │   │   ├── postgres/
│   │   │   ├── repositories/
│   │   │   └── migrations/
│   │   ├── kafka/
│   │   │   ├── producers/
│   │   │   └── consumers/
│   │   ├── grpc/
│   │   │   ├── driver.client.ts
│   │   │   └── payment.client.ts
│   │   ├── redis/
│   │   ├── outbox/
│   │   │   ├── outbox.publisher.ts
│   │   │   └── outbox.repository.ts
│   │   └── jobs/
│   │       └── reconciliation.processor.ts
│   │
│   └── health/
│       └── health.controller.ts
│
├── test/
│   ├── unit/
│   ├── integration/
│   ├── contract/
│   └── e2e/
│
├── Dockerfile
├── package.json
└── tsconfig.json
```

Note: health/operational endpoints may exist for Kubernetes probes if required by the deployment environment; business APIs remain GraphQL/gRPC and are not REST.

---

# 59. Application Layer Rules

Resolvers must be thin.

Bad:

```text
Resolver
  -> repository
  -> state transition
  -> Kafka
  -> payment
```

Good:

```text
Resolver
  -> Command Handler
       -> Domain
       -> Transaction
       -> Outbox
```

The application layer coordinates use cases. The domain layer owns business rules.

---

# 60. Domain Layer Rules

The domain layer should know nothing about:

- NestJS
- GraphQL
- Kafka
- Redis
- PostgreSQL
- gRPC
- BullMQ

It should implement:

```text
state transition rules
pricing/domain policies
cancellation rules
completion rules
invariants
```

This makes the core business logic testable.

---

# 61. Shared NestJS Packages

A common package may be used for reusable technical infrastructure.

Allowed examples:

```text
@platform/common
@platform/logger
@platform/observability
@platform/grpc
@platform/kafka
@platform/config
@platform/errors
```

Do NOT put Delivery business logic inside shared packages.

Bad:

```text
common/delivery-state-machine
```

Good:

```text
apps/delivery/src/domain/state-machine
```

Shared packages should reduce duplication without coupling business domains.

---

# 62. Docker

The Delivery Service must have an independent Docker image.

Development:

```text
Docker Compose
  |
  +-- delivery-service
  +-- postgres
  +-- redis
  +-- kafka
  +-- nats
```

Production:

```text
Kubernetes
  |
  +-- delivery deployment
  +-- delivery service
  +-- config
  +-- secrets
  +-- HPA
```

---

# 63. Kubernetes Deployment

Recommended resources:

```text
Deployment
Service
ConfigMap
Secret
HPA
PodDisruptionBudget [optional]
NetworkPolicy [optional]
ServiceMonitor [if Prometheus Operator is used]
```

Health probes:

```text
liveness
readiness
startup
```

Readiness should verify the service can safely accept traffic, not necessarily that every downstream dependency is healthy.

---

# 64. Skaffold

Skaffold is used for local Kubernetes development.

Expected workflow:

```text
Code Change
   |
   v
Skaffold Detect
   |
   v
Build Image
   |
   v
Deploy to Local Kubernetes
   |
   v
Port Forward / Test
```

Docker Compose remains useful for fast local infrastructure development; Skaffold is used when validating Kubernetes behavior.

---

# 65. Configuration

Configuration should come from environment variables / ConfigMaps / Secrets.

Example:

```text
NODE_ENV
PORT
GRPC_PORT
DATABASE_URL
REDIS_URL
KAFKA_BROKERS
KAFKA_CLIENT_ID
KAFKA_GROUP_ID
NATS_URL
DRIVER_GRPC_URL
PAYMENT_GRPC_URL
OTEL_EXPORTER_OTLP_ENDPOINT
```

Secrets must never be committed.

---

# 66. Observability

Every important Delivery operation should include:

```text
requestId
correlationId
traceId
userId
customerId
deliveryId
sagaId
eventId
```

Structured logs should make one delivery traceable across services.

---

# 67. Metrics

Important metrics:

```text
delivery_create_total
delivery_create_failures_total
delivery_state_transition_total
delivery_completion_total
delivery_cancellation_total
saga_started_total
saga_completed_total
saga_failed_total
saga_compensation_total
outbox_pending_total
outbox_publish_failures_total
kafka_consumer_lag
grpc_request_duration
redis_operation_duration
postgres_query_duration
```

Business metrics:

```text
average delivery duration
assignment success rate
payment failure rate
driver rejection rate
cancellation rate
completion rate
```

---

# 68. Distributed Tracing

A request should be traceable:

```text
GraphQL Gateway
      |
      v
Delivery Service
      |
      +--> PostgreSQL
      |
      +--> Driver gRPC
      |
      +--> Payment gRPC
      |
      +--> Kafka
             |
             +--> Notification
             +--> Analytics
             +--> Search
      |
      +--> NATS
             |
             v
         Realtime
```

Use OpenTelemetry context propagation across HTTP/GraphQL, gRPC, Kafka, and NATS where supported.

---

# 69. Error Model

Errors should be categorized:

```text
VALIDATION_ERROR
UNAUTHORIZED
FORBIDDEN
NOT_FOUND
INVALID_STATE
CONFLICT
IDEMPOTENCY_CONFLICT
DEPENDENCY_TIMEOUT
DEPENDENCY_UNAVAILABLE
PAYMENT_FAILED
DRIVER_UNAVAILABLE
INTERNAL_ERROR
```

Do not expose stack traces or infrastructure details to clients.

---

# 70. Failure Scenarios

## PostgreSQL unavailable

```text
Request
  |
  v
Fail fast
  |
  v
Return controlled error
```

Do not create an in-memory fake transactional database.

## Redis unavailable

Depending on operation:

- cache: bypass if safe
- idempotency fast-path: fall back to durable DB protection
- optional coordination: fail closed when correctness requires it

## Kafka unavailable

The transaction can still commit because the outbox event is stored in PostgreSQL.

```text
DB commit
  |
  v
Outbox pending
  |
  v
Kafka recovers
  |
  v
Publisher sends event
```

## Driver Service unavailable

Use timeout + bounded retry + circuit breaker + Saga recovery.

## Payment Service unavailable

Do not mark payment as completed. Persist Saga state and retry according to policy.

---

# 71. Exactly-Once Reality

Do not design the system around the assumption that distributed operations are exactly once.

The practical model is:

```text
At-least-once delivery
        +
Idempotent consumers
        +
Idempotent commands
        +
Transactional state transitions
```

This provides effectively-once business behavior for critical operations.

---

# 72. Race Conditions

Important races include:

```text
Two accept requests
Two cancellation requests
Driver accepts after cancellation
Payment completes after cancellation
Duplicate Kafka event
Outbox publisher crash after Kafka publish
Saga worker crashes mid-step
```

Protection:

```text
PostgreSQL transaction
Optimistic concurrency
Unique constraints
Idempotency keys
Durable Saga state
Outbox
Timeouts
Compensation
```

---

# 73. Outbox Crash Scenario

Consider:

```text
1. Delivery transaction commits
2. Outbox exists
3. Publisher sends Kafka event
4. Process crashes before marking published
```

The event may be published again.

Therefore consumers must be idempotent.

This is expected behavior, not necessarily a bug.

---

# 74. Saga Crash Scenario

```text
Saga Step: PAYMENT_PROCESSING
        |
        v
Process crashes
```

After restart:

```text
Load saga from PostgreSQL
        |
        v
Inspect current step
        |
        v
Check idempotency / payment state
        |
        v
Continue or compensate
```

Never keep the authoritative Saga state only in memory.

---

# 75. Delivery Creation — Complete Sequence

```text
1. Client sends createDelivery GraphQL mutation.
2. Gateway validates JWT.
3. Gateway applies rate limit.
4. Gateway propagates user/correlation/trace context.
5. Federation routes to Delivery Subgraph.
6. Resolver invokes CreateDelivery command.
7. Command validates input.
8. Command validates authorization.
9. Idempotency is checked.
10. PostgreSQL transaction begins.
11. Delivery is created as REQUESTED.
12. Addresses are persisted.
13. Status history is persisted.
14. Saga instance is persisted.
15. delivery.created outbox event is persisted.
16. delivery.driver.search.started outbox event is persisted.
17. Transaction commits.
18. Response returns to client.
19. Outbox publisher publishes events to Kafka.
20. Consumers process events independently.
```

---

# 76. Driver Assignment — Complete Sequence

```text
1. Saga observes driver-search step.
2. Delivery Service calls Driver Service through gRPC.
3. Driver Service performs Redis GEO search.
4. Driver Service filters available drivers.
5. Candidate list is returned.
6. Delivery Service chooses candidate according to policy.
7. Delivery Service requests assignment.
8. Driver Service acquires driver lock.
9. Driver Service validates availability.
10. Driver Service reserves driver.
11. Assignment result is returned.
12. Delivery transaction updates assigned_driver_id.
13. Status changes to DRIVER_ASSIGNED.
14. Outbox event is created.
15. Kafka receives durable event.
16. Realtime notification can be emitted through NATS.
17. Driver receives assignment over WebSocket.
```

---

# 77. Driver Rejection — Complete Sequence

```text
1. Driver rejects assignment.
2. Realtime Service receives driver action.
3. Driver Service validates assignment.
4. Driver Service releases driver.
5. Driver event is published.
6. Delivery Service consumes driver.rejected.
7. Delivery validates current state.
8. Delivery records rejection.
9. Saga chooses next candidate or cancellation.
10. Notification is emitted.
```

---

# 78. Payment Failure — Complete Sequence

```text
1. Saga starts payment.
2. Payment Service receives request.
3. Payment fails.
4. Payment Service publishes payment.failed.
5. Delivery Service consumes event.
6. Event is checked for idempotency.
7. Delivery payment state becomes FAILED.
8. Saga enters COMPENSATING.
9. Driver assignment is released.
10. Delivery becomes PAYMENT_FAILED / CANCELLED according to policy.
11. Notification event is emitted.
```

---

# 79. Successful Completion — Complete Sequence

```text
Driver
  |
  v
Realtime / Driver path
  |
  v
Delivery Service
  |
  +-- validate ownership
  +-- validate state
  +-- update status
  +-- write history
  +-- write outbox
  v
DELIVERED
  |
  v
Completion step
  |
  +-- persist proof reference
  +-- mark COMPLETED
  +-- publish delivery.completed
```

Then:

```text
delivery.completed
      |
      +--> Notification
      +--> Analytics
      +--> Search projection
      +--> Realtime
```

---

# 80. Security

Security requirements:

- JWT identity from Gateway
- Domain authorization inside Delivery
- No direct client-to-database access
- No cross-service database access
- Input validation
- GraphQL complexity limits at Gateway
- Idempotency for critical commands
- Secrets in Kubernetes Secrets / secure secret manager
- TLS for production internal traffic where required
- Audit status changes
- Avoid sensitive payment data
- Avoid logging tokens
- Avoid logging full addresses when unnecessary

---

# 81. Testing Strategy

## Unit Tests

Test:

```text
state machine
authorization policies
pricing rules
cancellation rules
completion rules
Saga transition logic
compensation logic
```

## Integration Tests

Test:

```text
PostgreSQL repositories
transactions
unique constraints
outbox
Redis integration
Kafka integration
```

## Contract Tests

Test:

```text
GraphQL schema
protobuf contracts
Kafka event schemas
```

## E2E

Test:

```text
Create Delivery
 -> Assign Driver
 -> Accept
 -> Payment
 -> Pickup
 -> Transit
 -> Complete
```

Also test failure paths.

---

# 82. Failure Injection Tests

Intentionally simulate:

```text
PostgreSQL unavailable
Redis unavailable
Kafka unavailable
Driver timeout
Driver rejection
Payment timeout
Payment failure
Duplicate event
Duplicate command
Process crash after DB commit
Process crash after Kafka publish
Saga worker crash
```

The goal is to verify behavior, not just successful requests.

---

# 83. Performance Considerations

Important bottlenecks:

```text
PostgreSQL connections
GraphQL request rate
gRPC dependency latency
Kafka throughput
Outbox polling
Saga worker concurrency
Redis latency
```

Avoid:

```text
remote calls inside DB transactions
unbounded GraphQL queries
unbounded delivery history queries
unbounded retries
```

---

# 84. Capacity Estimation

Start with explicit assumptions rather than pretending to know production traffic.

Example portfolio target:

```text
10k registered users
1k concurrent users
100 delivery creations/sec peak
1k status events/sec peak
10k+ realtime location events/sec handled by Realtime path
```

The Delivery Service should not process every GPS point itself. Location ingestion belongs to the Realtime/Driver path.

Delivery receives business-level state changes, not raw high-frequency location streams.

---

# 85. Scaling Strategy

Delivery Service is horizontally scalable because transactional state is externalized to PostgreSQL and coordination is externalized to Redis/Kafka.

```text
                 Load Balancer
                      |
          +-----------+-----------+
          |           |           |
          v           v           v
      Delivery-1  Delivery-2  Delivery-3
          |           |           |
          +-----------+-----------+
                      |
              PostgreSQL / Redis
                      |
                     Kafka
```

Do not use process memory for authoritative state.

---

# 86. Database Connection Pooling

Each Delivery instance must use bounded PostgreSQL connections.

Scaling pods without controlling the DB pool can overload PostgreSQL.

Example principle:

```text
Pod count x pool size <= safe DB connection capacity
```

Capacity must be measured rather than guessed.

---

# 87. Kafka Partitioning and Ordering

Partition durable delivery events using:

```text
deliveryId
```

This provides ordering for events of the same delivery while allowing multiple deliveries to process in parallel.

Do not require global ordering across the entire topic.

---

# 88. Data Consistency Model

Delivery PostgreSQL:

```text
Strong transactional consistency
```

Kafka:

```text
Durable asynchronous propagation
```

Search:

```text
Eventually consistent
```

Redis:

```text
Ephemeral / coordination
```

ClickHouse:

```text
Analytical eventual consistency
```

This separation is intentional.

---

# 89. No Distributed Database Transaction

Never attempt:

```text
PostgreSQL Delivery
 +
PostgreSQL Payment
 +
MongoDB Driver
```

inside one ACID transaction.

Use Saga instead.

```text
Local transaction
   |
   v
Domain event / command
   |
   v
Remote service
   |
   v
Compensation if required
```

---

# 90. Eventual Consistency User Experience

The client must understand that:

```text
Create Delivery
```

does not mean:

```text
Driver already assigned
Payment already completed
```

The initial response may be:

```text
REQUESTED
```

Realtime updates then move the UI through the lifecycle.

---

# 91. Realtime Status Projection

```text
Delivery Service
      |
      | domain state changed
      v
Kafka / NATS
      |
      v
Realtime Service
      |
      v
WebSocket
      |
      v
Client
```

The client should not poll aggressively for state changes.

---

# 92. Search Projection

Search is asynchronous:

```text
Delivery DB
   |
   v
Outbox
   |
   v
Kafka
   |
   v
Search Service
   |
   v
OpenSearch
```

If OpenSearch loses data:

```text
Delivery DB
   |
   v
Reindex process
   |
   v
OpenSearch
```

---

# 93. Media Integration

Delivery Service stores references, not file bytes.

```text
proofMediaId
```

Media Service owns:

- Upload session
- Presigned URLs
- Object storage
- Virus scanning
- Processing
- Metadata
- CDN

Delivery only needs the final media identifier/reference.

---

# 94. Notification Integration

Notifications are asynchronous.

```text
delivery.driver.assigned
        |
        v
Kafka
        |
        v
Notification Service
        |
        v
BullMQ
        |
        +--> Push
        +--> Email
        +--> In-App
```

Delivery does not wait for email or push delivery before changing business state.

---

# 95. Service Dependency Matrix

| Dependency | Protocol | Purpose | Failure Policy |
|---|---|---|---|
| API Gateway | GraphQL | Client API | Gateway handles transport |
| Driver/Dispatch | gRPC | Driver lookup/assignment | timeout + retry/circuit breaker |
| Payment | gRPC/events | Payment operations | timeout + Saga recovery |
| Realtime | NATS | Low-latency updates | transient |
| Kafka | Kafka | Durable events | Outbox buffers failure |
| Redis | Redis | Cache/coordination | operation-specific fallback |
| PostgreSQL | SQL | Source of truth | fail safely |
| Media | service contract | Proof reference | asynchronous where possible |

---

# 96. What Must NOT Be Added

Do not add these to the initial Delivery Service unless a documented requirement appears:

```text
REST business APIs
Event Sourcing
Separate Delivery database for reads
Debezium
NATS JetStream as duplicate Kafka
Qdrant
GenAI
AI Agents
OpenSearch direct writes
Multiple new microservices
```

The architecture should demonstrate strong engineering, not maximum technology count.

---

# 97. Future Enhancements

Future improvements can include:

```text
PostgreSQL read replicas
Denormalized CQRS read model
Debezium CDC
Advanced dispatch optimization
Dynamic pricing
Geospatial route estimation
Event sourcing for selected aggregates
Temporal-like workflow engine
```

These are future enhancements, not initial requirements.

---

# 98. GenAI Future Phase

GenAI is explicitly excluded from the initial Delivery implementation.

Later, a separate FastAPI AI Service may provide:

```text
Delivery assistant
Natural language delivery search
ETA explanation
Operational assistant
RAG over platform documentation
Agent tool calling
Semantic delivery analysis
```

Possible future architecture:

```text
AI Service
  |
  +--> gRPC -> Delivery Service
  +--> gRPC -> Driver Service
  +--> gRPC -> Payment Service
  +--> Qdrant / vector search
  +--> LLM
```

AI must not become the source of truth for delivery state.

---

# 99. Development Phases

## Phase 1 — Project Skeleton

```text
NestJS app
GraphQL Federation subgraph
PostgreSQL
Docker
Config
Health checks
Logging
```

## Phase 2 — Delivery Domain

```text
Delivery entity
Address entity
Status history
State machine
Validation
Authorization
```

## Phase 3 — CQRS

```text
Commands
Queries
Handlers
Repositories
Read/write logical separation
```

## Phase 4 — GraphQL

```text
createDelivery
getDelivery
listDeliveries
cancelDelivery
status history
```

## Phase 5 — Outbox

```text
Outbox table
Transactional publishing
Kafka producer
Event envelope
```

## Phase 6 — Saga

```text
Saga state
Saga steps
Driver step
Payment step
Compensation
Recovery
```

## Phase 7 — gRPC

```text
Driver contracts
Payment contracts
Timeouts
Retries
Circuit breaker
```

## Phase 8 — Kafka Consumers

```text
Payment events
Driver events
Idempotent consumers
DLQ
```

## Phase 9 — Redis

```text
Cache
Idempotency fast-path
Coordination
```

## Phase 10 — Reconciliation

```text
BullMQ
Stuck delivery detection
Outbox recovery
Saga recovery
```

## Phase 11 — Realtime Integration

```text
NATS
Realtime events
WebSocket client updates
```

## Phase 12 — Observability

```text
OpenTelemetry
Prometheus
Grafana
Jaeger
```

## Phase 13 — Kubernetes

```text
Deployment
Service
ConfigMap
Secret
HPA
Skaffold
```

## Phase 14 — Failure Testing

```text
Chaos/failure injection
Load testing
Race-condition testing
Recovery testing
```

---

# 100. Definition of Done

Delivery Service is considered complete when all of the following work:

```text
[ ] GraphQL Federation subgraph
[ ] Authentication context
[ ] Authorization
[ ] Delivery CRUD/use cases
[ ] State machine
[ ] PostgreSQL schema
[ ] Transactions
[ ] Status history
[ ] CQRS logical separation
[ ] Idempotency
[ ] Transactional Outbox
[ ] Kafka producer
[ ] Kafka consumer
[ ] Event versioning
[ ] Driver gRPC integration
[ ] Payment gRPC integration
[ ] Saga orchestration
[ ] Compensation
[ ] Retry + backoff
[ ] Timeout policies
[ ] Circuit breaker where justified
[ ] Redis cache
[ ] Reconciliation jobs
[ ] DLQ strategy
[ ] NATS realtime integration
[ ] Media reference integration
[ ] Notification events
[ ] Search events
[ ] Analytics events
[ ] Structured logging
[ ] Metrics
[ ] Distributed tracing
[ ] Unit tests
[ ] Integration tests
[ ] Contract tests
[ ] E2E tests
[ ] Docker
[ ] Kubernetes
[ ] Skaffold
[ ] HPA
[ ] Failure injection tests
```

---

# 101. Final End-to-End Architecture

```text
                                  CLIENT
                                     |
                                  GraphQL
                                     |
                                     v
                           +--------------------+
                           |    API Gateway     |
                           | NestJS Federation  |
                           +---------+----------+
                                     |
                              Delivery Subgraph
                                     |
                                     v
                           +--------------------+
                           |  DELIVERY SERVICE  |
                           |      NestJS        |
                           +---------+----------+
                                     |
              +----------------------+----------------------+
              |                      |                      |
              v                      v                      v
         PostgreSQL               Redis                   gRPC
              |                      |                      |
              |                      |             +--------+--------+
              |                      |             |                 |
              |                      |             v                 v
              |                      |        Driver/Dispatch    Payment
              |                      |             |                 |
              |                      |             +--------+--------+
              |                      |                      |
              v                      v                      v
         Outbox                   Cache                  External
              |                   Locks                  Services
              |                   Idempotency
              v
        Outbox Publisher
              |
              v
            Kafka
              |
      +-------+--------+----------------+
      |                |                |
      v                v                v
 Notification       Analytics         Search
      |              ClickHouse      OpenSearch
    BullMQ
      |
      v
 Workers

Delivery Status Events
              |
              v
            NATS
              |
              v
        Realtime Service
              |
              v
          WebSocket
              |
              v
            CLIENT
```

---

# 102. Architectural Rules for AI Coding Agents

Any AI coding agent working on the Delivery Service must follow these rules.

1. Do not introduce REST business APIs.
2. Do not create additional microservices without explicit approval.
3. Use GraphQL Federation for client-facing business operations.
4. Keep business logic out of the API Gateway.
5. Delivery Service owns Delivery business state.
6. Never access another service's database.
7. PostgreSQL is the Delivery source of truth.
8. Redis is not the transactional source of truth.
9. Use transactions for critical local state transitions.
10. Use Transactional Outbox for Kafka events.
11. Kafka consumers must be idempotent.
12. Use Saga orchestration for distributed delivery workflows.
13. Delivery Service owns the Saga orchestration.
14. Define compensation for every reversible Saga step.
15. Every remote call must have a timeout.
16. Retry only transient failures.
17. Use exponential backoff and jitter.
18. Use optimistic concurrency for state transitions.
19. Do not call remote services while holding a DB transaction open.
20. Use NATS for low-latency transient realtime integration.
21. Use Kafka for durable business events.
22. Do not duplicate Kafka's role with NATS JetStream without a concrete requirement.
23. Do not send notifications directly from Delivery Service.
24. Do not write directly to OpenSearch.
25. Do not store media bytes in Delivery Service.
26. Do not store driver location as Delivery source-of-truth data.
27. Do not implement GenAI in the initial phase.
28. Do not add Qdrant to the initial Delivery implementation.
29. Do not add Debezium initially.
30. Shared NestJS packages may contain technical utilities but never Delivery business rules.
31. Services must remain independently deployable.
32. Every important operation must be observable.
33. Every critical side effect must be idempotent.
34. All event schemas must be versioned.
35. Every failure path must have explicit behavior.

---

# 103. Quick Reference

## Communication

```text
Client -> Platform              GraphQL Federation
Delivery -> Driver              gRPC
Delivery -> Payment             gRPC
Delivery -> Durable consumers   Kafka
Delivery -> Realtime             NATS
Background reconciliation        BullMQ
```

## Data

```text
Delivery transactional state     PostgreSQL
Cache / coordination             Redis
Driver operational state         Driver Service / MongoDB
Analytics                        ClickHouse
Search projection                OpenSearch
Media                            Object Storage via Media Service
```

## Patterns

```text
Workflow                         Saga Orchestration
Reliable events                  Transactional Outbox
Read/write separation            CQRS
Duplicate prevention             Idempotency
Concurrent state protection      Optimistic Concurrency
Async durable communication      Kafka
Low-latency transient messaging  NATS
Background execution             BullMQ
```

## Infrastructure

```text
Containerization                 Docker
Local infrastructure             Docker Compose
Production                       Kubernetes
Local K8s workflow                Skaffold
Observability                    OpenTelemetry + Prometheus + Grafana + Jaeger
```

---

# 104. Final Architectural Decision

The Delivery Service should be implemented **now**, before Payment Service, because it is the central business aggregate and the natural owner of the distributed delivery Saga.

The correct implementation sequence is:

```text
Delivery Service
      |
      +--> PostgreSQL
      +--> State Machine
      +--> CQRS
      +--> GraphQL Federation
      +--> Outbox
      +--> Kafka
      |
      +--> Driver/Dispatch via gRPC
      |
      +--> Payment via gRPC/events
      |
      +--> Saga Orchestration
      |
      +--> Redis
      +--> Reconciliation / BullMQ
      +--> NATS -> Realtime
      |
      +--> Notification events
      +--> Search events
      +--> Analytics events
```

This keeps the project aligned with the overall architecture while avoiding unnecessary service proliferation and technology duplication.

---

# 105. Overall Project Implementation Order

Based on the current project architecture and the services already completed, the recommended next order is:

```text
DONE
├── API Gateway
├── User Service
├── Notification Service
├── Media Service
├── Realtime Service
└── Search Service

NEXT
└── Delivery Service       <-- CURRENT TARGET

THEN
├── Driver & Dispatch Service
├── Payment Service
├── Full Saga Integration
└── Analytics Service

FINALLY
├── Kubernetes hardening
├── Observability hardening
├── Load / failure testing
└── Future GenAI phase
```

The Delivery Service specification above is therefore the implementation contract for the next stage of the project.
