# Driver & Dispatch Service — System Architecture Specification

**Project:** Realtime Delivery Platform  
**Service:** Driver & Dispatch Service  
**Primary Language:** Go  
**API Exposure:** GraphQL Federation through the API Gateway  
**Internal Synchronous Communication:** gRPC  
**Low-Latency Messaging:** NATS  
**Durable Business Events:** Kafka  
**Operational Database:** MongoDB  
**Ephemeral State / Coordination:** Redis  
**Geospatial Index:** Redis GEO  
**Background Jobs:** Go workers / BullMQ only where a Node-owned queue is appropriate  
**Realtime Transport:** WebSocket is owned by Realtime Service, not this service  
**Patterns:** State Machines, CQRS-style command/query separation, Idempotency, Distributed Locking, Outbox/Event Integration, Retry, Backoff, Jitter, DLQ, Reconciliation  
**Infrastructure:** Docker, Docker Compose, Kubernetes, Skaffold  
**Observability:** OpenTelemetry, Prometheus, Grafana, Jaeger, Structured Logging  
**Initial Scope:** Driver profiles, availability, location, dispatch, assignment, acceptance/rejection, operational state  
**Future Scope:** Advanced dispatch optimization, ETA prediction, route optimization, ML/GenAI assistance

---

# 1. Executive Summary

The Driver & Dispatch Service is the operational service responsible for managing drivers and deciding which available driver should receive a delivery assignment.

It is deliberately implemented in **Go** because the service is a good place to demonstrate high-concurrency backend engineering, low-latency processing, goroutines, worker pools, gRPC, Redis GEO, distributed locking, and efficient network services.

The service does **not** own the Delivery aggregate. The Delivery Service remains the business owner of the delivery lifecycle and the Saga orchestrator. The Driver & Dispatch Service owns driver operational state and assignment state.

The service must be designed around the following rule:

> A driver can be assigned to at most one active delivery at a time.

This rule must remain true even when multiple delivery requests, service replicas, retries, duplicate messages, or concurrent assignment attempts occur.

The primary mechanisms used to enforce this are:

- MongoDB for durable driver/assignment operational data
- Redis GEO for fast proximity lookup
- Redis distributed locks for short critical sections
- MongoDB atomic/conditional updates for durable state transitions
- Idempotency keys for duplicate commands/events
- gRPC for synchronous commands from Delivery Service
- NATS for transient low-latency notifications and realtime coordination
- Kafka for durable domain events that other services must consume

---

# 2. Overall Project Architecture Review

## 2.1 Current Service Set

At the current implementation stage, the platform contains these application services/components:

| # | Component | Language | Primary Responsibility | Primary Data |
|---|---|---|---|---|
| 1 | API Gateway | NestJS | GraphQL Federation, auth enforcement, rate limiting, routing | None |
| 2 | User Service | NestJS | Identity/profile/user data | PostgreSQL |
| 3 | Delivery Service | NestJS | Delivery lifecycle and Saga orchestration | PostgreSQL |
| 4 | Driver & Dispatch Service | Go | Drivers, availability, proximity, assignment | MongoDB + Redis GEO |
| 5 | Payment Service | Go | Payments and refunds | PostgreSQL |
| 6 | Notification Service | NestJS | Push/email/in-app notifications | PostgreSQL + Redis/BullMQ |
| 7 | Realtime Service | NestJS | WebSocket connections and realtime fan-out | Redis + NATS |
| 8 | Media Service | Go | Uploads, object storage, media processing | DynamoDB + Redis + S3-compatible storage |
| 9 | Search Service | Go | Search projection and querying | OpenSearch |
| 10 | Analytics Service | Go/FastAPI future decision | Event analytics | ClickHouse |

For the current implementation, **Payment and Analytics can remain future/next-phase services if they are not yet implemented**. Do not force them into Driver & Dispatch just to increase technology count.

The architecture remains manageable because Driver & Dispatch combines driver management and dispatch into one bounded context instead of creating separate Driver, Location, Matching, and Dispatch microservices.

---

# 3. Overall Architecture

```text
                                      CLIENTS
                                         |
                                         | GraphQL
                                         v
                              +------------------------+
                              |     API GATEWAY        |
                              | NestJS + Federation    |
                              +-----------+------------+
                                          |
                  +-----------------------+-----------------------+
                  |                       |                       |
                  v                       v                       v
             USER SERVICE          DELIVERY SERVICE       DRIVER & DISPATCH
               NestJS                 NestJS                    Go
                  |                       |                       |
             PostgreSQL             PostgreSQL             MongoDB + Redis
                                          |                       |
                                          | gRPC                  |
                                          +---------------------->|
                                                                  |
                                          +-----------------------+
                                          |
                              +-----------+-----------+
                              |                       |
                            Kafka                    NATS
                              |                       |
              +---------------+--------+              v
              |               |        |        REALTIME SERVICE
        Notification       Search   Analytics          |
        Service            Service   Service            |
                                                        v
                                                   WebSocket
                                                        |
                                                     CLIENT

       MEDIA SERVICE
       Go + DynamoDB + Redis + Object Storage
       (files, images, videos, proof media)

       PAYMENT SERVICE
       Go + PostgreSQL
       (future/next core phase)
```

---

# 4. Bounded Context

The Driver & Dispatch bounded context contains:

```text
Driver
DriverProfile
DriverAvailability
DriverLocation
DriverAssignment
DispatchAttempt
DispatchPolicy
OperationalDriverState
```

It does not contain:

```text
Delivery lifecycle ownership
Payment records
User authentication
Notification templates
WebSocket connections
Media objects
Search indexes
Analytics facts
```

A driver ID may be referenced by other services, but the driver service remains the owner of driver operational data.

---

# 5. Core Responsibilities

## 5.1 Driver Management

The service manages:

- Driver registration from an authorized workflow
- Driver profile operational information
- Driver status
- Vehicle information required for dispatch
- Service area/capabilities
- Driver activation/deactivation
- Driver availability

## 5.2 Availability

The operational availability states are:

```text
OFFLINE
   |
   | goOnline
   v
AVAILABLE
   |
   | assignment reserved
   v
BUSY
   |
   | delivery completed/released
   v
AVAILABLE
```

Additional operational states may be represented internally:

```text
SUSPENDED
BLOCKED
MAINTENANCE
```

These should not be added to the public state machine unless required by the business domain.

## 5.3 Location

The service stores the driver's latest location for dispatch purposes.

Redis GEO is the fast lookup layer:

```text
GEOADD drivers:locations <longitude> <latitude> <driverId>
```

MongoDB stores durable operational information when required, but Redis remains the preferred hot path for high-frequency location proximity queries.

---

# 6. What Driver & Dispatch Does NOT Do

The service must not:

- Create delivery business records
- Change payment state
- Send email directly
- Own WebSocket connections
- Write to another service's database
- Write directly to OpenSearch
- Write directly to ClickHouse
- Upload files to object storage on behalf of normal delivery flow
- Become the Saga orchestrator
- Replace the Delivery Service
- Implement REST APIs

The Delivery Service owns the delivery Saga. The Driver & Dispatch Service is a participant in that Saga.

---

# 7. Communication Architecture

## 7.1 Client -> Gateway

```text
Client
  |
  | GraphQL
  v
API Gateway
  |
  | Federation
  v
Driver Subgraph
```

The client never directly calls the Go service's HTTP endpoint.

## 7.2 Delivery -> Driver/Dispatch

For commands that need an immediate answer:

```text
Delivery Service
      |
      | gRPC
      v
Driver & Dispatch Service
      |
      v
MongoDB / Redis
```

Examples:

```text
FindAvailableDrivers
ReserveDriver
AssignDriver
ReleaseDriver
GetDriverStatus
```

## 7.3 Driver/Dispatch -> Realtime

```text
Driver & Dispatch
       |
       | NATS
       v
Realtime Service
       |
       | WebSocket
       v
Driver / Customer / Admin
```

NATS is appropriate here because realtime messages are transient. Losing an old location update is generally acceptable because a newer location supersedes it.

## 7.4 Durable Business Events

```text
Driver & Dispatch
       |
       | Kafka
       v
Kafka Topic
       |
       +--> Notification
       +--> Analytics
       +--> Search where appropriate
       +--> Future consumers
```

Kafka is the durable event platform. NATS must not become a second Kafka.

---

# 8. Protocol Decision Matrix

| Requirement | Technology | Reason |
|---|---|---|
| Client API | GraphQL Federation | Unified client contract |
| Driver service sync | gRPC | Strong contracts + low latency |
| Durable events | Kafka | Replay, durability, consumer groups |
| Realtime transient events | NATS | Very low latency |
| Browser realtime | WebSocket | Bidirectional driver communication |
| Hot location lookup | Redis GEO | Fast proximity search |
| Distributed locking | Redis | Short-lived coordination |
| Durable driver state | MongoDB | Operational document model |
| Caching | Redis | Fast ephemeral access |
| Background reconciliation | Go worker/cron | Native to service |
| Metrics | Prometheus | Operational monitoring |
| Tracing | OpenTelemetry | Distributed tracing |

---

# 9. Driver State Machine

```text
                         +-------------+
                         |   OFFLINE   |
                         +------+------+ 
                                |
                             GoOnline
                                |
                                v
                         +-------------+
                         | AVAILABLE   |
                         +------+------+ 
                           |          |
                    Reserve/Assign  GoOffline
                           |          |
                           v          v
                     +-----+----+  +--+-------+
                     |   BUSY   |  |  OFFLINE |
                     +-----+----+  +----------+
                           |
                    Delivery Finished
                           |
                           v
                     +-------------+
                     | AVAILABLE   |
                     +-------------+
```

## 9.1 Transition Rules

| From | Action | To |
|---|---|---|
| OFFLINE | GoOnline | AVAILABLE |
| AVAILABLE | Reserve | BUSY/RESERVED depending on implementation |
| AVAILABLE | GoOffline | OFFLINE |
| BUSY | DeliveryCompleted | AVAILABLE |
| BUSY | GoOffline request | OFFLINE_AFTER_ACTIVE_DELIVERY / controlled transition |

