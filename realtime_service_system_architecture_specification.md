 Realtime Service — System Architecture Specification

## Realtime Delivery Platform

**Document Type:** Service-Level System Architecture Specification  
**Service:** Realtime Service  
**Primary Responsibility:** Bidirectional real-time communication between platform clients and the delivery platform  
**Language:** TypeScript  
**Framework:** NestJS  
**Primary Protocol:** WebSocket  
**Internal Low-Latency Transport:** NATS Core  
**Shared Realtime State:** Redis  
**Durable Business Events:** Kafka  
**Optional Durable NATS Messaging:** NATS JetStream — evaluated and reserved for specific use cases, not the primary location-update transport  
**Optional Browser Transport:** SSE — evaluated but not the primary transport  
**Background Jobs:** BullMQ — used only when the Realtime Service has deferred/retryable jobs; it is not the realtime transport  
**Public API:** GraphQL Federation through the API Gateway  
**Databases:** No business database owned by the Realtime Service  
**Infrastructure:** Docker, Docker Compose, Kubernetes, Skaffold  
**Observability:** OpenTelemetry, Prometheus, Grafana, centralized structured logs  
**Architecture Style:** Stateless horizontally scalable realtime gateway with externalized ephemeral state and event-driven fan-out

---

# 1. Purpose

The Realtime Service is responsible for delivering time-sensitive updates to connected clients without requiring clients to repeatedly poll the platform.

It is intentionally separated from domain services.

The Realtime Service does not own:

- users
- deliveries
- payments
- drivers as a business entity
- notifications as persistent records
- media
- business transactions

Instead, it owns the realtime connection layer:

```text
Client
  |
  | WebSocket
  v
Realtime Service
  |
  +---- Redis
  |
  +---- NATS
  |
  +---- Kafka
  |
  +---- Domain Services
```

The service acts as a bridge between client connections and internal platform events.

---

# 2. Architectural Position

The platform uses GraphQL Federation as its public API. There is no REST API in the initial architecture.

Realtime traffic is intentionally separated from GraphQL request/response traffic.

```text
                           CLIENT
                             |
             +---------------+---------------+
             |                               |
             | GraphQL                       | WebSocket
             v                               v
      +--------------+              +--------------------+
      | API Gateway  |              | Realtime Service   |
      | NestJS       |              | NestJS             |
      | Federation   |              | WebSocket          |
      +------+-------+              +---------+----------+
             |                                |
             |                                |
       Domain Services                    +---+---+
                                          |       |
                                         NATS   Redis
                                          |
                                      Kafka events
```

The Gateway handles normal API operations.

The Realtime Service handles:

- persistent client connections
- subscriptions
- connection authentication
- presence
- location streaming
- delivery tracking updates
- driver assignment events
- realtime status updates
- cross-instance fan-out
- reconnect handling
- connection cleanup
- realtime authorization

---

# 3. Core Architectural Principle

The Realtime Service must not become a second Delivery Service.

Bad:

```text
Realtime Service
    |
    +-- owns delivery status
    +-- owns driver state
    +-- owns payment state
    +-- writes business data
```

Correct:

```text
Delivery Service
    |
    +-- owns delivery state

Driver Service
    |
    +-- owns driver state

Payment Service
    |
    +-- owns payment state

Realtime Service
    |
    +-- owns connections
    +-- owns subscriptions
    +-- owns realtime routing
```

The source of truth remains in the owning service.

---

# 4. Goals

The Realtime Service must support:

1. Driver location updates.
2. Customer delivery tracking.
3. Driver assignment notifications.
4. Delivery status updates.
5. Driver presence.
6. Connection authentication.
7. Subscription authorization.
8. Multiple WebSocket instances.
9. Horizontal scaling.
10. Cross-instance message fan-out.
11. Reconnection.
12. Heartbeats.
13. Backpressure protection.
14. Rate limiting.
15. Duplicate event protection where necessary.
16. Observability.
17. Graceful shutdown.
18. Failure isolation.
19. Security.
20. High-frequency low-latency traffic.

---

# 5. Non-Goals

The Realtime Service must not:

- own delivery business rules
- own payment processing
- own notification templates
- send email
- send SMS
- persist complete delivery history
- replace Kafka
- replace NATS
- replace Redis
- replace the API Gateway
- expose REST endpoints
- directly access another service's database
- implement Saga orchestration

---

# 6. Technology Responsibility Matrix

| Requirement | Technology | Role |
|---|---|---|
| Public API | GraphQL Federation | Normal client request/response |
| Browser realtime | WebSocket | Bidirectional realtime |
| Alternative server-to-client stream | SSE | Evaluated, not primary |
| Low-latency transient messaging | NATS Core | Cross-instance fan-out |
| Durable business events | Kafka | Replayable domain events |
| Durable NATS messaging | JetStream | Optional selected use cases |
| Realtime state | Redis | Connections/subscriptions/presence |
| Redis messaging | Redis Pub/Sub | Alternative fan-out mechanism, not primary |
| Background jobs | BullMQ | Deferred/retryable work |
| Authentication | JWT | WebSocket handshake |
| Authorization | Guards/Policies | Subscription and action authorization |
| Rate limiting | Redis | Connection/message limits |
| Idempotency | Redis | Duplicate command protection |
| Distributed coordination | Redis | Locks when genuinely required |
| Metrics | Prometheus | Metrics |
| Dashboards | Grafana | Visualization |
| Tracing | OpenTelemetry | Distributed tracing |
| Logs | Structured JSON | Operational debugging |
| Containerization | Docker | Packaging |
| Local orchestration | Docker Compose | Development |
| Production orchestration | Kubernetes | Deployment/scaling |
| Local K8s loop | Skaffold | Build/deploy/watch |

---

# 7. Why WebSocket Is the Primary Protocol

WebSocket is selected because the delivery platform has genuine bidirectional realtime communication.

Drivers need to:

```text
Server -> Driver
    DELIVERY_ASSIGNED
    DELIVERY_CANCELLED
    ROUTE_CHANGED
    COMMAND

Driver -> Server
    ACCEPT_ASSIGNMENT
    REJECT_ASSIGNMENT
    LOCATION_UPDATE
    START_PICKUP
    MARK_PICKED_UP
    START_TRANSIT
    COMPLETE_DELIVERY
```

Customers primarily receive updates:

```text
Server -> Customer
    DRIVER_ASSIGNED
    DRIVER_LOCATION_UPDATED
    DELIVERY_STATUS_UPDATED
    DELIVERY_COMPLETED
```

The same connection can therefore support both directions.

NestJS supports WebSocket gateways and lifecycle hooks such as connection and disconnection handling.

---

# 8. WebSocket vs SSE

## WebSocket

Use for:

- driver applications
- customer tracking when bidirectional capability may be useful
- admin realtime dashboards
- commands
- acknowledgements
- location updates
- presence

Advantages:

- bidirectional
- long-lived connection
- low latency
- suitable for high-frequency updates
- one protocol for driver and customer clients

## SSE

SSE is appropriate when communication is strictly:

```text
Server -> Browser
```

Examples:

- live dashboard feed
- progress stream
- operational event stream
- notification stream

However, SSE is not the primary transport in this architecture because drivers require bidirectional communication.

SSE remains a documented alternative rather than an active second realtime transport.

---

# 9. Why GraphQL Subscriptions Are Not the Primary Realtime Transport

GraphQL Federation remains the public API for request/response operations.

Raw WebSocket is used for the Realtime Service because:

- location updates are high frequency
- realtime messages have different lifecycle requirements
- driver commands are bidirectional
- connection management should remain independent of GraphQL query execution
- the Gateway should remain thin
- high-frequency streams should not require GraphQL query execution for every event

GraphQL Subscriptions may be introduced later for low-frequency application events if a concrete requirement appears.

They are not required for the initial implementation.

---

# 10. High-Level Realtime Architecture

```text
                                  CLIENTS
                                     |
                    +----------------+----------------+
                    |                                 |
                GraphQL                           WebSocket
                    |                                 |
                    v                                 v
             API Gateway                    Realtime Service
             NestJS/Federation                    |
                                                   |
                           +-----------------------+------------------+
                           |                       |                  |
                           v                       v                  v
                         Redis                   NATS               Kafka
                           |                       |                  |
                           |                       |                  |
                           +-----------+-----------+                  |
                                       |                              |
                                       v                              |
                              Realtime Instances                     |
                                       |                              |
                            +----------+----------+                   |
                            |          |          |                   |
                            v          v          v                   |
                          Node A     Node B     Node C                 |
                            |          |          |                   |
                            +----------+----------+                   |
                                       |                              |
                                    Clients                           |
                                                                      |
                             Durable Domain Events <-----------------+
```

---

# 11. Realtime Service Deployment Model

The service must be horizontally scalable.

```text
                    Load Balancer
                         |
          +--------------+--------------+
          |              |              |
          v              v              v
     Realtime-1     Realtime-2     Realtime-3
          |              |              |
          +--------------+--------------+
                         |
                    NATS Cluster
                         |
                    Redis Cluster
```

A client may connect to any Realtime instance.

The system must not depend on local process memory for shared routing.

---

# 12. Stateless Application Design

