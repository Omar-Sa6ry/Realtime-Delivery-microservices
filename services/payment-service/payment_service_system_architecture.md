# Realtime Delivery Platform --- Payment Service System Architecture

**Status:** Next core service after Driver & Dispatch\
**Language:** Go\
**Database:** PostgreSQL\
**Public API:** GraphQL Federation through API Gateway\
**Internal synchronous communication:** gRPC\
**Durable events:** Kafka + Transactional Outbox\
**Transient realtime messaging:** NATS\
**Realtime client transport:** WebSocket through Realtime Service\
**Hot state / acceleration:** Redis\
**Background processing:** Native Go workers\
**Infrastructure:** Docker, Kubernetes, Skaffold\
**Observability:** OpenTelemetry, Prometheus, Grafana, Jaeger,
structured logs

------------------------------------------------------------------------

# 1. Purpose

The Payment Service is the financial bounded context of the Realtime
Delivery Platform. It owns payment state, payment attempts, provider
references, captures, authorization cancellation, refunds, idempotency,
audit history, reconciliation, and payment events.

It is a **Saga participant**, not the Saga orchestrator. The Delivery
Service remains the owner of the delivery lifecycle and the distributed
Delivery Saga.

The service must be designed around the hardest payment-system property:

> A provider timeout or service failure must never cause the platform to
> accidentally charge the customer twice.

The architecture therefore combines PostgreSQL transactions, explicit
state machines, application idempotency, provider idempotency,
transactional outbox, Kafka, reconciliation, optimistic/conditional
concurrency, and failure testing.

------------------------------------------------------------------------

# 2. Whole-System Review Before Payment Implementation

The platform is delivery-only. It deliberately has no products, shopping
cart, inventory, product catalog, or e-commerce order domain.

The core business object is **Delivery**.

The current application components are:

  -----------------------------------------------------------------------
  Component               Technology              Owns
  ----------------------- ----------------------- -----------------------
  API Gateway             NestJS + GraphQL        Public API composition,
                          Federation              JWT enforcement, rate
                                                  limiting, routing

  User Service            NestJS + PostgreSQL     Identity, profile,
                                                  credentials, user data

  Notification Service    NestJS + PostgreSQL +   Notification history,
                          Redis + BullMQ          templates, delivery and
                                                  retries

  Media Service           Go + DynamoDB + Redis + Upload sessions,
                          S3-compatible storage   metadata, processing,
                                                  object storage

  Realtime Service        NestJS + WebSocket +    Browser realtime
                          Redis + NATS            connections and fan-out

  Search Service          Go + OpenSearch         Search projections and
                                                  queries

  Delivery Service        NestJS + PostgreSQL     Delivery lifecycle and
                                                  Saga orchestration

  Driver & Dispatch       Go + MongoDB + Redis    Driver operational
  Service                 GEO                     state, location,
                                                  proximity, assignment

  Payment Service         Go + PostgreSQL         Financial operations
                                                  and payment state

  Analytics Service       Go + ClickHouse         Future event analytics
  -----------------------------------------------------------------------

Payment and Analytics should be treated as next/future phases until
implemented. Do not force future services into the current runtime
merely to increase technology count.

------------------------------------------------------------------------

# 3. Overall Architectural Rules

``` text
Client -> GraphQL Federation -> API Gateway -> domain subgraphs

Delivery -> Payment                 gRPC
Payment -> external provider        provider API / HTTPS
Durable business facts              Kafka
Reliable event publication           Transactional Outbox + Kafka
Transient realtime                  NATS
Browser realtime                    WebSocket via Realtime Service
Transactional payment data          PostgreSQL
Hot/ephemeral coordination          Redis
Analytics                           Kafka -> ClickHouse
Search                              Kafka -> OpenSearch
```

Rules:

1.  Every service owns its own data.
2.  No service directly accesses another service's database.
3.  GraphQL Federation is the client-facing business API.
4.  gRPC is the internal synchronous contract.
5.  Kafka is the durable business-event backbone.
6.  NATS is for low-latency transient communication, not financial
    durability.
7.  Realtime Service owns browser WebSockets.
8.  Redis is never the financial source of truth.
9.  Payment state lives in PostgreSQL.
10. Critical operations are idempotent.
11. Unknown provider outcomes are reconciled instead of blindly retried.
12. Payment state changes happen through explicit transitions.
13. Every important operation is observable.
14. Secrets and sensitive payment data are never logged.
15. GenAI, Qdrant, RAG, and AI agents remain future-only.

------------------------------------------------------------------------

# 4. Service Ownership Boundaries

## API Gateway

Owns:

-   GraphQL Federation
-   JWT validation
-   global rate limiting
-   correlation/trace propagation
-   GraphQL validation and complexity protection
-   routing

Does not own payment business rules.

## User Service

Owns:

-   user identity
-   credentials
-   profile
-   authentication-related data

Payment stores `userId` as a reference, not a duplicate user database.

## Delivery Service

Owns:

-   delivery aggregate
-   delivery state machine
-   pickup/dropoff data
-   delivery pricing snapshot
-   delivery status history
-   Saga state
-   Saga compensation
-   assigned-driver reference
-   delivery-side payment status reference

It does not own payment transactions.

## Driver & Dispatch

Owns:

-   driver operational state
-   availability
-   location hot path
-   proximity search
-   assignment
-   offer/accept/reject/expiry

It does not own payment.

## Payment Service

Owns:

-   payment aggregate
-   authorization
-   capture
-   void/cancel authorization
-   payment attempts
-   provider references
-   refunds
-   payment audit
-   idempotency
-   reconciliation
-   payment events

## Notification

Consumes payment events and decides how to notify the user.

## Realtime

Owns WebSocket connections and transient client updates.

## Search

Owns OpenSearch projections only.

## Analytics

Consumes Kafka events and writes ClickHouse projections.

------------------------------------------------------------------------

# 5. Why Payment Is a Separate Bounded Context

Payment has different:

-   security requirements
-   consistency requirements
-   provider integrations
-   retry semantics
-   failure modes
-   audit requirements
-   data retention requirements
-   operational alerts

The correct boundary is:

``` text
Delivery Service
      |
      | Authorize/Capture/Refund
      v
Payment Service
      |
      | provider adapter
      v
Payment Provider
```

Delivery must never contain provider-specific financial code.

------------------------------------------------------------------------

# 6. High-Level Payment Architecture

``` text
                                  CLIENT
                                     |
                                  GraphQL
                                     |
                                     v
                           +---------------------+
                           |     API Gateway     |
                           | NestJS Federation   |
                           +----------+----------+
                                      |
                                Payment Subgraph
                                      |
                                      v
                           +---------------------+
                           |   Payment Service   |
                           |        Go           |
                           +----------+----------+
                                      |
              +-----------------------+-----------------------+
              |                       |                       |
              v                       v                       v
         PostgreSQL                Redis                Provider Adapter
         source of truth       cache/coordination              |
              |                       |                        v
              v                       |                 External PSP
      Transactional Outbox             |
              |                 idempotency/cache
              v
            Kafka
        +-----+------+----------------+
        |            |                |
        v            v                v
 Notification   Analytics          Search
  + BullMQ      + ClickHouse      + OpenSearch

Payment realtime UX:

Payment -> NATS -> Realtime -> WebSocket -> Client
```

------------------------------------------------------------------------

# 7. Payment and Delivery Saga

Delivery remains the orchestrator.

Recommended business flow:

``` text
Customer
   |
   v
Delivery Service
   |
   +--> create delivery
   |
   +--> Driver & Dispatch: assign driver
   |
   +--> Payment: authorize
   |
   +--> delivery execution
   |
   +--> Payment: capture
   |
   v
Delivery completed
```

Payment reports facts. It does not orchestrate the entire delivery
workflow.

------------------------------------------------------------------------

# 8. Why Saga Is Required

This cannot safely be one ACID transaction:

``` text
Delivery PostgreSQL
       |
       +--> Payment PostgreSQL
       |
       +--> Driver MongoDB
       |
       +--> external payment provider
```

A provider API cannot participate in the same local PostgreSQL
transaction.

Use Saga state and compensating operations.

Example:

``` text
Driver assigned
      |
      v
Payment authorization fails
      |
      v
Delivery Saga compensates
      |
      v
Release driver
```

Another example:

``` text
Payment captured
      |
      v
Delivery later fails
      |
      v
Delivery Saga requests refund
```

------------------------------------------------------------------------

# 9. Authorization vs Capture

The service must distinguish:

``` text
Authorization = reserve/approve the amount
Capture       = actually capture/charge the authorized amount
Void          = cancel an authorization before capture
Refund        = return money after capture
```

A possible delivery workflow is:

``` text
REQUESTED
   |
   v
DRIVER_ASSIGNED
   |
   v
PAYMENT_AUTHORIZATION
   |
   v
AUTHORIZED
   |
   v
DELIVERY_IN_PROGRESS
   |
   v
DELIVERED
   |
   v
CAPTURE
   |
   v
CAPTURED
```

