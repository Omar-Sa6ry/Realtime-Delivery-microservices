 Realtime Delivery Platform — Analytics Service System Architecture

**Status:** Next core service after Payment Service  
**Language:** Go  
**Database:** ClickHouse  
**Input:** Kafka durable domain events  
**Public API:** GraphQL Federation through API Gateway  
**Internal sync:** gRPC only when explicitly justified  
**Realtime:** NATS only for transient updates; WebSocket remains owned by Realtime Service  
**Cache:** Redis, optional and non-authoritative  
**Infrastructure:** Docker, Kubernetes, Skaffold  
**Observability:** OpenTelemetry, Prometheus, Grafana, Jaeger, structured logs

---

1. Executive Summary

The Analytics Service is the analytical bounded context of the Realtime Delivery Platform. It converts durable business events into analytical facts, dimensions, aggregates, and dashboards in ClickHouse.

The service is intentionally eventually consistent. It never becomes the source of truth for Delivery, Driver & Dispatch, Payment, User, Notification, Media, or Search.

The central pipeline is:

```text
Delivery / Driver / Payment / Notification
                 |
              Outbox
                 |
               Kafka
                 |
        Analytics Consumer Group
                 |
      Validate -> Deduplicate -> Transform
                 |
             Batch Writer
                 |
             ClickHouse
                 |
        GraphQL Analytics API
                 |
             API Gateway
                 |
               Client
```

The service is designed to demonstrate Kafka consumer groups, at-least-once delivery, idempotent consumers, event versioning, partition ordering, late events, backpressure, DLQ, replay, reconciliation, ClickHouse schema design, materialized aggregates, retention, data quality, observability, horizontal scaling, and failure recovery.

---

2. Whole-System Review

The platform is delivery-only. There is no product catalog, shopping cart, inventory, or e-commerce order domain.

Current architecture:

| Service | Technology | Owns |
|---|---|---|
| API Gateway | NestJS + GraphQL Federation | Public API composition, JWT, rate limiting, routing |
| User | NestJS + PostgreSQL | Identity, credentials, profile |
| Notification | NestJS + PostgreSQL + Redis + BullMQ | Notification history and delivery |
| Media | Go + DynamoDB + Redis + S3 | Upload sessions, metadata, media processing |
| Realtime | NestJS + WebSocket + Redis + NATS | Browser realtime connections/fan-out |
| Search | Go + OpenSearch | Search projections |
| Delivery | NestJS + PostgreSQL | Delivery lifecycle and Saga orchestration |
| Driver & Dispatch | Go + MongoDB + Redis GEO | Driver state, location, proximity, assignment |
| Payment | Go + PostgreSQL | Payment state, attempts, captures, refunds |
| Analytics | Go + ClickHouse | Analytical facts, aggregates, reporting |
| Future AI | FastAPI + Qdrant + LLM | Future GenAI only |

The platform-wide communication rules remain:

```text
Client -> GraphQL Federation -> API Gateway
Synchronous service calls -> gRPC
Durable business facts -> Kafka
Transient internal messaging -> NATS
Browser realtime -> WebSocket via Realtime Service
Transactional state -> service-owned databases
Cache/coordination -> Redis
Search -> OpenSearch
Analytics -> ClickHouse
Files -> Object Storage
```

Every service owns its own database. Analytics must never connect directly to Delivery PostgreSQL, Payment PostgreSQL, Driver MongoDB, User PostgreSQL, or another service database.

---

3. Analytics Responsibilities

Analytics owns:

- Kafka event consumption
- event validation and routing
- analytical transformations
- raw event landing where justified
- analytical fact tables
- analytical dimensions
- aggregate tables
- materialized views
- analytics-specific data quality records
- replay/rebuild tooling
- analytical reconciliation
- analytics query API
- analytics retention policy
- analytics observability

Analytics does not own:

- Delivery state
- Payment state
- driver assignment
- current driver availability
- current driver location
- authentication
- notification execution
- media bytes
- search indexes
- Saga orchestration
- external payment provider calls
- WebSocket connections

---

4. Why Kafka -> ClickHouse

Kafka is the durable event-stream boundary. ClickHouse is the analytical storage layer.

Correct:

```text
Domain Service
     |
  Transaction
     |
   Outbox
     |
   Kafka
     |
 Analytics
     |
 ClickHouse
```

Incorrect:

```text
Analytics -> Delivery PostgreSQL
Analytics -> Payment PostgreSQL
Analytics -> Driver MongoDB
```

The domain service publishes the business fact. Analytics interprets that fact for historical and aggregate reporting.

Analytics failure must not stop the Delivery Saga, Payment operation, driver assignment, or notification execution.

---

5. Event Taxonomy

Primary Delivery events:

```text
delivery.created
delivery.driver.assigned
delivery.driver.accepted
delivery.pickup.started
delivery.picked_up
delivery.in_transit
delivery.completed
delivery.cancelled
delivery.failed
```

Driver/Dispatch facts:

```text
driver.available
driver.unavailable
driver.assignment.offered
driver.assignment.accepted
driver.assignment.rejected
driver.assignment.expired
driver.assignment.released
```

Payment facts defined by the Payment Service:

```text
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

Possible Notification facts:

```text
notification.created
notification.sent
notification.delivered
notification.failed
notification.retrying
```

Media events may be added only when the Media Service has a durable business reason to publish them.

---

6. Event Envelope

All durable events should follow the platform envelope:

```json
{
  "eventId": "evt_01...",
  "eventType": "payment.captured",
  "eventVersion": 1,
  "occurredAt": "2026-09-15T10:00:00Z",
  "producer": "payment-service",
  "aggregateType": "payment",
  "aggregateId": "pay_01...",
  "correlationId": "corr_01...",
  "causationId": "cmd_01...",
  "payload": {}
}
```

Analytics must preserve event identity and lineage fields:

```text
eventId
eventType
eventVersion
producer
aggregateType
aggregateId
occurredAt
correlationId
causationId
sourceTopic
sourcePartition
sourceOffset
ingestedAt
```

Event schemas must be versioned. Analytics must never silently parse a newer incompatible version as an older one.

---

7. Kafka Topics and Partitioning

Recommended domain topics:

```text
delivery.events
driver.events
payment.events
notification.events
media.events
```

The initial Analytics consumer group can be:

```text
analytics-service
```

Later, very high-volume domains may receive separate groups.

Recommended keys:

```text
delivery events -> deliveryId
payment events -> paymentId
assignment events -> assignmentId
driver events -> driverId
```

Kafka ordering is partition-local. Do not assume global ordering across the platform.

If all events for a delivery use `deliveryId` as the partition key, events for that delivery can preserve their relative order within the partition.

---

8. Analytics Consumer Pipeline

Recommended processing pipeline:

```text
Kafka Consumer
      |
      v
Decode
      |
      v
Envelope Validation
      |
      v
Event Schema Validation
      |
      v
Deduplication / Idempotency
      |
      v
Event Router
      |
      v
Domain Transformer
      |
      v
Bounded Batch Buffer
      |
      v
ClickHouse
      |
      v
Safe Kafka Progress
```

A failure path:

```text
processing failure
      |
      +--> transient -> bounded retry -> backoff + jitter
      |
      +--> permanent -> DLQ
```

Do not acknowledge an event merely because it was received. The analytical side effect must be handled safely first.

---

9. At-Least-Once and Idempotency

The architecture must assume at-least-once event delivery.

A crash can produce:

```text
ClickHouse insert succeeds
        |
Consumer crashes before safe progress
        |
Kafka redelivers
```

Therefore event processing must be idempotent.

The stable identity is primarily:

```text
eventId
```

with domain-specific business keys where required.

For example, receiving the same `payment.captured` event twice must not double the captured amount.

Do not claim global exactly-once processing. Use:

```text
at-least-once Kafka
+
stable event IDs
+
idempotent transformations
+
safe ClickHouse ingestion
+
reconciliation
+
replay
```

---

10. Raw Event Landing Layer

A raw/normalized event table is useful for lineage and replay.

Suggested fields:

```text
event_id
event_type
event_version
aggregate_type
aggregate_id
producer
occurred_at
ingested_at
correlation_id
causation_id
source_topic
source_partition
source_offset
payload_json
```

It should have an explicit retention policy.

Raw events should not be retained forever by default.

The landing layer is for:

```text
debugging
lineage
replay
data quality
audit support
transformation recovery
```

---

11. Fact Model

Recommended initial fact tables:

```text
fact_delivery_events
fact_delivery_completed
fact_driver_assignments
fact_payment_transactions
fact_notification_events
```

Facts represent measurable business events or derived analytical measurements.

Example delivery event:

```text
event_id
delivery_id
user_id
driver_id
event_type
occurred_at
ingested_at
city_id
zone_id
correlation_id
```

Example payment transaction:

```text
event_id
payment_id
delivery_id
user_id
provider
transaction_type
status
amount
currency
occurred_at
provider_latency_ms
```

---

12. Delivery Fact Model

`fact_delivery_events` stores the event history needed for flexible analytics.

`fact_delivery_completed` can store a denormalized delivery timeline:

```text
delivery_id
user_id
driver_id
created_at
assigned_at
accepted_at
pickup_started_at
picked_up_at
in_transit_at
delivered_at
completed_at
total_duration_seconds
assignment_duration_seconds
pickup_duration_seconds
transit_duration_seconds
```

This supports:

```text
completion rate
delivery duration
assignment latency
pickup duration
transit duration
SLA analysis
```

---

13. Driver Fact Model

`fact_driver_assignments`:

```text
assignment_id
delivery_id
driver_id
offered_at
accepted_at
rejected_at
expired_at
released_at
distance_meters
response_time_ms
assignment_result
```

It supports:

```text
acceptance rate
rejection rate
expiration rate
response time
assignment distance
completed deliveries
driver performance
```

Current driver availability remains owned by Driver & Dispatch.

---

14. Payment Fact Model

Payment analytics consumes the Payment Service's durable events.

Suggested fact:

```text
fact_payment_transactions
-------------------------
event_id
payment_id
delivery_id
user_id
provider
transaction_type
status
amount
currency
occurred_at
provider_latency_ms
```

Possible transaction types:

```text
AUTHORIZATION
CAPTURE
REFUND
CANCEL
```

Never ingest raw card numbers, CVV, provider secrets, or other sensitive payment credentials.

For money, use exact decimal semantics rather than binary floating point.

---

15. Dimensions

Useful dimensions:

```text
dim_date
dim_time
dim_user
dim_driver
dim_location
dim_payment_provider
dim_notification_channel
```

Do not blindly copy operational tables.

Only store attributes required by analytical queries.

Example `dim_user`:

```text
user_id
country
city
signup_date
user_type
created_at
```

Never copy passwords or authentication secrets.

Example `dim_location`:

```text
location_id
country
city
zone
latitude_bucket
longitude_bucket
```

Precise coordinates should not be retained indefinitely without a concrete requirement.

---

16. Star Schema vs Denormalization

A conceptual star schema is:

```text
                    dim_date
                       |
                       v
dim_user -> fact_delivery <- dim_location
                       |
                       v
                  dim_driver
