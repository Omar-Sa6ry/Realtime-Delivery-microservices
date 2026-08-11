# Realtime Delivery Platform

# System Architecture Specification

**Document Type:** System Architecture Specification
**Architecture Style:** Distributed Microservices Architecture
**Primary Domain:** Real-Time Delivery Platform
**Client API:** GraphQL Federation
**Internal Communication:** gRPC, NATS, Kafka
**Realtime Communication:** WebSocket
**Containerization:** Docker
**Orchestration:** Kubernetes
**Local Kubernetes Development:** Skaffold
**Primary Languages:** TypeScript/NestJS, Go
**Future AI Stack:** Python/FastAPI, Qdrant, LLMs

---

# 1. System Overview

The system is a production-oriented real-time delivery platform designed to demonstrate modern distributed systems, microservices architecture, event-driven architecture, real-time communication, distributed transactions, resilience, observability, and scalable infrastructure.

The platform allows customers to:

* Create delivery requests.
* Track deliveries in real time.
* View delivery status.
* Receive notifications.
* View delivery history.

Drivers can:

* Connect to the platform in real time.
* Receive delivery assignments.
* Accept or reject assignments.
* Send real-time location updates.
* Update delivery status.

The platform supports:

* Distributed delivery workflows.
* Driver assignment.
* Payment processing.
* Notifications.
* Real-time tracking.
* Analytics.
* Distributed transactions.
* Event-driven communication.
* Horizontal scaling.

The architecture is intentionally designed as a learning and portfolio system that demonstrates real production architecture patterns without unnecessarily creating a large number of microservices.

---

# 2. Architectural Goals

The system must demonstrate the following concepts:

```text
Microservices
GraphQL Federation
Event-Driven Architecture
gRPC
NATS
Kafka
WebSocket
Saga Pattern
Transactional Outbox
CQRS
Redis
Redis GEO
Distributed Locks
Idempotency
BullMQ
PostgreSQL
MongoDB
ClickHouse
Object Storage
Docker
Kubernetes
Skaffold
Observability
Resilience
Horizontal Scaling
```

Future architecture will introduce:

```text
FastAPI
Qdrant
RAG
LLM
AI Agents
OpenSearch
Debezium / CDC
```

---

# 3. Architectural Principles

The system follows these principles:

1. Each service owns its business logic.
2. Each service owns its persistent data.
3. Services communicate through explicit contracts.
4. Synchronous communication uses gRPC.
5. Durable business events use Kafka.
6. Low-latency transient communication uses NATS.
7. Client-facing APIs use GraphQL Federation.
8. High-frequency realtime communication uses WebSocket.
9. Distributed workflows use Saga orchestration.
10. Reliable event publishing uses Transactional Outbox.
11. Critical operations are idempotent.
12. Redis is used for ephemeral and coordination workloads.
13. PostgreSQL is used for transactional relational data.
14. MongoDB is used for flexible operational data.
15. ClickHouse is used for analytical workloads.
16. Business logic is never placed inside shared packages.
17. Services remain independently deployable.
18. No service directly accesses another service's database.
19. Failures must be isolated whenever possible.
20. All important operations must be observable.

---

# 4. High-Level Architecture

```text
                                  CLIENTS
                                     |
                     +---------------+---------------+
                     |                               |
                  GraphQL                         WebSocket
                     |                               |
                     v                               v
            +------------------+             +------------------+
            | GraphQL Gateway   |             | Realtime Service |
            |     NestJS        |             |     NestJS       |
            |    Federation     |             |    WebSocket     |
            +--------+----------+             +---------+--------+
                     |                                  |
        +------------+-------------+                    |
        |            |             |                    |
        v            v             v                   NATS
      Auth        Delivery       Driver                  |
      User         NestJS       Dispatch                 |
                     |            Go                    |
                     |            |                      |
                     |         MongoDB                  |
                     |                                  |
                  PostgreSQL                           Redis
                     |
             +-------+-------+
             |               |
           Outbox            CQRS
             |               |
             v               v
           Kafka         Read Model
             |
      +------+-------+---------+
      |              |         |
      v              v         v
 Payment        Notification Analytics
   Go              NestJS      Go
   |                 |           |
PostgreSQL         BullMQ    ClickHouse
                     |
                   Redis


                  Object Storage
                       |
                Driver Documents
                Delivery Proofs


                     FUTURE
                       |
                  AI Service
                    FastAPI
                       |
             +---------+---------+
             |                   |
            RAG                Agent
             |                   |
          Qdrant                gRPC
```

---

# 5. Services

The system contains eight application services.

| # | Service           | Language   | Framework | Database   |
| - | ----------------- | ---------- | --------- | ---------- |
| 1 | GraphQL Gateway   | TypeScript | NestJS    | None       |
| 2 | Auth & User       | TypeScript | NestJS    | PostgreSQL |
| 3 | Delivery          | TypeScript | NestJS    | PostgreSQL |
| 4 | Driver & Dispatch | Go         | Go        | MongoDB    |
| 5 | Payment           | Go         | Go        | PostgreSQL |
| 6 | Notification      | TypeScript | NestJS    | PostgreSQL |
| 7 | Realtime          | TypeScript | NestJS    | Redis      |
| 8 | Analytics         | Go         | Go        | ClickHouse |

Future:

| Service         | Technology | Purpose              |
| --------------- | ---------- | -------------------- |
| AI Service      | FastAPI    | GenAI / RAG / Agents |
| Search          | Go         | OpenSearch           |
| AI Vector Store | Qdrant     | Embeddings           |

The future services are intentionally excluded from the initial implementation.

---

# 6. GraphQL Gateway

The GraphQL Gateway is the only public application API.

```text
Client
   |
   | GraphQL
   v
GraphQL Gateway
   |
   +---- Auth Subgraph
   +---- Delivery Subgraph
   +---- Driver Subgraph
   +---- Payment Subgraph
   +---- Notification Subgraph
```

Responsibilities:

* GraphQL Federation.
* Authentication propagation.
* Request validation.
* Query routing.
* Authorization.
* Request tracing.
* Rate limiting.
* Aggregating data from subgraphs.

The Gateway does not contain business logic.

---

# 7. GraphQL Federation

Each domain exposes a GraphQL subgraph.

```text
                    GraphQL Gateway
                          |
             +------------+------------+
             |            |            |
             v            v            v
          Auth         Delivery      Driver
        Subgraph       Subgraph     Subgraph
             |            |            |
             v            v            v
          Auth/User     Delivery     Driver
           Service       Service      Service
```

The client sees one unified GraphQL schema.

---

# 8. GraphQL API

## Queries

```graphql
delivery(id: ID!): Delivery

myDeliveries(
  page: Int
  limit: Int
): DeliveryConnection

activeDelivery: Delivery

deliveryHistory(
  page: Int
  limit: Int
): DeliveryConnection

driver(id: ID!): Driver

myProfile: User

payment(id: ID!): Payment

notifications(
  page: Int
  limit: Int
): NotificationConnection
```

## Mutations

```graphql
createDelivery(input: CreateDeliveryInput!): Delivery

cancelDelivery(id: ID!): Delivery

acceptDelivery(id: ID!): Delivery

rejectDelivery(id: ID!): Delivery

startPickup(id: ID!): Delivery

markPickedUp(id: ID!): Delivery

startTransit(id: ID!): Delivery

completeDelivery(id: ID!): Delivery
```

Realtime location updates are intentionally NOT exposed through GraphQL subscriptions.

---

# 9. Authentication

Authentication is based on JWT.

```text
Client
  |
  | JWT
  v
GraphQL Gateway
  |
  v
Authentication
  |
  v
User Context
```

The authenticated context contains:

```text
userId
role
sessionId
```

The Gateway propagates trusted identity information to internal services.

Services must never trust a user ID supplied directly by the client.

---

# 10. Service-to-Service Communication

Three communication mechanisms are used.

| Technology | Purpose                         |
| ---------- | ------------------------------- |
| gRPC       | Synchronous request/response    |
| NATS       | Low-latency transient messaging |
| Kafka      | Durable business events         |

Decision:

```text
Need immediate response?
        |
       YES
        |
       gRPC


Need very low latency transient message?
        |
       YES
        |
       NATS


Need durable event?
        |
       YES
        |
      Kafka
```

---

# 11. gRPC

gRPC is used for synchronous internal operations.

Examples:

```text
Delivery -> Driver
Delivery -> Payment
Delivery -> Notification
AI -> Delivery (Future)
AI -> Driver (Future)
```

Example:

```proto
service DriverService {
  rpc FindAvailableDriver(
    FindAvailableDriverRequest
  ) returns (
    FindAvailableDriverResponse
  );

  rpc AssignDriver(
    AssignDriverRequest
  ) returns (
    AssignDriverResponse
  );

  rpc GetDriver(
    GetDriverRequest
  ) returns (
    GetDriverResponse
  );
}
```

Payment:

```proto
service PaymentService {
  rpc CreatePayment(
    CreatePaymentRequest
  ) returns (
    CreatePaymentResponse
  );

  rpc GetPaymentStatus(
    GetPaymentStatusRequest
  ) returns (
    GetPaymentStatusResponse
  );

  rpc RefundPayment(
    RefundPaymentRequest
  ) returns (
    RefundPaymentResponse
  );
}
```

All contracts are defined in `.proto` files.

---

# 12. NATS

NATS is used for low-latency transient communication.

Primary use case:

```text
Realtime Service
        |
       NATS
        |
+-------+-------+
|       |       |
v       v       v
Node 1  Node 2  Node 3
```

Subjects:

```text
delivery.location.updated
delivery.status.updated
driver.location.updated
driver.assignment.updated
driver.presence.updated
```

NATS is preferred for high-frequency realtime updates because those updates do not need Kafka-level persistence.

---

# 13. Kafka

Kafka is used for durable business events.

Examples:

```text
delivery.created
delivery.driver.assigned
delivery.driver.accepted
delivery.driver.rejected
delivery.picked_up
delivery.in_transit
delivery.completed
delivery.cancelled

payment.completed
payment.failed
payment.refunded
```

Consumers:

```text
Kafka
 |
 +---- Notification
 |
 +---- Analytics
 |
 +---- Delivery
 |
 +---- Future Consumers
```

Kafka provides:

* Durability.
* Replay.
* Consumer groups.
* Ordered partitions.
* Independent consumers.
* Horizontal scalability.

---

# 14. Kafka vs NATS

The system intentionally uses both.

```text
High-frequency realtime state
        |
       NATS


Durable business event
        |
      Kafka
```

Example:

```text
Driver location
    |
   NATS


Delivery completed
    |
  Kafka
```

Location updates do not need long-term event replay.

Business events do.

---

# 15. Transactional Outbox

Delivery Service uses the Transactional Outbox Pattern.

```text
PostgreSQL Transaction
        |
        +---- Delivery
        |
        +---- Outbox Event
        |
      COMMIT
        |
        v
Outbox Publisher
        |
        v
Kafka
```

This prevents the following failure:

```text
Database Commit
      |
      v
Application Crash
      |
      v
Kafka Event Lost
```

The outbox ensures the database state and event record are committed atomically.

---

# 16. Outbox Table

```text
OutboxEvent
---------------------
id
eventId
aggregateId
aggregateType
eventType
payload
published
createdAt
publishedAt
```

Initial implementation uses polling.

Future enhancement:

```text
PostgreSQL WAL
      |
  Debezium
      |
    Kafka
```

Debezium is not required in the initial implementation.

---

# 17. Saga Pattern

Delivery creation is a distributed workflow.

The Saga is orchestrated by the Delivery domain.

```text
                 Delivery Saga
                      |
        +-------------+-------------+
        |             |             |
        v             v             v
    Driver          Payment      Delivery
     Go               Go          State
```

The Saga coordinates:

```text
Driver Assignment
Payment
Delivery State
```

---

# 18. Saga Success Flow

```text
Create Delivery
      |
      v
Reserve Driver
      |
      v
Process Payment
      |
      v
Confirm Delivery
      |
      v
Delivery Active
```

---

# 19. Saga Compensation

If payment fails:

```text
Reserve Driver
      |
      v
Payment Failed
      |
      v
Release Driver
      |
      v
Cancel / Retry Delivery
```

If driver assignment fails:

```text
Payment
   |
   v
Driver Assignment Failed
   |
   v
Refund Payment
```

Every Saga step must define:

```text
Forward Action
Compensation Action
Timeout
Retry Policy
Idempotency Strategy
```

---

# 20. Delivery State Machine

The Delivery Service owns delivery state.

```text
PENDING
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
```

Cancellation may occur where business rules allow:

```text
PENDING
SEARCHING_DRIVER
DRIVER_ASSIGNED
DRIVER_ACCEPTED
```

Terminal states:

```text
DELIVERED
CANCELLED
FAILED
```

Invalid state transitions must be rejected.

---

# 21. CQRS

Delivery uses logical CQRS.