The business can choose immediate capture if that is the intended
product rule, but the architecture should still keep
authorization/capture semantics separate.

------------------------------------------------------------------------

# 10. Payment State Machine

Main payment state:

``` text
PENDING
  |
  +----> AUTHORIZED ----> CAPTURED
  |             |
  |             +-------> CANCELLED
  |
  +----> FAILED
  |
  +----> CANCELLED
```

Refund is represented by a separate refund lifecycle:

``` text
NOT_REFUNDED
      |
      v
REFUND_PENDING
      |
      +----> REFUNDED
      |
      +----> REFUND_FAILED
```

Provider operation state may additionally contain:

``` text
NOT_STARTED
PROCESSING
SUCCEEDED
FAILED
UNKNOWN
```

`UNKNOWN` is critical for network ambiguity.

------------------------------------------------------------------------

# 11. State Transition Rules

Allowed examples:

  Current          Operation                     Result
  ---------------- ----------------------------- ----------------
  PENDING          authorize success             AUTHORIZED
  PENDING          authorize permanent failure   FAILED
  PENDING          cancel                        CANCELLED
  AUTHORIZED       capture success               CAPTURED
  AUTHORIZED       void                          CANCELLED
  CAPTURED         refund                        REFUND_PENDING
  REFUND_PENDING   refund success                REFUNDED
  REFUND_PENDING   refund failure                REFUND_FAILED

Invalid examples:

``` text
FAILED -> CAPTURED
CANCELLED -> CAPTURED
REFUNDED -> CAPTURED
CAPTURED -> AUTHORIZED
```

Never expose a generic `updatePaymentStatus(status)` operation.

------------------------------------------------------------------------

# 12. Payment Aggregate

Conceptual aggregate:

``` text
Payment
  |
  +-- PaymentAttempt(s)
  +-- Transaction(s)
  +-- Refund(s)
  +-- Audit entries
```

Payment represents the business financial operation.

Attempts represent execution attempts.

Transactions represent normalized provider financial transactions.

------------------------------------------------------------------------

# 13. PostgreSQL Schema

Recommended tables:

``` text
payments
payment_attempts
payment_transactions
refunds
refund_attempts
idempotency_keys
payment_events_outbox
payment_audit_log
processed_provider_events
```

PostgreSQL is the authoritative local financial store.

------------------------------------------------------------------------

# 14. payments Table

Suggested fields:

``` text
id
 delivery_id
user_id
currency
amount_minor
status
payment_method_type
provider
provider_payment_id
authorized_amount_minor
captured_amount_minor
refunded_amount_minor
version
created_at
updated_at
authorized_at
captured_at
cancelled_at
failed_at
```

Money is stored in integer minor units.

Example:

``` text
500.75 EGP -> amount_minor = 50075, currency = EGP
```

Never use floating point as the financial representation.

------------------------------------------------------------------------

# 15. Payment Amount Invariants

Examples:

``` text
amount_minor > 0
captured_amount_minor <= authorized_amount_minor
refunded_amount_minor <= captured_amount_minor
```

For immediate-capture models, adapt authorization invariants to the
actual provider/business model.

------------------------------------------------------------------------

# 16. Currency

Always store currency explicitly:

``` text
currency = EGP
amount_minor = 50075
```

Do not infer currency from user locale.

Do not silently convert currencies inside Payment Service unless a real
FX requirement exists.

------------------------------------------------------------------------

# 17. payment_attempts Table

Suggested fields:

``` text
id
payment_id
attempt_number
operation
status
provider
provider_request_id
provider_transaction_id
provider_idempotency_key
error_code
error_category
safe_error_message
started_at
completed_at
created_at
```

The attempt history explains what actually happened during provider
communication.

------------------------------------------------------------------------

# 18. payment_transactions Table

Suggested fields:

``` text
id
payment_id
payment_attempt_id
type
provider
provider_transaction_id
amount_minor
currency
status
created_at
```

Types:

``` text
AUTHORIZATION
CAPTURE
VOID
REFUND
```

Provider transaction IDs should be unique when provider semantics allow
it.

------------------------------------------------------------------------

# 19. refunds Table

Suggested fields:

``` text
id
payment_id
delivery_id
amount_minor
currency
status
reason
provider_refund_id
idempotency_key
created_at
updated_at
completed_at
```

A payment may have multiple refunds.

The total refunded amount must never exceed the captured amount.

------------------------------------------------------------------------

# 20. Refund Concurrency

Example:

``` text
Captured = 500

Refund A = 400
Refund B = 200
```

The system must not end with:

``` text
Refunded = 600
```

Use a transaction and conditional state/balance reservation so only a
safe amount can be moved to `REFUND_PENDING`.

A useful invariant is:

``` text
availableRefund = capturedAmount
                - refundedAmount
                - pendingRefundAmount
```

------------------------------------------------------------------------

# 21. Idempotency Table

Suggested fields:

``` text
idempotency_keys
----------------
key
operation
request_hash
status
payment_id
response_payload
created_at
expires_at
```

Use a uniqueness rule scoped to the logical operation/context.

If the same key is reused with a different request hash, reject it.

------------------------------------------------------------------------

# 22. Three Idempotency Layers

Payment has three important idempotency boundaries:

``` text
Client command
     |
     v
Payment Service idempotency
     |
     v
Provider idempotency
```

They solve different problems.

Application idempotency prevents duplicate internal business operations.

Provider idempotency prevents duplicate external charges when the same
operation is retried.

------------------------------------------------------------------------

# 23. Provider Adapter

Do not put provider-specific SDK logic into domain code.

``` text
Application
     |
     v
PaymentProvider interface
     |
     +--> ProviderAAdapter
     +--> ProviderBAdapter
     +--> MockProvider
```

Conceptual methods:

``` text
Authorize
Capture
Void
Refund
GetPayment
GetTransaction
```

The adapter translates provider-specific responses into normalized
Payment errors/results.

------------------------------------------------------------------------

# 24. Provider Error Normalization

Normalize external errors into categories:

``` text
TEMPORARY
PERMANENT
DECLINED
AUTHENTICATION_ERROR
RATE_LIMITED
TIMEOUT
UNKNOWN
```

The domain/application layer should not depend on dozens of
provider-specific error codes.

------------------------------------------------------------------------

# 25. The Most Important Payment Failure

Consider:

``` text
Payment Service -> Provider: capture
Provider accepts request
Provider captures money
Network response is lost
Payment Service sees timeout
```

The local service does **not** know whether money was captured.

Wrong:

``` text
timeout -> assume failed -> retry with a new key
```

Potential result:

``` text
double charge
```

Correct:

``` text
operation = UNKNOWN
        |
        v
reconciliation
        |
        v
provider status lookup
        |
        +--> captured
        +--> not captured
        +--> still unknown
```

------------------------------------------------------------------------

# 26. Provider Idempotency Key

A logical operation must retain the same provider idempotency key across
retries.

Example:

``` text
capture:pay_123:v1
```

If the provider supports idempotency, reuse the same key for the same
logical operation.

Do not generate a fresh random key for every retry.

------------------------------------------------------------------------

# 27. Unknown Outcome State

Recommended operation lifecycle:

``` text
NOT_STARTED
    |
    v
PROCESSING
    |
    +----> SUCCEEDED
    |
    +----> FAILED
    |
    +----> UNKNOWN
```

`UNKNOWN` means:

> The platform cannot safely determine the provider result from the
> current response.

It is not equivalent to failure.

------------------------------------------------------------------------

# 28. Reconciliation

Reconciliation compares:

``` text
local Payment state
        vs
provider state
```

Example:

``` text
Local:    CAPTURE_UNKNOWN
Provider: CAPTURED
```

The reconciliation worker repairs the local state and emits the
appropriate durable event through the outbox.

------------------------------------------------------------------------

# 29. Reconciliation Worker

``` text
Scheduler
   |
   v
Find UNKNOWN / stuck operations
   |
   v
Provider status API
   |
   +--> success -> update local state
   |
   +--> failure -> update local state
   |
   +--> unknown -> retry later
```

Requirements:

-   bounded workload
-   exponential backoff
-   jitter
-   idempotent transitions
-   concurrency protection
-   metrics
-   audit logging

------------------------------------------------------------------------

# 30. Stuck Operation Recovery

If an operation remains `PROCESSING` beyond a configured threshold:

``` text
PROCESSING
    |
    v
UNKNOWN
    |
    v
Reconciliation
```

Do not immediately mark it `FAILED` merely because the local timeout
elapsed.

------------------------------------------------------------------------

# 31. Transactional Outbox

Never rely on:

``` text
DB update
   |
   v
Kafka publish
```

because Kafka can fail after the database commit.

Use:

``` text
BEGIN
  update payment
  insert outbox event
  insert audit entry
COMMIT

Outbox Worker
      |
      v
    Kafka
```

This couples the local state transition and event intent atomically.

------------------------------------------------------------------------

# 32. Outbox Table

Suggested:

``` text
payment_events_outbox
---------------------
id
aggregate_type
aggregate_id
event_type
event_version
payload
headers
occurred_at
published_at
attempts
last_error
```