```

However, ClickHouse often benefits from denormalized analytical tables.

Choose schema based on actual query patterns.

The goal is not academic normalization.

The goal is:

```text
fast analytical queries
predictable storage
simple ingestion
clear metric definitions
```

---

17. ClickHouse Engine Strategy

Start with the simplest suitable engine.

For immutable event facts:

```text
MergeTree
```

is often the clearest choice.

For aggregate-state workloads:

```text
AggregatingMergeTree
```

may be appropriate.

`ReplacingMergeTree` must not be treated as a magical exactly-once mechanism. Its replacement semantics happen during merges and must be understood before relying on it.

Do not choose engines merely because they sound advanced.

---

18. ClickHouse Partitioning

Partition large fact tables by time.

Example:

```sql
PARTITION BY toYYYYMM(occurred_at)
```

Benefits:

```text
partition pruning
retention management
maintenance
time-range operations
```

Do not create a partition for every:

```text
user
driver
delivery
```

because excessive small partitions can hurt performance.

---

19. ClickHouse ORDER BY

ClickHouse query performance depends strongly on the sorting key.

Possible examples:

```text
ORDER BY (event_type, occurred_at, delivery_id)
ORDER BY (city_id, occurred_at, delivery_id)
ORDER BY (driver_id, occurred_at)
```

The correct key must follow the real query workload.

Design queries first:

```text
What is queried?
What is filtered?
What is grouped?
What is the common time range?
```

Then choose the sorting key.

---

20. Data Types

Recommended principles:

```text
Counts/IDs -> integer types appropriate to range
Money -> Decimal
Timestamps -> DateTime64 where precision is useful
Low-cardinality dimensions -> LowCardinality(String) where appropriate
```

Use UTC for stored event timestamps.

Keep both:

```text
occurred_at
ingested_at
```

Business metrics use event time.

Pipeline freshness uses ingestion time.

---

21. Event Time and Late Events

An event can happen at 10:00 and reach Analytics at 10:10.

Store:

```text
occurred_at = 10:00
ingested_at = 10:10
```

Business reporting should normally use `occurred_at`.

Late events must be supported.

Do not assume:

```text
ingestion order == business event order
```

For advanced windowing, watermarks may be introduced later. The initial system can rely on event-time queries plus correction/replay mechanisms.

---

22. Data Quality

Analytics must detect suspicious records rather than silently accepting them.

Examples:

```text
negative delivery duration
completed before created
accepted before offered
refund amount > captured amount
negative payment amount
unknown delivery reference
unknown driver reference
future timestamp beyond tolerance
duplicate event identity
unsupported event version
missing aggregate ID
```

Suggested table:

```text
analytics_data_quality_issues
-----------------------------
issue_id
event_id
issue_type
aggregate_type
aggregate_id
detected_at
severity
details
resolved_at
```

Data-quality problems should generate metrics and structured logs.

---

23. Materialized Aggregates

Dashboards should not repeatedly scan enormous raw event tables.

Example:

```text
fact_delivery_events
        |
        v
materialized aggregation
        |
        v
delivery_hourly_metrics
```

Suggested aggregate:

```text
hour
city_id
total_deliveries
completed_deliveries
cancelled_deliveries
failed_deliveries
avg_duration_seconds
```

Similar tables:

```text
driver_hourly_metrics
payment_daily_metrics
```

Use materialized views or aggregate tables where they provide measurable benefit.

---

24. Delivery Analytics

Initial delivery metrics:

```text
deliveries per hour/day
deliveries by city/zone
completion rate
cancellation rate
failure rate
average duration
P50 duration
P95 duration
P99 duration
average assignment time
pickup duration
transit duration
```

Definitions must be explicit.

For example:

```text
total_delivery_duration = completed_at - created_at
transit_duration = completed_at/delivered_at - in_transit/pickup timestamp
```

Do not use one ambiguous metric called `delivery_duration` for different business meanings.

---

25. Driver Analytics

Initial driver metrics:

```text
offers
accepted
rejected
expired
acceptance rate
response time
assignment distance
completed deliveries
active time
idle time
utilization
```

Document formulas.

For example, these are different:

```text
accepted / all offers
accepted / responded offers
```

The metric dictionary must define which one is used.

---

26. Payment Analytics

Initial payment metrics:

```text
authorization count
authorization success rate
authorization failure rate
capture count
captured amount
refund count
refunded amount
refund rate
provider latency
unknown operation count
```

Payment Service remains the authority.

Analytics must never query a payment provider or decide whether money was captured.

---

27. Notification Analytics

If Notification publishes durable facts, Analytics can calculate:

```text
notifications created
sent
delivered
failed
retry count
channel distribution
provider latency
```

Notification Service remains responsible for actual email/push/in-app delivery.

Analytics must never send notifications directly.

---

28. Platform Overview

A platform overview can expose:

```text
totalDeliveries
completedDeliveries
cancelledDeliveries
activeDeliveries
completionRate
averageDeliveryDuration
paymentCapturedAmount
refundAmount
driverAcceptanceRate
analyticsAsOf
```

`analyticsAsOf` communicates eventual consistency.

The dashboard is not allowed to imply that analytical numbers are real-time transactional truth.

---

29. GraphQL Analytics API

Analytics may expose a GraphQL Federation subgraph.

Example:

```graphql
type Query {
  platformOverview(range: AnalyticsRange!): PlatformOverview!
  deliveryAnalytics(filter: DeliveryAnalyticsFilter!): DeliveryAnalytics!
  driverAnalytics(filter: DriverAnalyticsFilter!): DriverAnalytics!
  paymentAnalytics(filter: PaymentAnalyticsFilter!): PaymentAnalytics!
}
```

The API Gateway remains the public composition boundary.

Never expose arbitrary ClickHouse SQL through GraphQL.

---

30. Query Guardrails

Analytics queries can be expensive.

Enforce:

```text
maximum date range
maximum result size
query timeout
concurrency limits
role-based access
rate limiting
```

Avoid unbounded queries.

Prefer:

```text
time buckets
top-K
aggregates
pagination
bounded drill-down
```

Dashboard queries should use aggregate tables where possible.

---

31. Redis Cache

Redis can cache expensive analytics responses.

Example:

```text
GraphQL
   |