The Realtime application process should be as stateless as practical.

Do not rely only on:

```text
Map<socketId, Socket>
```

because:

```text
Node A
  |
  +-- socket X

Node B
  |
  +-- socket Y
```

A request arriving at Node A must still be able to discover enough routing metadata to reach the correct connection.

Local memory may still contain the actual socket object because the socket exists only on that node.

Redis stores distributed metadata.

NATS transports events between nodes.

---

# 13. Connection Ownership

A socket physically belongs to one Realtime instance.

Example:

```text
Customer A
    |
    v
Realtime Node B
    |
    +-- local socket object
```

Redis stores:

```text
user -> Node B / socket
delivery -> subscribed sockets
```

NATS allows other nodes to publish events.

The node that owns the socket performs the final write.

---

# 14. Connection Lifecycle

```text
CONNECT
   |
   v
TCP/TLS
   |
   v
HTTP Upgrade
   |
   v
Extract JWT
   |
   v
Validate JWT
   |
   +---- Invalid ----> Reject
   |
   v
Create Connection Context
   |
   v
Register Redis Metadata
   |
   v
Subscribe / Authenticate
   |
   v
CONNECTED
   |
   +---- Messages
   |
   +---- Heartbeats
   |
   +---- Subscriptions
   |
   v
DISCONNECT
   |
   v
Cleanup Redis
   |
   v
Remove local socket
```

---

# 15. WebSocket Authentication

JWT must be validated before the client is considered an authenticated realtime client.

Recommended handshake:

```text
Client
  |
  | Authorization: Bearer <JWT>
  v
Realtime Service
  |
  +-- verify signature
  +-- verify expiry
  +-- extract userId
  +-- extract role
  +-- validate token claims
  |
  +---- invalid -> reject
  |
  v
WebSocket accepted
```

The service should avoid treating an arbitrary client-provided `userId` as authoritative.

Identity comes from the validated JWT.

---

# 16. WebSocket Authorization

Authentication answers:

> Who are you?

Authorization answers:

> Are you allowed to perform this operation?

Example:

```text
Customer A
    |
    | subscribe to delivery X
    v
Realtime Service
    |
    +-- Is Customer A owner/participant of X?
    |
    +-- yes -> subscribe
    +-- no  -> reject
```

Driver example:

```text
Driver A
    |
    | LOCATION_UPDATE delivery X
    v
Realtime Service
    |
    +-- Does driver A own/hold assignment X?
    |
    +-- yes -> accept
    +-- no  -> reject
```

Authorization can use:

- validated JWT claims
- cached authorization context
- gRPC call to the owning service when required
- short-lived authorization cache

The Realtime Service must not bypass domain ownership.

---

# 17. Connection Context

Each active connection has:

```typescript
interface ConnectionContext {
  socketId: string;
  userId: string;
  role: 'CUSTOMER' | 'DRIVER' | 'ADMIN';
  connectedAt: string;
  lastSeenAt: string;
  nodeId: string;
  authenticated: boolean;
}
```

Optional:

```text
deviceId
platform
appVersion
ipHash
userAgent
```

Avoid storing unnecessary personal data.

---

# 18. Redis Data Model

Redis stores ephemeral realtime metadata.

Suggested keys:

```text
ws:connection:{socketId}
ws:user:{userId}:sockets
ws:delivery:{deliveryId}:subscribers
ws:socket:{socketId}:subscriptions
ws:node:{nodeId}:connections
ws:presence:{userId}
ws:heartbeat:{socketId}
ws:rate:{userId}
ws:idempotency:{commandId}
```

Driver location:

```text
driver:location:{driverId}
drivers:locations
```

The Realtime Service may read location state, but Driver Service remains the business owner.

---

# 19. Redis Sets

For user connections:

```text
SADD ws:user:{userId}:sockets {socketId}
```

For delivery subscriptions:

```text
SADD ws:delivery:{deliveryId}:subscribers {socketId}
```

For reverse subscription lookup:

```text
SADD ws:socket:{socketId}:subscriptions {deliveryId}
```

The reverse index is important for cleanup during disconnect.

---

# 20. Redis TTL

Connection metadata must have expiration protection.

Example:

```text
ws:connection:{socketId}
TTL = 60 seconds
```

Heartbeat refreshes TTL.

This protects against stale state when a process crashes without executing disconnect cleanup.

The system should still perform explicit cleanup during normal disconnect.

---

# 21. Presence

Presence is ephemeral.

Example:

```text
ONLINE
IDLE
OFFLINE
```

Redis:

```text
ws:presence:{userId}
```

Presence events:

```text
driver.presence.online
driver.presence.offline
driver.presence.idle
```

Presence is not a durable business record.

---

# 22. Redis Pub/Sub

Redis Pub/Sub is a valid realtime fan-out mechanism.

Example:

```text
Node A
  |
  | PUBLISH realtime:delivery:X
  v
Redis
  |
  +------ Node B
  +------ Node C
  +------ Node D
```

Redis Pub/Sub provides simple broadcast semantics.

Important limitation:

```text
Subscriber offline
      |
      v
Message missed
```

Redis Pub/Sub does not provide durable replay.

Therefore it is appropriate for transient broadcast, but not for durable business events.

In this architecture, NATS Core is the primary internal realtime transport. Redis Pub/Sub is documented as an alternative, not an additional required messaging layer.

---

# 23. Why NATS Core Is Primary

NATS Core is used for:

- low-latency transient events
- cross-instance realtime fan-out
- driver location updates
- presence updates
- realtime routing signals

Example:

```text
Driver
   |
WebSocket
   |
Realtime Node A
   |
NATS
   |
+--+---------+
|            |
Node B       Node C
|            |
Customer     Admin
```

NATS is intentionally separated from Kafka.

---

# 24. NATS Core vs Kafka

| Concern | NATS Core | Kafka |
|---|---|---|
| Primary use | Realtime transient messaging | Durable business events |
| Latency | Very low | Low |
| Replay | No | Yes |
| Long-term retention | No | Yes |
| Event history | No | Yes |
| Location stream | Yes | Usually no |
| Delivery lifecycle | Usually Kafka | Yes |
| Cross-instance fan-out | Yes | Possible but unnecessary |
| Consumer groups | Queue groups | Consumer groups |
| Operational role | Realtime transport | Event backbone |

Rule:

```text
If losing the event is acceptable:
    NATS Core

If the event must be retained/replayed:
    Kafka
```

---

# 25. NATS JetStream

JetStream is the persistence and streaming layer built into NATS.

It provides:

- persistence
- replay
- durable consumers
- acknowledgements
- consumer state
- retention policies

Example:

```text
Publisher
   |
NATS Subject
   |
JetStream Stream
   |
Durable Consumer
   |
Worker
```

JetStream should not automatically replace Kafka.

---

# 26. When to Use JetStream in This Project

Potential uses:

```text
realtime.command.audit
realtime.delivery.control
driver.command.retry
operational.realtime.events
```

JetStream can be useful when:

- the message belongs to the realtime domain
- replay is useful
- the message does not need to become a platform-wide business event
- NATS is already the operational transport
- durable consumption is required

For very high-frequency driver location updates, storing every location update in JetStream may create unnecessary storage and processing overhead.

Therefore:

```text
Location updates
    -> NATS Core

Durable business events
    -> Kafka

Selected durable NATS-native events
    -> JetStream
```

---

# 27. NATS JetStream Consumer Strategy

When JetStream is used for processing workloads, prefer durable consumers.

A durable consumer retains its progress and can recover after failures.

For scalable processing:

```text
JetStream Stream
      |
Pull Consumer
      |
+-----+-----+
|           |
Worker A    Worker B
```

Use acknowledgements.

Failure:

```text
Worker receives
     |
     X processing fails
     |
NACK / timeout
     |
redelivery
```

Configure a maximum delivery policy and route permanently failing messages to an operational failure path.

---

# 28. Kafka in the Realtime Service

Kafka is not the transport for every realtime message.

Realtime consumes Kafka when the event is a durable domain event that must eventually reach connected clients.

Examples:

```text
delivery.created
delivery.driver.assigned
delivery.driver.accepted
delivery.pickup.started
delivery.picked_up
delivery.in_transit
delivery.completed
delivery.cancelled
payment.completed
payment.failed
```

The Realtime Service can subscribe to the relevant Kafka topics using its own consumer group:

```text
realtime-service-group
```

---

# 29. Kafka Event Flow

```text
Delivery Service
      |
Transactional Outbox
      |
      v
    Kafka
      |
      +------------------+
      |                  |
      v                  v
Notification         Realtime
Service              Service
      |                  |
   BullMQ             NATS
                         |
                    WebSocket
```

This preserves separation:

- Kafka = durable source event
- NATS = realtime transport
- WebSocket = client delivery

---

# 30. Why Not Send Kafka Directly to WebSocket Clients?

Because Kafka is an internal durable event system.

Clients should not know:

- Kafka brokers
- partitions
- consumer groups
- offsets
- topics
- internal schemas

The Realtime Service converts domain events into client-safe realtime messages.

---

# 31. Event Transformation

Internal event:

```json
{
  "eventId": "uuid",
  "eventType": "delivery.status.updated",
  "deliveryId": "uuid",
  "customerId": "uuid",
  "driverId": "uuid",
  "status": "IN_TRANSIT",
  "occurredAt": "2026-08-16T12:00:00Z"
}
```

Client event:

```json
{
  "type": "DELIVERY_STATUS_UPDATED",
  "deliveryId": "uuid",
  "status": "IN_TRANSIT",
  "occurredAt": "2026-08-16T12:00:00Z"
}
```

Never expose internal infrastructure metadata to clients.

---

# 32. Event Envelope

Recommended internal event envelope:

```typescript
interface RealtimeEventEnvelope<T> {
  eventId: string;
  eventType: string;
  version: number;
  occurredAt: string;
  producer: string;
  correlationId: string;
  causationId?: string;
  payload: T;
}
```

---

# 33. Client Message Envelope

```typescript
interface RealtimeMessage<T> {
  messageId: string;
  type: string;
  version: number;
  timestamp: string;
  data: T;
}
```

Example:

```json
{
  "messageId": "msg-123",
  "type": "DELIVERY_LOCATION_UPDATED",
  "version": 1,
  "timestamp": "2026-08-16T12:00:00Z",
  "data": {
    "deliveryId": "delivery-123",
    "latitude": 31.20,
    "longitude": 31.80
  }
}
```

---

# 34. WebSocket Message Types

## Client -> Server

```text
AUTH
SUBSCRIBE_DELIVERY
UNSUBSCRIBE_DELIVERY
LOCATION_UPDATE
ACCEPT_ASSIGNMENT
REJECT_ASSIGNMENT
START_PICKUP
MARK_PICKED_UP
START_TRANSIT
COMPLETE_DELIVERY
PING
ACK
```

## Server -> Client

```text
CONNECTED
SUBSCRIBED
UNSUBSCRIBED
DELIVERY_LOCATION_UPDATED
DELIVERY_STATUS_UPDATED
DRIVER_ASSIGNED
DRIVER_PRESENCE_UPDATED
DELIVERY_COMPLETED
NOTIFICATION_RECEIVED
PONG
ERROR
```

---

# 35. Subscription Model

Customer:

```text
SUBSCRIBE_DELIVERY
{
  deliveryId
}
```

Realtime Service:

```text
authenticate
    |
authorize
    |
validate delivery participation
    |
register subscription
    |
ACK
```

Redis:

```text
ws:delivery:{deliveryId}:subscribers
```

---

# 36. Unsubscription

```text
Client
   |
UNSUBSCRIBE_DELIVERY
   |
Realtime
   |
Redis SREM
   |
ACK
```

Disconnect cleanup must perform the same operation automatically.

---

# 37. Driver Location Flow

```text
Driver
  |
  | LOCATION_UPDATE
  v
Realtime Node A
  |
  +-- authenticate
  +-- authorize
  +-- validate payload
  +-- rate limit
  +-- timestamp
  |
  +-- Redis current location
  |
  +-- NATS publish
  |
  v
NATS
  |
  +----------+----------+
  |          |          |
 Node A    Node B     Node C
  |          |          |
  v          v          v
Clients subscribed to delivery
```

---

# 38. Location Update Payload

```json
{
  "type": "LOCATION_UPDATE",
  "deliveryId": "delivery-123",
  "latitude": 31.2001,
  "longitude": 31.8012,
  "accuracy": 8.4,
  "speed": 34.5,
  "heading": 90,
  "timestamp": "2026-08-16T12:00:00Z"
}
```

The server must validate:

- latitude range
- longitude range
- numeric values
- timestamp
- delivery ID
- driver assignment
- acceptable update frequency

---

# 39. Location Frequency

Do not allow clients to publish unlimited location updates.

Example policy:

```text
Driver:
    target 1-2 updates/sec

Maximum:
    configurable per driver/device

Burst:
    small controlled burst

Violation:
    rate-limit / drop / disconnect depending on severity
```

The exact limit must be configurable.

---

# 40. Location Optimization

The system should avoid broadcasting every update to every client when unnecessary.

Possible optimizations:

- minimum distance threshold
- minimum time interval
- server-side throttling
- batching
- coalescing
- latest-value-wins
- adaptive frequency

Example:

```text
Driver sends:
10 updates/sec

System broadcasts:
2 updates/sec

Latest position is preferred.
```

This prevents unnecessary network and CPU usage.

---

# 41. Location State vs Location History

Realtime Service may keep the current location:

```text
driver:location:{driverId}
```

This is ephemeral state.

Historical location belongs elsewhere if required.

Do not turn Redis into a permanent location database.

Possible future analytics path:

```text
Driver
  |
Realtime
  |
Kafka / selected stream
  |
Analytics
  |
ClickHouse
```

Only do this if location history is a real requirement.

---

# 42. Delivery Status Flow

Durable event:

```text
Delivery Service
      |
delivery.in_transit
      |
Kafka
      |
Realtime Consumer
      |
NATS
      |
WebSocket
      |
Customer
```

Client receives:

```json
{
  "type": "DELIVERY_STATUS_UPDATED",
  "data": {
    "deliveryId": "delivery-123",
    "status": "IN_TRANSIT"
  }
}
```

---

# 43. Driver Assignment Flow

```text
Driver Service
      |
driver.assignment.updated
      |
NATS
      |
Realtime Service
      |
Driver WebSocket
      |
DRIVER_ASSIGNED
```

For durable business tracking:

```text
Driver Service
      |
delivery.driver.assigned
      |
Kafka
```

NATS and Kafka may both carry related information because their purposes are different.

---

# 44. Driver Command Flow

```text
Customer/Admin/Delivery Service
          |
          v
Domain Command
          |
          v
Realtime Service
          |
          v
NATS
          |
          v
Driver Realtime Connection
```

The Realtime Service should not silently invent business state.

For example:

```text
ACCEPT_ASSIGNMENT
```

is a command.

The Driver Service remains responsible for deciding whether the acceptance is valid.

---

# 45. Command vs Event

Command:

```text
ACCEPT_ASSIGNMENT
```

Means:

> Please perform this action.

Event:

```text
delivery.driver.accepted
```

Means:

> This action successfully happened.

The distinction must remain explicit.

---

# 46. Realtime Notification vs Notification Service

These are different.

Realtime Service:

```text
Immediate UI update
```

Notification Service:

```text
Email
Push
SMS
In-app persistent notification
```

Example:

```text
Delivery Completed
       |
       +---- Kafka
              |
      +-------+--------+
      |                |
      v                v
Realtime           Notification
      |                |
 WebSocket         BullMQ Worker
      |                |
   Browser       Email/Push/SMS
```

---

# 47. BullMQ Role

BullMQ is not a WebSocket transport.

Do not use:

```text
WebSocket
   |
BullMQ
   |
Client
```

BullMQ is for deferred/retryable work.

Potential Realtime use cases:

```text
realtime.connection.cleanup
realtime.delivery.snapshot
realtime.offline-message-preparation
realtime.audit-export
realtime.reconciliation
```

These are optional.

If no background job is required by the Realtime Service, do not add BullMQ merely to use it.

The Notification Service remains the primary BullMQ user.

---

# 48. Redis Rate Limiting

Rate limit dimensions:

```text
Per IP
Per user
Per connection
Per driver
Per delivery
Per message type
```

Examples:

```text
connection attempts
subscribe requests
location updates
commands
```

Redis can implement token bucket or sliding-window style controls.

---

# 49. Backpressure

The service must protect itself when clients cannot consume messages fast enough.

Possible strategy:

```text
Incoming events
      |
      v
Per-connection buffer
      |
      +---- healthy -> send
      |
      +---- slow -> coalesce
      |
      +---- very slow -> drop non-critical updates
      |
      +---- abusive -> disconnect
```

For location updates:

```text
latest location wins
```

For critical events:

```text
DELIVERY_COMPLETED
PAYMENT_STATUS
DRIVER_ASSIGNED
```

do not treat them like disposable location updates.

---

# 50. Message Priority

Suggested classes:

## Critical

```text
DELIVERY_COMPLETED
DELIVERY_CANCELLED
DRIVER_ASSIGNED
PAYMENT_STATUS_CHANGED
```

## Normal

```text
DELIVERY_STATUS_UPDATED
DRIVER_PRESENCE_UPDATED
```

## High-frequency / lossy

```text
DRIVER_LOCATION_UPDATED
```

The system may drop intermediate location updates but should not casually drop critical state transitions.

---

# 51. Heartbeat

WebSocket connections require liveness detection.

```text
Server -> PING
Client -> PONG
```

or:

```text
Client -> PING
Server -> PONG
```

Track:

```text
lastSeenAt
```

If heartbeat timeout expires:

```text
close socket
cleanup Redis
emit presence offline
```

---

# 52. Reconnection

Clients must reconnect automatically.

Recommended strategy:

```text
disconnect
   |
wait 1s
   |
retry
   |
2s
   |
4s
   |
8s
   |
...
```

Use exponential backoff with jitter.

Do not reconnect all clients simultaneously after a regional/network outage.

---

# 53. Reconnection and Missed Events

WebSocket itself is not a durable event store.

After reconnect, the client may need a state refresh.

Recommended pattern:

```text
Reconnect
   |
Authenticate
   |
Resubscribe
   |
Request current state
   |
Continue realtime stream
```

Current state may be retrieved through GraphQL.

Example:

```text
WebSocket reconnect
        |
        v
GraphQL getDelivery(deliveryId)
        |
        v
Current authoritative state
```

This is better than trying to replay every transient message.

---

# 54. Snapshot + Stream Pattern

Use:

```text
Snapshot
   +
Realtime Stream
```

Example:

```text
1. Client queries delivery current state
2. Client opens WebSocket
3. Client subscribes
4. Client receives future updates
```

This handles missed transient events.

---

# 55. Ordering

Ordering requirements differ by event type.

Location:

```text
latest timestamp wins
```

Status:

```text
state transition order matters
```

Critical events should include:

```text
sequence
version
occurredAt
```

Example:

```json
{
  "deliveryId": "123",
  "status": "IN_TRANSIT",
  "version": 7
}
```

Client can ignore stale versions:

```text
incoming version <= current version
    -> ignore
```

---

# 56. Duplicate Events

At-least-once systems can produce duplicate events.

Use:

```text
eventId
```

For important events:

```text
Redis SET realtime:processed:{eventId} 1 NX EX 86400
```

If key already exists:

```text
duplicate -> ignore
```

Do not use idempotency storage for every high-frequency location message unless required.

---

# 57. NATS Failure

If NATS is unavailable:

```text
Realtime node
    |
    X NATS
```

The node should:

- stop publishing dependent fan-out events
- keep local connections if safe
- expose degraded health
- reconnect to NATS
- avoid unbounded in-memory buffering
- drop stale transient location updates when necessary

Do not buffer unlimited location traffic in RAM.

---

# 58. Redis Failure

Redis failure affects:

- distributed connection metadata
- subscription lookup
- rate limiting
- presence
- ephemeral state

The service must define degraded behavior.

Possible strategy:

```text
Redis unavailable
     |
     +-- local connection still exists
     +-- local subscriptions can continue temporarily
     +-- cross-node routing may degrade
     +-- rate limiting falls back conservatively
     +-- mark service degraded
```

Never claim full distributed correctness while Redis is unavailable.

---

# 59. Kafka Failure

If Kafka is unavailable:

```text
Kafka consumer
     |
     X
```

The Realtime Service should not invent durable events.

Once Kafka recovers, consumer processing resumes.

For purely transient location traffic, continue using NATS if available.

This separation prevents Kafka outages from necessarily stopping live location streaming.

---

# 60. WebSocket Node Failure

If Realtime Node A crashes:

```text
Clients on Node A
       |
       X
```

Clients reconnect.

They then connect to:

```text
Node B
```

After authentication:

```text
resubscribe
    |
snapshot
    |
continue stream
```

Redis stale state expires or is cleaned.

---

# 61. NATS Node Failure

With a properly configured NATS deployment, clients should reconnect.

The Realtime Service should use:

- connection retry
- reconnect callbacks
- health monitoring
- bounded reconnect intervals
- metrics

---

# 62. Graceful Shutdown

Before a pod terminates:

```text
SIGTERM
  |
  +-- stop accepting new connections
  |
  +-- mark instance draining
  |
  +-- stop consuming new messages
  |
  +-- notify clients if appropriate
  |
  +-- close sockets gracefully
  |
  +-- cleanup Redis
  |
  +-- flush safe telemetry
  |
  +-- close NATS/Kafka connections
  |
  v
EXIT
```

Kubernetes termination grace period must be long enough for this process.

---

# 63. Kubernetes Scaling

Realtime is connection-oriented.

HPA should consider more than CPU.

Useful metrics:

```text
active_websocket_connections
connections_per_pod
messages_per_second
event_loop_lag
outbound_queue_size
NATS_publish_latency
NATS_consumer_lag
Redis_latency
```

A pod with 10,000 connections and a pod with 100 connections are not equivalent even if CPU is similar.

---

# 64. Load Balancing

For WebSocket:

```text
Client
  |
Load Balancer
  |
Realtime Pod
```

The system must support long-lived connections.

Possible requirements:

- WebSocket upgrade support
- appropriate idle timeout
- connection draining
- health checks
- TLS termination
- optionally sticky routing

The architecture should not rely on sticky sessions for correctness because shared routing metadata is externalized.

---

# 65. Redis Adapter vs NATS

NestJS can integrate Redis-based adapters for multi-instance WebSocket broadcasting.

However, this project intentionally uses NATS as the primary internal realtime event bus.

Therefore:

```text
WebSocket
    |
NestJS
    |
NATS
    |
other Realtime nodes
```

Redis remains the state/coordination layer.

Do not add Redis Adapter + NATS + Redis Pub/Sub simultaneously unless a concrete requirement justifies the extra complexity.

---

# 66. Why Not Use Redis Pub/Sub and NATS Together?

Both can perform transient fan-out.

Using both for the exact same responsibility creates:

- duplicate routing paths
- confusing failure semantics
- more operational complexity
- harder debugging

Decision:

```text
Primary realtime transport = NATS Core

Redis = state/coordination

Redis Pub/Sub = documented alternative, not required
```

---

# 67. Why Not Use JetStream for Everything?

JetStream provides persistence and replay, but persistent streaming has additional storage and operational cost.

High-frequency location:

```text
5,000 updates/sec
```

does not automatically need durable storage.

If every location message is persisted:

```text
5,000/sec
x
60
x
60
x
24
```

produces a very large stream.

Therefore:

```text
Realtime transient location -> NATS Core
Durable business events -> Kafka
Selected durable NATS workloads -> JetStream
```

---

# 68. Capacity Target

Initial architecture target:

```text
100K users
10K active drivers
20K concurrent WebSockets
50K deliveries/day
5K deliveries/hour peak
5K location updates/sec
```

These are design targets, not production guarantees.

The service must be load-tested against the actual deployment environment.

---

# 69. Rough Realtime Capacity Model

Assume:

```text
20,000 concurrent sockets
```

If average:

```text
1 outbound message/sec
```

then:

```text
20,000 messages/sec
```

At high location frequency, the system must reduce unnecessary fan-out through:

- throttling
- coalescing
- subscription filtering
- latest-value-wins

---

# 70. Fan-Out Complexity

Without filtering:

```text
N events
x
M clients
```

can become expensive.

Instead:

```text
event deliveryId = X
        |
        v
Redis subscribers for X
        |
        v
Only relevant sockets
```

This keeps fan-out proportional to subscribers.

---

# 71. Realtime Rooms / Channels

Logical channels:

```text
delivery:{deliveryId}
user:{userId}
driver:{driverId}
admin:operations
```

A connection may subscribe to multiple channels.

Example:

```text
Driver
  |
  +-- driver:{driverId}
  +-- delivery:{deliveryId}

Customer
  |
  +-- user:{userId}
  +-- delivery:{deliveryId}
```

Authorization must be checked before joining a channel.

---

# 72. Security Rules

Never trust:

```text
userId from client
role from client
driverId from client
```

Use:

```text
JWT -> identity
Domain service -> authorization truth
```

Validate every inbound command.

Apply:

- payload schema validation
- max message size
- rate limits
- connection limits
- origin policy where applicable
- TLS
- token expiry
- token revocation strategy where required
- audit logging for sensitive commands

---

# 73. Maximum Message Size

Set an explicit WebSocket message size.

Example:

```text
max payload = 16 KB
```

The exact value is configurable.

Realtime location payloads should remain very small.

Reject oversized payloads before parsing expensive data.

---

# 74. Input Validation

Every message must pass schema validation.

Example:

```typescript
LocationUpdate {
  deliveryId: UUID
  latitude: number
  longitude: number
  timestamp: ISODate
}
```

Reject:

```text
NaN
Infinity
invalid UUID
latitude > 90
latitude < -90
longitude > 180
longitude < -180
unexpected fields
oversized payload
```

---

# 75. Service Modules

Recommended NestJS modules:

```text
src/
├── main.ts
├── app.module.ts
│
├── config/
│   ├── configuration.ts
│   └── validation.ts
│
├── websocket/
│   ├── websocket.gateway.ts
│   ├── websocket.adapter.ts
│   ├── websocket-auth.guard.ts
│   ├── websocket-exception.filter.ts
│   └── websocket.interceptor.ts
│
├── connection/
│   ├── connection.service.ts
│   ├── connection.registry.ts
│   └── connection.types.ts
│
├── subscription/
│   ├── subscription.service.ts
│   ├── subscription.repository.ts
│   └── subscription.types.ts
│
├── authorization/
│   ├── realtime-authorization.service.ts
│   └── policies/
│
├── messaging/
│   ├── nats/
│   │   ├── nats.client.ts
│   │   ├── nats.publisher.ts
│   │   └── nats.subscriber.ts
│   │
│   ├── kafka/
│   │   ├── kafka.client.ts
│   │   ├── kafka.consumer.ts
│   │   └── event-handlers/
│   │
│   └── jetstream/
│       ├── jetstream.client.ts
│       └── consumers/
│
├── redis/
│   ├── redis.client.ts
│   ├── connection-state.store.ts
│   ├── subscription.store.ts
│   ├── presence.store.ts
│   ├── rate-limit.store.ts
│   └── idempotency.store.ts
│
├── location/
│   ├── location.service.ts
│   ├── location.validator.ts
│   └── location-throttler.ts
│
├── events/
│   ├── event.mapper.ts
│   ├── event.types.ts
│   └── event-deduplicator.ts
│
├── heartbeat/
│   └── heartbeat.service.ts
│
├── rate-limit/
│   └── realtime-rate-limiter.service.ts
│
├── jobs/
│   ├── realtime.processor.ts
│   └── queues.ts
│
├── health/
│   ├── health.controller.ts
│   └── health.service.ts
│
├── observability/
│   ├── metrics.service.ts
│   ├── tracing.ts
│   └── logging.ts
│
└── common/
    ├── constants.ts
    ├── enums.ts
    └── errors.ts
```