Indexes should support efficient retrieval of unpublished records.

------------------------------------------------------------------------

# 33. Outbox Concurrency

Multiple Payment replicas may publish events concurrently.

Use a safe claiming strategy such as PostgreSQL row locking with
`SKIP LOCKED`.

Conceptually:

``` sql
SELECT ...
FROM payment_events_outbox
WHERE published_at IS NULL
ORDER BY occurred_at
FOR UPDATE SKIP LOCKED
LIMIT N;
```

Do not assume only one worker exists.

------------------------------------------------------------------------

# 34. Kafka Events

Recommended durable events:

``` text
payment.created
payment.authorization.started
payment.authorized
payment.authorization.failed
payment.capture.started
payment.captured
payment.capture.failed
payment.cancelled
payment.refund.started
payment.refunded
payment.refund.failed
payment.failed
```

Events represent facts, not commands.

------------------------------------------------------------------------

# 35. Command vs Event

Command:

``` text
AuthorizePayment
```

means:

``` text
Please perform authorization.
```

Event:

``` text
payment.authorized
```

means:

``` text
Authorization succeeded.
```

This distinction should remain consistent throughout the platform.

------------------------------------------------------------------------

# 36. Kafka Event Envelope

Example:

``` json
{
  "eventId": "evt_01...",
  "eventType": "payment.authorized",
  "eventVersion": 1,
  "occurredAt": "2026-09-11T12:00:00Z",
  "producer": "payment-service",
  "aggregateType": "payment",
  "aggregateId": "pay_01...",
  "correlationId": "corr_01...",
  "causationId": "cmd_01...",
  "payload": {}
}
```

All durable events should preserve correlation and causation
information.

------------------------------------------------------------------------

# 37. Kafka Partitioning

A practical payment topic can be:

``` text
payments.events
```

Use:

``` text
partition key = paymentId
```

This keeps events for the same payment in the same partition and
preserves their order within that partition when produced correctly.

Do not claim global Kafka ordering.

------------------------------------------------------------------------

# 38. Kafka Consumer Groups

Possible consumer groups:

``` text
notification-payment-consumer
analytics-payment-consumer
search-payment-consumer
```

Each group independently consumes payment events.

Consumers must be idempotent because at-least-once delivery can produce
duplicates.

------------------------------------------------------------------------

# 39. Consumer Idempotency

Example:

``` text
payment.captured
      |
      v
Notification Service
      |
      X worker crashes
      |
      v
same event delivered again
```

The second delivery must not create a duplicate logical notification.

Use `eventId` or another deterministic event identity for deduplication.

------------------------------------------------------------------------

# 40. Payment -\> Delivery

The primary synchronous relationship is:

``` text
Delivery Service
      |
      | gRPC
      v
Payment Service
```

Possible calls:

``` text
CreatePayment
AuthorizePayment
CapturePayment
CancelAuthorization
CreateRefund
GetPayment
GetPaymentStatus
```

Delivery receives normalized results and advances or compensates its
Saga.

------------------------------------------------------------------------

# 41. Suggested gRPC Contract

``` proto
service PaymentService {
  rpc CreatePayment(CreatePaymentRequest)
      returns (CreatePaymentResponse);

  rpc AuthorizePayment(AuthorizePaymentRequest)
      returns (AuthorizePaymentResponse);

  rpc CapturePayment(CapturePaymentRequest)
      returns (CapturePaymentResponse);

  rpc CancelAuthorization(CancelAuthorizationRequest)
      returns (CancelAuthorizationResponse);

  rpc CreateRefund(CreateRefundRequest)
      returns (CreateRefundResponse);

  rpc GetPayment(GetPaymentRequest)
      returns (GetPaymentResponse);

  rpc GetPaymentStatus(GetPaymentStatusRequest)
      returns (GetPaymentStatusResponse);
}
```

Store the contract under a versioned path such as:

``` text
proto/payment/v1/payment.proto
```

------------------------------------------------------------------------

# 42. gRPC Rules

Every call should define:

-   deadline
-   timeout
-   service identity
-   correlation ID
-   request ID
-   idempotency where applicable
-   retry policy
-   error mapping

Retry only transient errors.

Do not blindly retry `INVALID_ARGUMENT`, payment declines, or invalid
state transitions.

------------------------------------------------------------------------

# 43. gRPC Error Mapping

Examples:

``` text
Invalid amount             -> INVALID_ARGUMENT
Payment not found          -> NOT_FOUND
Invalid payment state      -> FAILED_PRECONDITION
Duplicate resource        -> ALREADY_EXISTS
Provider unavailable       -> UNAVAILABLE
Provider timeout           -> DEADLINE_EXCEEDED
Rate limited               -> RESOURCE_EXHAUSTED
Permission failure         -> PERMISSION_DENIED
```

Provider-specific details should be normalized.

------------------------------------------------------------------------

# 44. Payment -\> Realtime

Payment does not own WebSockets.

Correct:

``` text
Payment
   |
   | NATS
   v
Realtime Service
   |
   | WebSocket
   v
Customer / Admin
```

Example transient update:

``` text
payment.status.updated
```

If a realtime message is lost, the client can query the authoritative
state through GraphQL.

------------------------------------------------------------------------

# 45. Payment -\> Notification

Payment should never send email/SMS directly.

Correct:

``` text
Payment
   |
   | Kafka
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

Payment only publishes the business fact.

------------------------------------------------------------------------

# 46. Payment -\> Analytics

Future Analytics consumes:

``` text
payment.created
payment.authorized
payment.authorization.failed
payment.captured
payment.capture.failed
payment.refunded
```

Flow:

``` text
Payment -> Kafka -> Analytics -> ClickHouse
```

Payment never writes directly to ClickHouse.

------------------------------------------------------------------------

# 47. Payment -\> Search

Payment is not a primary search domain.

If an admin search projection needs payment information:

``` text
Payment -> Kafka -> Search Service -> OpenSearch
```

OpenSearch remains eventually consistent and never becomes the payment
source of truth.

------------------------------------------------------------------------

# 48. Payment -\> User

Payment stores a stable:

``` text
userId
```

It does not duplicate the User Service database.

If display information is required, use GraphQL composition or an
appropriate read projection rather than cross-database access.

------------------------------------------------------------------------

# 49. Payment -\> Media

Payment does not store files.

If a future business process needs a payment-related document:

``` text
Client -> Media Service -> Object Storage
```

Payment stores only a media reference when a real requirement exists.

------------------------------------------------------------------------

# 50. Full Platform Flow

``` text
                           CLIENTS
                              |
                     GraphQL / WebSocket
                              |
              +---------------+---------------+
              |                               |
              v                               v
       +--------------+                +--------------+
       | API Gateway  |                |   Realtime   |
       | NestJS       |                | NestJS       |
       | Federation   |                | WebSocket    |
       +------+-------+                +------+-------+
              |                               |
       +------+------+                        |
       |      |      |                        |
       v      v      v                        |
     User  Delivery Media                     |
              |                               |
        +-----+------+                        |
        |            |                        |
        v            v                        |
 Driver/Dispatch  Payment                     |
      Go            Go                        |
        |            |                        |
        +------+-----+------------------------+
               |
             NATS
               |
               +--> Realtime

Durable path:

Delivery/Driver/Payment
          |
      Outbox
          |
        Kafka
          |
    +-----+------+------+
    |            |      |
    v            v      v
Notification  Analytics Search
```

------------------------------------------------------------------------

# 51. End-to-End Payment Authorization Sequence

``` text
Delivery
   |
   | AuthorizePayment(paymentId, deliveryId, amount, idempotencyKey)
   v
Payment Service
   |
   +--> idempotency lookup
   +--> load payment
   +--> validate state
   +--> create PROCESSING attempt
   |
   v
Provider Adapter
   |
   v
Payment Provider
   |
   +---- success
   |
   v
Payment Service
   |
   +--> update payment = AUTHORIZED
   +--> create transaction
   +--> create audit entry
   +--> create outbox event
   |
   v
COMMIT
   |
   v
Delivery receives success
   |
   v
Outbox publishes payment.authorized
```

The provider call should not hold a PostgreSQL transaction open.

------------------------------------------------------------------------

# 52. End-to-End Capture Sequence

``` text
Delivery
   |
   | CapturePayment
   v
Payment
   |
   +--> validate AUTHORIZED
   +--> create capture attempt
   +--> commit PROCESSING state
   |
   v
Provider Adapter
   |
   v
Provider
   |
   +---- success
   |
   v
Payment
   |
   +--> CAPTURED
   +--> transaction record
   +--> audit record
   +--> outbox event
   |
   v
Kafka -> Notification / Analytics / Search
```

------------------------------------------------------------------------

# 53. Unknown Capture Sequence

``` text
Delivery
   |
   v
Payment
   |
   v
Provider
   |
   v
Provider captures
   |
   X response lost
   |
   v
Payment operation = UNKNOWN
   |
   v