```text
                 Delivery Service
                       |
             +---------+---------+
             |                   |
             v                   v
       Command Model       Query Model
             |                   |
             v                   v
       Write Database       Read Database
```

Commands:

```text
CreateDelivery
AssignDriver
AcceptDelivery
RejectDelivery
StartPickup
MarkPickedUp
StartTransit
CompleteDelivery
CancelDelivery
```

Queries:

```text
GetDelivery
GetActiveDelivery
GetDeliveryHistory
GetCustomerDeliveries
GetDeliveryStatus
```

Initial implementation may use the same PostgreSQL instance.

Future:

```text
PostgreSQL Primary
      |
Replication
      |
Read Replica
```

Full Event Sourcing is not required.

---

# 22. Driver & Dispatch Service

Technology:

```text
Go
MongoDB
Redis
gRPC
NATS
```

Responsibilities:

* Driver management.
* Driver availability.
* Driver assignment.
* Driver proximity.
* Driver state.
* Dispatch logic.
* Location coordination.

---

# 23. Redis GEO

Driver proximity is handled by Redis GEO.

```text
Driver
   |
   v
Redis GEO
   |
   v
Nearest Drivers
```

Example:

```text
GEOADD drivers:locations
<longitude>
<latitude>
<driverId>
```

Search:

```text
GEOSEARCH drivers:locations
FROMLONLAT <longitude> <latitude>
BYRADIUS 5 km
ASC
```

Availability is maintained separately:

```text
drivers:availability:{driverId}
```

---

# 24. Distributed Driver Lock

Two delivery requests must not reserve the same driver.

Redis distributed locking is used.

```text
Delivery A
    |
    v
Acquire Driver Lock
    |
    v
Reserve Driver
    |
    v
Release Lock
```

Example key:

```text
driver:lock:{driverId}
```

The lock must have:

* TTL.
* Unique ownership token.
* Safe release.
* Retry behavior.

---

# 25. Realtime Service

Technology:

```text
NestJS
WebSocket
NATS
Redis
```

Responsibilities:

* WebSocket connections.
* WebSocket authentication.
* Connection management.
* Delivery subscriptions.
* Driver location updates.
* Realtime delivery status.
* Cross-instance fan-out.

---

# 26. WebSocket Decision

WebSocket is used for all realtime clients.

Although customers primarily receive updates, drivers require bidirectional communication.

Driver:

```text
Driver
  |
  +---- location
  |
  +---- accept assignment
  |
  +---- reject assignment
  |
  +---- receive commands
```

Therefore WebSocket is used instead of SSE.

GraphQL Subscriptions are also intentionally not used.

The Realtime Service remains independent from GraphQL.

---

# 27. WebSocket Authentication

```text
Client
   |
   | JWT
   v
WebSocket Handshake
   |
   v
Realtime Service
   |
   v
JWT Validation
   |
   +---- Invalid -> Reject
   |
   +---- Valid -> Upgrade
```

Connection context:

```text
userId
role
sessionId
```

---

# 28. Realtime Location Flow

```text
Driver
   |
   | WebSocket
   v
Realtime Service
   |
   +---- Redis
   |
   +---- NATS
           |
           +---- Realtime Node 1
           +---- Realtime Node 2
           +---- Realtime Node 3
                    |
                    v
                Customer
```

High-frequency location updates remain outside Kafka.

---

# 29. Notification Service

Technology:

```text
NestJS
PostgreSQL
BullMQ
Redis
```

Responsibilities:

* Notification creation.
* Notification templates.
* Notification delivery.
* Retry.
* Scheduling.
* Notification history.

Channels can include:

```text
Email
SMS
Push Notification
In-App Notification
```

---

# 30. BullMQ

Notification processing is asynchronous.

```text
Kafka Event
    |
    v
Notification Service
    |
    v
BullMQ
    |
    v
Worker
    |
    +---- Email
    +---- SMS
    +---- Push
    +---- In-App
```

BullMQ handles:

* Retries.
* Delayed jobs.
* Concurrency.
* Failed jobs.
* Background processing.

---

# 31. Payment Service

Technology:

```text
Go
PostgreSQL
gRPC
Kafka
```

Responsibilities:

* Payment creation.
* Payment processing.
* Payment status.
* Refunds.
* Idempotency.

Payment operations must be idempotent.

Example:

```text
idempotencyKey
```

Duplicate requests must not create duplicate charges.

---

# 32. Analytics Service

Technology:

```text
Go
ClickHouse
Kafka
```

Analytics is event-driven.

```text
Kafka
   |
   v
Analytics Service
   |
   v
ClickHouse
```

Analytics can track:

```text
Deliveries per day
Delivery completion rate
Average delivery time
Driver utilization
Payment success rate
Cancellation rate
Peak delivery times
Average assignment time
```

Analytics queries must not execute against transactional PostgreSQL databases.

---

# 33. Database Architecture

The platform uses different databases based on workload.

```text
PostgreSQL
    |
    +---- Auth
    +---- Delivery
    +---- Payment
    +---- Notification


MongoDB
    |
    +---- Driver
    +---- Dispatch


Redis
    |
    +---- Cache
    +---- Driver GEO
    +---- Distributed Locks
    +---- Idempotency
    +---- Realtime State
    +---- BullMQ


ClickHouse
    |
    +---- Analytics


Object Storage
    |
    +---- Documents
    +---- Delivery Proofs
    +---- Driver Documents
```

---

# 34. Database Ownership

No service may directly access another service's database.

```text
Auth Service
    |
PostgreSQL Auth Data


Delivery Service
    |
PostgreSQL Delivery Data


Payment Service
    |
PostgreSQL Payment Data


Driver Service
    |
MongoDB Driver Data
```

The only communication between services is through APIs/events.

---

# 35. Object Storage

Object Storage is used for large files.

Examples:

```text
Driver Documents
Delivery Proof
Delivery Images
Identity Documents
```

The database stores metadata, not binary files.

```text
Database
   |
   +---- fileId
   +---- objectKey
   +---- contentType
   +---- size
```

The actual file is stored in Object Storage.

---

# 36. Idempotency

Idempotency is required for:

```text
Create Payment
Process Payment
Assign Driver
Complete Delivery
Kafka Consumers
Notification Jobs
```

Idempotency can use:

```text
Redis
+
PostgreSQL
```

Example:

```text
Request
   |
idempotencyKey
   |
Redis
   |
Already processed?
   |
 +---+---+
 |       |
YES      NO
 |       |
Return   Process
result
```

Critical operations must also persist their idempotency state transactionally where appropriate.

---

# 37. Dead Letter Queue

Kafka consumer failure:

```text
Kafka
  |
Consumer
  |
Retry
  |
Retry
  |
Retry
  |
DLQ
```

Examples:

```text
delivery.created.dlq
payment.completed.dlq
delivery.completed.dlq
```

DLQ record:

```text
eventId
eventType
aggregateId
originalTopic
consumerGroup
failureReason
retryCount
failedAt
payload
```

BullMQ failed jobs also have a controlled failed/DLQ workflow.

---

# 38. Resilience

The system uses:

```text
Timeout
Retry
Circuit Breaker
Bulkhead
Idempotency
Dead Letter Queue
Graceful Degradation
Health Checks
```

Example:

```text
Service A
   |
   | gRPC
   v
Service B
   |
Timeout
   |
Retry
   |
Circuit Breaker
```

Retries must use exponential backoff.

Non-idempotent operations must not be blindly retried.

---

# 39. Circuit Breaker

Example:

```text
Payment Service
      |
      v
Repeated Failures
      |
      v
Circuit OPEN
      |
      v
Reject Fast
      |
      v
Recovery Test
      |
      v
Circuit CLOSED
```

Circuit breaker prevents cascading failures.

---

# 40. Rate Limiting

Rate limiting is implemented at the Gateway.

Examples:

```text
GraphQL Requests
Authentication Attempts
Delivery Creation
WebSocket Connection Attempts
```

Redis may be used for distributed rate limiting.

---

# 41. Realtime Scaling

Realtime Service must be horizontally scalable.

```text
                  Load Balancer
                       |
          +------------+------------+
          |            |            |
          v            v            v
      Realtime 1   Realtime 2   Realtime 3
          |            |            |
          +------------+------------+
                       |
                      NATS
                       |
                     Redis
```

NATS distributes realtime events across instances.

Redis stores required ephemeral connection/state information.

---

# 42. Kafka Consumer Groups

Each logical consumer has its own consumer group.

```text
Kafka
 |
 +---- Notification Group
 |
 +---- Analytics Group
 |
 +---- Delivery Group
 |
 +---- Future Group
```

This allows each service to consume the same event independently.

---

# 43. Kafka Partitioning

Events are partitioned by aggregate identifier where ordering matters.

Example:

```text
deliveryId
```

All events for the same delivery should preferably use the same partition key.

This preserves ordering for a delivery lifecycle.

---

# 44. Complete Delivery Creation Flow

```text
1. Client
       |
       | GraphQL
       v

2. GraphQL Gateway
       |
       v

3. Delivery Service
       |
       | BEGIN TRANSACTION
       |
       +---- Create Delivery
       |
       +---- Create Outbox Event
       |
       | COMMIT
       v

4. Outbox Publisher
       |
       v

5. Kafka
       |
       | delivery.created
       |
       +----------+-----------+
       |          |           |
       v          v           v
    Driver   Notification  Analytics
```

---

# 45. Complete Driver Assignment Flow

```text
Delivery
   |
   | gRPC
   v
Driver Service
   |
   v
Redis GEO
   |
   v
Nearest Drivers
   |
   v
Distributed Lock
   |
   v
Reserve Driver
   |
   v
NATS
   |
   v
Realtime Service
   |
   v
Driver WebSocket
```

---

# 46. Complete Payment Flow

```text
Delivery Saga
      |
      | gRPC
      v
Payment Service
      |
      v
Idempotency Check
      |
      v
PostgreSQL
      |
      v
Payment Provider
      |
      v
Payment Result
      |
      v
Kafka
      |
      v
Delivery Saga
```

---

# 47. Complete Realtime Tracking Flow

```text
Driver
   |
   | WebSocket
   v
Realtime Service
   |
   +---- Redis
   |
   +---- NATS
           |
           v
     Other Realtime Nodes
           |
           v
       Customers
```

---

# 48. Complete Notification Flow

```text
Kafka
  |
  v
Notification Service
  |
  v
BullMQ
  |
  v
Worker
  |
  +---- Email
  +---- SMS
  +---- Push
  +---- In-App
```

Failed jobs:

```text
Worker
  |
  v
Retry
  |
  v
Failed Queue / DLQ
```

---

# 49. Service Method Catalogue

## Delivery Service

```text
createDelivery()
getDelivery()
getDeliveries()
getActiveDelivery()
cancelDelivery()
assignDriver()
acceptDelivery()
rejectDelivery()
startPickup()
markPickedUp()
startTransit()
completeDelivery()
```

## Driver & Dispatch

```text
findAvailableDriver()
assignDriver()
releaseDriver()
acceptAssignment()
rejectAssignment()
updateAvailability()
updateLocation()
getDriver()
```

## Payment

```text
createPayment()
processPayment()
getPayment()
getPaymentStatus()
refundPayment()
```

## Notification

```text
createNotification()
sendNotification()
scheduleNotification()
retryNotification()
```

## Realtime

```text
connect()
authenticate()
subscribeToDelivery()
handleLocationUpdate()
broadcastLocation()
broadcastStatus()
disconnect()
```

---

# 50. Core Database Schemas

## Delivery

```text
id
customerId
driverId
pickupAddress
dropoffAddress
status
price
paymentStatus
createdAt
updatedAt
```

## DeliveryStatusHistory

```text
id
deliveryId
fromStatus
toStatus
changedBy
createdAt
```

## OutboxEvent

```text
id
eventId
aggregateId
aggregateType
eventType
payload
published
createdAt
publishedAt
```

## Driver

```text
id
userId
vehicleType
vehicleNumber
status
createdAt
updatedAt
```

## Payment

```text
id
deliveryId
customerId
amount
currency
status
idempotencyKey
createdAt
updatedAt
```

## Notification

```text
id
userId
type
channel
status
payload
createdAt
sentAt
```

---

# 51. Capacity Estimation

Initial target:

```text
Registered Users:       100,000
Active Drivers:          10,000
Concurrent WebSockets:   20,000
Deliveries / Day:        50,000
Peak Deliveries / Hour:   5,000
```

Driver location updates:

```text
10,000 drivers
×
1 update / 2 seconds
=
5,000 updates / second
```

This justifies:

```text
WebSocket
NATS
Redis
Horizontal Realtime Scaling
```

It does not justify sending every location update through Kafka.

Kafka remains focused on durable business events.

---

# 52. Failure Scenarios

## Kafka Down

Events remain in the Outbox until publishing succeeds.

## Kafka Consumer Down

Kafka retains events until the consumer recovers.

## Payment Down

Saga waits/retries according to policy.

## Driver Service Down

Delivery remains in searching state and retries assignment.

## Notification Worker Down