There should be no REST API just because a health module exists. Infrastructure health endpoints may be exposed separately for Kubernetes probes if required by deployment policy; they are not part of the public application API.

---

# 76. Detailed Gateway Responsibilities

The WebSocket Gateway should remain thin.

It handles:

```text
connect
disconnect
message routing
validation
calling application services
sending responses
```

It should not contain business logic.

Bad:

```typescript
gateway.handleLocation() {
  // 200 lines of business rules
}
```

Correct:

```typescript
gateway.handleLocation(payload) {
  return this.locationService.handle(payload);
}
```

---

# 77. Connection Service

Responsibilities:

- register socket
- unregister socket
- map socket -> user
- map user -> sockets
- track node ownership
- update heartbeat
- expose connection metrics

Methods:

```text
register()
unregister()
getConnection()
getUserConnections()
refreshHeartbeat()
getConnectionCount()
```

---

# 78. Subscription Service

Responsibilities:

- subscribe
- unsubscribe
- authorization
- cleanup
- lookup subscribers

Methods:

```text
subscribeToDelivery()
unsubscribeFromDelivery()
getDeliverySubscribers()
removeSocketFromAllSubscriptions()
```

---

# 79. Location Service

Responsibilities:

- validate location
- authorize driver
- throttle
- update current location
- publish NATS event
- optionally emit metrics

Flow:

```text
Gateway
  |
LocationService
  |
Validate
  |
Authorize
  |
Rate Limit
  |
Redis
  |
NATS
```

---

# 80. NATS Subjects

Suggested subjects:

```text
realtime.location.driver.updated
realtime.delivery.location.updated
realtime.delivery.status.updated
realtime.driver.assignment.updated
realtime.driver.presence.updated
realtime.command.driver
realtime.command.delivery
```

Domain-level subjects from the existing architecture may also be used:

```text
delivery.location.updated
delivery.status.updated
driver.assignment.updated
driver.presence.updated
```

Choose one naming convention and keep it consistent.

---

# 81. Kafka Topics

The Realtime Service consumes only topics it needs.

Example:

```text
delivery.created
delivery.driver.assigned
delivery.driver.accepted
delivery.pickup.started
delivery.picked_up
delivery.in_transit
delivery.completed
delivery.cancelled
payment.completed
payment.failed
```

Consumer group:

```text
realtime-service-group
```

---

# 82. Kafka Consumer Responsibilities

For every event:

```text
receive
  |
deserialize
  |
validate schema
  |
deduplicate when required
  |
authorize routing
  |
map to client event
  |
publish NATS
  |
commit offset
```

Do not commit the Kafka offset before the event is safely handed to the internal processing path.

---

# 83. Kafka Retry Strategy

Transient failures:

```text
retry
```

Persistent failures:

```text
DLQ
```

Example:

```text
realtime.delivery.completed.dlq
```

However, if the message is only a client update and the authoritative state can be fetched after reconnect, the recovery strategy may be:

```text
log
metric
retry
snapshot recovery
```

rather than indefinitely buffering stale realtime messages.

---

# 84. DLQ

DLQ record should contain:

```json
{
  "eventId": "uuid",
  "eventType": "delivery.completed",
  "failedAt": "timestamp",
  "attempts": 5,
  "service": "realtime-service",
  "errorCode": "MAPPING_ERROR",
  "errorMessage": "...",
  "originalPayload": {}
}
```

DLQ must support:

- alerting
- inspection
- replay
- manual recovery

---

# 85. Idempotency Strategy

Critical commands:

```text
ACCEPT_ASSIGNMENT
REJECT_ASSIGNMENT
COMPLETE_DELIVERY
```

may carry:

```text
commandId
```

Store:

```text
realtime:idempotency:{commandId}
```

The Realtime Service protects against duplicate delivery of the same command to the domain service.

The domain service must still enforce its own business-level idempotency.

---

# 86. Realtime Service Is Not the Final Authority

For example:

```text
Driver sends:
ACCEPT_ASSIGNMENT
```

Realtime Service:

```text
authenticate
authorize
forward command
```

Driver Service:

```text
check assignment state
check driver state
apply transition
persist
publish accepted event
```

This prevents the Realtime Service from becoming a business-state authority.

---

# 87. Command Routing

Example:

```text
Driver
 |
WebSocket
 |
Realtime
 |
NATS
 |
Driver Service
 |
MongoDB
 |
Kafka
 |
Realtime
 |
WebSocket
 |
Customer
```

This creates a clean command/event distinction.

---

# 88. Complete Location Flow

```text
STEP 1
Driver sends LOCATION_UPDATE

STEP 2
Realtime validates JWT context

STEP 3
Realtime validates payload

STEP 4
Realtime checks rate limit

STEP 5
Realtime validates driver-delivery relationship

STEP 6
Realtime updates current location in Redis

STEP 7
Realtime publishes:
delivery.location.updated

STEP 8
Other Realtime instances receive NATS message

STEP 9
Each instance checks local subscribers

STEP 10
Only relevant sockets receive the update

STEP 11
Metrics are recorded

STEP 12
Client renders latest location
```

---

# 89. Complete Delivery Status Flow

```text
Delivery Service
      |
      | Transaction
      | Delivery state + Outbox
      v
    Kafka
      |
      v
Realtime Kafka Consumer
      |
      v
Event validation
      |
      v
Event mapping
      |
      v
NATS
      |
      v
Realtime instances
      |
      v
Subscribed sockets
      |
      v
Customer / Driver / Admin
```

---

# 90. Complete Driver Assignment Flow

```text
Delivery Service
      |
      | gRPC
      v
Driver Service
      |
      | reserve driver
      |
      +-- Redis Lock
      +-- MongoDB
      |
      v
Assignment Created
      |
      +-- NATS realtime event
      |
      +-- Kafka durable business event
```

Realtime:

```text
NATS
  |
Realtime
  |
Driver socket
  |
DELIVERY_ASSIGNED
```

Kafka:

```text
Kafka
  |
Notification
  |
Analytics
  |
Realtime
```

---

# 91. Complete Customer Tracking Flow

```text
Customer
   |
WebSocket
   |
subscribe(deliveryId)
   |
Realtime
   |
Authorization
   |
Redis subscription
   |
CONNECTED TO DELIVERY STREAM
```

Then:

```text
Driver
   |
LOCATION_UPDATE
   |
Realtime
   |
NATS
   |
Realtime Nodes
   |
Redis subscriber lookup
   |
Customer socket
```

---

# 92. Complete Reconnect Flow

```text
Customer loses connection
        |
        v
WebSocket CLOSED
        |
        v
Client exponential backoff
        |
        v
Reconnect
        |
        v
JWT authentication
        |
        v
Restore subscriptions
        |
        v
GraphQL snapshot
        |
        v
Realtime stream resumes
```

---

# 93. Offline State Recovery

Do not try to reconstruct every transient location message.

Instead:

```text
Current authoritative state
+
future realtime events
```

Example:

```text
GraphQL:
delivery(id)
```

returns:

```text
status = IN_TRANSIT
current driver location = ...
```

Then WebSocket delivers future changes.

---

# 94. Ordering Model

Location:

```text
latest timestamp wins
```

Delivery status:

```text
monotonic state version
```

Commands:

```text
commandId + domain validation
```

Events:

```text
eventId + version
```

This prevents stale events from overwriting newer state.

---

# 95. Event Versioning

Every client event should include a version.

```json
{
  "type": "DELIVERY_STATUS_UPDATED",
  "version": 3,
  "data": {
    "deliveryId": "123",
    "status": "PICKED_UP"
  }
}
```

When the protocol changes:

```text
version 1
version 2
```

Maintain backward compatibility where practical.

---

# 96. Error Protocol

All WebSocket errors use a consistent format.

```json
{
  "type": "ERROR",
  "requestId": "req-123",
  "code": "FORBIDDEN",
  "message": "You cannot subscribe to this delivery.",
  "retryable": false
}
```

Possible codes:

```text
UNAUTHENTICATED
FORBIDDEN
INVALID_MESSAGE
INVALID_DELIVERY_ID
RATE_LIMITED
TOO_LARGE
NOT_FOUND
STALE_COMMAND
INTERNAL_ERROR
SERVICE_UNAVAILABLE
```

Do not expose stack traces.

---

# 97. Request IDs

Every inbound command should have:

```text
requestId
```

Example:

```json
{
  "requestId": "req-123",
  "type": "ACCEPT_ASSIGNMENT",
  "data": {}
}
```

Use it for tracing and debugging.

---

# 98. Correlation IDs

The service should propagate:

```text
traceId
correlationId
causationId
requestId
eventId
```

Example:

```text
GraphQL Request
      |
correlationId = C1
      |
Delivery Service
      |
Kafka event
      |
Realtime
      |
WebSocket message
```

This makes an end-to-end trace possible.

---

# 99. Observability

## Metrics

Expose:

```text
active_connections
connections_opened_total
connections_closed_total
connection_auth_failures_total
subscriptions_total
active_subscriptions
messages_received_total
messages_sent_total
messages_dropped_total
location_updates_total
location_updates_throttled_total
nats_publish_latency
nats_receive_latency
redis_latency
kafka_consumer_lag
websocket_send_queue_size
websocket_errors_total
reconnects_total
```

---

# 100. Business-Level Realtime Metrics

Useful metrics:

```text
average location propagation latency
p95 location propagation latency
p99 location propagation latency
delivery status propagation latency
driver assignment delivery latency
WebSocket connection success rate
subscription authorization failure rate
```

Example:

```text
Driver location timestamp
       |
       v
Customer received timestamp
       |
       v
propagation latency
```

---

# 101. Distributed Tracing

Trace:

```text
Driver WebSocket
    |
Realtime
    |
NATS
    |
Realtime Node
    |
Customer WebSocket
```

For durable events:

```text
Delivery
    |
Outbox
    |
Kafka
    |
Realtime Consumer
    |
NATS
    |
WebSocket
```

OpenTelemetry should propagate trace context where supported.

---

# 102. Structured Logging

Every important log should contain:

```json
{
  "timestamp": "...",
  "level": "INFO",
  "service": "realtime-service",
  "instanceId": "realtime-7c8",
  "traceId": "...",
  "correlationId": "...",
  "requestId": "...",
  "userId": "...",
  "socketId": "...",
  "deliveryId": "...",
  "eventId": "...",
  "eventType": "...",
  "message": "..."
}
```

Never log JWTs or secrets.

---

# 103. Health Checks

Health should distinguish:

```text
Liveness
Readiness
Degraded
```

Dependencies:

```text
Redis
NATS
Kafka
```

A temporary Kafka outage should not necessarily make location streaming unavailable if NATS and WebSocket are healthy.

Therefore dependency health should be classified by criticality.

---

# 104. Failure Matrix

| Failure | Impact | Strategy |
|---|---|---|
| Client disconnect | One connection | Reconnect |
| Redis unavailable | Distributed routing degraded | Local fallback + degraded state |
| NATS unavailable | Cross-node realtime degraded | Reconnect + bounded buffering |
| Kafka unavailable | Durable event consumption delayed | Retry consumer |
| Slow client | Memory pressure | Backpressure/coalescing |
| Duplicate event | Duplicate update | eventId/version |
| Node crash | Connected clients lost | Reconnect |
| Invalid JWT | Connection rejected | No upgrade |
| Unauthorized subscription | Subscription rejected | Error |
| Message flood | Resource exhaustion | Rate limit |
| Malformed payload | Invalid command | Validation |
| Kafka poison event | Consumer blocked | Retry + DLQ |
| Redis stale key | Routing inconsistency | TTL + cleanup |

---

# 105. Circuit Breaker

Circuit breaker may be used for synchronous calls from Realtime to other services.

However, the Realtime Service should minimize synchronous dependencies.

If it must call a domain service:

```text
Realtime
   |
Circuit Breaker
   |
gRPC
   |
Domain Service
```

States:

```text
CLOSED
OPEN
HALF_OPEN
```

---

# 106. Timeouts

Every synchronous operation must define a timeout.

Example:

```text
authorization check: 300ms
snapshot lookup: 500ms
gRPC command: 500ms
```

Values must be configured according to real measurements.

Never allow an unbounded network call from a WebSocket handler.

---

# 107. Retry Rules

Do not blindly retry all realtime operations.

Safe:

```text
connect to NATS
connect to Redis
connect to Kafka
```

Potentially unsafe:

```text
ACCEPT_ASSIGNMENT
COMPLETE_DELIVERY
```

Commands require idempotency before retries.

---

# 108. BullMQ Failure Handling

If BullMQ is used:

```text
Job
 |
Worker
 |
Success -> complete
 |
Failure
 |
retry + exponential backoff
 |
max attempts
 |
DLQ / failed queue
```

Use BullMQ only for work that can be asynchronous.

Do not use BullMQ as the primary realtime event bus.

---

# 109. Realtime + Notification Integration

Example:

```text
delivery.completed
       |
      Kafka
       |
 +-----+------+
 |            |
 v            v
Realtime   Notification
 |            |
WS          BullMQ
 |            |
UI         Push/Email
```

This allows:

- instant UI updates
- durable notifications

without coupling the two services.

---

# 110. Realtime + Media Integration

For media processing:

```text
Media Service
     |
media.processing.progress
     |
Kafka/NATS depending on durability
     |
Realtime
     |
WebSocket
     |
Frontend
```

Example:

```json
{
  "type": "MEDIA_PROCESSING_PROGRESS",
  "data": {
    "fileId": "file-123",
    "stage": "TRANSCODING",
    "progress": 72
  }
}
```

High-frequency progress may use NATS.

Durable final state:

```text
media.ready
```

may use Kafka.

---

# 111. Realtime + Search

Search is not a realtime transport.

Search Service:

```text
Go
+
Elasticsearch
+
Kafka
```

Realtime can notify the UI after indexed data becomes available:

```text
Entity Updated
    |
Kafka
    |
Search Indexer
    |
Elasticsearch
    |
Search Ready
    |
NATS
    |
Realtime
    |
WebSocket
```

Only add this if the UI needs realtime search-index status.

---

# 112. Realtime + Analytics

Realtime metrics can be exported to observability infrastructure.

Business events should go to Kafka and then Analytics.

Do not make Realtime directly write ClickHouse for every socket message unless a concrete analytics requirement exists.

---

# 113. Data Ownership

Realtime owns:

```text
connection metadata
subscription metadata
presence state
temporary routing state
```

It does not own:

```text
users
drivers
deliveries
payments
notifications
media
```

---

# 114. Persistence Strategy

The Realtime Service intentionally has no primary business database.

Primary state:

```text
Redis
```

but Redis data is ephemeral.

If the service loses Redis:

```text
clients reconnect
subscriptions are recreated
state is rebuilt
```

This is a feature of the architecture.

---

# 115. What Must Not Be Stored Permanently

Do not permanently store:

```text
every WebSocket ping
every location packet
every presence update
every connection lifecycle event
```

unless analytics or auditing explicitly requires it.

---

# 116. Optional Audit Stream

If audit requirements appear:

```text
Realtime
   |
Kafka
   |
Audit Consumer
   |
ClickHouse/Object Storage
```

Keep the realtime path separate from the audit path.

---

# 117. API Gateway Relationship

The Gateway is responsible for:

```text
GraphQL
authentication
rate limiting
query complexity
federation
```

Realtime is responsible for:

```text
WebSocket authentication
connection lifecycle
realtime authorization
realtime rate limiting
```

Some responsibilities overlap intentionally because the protocols are different.

---

# 118. Authentication Shared Package

A common NestJS package may contain infrastructure such as:

```text
JWT verification helper
JWT claims types
guards
decorators
request context types
logger
tracing
Redis client abstraction
NATS abstraction
```

It must not contain business logic.

Good:

```text
JwtVerifier
```

Bad:

```text
DeliveryAuthorizationService
```

The latter belongs to the relevant service.

---

# 119. NATS Client Lifecycle

On startup:

```text
create connection
 |
authenticate
 |
connect
 |
create subscriptions
 |
mark dependency healthy
```

On shutdown:

```text
stop new subscriptions
 |
drain
 |
close
```

Use reconnect handling.

---

# 120. Kafka Client Lifecycle

On startup:

```text
connect
 |
join consumer group
 |
subscribe topics
 |
start consuming
```

On shutdown:

```text
stop consuming
 |
commit safe offsets
 |
disconnect
```

Avoid duplicate consumer instances with conflicting group configuration.

---

# 121. Redis Client Lifecycle

Use separate logical clients/connections when required by library behavior:

```text
command client
pub/sub client
```

if Redis Pub/Sub is used.

Configure:

- reconnect
- timeout
- retry strategy
- connection pool if appropriate

---

# 122. Docker

Realtime Docker image should:

- use multi-stage build
- run as non-root
- expose only required ports
- include health support
- use production dependencies only
- use environment variables for configuration

Example:

```text
build stage
    |
compile NestJS
    |
production stage
    |
node dist/main.js
```

---

# 123. Docker Compose

Local stack:

```text
realtime-service
redis
nats
kafka
zookeeper/KRaft depending on Kafka setup
api-gateway
user-service
delivery-service
driver-service
notification-service
```

The Realtime Service must be able to run independently with mocked event publishers for unit tests.

---

# 124. Kubernetes

Required resources:

```text
Deployment
Service
ConfigMap
Secret
HPA
PodDisruptionBudget
ServiceAccount if required
NetworkPolicy where appropriate
```