Reconciliation Worker
   |
   v
Provider status query
   |
   +---- CAPTURED -> update local state
   |
   +---- NOT CAPTURED -> safe recovery
   |
   +---- UNKNOWN -> retry later
```

This scenario must have an automated test.

------------------------------------------------------------------------

# 54. Refund Sequence

``` text
Delivery/Admin
   |
   | CreateRefund
   v
Payment
   |
   +--> validate captured balance
   +--> reserve refund amount
   +--> create refund attempt
   |
   v
Provider
   |
   +---- success
   |
   v
Payment
   |
   +--> refundedAmount += amount
   +--> REFUNDED
   +--> outbox
   |
   v
Kafka
```

For partial refunds, the sum of successful and pending refunds must
remain within the captured amount.

------------------------------------------------------------------------

# 55. Delivery Compensation After Authorization Failure

``` text
Delivery
   |
   v
Driver Assigned
   |
   v
Authorize Payment
   |
   X
Payment authorization failed
   |
   v
Delivery Saga
   |
   +--> release driver via gRPC
   +--> mark payment failure reference
   +--> transition delivery to failure/cancellation path
```

Payment does not directly release the driver.

------------------------------------------------------------------------

# 56. Delivery Compensation After Capture

If a delivery fails after capture:

``` text
Delivery FAILED
      |
      v
Delivery Saga decides refund
      |
      | gRPC
      v
Payment
      |
      v
Provider refund
      |
      v
REFUNDED
      |
      v
payment.refunded
```

The Delivery Saga records the compensation result on its own side.

------------------------------------------------------------------------

# 57. Provider Webhooks

Some providers confirm operations asynchronously.

The service should support a provider webhook integration boundary where
required.

``` text
Provider
   |
   | signed webhook
   v
Payment Webhook Adapter
   |
   +--> verify signature
   +--> validate timestamp/replay protection
   +--> deduplicate provider event
   +--> normalize provider event
   |
   v
Payment Application
   |
   +--> validate state
   +--> update PostgreSQL
   +--> audit
   +--> outbox
   |
   v
Kafka
```

This webhook is provider-facing infrastructure, not the normal client
API.

------------------------------------------------------------------------

# 58. Webhook Security

Validate according to provider requirements:

-   signature
-   timestamp
-   replay protection
-   provider event identity
-   authenticated source

Never trust a webhook merely because it contains a matching `paymentId`.

------------------------------------------------------------------------

# 59. Webhook Idempotency

Providers can send the same event multiple times.

Store provider event identity:

``` text
provider
provider_event_id
```

Use a uniqueness constraint where appropriate.

Duplicate webhook:

``` text
first -> process
second -> recognize duplicate -> no duplicate side effect
```

------------------------------------------------------------------------

# 60. Webhook Ordering

Do not blindly apply events in arrival order.

Possible mechanisms:

-   provider sequence number
-   provider transaction timestamp
-   operation state
-   current local state
-   reconciliation

An old event must never move a payment backwards, for example:

``` text
CAPTURED -> AUTHORIZED
```

------------------------------------------------------------------------

# 61. Database Concurrency

Payment is a high-value concurrency domain.

Example:

``` text
Request A -> Capture
Request B -> Capture
```

Protection should combine:

``` text
idempotency
+
state validation
+
conditional update / optimistic version
+
provider idempotency
```

------------------------------------------------------------------------

# 62. Optimistic Concurrency

Use a `version` field or conditional state transition.

Conceptually:

``` sql
UPDATE payments
SET status = 'CAPTURED',
    version = version + 1
WHERE id = $1
  AND version = $2
  AND status = 'AUTHORIZED';
```

If affected rows are zero, the state changed concurrently and the
operation must be re-evaluated.

------------------------------------------------------------------------

# 63. Do Not Hold DB Locks During Provider Calls

Bad:

``` text
BEGIN
lock payment row
call provider
wait
COMMIT
```

Better:

``` text
short transaction
  -> mark operation PROCESSING
  -> commit

provider call

short transaction
  -> store result
  -> transition state
  -> outbox
  -> commit
```

This prevents long database locks and connection exhaustion.

------------------------------------------------------------------------

# 64. Refund Reservation Concurrency

For:

``` text
Captured = 500
Refund A = 400
Refund B = 200
```

the application should atomically reserve one request before allowing
the provider call.

Possible approach:

``` text
BEGIN
  lock payment row
  calculate remaining refundable amount
  if amount is valid:
      pendingRefundAmount += amount
      create refund attempt
  COMMIT
```

Then perform the provider operation outside the transaction.

On success:

``` text
pendingRefundAmount -= amount
refundedAmount += amount
```

On failure:

``` text
pendingRefundAmount -= amount
```

All transitions remain conditional and idempotent.

------------------------------------------------------------------------

# 65. Redis Usage

Redis is optional acceleration/coordination, not the source of financial
truth.

Use Redis for:

-   idempotency fast path
-   short-lived cache
-   rate limiting
-   short-lived coordination where justified
-   operational hot state

Do not store the only copy of:

``` text
payment status
refund balance
provider transaction
```

in Redis.

------------------------------------------------------------------------

# 66. Redis Failure

If Redis disappears:

``` text
Payment financial data survives in PostgreSQL.
```

The service may temporarily lose:

-   cache
-   fast idempotency path
-   non-critical coordination

The application should fall back to durable PostgreSQL logic where safe.

------------------------------------------------------------------------

# 67. Distributed Locks

Do not use Redis distributed locks as the primary mechanism for every
payment transition.

Prefer PostgreSQL transactions and conditional updates for payment
correctness.

A Redis lock can be useful for a short operational coordination task,
such as preventing duplicate reconciliation work, but it must have:

-   TTL
-   unique owner token
-   safe release
-   bounded critical section

Never hold it while waiting on a provider response.

------------------------------------------------------------------------

# 68. CQRS

Use CQRS as a logical separation:

``` text
Commands -> domain -> PostgreSQL
Queries  -> read repository -> PostgreSQL/Redis
```

Do not add a separate read database merely to demonstrate CQRS.

A simple relational read model is enough until real requirements justify
more infrastructure.

------------------------------------------------------------------------

# 69. GraphQL Federation

The Payment subgraph may expose:

``` graphql
payment(id: ID!): Payment
paymentStatus(paymentId: ID!): PaymentStatus
```

Possible user-facing operations depend on product policy.

Do not expose unrestricted:

``` graphql
capturePayment
```

if capture is supposed to be controlled by the Delivery Saga.

Internal Saga commands should use gRPC.

------------------------------------------------------------------------

# 70. GraphQL Payment Data

Safe example:

``` text
id
deliveryId
amount
currency
status
capturedAmount
refundedAmount
createdAt
updatedAt
```

Do not expose:

``` text
provider secret
raw card number
CVV
internal provider credentials
internal retry tokens
sensitive provider payloads
```

------------------------------------------------------------------------

# 71. Authentication and Authorization

Authentication boundary:

``` text
Client
  |
  v
API Gateway
  |
  +--> JWT validation
  |
  v
Payment Subgraph
```

Payment still enforces domain authorization.

Examples:

``` text
Customer -> view own payment
Delivery Service -> authorize/capture/cancel according to service policy
Admin -> inspect/reconcile/refund according to privileged policy
```

------------------------------------------------------------------------

# 72. Service-to-Service Identity

Payment should identify the caller:

``` text
caller = delivery-service
```

and apply least privilege.

For example:

``` text
delivery-service -> authorize
                   capture
                   cancel authorization
                   refund when Saga compensation requires it
```

Other services should not automatically receive the same privileges.

------------------------------------------------------------------------

# 73. Sensitive Data Rules

Never store:

-   raw card number
-   CVV
-   PIN
-   magnetic stripe data
-   provider secret keys

Prefer tokenized references:

``` text
payment_method_reference
provider_customer_reference
provider_payment_method_id
```

Optional display metadata such as brand/last4 should only be stored if
legitimately required.

------------------------------------------------------------------------

# 74. Secrets

Provider secrets belong in:

``` text
Kubernetes Secrets
```

or an external secret-management system.

Never:

``` text
hardcode secret
commit secret
store secret in payment row
log secret
```

------------------------------------------------------------------------

# 75. Logging

Allowed examples:

``` text
paymentId
deliveryId
operation
status
provider
providerTransactionId
correlationId
duration
errorCategory
```

Never log sensitive payment credentials or raw provider payloads that
contain them.

------------------------------------------------------------------------

# 76. Correlation ID and Causation ID

Example:

``` text
GraphQL request
requestId = req_1
correlationId = corr_9

Delivery gRPC
requestId = req_2
correlationId = corr_9

Payment provider call
requestId = req_3
correlationId = corr_9

Kafka event
 eventId = evt_4
 correlationId = corr_9
 causationId = req_2