BullMQ retains pending jobs.

## Redis Down

Realtime functionality may degrade, but transactional data remains safe in PostgreSQL.

## Realtime Node Down

Clients reconnect to another node.

## Duplicate Event

Consumer checks event ID and ignores already processed events.

## Duplicate Payment Request

Idempotency key prevents duplicate payment.

## Driver Reservation Race

Distributed lock prevents multiple deliveries from reserving the same driver.

---

# 53. WebSocket Reconnection

Clients must implement:

```text
Initial Connection
       |
       v
Connection Lost
       |
       v
Exponential Backoff
       |
       v
Reconnect
       |
       v
Re-authenticate
       |
       v
Restore Subscriptions
```

The server must handle reconnects safely.

---

# 54. Observability

Every service must support:

```text
Structured Logging
Metrics
Distributed Tracing
Health Checks
Readiness Checks
Liveness Checks
```

Every request/event should carry:

```text
requestId
correlationId
traceId
```

Example:

```text
Client
 |
GraphQL
 |
Gateway
 |
Delivery
 |
Kafka
 |
Notification
```

The same trace/correlation context should be propagated where possible.

---

# 55. Health Checks

Each service exposes health information internally.

Checks include:

```text
Application
Database
Redis
Kafka
NATS
External Dependencies
```

Kubernetes uses:

```text
Liveness Probe
Readiness Probe
```

---

# 56. Docker

Every application service is containerized.

```text
GraphQL Gateway
Auth
Delivery
Driver
Payment
Notification
Realtime
Analytics
```

Infrastructure can also run in containers:

```text
PostgreSQL
MongoDB
Redis
Kafka
NATS
ClickHouse
Object Storage
```

Docker Compose is used for simple local development.

---

# 57. Kubernetes

Kubernetes is the production orchestration platform.

The cluster manages:

```text
Deployments
Services
Ingress
ConfigMaps
Secrets
Horizontal Pod Autoscaling
Resource Requests
Resource Limits
Liveness Probes
Readiness Probes
```

Example:

```text
Kubernetes Cluster
 |
 +---- GraphQL Gateway
 +---- Auth
 +---- Delivery
 +---- Driver
 +---- Payment
 +---- Notification
 +---- Realtime
 +---- Analytics
```

---

# 58. Skaffold

Skaffold provides the local Kubernetes development loop.

```text
Code Change
     |
     v
Skaffold
     |
     v
Docker Build
     |
     v
Kubernetes Deploy
     |
     v
Running Service
```

Skaffold eliminates repetitive manual build/deploy operations during development.

---

# 59. Environment Strategy

Three environments are defined.

```text
Development
    |
Docker Compose

Local Kubernetes
    |
Skaffold + Kubernetes

Production
    |
Kubernetes
```

Configuration must be externalized.

Secrets must never be committed to source control.

---

# 60. NestJS Common Package

A common NestJS package may be created for genuinely reusable infrastructure.

Possible contents:

```text
common/
├── guards/
├── interceptors/
├── filters/
├── pipes/
├── decorators/
├── logging/
├── configuration/
├── validation/
├── errors/
└── observability/
```

Examples:

```text
Correlation ID
Global Exception Filter
Logging
Validation
Auth Guards
Tracing
Configuration
```

The package must never contain domain logic.

Incorrect:

```text
DeliveryService
PaymentService
DriverAssignmentService
```

Correct:

```text
Logger
Guard
Interceptor
Validation Helper
Tracing Helper
```

---

# 61. Shared Package Rules

Shared code is allowed only when:

```text
Multiple NestJS services
        |
        v
Same technical functionality
        |
        v
Common Package
```

Business logic remains local:

```text
Delivery Rules -> Delivery
Payment Rules -> Payment
Driver Rules -> Driver
Notification Rules -> Notification
```

Every service remains independently deployable.

---

# 62. Repository Structure

```text
delivery-platform/
│
├── apps/
│   ├── graphql-gateway/
│   ├── auth-user/
│   ├── delivery/
│   ├── driver-dispatch/
│   ├── payment/
│   ├── notification/
│   ├── realtime/
│   └── analytics/
│
├── packages/
│   ├── nest-common/
│   └── proto/
│
├── infrastructure/
│   ├── docker/
│   ├── kafka/
│   ├── nats/
│   ├── redis/
│   └── databases/
│
├── k8s/
│   ├── gateway/
│   ├── auth/
│   ├── delivery/
│   ├── driver/
│   ├── payment/
│   ├── notification/
│   ├── realtime/
│   └── analytics/
│
├── skaffold.yaml
├── docker-compose.yml
└── README.md
```

---

# 63. API Gateway Responsibilities

The Gateway is responsible for:

```text
Authentication
Authorization
GraphQL Federation
Rate Limiting
Request Validation
Tracing
Request Correlation
```

The Gateway is NOT responsible for:

```text
Payment Logic
Driver Assignment
Delivery State Machine
Notification Processing
Saga Business Logic
```

---

# 64. Security

Security requirements:

```text
JWT Authentication
Role-Based Authorization
TLS
Input Validation
Rate Limiting
Secret Management
Idempotency
Audit Logging
```

Sensitive data must not be logged.

Internal service communication should use authenticated channels.

---

# 65. Authorization

Roles may include:

```text
CUSTOMER
DRIVER
ADMIN
```

Examples:

```text
CUSTOMER
  -> Create Delivery
  -> Track Own Delivery

DRIVER
  -> View Assigned Deliveries
  -> Update Location
  -> Update Delivery State

ADMIN
  -> View System
  -> View Deliveries
  -> View Analytics
```

Authorization must be enforced server-side.

---

# 66. Data Consistency

Strong consistency is required inside transactional service boundaries.

Example:

```text
Delivery + Outbox
```

are committed in one PostgreSQL transaction.

Cross-service consistency is eventually consistent.

Example:

```text
Delivery
   |
Kafka
   |
Analytics
```

Analytics does not need immediate consistency with Delivery.

---

# 67. Event Schema

Events should contain:

```text
eventId
eventType
aggregateId
aggregateType
timestamp
version
producer
payload
metadata
```

Example:

```json
{
  "eventId": "evt-123",
  "eventType": "delivery.created",
  "aggregateId": "delivery-123",
  "aggregateType": "Delivery",
  "version": 1,
  "producer": "delivery-service",
  "timestamp": "...",
  "payload": {}
}
```

---

# 68. Event Versioning

Events must be versioned.

```text
delivery.created.v1
delivery.created.v2
```

Consumers must tolerate compatible schema evolution.

Breaking changes require a new version.

---

# 69. Ordering

Ordering is required for delivery lifecycle events.

Kafka partition key:

```text
deliveryId
```

Example:

```text
delivery.created
delivery.driver.assigned
delivery.driver.accepted
delivery.picked_up
delivery.in_transit
delivery.completed
```

These should remain ordered for the same delivery.

---

# 70. Exactly-Once Considerations

The system does not assume global exactly-once processing.

Instead it uses:

```text
At-least-once delivery
+
Idempotent Consumers
+
Transactional State Changes
```

This provides practical reliability without introducing unnecessary complexity.

---

# 71. Delivery Tracking

Customer tracking consists of:

```text
Current Delivery State
+
Driver Location
+
Estimated Progress
+
Status History
```

Delivery state comes from the Delivery Service.

Current location comes from Realtime/Redis.

Historical analytics comes from ClickHouse.

---

# 72. Location Data Strategy

Current driver location:

```text
Redis
```

Driver operational profile:

```text
MongoDB
```

Business delivery state:

```text
PostgreSQL
```

Long-term analytics:

```text
ClickHouse
```

This prevents high-frequency location writes from overwhelming PostgreSQL.

---

# 73. Data Retention

Different data has different retention policies.

```text
Redis
    -> Short-lived

Realtime Location
    -> Short-lived / configurable

Kafka
    -> Configurable event retention

PostgreSQL
    -> Business records

MongoDB
    -> Operational driver data

ClickHouse
    -> Long-term analytics

Object Storage
    -> Long-term files
```

---

# 74. Notification Reliability

Notification delivery must support:

```text
Retry
Backoff
Idempotency
Provider Failure Handling
DLQ
Status Tracking
```

Example:

```text
Notification
     |
Provider
     |
Failure
     |
Retry
     |
Retry
     |
DLQ
```

---

# 75. External Provider Isolation

External providers must be isolated behind provider interfaces.

Example:

```text
Notification Service
       |
       v
Notification Provider Interface
       |
       +---- Email Provider
       +---- SMS Provider
       +---- Push Provider
```

This prevents external provider details from leaking into business logic.

---

# 76. Payment Provider Isolation

Payment follows the same principle.

```text
Payment Service
      |
      v
Payment Provider Interface
      |
      +---- Provider A
      +---- Provider B
```

Provider-specific logic remains isolated.

---

# 77. Future AI Architecture

GenAI is a future phase and is NOT required for the initial implementation.

Future:

```text
                   AI Service
                     FastAPI
                        |
            +-----------+-----------+
            |           |           |
            v           v           v
          Chat         RAG        Agent
                        |
                        v
                      Qdrant
                        |
                        v
                       LLM
```

---

# 78. Future RAG

Potential knowledge sources:

```text
Delivery Documentation
Driver Policies
Support Documentation
Operational Rules
FAQ
System Documentation
```

Pipeline:

```text
Documents
   |
   v
Chunking
   |
   v
Embeddings
   |
   v
Qdrant
   |
   v
Similarity Search
   |
   v
LLM
```

---

# 79. Future AI Agent

The AI Agent may interact with services through controlled APIs.

```text
AI Agent
   |
   +---- gRPC -> Delivery
   |
   +---- gRPC -> Driver
   |
   +---- gRPC -> Notification
```

The AI Agent must not directly access service databases.

All AI actions must respect authorization and business rules.

---

# 80. Future OpenSearch

OpenSearch may be introduced later for advanced search.

Potential use cases:

```text
Delivery Search
Driver Search
Operational Search
Admin Search
Log Search
```

It is intentionally not required for the first implementation.

---

# 81. Future Debezium

The initial system uses:

```text
Transactional Outbox
+
Polling Publisher
```

Future:

```text
PostgreSQL WAL
      |
   Debezium
      |
     Kafka
```

Debezium should only be introduced when the operational complexity is justified.

---

# 82. Architectural Trade-offs

The architecture intentionally avoids unnecessary technologies.

Examples:

```text
No REST API
No GraphQL Subscriptions
No Event Sourcing
No Debezium initially
No SSE
No excessive microservices
```

The goal is to demonstrate the right technology for the right problem.

---

# 83. Technology Responsibility Matrix

| Requirement                  | Technology         |
| ---------------------------- | ------------------ |
| Public API                   | GraphQL Federation |
| Synchronous Communication    | gRPC               |
| Low-Latency Messaging        | NATS               |
| Durable Events               | Kafka              |
| Browser Realtime             | WebSocket          |
| Background Jobs              | BullMQ             |
| Cache                        | Redis              |
| Driver Proximity             | Redis GEO          |
| Distributed Locks            | Redis              |
| Idempotency                  | Redis + PostgreSQL |
| Transactional Data           | PostgreSQL         |
| Flexible Operational Data    | MongoDB            |
| Analytics                    | ClickHouse         |
| Files                        | Object Storage     |
| Containerization             | Docker             |
| Orchestration                | Kubernetes         |
| Local K8s Development        | Skaffold           |
| NestJS Shared Infrastructure | Common Package     |
| Future AI                    | FastAPI            |
| Future Vector Search         | Qdrant             |
| Future Search                | OpenSearch         |
| Future CDC                   | Debezium           |

---

# 84. Initial Implementation Phases

## Phase 1 — Foundation

```text
Repository
Docker
Docker Compose
NestJS
Go
PostgreSQL
MongoDB
Redis
```

## Phase 2 — Authentication

```text
Auth
User
JWT
GraphQL
Authorization
```

## Phase 3 — Delivery

```text
Delivery Service
State Machine
PostgreSQL
CQRS
```

## Phase 4 — Driver & Dispatch

```text
Go
MongoDB
Redis GEO
Driver Assignment
Distributed Locks
```

## Phase 5 — gRPC

```text
Proto
Delivery -> Driver
Delivery -> Payment
```

## Phase 6 — Payment

```text
Payment Service
Idempotency
Transactions
Saga
```

## Phase 7 — Event-Driven Architecture

```text
Kafka
Transactional Outbox
Consumers
Consumer Groups
DLQ
```

## Phase 8 — Notifications

```text
Notification Service
BullMQ
Workers
Retries
```

## Phase 9 — Realtime

```text
WebSocket
NATS
Redis
Horizontal Scaling
```

## Phase 10 — Analytics

```text
Analytics Service
ClickHouse
Kafka Consumers
```

## Phase 11 — Kubernetes

```text
Kubernetes
Deployments
Services
Ingress
Secrets
ConfigMaps
HPA
```

## Phase 12 — Skaffold

```text
Skaffold
Local Kubernetes Development
Automated Build/Deploy
```