No transition should be accepted without authorization and a valid current state.

---

# 10. Assignment State Machine

Assignment state should be separate from driver availability.

```text
NONE
 |
 | create assignment
 v
OFFERED
 |
 +--------------------+
 |                    |
Accept               Reject
 |                    |
 v                    v
ACCEPTED          REJECTED
 |
 | delivery starts
 v
ACTIVE
 |
 | delivery completed
 v
COMPLETED
```

Timeout path:

```text
OFFERED
   |
   | timeout
   v
EXPIRED
   |
   v
NEXT CANDIDATE
```

Cancellation path:

```text
OFFERED / ACCEPTED / ACTIVE
          |
          | cancel/release
          v
       CANCELLED
```

The exact allowed transitions must be enforced centrally in the domain layer.

---

# 11. Dispatch Algorithm

The first implementation should intentionally be simple and deterministic.

```text
1. Receive delivery assignment request.
2. Read pickup coordinates.
3. Search nearby drivers using Redis GEO.
4. Filter by AVAILABLE state.
5. Exclude drivers already attempted for this delivery.
6. Rank candidates.
7. Attempt atomic reservation.
8. If reservation succeeds, create/send assignment offer.
9. Notify driver through NATS -> Realtime -> WebSocket.
10. Wait for accept/reject/timeout.
11. On accept, confirm assignment.
12. On reject/timeout, release and try next candidate.
13. If no candidate remains, return dispatch failure.
```

---

# 12. Candidate Ranking

Initial ranking can be:

```text
score = distanceWeight * normalizedDistance
      + availabilityWeight * availabilityScore
      + vehicleCompatibilityWeight * compatibilityScore
```

For the first implementation, avoid complex ML ranking.

A simple strategy is enough:

```text
Nearest AVAILABLE compatible driver first
```

Future enhancements may include:

- Driver acceptance rate
- Historical completion rate
- Estimated pickup time
- Driver workload
- Traffic conditions
- Vehicle capability
- Dynamic dispatch score
- ML prediction

---

# 13. Redis GEO Design

## 13.1 Key

```text
drivers:locations
```

Example:

```text
GEOADD drivers:locations 31.245 31.416 driver-123
```

## 13.2 Query

Use GEOSEARCH around pickup coordinates.

Conceptually:

```text
GEOSEARCH drivers:locations
  FROMLONLAT <lng> <lat>
  BYRADIUS 5 km
  ASC
  WITHDIST
```

## 13.3 Location Metadata

A separate key can hold the latest operational location:

```text
SET driver:location:{driverId}
```

Suggested value:

```json
{
  "lat": 31.416,
  "lng": 31.245,
  "accuracy": 8.2,
  "heading": 120,
  "speed": 14.5,
  "timestamp": "2026-09-02T12:00:00Z"
}
```

## 13.4 TTL

Location metadata should expire when the driver stops sending heartbeats.

Do not interpret an expired Redis location as a durable driver status change automatically without a defined reconciliation policy.

---

# 14. Driver Location Flow

```text
Driver App
    |
    | WebSocket location update
    v
Realtime Service
    |
    | NATS: driver.location.updated
    v
Driver & Dispatch Service
    |
    +--> Validate driver context
    |
    +--> Validate timestamp / coordinates
    |
    +--> GEOADD drivers:locations
    |
    +--> SET driver:location:{driverId}
    |
    +--> Update durable location only when business rules require it
```

Location updates should not be written to MongoDB for every GPS tick unless there is a concrete requirement. High-frequency telemetry belongs in the hot path.

---

# 15. Location Validation

Reject or ignore invalid updates such as:

- Missing driver ID
- Invalid latitude
- Invalid longitude
- Impossible coordinates
- Old/stale timestamp
- Excessively large timestamp jump
- Invalid speed
- Invalid accuracy
- Unauthorized driver identity

Optional sanity checks:

```text
max location age
max speed threshold
minimum update interval
maximum coordinate jump
```

These limits should be configurable.

---

# 16. Distributed Locking

The most important concurrency problem is:

```text
Delivery A -> Driver X
Delivery B -> Driver X
```

Both requests may see:

```text
Driver X = AVAILABLE
```

without synchronization.

Therefore the assignment critical section uses a distributed lock.

Suggested key:

```text
lock:driver:{driverId}
```

The lock must have:

- Short TTL
- Unique owner token
- Safe release
- Timeout
- No indefinite blocking

---

# 17. Correct Assignment Critical Section

```text
Find Candidate
      |
      v
Acquire lock:driver:{id}
      |
      +---- failed --> try next candidate
      |
      v
Read current driver state
      |
      +---- not AVAILABLE --> release + try next
      |
      v
Conditional MongoDB update
AVAILABLE -> BUSY
      |
      v
Create assignment record
      |
      v
Commit
      |
      v
Release lock
```

The lock protects the short critical section. It must not be held while waiting for the driver to respond.

---

# 18. Important Lock Rule

Never do this:

```text
Acquire lock
   |
   | wait 30 seconds for driver
   |
Release lock
```

Instead:

```text
Acquire lock
   |
Reserve driver
   |
Release lock
   |
Create OFFERED assignment
   |
Wait for driver response without holding lock
```

This prevents lock contention and makes horizontal scaling possible.

---

# 19. MongoDB Data Model

MongoDB is the primary operational database for this service.

Recommended collections:

```text
drivers
driver_assignments
dispatch_attempts
idempotency_records
```

An outbox collection is optional if this service directly guarantees durable Kafka publication through an outbox. If the implementation uses Kafka transactions or another reliable event mechanism, document that decision explicitly. Do not introduce two competing reliability mechanisms unnecessarily.

---

# 20. drivers Collection

Example document:

```json
{
  "_id": "driver-123",
  "userId": "user-456",
  "status": "AVAILABLE",
  "vehicle": {
    "type": "CAR",
    "plateNumber": "ABC-123",
    "capacityKg": 50
  },
  "capabilities": ["STANDARD"],
  "serviceArea": "DAMietta",
  "version": 17,
  "createdAt": "2026-09-02T10:00:00Z",
  "updatedAt": "2026-09-02T12:00:00Z"
}
```

Never store another service's full user record here.

---

# 21. driver_assignments Collection

Example:

```json
{
  "_id": "assignment-123",
  "deliveryId": "delivery-456",
  "driverId": "driver-123",
  "status": "OFFERED",
  "attemptNumber": 1,
  "offeredAt": "2026-09-02T12:01:00Z",
  "expiresAt": "2026-09-02T12:01:20Z",
  "acceptedAt": null,
  "rejectedAt": null,
  "completedAt": null,
  "createdAt": "2026-09-02T12:01:00Z",
  "updatedAt": "2026-09-02T12:01:00Z"
}
```

Recommended indexes:

```text
deliveryId + status
driverId + status
expiresAt
createdAt
```

---

# 22. dispatch_attempts Collection

This records why a candidate was or was not selected.

Example:

```json
{
  "_id": "attempt-123",
  "deliveryId": "delivery-456",
  "driverId": "driver-123",
  "distanceMeters": 850,
  "attemptNumber": 1,
  "result": "REJECTED",
  "reason": "DRIVER_REJECTED",
  "createdAt": "2026-09-02T12:01:00Z"
}
```

This becomes valuable for operational debugging and future dispatch optimization.

---

# 23. Idempotency

The service must assume:

```text
same command may arrive twice
same Kafka event may arrive twice
same gRPC request may be retried
client may reconnect
service may crash after writing but before responding
```

Critical operations requiring idempotency:

```text
GoOnline
GoOffline
ReserveDriver
AssignDriver
AcceptAssignment
RejectAssignment
ReleaseDriver
CompleteAssignment
```

Use an idempotency key such as:

```text
{operation}:{deliveryId}:{driverId}:{requestId}
```

For durable business commands, persist the idempotency result where necessary. Redis can provide a fast layer, but Redis must not become the source of truth for critical business state.

---

# 24. MongoDB Conditional Updates

Redis locking alone is not enough.

The durable state transition should also be conditional.

Conceptually:

```text
UPDATE driver
WHERE _id = driverId
AND status = AVAILABLE
SET status = BUSY
```

MongoDB's atomic update semantics should be used to prevent stale-state writes.

This provides defense in depth:

```text
Redis lock
    +
MongoDB conditional update
    +
Idempotency
```

---

# 25. Driver Reservation

Reservation is different from assignment acceptance.

```text
AVAILABLE
   |
   | reserve
   v
BUSY
   |
   | offer assignment
   v
OFFERED
```

The driver is temporarily reserved so another delivery cannot simultaneously select the same driver.

If the driver rejects or the offer expires:

```text
OFFERED
   |
   v
release reservation
   |
   v
AVAILABLE
```

---

# 26. Driver Assignment Flow

```text
Delivery Service
      |
      | gRPC FindAvailableDrivers
      v
Driver & Dispatch
      |
      v
Redis GEOSEARCH
      |
      v
Candidate Drivers
      |
      v
Filter AVAILABLE
      |
      v
Rank candidates
      |
      v
Acquire driver lock
      |
      v
Conditional state transition
      |
      v
Create assignment OFFERED
      |
      v
Publish NATS assignment event
      |
      v
Realtime Service
      |
      v
Driver WebSocket
```

---

# 27. Driver Accept Flow

```text
Driver
  |
  | WebSocket
  v
Realtime Service
  |
  | NATS
  v
Driver & Dispatch
  |
  +--> Validate assignment
  +--> Validate driver identity
  +--> Idempotency check
  +--> Conditional OFFERED -> ACCEPTED
  +--> Persist acceptedAt
  +--> Publish durable event
  |
  v
Delivery Service
```

The Delivery Service can then advance its Saga.

---

# 28. Driver Reject Flow