Redis
   |
   +--> hit -> response
   |
   +--> miss -> ClickHouse -> cache
```

Cache keys must include all relevant filters and authorization scope.

Example:

```text
analytics:delivery:city=CAIRO:range=7d:bucket=hour:v1
```

If Redis fails, correctness must remain intact:

```text
Redis unavailable -> ClickHouse
```

Redis is not durable analytics storage.

---

32. Realtime Relationship

Realtime Service owns browser WebSockets.

Analytics does not.

Optional transient flow:

```text
Analytics
   |
  NATS
   |
Realtime Service
   |
WebSocket
   |
Client
```

A lost realtime dashboard update must not corrupt analytics.

The client can query GraphQL again.

Durable analytics ingestion remains:

```text
Domain -> Outbox -> Kafka -> Analytics
```

---

33. Search Relationship

Search and Analytics are independent projections:

```text
Kafka
  |
  +--> Search -> OpenSearch
  |
  +--> Analytics -> ClickHouse
```

Search answers:

```text
Find things
```

Analytics answers:

```text
Measure things
```

Analytics never writes OpenSearch directly, and Search never becomes the analytics database.

---

34. Media Relationship

If Media publishes useful durable lifecycle events:

```text
media.uploaded
media.processing.started
media.ready
media.quarantined
media.processing.failed
```

Analytics can calculate:

```text
upload volume
processing time
failure rate
file-type distribution
```

Analytics stores metadata only.

It never stores media bytes.

---

35. User Relationship

Analytics may consume safe user lifecycle events:

```text
user.created
user.activated
user.deleted
```

Use a minimal analytical dimension.

Do not replicate authentication data.

Do not connect to User PostgreSQL directly.

---

36. Delivery Saga Relationship

Delivery remains the Saga orchestrator.

Analytics is outside the Saga.

Correct:

```text
Delivery Saga
   |
   +--> Driver & Dispatch
   |
   +--> Payment
   |
   +--> complete

Separately:

Delivery / Driver / Payment
          |
        Kafka
          |
      Analytics
```

The Saga must never wait for Analytics.

If Analytics is down, delivery business operations continue.

---

37. Payment Relationship

Payment publishes durable events through its own transactional outbox.

Example:

```text
Payment PostgreSQL
       |
    Outbox
       |
     Kafka
       |
   Analytics
       |
  ClickHouse
```

The Payment Service's events include authorization, capture, refund, failure, and cancellation facts.

Analytics calculates historical metrics only.

The Payment Service remains responsible for provider reconciliation, idempotency, and financial correctness.

---

38. Driver & Dispatch Relationship

Driver & Dispatch publishes durable assignment/driver facts.

Analytics calculates:

```text
acceptance rate
rejection rate
expiration rate
response time
assignment distance
driver utilization
```

Current location and availability remain in Driver & Dispatch.

Do not write every GPS heartbeat to normal analytical facts without a dedicated telemetry strategy.

---

39. High-Frequency GPS

GPS can create much higher event volume than normal business events.

Do not blindly implement:

```text
GPS every second
   |
Kafka
   |
ClickHouse normal facts
```

at scale.

If historical location analytics is needed later, introduce:

```text
sampling
aggregation
dedicated telemetry topic
controlled retention
geospatial analytical schema
```

This is a future extension, not a requirement for the initial Analytics Service.

---

40. Replay

Analytics must support rebuilding analytical projections.

Use cases:

```text
transformation bug
new metric
new table
schema migration
data corruption
historical backfill
```

Flow:

```text
Kafka historical events
        |
        v
Replay consumer
        |
        v
staging/rebuild ClickHouse tables
        |
        v
validate totals
        |
        v
promote
```

Replay must not accidentally consume the live stream with conflicting semantics.

---

41. DLQ

Permanent processing failures go to a dedicated DLQ.

Suggested metadata:

```text
eventId
eventType
eventVersion
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

Replay after fixing the root cause.

Do not permanently discard malformed events without traceability.

---

42. Retry Policy

Retry only transient failures:

```text
temporary ClickHouse outage
network timeout
temporary dependency failure
```

Do not retry forever:

```text
invalid schema
unsupported version
missing mandatory field
invalid negative amount
```

Use:

```text
bounded retries
exponential backoff
jitter
```

Example:

```text
100ms + jitter
200ms + jitter
400ms + jitter
800ms + jitter
1600ms + jitter
```

---

43. Backpressure

If ClickHouse slows down:

```text
Kafka
  |
Analytics
  |
bounded buffer
  |
ClickHouse slow
```

Do not allow unlimited memory growth.

Use:

```text
bounded batches
bounded in-flight writes
controlled concurrency
consumer lag monitoring
```

Kafka should absorb temporary ingestion pressure.

---

44. Consumer Scaling

Kafka partitions determine useful consumer parallelism.

Example:

```text
partition 0 -> consumer A
partition 1 -> consumer B
partition 2 -> consumer C
partition 3 -> consumer D
```

More replicas than useful partitions do not automatically increase throughput.

Scale based on:

```text
partition count
event rate
ClickHouse write capacity
consumer lag
CPU/memory
```

Preserve partition ordering where required.

---

45. Query vs Ingestion Isolation

Analytics has two workloads:

```text
Ingestion:
Kafka -> ClickHouse

Query:
GraphQL -> ClickHouse
```

A heavy dashboard query should not starve ingestion.

Use:

```text
query limits
query timeouts
caching
bounded date ranges
appropriate ClickHouse resource controls
```

As scale grows, consider workload/resource isolation.

---

46. Freshness

Define:

```text
analytics freshness =
current time - latest successfully processed event time
```

Expose:

```text
analytics_freshness_seconds
```