## Phase 13 — Observability

```text
Logging
Metrics
Tracing
Correlation IDs
Health Checks
```

## Phase 14 — Advanced Reliability

```text
Circuit Breaker
Bulkhead
Advanced Retry
DLQ Management
Failure Testing
```

## Phase 15 — Future GenAI

```text
FastAPI
Qdrant
Embeddings
RAG
LLM
AI Agent
```

---

# 85. WebSocket vs SSE Decision

WebSocket is the selected realtime protocol.

Reason:

```text
Driver
  |
  +---- Send Location
  +---- Receive Assignment
  +---- Accept
  +---- Reject
  +---- Receive Commands
```

This requires bidirectional communication.

SSE is therefore not included in the initial architecture.

---

# 86. Redis GEO — Driver Proximity

Driver locations are indexed using Redis GEO.

```text
GEOADD
   |
   v
drivers:locations
   |
   v
GEOSEARCH
   |
   v
Nearest Available Drivers
```

Availability is maintained separately.

---

# 87. CQRS — Delivery Read Model

The Delivery Service separates commands from queries logically.

```text
Commands -> Write Model
Queries  -> Read Model
```

This allows future read scaling without introducing Event Sourcing.

---

# 88. DLQ Strategy

Each Kafka consumer must define:

```text
Retry Policy
Maximum Attempts
DLQ
Alerting
Manual Recovery
```

BullMQ workers follow the same principle.

---

# 89. WebSocket Authentication

JWT authentication occurs before accepting the WebSocket connection.

Invalid clients are rejected before entering the realtime system.

---

# 90. gRPC Contract Strategy

All cross-language communication uses `.proto` definitions.

The same contracts are consumed by:

```text
NestJS
Go
Future FastAPI integrations where required
```

The `.proto` files are the source of truth.

---

# 91. GraphQL API Strategy

GraphQL Federation is the only public API.

There is no REST API in the architecture.

```text
Client
   |
GraphQL
   |
Gateway
   |
Federated Subgraphs
```

---

# 92. Service Method Catalogue

Every service must maintain explicit application methods and contracts.

No service should expose internal implementation details.

---

# 93. Complete Delivery Creation Flow

```text
Client
  |
GraphQL
  |
Gateway
  |
Delivery
  |
PostgreSQL Transaction
  |
Outbox
  |
Kafka
  |
+---- Driver
+---- Notification
+---- Analytics
```

---

# 94. Complete Driver Assignment Flow

```text
Delivery
  |
gRPC
  |
Driver
  |
Redis GEO
  |
Distributed Lock
  |
Driver Reserved
  |
NATS
  |
Realtime
  |
Driver WebSocket
```

---

# 95. Complete Payment Flow

```text
Delivery Saga
  |
gRPC
  |
Payment
  |
Idempotency
  |
PostgreSQL
  |
Provider
  |
Kafka
  |
Saga
```

---

# 96. Complete Realtime Tracking Flow

```text
Driver
  |
WebSocket
  |
Realtime
  |
Redis
  |
NATS
  |
Realtime Nodes
  |
Customers
```

---

# 97. Complete Notification Flow

```text
Kafka
  |
Notification
  |
BullMQ
  |
Worker
  |
Provider
```

---

# 98. Kafka Topic Catalogue

```text
delivery.created
delivery.driver.assigned
delivery.driver.accepted
delivery.driver.rejected
delivery.pickup.started
delivery.picked_up
delivery.in_transit
delivery.completed
delivery.cancelled

payment.completed
payment.failed
payment.refunded

driver.available
driver.unavailable
```

DLQ:

```text
delivery.created.dlq
delivery.completed.dlq
payment.completed.dlq
payment.failed.dlq
```

---

# 99. NATS Subject Catalogue

```text
delivery.location.updated
delivery.status.updated
driver.location.updated
driver.assignment.updated
driver.presence.updated
```

NATS is used for transient realtime events.

---

# 100. Database Schema Definitions

All services own their schemas.

No cross-service database access is allowed.

Core entities:

```text
User
Driver
Delivery
DeliveryStatusHistory
Payment
Notification
OutboxEvent
```

---

# 101. Capacity Estimation

Target:

```text
100K users
10K active drivers
20K concurrent WebSockets
50K deliveries/day
5K deliveries/hour peak
5K location updates/sec
```

This validates the need for:

```text
Redis
NATS
WebSocket
Horizontal Scaling
```

---

# 102. Failure Scenarios

The system must explicitly handle:

```text
Database Failure
Redis Failure
Kafka Failure
NATS Failure
gRPC Timeout
Payment Failure
Driver Service Failure
Notification Failure
WebSocket Disconnect
Duplicate Event
Duplicate Request
Concurrent Driver Assignment
```

---

# 103. CDC / Debezium Future Note

Debezium is a future optimization for the Outbox pattern.

Initial:

```text
Outbox Polling
```

Future:

```text
PostgreSQL WAL
      |
Debezium
      |
Kafka
```

---

# 104. Architecture Validation Rules

Before implementation, verify:

```text
[ ] No REST API exists
[ ] GraphQL Federation is the public API
[ ] Every service owns its data
[ ] No cross-service DB access
[ ] gRPC is used for synchronous calls
[ ] Kafka is used for durable business events
[ ] NATS is used for low-latency transient events
[ ] WebSocket handles realtime communication
[ ] BullMQ handles background notification jobs
[ ] Saga handles distributed workflows
[ ] Outbox guarantees reliable event publishing
[ ] Redis handles ephemeral state and coordination
[ ] Redis GEO handles driver proximity
[ ] CQRS separates delivery reads/writes logically
[ ] DLQ exists for failed consumers
[ ] Idempotency exists for critical operations
[ ] Kubernetes manages production workloads
[ ] Docker containerizes services
[ ] Skaffold supports local Kubernetes development
[ ] Common NestJS package contains infrastructure only
[ ] GenAI remains a future phase
[ ] AI services never directly access databases
```

---

# 105. Final Architecture

```text
                              CLIENTS
                                 |
                    +------------+------------+
                    |                         |
                 GraphQL                   WebSocket
                    |                         |
                    v                         v
          +--------------------+     +--------------------+
          |  GraphQL Gateway   |     |  Realtime Service  |
          |      NestJS        |     |      NestJS        |
          |    Federation      |     |     WebSocket      |
          +---------+----------+     +---------+----------+
                    |                          |
       +------------+------------+            |
       |            |            |           NATS
       v            v            v            |
      Auth       Delivery      Driver         |
      User        NestJS      Dispatch        |
      NestJS         |           Go           |
        |            |           |            |
   PostgreSQL    PostgreSQL    MongoDB       |
                     |                         |
                   Outbox                    Redis
                     |
                   Kafka
                     |
       +-------------+-------------+
       |             |             |
       v             v             v
    Payment     Notification    Analytics
      Go           NestJS          Go
      |              |              |
 PostgreSQL        BullMQ       ClickHouse
                     |
                   Redis


                 Object Storage
                      |
             +--------+--------+
             |                 |
       Driver Documents   Delivery Proofs


                    FUTURE
                      |
                 AI Service
                   FastAPI
                      |
            +---------+---------+
            |                   |
           RAG                Agent
            |                   |
         Qdrant               gRPC
            |
           LLM


             FUTURE INFRASTRUCTURE
                      |
          +-----------+-----------+
          |                       |
      OpenSearch              Debezium
```

---

# 106. Final Technology Stack

```text
Languages
--------
TypeScript
Go

Frameworks
----------
NestJS
FastAPI (Future)

API
---
GraphQL Federation

Communication
-------------
gRPC
NATS
Kafka
WebSocket

Databases
---------
PostgreSQL
MongoDB
Redis
ClickHouse

Storage
-------
Object Storage

Async Processing
---------------
BullMQ

Distributed Patterns
--------------------
Saga
Transactional Outbox
CQRS
Idempotency
Distributed Locks
Circuit Breaker
Retry
DLQ

Infrastructure
--------------
Docker
Docker Compose
Kubernetes
Skaffold

Future AI
---------
FastAPI
Qdrant
RAG
LLM
AI Agents

Future Infrastructure
---------------------
OpenSearch
Debezium
```

---

# 107. Final Design Philosophy

The architecture is not designed to use as many technologies as possible.

Each technology exists because it solves a specific architectural problem.

```text
GraphQL
    -> Client API

gRPC
    -> Synchronous service communication

NATS
    -> Low-latency realtime messaging

Kafka
    -> Durable business events

WebSocket
    -> Realtime clients

BullMQ
    -> Background jobs

Redis
    -> Cache / GEO / Locks / Idempotency / Realtime state

PostgreSQL
    -> Transactions

MongoDB
    -> Flexible driver data

ClickHouse
    -> Analytics

Object Storage
    -> Files

Kubernetes
    -> Orchestration

Docker
    -> Containerization

Skaffold
    -> Kubernetes development

Saga
    -> Distributed transactions

Outbox
    -> Reliable event publishing

CQRS
    -> Scalable reads

GenAI
    -> Future intelligent capabilities
```

The final system should remain a **real-time delivery platform**, not an e-commerce system, and should avoid adding additional microservices unless a clear business or scalability requirement justifies them.

---

# 108. Auth & User Service — Detailed Workflow

## Overview

The Auth & User Service owns all identity, authentication, and authorization data.

It is the only service that can produce a valid JWT.

---

## 108.1 Registration Flow

```text
Client
   |
   | GraphQL Mutation: register(input)
   v
GraphQL Gateway
   |
   v
Auth & User Service
   |
   +---- Validate Input
   |
   +---- Check Email Uniqueness
   |
   +---- Hash Password (bcrypt)
   |
   +---- Create User Record (PostgreSQL)
   |
   +---- Assign Role (CUSTOMER or DRIVER)
   |
   +---- Return User
```

---

## 108.2 Login Flow

```text
Client
   |
   | GraphQL Mutation: login(email, password)
   v
Auth & User Service
   |
   +---- Find User by Email
   |
   +---- Verify Password Hash
   |
   +---- Generate Access Token (JWT)
   |
   +---- Generate Refresh Token
   |
   +---- Store Refresh Token (PostgreSQL)
   |
   +---- Return { accessToken, refreshToken }
```

JWT payload:

```text
userId
role
sessionId
iat
exp
```

---

## 108.3 Token Refresh Flow

```text
Client
   |
   | GraphQL Mutation: refreshToken(refreshToken)
   v
Auth & User Service
   |
   +---- Validate Refresh Token Signature
   |
   +---- Lookup Refresh Token in DB
   |
   +---- Check Not Revoked
   |
   +---- Check Not Expired
   |
   +---- Rotate: Revoke Old, Issue New
   |
   +---- Return { accessToken, refreshToken }
```

Refresh token rotation must be atomic.

---

## 108.4 Logout Flow

```text
Client
   |
   | GraphQL Mutation: logout
   v
Auth & User Service
   |
   +---- Revoke Refresh Token
   |
   +---- Return Success
```

---

## 108.5 Token Validation (Internal)

```text
GraphQL Gateway
   |
   | Validate JWT
   v
Auth & User Service
   |
   +---- Verify Signature
   |
   +---- Check Expiry
   |
   +---- Return { userId, role, sessionId }
```

---

## 108.6 Profile Management

```text
getMyProfile()
updateMyProfile(input)
changePassword(input)
```

---

## 108.7 Service Methods

```text
register(input)             -> User
login(email, password)      -> AuthTokens
logout(userId)              -> void
refreshToken(token)         -> AuthTokens
validateToken(token)        -> UserContext
getUser(userId)             -> User
getUserByEmail(email)       -> User
updateUser(userId, input)   -> User
changePassword(userId, ...) -> void
assignRole(userId, role)    -> User
```

---

## 108.8 PostgreSQL Schema

```text
users
-----
id
email
passwordHash
role
firstName
lastName
phoneNumber
isActive
createdAt
updatedAt

refresh_tokens
--------------
id
userId
token
expiresAt
revokedAt
createdAt
```

---

# 109. Delivery Service — Detailed Workflow

## Overview

The Delivery Service is the primary business service.

It owns:

```text
Delivery lifecycle
Delivery state machine
Saga orchestration
Transactional Outbox
CQRS read model
```

---

## 109.1 Create Delivery Flow

```text
1. Client sends: createDelivery(input) via GraphQL

2. Gateway validates JWT, extracts userId

3. Delivery Service receives command:
   {
     customerId,
     pickupAddress,
     dropoffAddress,
     packageDescription,
     estimatedWeight,
   }

4. BEGIN TRANSACTION
   |
   +---- Validate addresses
   |
   +---- Calculate pricing
   |
   +---- Create Delivery record (status=PENDING)
   |
   +---- Create DeliveryStatusHistory record
   |
   +---- Create OutboxEvent { delivery.created }
   |
   COMMIT

5. Outbox Publisher reads OutboxEvent

6. Publishes to Kafka: delivery.created

7. Return Delivery to client
```

---