```text
Driver
  |
  | reject assignment
  v
Realtime
  |
  | NATS
  v
Driver & Dispatch
  |
  +--> Conditional OFFERED -> REJECTED
  +--> Release driver
  +--> Record dispatch attempt
  +--> Publish driver.assignment.rejected
  |
  v
Delivery Service
  |
  +--> Try next driver
  +--> Or fail/cancel dispatch
```

---

# 29. Assignment Timeout

A background worker checks expired offers.

```text
OFFERED
   |
   | expiresAt < now
   v
EXPIRED
   |
   +--> Release driver
   +--> Record attempt
   +--> Publish assignment.expired
   +--> Notify Delivery Service
```

Expiration must be idempotent. Two workers may discover the same expired assignment.

Only one should win the conditional transition:

```text
OFFERED -> EXPIRED
```

---

# 30. gRPC Contract

The service exposes internal gRPC contracts through `.proto` files.

Suggested package:

```text
proto/driver/v1/driver.proto
```

Conceptual contract:

```proto
service DriverService {
  rpc FindAvailableDrivers(FindAvailableDriversRequest)
      returns (FindAvailableDriversResponse);

  rpc ReserveDriver(ReserveDriverRequest)
      returns (ReserveDriverResponse);

  rpc ReleaseDriver(ReleaseDriverRequest)
      returns (ReleaseDriverResponse);

  rpc GetDriver(GetDriverRequest)
      returns (GetDriverResponse);

  rpc GetDriverStatus(GetDriverStatusRequest)
      returns (GetDriverStatusResponse);
}
```

The exact generated Go implementation belongs in the service. NestJS consumes the generated contract from the same versioned proto definition.

---

# 31. gRPC Design Rules

Every gRPC call must define:

- Deadline
- Timeout
- Error mapping
- Request ID
- Correlation ID
- Authentication/service identity
- Idempotency where applicable
- Retry policy

Do not blindly retry every gRPC error.

Retry only transient failures.

---

# 32. gRPC Error Classification

```text
INVALID_ARGUMENT
UNAUTHENTICATED
PERMISSION_DENIED
NOT_FOUND
ALREADY_EXISTS
FAILED_PRECONDITION
RESOURCE_EXHAUSTED
DEADLINE_EXCEEDED
UNAVAILABLE
INTERNAL
```

Examples:

```text
Driver unavailable -> FAILED_PRECONDITION
Driver not found -> NOT_FOUND
Invalid coordinates -> INVALID_ARGUMENT
Service timeout -> DEADLINE_EXCEEDED
Temporary downstream outage -> UNAVAILABLE
```

---

# 33. Kafka Events

Durable events produced by Driver & Dispatch may include:

```text
driver.created
driver.activated
driver.deactivated
driver.available
driver.unavailable
driver.assignment.offered
driver.assignment.accepted
driver.assignment.rejected
driver.assignment.expired
driver.assignment.released
driver.location.snapshot.updated   (only if durable telemetry is actually required)
```

Do not publish high-frequency GPS events to Kafka by default. Use NATS for realtime location propagation.

---

# 34. Kafka Event Envelope

All durable events should follow the platform envelope:

```json
{
  "eventId": "uuid",
  "eventType": "driver.assignment.accepted",
  "version": 1,
  "aggregateId": "assignment-123",
  "aggregateType": "DriverAssignment",
  "producer": "driver-dispatch-service",
  "correlationId": "uuid",
  "causationId": "uuid",
  "occurredAt": "timestamp",
  "payload": {}
}
```

Events represent facts that already happened.

Good:

```text
driver.assignment.accepted
```

Avoid event names that represent commands:

```text
assign.driver
```

Commands belong in gRPC/command paths; facts belong in Kafka events.

---

# 35. Kafka Consumer Responsibilities

Driver & Dispatch should consume only events that it actually needs.

Potential consumers:

```text
delivery.cancelled
 delivery.completed
 delivery.failed
```

For example:

```text
delivery.completed
      |
      v
Driver & Dispatch
      |
      +--> Find active assignment
      +--> Mark assignment COMPLETED
      +--> Release driver
      +--> Publish driver.available
```

Do not consume the entire Kafka topic merely because it exists.

---

# 36. Kafka Consumer Idempotency

Every Kafka consumer must tolerate duplicate events.

Example:

```text
delivery.completed
        |
        +--> first delivery: COMPLETED -> release driver
        |
        +--> duplicate: already COMPLETED -> no-op
```

Use a durable processed-event/idempotency record when the operation cannot safely be made naturally idempotent.

---

# 37. NATS Subjects

Suggested transient subjects:

```text
driver.location.updated
driver.assignment.offered
driver.assignment.cancelled
driver.assignment.updated
driver.status.updated
```

Example:

```text
Realtime Service
      |
      | driver.location.updated
      v
Driver & Dispatch
```

And:

```text
Driver & Dispatch
      |
      | driver.assignment.offered
      v
Realtime Service
```

---

# 38. NATS vs Kafka Decision

Use NATS when:

- Message is transient
- Latest state supersedes older state
- Very low latency matters
- Replay is not required
- Browser realtime fan-out is involved

Use Kafka when:

- Event represents a durable business fact
- Consumers need replay
- Analytics needs historical events
- Multiple independent consumer groups exist
- Event must survive temporary consumer downtime

Example:

```text
Driver GPS update       -> NATS
Driver accepted         -> Kafka
Driver rejected         -> Kafka
Delivery completed      -> Kafka
WebSocket fan-out       -> NATS
```

---

# 39. NATS JetStream Decision

JetStream should **not** be introduced simply because it is available.

The platform already uses Kafka for durable business events.

Therefore:

```text
NATS Core -> transient realtime/internal messages
Kafka     -> durable business events
```

JetStream may be evaluated later for a narrowly scoped operational workflow where NATS-native persistence provides a real benefit. It must not become a duplicate Kafka without justification.

---

# 40. WebSocket Boundary

The Driver & Dispatch Service does not own browser WebSocket connections.

The correct architecture is:

```text
Driver App
    |
    | WebSocket
    v
Realtime Service
    |
    | NATS
    v
Driver & Dispatch
```

For outgoing assignment:

```text
Driver & Dispatch
    |
    | NATS
    v
Realtime Service
    |
    | WebSocket
    v
Driver App
```

This keeps connection management separate from dispatch business logic.

---

# 41. WebSocket Authentication Boundary

JWT validation belongs at the Realtime Service before accepting the WebSocket session.

The Driver & Dispatch service trusts the authenticated internal identity propagated through NATS/gRPC according to the platform's service authentication model.

Do not duplicate browser authentication logic inside every service.

---

# 42. Realtime Location Fan-Out

```text
Driver
  |
  | location
  v
Realtime Service
  |
  | NATS
  v
Driver & Dispatch
  |
  +--> update Redis GEO
  |
  +--> optional operational processing
```

For customer tracking:

```text
Driver
  |
  v
Realtime
  |
  v
NATS
  |
  v
Customer WebSocket
```

The customer does not need to wait for Driver & Dispatch to persist every location update.

---

# 43. Redis Keys

Suggested key catalogue:

```text
drivers:locations

driver:location:{driverId}

driver:status:{driverId}

driver:lock:{driverId}

driver:assignment:{driverId}

delivery:dispatch:{deliveryId}

dispatch:attempts:{deliveryId}
idempotency:driver:{operation}:{key}
```

Use namespaces consistently.

---

# 44. Redis Is Not the Source of Truth

Redis stores:

- Latest location
- Locks
- Cache
- Temporary assignment state
- Idempotency acceleration
- Rate limiting if needed

MongoDB stores durable operational state.

If Redis disappears:

```text
Driver profiles -> survive
Assignments      -> survive
Driver state     -> survive
Latest hot GPS   -> may be temporarily unavailable
Locks            -> disappear and can be reacquired
```

The service must recover safely.

---

# 45. Dispatch Concurrency

Consider:

```text
Delivery A -> Candidate Driver 1
Delivery B -> Candidate Driver 1
Delivery C -> Candidate Driver 1
```

All three may query Redis GEO simultaneously.

That is fine.

The critical guarantee happens during reservation:

```text
A -> lock -> AVAILABLE -> BUSY -> success
B -> lock fails / state BUSY -> next candidate
C -> state BUSY -> next candidate
```

This is why proximity lookup and reservation must be treated as separate stages.

---

# 46. Multi-Instance Scaling

```text
                 Load / gRPC
                     |
        +------------+------------+
        |            |            |
        v            v            v
     Driver-1     Driver-2     Driver-3
        |            |            |
        +------------+------------+
                     |
              Shared Redis
                     |
                Redis GEO
                     |
               Shared MongoDB
```

Any instance may process an assignment.

Distributed locks ensure that instances do not assign the same driver concurrently.

---

# 47. Worker Pool Design

Go is particularly useful for dispatch background work.

Example worker responsibilities:

```text
Assignment expiration
Driver heartbeat reconciliation
Stale location cleanup
Retry dispatch attempts
Periodic availability reconciliation
Operational cleanup
```

Conceptually:

```text
                 Job Queue
                    |
        +-----------+-----------+
        |           |           |
      Worker      Worker      Worker
        |           |           |
        +-----------+-----------+
                    |
                 MongoDB
                 Redis
```

Do not create an enormous worker pool. Size it based on measured workload.

---

# 48. BullMQ Decision

BullMQ is already part of the overall platform for Node/NestJS-owned background jobs, especially Notification Service.

The Go Driver & Dispatch Service should not introduce BullMQ merely for consistency because BullMQ is a Node ecosystem queue.

Use native Go workers/cron for Driver & Dispatch operational jobs.

Use BullMQ where the job is actually owned by a NestJS service.

This keeps technology boundaries clean.

---

# 49. Retry Policy

Retry only transient failures.

Typical retryable failures:

```text
MongoDB temporary network failure
Redis temporary timeout
NATS temporary connection issue
Kafka temporary unavailable
Transient gRPC UNAVAILABLE
```

Do not retry:

```text
invalid coordinates
permission denied
invalid driver state
assignment rejected by driver
unknown driver
```

---