```

This makes the full Saga traceable.

------------------------------------------------------------------------

# 77. Retry Policy

Retry only transient failures.

Typical retryable cases:

``` text
network failure
provider 5xx
temporary unavailable
rate limit when provider policy permits
```

Usually not retryable:

``` text
card declined
invalid payment method
invalid request
unsupported currency
invalid state
```

Unknown financial outcomes should trigger reconciliation, not blind
retries.

------------------------------------------------------------------------

# 78. Exponential Backoff + Jitter

Conceptually:

``` text
attempt 1 -> 100ms
attempt 2 -> 250ms
attempt 3 -> 500ms
attempt 4 -> 1s
```

with jitter.

Without jitter:

``` text
many workers fail together
      |
      v
all retry together
      |
      v
provider overload
```

Use bounded retries and maximum elapsed time.

------------------------------------------------------------------------

# 79. Dead Letter Handling

Asynchronous payment consumers can have permanent failures.

``` text
Kafka
  |
  v
Consumer
  |
  +--> success
  +--> retry transient
  +--> DLQ permanent/unprocessable
```

DLQ records should preserve:

``` text
eventId
eventType
aggregateId
correlationId
error
attemptCount
timestamp
```

A DLQ is not a garbage bin. It needs monitoring and a controlled
replay/recovery process.

------------------------------------------------------------------------

# 80. Background Workers

Native Go workers are appropriate for Payment Service:

``` text
Outbox publisher
Reconciliation
Stuck-operation recovery
Webhook processing where queued
Cleanup/retention jobs
```

Do not introduce BullMQ into Go just because Notification uses BullMQ.

BullMQ remains appropriate for NestJS-owned notification jobs.

------------------------------------------------------------------------

# 81. Worker Scaling

Workers should be horizontally scalable.

Use:

-   bounded batches
-   safe claiming
-   idempotency
-   conditional updates
-   backoff
-   metrics

Do not create an enormous worker pool without measuring workload.

------------------------------------------------------------------------

# 82. Payment Observability

Track:

``` text
payment_operations_total
payment_operations_failed_total
payment_operation_duration_seconds
payment_provider_requests_total
payment_provider_errors_total
payment_provider_latency_seconds
payment_unknown_operations_total
payment_reconciliation_pending
payment_outbox_pending
payment_webhook_total
payment_idempotency_conflicts_total
payment_dlq_total
```

Do not use high-cardinality IDs such as `paymentId` or `userId` as
Prometheus labels.

------------------------------------------------------------------------

# 83. Distributed Tracing

Trace:

``` text
GraphQL
  |
  v
Delivery
  |
  | gRPC
  v
Payment
  |
  | provider API
  v
Provider
  |
  v
PostgreSQL
  |
  v
Outbox -> Kafka
  |
  +--> Notification
  +--> Analytics
  +--> Search
```

Useful spans:

``` text
payment.create
payment.authorize
payment.capture
payment.refund
provider.authorize
provider.capture
provider.refund
payment.reconcile
outbox.publish
```

------------------------------------------------------------------------

# 84. Dashboards

Recommended Grafana dashboards:

## Payment Health

``` text
operations/min
success rate
failure rate
latency p50/p95/p99
```

## Provider Health

``` text
provider latency
5xx
timeouts
declines
rate limits
```

## Reliability

``` text
unknown operations
reconciliation backlog
outbox backlog
DLQ messages
webhook duplicates
idempotency conflicts
```

------------------------------------------------------------------------

# 85. Alerts

Potential alerts:

``` text
payment success rate below threshold
provider timeout spike
unknown operations increasing
reconciliation backlog increasing
outbox backlog increasing
Kafka publish failures
PostgreSQL saturation
provider rate limiting
DLQ growth
```

Thresholds should be based on measured baselines.

------------------------------------------------------------------------

# 86. Health Checks

Provide:

``` text
liveness
readiness
```

Do not make liveness depend on an external payment provider. A provider
outage should not automatically restart every Payment Service instance.

Readiness should reflect whether the instance can safely accept the
workload required by the deployment.

------------------------------------------------------------------------

# 87. Kubernetes

Deployment components:

``` text
payment-service Deployment
payment-service Service
ConfigMap
Secret
HPA
PDB
liveness probe
readiness probe
```

Scale according to actual bottlenecks.

Provider latency may make CPU a poor sole scaling signal.

------------------------------------------------------------------------

# 88. ConfigMap vs Secret

ConfigMap:

``` text
GRPC_PORT
KAFKA_BROKERS
NATS_URL
REDIS_URL
POSTGRES_HOST
POSTGRES_PORT
RECONCILIATION_INTERVAL
RETRY_LIMIT
```

Secret:

``` text
POSTGRES_PASSWORD
PROVIDER_SECRET
WEBHOOK_SECRET
```

------------------------------------------------------------------------

# 89. HPA

Possible signals:

``` text
CPU
memory
request rate
gRPC latency
worker backlog
outbox backlog
reconciliation backlog
```

Do not scale blindly if the real bottleneck is PostgreSQL or the
provider.

------------------------------------------------------------------------

# 90. PDB

Use a Pod Disruption Budget so voluntary infrastructure disruptions do
not remove every Payment replica simultaneously.

The exact minimum depends on the number of replicas and deployment
environment.

------------------------------------------------------------------------

# 91. Graceful Shutdown

On SIGTERM:

``` text
stop accepting new work
finish safe in-flight work
stop workers
flush telemetry
close Kafka
close DB
close Redis
exit
```

Do not terminate in the middle of a critical local state transition
without recovery semantics.

------------------------------------------------------------------------

# 92. Docker

The service should have:

-   small Go binary image
-   minimal runtime image
-   non-root execution where practical
-   environment-based configuration
-   health checks
-   deterministic build

It must be independently deployable.

------------------------------------------------------------------------

# 93. Docker Compose

Local development can include:

``` text
payment-service
postgres
redis
kafka
nats
mock-payment-provider
otel-collector
prometheus
grafana
jaeger
```

If the root project already owns shared infrastructure containers, reuse
them rather than creating duplicates per service.

------------------------------------------------------------------------

# 94. Database Migrations

Use versioned migrations:

``` text
001_create_payments.sql
002_create_payment_attempts.sql
003_create_payment_transactions.sql
004_create_refunds.sql
005_create_idempotency.sql
006_create_outbox.sql
007_create_audit_log.sql
008_create_provider_events.sql
```

Avoid destructive automatic schema synchronization in production-like
environments.

------------------------------------------------------------------------

# 95. Suggested Go Structure

``` text
payment-service/
|
+-- cmd/
|   +-- server/
|   |   +-- main.go
|   +-- worker/
|       +-- main.go
|
+-- internal/
|   +-- domain/
|   |   +-- payment/
|   |   +-- refund/
|   |   +-- attempt/
|   |   +-- money/
|   |
|   +-- application/
|   |   +-- commands/
|   |   +-- queries/
|   |   +-- services/
|   |
|   +-- ports/
|   |   +-- payment_provider.go
|   |   +-- repositories.go
|   |   +-- publisher.go
|   |
|   +-- infrastructure/
|       +-- postgres/
|       +-- redis/
|       +-- kafka/
|       +-- grpc/
|       +-- providers/
|       +-- webhook/
|       +-- observability/
|
+-- proto/payment/v1/payment.proto
+-- migrations/
+-- configs/
+-- tests/
+-- Dockerfile
+-- docker-compose.yml
```

------------------------------------------------------------------------

# 96. Layering

``` text
Transport
   |
   v
Application
   |
   v
Domain
   ^
   |
Infrastructure implements ports
```

The domain should not import PostgreSQL, Kafka, Redis, or a provider
SDK.

------------------------------------------------------------------------

# 97. Domain Layer

Contains:

-   Payment entity
-   Refund entity
-   PaymentAttempt
-   Money value object
-   state transitions
-   domain errors
-   financial invariants

Examples:

``` text
payment.CanAuthorize()
payment.CanCapture()
payment.CanCancel()
payment.CanRefund()
```

------------------------------------------------------------------------

# 98. Application Layer

Use cases:

``` text
CreatePayment
AuthorizePayment
CapturePayment
CancelAuthorization
CreateRefund
GetPayment
GetPaymentStatus
ReconcilePayment
```

Application services coordinate repositories, provider adapters,
idempotency, transactions, audit, and outbox creation.

------------------------------------------------------------------------

# 99. Infrastructure Layer

Contains:

``` text
PostgreSQL repositories
Redis implementation
Kafka publisher
NATS publisher
provider adapters
webhook adapter
gRPC server
observability
```

Infrastructure translates external systems into internal contracts.

------------------------------------------------------------------------

# 100. Security Model

Payment is a high-security service.

Controls:

``` text
least privilege
service authentication
authorization
secret management
input validation
rate limiting
idempotency
webhook verification
log redaction
audit logging
TLS/mTLS where appropriate
```

Do not confuse authentication with authorization.

------------------------------------------------------------------------

# 101. Provider Mock

A fake provider should exist for local development and tests.

Supported deterministic scenarios:

``` text
success
declined
timeout
5xx
rate limited
unknown outcome
duplicate request
webhook
```

This allows realistic failure injection without repeatedly charging a
real provider.

------------------------------------------------------------------------

# 102. Failure Injection Matrix

Test at least:

  -----------------------------------------------------------------------
  Failure                             Expected behavior
  ----------------------------------- -----------------------------------
  Provider 5xx                        bounded retry/backoff

  Provider timeout before acceptance  UNKNOWN + reconciliation
  known                               

  Provider success but response lost  reconciliation finds success

  PostgreSQL unavailable              fail safely, no charge attempt if
                                      state cannot be safely created

  Kafka unavailable after DB commit   outbox retries later

  Redis unavailable                   fall back to durable correctness
                                      where possible

  Duplicate command                   idempotent response

  Same key different payload          reject conflict

  Duplicate webhook                   no duplicate side effect

  Duplicate Kafka event               idempotent consumer

  Concurrent capture                  one logical capture

  Concurrent refund                   refundable balance never exceeded

  Worker crash                        recovery/reconciliation
  -----------------------------------------------------------------------

------------------------------------------------------------------------

# 103. Critical Failure: DB Commit + Kafka Failure

``` text
DB transaction
  |
  +--> payment CAPTURED
  +--> outbox payment.captured
  |
  v