## 109.2 Delivery State Machine Transitions

```text
PENDING
  |
  +-- [System: Find Driver] --> SEARCHING_DRIVER
  |
  +-- [Customer: Cancel]    --> CANCELLED

SEARCHING_DRIVER
  |
  +-- [Driver Found]        --> DRIVER_ASSIGNED
  |
  +-- [No Driver Found]     --> FAILED
  |
  +-- [Customer: Cancel]    --> CANCELLED

DRIVER_ASSIGNED
  |
  +-- [Driver: Accept]      --> DRIVER_ACCEPTED
  |
  +-- [Driver: Reject]      --> SEARCHING_DRIVER (retry)
  |
  +-- [Timeout]             --> SEARCHING_DRIVER (retry)
  |
  +-- [Customer: Cancel]    --> CANCELLED

DRIVER_ACCEPTED
  |
  +-- [Driver: StartPickup] --> PICKUP_STARTED

PICKUP_STARTED
  |
  +-- [Driver: Pickup]      --> PICKED_UP

PICKED_UP
  |
  +-- [Driver: Transit]     --> IN_TRANSIT

IN_TRANSIT
  |
  +-- [Driver: Complete]    --> DELIVERED

DELIVERED
  --> terminal state
CANCELLED
  --> terminal state
FAILED
  --> terminal state
```

Each transition must:

```text
1. Validate the transition is legal
2. Update the Delivery record
3. Append to DeliveryStatusHistory
4. Write OutboxEvent
5. Commit atomically
```

---

## 109.3 CQRS: Commands

```text
CreateDelivery(input)
CancelDelivery(deliveryId, requestedBy)
AssignDriver(deliveryId, driverId)
AcceptDelivery(deliveryId, driverId)
RejectDelivery(deliveryId, driverId)
StartPickup(deliveryId, driverId)
MarkPickedUp(deliveryId, driverId)
StartTransit(deliveryId, driverId)
CompleteDelivery(deliveryId, driverId, proofUrl)
MarkPaymentCompleted(deliveryId, paymentId)
MarkPaymentFailed(deliveryId, reason)
```

---

## 109.4 CQRS: Queries

```text
GetDelivery(deliveryId)
GetActiveDelivery(customerId)
GetDeliveries(customerId, page, limit)
GetDeliveryHistory(customerId, page, limit)
GetDeliveryStatus(deliveryId)
GetDeliveryStatusHistory(deliveryId)
GetDriverActiveDelivery(driverId)
GetAllDeliveries(filter, page, limit)         [ADMIN]
GetDeliveryMetrics(timeRange)                  [ADMIN]
```

---

## 109.5 Kafka Consumer: Delivery Service

The Delivery Service also consumes events:

```text
payment.completed
   -> MarkPaymentCompleted(deliveryId)
   -> Advance saga

payment.failed
   -> MarkPaymentFailed(deliveryId)
   -> Trigger compensation

delivery.driver.accepted
   -> AcceptDelivery(deliveryId, driverId)
   -> Trigger payment step

delivery.driver.rejected
   -> RejectDelivery(deliveryId)
   -> Retry driver search or cancel
```

---

## 109.6 Outbox Events Produced

```text
delivery.created
delivery.driver.assigned
delivery.driver.accepted
delivery.driver.rejected
delivery.pickup.started
delivery.picked_up
delivery.in_transit
delivery.completed
delivery.cancelled
delivery.failed
```

---

## 109.7 PostgreSQL Schema

```text
deliveries
----------
id
customerId
driverId (nullable)
pickupAddressId
dropoffAddressId
status
price
currency
paymentStatus
paymentId (nullable)
proofUrl (nullable)
createdAt
updatedAt

delivery_addresses
------------------
id
deliveryId
type (PICKUP | DROPOFF)
street
city
country
postalCode
latitude
longitude

delivery_status_history
------------------------
id
deliveryId
fromStatus
toStatus
changedBy
reason (nullable)
createdAt

outbox_events
-------------
id
eventId
aggregateId
aggregateType
eventType
version
payload (jsonb)
published
createdAt
publishedAt
```

---

# 110. Driver & Dispatch Service — Detailed Workflow

## Overview

The Driver & Dispatch Service is written in Go.

It manages:

```text
Driver profiles
Driver state (AVAILABLE, BUSY, OFFLINE)
Driver location (Redis GEO)
Driver assignment
Dispatch logic
```

---

## 110.1 Driver Availability State Machine

```text
OFFLINE
   |
   +-- [Driver: GoOnline]   --> AVAILABLE
   |

AVAILABLE
   |
   +-- [Assignment]         --> BUSY
   |
   +-- [Driver: GoOffline]  --> OFFLINE
   |

BUSY
   |
   +-- [Delivery Complete]  --> AVAILABLE
   |
   +-- [Driver: GoOffline]  --> OFFLINE (after delivery)
```

---

## 110.2 Location Update Flow

```text
Driver
   |
   | WebSocket: location update
   v
Realtime Service
   |
   | NATS: driver.location.updated
   v
Driver & Dispatch Service
   |
   +---- GEOADD drivers:locations <lng> <lat> <driverId>
   |
   +---- SET driver:location:{driverId} { lat, lng, timestamp }
   |
   +---- Publish to NATS: driver.location.updated
```

---

## 110.3 Find Available Driver Flow

```text
Delivery Service
   |
   | gRPC: FindAvailableDriver(lat, lng, radius, vehicleType)
   v
Driver & Dispatch Service
   |
   +---- GEOSEARCH drivers:locations FROMLONLAT <lng> <lat>
   |       BYRADIUS 5 km ASC COUNT 10
   |
   +---- Filter: only AVAILABLE drivers
   |
   +---- Filter: matching vehicleType (if required)
   |
   +---- Return ordered list of { driverId, distance }
```

---

## 110.4 Driver Assignment Flow

```text
Driver & Dispatch Service
   |
   +---- For each candidate driver:
   |
   |   +---- Acquire Redis Lock: driver:lock:{driverId}
   |   |       TTL: 30s, retry: 3x with backoff
   |   |
   |   +---- Check driver still AVAILABLE
   |   |
   |   +---- Mark driver BUSY (MongoDB)
   |   |
   |   +---- Create DriverAssignment (MongoDB)
   |   |
   |   +---- Release Lock
   |   |
   |   +---- Return AssignDriverResponse
   |
   +---- If all candidates fail:
           Return: NO_DRIVER_AVAILABLE
```

---

## 110.5 Driver Accept/Reject Flow

```text
Driver
   |
   | WebSocket: accept_assignment / reject_assignment
   v
Realtime Service
   |
   | NATS: driver.assignment.updated
   v
Driver & Dispatch Service
   |
   +---- Validate assignment belongs to driver
   |
   +---- [Accept]:
   |       Update DriverAssignment status -> ACCEPTED
   |       Publish Kafka: delivery.driver.accepted
   |
   +---- [Reject]:
           Update DriverAssignment status -> REJECTED
           Set driver -> AVAILABLE
           Publish Kafka: delivery.driver.rejected
```

---

## 110.6 gRPC Methods (Server)

```proto
service DriverService {
  rpc FindAvailableDriver(FindAvailableDriverRequest)
    returns (FindAvailableDriverResponse);

  rpc AssignDriver(AssignDriverRequest)
    returns (AssignDriverResponse);

  rpc ReleaseDriver(ReleaseDriverRequest)
    returns (ReleaseDriverResponse);

  rpc GetDriver(GetDriverRequest)
    returns (GetDriverResponse);

  rpc GetDriverStatus(GetDriverStatusRequest)
    returns (GetDriverStatusResponse);

  rpc GetActiveAssignment(GetActiveAssignmentRequest)
    returns (GetActiveAssignmentResponse);
}
```

---

## 110.7 NATS Subscriptions

```text
driver.location.updated     -> Update Redis GEO
driver.assignment.updated   -> Process accept/reject
driver.presence.updated     -> Update availability cache
```

---

## 110.8 NATS Publications

```text
delivery.driver.assignment.sent   -> notify Realtime
driver.location.updated           -> fan-out to Realtime nodes
```

---

## 110.9 MongoDB Schema

```text
drivers
-------
_id
userId
status           (AVAILABLE | BUSY | OFFLINE)
vehicleType      (BIKE | CAR | VAN | TRUCK)
vehicleNumber
skills           []string
rating
totalDeliveries
isVerified
onlineAt
offlineAt
createdAt
updatedAt

driver_assignments
------------------
_id
deliveryId
driverId
status          (PENDING | ACCEPTED | REJECTED | COMPLETED)
assignedAt
acceptedAt
rejectedAt
completedAt
```

---

## 110.10 Redis Keys

```text
drivers:locations                     -> Redis GEO sorted set
driver:location:{driverId}            -> Latest location hash
driver:availability:{driverId}        -> AVAILABLE | BUSY | OFFLINE
driver:lock:{driverId}                -> Distributed lock
driver:assignment:{driverId}          -> Current deliveryId
```

---

# 111. Payment Service — Detailed Workflow

## Overview

The Payment Service is written in Go.

It owns all payment operations:

```text
Payment creation
Payment processing
Payment state
Refunds
Idempotency
```

---

## 111.1 Payment State Machine

```text
PENDING
   |
   +-- [Process]            --> PROCESSING
   |
   +-- [Cancel]             --> CANCELLED

PROCESSING
   |
   +-- [Provider Success]   --> COMPLETED
   |
   +-- [Provider Failure]   --> FAILED
   |

COMPLETED
   |
   +-- [Refund Request]     --> REFUNDING
   |

REFUNDING
   |
   +-- [Refund Complete]    --> REFUNDED
   |
   +-- [Refund Failed]      --> REFUND_FAILED

FAILED  -> terminal
CANCELLED -> terminal
```

---

## 111.2 Create & Process Payment Flow

```text
Delivery Saga
   |
   | gRPC: CreatePayment(deliveryId, amount, currency, idempotencyKey)
   v
Payment Service
   |
   +---- Check idempotency:
   |       GET idempotency:{key} from Redis
   |       If found -> Return cached result
   |
   +---- BEGIN TRANSACTION
   |
   +---- Create Payment record (status=PENDING)
   |
   +---- COMMIT
   |
   +---- Call Payment Provider
   |
   +---- BEGIN TRANSACTION
   |       Update Payment (status=PROCESSING or COMPLETED or FAILED)
   |       Create PaymentTransaction record
   |       Store idempotency result in Redis (TTL: 24h)
   |       COMMIT
   |
   +---- Publish Kafka:
           payment.completed  or  payment.failed
```

---

## 111.3 Refund Flow

```text
Delivery Saga (compensation)
   |
   | gRPC: RefundPayment(paymentId, reason, idempotencyKey)
   v
Payment Service
   |
   +---- Check idempotency
   |
   +---- Verify payment is COMPLETED
   |
   +---- Call Provider Refund API
   |
   +---- Update Payment status -> REFUNDING -> REFUNDED
   |
   +---- Publish Kafka: payment.refunded
```

---

## 111.4 gRPC Methods (Server)

```proto
service PaymentService {
  rpc CreatePayment(CreatePaymentRequest)
    returns (CreatePaymentResponse);

  rpc ProcessPayment(ProcessPaymentRequest)
    returns (ProcessPaymentResponse);

  rpc GetPayment(GetPaymentRequest)
    returns (GetPaymentResponse);

  rpc GetPaymentStatus(GetPaymentStatusRequest)
    returns (GetPaymentStatusResponse);

  rpc RefundPayment(RefundPaymentRequest)
    returns (RefundPaymentResponse);

  rpc GetPaymentByIdempotencyKey(GetByIdempotencyKeyRequest)
    returns (GetPaymentResponse);
}
```

---

## 111.5 Kafka Events Produced

```text
payment.completed
payment.failed
payment.refunded
```

Event envelope:

```json
{
  "eventId": "uuid",
  "eventType": "payment.completed",
  "aggregateId": "payment-id",
  "aggregateType": "Payment",
  "version": 1,
  "producer": "payment-service",
  "timestamp": "...",
  "payload": {
    "paymentId": "...",
    "deliveryId": "...",
    "amount": 25.00,
    "currency": "USD",
    "status": "COMPLETED"
  }
}
```

---

## 111.6 PostgreSQL Schema

```text
payments
--------
id
deliveryId
customerId
amount
currency
status
idempotencyKey
providerTransactionId (nullable)
failureReason (nullable)
createdAt
updatedAt

payment_transactions
--------------------
id
paymentId
type        (CHARGE | REFUND)
amount
status
providerResponse (jsonb)
createdAt

refunds
-------
id
paymentId
amount
reason
status
idempotencyKey
createdAt
processedAt
```

---

# 112. Notification Service — Detailed Workflow

## Overview

The Notification Service is written in NestJS.

It is entirely event-driven — it does not receive GraphQL mutations for sending notifications.

---

## 112.1 Kafka Consumer Setup

The Notification Service subscribes to:

```text
delivery.created
delivery.driver.assigned
delivery.driver.accepted
delivery.driver.rejected
delivery.pickup.started
delivery.picked_up
delivery.in_transit
delivery.completed
delivery.cancelled

payment.completed
payment.failed
payment.refunded
```

Consumer group:

```text
notification-service-group
```

---

## 112.2 Event-to-Notification Mapping

```text
delivery.created
   -> Customer: "Your delivery has been created"
   -> Channel: Email + In-App

delivery.driver.assigned
   -> Customer: "A driver has been assigned"
   -> Channel: Push + In-App

delivery.driver.accepted
   -> Customer: "Your driver accepted the delivery"
   -> Channel: Push + In-App

delivery.pickup.started
   -> Customer: "Driver is heading to pickup location"
   -> Channel: Push + In-App

delivery.picked_up
   -> Customer: "Package has been picked up"
   -> Channel: Push + In-App

delivery.in_transit
   -> Customer: "Package is on its way"
   -> Channel: Push + In-App

delivery.completed
   -> Customer: "Delivery completed successfully"
   -> Channel: Email + Push + In-App

delivery.cancelled
   -> Customer: "Delivery has been cancelled"
   -> Channel: Email + Push + In-App

payment.completed
   -> Customer: "Payment successful"
   -> Channel: Email + In-App

payment.failed
   -> Customer: "Payment failed. Please retry."
   -> Channel: Email + Push + In-App
```

---

## 112.3 BullMQ Job Processing

```text
Kafka Consumer
   |
   | Deserialize event
   v
Resolve user preferences
   |
   v
Create Notification record (PostgreSQL)
   |
   v
Add job to BullMQ queue:
   {
     notificationId,
     userId,
     channel,  (EMAIL | SMS | PUSH | IN_APP)
     template,
     payload
   }
   |
   v
BullMQ Worker processes job
   |
   v
Call provider
   |
   +---- Success -> Update status SENT
   |
   +---- Failure -> Retry (exponential backoff)
                       |
                    Max retries
                       |
                      DLQ
```

---

## 112.4 BullMQ Queue Configuration

```text
email-notifications
   concurrency: 5
   maxAttempts: 5
   backoff: exponential (2s, 4s, 8s, 16s, 32s)
   removeOnComplete: false
   removeOnFail: false

push-notifications
   concurrency: 20
   maxAttempts: 3
   backoff: exponential (1s, 2s, 4s)

in-app-notifications
   concurrency: 10
   maxAttempts: 3
   backoff: exponential (1s, 2s, 4s)
```

---

## 112.5 Idempotency

Each notification job must be idempotent.

```text
idempotency key = eventId + channel + userId
```

If the same event is received twice, the second should not create a duplicate notification.

---

## 112.6 Service Methods

```text
createNotification(userId, type, channel, payload)
markNotificationSent(notificationId)
markNotificationFailed(notificationId, reason)
getNotifications(userId, page, limit)
getUnreadCount(userId)
markAsRead(notificationId)
markAllAsRead(userId)
scheduleNotification(userId, type, channel, payload, sendAt)
retryFailedNotification(notificationId)
```

---

## 112.7 PostgreSQL Schema

```text
notifications
-------------
id
userId
type
channel   (EMAIL | SMS | PUSH | IN_APP)
status    (PENDING | SENT | FAILED | READ)
title
body
payload (jsonb)
eventId
isRead
readAt
sentAt
failureReason
createdAt
updatedAt

notification_templates
----------------------
id
type
channel
titleTemplate
bodyTemplate
createdAt
updatedAt
```

---

# 113. Realtime Service — Detailed Workflow

## Overview

The Realtime Service is written in NestJS.

It is the bridge between:

```text
WebSocket Clients  <-->  Platform Internal (NATS / Redis)
```

It does NOT own business data.

---

## 113.1 WebSocket Connection Lifecycle

```text
1. Client opens WebSocket connection
   |
   | HTTP Upgrade Request
   | Authorization: Bearer <JWT>
   v

2. Realtime Service
   |
   +---- Extract JWT from header or query param
   |
   +---- Validate JWT
   |
   +---- [Invalid] -> Reject (401, close connection)
   |
   +---- [Valid] -> Upgrade to WebSocket
   |
   +---- Store connection context:
           {
             socketId,
             userId,
             role,
             connectedAt
           }

3. Client is connected

4. Client sends subscribeToDelivery(deliveryId)
   |
   +---- Validate authorization (can this user see this delivery?)
   |
   +---- Register: delivery:{deliveryId}:subscribers -> add socketId
   |
   +---- Client is now subscribed

5. Updates are pushed as they arrive via NATS

6. Client disconnects
   |
   +---- Remove from all subscriptions
   |
   +---- Clean up Redis state
   |
   +---- Log disconnect
```

---

## 113.2 Location Update Flow (Driver → Customer)

```text
Driver
   |
   | WebSocket message: { type: LOCATION_UPDATE, lat, lng, deliveryId }
   v

Realtime Service (Node A)
   |
   +---- Update Redis:
   |       SET driver:location:{driverId} { lat, lng, ts }
   |       GEOADD drivers:locations lng lat driverId
   |
   +---- Publish NATS: delivery.location.updated
           {
             deliveryId,
             driverId,
             latitude,
             longitude,
             timestamp
           }

All NATS subscribers (Node A, B, C)
   |
   v
Find all WebSocket clients subscribed to this deliveryId
   |
   v
Push to each client:
   {
     type: LOCATION_UPDATE,
     deliveryId,
     latitude,
     longitude,
     timestamp
   }
```

---

## 113.3 Delivery Status Update Flow

```text
Kafka: delivery.status.updated
   |
   | (Realtime subscribes to relevant Kafka topics OR receives via NATS)
   v
Realtime Service
   |
   +---- Find all subscribers of deliveryId
   |
   +---- Push WebSocket message:
           {
             type: STATUS_UPDATE,
             deliveryId,
             status,
             changedAt
           }
```

---

## 113.4 Cross-Instance Fan-Out

```text
Driver connects to Node A
Customer connects to Node B

Node A receives location update
   |
   +---- Publishes NATS: delivery.location.updated

Node B receives NATS message
   |
   +---- Finds customer's socket connection
   |
   +---- Pushes to Customer
```

NATS is the cross-instance event bus.

Redis stores connection metadata for routing.

---

## 113.5 WebSocket Message Protocol

Inbound (Client → Server):

```json
{
  "type": "SUBSCRIBE_DELIVERY",
  "deliveryId": "..."
}

{
  "type": "UNSUBSCRIBE_DELIVERY",
  "deliveryId": "..."
}

{
  "type": "LOCATION_UPDATE",
  "deliveryId": "...",
  "latitude": 31.2,
  "longitude": 31.8
}

{
  "type": "PING"
}
```

Outbound (Server → Client):

```json
{
  "type": "LOCATION_UPDATE",
  "deliveryId": "...",
  "driverId": "...",
  "latitude": 31.2,
  "longitude": 31.8,
  "timestamp": "..."
}

{
  "type": "STATUS_UPDATE",
  "deliveryId": "...",
  "status": "IN_TRANSIT",
  "changedAt": "..."
}

{
  "type": "DRIVER_ASSIGNED",
  "deliveryId": "...",
  "driverId": "...",
  "driverName": "..."
}

{
  "type": "PONG"
}

{
  "type": "ERROR",
  "code": "UNAUTHORIZED",
  "message": "..."
}
```

---

## 113.6 Service Methods

```text
handleConnection(socket, jwt)
handleDisconnect(socketId)
authenticate(socket, jwt)
subscribeToDelivery(socketId, deliveryId)
unsubscribeFromDelivery(socketId, deliveryId)
handleLocationUpdate(socketId, payload)
handlePing(socketId)
broadcastLocationUpdate(deliveryId, payload)
broadcastStatusUpdate(deliveryId, payload)
broadcastDriverAssigned(deliveryId, driverId)
getConnectedCount()
getSubscribers(deliveryId)
```

---

## 113.7 Redis Keys

```text
ws:connection:{socketId}             -> { userId, role, connectedAt }
ws:user:{userId}:sockets             -> Set of socketIds
ws:delivery:{deliveryId}:subscribers -> Set of socketIds
ws:stats:connected_count             -> Integer
driver:location:{driverId}           -> { lat, lng, timestamp }
drivers:locations                    -> Redis GEO sorted set
```

---

## 113.8 NATS Subjects Published

```text
delivery.location.updated
delivery.status.updated
driver.presence.updated
```

---

## 113.9 NATS Subjects Subscribed

```text
delivery.location.updated
delivery.status.updated
driver.assignment.updated
driver.presence.updated
```

---

# 114. Analytics Service — Detailed Workflow

## Overview

The Analytics Service is written in Go.

It is a pure Kafka consumer that writes events to ClickHouse.

It does not expose any API to other services.

---

## 114.1 Kafka Consumer Setup

```text
Consumer Group: analytics-service-group

Topics:
  delivery.created
  delivery.driver.assigned
  delivery.driver.accepted
  delivery.driver.rejected
  delivery.pickup.started
  delivery.picked_up
  delivery.in_transit
  delivery.completed
  delivery.cancelled

  payment.completed
  payment.failed
  payment.refunded

  driver.available
  driver.unavailable
```

---

## 114.2 Event Processing Flow

```text
Kafka Event
   |
   v
Analytics Consumer
   |
   +---- Deserialize event
   |
   +---- Validate schema
   |
   +---- Check idempotency (eventId seen before?)
   |
   +---- Transform to ClickHouse row
   |
   +---- Batch insert to ClickHouse
   |
   +---- Commit Kafka offset
```

Batch insert strategy:

```text
Buffer events
   |
   v
Every 1000 events OR every 5 seconds
   |
   v
Batch INSERT into ClickHouse
```

---

## 114.3 ClickHouse Tables

```text
delivery_events
---------------
event_id     UUID
event_type   String
delivery_id  UUID
customer_id  UUID
driver_id    Nullable(UUID)
status       String
occurred_at  DateTime64
created_at   DateTime64

payment_events
--------------
event_id       UUID
event_type     String
payment_id     UUID
delivery_id    UUID
customer_id    UUID
amount         Decimal(10,2)
currency       String
status         String
occurred_at    DateTime64
created_at     DateTime64

driver_events
-------------
event_id    UUID
event_type  String
driver_id   UUID
status      String
occurred_at DateTime64
created_at  DateTime64
```

---

## 114.4 Key Metrics Queries

Total deliveries per day:

```sql
SELECT
  toDate(occurred_at) AS day,
  count() AS total
FROM delivery_events
WHERE event_type = 'delivery.created'
GROUP BY day
ORDER BY day DESC;
```

Average delivery time:

```sql
SELECT
  avg(dateDiff('minute', created_at, completed_at)) AS avg_minutes
FROM (
  SELECT
    delivery_id,
    minIf(occurred_at, event_type = 'delivery.created') AS created_at,
    maxIf(occurred_at, event_type = 'delivery.completed') AS completed_at
  FROM delivery_events
  GROUP BY delivery_id
  HAVING completed_at > created_at
);
```

Driver acceptance rate:

```sql
SELECT
  countIf(event_type = 'delivery.driver.accepted') /
  countIf(event_type = 'delivery.driver.assigned') AS acceptance_rate
FROM delivery_events
WHERE occurred_at >= now() - INTERVAL 1 DAY;
```

---

## 114.5 Service Methods

```text
consumeDeliveryEvent(event)
consumePaymentEvent(event)
consumeDriverEvent(event)
insertDeliveryEventBatch(events[])
insertPaymentEventBatch(events[])
insertDriverEventBatch(events[])
isEventProcessed(eventId)
markEventProcessed(eventId)
```

---

# 115. GraphQL Gateway — Detailed Workflow

## Overview

The GraphQL Gateway is the only public-facing API.

It uses NestJS + Apollo Federation.

---

## 115.1 Request Lifecycle

```text
1. Client sends GraphQL request (HTTP POST)

2. Gateway receives request

3. Rate Limiting Check
   |
   +---- GET rate:ip:{ip} from Redis
   |
   +---- Increment count
   |
   +---- If over limit -> 429 Too Many Requests

4. Authentication
   |
   +---- Extract Bearer JWT
   |
   +---- Validate signature + expiry
   |
   +---- Extract { userId, role }

5. Request Parsing + Validation
   |
   +---- Parse GraphQL document
   |
   +---- Validate against schema
   |
   +---- Check query depth / complexity

6. Federation Router
   |
   +---- Determine which subgraphs to call
   |
   +---- Forward requests with user context headers:
           X-User-Id: uuid
           X-User-Role: CUSTOMER
           X-Correlation-Id: uuid

7. Aggregate responses

8. Return to client
```

---

## 115.2 Authentication Middleware

```text
Request
   |
   | Extract Authorization header
   v
Check public operations (login, register)
   |
   +---- [Public] -> Skip auth
   |
   +---- [Protected]
              |
              v
         Validate JWT
              |
              +---- [Invalid] -> Return 401
              |
              +---- [Valid] -> Attach user context
```