# 50. Exponential Backoff + Jitter

Example:

```text
attempt 1 -> 100ms + jitter
attempt 2 -> 200ms + jitter
attempt 3 -> 400ms + jitter
attempt 4 -> 800ms + jitter
attempt 5 -> 1600ms + jitter
```

Jitter prevents many replicas from retrying simultaneously and creating a retry storm.

---

# 51. Dead Letter Queue

Kafka consumers must have a defined DLQ strategy.

Example:

```text
delivery.completed
       |
       v
Driver Consumer
       |
       +--> success
       |
       +--> retry
       |
       +--> max retries
                 |
                 v
       delivery.completed.driver-dlq
```

DLQ records should contain:

```text
eventId
eventType
consumerGroup
originalTopic
partition
offset
attemptCount
error
failedAt
payload
correlationId
```

DLQ messages should be inspectable and replayable after fixing the problem.

---

# 52. Reconciliation

Distributed systems can become inconsistent even when individual operations are correct.

The service should periodically inspect:

```text
Drivers marked BUSY without active assignment
Assignments OFFERED past expiresAt
Assignments ACTIVE without valid delivery state
Redis GEO entries for OFFLINE drivers
Missing driver location heartbeats
Stale locks
```

Example:

```text
Reconciliation Worker
        |
        +--> MongoDB
        +--> Redis
        |
        v
Repair safe inconsistencies
```

Reconciliation must be conservative. It should never blindly overwrite state without verifying the source of truth.

---

# 53. Driver Heartbeat

A driver should periodically send a heartbeat.

Example:

```text
heartbeat every 10 seconds
stale after 30 seconds
```

If the heartbeat becomes stale:

```text
AVAILABLE
   |
   | no heartbeat
   v
temporarily unavailable
```

The exact transition should be configurable.

Do not mark a driver OFFLINE based on one missed packet.

---

# 54. Dispatch Timeout

Each assignment offer should have a bounded response window.

Example:

```text
OFFERED
  |
  | 20 seconds
  v
EXPIRED
```

The timeout should be configurable.

When an offer expires:

```text
conditional transition
release driver
record attempt
try next candidate
```

---

# 55. Full Dispatch Flow

```text
STEP 1
Delivery Service decides that a driver is required.

STEP 2
Delivery Service calls FindAvailableDrivers via gRPC.

STEP 3
Driver & Dispatch executes Redis GEOSEARCH.

STEP 4
Candidates are filtered by operational state.

STEP 5
Candidates are ranked by distance and compatibility.

STEP 6
Driver & Dispatch attempts reservation.

STEP 7
Redis lock is acquired for the candidate.

STEP 8
MongoDB conditionally changes AVAILABLE -> BUSY.

STEP 9
Assignment record is created as OFFERED.

STEP 10
Lock is released.

STEP 11
NATS event is published to Realtime Service.

STEP 12
Realtime Service sends WebSocket message to driver.

STEP 13
Driver accepts or rejects.

STEP 14
Realtime sends result through NATS.

STEP 15
Driver & Dispatch conditionally updates assignment.

STEP 16
Kafka publishes durable assignment fact.

STEP 17
Delivery Service receives event and advances/compensates its Saga.
```

---

# 56. Failure During Reservation

If MongoDB fails after Redis lock acquisition:

```text
Acquire lock
   |
MongoDB unavailable
   |
Release lock
   |
Return transient error
```

Never leave a lock indefinitely.

If the process crashes while holding the lock, TTL allows recovery.

---

# 57. Failure After Driver Becomes BUSY

Potential failure:

```text
MongoDB: AVAILABLE -> BUSY
Process crashes
Before assignment is fully persisted
```

Reconciliation detects:

```text
BUSY driver
without active assignment
```

and safely repairs it after verifying there is no valid active delivery assignment.

This is why durable state and reconciliation are both important.

---

# 58. Failure During Driver Acceptance

Suppose the driver accepts but the service crashes before responding.

The client may retry.

The operation must be idempotent:

```text
OFFERED -> ACCEPTED

retry:
ACCEPTED -> no-op / return existing result
```

The same assignment must not produce two active assignments.

---

# 59. Failure During Event Publishing

If the service changes durable state but Kafka publishing fails, the event must not be silently lost.

Preferred architecture:

```text
MongoDB transaction / reliable event record
          |
          v
Outbox / event relay
          |
          v
Kafka
```

If the implementation chooses an alternative reliable publication mechanism, document it explicitly and verify its guarantees.

Do not claim at-most-once delivery if the actual implementation is at-least-once.

---

# 60. Transactional Outbox Consideration

Because MongoDB is the Driver & Dispatch database, an outbox pattern can be used when durable Kafka events must be atomically associated with state changes.

Conceptually:

```text
MongoDB Transaction
   |
   +--> update assignment
   +--> insert outbox event
   |
   +--> COMMIT

Outbox Relay
   |
   v
Kafka
```

The initial implementation may use a MongoDB outbox collection and polling/worker relay.

Debezium/CDC is a future enhancement and should not be introduced initially unless there is a measured requirement.

---

# 61. GraphQL Federation

The Driver subgraph may expose driver-facing/admin/customer-safe queries and mutations through the Gateway.

Suggested operations:

```text
Queries:
  driver(id)
  driverStatus(driverId)
  nearbyDrivers(input)
  driverActiveAssignment(driverId)

Mutations:
  goOnline
  goOffline
  acceptAssignment
  rejectAssignment
  acknowledgeAssignment
```

High-frequency location updates should remain on the realtime path rather than being sent as GraphQL mutations.

---

# 62. GraphQL Security

The Gateway performs:

```text
JWT validation
Rate limiting
Correlation ID
Authorization context
GraphQL validation
```

The Driver subgraph performs domain authorization checks as defense in depth.

Examples:

```text
Driver can accept only own assignment.
Customer cannot change driver state.
Admin can manage driver status if authorized.
```

---

# 63. Service Authorization Matrix

| Operation | Driver | Customer | Admin |
|---|---:|---:|---:|
| View own status | Yes | No | Yes |
| Go online/offline | Yes | No | Yes |
| Accept own assignment | Yes | No | Yes |
| Reject own assignment | Yes | No | Yes |
| View nearby drivers | No | No | Yes / internal only |
| Assign driver | No | No | Internal Delivery/Dispatch |
| Suspend driver | No | No | Yes |
| View driver operational info | Limited | No | Yes |

Exact permissions should follow the platform's authorization model.

---

# 64. Internal API vs Public API

There are two distinct API categories.

### Public/business API

```text
Client -> GraphQL Gateway -> Driver Subgraph
```

### Internal service API

```text
Delivery -> gRPC -> Driver & Dispatch
```

Never expose the internal gRPC service directly to the public internet.

---

# 65. Correlation and Causation

Every operation should propagate:

```text
requestId
correlationId
causationId
userId
driverId
deliveryId
assignmentId
traceId
```

Example:

```text
createDelivery
  correlationId = C1
      |
      v
FindAvailableDriver
      correlationId = C1
      |
      v
assignment.accepted
      correlationId = C1
      causationId = E1
```

This makes distributed debugging possible.

---

# 66. Observability

The service should expose:

### Metrics

```text
dispatch_requests_total
dispatch_success_total
dispatch_failure_total
dispatch_duration_seconds
driver_assignment_total
driver_assignment_accept_total
driver_assignment_reject_total
driver_assignment_expire_total
driver_reservation_conflict_total
driver_location_updates_total
driver_location_stale_total
redis_geo_query_duration_seconds
redis_lock_contention_total
kafka_publish_failures_total
kafka_consumer_lag
nats_publish_failures_total
grpc_request_duration_seconds
```

---

# 67. Distributed Tracing

Tracing path:

```text
GraphQL Gateway
      |
      v
Delivery Service
      |
      | gRPC
      v
Driver & Dispatch
      |
      +--> Redis GEO
      +--> Redis Lock
      +--> MongoDB
      +--> Kafka
      +--> NATS
      |
      v
Realtime Service
      |
      v
WebSocket Client
```

OpenTelemetry should propagate trace context across gRPC and messaging boundaries wherever supported by the chosen libraries.

---

# 68. Structured Logging

Every important log should include structured fields.

Example:

```json
{
  "level": "INFO",
  "message": "driver assignment accepted",
  "service": "driver-dispatch-service",
  "traceId": "...",
  "correlationId": "...",
  "deliveryId": "delivery-123",
  "driverId": "driver-456",
  "assignmentId": "assignment-789"
}
```

Never log:

- JWTs
- passwords
- payment secrets
- private credentials
- unnecessary personal information

---

# 69. Health Checks

Provide internal health endpoints according to the service runtime/platform conventions.

Checks:

```text
Process alive
MongoDB connectivity
Redis connectivity
Kafka connectivity if producer/consumer enabled
NATS connectivity if enabled
```

Separate:

```text
liveness
readiness
```

A temporary MongoDB outage may make the service not ready while the process remains alive.

---

# 70. Go Project Structure

Recommended structure:

```text
driver-dispatch-service/
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── domain/
│   │   ├── driver.go
│   │   ├── assignment.go
│   │   ├── dispatch.go
│   │   ├── state.go
│   │   └── errors.go
│   │
│   ├── application/
│   │   ├── commands/
│   │   │   ├── go_online.go
│   │   │   ├── go_offline.go
│   │   │   ├── reserve_driver.go
│   │   │   ├── accept_assignment.go
│   │   │   ├── reject_assignment.go
│   │   │   └── release_driver.go
│   │   │
│   │   ├── queries/
│   │   │   ├── find_available_drivers.go
│   │   │   ├── get_driver.go
│   │   │   └── get_assignment.go
│   │   │
│   │   └── services/
│   │       └── dispatch_service.go
│   │
│   ├── ports/
│   │   ├── driver_repository.go
│   │   ├── assignment_repository.go
│   │   ├── location_store.go
│   │   ├── lock_manager.go
│   │   ├── event_publisher.go
│   │   └── idempotency_store.go
│   │
│   ├── adapters/
│   │   ├── mongodb/
│   │   ├── redis/
│   │   ├── kafka/
│   │   ├── nats/
│   │   └── grpc/
│   │
│   ├── workers/
│   │   ├── assignment_expiry.go
│   │   ├── reconciliation.go
│   │   └── heartbeat_monitor.go
│   │
│   └── transport/
│       └── graphql/
│
├── proto/
│   └── driver/v1/driver.proto
│
├── migrations/
│   └── README.md
│
├── tests/
│   ├── unit/
│   ├── integration/
│   ├── contract/
│   └── e2e/
│
├── deploy/
│   └── kubernetes/
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── configmap.yaml
│       ├── secret.example.yaml
│       ├── hpa.yaml
│       ├── pdb.yaml
│       └── network-policy.yaml
│
├── configs/
│   ├── config.yaml
│   └── config.example.yaml
│
├── scripts/
│   ├── test.sh
│   └── generate-proto.sh
│
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

---

# 71. Dependency Direction

Keep dependency direction clear:

```text
transport
   |
application
   |
domain
   |
ports
   ^
adapters
```

The domain must not depend directly on MongoDB, Redis, Kafka, NATS, or gRPC implementations.

---

# 72. Domain Layer

The domain layer should contain:

```text
Driver
Assignment
State transitions
Dispatch rules
Business invariants
Domain errors
```

It should not contain:

```text
Mongo queries
Redis commands
Kafka client code
NATS client code
HTTP handlers
GraphQL resolver logic
```

---

# 73. Application Layer

The application layer coordinates use cases:

```text
FindAvailableDrivers
ReserveDriver
AcceptAssignment
RejectAssignment
ReleaseDriver
HandleDeliveryCompleted
ReconcileDriver
```

It calls ports/interfaces instead of concrete infrastructure.

---

# 74. Adapter Layer

Adapters implement infrastructure contracts:

```text
MongoDBRepository
RedisGeoStore
RedisLockManager
KafkaPublisher
NATSClient
GrpcServer
GraphQLAdapter
```

---

# 75. Configuration

Configuration should include:

```text
SERVICE_NAME
SERVICE_PORT
GRPC_PORT
MONGODB_URI
MONGODB_DATABASE
REDIS_URL
KAFKA_BROKERS
KAFKA_CLIENT_ID
NATS_URL
DISPATCH_SEARCH_RADIUS_KM
ASSIGNMENT_TIMEOUT_SECONDS
LOCATION_STALE_SECONDS
LOCK_TTL_MS
MAX_DISPATCH_ATTEMPTS
```

Never hard-code secrets.

Use environment variables locally and Kubernetes Secrets/ConfigMaps appropriately.

---

# 76. Docker

The service should use a multi-stage Go build.

Conceptually:

```text
Builder Image
   |
   +--> go mod download
   +--> go build
   |
   v
Minimal Runtime Image
   |
   v
Driver & Dispatch
```

The final image should contain only what is required to run the compiled binary.

---

# 77. Docker Compose

Local infrastructure:

```text
Driver Service
MongoDB
Redis
Kafka
NATS
API Gateway
Delivery Service
Realtime Service
User Service
Notification Service
Media Service
Search Service
```

The compose environment should allow the complete assignment flow to be tested locally.

---

# 78. Kubernetes

Production-like deployment should include:

```text
Deployment
Service
ConfigMap
Secret
HPA
PDB
NetworkPolicy
```

HPA signals may include:

```text
CPU
Memory
gRPC request rate
dispatch queue/work backlog
custom latency metrics
```

Do not autoscale only because Kubernetes supports it. Define an actual scaling signal.

---

# 79. Skaffold

Skaffold should support:

```text
build
push/load
deploy
port-forward
local development loop
```

Recommended development flow:

```text
Edit Go code
    |
    v
Skaffold detects change
    |
    v
Build image
    |
    v
Deploy/update K8s
    |
    v
Run service
```

---

# 80. Network Security

Internal traffic should use:

```text
gRPC service identity
TLS/mTLS where required
Kubernetes NetworkPolicies
Secrets management
```

The service should not expose MongoDB or Redis publicly.

---

# 81. Rate Limiting

Rate limiting for public client operations belongs primarily at the Gateway.

Driver & Dispatch may still enforce domain-specific protection:

```text
maximum location updates per driver
maximum assignment commands
maximum dispatch attempts
```

This is defense in depth, not a replacement for Gateway rate limiting.

---

# 82. Backpressure

High-frequency location traffic can overload downstream components.

Use:

```text
bounded channels
worker pools
rate limits
sampling/coalescing
latest-state semantics
```

For location updates, it is often preferable to process the latest location rather than queue every stale GPS packet indefinitely.

---

# 83. Location Coalescing

Example:

```text
L1
L2
L3
L4
```

If the worker has not processed L1 yet and L4 is already available, the system may safely prefer L4 for current operational location if the domain does not require historical telemetry.

Historical telemetry should be a separate analytics requirement.

---

# 84. Dispatch Backpressure

If dispatch demand exceeds available drivers:

```text
Requests
   |
   v
Dispatch Engine
   |
   +--> available drivers
   |
   +--> no drivers
             |
             v
        WAITING / FAILED
```

The service should return a meaningful state to the Delivery Service instead of retrying infinitely.

---

# 85. No Driver Available

Possible policy:

```text
Search radius 2km
   |
   no driver
   v
Search radius 5km
   |
   no driver
   v
Search radius 10km
   |
   no driver
   v
Dispatch unavailable
```

The radius expansion must have a hard maximum.

Do not perform unlimited GEO searches.

---

# 86. Duplicate Assignment Prevention

Invariant:

```text
One active delivery per driver
```

Enforcement layers:

```text
Redis distributed lock
        +
MongoDB conditional state update
        +
Unique/index constraints where possible
        +
Idempotency
        +
Reconciliation
```

This is stronger than relying on a single mechanism.

---

# 87. MongoDB Index Strategy

Suggested indexes:

```text
drivers:
  { status: 1 }
  { userId: 1 }

assignments:
  { driverId: 1, status: 1 }
  { deliveryId: 1, status: 1 }
  { expiresAt: 1 }

attempts:
  { deliveryId: 1, createdAt: -1 }
  { driverId: 1, createdAt: -1 }
```

Indexes must be validated against actual query patterns.

---

# 88. Data Retention

Operational data should have a retention strategy.

For example:

```text
Current driver state -> long-lived
Active assignments   -> long-lived until completed
Dispatch attempts    -> retained for operational analytics
Expired assignments  -> archived/retained according to policy
Location snapshots   -> short retention unless analytics requires longer
```

Do not retain unlimited high-frequency location records in MongoDB.

---

# 89. Privacy

Driver location is sensitive operational information.

Apply:

- Least privilege
- Minimal exposure
- No public raw location queries unless authorized
- Audit administrative access
- Avoid logging exact coordinates unnecessarily

Customer-facing location should expose only what the product requires.

---

# 90. Audit Events

Administrative changes may produce durable audit events:

```text
driver.suspended
driver.activated
driver.status.changed
assignment.manually.reassigned
```

The audit trail should not be stored only in Redis.

---

# 91. Saga Relationship

The Delivery Service owns the Saga.

Driver & Dispatch participates in the Saga.

```text
                DELIVERY SAGA
                     |
                     v
             Delivery Service
                     |
            Find / Assign Driver
                     |
                     v
            Driver & Dispatch
                     |
            +--------+--------+
            |                 |
         accepted          rejected
            |                 |
            v                 v
       continue saga      next candidate
```

Driver & Dispatch must not independently decide payment or delivery completion.

---

# 92. Saga Compensation

If payment fails after a driver has been assigned, the Delivery Service may request driver release.

```text
Payment Failed
     |
     v
Delivery Saga
     |
     | ReleaseDriver
     v
Driver & Dispatch
     |
     v
BUSY -> AVAILABLE
```

The release operation must be idempotent.

---

# 93. Delivery Cancellation

```text
Customer cancels delivery
        |
        v
Delivery Service
        |
        | cancellation command/event
        v
Driver & Dispatch
        |
        +--> Find active assignment
        +--> Cancel/release assignment
        +--> Release driver
        +--> Publish driver.available
```

If the driver has already started delivery, business rules may prohibit cancellation or require a different compensation path.

---

# 94. Delivery Completion

```text
Delivery Service
      |
      | delivery.completed
      v
Driver & Dispatch
      |
      +--> locate active assignment
      +--> mark COMPLETED
      +--> release driver
      +--> update operational state
      +--> publish driver.available
```

The event handler must be idempotent.

---

# 95. Driver Offline During Assignment

If a driver disconnects while an assignment is OFFERED:

```text
OFFERED
  |
  | driver becomes stale/offline
  v
EXPIRED / CANCELLED
  |
  v
release driver
  |
  v
next candidate
```

If the driver is already ACTIVE on a delivery, do not automatically reassign without applying the business recovery policy.

---

# 96. Reassignment

Reassignment should be an explicit operation.

```text
Current driver
      |
      | release
      v
AVAILABLE

Delivery
      |
      v
Dispatch again
      |
      v
New driver
```

The service should preserve the assignment history rather than overwriting the old record without traceability.

---

# 97. Assignment History

Do not destroy previous attempts.

Example:

```text
Attempt 1 -> Driver A -> rejected
Attempt 2 -> Driver B -> timeout
Attempt 3 -> Driver C -> accepted
```

This enables:

- Debugging
- Analytics
- Driver performance analysis
- Dispatch optimization
- Future ML training data

---

# 98. Search Service Integration

Search Service owns OpenSearch.

Driver & Dispatch must not write directly to OpenSearch.

If driver search becomes a requirement:

```text
Driver DB
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

Driver & Dispatch remains the source of truth.

---

# 99. Analytics Integration

Analytics should consume durable Kafka events.