COMMIT

Kafka unavailable
```

Result:

``` text
Payment remains CAPTURED.
Outbox remains pending.
Publisher retries later.
```

No business event is lost.

------------------------------------------------------------------------

# 104. Critical Failure: Provider Success + Local Crash

``` text
Provider captures money
       |
       v
Payment Service crashes
       |
       v
Local operation = PROCESSING/UNKNOWN
       |
       v
Service restarts
       |
       v
Reconciliation
       |
       v
Provider confirms capture
       |
       v
Local CAPTURED + outbox
```

No second charge should occur.

------------------------------------------------------------------------

# 105. Critical Failure: Duplicate Capture

``` text
Delivery -> Capture(idempotency=A)
Delivery -> Capture(idempotency=A)
```

Both represent one logical operation.

Expected:

``` text
one capture
same logical response
no duplicate provider charge
```

------------------------------------------------------------------------

# 106. Critical Failure: Concurrent Refund

``` text
Captured = 500

A -> refund 400
B -> refund 200
```

Expected:

``` text
one request succeeds
or one waits/fails safely

refunded <= 500
```

------------------------------------------------------------------------

# 107. Payment Source of Truth

Use this mental model:

``` text
PostgreSQL
    = local authoritative payment state

External Provider
    = external financial authority

Kafka
    = durable event history/transport

Redis
    = cache/coordination

NATS
    = transient messaging

WebSocket
    = realtime UX transport
```

None of the last four replaces PostgreSQL as the local payment source of
truth.

------------------------------------------------------------------------

# 108. Exactly-Once Reality

Do not claim that the entire distributed system provides global
exactly-once processing.

Use:

``` text
at-least-once delivery
+
transactional outbox
+
application idempotency
+
provider idempotency
+
idempotent consumers
+
reconciliation
```

This provides reliable business effects without pretending distributed
systems magically provide global exactly-once execution.

------------------------------------------------------------------------

# 109. Database Indexes

Useful indexes:

``` text
payments(delivery_id)
payments(user_id, created_at DESC)
payments(status, updated_at)
payment_attempts(payment_id, attempt_number)
payment_attempts(status, created_at)
refunds(payment_id, created_at)
payment_events_outbox(published_at, occurred_at)
```

Use partial indexes where they materially improve pending/outbox
queries.

------------------------------------------------------------------------

# 110. Unique Constraints

Examples:

``` text
UNIQUE(provider, provider_payment_id)
UNIQUE(provider, provider_transaction_id)
UNIQUE(operation, idempotency_key)
UNIQUE(provider, provider_event_id)
```

Exact scopes should reflect provider and business semantics.

Database constraints are a second line of defense against application
bugs.

------------------------------------------------------------------------

# 111. Retention

Define separate retention policies for:

``` text
payment records
audit records
provider operations
outbox records
idempotency records
processed webhook IDs
```

Financial/audit records should not be deleted merely for convenience.

Operational records can have shorter retention when safe.

------------------------------------------------------------------------

# 112. Outbox Cleanup

Published outbox events can be archived/deleted only after the
configured operational replay window.

``` text
published
   |
   | retention period
   v
archive/delete
```

Do not delete immediately after publish if recovery/replay requires the
data.

------------------------------------------------------------------------

# 113. Idempotency Retention

Idempotency records may eventually expire, but the retention period must
cover realistic client/provider retry windows.

Use:

``` text
expires_at
```

and never let expiration accidentally create a second financial
operation for a still-active logical request.

------------------------------------------------------------------------

# 114. Payment API Matrix

  Operation                                     Customer   Delivery Service        Admin       Driver
  ---------------------------- ------------------------- ------------------ ------------ ------------
  View own payment                                   Yes                Yes          Yes   Limited/No
  Create payment                          Yes/controlled                Yes   Controlled           No
  Authorize                      Usually Saga-controlled                Yes   Controlled           No
  Capture                                             No                Yes   Controlled           No
  Cancel authorization                                No                Yes   Controlled           No
  Refund                               Eligible workflow                Yes          Yes           No
  Reconcile                                           No                 No          Yes           No
  Inspect provider operation                          No            Limited   Restricted           No

Exact product policy can be refined during implementation.

------------------------------------------------------------------------

# 115. Payment Event Consumers in the Platform

``` text
payment.authorized
        |
        +--> Delivery Saga interpretation
        +--> Notification
        +--> Analytics
        +--> optional Search projection
        +--> Realtime transient update path
```

The same business fact may have multiple consumers with different
consistency requirements.

------------------------------------------------------------------------

# 116. Consistency Model by Consumer

  -----------------------------------------------------------------------
  Consumer                            Consistency
  ----------------------------------- -----------------------------------
  Delivery Saga                       business workflow must be reliable;
                                      event/gRPC contract is explicit

  Payment PostgreSQL                  strong local consistency

  Notification                        eventual

  Realtime                            transient/eventual UX

  Search                              eventual

  Analytics                           eventual
  -----------------------------------------------------------------------

This is intentional.

------------------------------------------------------------------------

# 117. Payment and Driver Dispatch

They are coordinated by Delivery:

``` text
Delivery
   |
   +--> Driver & Dispatch
   |
   +--> Payment
```

Driver & Dispatch never needs:

``` text
card details
provider credentials
payment state
```

Payment never needs:

``` text
driver GPS
Redis GEO
assignment locks
```

This is clean bounded-context separation.

------------------------------------------------------------------------

# 118. Payment and Realtime

Realtime is the browser-facing transport.

``` text
Payment
   |
   | NATS
   v
Realtime
   |
   | WebSocket
   v
Customer
```

Payment state changes must remain correct even if every WebSocket update
is lost.

------------------------------------------------------------------------

# 119. Payment and Notification

``` text
Payment
   |
   | Kafka
   v
Notification
   |
   | BullMQ
   v
Provider channels
```

Payment should not wait for notification delivery before returning a
financial operation result.

------------------------------------------------------------------------

# 120. Payment and Analytics

``` text
Payment
   |
   v
Kafka
   |
   v
Analytics
   |
   v
ClickHouse
```

Useful future metrics:

``` text
payment volume
success rate
failure rate
provider latency
refund rate
capture latency
```

------------------------------------------------------------------------

# 121. Payment and Search

Search is a projection.

``` text
Payment PostgreSQL
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

Do not query OpenSearch to decide whether money was captured.

------------------------------------------------------------------------

# 122. API Gateway Boundary

``` text
Client
  |
GraphQL
  v
API Gateway
  |
JWT + rate limit + complexity + correlation
  |
Federation
  v
Payment Subgraph
```

Payment-specific authorization and business validation remain inside the
Payment domain/application layer.

------------------------------------------------------------------------

# 123. Payment Service Does Not Do

The service must not:

1.  own Delivery status;
2.  assign drivers;
3.  own driver GPS;
4.  send email directly;
5.  own WebSockets;
6.  write another service's database;
7.  write directly to ClickHouse;
8.  write directly to OpenSearch;
9.  store raw card data;
10. store provider secrets in PostgreSQL;
11. trust arbitrary client ownership IDs;
12. treat Redis as financial truth;
13. use NATS as durable payment storage;
14. blindly retry unknown provider outcomes;
15. hold database locks during provider calls;
16. allow arbitrary payment status updates;
17. refund above the captured balance;
18. add GenAI to the initial service;
19. become the Saga orchestrator;
20. create unnecessary microservices.

------------------------------------------------------------------------

# 124. Testing Strategy

Use:

``` text
Unit tests
Integration tests
Provider contract tests
gRPC contract tests
Kafka consumer tests
Concurrency tests
Failure-injection tests
End-to-end tests
```

The payment test suite should focus more on failure and concurrency than
a simple CRUD service would.

------------------------------------------------------------------------

# 125. Unit Tests

Test:

``` text
money calculations
state transitions
refund limits
idempotency conflicts
provider error normalization
domain validation
```

Examples:

``` text
AUTHORIZED -> CAPTURED = valid
FAILED -> CAPTURED = invalid
REFUNDED -> CAPTURED = invalid
refund > remaining refundable amount = invalid
```

------------------------------------------------------------------------

# 126. Integration Tests

Use real/test containers for:

``` text
PostgreSQL
Redis
Kafka
NATS
```

Test:

-   transactions
-   unique constraints
-   outbox
-   idempotency
-   concurrency
-   event consumption
-   recovery after restart

------------------------------------------------------------------------

# 127. Provider Contract Tests

Test the adapter against:

``` text
success
decline
5xx
timeout
rate limit
unknown outcome
duplicate operation
webhook
```

The application should not care which provider implementation is behind
the interface.

------------------------------------------------------------------------

# 128. Concurrency Tests

Examples:

``` text
100 concurrent capture commands
100 concurrent refund commands
multiple reconciliation workers
duplicate webhook delivery
duplicate Kafka event
multiple service replicas
```

Expected result is one logical financial effect per logical operation.

------------------------------------------------------------------------

# 129. Full E2E Test

``` text
Customer
   |
   v
GraphQL Gateway
   |
   v
Delivery
   |
   +--> Driver & Dispatch
   |
   +--> Payment
   |
   v
Delivery completed
```

Verify:

``` text
payment = CAPTURED
delivery = COMPLETED
payment.captured event exists
notification consumes event
analytics can consume event
realtime update can be produced
```

------------------------------------------------------------------------

# 130. Failure E2E Test

The most valuable E2E scenario:

``` text
Payment sends capture to provider
Provider succeeds
Network response is lost
Payment service crashes
Payment restarts
Reconciliation runs
```

Expected:

``` text
payment eventually = CAPTURED
no duplicate provider charge
payment.captured emitted once logically
```

------------------------------------------------------------------------

# 131. Operational Runbook

When payment incidents occur:

``` text
1. Check provider health.
2. Check Payment Service health.
3. Check PostgreSQL.
4. Check unknown operations.
5. Check reconciliation backlog.
6. Check outbox backlog.
7. Check Kafka.
8. Check webhook failures.
9. Check provider rate limits.
10. Do not manually mutate payment status as a first response.
```

Controlled administrative operations should be audited.

------------------------------------------------------------------------

# 132. Manual Admin Operations

Do not expose raw database updates.

Bad:

``` sql
UPDATE payments SET status = 'CAPTURED';
```

Better:

``` text
InspectPayment
ReconcilePayment
RetrySafeOperation
CreateControlledRefund
```

All privileged financial actions must produce audit records.

------------------------------------------------------------------------

# 133. Kubernetes and Reliability Hardening

Later hardening can include:

``` text
Pod anti-affinity
PDB
HPA
resource requests/limits
NetworkPolicies
mTLS
secret rotation
backup/restore testing
PostgreSQL HA
Kafka replication
```

Do not introduce advanced infrastructure until the basic correctness
model is working.

------------------------------------------------------------------------

# 134. Backup and Recovery

Payment requires a tested recovery plan.

At minimum:

``` text
PostgreSQL backups
restore test
migration recovery plan
outbox recovery
provider reconciliation
```

A backup that has never been restored is not a proven recovery strategy.

------------------------------------------------------------------------

# 135. Disaster Recovery Concept

If Payment Service is unavailable:

``` text
new payment commands may fail safely
existing financial state remains in PostgreSQL
provider operations may continue externally
reconciliation catches unknown outcomes
outbox resumes after recovery
```

The system should prefer temporary unavailability over unsafe duplicate
charges.

------------------------------------------------------------------------

# 136. Availability vs Correctness

For payment operations, correctness is more important than blindly
maximizing availability.

It is acceptable to return:

``` text
PAYMENT_OPERATION_IN_PROGRESS
```

or:

``` text
PAYMENT_RESULT_UNKNOWN
```

rather than risk:

``` text
duplicate charge
```

------------------------------------------------------------------------

# 137. Technology Decision Matrix

  -----------------------------------------------------------------------
  Requirement             Technology              Why
  ----------------------- ----------------------- -----------------------
  Public API              GraphQL Federation      unified client contract

  Internal commands       gRPC                    typed low-latency
                                                  service contract

  Local financial state   PostgreSQL              ACID, constraints,
                                                  transactions

  Durable events          Kafka                   durable, replayable,
                                                  consumer groups

  Reliable event          Outbox                  DB state + event intent
  publication                                     atomically

  Realtime UX             NATS + WebSocket        low latency without
                                                  making UX the source of
                                                  truth

  Cache/coordination      Redis                   low latency

  External financial      Provider adapter        isolation/testability
  integration                                     

  Background work         Go workers              native service
                                                  implementation

  Metrics                 Prometheus              operational monitoring

  Tracing                 OpenTelemetry + Jaeger  distributed tracing

  Dashboards              Grafana                 operational visibility

  Deployment              Kubernetes              orchestration/scaling

  Dev loop                Skaffold                local K8s workflow
  -----------------------------------------------------------------------

------------------------------------------------------------------------

# 138. Why Go

Go is suitable because Payment Service needs:

-   reliable networking
-   gRPC
-   explicit error handling
-   lightweight concurrency
-   efficient workers
-   predictable resource usage
-   easy containerization

The important correctness mechanisms are not the language itself. They
are:

``` text
state machine
transactions
idempotency
provider idempotency
outbox
reconciliation
concurrency control
```

------------------------------------------------------------------------

# 139. Implementation Phases

## Phase 1 --- Foundation

``` text
Go skeleton
configuration
PostgreSQL
migrations
logging
health checks
Docker
```

## Phase 2 --- Domain

``` text
Payment
Money
PaymentAttempt
Refund
state machine
errors
```

## Phase 3 --- Persistence

``` text
repositories
transactions
constraints
idempotency
```

## Phase 4 --- Provider

``` text
provider interface
mock provider
success
decline
timeout
unknown
```

## Phase 5 --- Payment Operations

``` text
Create
Authorize
Capture
Cancel Authorization
Refund
```

## Phase 6 --- Reliable Events

``` text
Outbox
Kafka publisher
event envelope
versioning
```

## Phase 7 --- Delivery Integration

``` text
gRPC
Saga authorization
Saga capture
compensation/refund
```

## Phase 8 --- Webhooks + Reconciliation

``` text
signature verification
deduplication
provider lookup
stuck-operation recovery
```

## Phase 9 --- Realtime + Notifications

``` text
NATS
Realtime
Kafka
Notification
```

## Phase 10 --- Reliability + Observability

``` text
retry
backoff
jitter
DLQ
OpenTelemetry
Prometheus
Grafana
Jaeger
```

## Phase 11 --- Infrastructure

``` text
Kubernetes
ConfigMap
Secrets
HPA
PDB
Skaffold
```

## Phase 12 --- Full E2E

``` text
Delivery + Driver + Payment Saga
failure injection
load/concurrency testing
recovery testing
```

------------------------------------------------------------------------

# 140. Recommended Coding Order

``` text
1. Create Go service skeleton
2. Add configuration
3. Add PostgreSQL
4. Add migrations
5. Implement Money
6. Implement Payment domain
7. Implement state machine
8. Implement repositories
9. Implement idempotency
10. Implement mock provider
11. Implement CreatePayment
12. Implement AuthorizePayment
13. Implement CapturePayment
14. Implement CancelAuthorization
15. Implement Refund
16. Implement audit log
17. Implement transactional outbox
18. Implement Kafka publisher
19. Implement gRPC server
20. Integrate Delivery Service
21. Add provider webhook adapter
22. Add reconciliation worker
23. Add NATS realtime events
24. Add retries/backoff/jitter
25. Add DLQ/recovery procedures
26. Add OpenTelemetry
27. Add Prometheus
28. Add tests
29. Dockerize
30. Kubernetes
31. Skaffold
32. Full Saga E2E
33. Failure E2E
```

------------------------------------------------------------------------

# 141. Definition of Done

## Domain

-   [ ] Explicit payment state machine.
-   [ ] Explicit refund state machine.
-   [ ] Integer minor-unit money.
-   [ ] Invalid transitions rejected.
-   [ ] Refund balance protected.

## Persistence

-   [ ] PostgreSQL schema.
-   [ ] Migrations.
-   [ ] Transactions.
-   [ ] Unique constraints.
-   [ ] Optimistic/conditional concurrency.
-   [ ] Audit log.

## Idempotency

-   [ ] Command idempotency.
-   [ ] Request hash conflict detection.
-   [ ] Provider idempotency.
-   [ ] Webhook idempotency.
-   [ ] Consumer idempotency.

## Provider

-   [ ] Adapter interface.
-   [ ] Mock provider.
-   [ ] Timeout handling.
-   [ ] Decline handling.
-   [ ] Unknown outcome handling.
-   [ ] Error normalization.

## Events

-   [ ] Transactional outbox.
-   [ ] Kafka publisher.
-   [ ] Versioned event envelope.
-   [ ] Durable payment events.

## Saga