---

## 115.3 Federation Subgraphs

```text
Gateway
   |
   +---- Auth Subgraph (Auth & User Service)
   +---- Delivery Subgraph (Delivery Service)
   +---- Driver Subgraph (Driver & Dispatch Service)
   +---- Payment Subgraph (Payment Service)
   +---- Notification Subgraph (Notification Service)
```

Each subgraph exposes its own types.

The Gateway composes them into a unified schema.

---

## 115.4 Gateway Responsibilities

```text
JWT Validation
Rate Limiting
Correlation ID injection
Request context propagation
Schema composition
Query plan execution
Error normalization
Tracing headers
```

---

## 115.5 Gateway Non-Responsibilities

```text
Business logic
Database access
Event publishing
Payment processing
Driver assignment
```

---

# 116. Cross-Service Saga — Complete Step-by-Step

## Overview

The Delivery Saga is orchestrated by the Delivery Service.

This section describes the complete end-to-end flow.

---

## 116.1 Full Delivery Lifecycle

```text
STEP 1: Customer Creates Delivery
─────────────────────────────────
Client
  -> GraphQL: createDelivery(input)
  -> Gateway
  -> Delivery Service

  BEGIN TRANSACTION
    Create Delivery (PENDING)
    Create OutboxEvent: delivery.created
  COMMIT

  Outbox Publisher -> Kafka: delivery.created

  Kafka consumers:
    Notification: "Delivery created" notification
    Analytics: Record delivery.created event


STEP 2: Driver Search
─────────────────────
Delivery Service
  -> gRPC: FindAvailableDriver(lat, lng, radius)
  -> Driver Service

  Driver Service:
    GEOSEARCH Redis GEO
    Filter AVAILABLE drivers
    Return driver candidates

  Delivery Service:
    For each candidate:
      gRPC: AssignDriver(driverId, deliveryId)


STEP 3: Driver Reservation
──────────────────────────
Driver Service:
  Acquire Redis Lock: driver:lock:{driverId}
  Check driver still AVAILABLE
  Mark driver BUSY (MongoDB)
  Create DriverAssignment (PENDING)
  Release lock

  Publish NATS: driver.assignment.sent

Realtime Service:
  Receives NATS message
  Pushes to Driver's WebSocket:
    { type: DELIVERY_ASSIGNED, deliveryId }

  BEGIN TRANSACTION (Delivery Service)
    Update Delivery: status -> DRIVER_ASSIGNED
    Create OutboxEvent: delivery.driver.assigned
  COMMIT

  Outbox -> Kafka: delivery.driver.assigned

  Kafka consumers:
    Notification: "Driver assigned" push
    Analytics: Record event


STEP 4: Driver Accepts
──────────────────────
Driver
  -> WebSocket: { type: ACCEPT_ASSIGNMENT, deliveryId }
  -> Realtime Service
  -> NATS: driver.assignment.updated

Driver Service:
  Update DriverAssignment -> ACCEPTED
  Publish Kafka: delivery.driver.accepted

Delivery Service (Kafka consumer: delivery.driver.accepted):
  BEGIN TRANSACTION
    Update Delivery: status -> DRIVER_ACCEPTED
    Create OutboxEvent: delivery.driver.accepted
  COMMIT

  Trigger next saga step: Process Payment


STEP 5: Payment Processing
──────────────────────────
Delivery Service (Saga)
  -> gRPC: CreatePayment(deliveryId, amount, idempotencyKey)
  -> Payment Service

  Payment Service:
    Check idempotency key (Redis)
    Create Payment (PENDING)
    Call Payment Provider
    Update Payment (COMPLETED)
    Store idempotency result (Redis, TTL 24h)
    Publish Kafka: payment.completed

Delivery Service (Kafka consumer: payment.completed):
  BEGIN TRANSACTION
    Update Delivery: paymentStatus -> PAID
    Create OutboxEvent: delivery.payment.completed
  COMMIT


STEP 6: Active Delivery
───────────────────────
Driver performs delivery:

  Driver
    -> WebSocket: START_PICKUP
    -> Realtime -> NATS -> Delivery Service

  Delivery Service:
    Transition: DRIVER_ACCEPTED -> PICKUP_STARTED
    OutboxEvent -> Kafka

  Driver
    -> WebSocket: MARK_PICKED_UP
    -> Delivery Service:
       Transition: PICKUP_STARTED -> PICKED_UP

  Driver
    -> WebSocket: START_TRANSIT
    -> Delivery Service:
       Transition: PICKED_UP -> IN_TRANSIT


STEP 7: Complete Delivery
─────────────────────────
Driver
  -> WebSocket: COMPLETE_DELIVERY { proofUrl }
  -> Delivery Service

  BEGIN TRANSACTION
    Update Delivery: status -> DELIVERED
    Store proofUrl
    Create OutboxEvent: delivery.completed
  COMMIT

  Outbox -> Kafka: delivery.completed

  Kafka consumers:
    Notification: "Delivery completed" email + push
    Analytics: Record completion

  gRPC -> Driver Service: ReleaseDriver(driverId)
  Driver Service:
    Update driver -> AVAILABLE
    Update DriverAssignment -> COMPLETED


STEP 8: Saga Complete
─────────────────────
All steps completed.

Delivery in terminal state: DELIVERED

All relevant parties notified.

All analytics recorded.

Driver available for next delivery.
```

---

## 116.2 Compensation: Payment Failure

```text
STEP 1-4 completed (Driver accepted)

STEP 5: Payment Fails
─────────────────────
Payment Service:
  Payment -> FAILED
  Publish Kafka: payment.failed

Delivery Service (consumer: payment.failed):
  Trigger compensation

  Compensation A: Release Driver
    -> gRPC: ReleaseDriver(driverId)
    -> Driver Service: driver -> AVAILABLE

  Compensation B: Cancel Delivery
    BEGIN TRANSACTION
      Update Delivery: status -> CANCELLED
      Create OutboxEvent: delivery.cancelled
    COMMIT

  Notification: "Payment failed, delivery cancelled"

  Saga ends.
```

---

## 116.3 Compensation: Driver Rejects

```text
STEP 3: Driver Reserved
STEP 4: Driver Rejects
──────────────────────
Driver
  -> WebSocket: REJECT_ASSIGNMENT
  -> Driver Service:
       DriverAssignment -> REJECTED
       Driver -> AVAILABLE
       Publish Kafka: delivery.driver.rejected

Delivery Service (consumer: delivery.driver.rejected):
  Attempt next driver candidate

  If no more candidates:
    Update Delivery: status -> FAILED
    OutboxEvent: delivery.failed
    Notification: "No driver available"

  Saga ends with failure.
```

---

# 117. Outbox Publisher — Detailed Workflow

## Overview

The Outbox Publisher is a background process inside the Delivery Service.

---

## 117.1 Publisher Flow

```text
Scheduler: every 1 second

Outbox Publisher:
  |
  +---- SELECT * FROM outbox_events
  |       WHERE published = false
  |       ORDER BY created_at ASC
  |       LIMIT 100
  |
  +---- FOR EACH event:
  |       |
  |       +---- Publish to Kafka (with eventType as topic)
  |       |
  |       +---- [Success]:
  |       |       UPDATE outbox_events
  |       |       SET published = true, published_at = now()
  |       |       WHERE id = event.id
  |       |
  |       +---- [Failure]:
  |               Log error
  |               Leave as unpublished
  |               Retry on next poll cycle
  |
  +---- Release lock
```

---

## 117.2 Distributed Lock for Publisher

Multiple Delivery Service instances must not publish the same event twice.

```text
TRY Acquire Redis Lock: outbox:publisher:lock
  TTL: 30s

If lock acquired:
  Run publisher cycle
  Release lock

If lock not acquired:
  Skip this cycle
  Another instance is publishing
```

---

## 117.3 Future Debezium Replacement

```text
Current:
  PostgreSQL
     |
  Polling Publisher
     |
  Kafka

Future:
  PostgreSQL WAL
     |
  Debezium
     |
  Kafka
```

Debezium eliminates polling overhead but adds operational complexity.

It should only be introduced when justified.

---

# 118. gRPC Contracts

## 118.1 driver.proto

```proto
syntax = "proto3";

package driver;

service DriverService {
  rpc FindAvailableDriver(FindAvailableDriverRequest)
    returns (FindAvailableDriverResponse);

  rpc AssignDriver(AssignDriverRequest)
    returns (AssignDriverResponse);

  rpc ReleaseDriver(ReleaseDriverRequest)
    returns (ReleaseDriverResponse);

  rpc GetDriver(GetDriverRequest)
    returns (GetDriverResponse);

  rpc GetDriverStatus(GetDriverStatusRequest)
    returns (GetDriverStatusResponse);

  rpc GetActiveAssignment(GetActiveAssignmentRequest)
    returns (GetActiveAssignmentResponse);
}

message FindAvailableDriverRequest {
  double latitude = 1;
  double longitude = 2;
  double radiusKm = 3;
  string vehicleType = 4;
}

message FindAvailableDriverResponse {
  repeated DriverCandidate candidates = 1;
}

message DriverCandidate {
  string driverId = 1;
  double distanceKm = 2;
  string vehicleType = 3;
}

message AssignDriverRequest {
  string driverId = 1;
  string deliveryId = 2;
  string idempotencyKey = 3;
}

message AssignDriverResponse {
  bool success = 1;
  string assignmentId = 2;
  string reason = 3;
}

message ReleaseDriverRequest {
  string driverId = 1;
  string deliveryId = 2;
}

message ReleaseDriverResponse {
  bool success = 1;
}

message GetDriverRequest {
  string driverId = 1;
}

message GetDriverResponse {
  Driver driver = 1;
}

message Driver {
  string driverId = 1;
  string status = 2;
  string vehicleType = 3;
  string vehicleNumber = 4;
}

message GetDriverStatusRequest {
  string driverId = 1;
}

message GetDriverStatusResponse {
  string status = 1;
}

message GetActiveAssignmentRequest {
  string driverId = 1;
}

message GetActiveAssignmentResponse {
  string deliveryId = 1;
  string assignmentId = 2;
}
```

---

## 118.2 payment.proto

```proto
syntax = "proto3";

package payment;

service PaymentService {
  rpc CreatePayment(CreatePaymentRequest)
    returns (CreatePaymentResponse);

  rpc GetPayment(GetPaymentRequest)
    returns (GetPaymentResponse);

  rpc GetPaymentStatus(GetPaymentStatusRequest)
    returns (GetPaymentStatusResponse);

  rpc RefundPayment(RefundPaymentRequest)
    returns (RefundPaymentResponse);
}

message CreatePaymentRequest {
  string deliveryId = 1;
  string customerId = 2;
  double amount = 3;
  string currency = 4;
  string idempotencyKey = 5;
}

message CreatePaymentResponse {
  bool success = 1;
  string paymentId = 2;
  string status = 3;
  string reason = 4;
}

message GetPaymentRequest {
  string paymentId = 1;
}

message GetPaymentResponse {
  Payment payment = 1;
}

message Payment {
  string paymentId = 1;
  string deliveryId = 2;
  double amount = 3;
  string currency = 4;
  string status = 5;
  string createdAt = 6;
}

message GetPaymentStatusRequest {
  string paymentId = 1;
}

message GetPaymentStatusResponse {
  string status = 1;
}

message RefundPaymentRequest {
  string paymentId = 1;
  double amount = 2;
  string reason = 3;
  string idempotencyKey = 4;
}

message RefundPaymentResponse {
  bool success = 1;
  string refundId = 2;
  string reason = 3;
}
```

---

# 119. Development Phases — Detailed

## Phase 1: Repository & Foundation

```text
Deliverables:
  Monorepo setup
  Docker Compose with all infrastructure
  Base NestJS gateway stub
  Base Go service stubs
  PostgreSQL, MongoDB, Redis, Kafka, NATS, ClickHouse running
  Health check endpoints

Estimated Infrastructure:
  docker-compose.yml
  packages/nest-common (empty scaffold)
  packages/proto (empty scaffold)
  apps/graphql-gateway (stub)
  apps/auth-user (stub)
  apps/delivery (stub)
  apps/driver-dispatch (stub)
  apps/payment (stub)
  apps/notification (stub)
  apps/realtime (stub)
  apps/analytics (stub)
```

---

## Phase 2: Auth & User

```text
Deliverables:
  User entity (PostgreSQL)
  Password hashing (bcrypt)
  JWT generation and validation
  Refresh token lifecycle
  GraphQL mutations: register, login, logout, refreshToken
  Auth subgraph
  Role support: CUSTOMER, DRIVER, ADMIN
  Auth guard for protected operations
```

---

## Phase 3: Delivery Service

```text
Deliverables:
  Delivery entity (PostgreSQL)
  Delivery state machine
  CQRS commands and queries
  createDelivery mutation (with Outbox)
  cancelDelivery mutation
  Delivery GraphQL subgraph
  DeliveryStatusHistory
  OutboxEvent table
  Basic Outbox Publisher (polling)
```