---

# 125. Kubernetes Realtime Deployment

Example topology:

```text
Realtime Deployment
   replicas: 3+
        |
        +-- realtime-1
        +-- realtime-2
        +-- realtime-3
```

All instances connect to:

```text
Redis
NATS
Kafka
```

---

# 126. HPA

Recommended scaling signals:

```text
CPU
Memory
active connections
messages/sec
event-loop lag
```

CPU-only scaling is insufficient for connection-heavy workloads.

---

# 127. Pod Disruption

Use a PodDisruptionBudget to prevent Kubernetes from terminating all realtime instances simultaneously.

Example concept:

```text
minAvailable: 2
```

Exact values depend on cluster size.

---

# 128. Skaffold

Skaffold should provide:

```text
source change
    |
build image
    |
deploy
    |
wait for readiness
    |
stream logs
```

Realtime development should support fast iteration without rebuilding the entire platform unnecessarily.

---

# 129. Testing Strategy

## Unit Tests

Test:

- message validation
- authorization
- connection service
- subscription service
- location throttling
- event mapping
- deduplication
- rate limiting

## Integration Tests

Test:

- Redis
- NATS
- Kafka
- WebSocket
- reconnect
- cross-instance fan-out

## E2E

Test:

```text
Driver
  |
WebSocket
  |
Realtime A
  |
NATS
  |
Realtime B
  |
Customer
```

---

# 130. Cross-Instance Test

Scenario:

```text
Customer connects to Node B
Driver connects to Node A

Driver sends location
      |
Node A
      |
NATS
      |
Node B
      |
Customer
```

Assertion:

```text
Customer receives correct location update.
```

---

# 131. Failure Tests

Simulate:

```text
kill realtime node
kill Redis
restart NATS
restart Kafka
disconnect client
flood location messages
duplicate commands
duplicate Kafka event
slow client
invalid JWT
expired JWT
unauthorized subscription
```

---

# 132. Load Testing

Measure:

```text
10K connections
20K connections
50K connections
```

Measure:

```text
connection latency
message latency
p95
p99
CPU
memory
event-loop lag
Redis latency
NATS latency
network throughput
```

---

# 133. Location Load Test

Generate:

```text
10,000 drivers
x
0.5 updates/sec
=
5,000 location updates/sec
```

Then measure:

```text
NATS throughput
Redis writes
WebSocket fan-out
CPU
memory
latency
```

---

# 134. Backpressure Load Test

Create:

```text
fast producer
slow consumer
```

Verify:

```text
memory remains bounded
location updates are coalesced
critical messages remain protected
slow clients are disconnected when necessary
```

---

# 135. Security Testing

Test:

```text
expired JWT
invalid signature
wrong role
unauthorized delivery
forged driverId
forged userId
oversized payload
message flood
connection flood
replay command
duplicate command
```

---

# 136. Operational Dashboards

Grafana dashboard should contain:

```text
Active connections
Connections by role
Connections by pod
Messages/sec
Location updates/sec
Dropped messages
NATS latency
Redis latency
Kafka lag
WebSocket latency
Auth failures
Rate limit violations
Error rate
Event loop lag
```

---

# 137. Alerting

Potential alerts:

```text
WebSocket error rate high
NATS disconnected
Redis latency high
Kafka consumer lag high
Active connections suddenly drop
Memory high
Event loop lag high
Location propagation latency high
DLQ growth high
Connection authentication failures spike
```

---

# 138. Architecture Decision Records

Create ADRs for:

```text
ADR-001 WebSocket over SSE
ADR-002 NATS Core for realtime fan-out
ADR-003 Kafka for durable business events
ADR-004 Redis for realtime state
ADR-005 JetStream as optional durable NATS layer
ADR-006 Raw WebSocket over GraphQL Subscriptions
ADR-007 No Realtime database
ADR-008 Snapshot + stream reconnect strategy
ADR-009 Location latest-value-wins
ADR-010 Horizontal scaling strategy
```

---

# 139. Decision: WebSocket

Decision:

```text
USE WebSocket
```

Reason:

```text
Bidirectional driver communication
High-frequency updates
Commands
Long-lived connection
Unified protocol
```

SSE remains an alternative for future server-only streams.

---

# 140. Decision: NATS Core

Decision:

```text
USE NATS Core
```

Reason:

```text
Low latency
Transient messages
Cross-instance fan-out
High-frequency location
```

---

# 141. Decision: Kafka

Decision:

```text
USE Kafka
```

Reason:

```text
Durable domain events
Replay
Multiple consumers
Analytics
Notification
Audit
```

---

# 142. Decision: Redis

Decision:

```text
USE Redis
```

Reason:

```text
Ephemeral connection state
Subscription state
Presence
Rate limits
Idempotency
Coordination
```

---

# 143. Decision: JetStream

Decision:

```text
OPTIONAL
```

Use only when:

```text
NATS-native durability is actually required.
```

Do not introduce it merely because it exists.

---

# 144. Decision: Redis Pub/Sub

Decision:

```text
ALTERNATIVE
```

It can replace NATS for simple Redis-centric broadcast.

It is not used as the primary transport because NATS provides a cleaner dedicated messaging role in this architecture.

---

# 145. Decision: BullMQ

Decision:

```text
USE WHERE ASYNCHRONOUS JOBS EXIST
```

It is not part of the hot realtime message path.

Notification Service is the primary BullMQ consumer.

---

# 146. Complete Technology Flow

```text
                           CLIENT
                              |
              +---------------+---------------+
              |                               |
           GraphQL                         WebSocket
              |                               |
              v                               v
       API Gateway                     Realtime Service
                                             |
                        +--------------------+-------------------+
                        |                    |                   |
                        v                    v                   v
                      Redis                NATS                Kafka
                        |                    |                   |
                        |                    |                   |
                 Connection State     Realtime Fanout     Durable Events
                 Subscriptions        Cross-node          Domain Events
                 Presence             Commands            Replay
                 Rate Limits
                        |
                        +--------------------+
                                             |
                                             v
                                    Realtime Instances
                                             |
                                  +----------+----------+
                                  |          |          |
                                  v          v          v
                                Client     Client     Client
```

---

# 147. Complete Delivery Tracking Flow

```text
                    DRIVER
                       |
                  WebSocket
                       |
                       v
                Realtime Node A
                       |
             +---------+---------+
             |                   |
             v                   v
           Redis               NATS
        current state            |
                                 v
                         Realtime Node B
                                 |
                         Redis subscribers
                                 |
                                 v
                            CUSTOMER
```

Durable business status:

```text
Delivery Service
      |
    Outbox
      |
    Kafka
      |
Realtime Consumer
      |
    NATS
      |
WebSocket
```

---

# 148. Complete Driver Command Flow

```text
Driver
  |
WebSocket
  |
Realtime
  |
NATS command
  |
Driver Service
  |
Business validation
  |
MongoDB
  |
Kafka event
  |
Realtime
  |
NATS
  |
WebSocket
```

---

# 149. Complete Realtime Failure-Recovery Flow

```text
Failure
  |
  +-- WebSocket -> reconnect
  |
  +-- Realtime pod -> new pod
  |
  +-- Redis -> rebuild ephemeral state
  |
  +-- NATS -> reconnect
  |
  +-- Kafka -> resume consumer
  |
  +-- Client -> snapshot + resubscribe
```

---

# 150. Configuration

Example environment variables:

```text
NODE_ENV
PORT
SERVICE_NAME
INSTANCE_ID

JWT_PUBLIC_KEY
JWT_ISSUER
JWT_AUDIENCE

REDIS_URL
REDIS_CONNECT_TIMEOUT
REDIS_RETRY_LIMIT

NATS_SERVERS
NATS_CLIENT_NAME
NATS_RECONNECT_TIME_WAIT

KAFKA_BROKERS
KAFKA_CLIENT_ID
KAFKA_GROUP_ID
KAFKA_SESSION_TIMEOUT

WS_MAX_PAYLOAD
WS_HEARTBEAT_INTERVAL
WS_HEARTBEAT_TIMEOUT

LOCATION_UPDATE_RATE
LOCATION_BROADCAST_RATE

RATE_LIMIT_CONNECTIONS
RATE_LIMIT_MESSAGES

OTEL_EXPORTER_OTLP_ENDPOINT
```

Secrets must not be committed.

---

# 151. Environment Profiles

## Development

```text
Docker Compose
single Redis
single NATS
single Kafka
1 Realtime instance
```

## Integration

```text
multiple Realtime instances
shared Redis
NATS cluster
Kafka
load test client
```

## Production

```text
Kubernetes
multiple Realtime replicas
HA Redis
HA NATS
Kafka cluster
HPA
PDB
observability
```

---

# 152. Recommended Implementation Order

## Phase 1 — Skeleton

```text
NestJS
Docker
configuration
health
logging
```

## Phase 2 — WebSocket

```text
Gateway
connect
disconnect
ping/pong
message validation
```

## Phase 3 — JWT

```text
handshake authentication
connection context
authorization
```

## Phase 4 — Redis

```text
connection registry
subscriptions
presence
TTL
```

## Phase 5 — NATS