Example operational interpretation:

```text
0-10 sec   healthy
10-60 sec  warning
>60 sec    degraded
```

These thresholds are configurable.

Every analytics response can expose:

```text
dataAsOf
```

when the UI needs to communicate freshness.

---

47. Observability

Prometheus metrics should include:

```text
analytics_events_consumed_total
analytics_events_processed_total
analytics_events_failed_total
analytics_events_duplicate_total
analytics_events_dlq_total
analytics_processing_latency_ms
analytics_batch_size
analytics_batch_flush_total
analytics_query_latency_ms
analytics_query_errors_total
analytics_replay_events_total
analytics_data_quality_issues_total
analytics_freshness_seconds
kafka_consumer_lag
clickhouse_insert_latency
```

Grafana should visualize:

```text
Kafka lag
events/sec
processing latency
ClickHouse insert latency
ClickHouse query latency
DLQ
duplicate events
data quality
freshness
CPU
memory
disk
replicas
```

---

48. Distributed Tracing and Logs

Propagate:

```text
traceId
correlationId
causationId
```

Structured logs should contain:

```json
{
  "level": "INFO",
  "service": "analytics-service",
  "eventType": "payment.captured",
  "eventId": "evt_123",
  "aggregateId": "pay_123",
  "topic": "payment.events",
  "partition": 3,
  "offset": 18392,
  "correlationId": "corr_123",
  "traceId": "trace_123",
  "message": "analytics event processed"
}
```

Never log card numbers, secrets, passwords, or credentials.

---

49. Security

Analytics can contain sensitive operational and financial information.

Requirements:

```text
authentication
authorization
TLS
service identity
least privilege
secret management
audit logging
PII minimization
```

Kafka permissions should be read-only for required topics.

ClickHouse application credentials should have only required SELECT/INSERT access.

Do not put secrets in source code.

---

50. PII and Privacy

Minimize PII.

Prefer:

```text
userId
driverId
city
zone
```

over:

```text
full name
phone
email
exact address
```

unless there is a documented requirement.

Never ingest raw payment card data.

Retention should be defined per dataset.

---

51. Failure Scenarios

### ClickHouse unavailable

```text
Analytics -> ClickHouse X
```

Result:

```text
bounded retry
Kafka lag increases
no false success acknowledgement
eventual catch-up after recovery
```

### Analytics crashes

Kafka retains unprocessed events. Consumer group resumes after restart.

### Duplicate event

Same `eventId` produces one logical analytical effect.

### Malformed event

Validate -> bounded retry where appropriate -> DLQ.

### Unknown event type

Do not crash the process. Record a metric/log and apply the documented unknown-event policy.

### Redis unavailable

Bypass cache; correctness remains unchanged.

---

52. Failure Injection Tests

Required failure tests:

```text
ClickHouse down
Kafka unavailable
consumer crash
crash after ClickHouse insert
duplicate event
out-of-order event
late event
malformed event
unsupported event version
DLQ replay
replay job crash
high Kafka lag
slow ClickHouse
query overload
Redis unavailable
```

The goal is to verify that failures cause:

```text
lag
retry
recovery
degradation
```

rather than silent data loss or corrupted metrics.

---

53. Data Reconciliation

Reconciliation should periodically inspect:

```text
consumer lag
ingestion gaps
duplicate rates
event/fact count discrepancies
unexpected event types
stale processing state
aggregate anomalies
```

Example:

```text
Kafka events for 10:00-11:00
        |
        v
Analytics facts only until 10:45
        |
        v
Gap detected
        |
        v
Replay/reprocess
```

Reconciliation must be conservative and observable.

---

54. Metric Dictionary

Document every metric.

| Metric | Definition |
|---|---|
| Completion Rate | Completed eligible deliveries / eligible created deliveries |
| Assignment Time | Assigned timestamp - created timestamp |
| Acceptance Response | Response timestamp - offered timestamp |
| Total Delivery Duration | Completed timestamp - created timestamp |
| Payment Success Rate | Successful eligible operations / eligible attempts |
| Refund Rate | Refunded amount / captured amount |
| Analytics Freshness | Now - latest processed event time |

Definitions are part of the API contract. Changing a definition should be treated as a versioned analytical change.

---

55. API Authorization

Possible roles:

```text
customer
driver
operations
finance
admin
```

Examples:

```text
customer -> own scoped analytics where required
driver -> own performance
operations -> fleet/delivery metrics
finance -> payment analytics
admin -> platform-wide analytics
```

Gateway handles global authentication. Analytics enforces analytics-specific authorization.

Never trust a client-supplied ownership ID without authorization.

---

56. Infrastructure

Local:

```text
Docker Compose
  |
  +--> analytics-service
  +--> Kafka
  +--> ClickHouse
  +--> Redis
```

Kubernetes:

```text
Deployment
Service
ConfigMap
Secret
HPA
PDB
ServiceAccount
NetworkPolicy
CronJob/Job where needed
```

Use Skaffold for the local Kubernetes development loop.

---

57. Health and Graceful Shutdown

Expose:

```text
/health/live
/health/ready
```

Readiness should check critical dependencies such as Kafka consumer initialization and ClickHouse connectivity.

Graceful shutdown:

```text
stop new GraphQL work
stop new Kafka consumption
finish in-flight processing
flush batches
safely finish ClickHouse writes
close Kafka/ClickHouse connections
close telemetry
exit
```

Kubernetes termination grace period must allow enough time for safe shutdown.

---

58. Go Project Structure

Recommended:

```text
analytics-service/
|
+-- cmd/
|   +-- server/main.go
|
+-- internal/
|   +-- domain/
|   +-- application/
|   |   +-- ingestion/
|   |   +-- analytics/
|   |   +-- replay/
|   |   +-- reconciliation/
|   |
|   +-- infrastructure/
|   |   +-- kafka/
|   |   +-- clickhouse/
|   |   +-- redis/
|   |   +-- observability/
|   |
|   +-- interfaces/
|       +-- graphql/
|       +-- health/
|       +-- metrics/
|
+-- migrations/
+-- proto/
+-- deployments/
+-- tests/
+-- docs/
+-- go.mod
```

Kafka handlers should not contain ClickHouse SQL directly. Keep transport, application, domain transformation, and infrastructure concerns separated.

---

59. Testing Strategy

Unit tests:

```text
event validation
routing
transformation
duration calculations
percentages
metric definitions
version compatibility
idempotency
```

Integration tests:

```text
Kafka -> Analytics -> ClickHouse
```

E2E:

```text
create delivery
-> driver assigned
-> payment captured
-> delivery completed
-> Kafka
-> Analytics
-> GraphQL dashboard
```

Verify eventual correctness, not synchronous availability of analytics.

---

60. Critical Test Cases

### Duplicate payment capture

```text
payment.captured E1
payment.captured E1
```

Expected:

```text
one analytical capture
```

### Late event

```text
occurred_at = 10:00
ingested_at = 10:10
```

Expected:

```text
business metric uses 10:00
pipeline freshness uses 10:10
```

### Consumer crash

Expected:

```text
event may redeliver
idempotency prevents double counting
```

### ClickHouse outage

Expected:

```text
Kafka lag increases
no silent loss
catch-up after recovery
```

### DLQ

Expected:

```text
bad event -> retry policy -> DLQ
```

Then replay after correction.

---

61. Capacity Planning

Track:

```text
events/sec
average event size
Kafka partitions
consumer replicas
ClickHouse rows/sec
ClickHouse insert latency
query/sec
dashboard concurrency
```

The core condition is:

```text
sustainable processing throughput >= incoming event rate
```

with operational headroom.

Storage estimate:

```text
daily raw volume
= events/day * average row size
```

Then account for compression, sorting structures, aggregate tables, retention, and replication where applicable.

---

62. Retention

Define retention separately for:

```text
raw_events
delivery facts
driver facts
payment facts
aggregates
data quality issues
DLQ
logs
```

Example starting policy:

```text
raw event landing: 30-90 days
operational analytical facts: 1-2 years
aggregates: longer if useful
DLQ: until resolved + controlled retention
logs: short operational retention
```

These are engineering starting points, not legal requirements. Financial/legal retention must be reviewed separately for a real production deployment.

---

63. What Must Not Be Added Initially

Do not add merely for technology count:

```text
NATS JetStream as a duplicate Kafka
Debezium/CDC
Event Sourcing
Temporal
Redis Cluster
another analytics database
AI/GenAI
Qdrant
ML dispatch
high-frequency GPS warehouse
complex SCD Type 2 everywhere
```

Add advanced technology only when a real requirement justifies it.

The project should demonstrate strong architecture, not maximum technology count.

---

64. Future AI Boundary

GenAI remains future-only.

Possible future:

```text
ClickHouse
    |
approved Analytics API/tools
    |
FastAPI AI Service
    |
LLM / Qdrant / RAG
```

Possible use cases:

```text
natural-language analytics
anomaly explanation
operational assistant
semantic delivery analysis
```

AI must not receive unrestricted ClickHouse credentials and must not become the source of truth.

---

65. Implementation Phases

### Phase 1 — Core

```text
Go skeleton
configuration
health
logging
Kafka consumer
event envelope
validation
ClickHouse
delivery/payment/driver handlers
batch writer
basic idempotency
```

### Phase 2 — Analytics API

```text
GraphQL subgraph
platform overview
delivery analytics
driver analytics
payment analytics
query guardrails
```

### Phase 3 — Reliability

```text
retry
backoff
jitter
DLQ
reconciliation
data quality
replay
```

### Phase 4 — Optimization

```text
materialized aggregates
Redis cache
ClickHouse optimization
load testing
```

### Phase 5 — Infrastructure

```text
Docker
Compose
Kubernetes
HPA
PDB
NetworkPolicy
Skaffold
Grafana
```

### Phase 6 — Future

```text
advanced analytics
telemetry analytics
anomaly detection
GenAI
```

---

66. Definition of Done

```text
[ ] Go service starts
[ ] Kafka consumer group works
[ ] ClickHouse works
[ ] Event envelope validated
[ ] Versioning supported
[ ] Delivery events processed
[ ] Driver events processed
[ ] Payment events processed
[ ] Notification events optionally processed
[ ] Duplicate events are safe
[ ] Late events are supported
[ ] Out-of-order events are safe
[ ] Monetary values use exact representation
[ ] Fact tables are correct
[ ] Aggregate tables are correct
[ ] GraphQL queries work
[ ] Authorization works
[ ] Query limits work
[ ] Kafka lag is observable
[ ] Analytics freshness is observable
[ ] ClickHouse performance is observable
[ ] Retry works
[ ] DLQ works
[ ] Replay works
[ ] Reconciliation works
[ ] Data-quality checks work
[ ] Structured logs work
[ ] Distributed tracing works
[ ] Prometheus metrics work
[ ] Docker works
[ ] Kubernetes manifests exist
[ ] HPA/PDB exist
[ ] NetworkPolicy exists
[ ] Integration tests pass
[ ] E2E tests pass
[ ] Failure tests pass
[ ] Load tests executed
```

---

67. AI Coding Agent Rules