---

## Phase 4: Driver & Dispatch

```text
Deliverables:
  Driver entity (MongoDB)
  Driver availability state machine
  Redis GEO integration (GEOADD, GEOSEARCH)
  Driver availability management
  gRPC server implementation
  FindAvailableDriver
  AssignDriver
  ReleaseDriver
  DriverAssignment entity
  Distributed Redis lock for assignment
```

---

## Phase 5: gRPC Integration

```text
Deliverables:
  driver.proto (finalized)
  payment.proto (finalized)
  NestJS gRPC client in Delivery Service
  Go gRPC server in Driver Service
  Go gRPC server in Payment Service
  End-to-end test: Delivery -> Driver gRPC call
```

---

## Phase 6: Payment Service

```text
Deliverables:
  Payment entity (PostgreSQL)
  Payment state machine
  Idempotency key handling (Redis)
  gRPC server implementation
  CreatePayment
  RefundPayment
  Mock payment provider integration
  Kafka producer: payment.completed, payment.failed
```

---

## Phase 7: Saga + Event-Driven Architecture

```text
Deliverables:
  Full Delivery Saga implementation
  Saga compensation logic
  Kafka consumers in Delivery Service:
    payment.completed
    payment.failed
    delivery.driver.accepted
    delivery.driver.rejected
  Outbox Publisher with distributed lock
  Kafka topics created
  Consumer groups configured
  DLQ configuration
  Idempotent consumers
```

---

## Phase 8: Notification Service

```text
Deliverables:
  Notification entity (PostgreSQL)
  BullMQ setup
  Kafka consumer group
  Email worker (mock provider)
  Push worker (mock provider)
  In-App worker
  Retry/backoff configuration
  DLQ for failed jobs
  Notification template mapping
  getNotifications query
```

---

## Phase 9: Realtime Service

```text
Deliverables:
  WebSocket gateway (NestJS)
  JWT authentication on connect
  Subscription management
  Location update handler
  NATS publisher/subscriber
  Redis connection tracking
  Cross-instance fan-out
  Driver location update flow
  Customer receives real-time updates
  Reconnection handling
```

---

## Phase 10: Analytics Service

```text
Deliverables:
  Kafka consumer (Go)
  ClickHouse tables
  Event-to-row transformer
  Batch insert
  Idempotent processing
  Key metrics available:
    Total deliveries
    Completion rate
    Average delivery time
    Driver acceptance rate
    Payment success rate
```

---

## Phase 11: Kubernetes

```text
Deliverables:
  k8s Deployment manifests for each service
  k8s Service manifests
  k8s Ingress
  ConfigMaps
  Secrets
  HPA for Realtime and Gateway
  Liveness and Readiness probes
  Resource requests and limits
```

---

## Phase 12: Skaffold

```text
Deliverables:
  skaffold.yaml
  Local Kubernetes development loop
  Automated build and deploy on code change
  Profile for Docker Compose vs Kubernetes
```

---

## Phase 13: Observability

```text
Deliverables:
  OpenTelemetry SDK in all services
  Structured logging (JSON)
  Correlation ID propagation
  Prometheus metrics
  Grafana dashboards
  Jaeger distributed tracing
  Trace context propagation across:
    GraphQL -> Delivery -> gRPC -> Driver
    Delivery -> Kafka -> Notification
```

---

## Phase 14: Reliability Hardening

```text
Deliverables:
  Circuit breaker on Delivery -> Driver gRPC
  Circuit breaker on Delivery -> Payment gRPC
  Retry policies configured per operation
  DLQ monitoring
  Graceful shutdown for all services
  Failure simulation tests
```

---

## Phase 15: Future GenAI Phase

```text
Deliverables (future only):
  FastAPI AI Service
  Qdrant vector database
  RAG pipeline
  AI Agent with tool calling
  LLM integration
  Streaming responses
  AI-specific GraphQL subgraph or REST endpoints
```

---

# 120. Architecture Rules for Agents

These rules must be followed by any agent working on this codebase.

```text
1.  Do not create REST API endpoints
2.  Do not add microservices beyond the 8 defined
3.  Do not implement GenAI in phases 1-14
4.  Do not introduce Qdrant until GenAI phase
5.  Do not introduce OpenSearch until explicitly required
6.  Do not introduce e-commerce features
7.  Do not allow cross-service database access
8.  Use GraphQL Federation for public API
9.  Use gRPC for synchronous service communication
10. Use NATS for low-latency transient messaging
11. Use Kafka for durable business events
12. Use WebSocket for browser realtime
13. Use BullMQ for background jobs
14. Use Redis for ephemeral and coordination
15. Use PostgreSQL for transactional data
16. Use MongoDB for Driver operational data
17. Use ClickHouse for analytics
18. Use Transactional Outbox for reliable Kafka publishing
19. Use Saga orchestration for distributed workflows
20. Use idempotency for all payment and critical operations
21. Use Redis GEO for driver proximity
22. Use distributed locks for driver assignment
23. Use CQRS logical separation in Delivery Service
24. Use DLQ for all Kafka consumer failure paths
25. JWT must be validated before WebSocket upgrade
26. Common NestJS package must not contain business logic
27. Services must remain independently deployable
28. Every operation must define timeout and failure behavior
29. Every gRPC contract must be defined in .proto files
30. Debezium is future only - do not introduce it initially
```

---

# 121. Quick Reference Card

## Communication

```text
Client -> Platform         : GraphQL
Service -> Service (sync)  : gRPC
Platform -> Browser        : WebSocket
Service -> Service (async) : NATS (transient) | Kafka (durable)
Background processing      : BullMQ
```

## Databases

```text
ACID transactions          : PostgreSQL
Flexible operational data  : MongoDB
High-speed ephemeral       : Redis
Driver proximity           : Redis GEO
Analytics                  : ClickHouse
Large files                : Object Storage
```

## Patterns

```text
Distributed workflow       : Saga (Orchestration)
Reliable event publishing  : Transactional Outbox
Delivery read/write split  : CQRS
Duplicate prevention       : Idempotency
Driver exclusivity         : Distributed Lock
Failed consumer handling   : Dead Letter Queue
```

## Infrastructure

```text
Containerization           : Docker
Local development          : Docker Compose
Production                 : Kubernetes
Local K8s dev              : Skaffold
```








Realtime Delivery Platform — Implementation Plan & Service Workflows
Architecture Verdict


Concern	Status
GraphQL Federation	✅ Correct
gRPC for sync communication	✅ Correct
Kafka for durable events	✅ Correct
NATS for low-latency transient	✅ Correct
WebSocket (not SSE) with justification	✅ Correct
Transactional Outbox	✅ Correct
Saga Orchestration	✅ Correct
CQRS for Delivery	✅ Added correctly
Redis GEO for driver proximity	✅ Added correctly
Distributed Lock for driver assignment	✅ Correct
DLQ strategy	✅ Added correctly
WebSocket JWT auth	✅ Added correctly
Capacity estimation	✅ Added correctly
Failure scenarios	✅ Added correctly
Debezium as future note	✅ Correctly deferred
Provider isolation (Payment, Notification)	✅ Added correctly
Implementation Plan: Service Workflows
Services to implement in order
Phase	Service	Language	Depends On
1	Repository & Docker Compose	—	—
2	Auth & User	NestJS	PostgreSQL, Redis
3	GraphQL Gateway	NestJS	Auth Service
4	Delivery Service	NestJS	PostgreSQL, Auth
5	Driver & Dispatch	Go	MongoDB, Redis GEO
6	Payment Service	Go	PostgreSQL
7	gRPC Contracts	Proto	Delivery, Driver, Payment
8	Saga + Outbox	NestJS	Delivery, Kafka
9	Notification	NestJS	Kafka, BullMQ, Redis
10	Realtime	NestJS	NATS, Redis, WebSocket
11	Analytics	Go	Kafka, ClickHouse
12	Kubernetes + Skaffold	K8s	All services
13	Observability	OTel	All services
Proposed Changes to system_artichiture.md
The document is already complete and correct. The only addition needed is a detailed Service Workflow section (Section 108 onwards) covering:

New Sections to Append
[MODIFY] 
system_artichiture.md
Append the following sections after Section 107:

Section 108 — Auth & User Service: Detailed Workflow
All mutations, methods, internal flows for authentication, registration, JWT, refresh tokens.

Section 109 — Delivery Service: Detailed Workflow
All CQRS command/query methods, state machine transitions, outbox steps, saga orchestration interface.

Section 110 — Driver & Dispatch Service: Detailed Workflow
Redis GEO operations, distributed lock acquire/release, gRPC server methods, NATS publisher methods, availability state machine.

Section 111 — Payment Service: Detailed Workflow
Idempotency check flow, payment state machine, gRPC server methods, Kafka producer, refund compensation flow.

Section 112 — Notification Service: Detailed Workflow
Kafka consumer setup, BullMQ job definitions per channel, retry/backoff/DLQ policy, template mapping.

Section 113 — Realtime Service: Detailed Workflow
WebSocket lifecycle (connect → auth → subscribe → receive → disconnect), NATS subscriber, Redis read, cross-instance fan-out pattern.

Section 114 — Analytics Service: Detailed Workflow
Kafka consumer setup, ClickHouse table schema, event-to-row mapping, aggregation queries.

Section 115 — GraphQL Gateway: Detailed Workflow
Federation setup, supergraph composition, auth middleware chain, rate limiting middleware, tracing middleware.

Section 116 — Cross-Service Saga: Complete Step-by-Step
Full delivery lifecycle from createDelivery mutation to delivery.completed event with all compensation paths.

Verification Plan
Automated Tests
Each service must have unit tests for business rules
Integration tests for gRPC contracts
Consumer tests for Kafka consumers
E2E test for full delivery lifecycle
Manual Verification
Launch full stack via Docker Compose
Create a delivery via GraphQL Playground
Observe Kafka events in a Kafka UI tool
Observe WebSocket messages in browser DevTools
Verify DLQ receives messages on simulated failure

---

# Section 117 — Implementation Status & Progress Tracker

This section maintains an up-to-date record of all components, packages, services, and infrastructure implemented in the codebase.

## 1. Shared TypeScript Package (`@delivery/common` -> `packages/ts`)
* **Package Scope**: Named as `@delivery/common` with `tsconfig.json` outputting compiled JavaScript & Declaration types to `dist/`.
* **Constants & Enums**: `Role` (`ADMIN`, `USER`), `PaymentMethod`, `PaymentStatus`, `HeaderKeys` (`x-user-id`, `x-user-role`, `x-correlation-id`, `x-user-session`).
* **GraphQL & DTOs**: Base `PaginationInput`, `User` GraphQL ObjectType definitions.
* **Decorators & Guards**:
  * `@Auth()` and `@CurrentUser()` context extractors.
  * `RoleGuard` supporting JWT decoding and role permissions mapping.
  * `RateLimiterGuard` integrated with `@bts-soft/validation` and `@bts-soft/cache` (Redis).
* **NATS Client Module**: Shared `NatsModule` & `NatsService` wrapper.

## 2. Shared Go Module (`github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go` -> `packages/go`)
* **Go Module Initialization**: Independent Go module created at `packages/go`.
* **Constants**: State machine status strings (`DeliveryStatusPending`, `DeliveryStatusDelivered`, etc.), Payment statuses, Header keys.
* **Events & Payloads**:
  * NATS Subjects (`driver.location.updated`, `driver.presence.updated`, etc.).
  * Kafka Topics (`delivery.created`, `payment.completed`, etc.).
  * Go struct definitions for JSON serialization/deserialization.
* **gRPC Interceptors (`middleware/`)**:
  * `UnaryServerMetadataInterceptor()` for automatic header context injection.
  * `AuthInterceptor()` for gRPC `codes.Unauthenticated` checks.
  * `RequireRoleInterceptor(...)` for gRPC `codes.PermissionDenied` role authorization checks.
  * Context getters (`GetUserID`, `GetUserRole`, `GetCorrelationID`).

## 3. GraphQL API Gateway (`services/api-gateway`)
* **NestJS App Structure**: Configured as GraphQL Federation Gateway with Shutdown Hooks enabled.
* **Rate Limiting & Redis**: Configured with `@bts-soft/cache` Redis store and environment variables (`REDIS_HOST`, `REDIS_PORT`, `RATE_LIMIT_LIMIT`, `RATE_LIMIT_WINDOW_MS`).
* **Dockerfile Optimization**: Updated build context to copy `packages/ts` before `npm install` for local dependency resolution inside Docker containers.

## 5. Kubernetes & Skaffold Infrastructure (`k8s/`, `skaffold.yaml`)
* **Skaffold Sync Rules**: Mapped `services/api-gateway/src/**/*.ts` and `protos/**/*.proto` for real-time Kubernetes hot-reloading.
* **K8s Deployments**: Configured `api-gateway-depl.yaml`, `redis-depl.yaml`, `user-secrets.yaml`, and secrets management.