-   [ ] Delivery -\> Payment gRPC.
-   [ ] Authorization step.
-   [ ] Capture step.
-   [ ] Cancellation/void step.
-   [ ] Refund compensation.

## Realtime

-   [ ] Payment owns no WebSockets.
-   [ ] NATS transient updates.
-   [ ] Realtime owns browser connections.

## Reliability

-   [ ] Retry classification.
-   [ ] Exponential backoff.
-   [ ] Jitter.
-   [ ] Unknown-state reconciliation.
-   [ ] Stuck-operation recovery.
-   [ ] DLQ/replay procedure.

## Security

-   [ ] No raw card storage.
-   [ ] No secret logging.
-   [ ] Provider secrets externalized.
-   [ ] Webhook verification.
-   [ ] Service authorization.
-   [ ] Audit privileged operations.

## Observability

-   [ ] OpenTelemetry.
-   [ ] Prometheus.
-   [ ] Grafana.
-   [ ] Jaeger.
-   [ ] Structured logs.
-   [ ] Correlation IDs.

## Infrastructure

-   [ ] Docker.
-   [ ] Docker Compose integration.
-   [ ] Kubernetes Deployment.
-   [ ] ConfigMap.
-   [ ] Secrets.
-   [ ] HPA.
-   [ ] PDB.
-   [ ] Probes.
-   [ ] Skaffold.

## Testing

-   [ ] Unit.
-   [ ] Integration.
-   [ ] Provider contract.
-   [ ] gRPC contract.
-   [ ] Concurrency.
-   [ ] Duplicate webhook.
-   [ ] Duplicate command.
-   [ ] Unknown outcome.
-   [ ] DB + Kafka failure.
-   [ ] Full Delivery + Driver + Payment E2E.

------------------------------------------------------------------------

# 142. AI Coding Agent Rules

When this document is provided to a coding agent:

1.  Do not create unnecessary microservices.
2.  Do not create a REST business API.
3.  Use the existing GraphQL Federation Gateway for client APIs.
4.  Use gRPC for Delivery -\> Payment commands.
5.  Use PostgreSQL as Payment's local source of truth.
6.  Never access another service's database.
7.  Use a provider adapter.
8.  Never store raw card data.
9.  Never log secrets.
10. Use application idempotency.
11. Use provider idempotency.
12. Never blindly retry an unknown financial outcome.
13. Use reconciliation.
14. Use transactional outbox.
15. Use Kafka for durable events.
16. Use NATS only for transient realtime communication.
17. Never own browser WebSockets.
18. Use explicit state transitions.
19. Protect refunds against concurrent over-refunding.
20. Preserve correlation IDs.
21. Version protobuf contracts.
22. Version Kafka event schemas.
23. Add failure-path tests.
24. Do not introduce GenAI/Qdrant in the initial service.
25. Do not introduce a workflow engine just to avoid Saga
    implementation.
26. Keep the service independently deployable.
27. Do not add a technology without a concrete responsibility.

------------------------------------------------------------------------

# 143. Anti-Patterns to Avoid

## Direct provider integration from Delivery

Wrong:

``` text
Delivery -> Provider
```

Correct:

``` text
Delivery -> Payment -> Provider
```

## Shared database

Wrong:

``` text
Delivery -> Payment DB
```

Correct:

``` text
Delivery -> gRPC/event -> Payment
```

## Lost events

Wrong:

``` text
update DB -> publish Kafka
```

Correct:

``` text
update DB + outbox in one transaction
```

## Blind payment retry

Wrong:

``` text
timeout -> new provider key -> charge again
```

Correct:

``` text
stable idempotency key
or
UNKNOWN -> reconciliation
```

## WebSocket ownership

Wrong:

``` text
Payment -> browser WebSocket
```

Correct:

``` text
Payment -> NATS -> Realtime -> WebSocket
```

## Redis as source of truth

Wrong:

``` text
Redis = payment database
```

Correct:

``` text
PostgreSQL = payment database
Redis = acceleration/coordination
```

------------------------------------------------------------------------

# 144. System Design Concepts Demonstrated

This service demonstrates:

``` text
Bounded Context
Microservices
GraphQL Federation
gRPC
Saga
State Machine
ACID Transactions
Optimistic Concurrency
Transactional Outbox
Idempotency
Provider Idempotency
Kafka
NATS
WebSocket Boundary
Redis
Retry
Exponential Backoff
Jitter
DLQ
Reconciliation
Provider Adapter
Webhooks
Distributed Tracing
Metrics
Horizontal Scaling
Kubernetes
```

The strongest interview topics are:

``` text
How do you prevent double charging?
What happens when the provider times out?
What if the provider succeeds but your service crashes?
How do you publish reliable payment events?
How do you handle duplicate webhooks?
How do you prevent concurrent refunds from exceeding the captured amount?
Why is Payment a Saga participant?
Why is Redis not the source of truth?
Why does Payment use gRPC internally and GraphQL externally?
```

------------------------------------------------------------------------

# 145. Final Architecture After Payment

``` text
                             CLIENTS
                                |
                       GraphQL / WebSocket
                                |
               +----------------+----------------+
               |                                 |
               v                                 v
        +-------------+                    +-------------+
        | API Gateway |                    |  Realtime   |
        | NestJS      |                    | NestJS      |
        | Federation  |                    | WebSocket   |
        +------+------+                    +------+------+ 
               |                                  |
       +-------+-------+                           |
       |       |       |                           |
       v       v       v                           |
     User   Delivery  Media                        |
              |                                    |
         +----+-----+                              |
         |          |                              |
         v          v                              |
 Driver/Dispatch  Payment                          |
      Go            Go                             |
         |          |                              |
         +-----+----+------------------------------+
               |
              NATS
               |
               v
           Realtime

Durable business events:

Delivery / Driver / Payment
          |
       Outbox
          |
        Kafka
          |
    +-----+------+------+
    |            |      |
    v            v      v
Notification  Analytics Search
   NestJS      ClickHouse OpenSearch
   BullMQ
```

------------------------------------------------------------------------

# 146. Final Project Roadmap

``` text
DONE
├── API Gateway
├── User Service
├── Notification Service
├── Media Service
├── Realtime Service
├── Search Service
├── Delivery Service
└── Driver & Dispatch Service

NEXT
└── Payment Service

THEN
├── Full Delivery + Driver + Payment Saga integration
├── Analytics Service
├── Kubernetes hardening
├── Observability hardening
├── Load testing
└── Failure-injection testing

FUTURE ONLY
├── FastAPI
├── Qdrant
├── RAG
├── LLM
├── AI Agents
├── Recommendations
└── Advanced AI dispatch
```

------------------------------------------------------------------------

# 147. Final Architectural Guarantee

The Payment Service should be judged by this guarantee:

``` text
A retry, duplicate request, duplicate webhook,
service crash, Kafka outage, Redis outage,
provider timeout, or concurrent refund request
must not silently corrupt financial state or create
a duplicate logical charge.
```

The architecture achieves this through:

``` text
PostgreSQL Transactions
        +
Explicit State Machine
        +
Application Idempotency
        +
Provider Idempotency
        +
Optimistic/Conditional Concurrency
        +
Transactional Outbox
        +
Kafka
        +
Reconciliation
        +
Retry + Backoff + Jitter
        +
Audit Logging
        +
Failure Injection Tests
```

------------------------------------------------------------------------

# 148. Final Quick Reference

``` text
Service:
    Payment Service

Language:
    Go

Database:
    PostgreSQL

Public API:
    GraphQL Federation

Internal API:
    gRPC

Durable Events:
    Kafka

Reliable Event Publication:
    Transactional Outbox

Realtime:
    NATS -> Realtime -> WebSocket

Cache/Coordination:
    Redis

External Integration:
    Payment Provider Adapter

Background:
    Native Go Workers

Saga:
    Delivery Service = Orchestrator
    Payment Service  = Participant

Observability:
    OpenTelemetry + Prometheus + Grafana + Jaeger

Deployment:
    Docker + Kubernetes + Skaffold

Core correctness:
    Idempotency + Transactions + State Machine + Reconciliation
```

------------------------------------------------------------------------

# 149. Final Architectural Decision

The correct relationship between the three core business services is:

``` text
                         Delivery Service
                       Saga Orchestrator
                       PostgreSQL/NestJS
                         /            \
                        /              \
                     gRPC              gRPC
                      /                  \
                     v                    v
          Driver & Dispatch             Payment
               Go/MongoDB               Go/PostgreSQL
               Redis GEO                    |
                    |                       |
                    |                  Provider API
                    |                       |
                    +----------+------------+
                               |
                              Kafka
                               |
             +-----------------+------------------+
             |                 |                  |
             v                 v                  v
       Notification        Analytics            Search
```

Driver & Dispatch solves **who should perform the delivery**.

Delivery Service solves **what is the current delivery workflow and
which Saga step comes next**.

Payment Service solves **whether the financial operation succeeded and
how to recover safely when the provider or network fails**.

That separation is the central architecture to preserve as the project
grows.