1. Do not access another service's database.
2. Consume durable events through Kafka.
3. Keep ClickHouse inside Analytics ownership.
4. Treat events as at-least-once.
5. Make handlers idempotent.
6. Preserve eventId and lineage.
7. Do not assume global event ordering.
8. Handle late events.
9. Never silently discard malformed events.
10. Use bounded retries.
11. Use exponential backoff and jitter.
12. Use a DLQ for permanent failures.
13. Support replay.
14. Keep money as exact decimal values.
15. Do not expose arbitrary SQL through GraphQL.
16. Bound analytics queries.
17. Keep PII minimal.
18. Never ingest raw card data.
19. Do not send notifications directly.
20. Do not own WebSockets.
21. Do not participate in the Delivery Saga.
22. Do not use NATS as the durable analytics backbone.
23. Do not use Redis as durable analytics storage.
24. Do not add GenAI initially.
25. Keep the service independently deployable.
26. Make Kafka lag and freshness observable.
27. Test duplicate/crash/late-event scenarios.
28. Test ClickHouse failure.
29. Keep future telemetry/ML behind explicit boundaries.
30. Do not change producer event semantics silently.

---

68. ADRs

Recommended Architecture Decision Records:

```text
ADR-001 Kafka is Analytics ingestion boundary
ADR-002 ClickHouse is analytical storage
ADR-003 Analytics is eventually consistent
ADR-004 EventId is the primary event identity
ADR-005 Partition keys preserve aggregate ordering
ADR-006 Raw event landing layer
ADR-007 Fact/aggregate schema strategy
ADR-008 ClickHouse partitioning and sorting strategy
ADR-009 Analytics query guardrails
ADR-010 Replay/rebuild strategy
ADR-011 Retention strategy
ADR-012 High-frequency location telemetry strategy
ADR-013 Future AI boundary
```

---

69. Complete Platform Flow

```text
                              CLIENTS
                                 |
                    +------------+------------+
                    |                         |
                 GraphQL                   WebSocket
                    |                         |
                    v                         v
             +-------------+           +-------------+
             | API Gateway |           |  Realtime   |
             |   NestJS    |           |   Service   |
             +------+------+           +------+------+
                    |                         |
             Federation                        |
                    |                         |
       +------------+--------------------------+
       |       |        |        |       |
       v       v        v        v       v
     User   Delivery   Media   Search  Analytics
               |                   |       |
               |                   |       |
               +---- Driver -------+       |
               |                          |
               +---- Payment              |
               |                          |
               +---- Notification         |
                                          |
                 Domain Services          |
                      |                   |
                   Outbox                 |
                      |                   |
                    Kafka <---------------+
                      |
        +-------------+-------------+
        |             |             |
        v             v             v
   Notification   Analytics      Search
                     |
                     v
                 ClickHouse
                     |
                     v
              Analytics GraphQL
                     |
                     v
                API Gateway
                     |
                     v
                   Client
```

This keeps the transactional, streaming, search, analytics, and realtime responsibilities separated.

---

70. Final Architecture Decision

The Analytics Service is:

```text
Go
  |
Kafka Consumer Group
  |
Validation
  |
Idempotency
  |
Transformation
  |
Batching
  |
ClickHouse
  |
Facts + Dimensions + Aggregates
  |
GraphQL Analytics API
```

The durable relationship with the rest of the platform is:

```text
Delivery --------Driver & Dispatch -Payment ------------+--> Outbox --> Kafka --> Analytics --> ClickHouse
Notification -------/
Media --------------/
User ---------------/
```

The key boundary is:

```text
Transactional World
        |
        | durable business facts
        v
Event Streaming World
        |
        v
Analytical World
```

The system therefore remains:

```text
Delivery / Driver / Payment
= transactional business systems

Kafka
= durable event backbone

Analytics
= analytical transformation layer

ClickHouse
= analytical storage

GraphQL
= controlled analytics read API

Realtime
= transient browser UX

Redis
= cache/ephemeral state

GenAI
= future extension
```

After Analytics is stable, the project should move to full cross-service integration, observability hardening, load testing, failure injection, Kubernetes hardening, and replay/recovery testing before starting the future GenAI phase.

---


# Appendix A — Service Relationship Matrix

| Service | Sends to Analytics | Analytics Uses | Analytics Does Not Do |
|---|---|---|---|
| Delivery | delivery lifecycle events | funnel, durations, completion | change delivery state |
| Driver & Dispatch | driver/assignment facts | acceptance, response, utilization | assign/release drivers |
| Payment | payment lifecycle events | volume, capture, refund, latency | charge/refund money |
| Notification | notification lifecycle facts | delivery-channel metrics | send notifications |
| User | safe lifecycle facts | user dimensions/cohorts | authenticate users |
| Media | optional media lifecycle facts | processing/upload metrics | store media bytes |
| Realtime | normally no durable dependency | optional transient dashboard signals | own sockets |
| Search | independent projection | no direct dependency | write OpenSearch |
| API Gateway | GraphQL requests | analytics query transport | perform analytics SQL |
| AI Service | future | approved analytical tools | become source of truth |

# Appendix B — Canonical Data Flow Per Business Operation

## Delivery Created

```text
Client
 -> GraphQL Gateway
 -> Delivery Service
 -> PostgreSQL transaction
 -> Outbox
 -> Kafka delivery.events
 -> Analytics consumer
 -> fact_delivery_events
 -> aggregates
```

## Driver Accepted

```text
Driver
 -> Realtime WebSocket
 -> Realtime/NATS
 -> Driver & Dispatch
 -> Kafka driver.events
 -> Analytics
 -> fact_driver_assignments
```

## Payment Captured

```text
Delivery Saga
 -> Payment gRPC
 -> Payment Provider
 -> Payment PostgreSQL
 -> Outbox
 -> Kafka payment.events
 -> Analytics
 -> fact_payment_transactions
```

## Delivery Completed

```text
Delivery Service
 -> PostgreSQL state transition
 -> Outbox
 -> Kafka delivery.events
 -> Analytics
 -> fact_delivery_completed
 -> hourly/daily aggregates
```

# Appendix C — Recommended GraphQL Analytics Types