```text
publisher
subscriber
cross-instance fan-out
```

## Phase 6 — Location

```text
location update
validation
rate limiting
Redis current state
NATS broadcast
```

## Phase 7 — Kafka

```text
consumer
domain events
event mapping
idempotency
DLQ
```

## Phase 8 — Commands

```text
driver commands
NATS command routing
domain-service validation
```

## Phase 9 — Reliability

```text
reconnect
backpressure
graceful shutdown
timeouts
retry
circuit breaker where needed
```

## Phase 10 — Observability

```text
metrics
tracing
dashboards
alerts
```

## Phase 11 — Kubernetes

```text
Deployment
Service
HPA
PDB
probes
resources
```

## Phase 12 — Load Testing

```text
20K sockets
5K location updates/sec
failure testing
```

---

# 153. Implementation Checklist

## WebSocket

- [ ] Gateway implemented
- [ ] JWT handshake
- [ ] Connection lifecycle
- [ ] Ping/Pong
- [ ] Max payload
- [ ] Message validation
- [ ] Error protocol
- [ ] Graceful shutdown

## Redis

- [ ] Connection registry
- [ ] User-to-socket index
- [ ] Delivery subscriptions
- [ ] Reverse subscription index
- [ ] Presence
- [ ] TTL
- [ ] Rate limiting
- [ ] Idempotency

## NATS

- [ ] Connection
- [ ] Reconnect
- [ ] Publisher
- [ ] Subscriber
- [ ] Subject naming
- [ ] Cross-instance fan-out
- [ ] Drain on shutdown

## Kafka

- [ ] Consumer
- [ ] Consumer group
- [ ] Topic configuration
- [ ] Schema validation
- [ ] Event mapping
- [ ] Idempotency
- [ ] Retry
- [ ] DLQ
- [ ] Offset handling

## JetStream

- [ ] Only if concrete durable NATS use case exists
- [ ] Stream
- [ ] Durable consumer
- [ ] Ack policy
- [ ] Max deliveries
- [ ] Replay test

## BullMQ

- [ ] Only if realtime background job exists
- [ ] Queue
- [ ] Worker
- [ ] Retry
- [ ] Backoff
- [ ] Failed jobs
- [ ] Monitoring

## Reliability

- [ ] Reconnection
- [ ] Backpressure
- [ ] Coalescing
- [ ] Rate limits
- [ ] Timeouts
- [ ] Circuit breaker
- [ ] Graceful shutdown

## Observability

- [ ] Structured logs
- [ ] Metrics
- [ ] Traces
- [ ] Correlation IDs
- [ ] Dashboards
- [ ] Alerts

## Infrastructure

- [ ] Dockerfile
- [ ] Docker Compose
- [ ] Kubernetes Deployment
- [ ] Kubernetes Service
- [ ] ConfigMap
- [ ] Secret
- [ ] HPA
- [ ] PDB
- [ ] Probes
- [ ] Skaffold

---

# 154. Architecture Validation Rules

Before considering the Realtime Service complete:

```text
[ ] No public REST API is introduced
[ ] GraphQL remains the public request/response API
[ ] WebSocket is the primary realtime protocol
[ ] SSE is documented as an alternative, not duplicated unnecessarily
[ ] Realtime owns no business database
[ ] No cross-service database access exists
[ ] JWT is validated during WebSocket authentication
[ ] Authorization is checked for subscriptions and commands
[ ] Redis stores ephemeral realtime state
[ ] Redis Pub/Sub is not duplicated with NATS without justification
[ ] NATS Core handles transient realtime fan-out
[ ] Kafka handles durable business events
[ ] JetStream is optional and justified per use case
[ ] BullMQ is not used as realtime transport
[ ] High-frequency location traffic is throttled
[ ] Slow clients cannot cause unbounded memory growth
[ ] Critical events are not treated like disposable location updates
[ ] Reconnection is supported
[ ] Snapshot recovery exists
[ ] Event IDs/versioning exist
[ ] Kafka consumers are idempotent
[ ] DLQ exists for poison events
[ ] Metrics exist
[ ] Distributed tracing exists
[ ] Structured logs exist
[ ] Graceful shutdown exists
[ ] Kubernetes scaling considers active connections
[ ] Load tests cover concurrent WebSockets
[ ] Failure tests cover Redis/NATS/Kafka failures
[ ] No business logic is placed in shared NestJS packages
```

---

# 155. Recommended Final Architecture

```text
                                      CLIENTS
                                         |
                          +--------------+--------------+
                          |                             |
                       GraphQL                       WebSocket
                          |                             |
                          v                             v
                  +---------------+             +------------------+
                  | API Gateway   |             | Realtime Service |
                  | NestJS        |             | NestJS            |
                  | Federation    |             | WebSocket         |
                  +-------+-------+             +--------+---------+
                          |                              |
                  Domain Services                         |
                                                         |
                              +--------------------------+----------------------+
                              |                          |                      |
                              v                          v                      v
                            Redis                      NATS                   Kafka
                              |                          |                      |
                       Ephemeral State              Transient              Durable
                       Connections                  Realtime               Business
                       Presence                     Fan-out                 Events
                       Subscriptions               Commands                Replay
                       Rate Limits
                              |                          |
                              +------------+-------------+
                                           |
                                           v
                                  Realtime Instances
                                           |
                              +------------+------------+
                              |            |            |
                              v            v            v
                          Customer      Driver        Admin
                          WebSocket     WebSocket     WebSocket
```

---

# 156. Final Technology Decisions

```text
Public API
    -> GraphQL Federation

Realtime client protocol
    -> WebSocket

Server-only streaming alternative
    -> SSE (not initial)

Low-latency transient messaging
    -> NATS Core

Durable NATS messaging
    -> JetStream only when justified

Durable business event backbone
    -> Kafka

Realtime shared state
    -> Redis

Redis messaging alternative
    -> Redis Pub/Sub, not primary

Background jobs
    -> BullMQ when required

Authentication
    -> JWT

Authorization
    -> Guards + domain authorization

Rate limiting
    -> Redis

Idempotency
    -> Redis + domain-service persistence where required

Observability
    -> OpenTelemetry + Prometheus + Grafana + structured logs

Containerization
    -> Docker

Local infrastructure
    -> Docker Compose

Production
    -> Kubernetes

Local Kubernetes workflow
    -> Skaffold
```

---

# 157. Final Design Philosophy

The Realtime Service should not exist to demonstrate that every technology can be inserted into one service.

Each technology has one clear responsibility:

```text
WebSocket
    -> browser/driver realtime connection

NATS Core
    -> low-latency transient internal fan-out

Kafka
    -> durable business events

JetStream
    -> selected durable NATS-native streams

Redis
    -> ephemeral shared state and coordination

Redis Pub/Sub
    -> alternative simple transient broadcast

BullMQ
    -> deferred/retryable jobs

GraphQL
    -> normal client API

gRPC
    -> synchronous domain-service communication

Kubernetes
    -> production orchestration

Skaffold
    -> local Kubernetes development
```

The architecture therefore demonstrates the important System Design principle:

> Use the simplest technology that correctly satisfies the requirement.

---

# 158. Realtime Service in the Full Platform

```text
                           CLIENT
                              |
                +-------------+-------------+
                |                           |
             GraphQL                     WebSocket
                |                           |
                v                           v
         GraphQL Gateway              Realtime Service
                |                           |
       +--------+---------+            +----+----+
       |        |         |            |         |
       v        v         v            v         v
     User    Delivery   Media        Redis      NATS
                         |                       |
                         +----------+------------+
                                    |
                                  Kafka
                                    |
                 +------------------+------------------+
                 |                  |                  |
                 v                  v                  v
             Notification       Analytics          Search
                BullMQ           ClickHouse       Elasticsearch
```

---

# 159. Relationship to the Overall Learning Goals

This service gives practical experience with:

```text
Microservices
Distributed Systems
Event-Driven Architecture
WebSockets
NATS
NATS JetStream
Kafka
Redis
Pub/Sub
Rate Limiting
Idempotency
Backpressure
Horizontal Scaling
Load Balancing
Connection Management
Failure Recovery
Observability
Kubernetes
Docker
Skaffold
CQRS-style read flows
Eventual Consistency
Snapshot + Stream
Message Ordering
At-Least-Once Processing
DLQ
Retry
Circuit Breaker
Distributed Tracing
```

It should therefore be implemented as a serious standalone service rather than as a few WebSocket handlers inside another service.

---

# 160. Final Recommendation

Initial production-oriented path:

```text
WebSocket
    +
Redis
    +
NATS Core
    +
Kafka
    +
OpenTelemetry
```

Then add only when justified:

```text
JetStream
BullMQ
SSE
Redis Pub/Sub
```

The initial implementation should not use all messaging systems for the same traffic.

The strongest architecture is:

```text
Location / Presence
    -> WebSocket + Redis + NATS Core

Durable Delivery Events
    -> Kafka

Selected durable NATS-native events
    -> JetStream

Background Jobs
    -> BullMQ

Server-only streaming
    -> SSE if a concrete requirement appears

Normal API
    -> GraphQL Federation
```

This keeps the Realtime Service technically rich while preserving clear architectural boundaries.