```text
Driver & Dispatch
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

Potential metrics:

```text
average assignment time
acceptance rate
rejection rate
timeout rate
distance to assigned driver
dispatch success rate
active drivers
```

---

# 100. Notification Integration

Driver & Dispatch should not send email or push notifications directly.

Instead:

```text
Driver & Dispatch
       |
       | driver.assignment.offered
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

Realtime assignment delivery can happen through NATS/WebSocket independently.

---

# 101. Media Integration

If a driver uploads proof-of-delivery media:

```text
Driver
  |
  v
Media Service
  |
  v
Object Storage
```

Driver & Dispatch should store only a reference if operationally necessary.

It must not become the owner of uploaded file bytes.

---

# 102. Payment Integration

Payment is a separate bounded context.

Driver & Dispatch must not store payment details.

Potential future Saga:

```text
Delivery Created
      |
      v
Driver Assigned
      |
      v
Payment
      |
      v
Delivery Started
```

If payment fails:

```text
Payment Failed
      |
      v
Delivery Saga
      |
      v
Release Driver
```

---

# 103. API Gateway Responsibilities

The Gateway handles:

```text
GraphQL Federation
JWT authentication
Rate limiting
Correlation ID
Request context
Query validation
Error normalization
```

It must not handle:

```text
Driver matching
Redis GEO
Assignment locking
MongoDB access
Dispatch algorithms
```

---

# 104. Error Model

Internal domain errors should map to stable API/gRPC errors.

Examples:

```text
DRIVER_NOT_FOUND
DRIVER_NOT_AVAILABLE
DRIVER_ALREADY_ASSIGNED
ASSIGNMENT_NOT_FOUND
ASSIGNMENT_EXPIRED
INVALID_ASSIGNMENT_STATE
UNAUTHORIZED_DRIVER
NO_AVAILABLE_DRIVER
DISPATCH_EXHAUSTED
```

Errors should be machine-readable and human-debuggable.

---

# 105. Timeout Policy

Every distributed call needs a deadline.

Example starting values:

```text
Redis GEO query       < 50ms target
Redis lock operation  < 50ms target
gRPC internal call    500ms–2s depending on operation
MongoDB command       bounded by request deadline
NATS publish          bounded
Kafka publish         bounded + retry
```

These are initial engineering targets, not guaranteed SLAs. Benchmark and adjust them.

---

# 106. Circuit Breaker

Circuit breakers may be used around unstable remote dependencies.

Example:

```text
Driver Service
      |
      v
Redis/Mongo
```

If a dependency is repeatedly unavailable:

```text
CLOSED
  |
  | failures
  v
OPEN
  |
  | cooldown
  v
HALF_OPEN
  |
  +--> success -> CLOSED
  +--> failure -> OPEN
```

Do not add circuit breakers around local operations without evidence they are needed.

---

# 107. Testing Strategy

## Unit Tests

Test:

```text
state transitions
dispatch ranking
assignment rules
idempotency rules
authorization rules
```

## Integration Tests

Test:

```text
MongoDB
Redis GEO
Redis locks
Kafka
NATS
gRPC
```

## Contract Tests

Validate `.proto` compatibility between Delivery and Driver services.

## E2E

Test:

```text
Create delivery
Find driver
Offer assignment
Driver accepts
Delivery progresses
Delivery completes
Driver becomes AVAILABLE
```

---

# 108. Concurrency Tests

This service requires dedicated concurrency tests.

Example:

```text
100 concurrent deliveries
100 concurrent assignment attempts
same driver candidate
```

Expected invariant:

```text
one driver -> max one active assignment
```

Also test:

```text
100 duplicate accept commands
100 duplicate reject commands
multiple service replicas
Redis lock contention
MongoDB transient failures
```

---

# 109. Chaos / Failure Testing

Simulate:

```text
Redis unavailable
MongoDB unavailable
Kafka unavailable
NATS unavailable
Delivery Service unavailable
Realtime Service unavailable
network latency
process crash during reservation
process crash after state update
duplicate events
out-of-order events
```

The service should fail safely rather than corrupt assignment state.

---

# 110. Capacity Estimation

The architecture should start with explicit assumptions.

Example development target:

```text
10,000 registered drivers
1,000 concurrent online drivers
100 location updates/second
100 dispatch requests/second peak
10–20 concurrent service replicas in a future scale test
```

These numbers are for engineering exercises, not production commitments.

The most important load is usually location traffic, not driver CRUD.

---

# 111. Location Throughput Calculation

If:

```text
1,000 online drivers
1 location update / 5 seconds
```

Then:

```text
1000 / 5 = 200 updates/sec
```

If every update were persisted directly to MongoDB, unnecessary write pressure would be created.

Therefore:

```text
GPS -> Realtime -> NATS -> Redis GEO
```

is preferable for the hot path.

---

# 112. Scaling Strategy

Scale independently by workload.

```text
Dispatch load increases
        |
        v
Scale Driver & Dispatch replicas
```

Redis and MongoDB become shared infrastructure.

Avoid local in-memory driver state as the authoritative state because multiple replicas must agree.

---

# 113. MongoDB Scaling

Future options:

```text
Indexes
Replica Set
Read scaling
Sharding if required
```

Do not introduce MongoDB sharding initially.

First prove that indexes and replica scaling are insufficient.

---

# 114. Redis Scaling

Redis is critical for:

```text
GEO
Locks
Location cache
Idempotency
```

Future scaling can use:

```text
Redis Sentinel
Redis Cluster
```

Only introduce these when the deployment requirement justifies them.

---

# 115. Driver Presence Model

A driver should be considered dispatchable only when:

```text
status = AVAILABLE
AND
heartbeat is fresh
AND
location exists
AND
location is fresh
AND
not blocked
AND
vehicle capability matches
```

This is more correct than simply checking:

```text
status == AVAILABLE
```

---

# 116. Dispatch Eligibility Function

Conceptually:

```text
eligible(driver, delivery):

  driver.status == AVAILABLE
  AND heartbeatFresh(driver)
  AND locationFresh(driver)
  AND vehicleCompatible(driver, delivery)
  AND notAlreadyAssigned(driver)
```

The function should be deterministic and testable.

---

# 117. Location Freshness

Suggested model:

```text
FRESH       < 15 sec
STALE       15–30 sec
OFFLINE     > 30 sec
```

These are configurable examples.

Do not use a single hard-coded threshold throughout the code.

---

# 118. Dispatch Radius

Configuration:

```text
INITIAL_RADIUS_KM=2
SECOND_RADIUS_KM=5
MAX_RADIUS_KM=10
```

Dispatch should stop when:

```text
MAX_RADIUS_KM reached
OR
MAX_ATTEMPTS reached
```

---

# 119. Assignment Offer Payload

NATS payload may contain:

```json
{
  "eventId": "uuid",
  "deliveryId": "delivery-123",
  "assignmentId": "assignment-456",
  "driverId": "driver-789",
  "pickup": {
    "latitude": 31.4,
    "longitude": 31.2
  },
  "expiresAt": "timestamp",
  "correlationId": "uuid"
}
```

The payload should contain only what the receiving realtime layer needs.

---

# 120. Location Update Payload

Example:

```json
{
  "driverId": "driver-123",
  "latitude": 31.416,
  "longitude": 31.245,
  "accuracy": 8.5,
  "speed": 14.2,
  "heading": 90,
  "timestamp": "2026-09-02T12:00:00Z",
  "correlationId": "uuid"
}
```

Validate this payload before touching Redis.

---

# 121. Ordering

Location updates may arrive out of order.

Example:

```text
L10 timestamp 10:10:10
L9  timestamp 10:10:09
```

If L10 is already stored, L9 should not overwrite it.

Store the latest accepted timestamp and compare before updating current location.

---

# 122. Exactly Once vs At Least Once

The system should assume **at-least-once delivery** for messaging.

Do not design around exactly-once assumptions unless a specific mechanism and guarantee has been proven.

Therefore:

```text
Kafka -> duplicates possible
NATS -> delivery semantics depend on configuration
WebSocket -> reconnect/retry possible
gRPC -> client retry possible
```

Idempotency is mandatory.

---

# 123. Out-of-Order Events

Example:

```text
driver.assignment.accepted
        arrives after
 delivery.cancelled
```

The service must validate current state before applying the event.

State transitions should be conditional:

```text
OFFERED -> ACCEPTED
```

not:

```text
set status = ACCEPTED
```

blindly.

---

# 124. Event Versioning

Events should include:

```text
version: 1
```

When payload changes:

```text
version 2
```

Consumers should support compatible versions or use explicit migration logic.

---

# 125. Schema Evolution

For MongoDB:

- Prefer additive changes
- Maintain compatibility during rolling deployment
- Avoid requiring every document to migrate instantly

For protobuf:

- Never reuse field numbers
- Add fields instead of changing meaning
- Preserve backward compatibility

For Kafka events:

- Version event contracts
- Keep consumers tolerant of additional fields

---

# 126. Operational Dashboard

Recommended dashboard panels:

```text
Online drivers
Available drivers
Busy drivers
Assignments/minute
Assignment success rate
Assignment rejection rate
Assignment timeout rate
Average assignment time
Average distance to driver
Dispatch failures
Redis GEO latency
Lock contention
MongoDB latency
Kafka lag
NATS errors
gRPC latency
```

---

# 127. Alerting

Alerts may include:

```text
No available drivers for extended period
High assignment rejection rate
High assignment timeout rate
Redis GEO latency high
MongoDB errors high
Kafka consumer lag high
NATS connection failures
Lock contention spike
Stale driver count spike
```

Alerts should represent actionable conditions.

---

# 128. Security Threats

Protect against:

```text
Driver impersonation
Unauthorized assignment acceptance
Location spoofing
Replay of assignment commands
Excessive location spam
Unauthorized admin driver modification
Credential leakage
```

Mitigations:

```text
JWT authentication at Gateway/Realtime
Service-to-service authentication
Authorization checks
Idempotency
Rate limiting
Input validation
Audit logs
TLS
```

---

# 129. Location Spoofing

The system cannot assume GPS data is truthful.

Future anti-fraud checks may include:

```text
speed anomaly
teleportation detection
impossible route movement
device integrity
historical patterns
```

These are future enhancements, not initial requirements.

---

# 130. API Versioning

GraphQL is schema-based, so prefer additive schema evolution instead of URL versioning.

gRPC uses versioned packages:

```text
package driver.v1;
```

Future breaking changes can use:

```text
v2
```

---

# 131. Service-Level Invariants

The service must enforce:

```text
1. A blocked driver cannot receive an assignment.
2. An OFFLINE driver cannot receive an assignment.
3. A driver cannot have two active assignments.
4. An assignment cannot be accepted twice with different results.
5. A rejected assignment cannot become accepted later.
6. An expired offer cannot become accepted after expiration.
7. A completed assignment cannot return to OFFERED.
8. A driver cannot release another driver's assignment.
9. Redis failure must not corrupt MongoDB business state.
10. Duplicate events must not duplicate assignments.
```

---

# 132. Complete State Transition Table

| Entity | From | Event | To |
|---|---|---|---|
| Driver | OFFLINE | GO_ONLINE | AVAILABLE |
| Driver | AVAILABLE | RESERVE | BUSY |
| Driver | BUSY | RELEASE | AVAILABLE |
| Driver | BUSY | GO_OFFLINE | controlled/offline state |
| Assignment | NONE | OFFER | OFFERED |
| Assignment | OFFERED | ACCEPT | ACCEPTED |
| Assignment | OFFERED | REJECT | REJECTED |
| Assignment | OFFERED | TIMEOUT | EXPIRED |
| Assignment | ACCEPTED | START | ACTIVE |
| Assignment | ACTIVE | COMPLETE | COMPLETED |
| Assignment | OFFERED | CANCEL | CANCELLED |

Every transition must validate the current state.

---

# 133. End-to-End Example

```text
Customer
   |
   | createDelivery
   v
API Gateway
   |
   v
Delivery Service
   |
   | delivery created
   v
Delivery Saga
   |
   | FindAvailableDrivers
   | gRPC
   v
Driver & Dispatch
   |
   | Redis GEO
   v
Driver A / Driver B / Driver C
   |
   | rank
   v
Driver B
   |
   | Redis lock
   v
MongoDB conditional update
   |
   | AVAILABLE -> BUSY
   v
Assignment OFFERED
   |
   | NATS
   v
Realtime Service
   |
   | WebSocket
   v
Driver B
   |
   | ACCEPT
   v
Realtime
   |
   | NATS
   v
Driver & Dispatch
   |
   | ACCEPTED
   v
Kafka
   |
   v
Delivery Service
   |
   v
Saga advances
```

---

# 134. Driver Location End-to-End Example

```text
Driver GPS
   |
   v
WebSocket
   |
   v
Realtime Service
   |
   | NATS
   v
Driver & Dispatch
   |
   +--> validate
   +--> timestamp check
   +--> Redis GEOADD
   +--> update latest location
   |
   v
Redis
```

The next dispatch request can immediately use the new position.

---

# 135. Customer Tracking Example

```text
Driver
  |
  | GPS
  v
Realtime
  |
  +--------------------+
  |                    |
  v                    v
NATS              WebSocket
  |                    |
  v                    v
Driver/Dispatch      Customer
```

The dispatch service may consume location information for operational decisions while Realtime can independently fan out updates to subscribed clients.

---

# 136. Realtime vs Durable Location

The architecture intentionally separates:

```text
Current operational location -> Redis
Durable business facts       -> Kafka
Historical analytics         -> ClickHouse/future telemetry pipeline
```

Do not send every GPS packet through every infrastructure component.

---

# 137. Why Go for Driver & Dispatch

Go is particularly suitable here because the service benefits from:

- Efficient concurrency
- Goroutines
- Channels
- Worker pools
- Low memory overhead
- Strong gRPC ecosystem
- Fast startup
- Simple deployment
- Good networking performance

This service is also a useful place to demonstrate production Go rather than using Go only for CRUD.

---

# 138. Go Concurrency Model

Potential goroutines:

```text
main
  |
  +--> gRPC server
  +--> GraphQL adapter
  +--> NATS subscriber
  +--> Kafka consumer
  +--> assignment expiry worker
  +--> reconciliation worker
  +--> heartbeat monitor
```

Use controlled shutdown with context cancellation.

---

# 139. Graceful Shutdown

On SIGTERM:

```text
1. Stop accepting new requests.
2. Stop new dispatch work.
3. Finish safe in-flight operations.
4. Stop Kafka consumers.
5. Stop NATS subscriptions.
6. Stop workers.
7. Flush telemetry.
8. Close Redis/Mongo connections.
9. Exit.
```

Kubernetes termination grace period must be long enough for this process.

---

# 140. Common NestJS Package Boundary

The overall repository may contain a shared NestJS common package for NestJS services.

Driver & Dispatch is Go and must not depend on NestJS packages.

Cross-language shared contracts should be:

```text
protobuf
event schemas
documented conventions
```

Not shared TypeScript business logic.

---

# 141. Repository-Level Shared Components

Recommended shared repository areas:

```text
libs/contracts
libs/events
libs/observability
libs/messaging
proto/
```

The Driver service may consume generated protobuf code and shared event schemas without importing business logic from another service.

---

# 142. Environment Separation

```text
local
staging
production
```

Each environment should have independent:

```text
MongoDB
Redis
Kafka topics
NATS subjects/environment namespace
secrets
observability resources
```

Avoid accidentally connecting local services to production infrastructure.

---

# 143. Namespace Strategy

Kafka topic example:

```text
prod.driver.assignment.accepted
staging.driver.assignment.accepted
```

Or isolate environments at the cluster level. Pick one strategy and apply it consistently.

NATS can use subjects such as:

```text
prod.driver.location.updated
prod.driver.assignment.offered
```

---

# 144. Configuration of Dispatch Policy

The dispatch policy should be configuration-driven:

```text
initialRadius
maxRadius
maxCandidates
offerTimeout
maxAttempts
staleLocationThreshold
allowedVehicleTypes
```

Do not hard-code these values throughout the domain logic.

---

# 145. Feature Flags

Future dispatch strategies may be introduced behind feature flags:

```text
DISPATCH_STRATEGY=nearest
```

Future:

```text
nearest
weighted
traffic_aware
ml_ranked
```

Initial implementation should use `nearest`.

---

# 146. Future Advanced Dispatch

Future enhancements may include:

```text
Traffic-aware ETA
Route duration
Driver acceptance probability
Multi-stop optimization
Batch dispatch
Dynamic radius
Load balancing
Predictive availability
ML-based driver ranking
```

These should not complicate the first implementation.

---

# 147. Future GenAI Integration

GenAI is **future only**.

The initial Driver & Dispatch Service must not depend on an LLM.

A future FastAPI AI Service could provide:

```text
Dispatch explanation
Operational assistant
Natural-language driver operations
Anomaly explanation
Natural-language delivery investigation
```

Potential future flow:

```text
Client
  |
  v
AI Service (FastAPI)
  |
  | gRPC/tool calls
  +--> Delivery
  +--> Driver & Dispatch
  +--> Payment
  +--> Notification
```

AI must not directly mutate operational state without an explicit, authorized command path.

---

# 148. Future ML Dispatch

A future ML model may calculate:

```text
P(driver accepts)
ETA
probability of completion
expected pickup time
```

Then:

```text
candidate drivers
       |
       v
ML scoring
       |
       v
Dispatch policy
       |
       v
Reservation
```

The model should remain advisory until thoroughly validated.

---

# 149. Future Event Sourcing

Event sourcing is not required initially.

Current design:

```text
MongoDB current state
+ assignment history
+ Kafka domain events
```

Future event sourcing may be evaluated for selected aggregates if operational requirements justify it.

---

# 150. What NOT to Add Initially

Do not add:

```text
REST API
NATS JetStream as a Kafka replacement
Event sourcing
Temporal workflow engine
Redis Cluster
MongoDB sharding
ML dispatch
GenAI
Qdrant
Extra Driver microservices
Dedicated Location microservice
Dedicated Matching microservice
Dedicated Dispatch microservice
```

The goal is a strong architecture, not maximum technology count.

---

# 151. Implementation Phases

## Phase 1 — Service Skeleton

```text
Go module
configuration
logging
health checks
Docker
basic Kubernetes manifests
```

## Phase 2 — MongoDB

```text
driver repository
assignment repository
indexes
state models
```

## Phase 3 — Driver State

```text
GoOnline
GoOffline
status transitions
authorization
```

## Phase 4 — Redis GEO

```text
location updates
GEOADD
GEOSEARCH
latest location
stale detection
```

## Phase 5 — Assignment

```text
candidate selection
Redis locks
conditional MongoDB updates
assignment state machine
```

## Phase 6 — gRPC

```text
proto
code generation
Delivery -> Driver calls
```

## Phase 7 — NATS + Realtime

```text
location subjects
assignment subjects
Realtime integration
```

## Phase 8 — Kafka

```text
durable assignment events
consumer idempotency
DLQ
outbox/relay if required
```

## Phase 9 — Reconciliation

```text
expired assignments
stale drivers
orphaned BUSY state
```

## Phase 10 — Observability

```text
OpenTelemetry
Prometheus
Grafana
Jaeger
structured logs
```

## Phase 11 — Kubernetes

```text
Deployment
HPA
PDB
NetworkPolicy
Skaffold
```

## Phase 12 — Full E2E

```text
Customer -> Delivery -> Driver -> Realtime -> Kafka -> Completion
```

---

# 152. Definition of Done

The service is considered complete when:

- [ ] Go service starts locally
- [ ] MongoDB repository works
- [ ] Driver state machine is enforced
- [ ] Redis GEO stores current locations
- [ ] Stale locations are handled
- [ ] Candidate drivers can be found
- [ ] Driver reservation is concurrency-safe
- [ ] Assignment state machine works
- [ ] Duplicate commands are idempotent
- [ ] gRPC contract is implemented
- [ ] Delivery Service can request drivers
- [ ] Driver can accept/reject through Realtime
- [ ] NATS integration works
- [ ] Durable Kafka events are published
- [ ] Kafka consumers are idempotent
- [ ] DLQ strategy works
- [ ] Reconciliation works
- [ ] Metrics exist
- [ ] Tracing exists
- [ ] Structured logging exists
- [ ] Docker image works
- [ ] Kubernetes deployment works
- [ ] Skaffold workflow works
- [ ] Concurrency tests pass
- [ ] E2E delivery flow passes

---

# 153. AI Coding Agent Rules

Any AI coding agent working on this service must follow these rules.

```text
1. Do not introduce REST business APIs.
2. Do not create additional microservices.
3. Driver & Dispatch must remain a Go service.
4. MongoDB is the source of truth for driver operational data.
5. Redis is not the source of truth for business state.
6. Redis GEO is used for proximity lookup.
7. Redis locks protect short assignment critical sections.
8. MongoDB conditional updates must protect durable state transitions.
9. Delivery Service owns the Delivery Saga.
10. Driver & Dispatch is a Saga participant.
11. Use gRPC for synchronous internal commands.
12. Use NATS for transient low-latency communication.
13. Use Kafka for durable business events.
14. Do not use NATS JetStream as a duplicate Kafka without justification.
15. WebSocket connections belong to Realtime Service.
16. Do not write directly to another service's database.
17. Do not write directly to OpenSearch.
18. Do not write directly to ClickHouse.
19. Do not put business logic in transport adapters.
20. All critical state transitions must validate current state.
21. All critical commands must be idempotent.
22. All Kafka consumers must be idempotent.
23. All distributed calls must have timeouts.
24. Retry only transient failures.
25. Use exponential backoff and jitter.
26. Every important failure path must be observable.
27. Use reconciliation for recoverable inconsistencies.
28. Do not introduce GenAI in the initial implementation.
29. Do not introduce ML dispatch in the initial implementation.
30. Prefer simple deterministic dispatch before advanced optimization.
31. Preserve assignment history.
32. Do not hold distributed locks while waiting for driver responses.
33. Never trust client MIME/location/state blindly.
34. Preserve protobuf compatibility.
35. Keep services independently deployable.
```

---

# 154. Architecture Decision Records

## ADR-001 — Go

**Decision:** Go is the implementation language.

**Reason:** concurrency, networking, gRPC, worker pools, low overhead, and a strong fit for dispatch/location workloads.

## ADR-002 — MongoDB

**Decision:** MongoDB owns Driver & Dispatch operational data.

**Reason:** operational driver/assignment documents evolve independently and do not require the same relational model as Delivery transactions.

## ADR-003 — Redis GEO

**Decision:** Redis GEO handles proximity lookup.

**Reason:** the hot dispatch path needs fast geographic candidate lookup.

## ADR-004 — Redis Lock

**Decision:** Redis distributed locks protect driver reservation.

**Reason:** multiple service replicas may concurrently attempt the same driver.

## ADR-005 — NATS

**Decision:** NATS handles transient low-latency messages.

**Reason:** location and realtime coordination do not require Kafka-style durable replay.

## ADR-006 — Kafka

**Decision:** Kafka handles durable business facts.

**Reason:** assignment acceptance/rejection and driver lifecycle events may be consumed independently and replayed.

## ADR-007 — Realtime Boundary

**Decision:** Realtime Service owns WebSockets.

**Reason:** connection management and business dispatch logic should remain separate.

## ADR-008 — No Separate Dispatch Microservice

**Decision:** Driver and Dispatch remain one service.

**Reason:** the project is a personal learning system and both domains share tight operational state and concurrency requirements. Splitting them would increase operational complexity without adding meaningful learning value.

---

# 155. Final Architecture Summary

```text
                         CLIENT
                           |
                        GraphQL
                           |
                           v
                    API GATEWAY
                    NestJS/Federation
                           |
                           v
                    DELIVERY SERVICE
                        NestJS
                           |
                         gRPC
                           |
                           v
               DRIVER & DISPATCH SERVICE
                           Go
                           |
          +----------------+----------------+
          |                |                |
          v                v                v
      MongoDB           Redis            Redis GEO
      Durable          Locks             Proximity
       State
          |
          |
          +--------------------+
          |                    |
         Kafka                NATS
          |                    |
          v                    v
   Durable Events         Realtime Service
                               |
                           WebSocket
                               |
                               v
                            CLIENT
```

The critical dispatch invariant is:

```text
ONE DRIVER
    |
    +---- MAX ONE ACTIVE ASSIGNMENT
```

The critical ownership rules are:

```text
Delivery Service
    -> Delivery lifecycle + Saga

Driver & Dispatch
    -> Driver operational state + Dispatch

Realtime Service
    -> WebSocket connections + realtime fan-out

Notification Service
    -> Notification delivery

Media Service
    -> File/media lifecycle

Search Service
    -> Search projection/index

Payment Service
    -> Payment lifecycle

Analytics Service
    -> Analytical projections
```

The Driver & Dispatch Service should therefore remain focused on one difficult problem: **reliably selecting, reserving, assigning, and releasing drivers under concurrent distributed-system conditions.**

---

# 156. Final End-to-End Architecture

```text
                                      CUSTOMER
                                         |
                                         | GraphQL
                                         v
                                +------------------+
                                |   API GATEWAY    |
                                | NestJS Federation|
                                +--------+---------+
                                         |
                                         v
                                +------------------+
                                | DELIVERY SERVICE  |
                                | NestJS + Postgres |
                                +--------+---------+
                                         |
                                         | gRPC
                                         v
                         +-------------------------------+
                         | DRIVER & DISPATCH SERVICE     |
                         |             Go                |
                         |                               |
                         | MongoDB = durable state       |
                         | Redis = locks/cache/state      |
                         | Redis GEO = proximity         |
                         +------+------------------------+
                                |
                   +------------+-------------+
                   |                          |
                 NATS                       Kafka
                   |                          |
                   v                          +----------------+
           REALTIME SERVICE                   |                |
                   |                     Notification       Analytics
              WebSocket                       |                |
                   |                         BullMQ        ClickHouse
                   v
        +-----------------------+
        | Customer / Driver UI  |
        +-----------------------+

Additional platform services:

Media Service -> Object Storage / DynamoDB / Redis
Search Service -> OpenSearch
Payment Service -> PostgreSQL
User Service -> PostgreSQL
```

---

# 157. Final Engineering Principle

The architecture intentionally uses multiple technologies because each solves a distinct problem:

```text
GraphQL Federation -> public API composition
Go                -> high-concurrency dispatch service
gRPC              -> synchronous service contracts
MongoDB           -> driver operational state
Redis             -> fast ephemeral state + locks
Redis GEO         -> driver proximity
NATS              -> transient realtime messaging
Kafka             -> durable business events
WebSocket         -> browser realtime
OpenTelemetry     -> distributed tracing
Prometheus        -> metrics
Kubernetes        -> deployment/scaling
Skaffold          -> development workflow
```

The service should not use a technology merely to demonstrate that technology. Every component must have a concrete responsibility.

---

# 158. Recommended Implementation Order

```text
1. Create Go service skeleton
2. Define domain models
3. Implement driver state machine
4. Implement MongoDB repositories
5. Implement Redis location store
6. Implement Redis GEO search
7. Implement Redis distributed lock
8. Implement assignment state machine
9. Implement candidate ranking
10. Implement reservation flow
11. Add gRPC server
12. Integrate Delivery Service
13. Add NATS location/assignment messaging
14. Integrate Realtime Service
15. Add Kafka durable events
16. Add idempotent consumers
17. Add outbox/relay if required
18. Add assignment expiration worker
19. Add reconciliation worker
20. Add observability
21. Add concurrency tests
22. Add failure tests
23. Dockerize
24. Deploy with Kubernetes
25. Add Skaffold
26. Run full E2E flow
```

This order minimizes premature distributed complexity while still allowing the service to demonstrate the major system-design concepts required by the overall Realtime Delivery Platform.

---

# 159. Quick Reference

## Service

```text
Driver & Dispatch Service
```

## Language

```text
Go
```

## Database

```text
MongoDB
```

## Hot State

```text
Redis
```

## Geospatial

```text
Redis GEO
```

## Sync

```text
gRPC
```

## Transient Messaging

```text
NATS
```

## Durable Events

```text
Kafka
```

## Browser Realtime

```text
WebSocket through Realtime Service
```

## Patterns

```text
State Machine
Idempotency
Distributed Lock
Conditional Update
Retry + Backoff + Jitter
DLQ
Reconciliation
Transactional Outbox where required
Saga participation
```

## Infrastructure

```text
Docker
Docker Compose
Kubernetes
Skaffold
```

## Observability

```text
OpenTelemetry
Prometheus
Grafana
Jaeger
Structured Logging
```

## Future Only

```text
Advanced ML Dispatch
GenAI
AI Agents
Event Sourcing
Temporal-like workflow engines
NATS JetStream for durable workflows
Redis Cluster
MongoDB Sharding
```

---

# 160. Conclusion

The Driver & Dispatch Service is the next logical core service after the Delivery Service.

Its architecture is intentionally centered on one hard distributed-systems problem:

> **How can many delivery requests concurrently find and reserve nearby drivers without assigning the same driver twice, while keeping realtime communication fast and durable business events reliable?**

The answer is:

```text
GraphQL Federation
        |
        v
gRPC
        |
        v
Go Driver & Dispatch
        |
   +----+----+
   |         |
MongoDB    Redis
              |
        +-----+------+
        |            |
     Redis GEO    Distributed Lock
        |
        v
 Candidate Selection
        |
        v
 Conditional Reservation
        |
    +---+---+
    |       |
   NATS    Kafka
    |       |
    v       v
Realtime   Durable Events
    |
 WebSocket
    |
 Client
```

This keeps the service independently deployable, horizontally scalable, observable, and consistent with the architecture of the entire Realtime Delivery Platform.