```graphql
scalar DateTime

enum AnalyticsGranularity {
  HOUR
  DAY
  WEEK
  MONTH
}

input AnalyticsRange {
  from: DateTime!
  to: DateTime!
  granularity: AnalyticsGranularity!
}

type PlatformOverview {
  totalDeliveries: Long!
  completedDeliveries: Long!
  cancelledDeliveries: Long!
  failedDeliveries: Long!
  completionRate: Float!
  averageDeliveryDurationSeconds: Float!
  paymentCapturedAmount: Decimal!
  refundedAmount: Decimal!
  driverAcceptanceRate: Float!
  dataAsOf: DateTime!
}

type DeliveryAnalytics {
  total: Long!
  completed: Long!
  cancelled: Long!
  failed: Long!
  completionRate: Float!
  averageDurationSeconds: Float!
  p50DurationSeconds: Float!
  p95DurationSeconds: Float!
  p99DurationSeconds: Float!
  averageAssignmentTimeSeconds: Float!
  dataAsOf: DateTime!
}

type DriverAnalytics {
  driverId: ID!
  offers: Long!
  accepted: Long!
  rejected: Long!
  expired: Long!
  acceptanceRate: Float!
  averageResponseTimeMs: Float!
  completedDeliveries: Long!
  dataAsOf: DateTime!
}

type PaymentAnalytics {
  authorizationCount: Long!
  authorizationSuccessRate: Float!
  captureCount: Long!
  capturedAmount: Decimal!
  refundCount: Long!
  refundedAmount: Decimal!
  refundRate: Float!
  averageProviderLatencyMs: Float!
  dataAsOf: DateTime!
}
```

The actual GraphQL schema must be aligned with the existing federation conventions in the project.

# Appendix D — Example ClickHouse DDL Shape

The following is architectural pseudocode and must be adapted to the actual ClickHouse version and workload.

```sql
CREATE TABLE fact_delivery_events
(
    event_id String,
    delivery_id String,
    user_id String,
    driver_id String,
    event_type LowCardinality(String),
    event_version UInt16,
    city_id String,
    zone_id String,
    occurred_at DateTime64(3, 'UTC'),
    ingested_at DateTime64(3, 'UTC'),
    correlation_id String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (event_type, occurred_at, delivery_id);
```

Payment:

```sql
CREATE TABLE fact_payment_transactions
(
    event_id String,
    payment_id String,
    delivery_id String,
    user_id String,
    provider LowCardinality(String),
    transaction_type LowCardinality(String),
    status LowCardinality(String),
    amount Decimal(18, 2),
    currency LowCardinality(String),
    provider_latency_ms UInt64,
    occurred_at DateTime64(3, 'UTC'),
    ingested_at DateTime64(3, 'UTC')
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (provider, occurred_at, payment_id);
```

These definitions are starting points, not immutable production schemas.

# Appendix E — Failure Matrix

| Failure | Expected Behavior |
|---|---|
| Kafka unavailable | ingestion degraded; existing analytics remains readable |
| ClickHouse unavailable | bounded retry; Kafka lag grows; no silent acknowledgement |
| Analytics crash | Kafka consumer group resumes |
| Crash after ClickHouse write | redelivery is safe through idempotency |
| Duplicate event | one logical analytical effect |
| Malformed event | validation failure -> retry/DLQ |
| Unsupported version | explicit incompatibility handling |
| Late event | retain event time; correct/rebuild if required |
| Out-of-order event | safe fact ingestion + data-quality signal |
| Redis unavailable | bypass cache |
| Query overload | limits/timeouts/cache |
| DLQ growth | alert + investigate producer/schema/consumer |
| Replay crash | replay resumes or restarts safely |
| Disk pressure | alert, retention/partition management |
| Consumer lag spike | scale consumers/ClickHouse or investigate bottleneck |

# Appendix F — Final Implementation Checklist

```text
ARCHITECTURE
[ ] Bounded context documented
[ ] No cross-database access
[ ] Kafka is ingestion boundary
[ ] ClickHouse is analytical storage
[ ] Analytics is outside the Saga

KAFKA
[ ] Topics documented
[ ] Partition keys documented
[ ] Consumer group configured
[ ] Event versions supported
[ ] Offset behavior defined
[ ] Lag metrics available

INGESTION
[ ] Envelope validation
[ ] Event validation
[ ] Idempotency
[ ] Batch writer
[ ] Bounded buffers
[ ] Retry/backoff/jitter
[ ] DLQ

CLICKHOUSE
[ ] Fact tables
[ ] Aggregate tables
[ ] Partition strategy
[ ] ORDER BY strategy
[ ] Decimal money types
[ ] Retention
[ ] Query indexes/sorting appropriate to workload

API
[ ] GraphQL subgraph
[ ] Authorization
[ ] Query limits
[ ] Date range limits
[ ] Pagination/top-K
[ ] dataAsOf

OPERATIONS
[ ] Health checks
[ ] Graceful shutdown
[ ] OpenTelemetry
[ ] Prometheus
[ ] Grafana
[ ] Jaeger
[ ] Structured logs
[ ] Alerts

RECOVERY
[ ] Reconciliation
[ ] Replay
[ ] DLQ replay
[ ] Backfill procedure
[ ] Table rebuild procedure

TESTING
[ ] Unit tests
[ ] Integration tests
[ ] E2E tests
[ ] Duplicate-event test
[ ] Late-event test
[ ] Out-of-order test
[ ] ClickHouse outage test
[ ] Consumer crash test
[ ] DLQ test
[ ] Replay test
[ ] Load test

INFRASTRUCTURE
[ ] Docker
[ ] Compose
[ ] Kubernetes
[ ] ConfigMap
[ ] Secret
[ ] HPA
[ ] PDB
[ ] NetworkPolicy
[ ] Skaffold
```
