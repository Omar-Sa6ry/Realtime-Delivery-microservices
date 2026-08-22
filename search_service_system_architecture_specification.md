# Search Service — System Architecture Specification

**Project:** Realtime Delivery Platform  
**Service:** Search Service  
**Primary Language:** Go  
**Client API:** GraphQL Federation through the API Gateway  
**Search Engine:** OpenSearch  
**Purpose:** Centralized search, filtering, autocomplete, geo search, relevance ranking, and future semantic/hybrid search.

---

## 1. Executive Decision

For this personal project, use **OpenSearch**, not Elasticsearch.

The goal is to learn distributed search deeply while keeping the architecture open-source-first and avoiding unnecessary infrastructure duplication. OpenSearch provides BM25/full-text search, analyzers, filtering, autocomplete, aggregations, geo search, vector search, and hybrid search. citeturn0search0turn0search5

Elasticsearch is also technically excellent and provides full-text search, aggregations, geospatial search, vector search, relevance tooling, and autocomplete. citeturn0search2turn0search4

**Do not run both.**

```text
Search Service
      |
      v
 OpenSearch
```

---

# 2. Service Responsibility

Search Service owns:

- Search indexes
- Search documents
- Mappings
- Analyzers
- Query construction
- Ranking
- Filtering
- Sorting
- Pagination
- Autocomplete
- Fuzzy search
- Geo search
- Suggestions
- Index synchronization
- Reindexing
- Search cache
- Search metrics
- Relevance experiments

It does **not** own transactional business data.

```text
Business Service
      |
      v
PostgreSQL / MongoDB
      |
      | source of truth
      v
     Kafka
      |
      v
Search Service
      |
      v
 OpenSearch
      |
      | read/search projection
      v
   Clients
```

---

# 3. High-Level Architecture

```text
CLIENT
  |
GraphQL
  |
  v
+----------------------+
| API Gateway          |
| NestJS Federation    |
+----------+-----------+
           |
           v
+----------------------+
| Search Subgraph      |
+----------+-----------+
           |
           v
+----------------------+
| Search Service       |
| Go                   |
+----+------------+----+
     |            |
     v            v
  Redis       OpenSearch
  Cache       Search Engine

Source Services
      |
      v
   Kafka
      |
      v
Search Consumer
      |
      v
OpenSearch
```

---

# 4. Why Search Is a Separate Service

Avoid:

```text
Delivery Service ---> OpenSearch
User Service ------> OpenSearch
Media Service -----> OpenSearch
```

Prefer:

```text
Delivery Service
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

Benefits:

1. Search logic is centralized.
2. Business services do not depend directly on OpenSearch.
3. Search mappings are isolated.
4. Reindexing is independent.
5. Search can scale independently.
6. Search failures do not corrupt transactional databases.
7. Multiple domains can produce searchable projections.

---

# 5. Source of Truth Rule

OpenSearch is **never** the transactional source of truth.

If OpenSearch is lost:

```text
PostgreSQL / MongoDB
        |
        v
Reindex Worker
        |
        v
OpenSearch
```

The entire search layer must be rebuildable.

---

# 6. Searchable Domains

Recommended:

| Source | Searchable Data |
|---|---|
| User Service | Users / Drivers |
| Delivery Service | Deliveries |
| Media Service | Media metadata |
| Future Dispatch Service | Driver/operational discovery |

Initial priority:

```text
1. Deliveries
2. Drivers
3. Media
```

---

# 7. Indexes

Use separate indexes:

```text
deliveries-v1
drivers-v1
media-v1
```

Do not create:

```text
everything-v1
```

Separate indexes allow different mappings, analyzers, scaling, retention, and independent reindexing.

---

# 8. Index Aliases

Application code should use aliases:

```text
deliveries
drivers
media
```

Instead of:

```text
deliveries-v1
```

Example:

```text
deliveries
    |
    v
deliveries-v2
```

During reindex:

```text
deliveries-v1
deliveries-v2
      |
      v
  alias switch
```

---

# 9. Delivery Search Document

```json
{
  "delivery_id": "del_123",
  "customer_id": "usr_123",
  "driver_id": "drv_456",
  "status": "IN_TRANSIT",
  "pickup": {
    "city": "Damietta",
    "country": "Egypt",
    "location": {
      "lat": 31.4165,
      "lon": 31.8133
    }
  },
  "dropoff": {
    "city": "Mansoura",
    "country": "Egypt",
    "location": {
      "lat": 31.0409,
      "lon": 31.3785
    }
  },
  "created_at": "2026-08-17T10:00:00Z",
  "updated_at": "2026-08-17T10:30:00Z",
  "source_version": 7
}
```

Only index fields required for search/filtering/sorting.

---

# 10. Driver Search Document

```json
{
  "driver_id": "drv_123",
  "name": "Driver Name",
  "status": "AVAILABLE",
  "vehicle_type": "CAR",
  "location": {
    "lat": 31.4165,
    "lon": 31.8133
  },
  "rating": 4.8,
  "updated_at": "2026-08-17T10:30:00Z",
  "source_version": 12
}
```

Do **not** stream every driver GPS update into OpenSearch.

High-frequency live location belongs to Realtime/Dispatch infrastructure and Redis GEO. OpenSearch can hold searchable/historical location information.

---

# 11. Media Search Document

```json
{
  "media_id": "media_123",
  "owner_id": "usr_123",
  "file_name": "delivery-proof.jpg",
  "mime_type": "image/jpeg",
  "media_type": "IMAGE",
  "status": "READY",
  "size": 1827364,
  "created_at": "2026-08-17T10:00:00Z",
  "source_version": 3
}
```

---

# 12. Search Capabilities

The service should support:

```text
Exact search
Full-text search
Prefix search
Autocomplete
Fuzzy search
Filtering
Range queries
Geo search
Sorting
Aggregations
Highlighting
Cursor pagination
Relevance ranking
```

---

# 13. Relevance

Search is not simply database lookup.

```text
Query
  |
  v
Analyzer
  |
  v
Inverted Index
  |
  v
BM25
  |
  v
Score
  |
  v
Ranking
  |
  v
Results
```

OpenSearch supports BM25 lexical search and relevance optimization. citeturn0search0

---

# 14. Analyzers

Use:

```text
Tokenizer
Lowercase
Stopwords
Stemming
Synonyms
N-grams
```

Example:

```text
"Delivery Driver"
       |
       v
 lowercase
       |
       v
"delivery driver"
       |
       v
 tokens
```

OpenSearch supports index-time and search-time analyzers and custom analyzer configurations. citeturn0search1

---

# 15. Autocomplete

Use an analyzer designed for prefix matching, such as edge n-grams.

Example:

```text
Input:
dam
```

Generated prefix tokens conceptually:

```text
d
da
dam
```

This is appropriate for type-ahead search. OpenSearch documents support for autocomplete/result navigation and analyzer-based prefix behavior. citeturn0search0turn0search1

---

# 16. Fuzzy Search

Example:

```text
User:
"Mansor"

Indexed:
"Mansoura"
```

Fuzzy search can tolerate small spelling differences.

Do not overuse fuzziness because it can increase cost and reduce precision.

---

# 17. Geo Search

OpenSearch should handle search-oriented geographic queries.

Example:

```text
Find deliveries within 10 km
of:
31.4165, 31.8133
```

Use:

```text
geo_point
geo_distance
```

For current driver proximity:

```text
Realtime / Dispatch
       |
       v
Redis GEO
```

For searchable historical/geographic data:

```text
Search Service
       |
       v
OpenSearch
```

---

# 18. Redis

Redis is an optimization/control layer.

```text
Search Service
   |
   +---- Redis
   |
   +---- OpenSearch
```

Use Redis for:

```text
Search cache
Autocomplete cache
Rate limiting
Temporary coordination
Idempotency where needed
```

Example keys:

```text
search:{hash}
suggest:{hash}
```

Recommended TTLs are workload-dependent; start with short TTLs such as 30 seconds to 5 minutes.

---

# 19. Cache Key

Canonicalize the search request:

```text
query
+
filters
+
sort
+
pagination
```

Then:

```text
SHA256(canonical_request)
```

Example:

```text
search:9c72...
```

---

# 20. Cache Strategy

Prefer:

```text
short TTL
+
versioned keys
```

Do not build complicated per-document invalidation initially.

Cache is never authoritative.

---

# 21. Kafka

Kafka is the primary durable synchronization mechanism.

```text
Delivery Service
      |
      v
Transactional Outbox
      |
      v
Kafka
      |
      v
Search Consumer
      |
      v
OpenSearch
```

This keeps business transactions separate from search indexing.

---

# 22. Kafka Topics

Recommended:

```text
delivery.created
delivery.updated
delivery.deleted

driver.created
driver.updated
driver.deleted

media.created
media.updated
media.deleted
```

You can also consolidate by domain if operational simplicity becomes more important.

---

# 23. Consumer Group

Use:

```text
search-service
```

Example:

```text
Kafka
 |
 +------------------+
 |                  |
 v                  v
Analytics        Search
Service          Service
                    |
                    v
               OpenSearch
```

Each consumer group has independent offsets.

---

# 24. Event Contract

```json
{
  "event_id": "evt_123",
  "event_type": "DELIVERY_UPDATED",
  "aggregate_id": "del_123",
  "aggregate_version": 7,
  "occurred_at": "2026-08-17T10:30:00Z",
  "payload": {
    "status": "IN_TRANSIT"
  }
}
```

---

# 25. Idempotency

Kafka can redeliver events.

Use deterministic document IDs:

```text
delivery_id
driver_id
media_id
```

Then perform:

```text
UPSERT
```

instead of creating random search documents.

Repeated event:

```text
UPSERT del_123
```

does not create duplicates.

---

# 26. Event Ordering

Events can arrive out of order.

Example:

```text
version 8
version 7
```

If OpenSearch contains version 8:

```text
incoming version 7
       |
       v
ignore
```

Always track:

```text
source_version
```

---

# 27. Delete Flow

```text
Delivery Service
      |
      v
DELIVERY_DELETED
      |
      v
Kafka
      |
      v
Search Service
      |
      v
OpenSearch DELETE
```

The client never directly manipulates OpenSearch.

---

# 28. Reindexing

Reindex when:

```text
Mapping changes
Analyzer changes
Ranking changes
New fields are added
Bug is fixed
Index corruption occurs
Search engine migration occurs
```

Flow:

```text
Source DB
   |
   v
Reindex Worker
   |
   v
deliveries-v2
   |
   v
Validation
   |
   v
Alias Switch
```

---

# 29. Zero-Downtime Reindex

```text
1. Create v2
2. Apply mappings
3. Bulk index source data
4. Validate count
5. Validate sample queries
6. Switch alias
7. Keep v1 temporarily
8. Delete v1 later
```

---

# 30. Bulk Indexing

Do not index millions of documents individually.

Use:

```text
Bulk Request
  |
  +-- doc
  +-- doc
  +-- doc
  +-- doc
```

Inspect individual bulk item results because a successful HTTP response does not necessarily mean every item succeeded.

---

# 31. BullMQ

BullMQ is for Node-owned operational background jobs.

Possible queues:

```text
search-reindex
search-backfill
search-maintenance
search-cache-warmup
```

Example:

```text
Admin
 |
 v
Start Reindex
 |
 v
BullMQ
 |
 v
Worker
 |
 v
OpenSearch
```

Do **not** use BullMQ as the cross-service event bus.

Because Search Service is Go, continuous event consumption should be implemented with Go workers around Kafka rather than forcing BullMQ into the Go runtime.

---

# 32. NATS

NATS is not the primary indexing pipeline.

Use it only for transient low-latency signals when a concrete use case exists.

Examples:

```text
search.query.started
search.query.completed
```

Durable synchronization:

```text
Kafka
```

---

# 33. NATS JetStream

JetStream should be optional.

Use it when you specifically need:

```text
Durability
Acknowledgements
Replay
Persistent consumers
```

Do not duplicate the same durable search event stream in:

```text
Kafka + JetStream
```

Use Kafka as the primary durable domain-event pipeline for Search Service.

---

# 34. Redis Pub/Sub

Redis Pub/Sub is transient.

Possible use:

```text
Search Instance A
      |
      v
Redis Pub/Sub
      |
      v
Search Instance B
```

Do not use Redis Pub/Sub for events that must survive consumer downtime.

---

# 35. WebSocket

Search Service should not own browser WebSocket connections.

Realtime remains:

```text
Realtime Service
      |
      v
WebSocket
```

If an admin reindex operation needs realtime progress:

```text
Search Service
      |
      v
NATS / Kafka
      |
      v
Realtime Service
      |
      v
WebSocket
      |
      v
Admin Dashboard
```

---

# 36. SSE

SSE is optional and not part of the primary Search architecture.

If a future administrative operation needs one-way progress streaming, SSE can be considered.

Primary realtime transport remains:

```text
Realtime Service -> WebSocket
```

---

# 37. GraphQL Federation

Clients never access OpenSearch directly.

```text
Client
  |
  v
API Gateway
  |
  v
Search Subgraph
  |
  v
Search Service
  |
  v
OpenSearch
```

Example:

```graphql
query {
  searchDeliveries(
    input: {
      query: "Damietta"
      status: IN_TRANSIT
    }
  ) {
    items {
      id
      status
      pickup
      dropoff
      createdAt
    }
    total
    pageInfo {
      hasNextPage
    }
  }
}
```

---

# 38. Search Input

```graphql
input DeliverySearchInput {
  query: String
  status: DeliveryStatus
  city: String
  driverId: ID
  fromDate: DateTime
  toDate: DateTime
  latitude: Float
  longitude: Float
  radiusKm: Float
  sort: DeliverySearchSort
  pagination: PaginationInput
}
```

---

# 39. Search Operations

Recommended:

```text
SearchDeliveries
SearchDrivers
SearchMedia
Autocomplete
Suggest
NearbyDeliveries
NearbyDrivers
GetSearchStats
StartReindex
GetReindexStatus
```

OpenSearch DSL remains internal to the Search Service.

---

# 40. Go Technology Stack

```text
Language:
Go

API:
GraphQL Federation subgraph

Search:
OpenSearch Go client

Messaging:
Kafka client for Go

Cache:
Redis client for Go

Background:
Go workers for continuous search event processing

Operational jobs:
BullMQ only where a Node-owned job boundary exists

Observability:
OpenTelemetry

Metrics:
Prometheus

Logging:
Structured JSON

Containers:
Docker

Orchestration:
Kubernetes

Local Kubernetes development:
Skaffold
```

---

# 41. Go Project Structure

```text
search-service/
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── domain/
│   │   ├── delivery/
│   │   ├── driver/
│   │   ├── media/
│   │   └── search/
│   │
│   ├── application/
│   │   ├── search/
│   │   ├── autocomplete/
│   │   ├── reindex/
│   │   └── suggestions/
│   │
│   ├── infrastructure/
│   │   ├── opensearch/
│   │   ├── kafka/
│   │   ├── redis/
│   │   ├── metrics/
│   │   └── tracing/
│   │
│   ├── interfaces/
│   │   ├── graphql/
│   │   └── health/
│   │
│   └── config/
│
├── migrations/
│   └── indexes/
│
├── deployments/
│   ├── docker/
│   └── kubernetes/
│
├── tests/
│   ├── unit/
│   ├── integration/
│   └── e2e/
│
├── scripts/
│   ├── create-indexes.sh
│   ├── reindex.sh
│   └── seed.sh
│
├── Dockerfile
├── docker-compose.yml
├── skaffold.yaml
├── go.mod
├── go.sum
└── README.md
```

---

# 42. Dependency Direction

```text
Interfaces
    |
    v
Application
    |
    v
Domain

Infrastructure
    |
    +---- implements application interfaces
```

Domain should not import OpenSearch/Kafka/Redis.

---

# 43. Repository Interface

Conceptually:

```go
type SearchRepository interface {
    SearchDeliveries(ctx context.Context, q DeliverySearchQuery) (SearchResult[DeliveryDocument], error)
    SearchDrivers(ctx context.Context, q DriverSearchQuery) (SearchResult[DriverDocument], error)
    SearchMedia(ctx context.Context, q MediaSearchQuery) (SearchResult[MediaDocument], error)

    UpsertDelivery(ctx context.Context, doc DeliveryDocument) error
    DeleteDelivery(ctx context.Context, id string) error

    Reindex(ctx context.Context, source string, target string) error
}
```

---

# 44. Search Request Flow

```text
1. Client sends GraphQL query
2. Gateway authenticates request
3. Gateway routes to Search Subgraph
4. Search Subgraph calls Search Service
5. Search Service validates input
6. Search Service builds query
7. Redis cache is checked
8. On miss, OpenSearch is queried
9. Results are transformed to DTOs
10. Safe results are cached
11. GraphQL returns response
```

---

# 45. Indexing Flow

```text
1. Delivery transaction commits
2. Outbox event is created
3. Event reaches Kafka
4. Search consumer receives event
5. Event is validated
6. Version is checked
7. Search document is built
8. OpenSearch UPSERT is executed
9. Metrics are recorded
```

---

# 46. Failure: OpenSearch Down

```text
Client
  |
  v
Search Service
  |
  X
OpenSearch
```

Return a controlled search-unavailable error.

Do **not** blindly fall back to full database scans because that can overload PostgreSQL/MongoDB.

A safe cached response may be used when appropriate.

---

# 47. Failure: Kafka Down

Existing search data remains available.

```text
Kafka unavailable
       |
       v
Consumer stops
       |
       v
Kafka retains events
       |
       v
Consumer resumes
       |
       v
Index catches up
```

---

# 48. Failure: Consumer Crash

Kafka offsets allow the consumer to resume.

Idempotent upserts make replay safe.

---

# 49. Failure: Duplicate Event

```text
Event 123
Event 123
```

Both become:

```text
UPSERT same document ID
```

No duplicate search document.

---

# 50. Failure: Out-of-Order Event

```text
version 8
version 7
```

If version 8 is already indexed:

```text
ignore version 7
```

---

# 51. Failure: Partial Bulk Failure

A bulk request can contain successful and failed operations.

The worker must:

```text
inspect item results
retry only failed items
record errors
send persistent failures to DLQ
```

---

# 52. Retry Strategy

Use:

```text
Exponential Backoff
+
Jitter
+
Maximum Attempts
```

Example:

```text
1s
2s
4s
8s
```

with jitter.

---

# 53. Dead Letter Queue

```text
Kafka
 |
 v
Search Consumer
 |
 +--> success
 |
 +--> retry
       |
       v
     DLQ
```

DLQ message:

```text
event_id
event_type
aggregate_id
error
attempts
first_failed_at
last_failed_at
payload
```

Never silently discard failed events.

---

# 54. Security

Never allow clients to submit arbitrary OpenSearch DSL.

Bad:

```graphql
search(queryDsl: JSON)
```

Better:

```graphql
searchDeliveries(input: DeliverySearchInput!)
```

The Search Service builds the query.

OpenSearch must never be exposed directly to the frontend.

---

# 55. Authorization

Search results must respect authorization.

Example:

```text
Customer
   |
   v
Search Deliveries
   |
   v
Only authorized deliveries
```

Admin can see only data permitted by role/policy.

If multi-tenancy is introduced later, every document should include:

```text
tenant_id
```

and every query must enforce it.

---

# 56. Rate Limiting

Apply rate limiting at:

```text
API Gateway
```

and optionally Search Service for defense in depth.

Protect especially:

```text
Autocomplete
Fuzzy Search
Geo Search
Large Aggregations
```

---

# 57. Query Cost Protection

Prevent:

```text
Huge page sizes
Deep pagination
Expensive wildcard queries
Unbounded aggregations
Very long query strings
```

Example initial limits:

```text
max page size = 100
max query length = controlled
aggregation bucket count = controlled
```

Tune by load testing.

---

# 58. Pagination

For large result sets, prefer:

```text
search_after
```

over deep:

```text
from + size
```

GraphQL should expose cursor-based pagination.

---

# 59. Cursor Design

```text
Page 1
  |
  v
cursor A
  |
  v
Page 2
```

Cursor can encode OpenSearch sort values.

Do not expose raw OpenSearch internals unnecessarily.

---

# 60. Sorting

Examples:

```text
relevance DESC
created_at DESC
updated_at DESC
distance ASC
rating DESC
```

Use deterministic secondary sorting:

```text
score DESC
created_at DESC
delivery_id ASC
```

---

# 61. Aggregations

Examples:

```text
Deliveries by status
Deliveries by city
Deliveries by vehicle type
```

Example:

```text
IN_TRANSIT = 120
DELIVERED = 500
CANCELLED = 30
```

Control aggregation size/cost.

---

# 62. Search Analytics

Track:

```text
Search count
Search latency
Cache hit ratio
Zero-result searches
OpenSearch errors
Kafka lag
Indexing latency
Projection lag
```

For high-volume analytics:

```text
Search Service
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

---

# 63. Observability

Use:

```text
OpenTelemetry
Prometheus
Grafana
Structured JSON Logging
```

Trace:

```text
GraphQL Gateway
      |
      v
Search Service
      |
      +--> Redis
      |
      +--> OpenSearch
```

Indexing trace:

```text
Delivery
   |
   v
Kafka
   |
   v
Search Consumer
   |
   v
OpenSearch
```

---

# 64. Important Metrics

Application:

```text
search_requests_total
search_errors_total
search_latency_ms
autocomplete_latency_ms
search_cache_hit_ratio
zero_result_rate
```

Kafka:

```text
consumer_lag
records_consumed
processing_errors
```

OpenSearch:

```text
query_latency
indexing_latency
rejected_requests
CPU
memory
disk
heap
shard_health
```

---

# 65. Projection Lag

Store:

```text
event_occurred_at
indexed_at
```

Calculate:

```text
projection_lag =
indexed_at - event_occurred_at
```

Metric:

```text
search_projection_lag_ms
```

This is an important distributed-system metric.

---

# 66. Reconciliation

Periodically detect:

```text
Source DB exists
but search document missing
```

or:

```text
Source version = 12
Search version = 10
```

Then enqueue a repair/reindex operation.

---

# 67. Docker

Local environment:

```text
search-service
opensearch
redis
kafka
```

Run only the dependencies needed for the current development task.

---

# 68. Kubernetes

Deploy:

```text
Search Service Deployment
Search Service Service
ConfigMap
Secret
HPA
```

OpenSearch should have its own lifecycle/deployment configuration.

---

# 69. HPA

```text
             +-- Search Pod 1
Gateway -----+-- Search Pod 2
             +-- Search Pod 3
```

Scale using:

```text
CPU
Memory
Requests/sec
Latency/custom metrics
```

For Kafka consumers, consumer lag is an important scaling signal.

---

# 70. Skaffold

```text
Source
  |
  v
Skaffold
  |
  +--> Build
  +--> Deploy
  +--> Watch
```

Use it for rapid local Kubernetes development.

---

# 71. Environment Configuration

```text
.env.example
config.example.yaml
```

Environments:

```text
local
development
staging
production
```

Never commit:

```text
OpenSearch credentials
Kafka credentials
Redis credentials
JWT secrets
Cloud credentials
```

---

# 72. Testing

## Unit

Test:

```text
Query Builder
Filter Builder
Ranking
Cursor Encoding
Version Checking
Idempotency
```

## Integration

Run against:

```text
OpenSearch
Redis
Kafka
```

## E2E

```text
GraphQL
 -> Search Service
 -> OpenSearch
```

## Load

Test progressively:

```text
100 req/s
500 req/s
1000 req/s
```

Measure:

```text
p50
p95
p99
```

---

# 73. Search Relevance Evaluation

Create a dataset:

```text
Query
Expected Results
Actual Results
```

Measure:

```text
Precision
Recall
MRR
NDCG
```

This makes search quality measurable.

---

# 74. Future Relevance Pipeline

Initial:

```text
BM25
```

Then:

```text
BM25
+
Filters
+
Business Signals
```

Future:

```text
BM25
+
Vector Search
+
Reranking
```

---

# 75. Future GenAI Phase

GenAI is intentionally deferred.

Initial:

```text
Query
 |
 v
BM25
 |
 v
Results
```

Future:

```text
             Query
              |
       +------+------+
       |             |
       v             v
     BM25        Embedding
       |             |
       |             v
       |        Vector Search
       |             |
       +------+------+
              |
              v
        Hybrid Search
              |
              v
           Reranker
              |
              v
           Results
```

OpenSearch supports dense vector, sparse vector, and hybrid search capabilities. citeturn0search5turn0search7

---

# 76. Qdrant vs OpenSearch Future

The project already has Qdrant as a planned vector/recommendation technology.

Two possible future architectures:

### Option A

```text
Search Service
      |
      v
OpenSearch
 |
 +-- BM25
 +-- Vector
```

### Option B

```text
Search Service
   |
   +------ OpenSearch
   |          |
   |         BM25
   |
   +------ Qdrant
              |
            Vector
```

For this project:

```text
OpenSearch = primary search engine
Qdrant = future specialized vector/recommendation workload
```

Do not introduce both into the first Search Service implementation.

---

# 77. Why This Service Is Valuable for System Design

It demonstrates:

```text
Inverted Index
BM25
Distributed Search
Sharding
Replication
Eventual Consistency
CQRS Read Model
Denormalization
Caching
Kafka
Consumer Groups
Backpressure
Bulk Processing
Idempotency
Reindexing
Geo Search
Ranking
Pagination
Observability
Horizontal Scaling
Failure Recovery
```

This makes Search Service one of the strongest services in the project for System Design Interview practice.

---

# 78. Why Not PostgreSQL Search?

PostgreSQL is excellent for transactional workloads.

A specialized search engine becomes valuable when requirements include:

```text
Relevance
Autocomplete
Fuzzy search
Faceting
Geo search
Advanced ranking
Large-scale indexing
Vector search
```

Search Service isolates these workloads.

---

# 79. Consistency Model

Search is eventually consistent.

```text
Business Transaction
      |
      v
PostgreSQL commit
      |
      v
Outbox
      |
      v
Kafka
      |
      v
Search Consumer
      |
      v
OpenSearch
```

A small propagation delay is expected.

---

# 80. Read-After-Write

Immediately after:

```text
Create Delivery
```

the search index may temporarily not contain the new delivery.

For immediate authoritative data:

```text
Delivery Service
```

For search/discovery:

```text
Search Service
```

Do not force a distributed transaction between PostgreSQL and OpenSearch.

---

# 81. Architecture With All Technologies

```text
                         CLIENT
                            |
                         GraphQL
                            |
                            v
                  +--------------------+
                  | API Gateway        |
                  | NestJS Federation  |
                  +---------+----------+
                            |
                            v
                  +--------------------+
                  | Search Subgraph    |
                  +---------+----------+
                            |
                            v
                  +--------------------+
                  | Search Service     |
                  | Go                 |
                  +----+----------+----+
                       |          |
                       v          v
                    Redis      OpenSearch
                    Cache      Search Index
                                  ^
                                  |
                               Kafka
                                  ^
                                  |
                         Source Services
```

Other technologies:

```text
NATS
  -> transient low-latency signals

NATS JetStream
  -> optional durable NATS use case

BullMQ
  -> Node-owned operational jobs

WebSocket
  -> Realtime Service

SSE
  -> optional realtime transport

ClickHouse
  -> Search analytics

Qdrant
  -> future vector/recommendation workload

FastAPI
  -> future GenAI service

Docker
  -> containers

Kubernetes
  -> orchestration

Skaffold
  -> local Kubernetes workflow
```

---

# 82. Complete Search Event Flow

```text
                  DELIVERY SERVICE
                         |
                  PostgreSQL TX
                         |
                       Outbox
                         |
                         v
                       Kafka
                         |
                         v
                  Search Consumer
                         |
              +----------+----------+
              |                     |
              v                     v
          Validation           Version Check
                                    |
                                    v
                             Document Builder
                                    |
                                    v
                               OpenSearch
```

---

# 83. Complete Search Query Flow

```text
CLIENT
  |
  | GraphQL
  v
API Gateway
  |
  v
Search Subgraph
  |
  v
Search Service
  |
  +------> Redis
  |          |
  |       cache hit
  |
  +------> OpenSearch
              |
              v
           Ranking
              |
              v
           Results
              |
              v
        GraphQL Response
```

---

# 84. Search + Analytics

```text
Search Service
      |
      +------------------+
      |                  |
      v                  v
OpenSearch              Kafka
                           |
                           v
                    Analytics Service
                           |
                           v
                       ClickHouse
```

---

# 85. Search + Realtime

Search does not own browser connections.

```text
Search Service
      |
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
Admin Dashboard
```

Example:

```text
Reindex Started
Reindex 25%
Reindex 50%
Reindex 100%
```

---

# 86. Health Checks

Internal health endpoints:

```text
/health/live
/health/ready
```

Readiness should verify the dependencies required by the service startup policy.

These are infrastructure endpoints, not the client business API.

---

# 87. Graceful Shutdown

On SIGTERM:

```text
1. Stop accepting new work
2. Stop Kafka consumption
3. Finish in-flight indexing
4. Flush telemetry
5. Close OpenSearch
6. Close Redis
7. Close Kafka
8. Exit
```

---

# 88. Backpressure

If OpenSearch becomes slow:

```text
Kafka
  |
  v
Consumer
  |
  v
OpenSearch
  X
```

Do not endlessly increase concurrency.

Use:

```text
Bounded worker pool
Batching
Retry
Backoff
Controlled consumption
```

Kafka acts as the durable buffer.

---

# 89. Capacity Planning

Start with explicit assumptions, for example:

```text
Search:
100–500 requests/sec

Index events:
50–200 events/sec

Autocomplete:
higher request rate

Concurrent users:
thousands
```

These are test targets, not guaranteed capacity.

Measure:

```text
p50
p95
p99
```

for:

```text
Search latency
Autocomplete latency
Indexing latency
Kafka lag
```

---

# 90. Performance Rules

1. Keep documents reasonably small.
2. Index only fields required for search.
3. Disable indexing for fields never queried.
4. Use `keyword` for exact filters.
5. Use `text` for full-text search.
6. Use `geo_point` for locations.
7. Use bulk indexing.
8. Use `search_after` for deep pagination.
9. Cache safe repetitive queries.
10. Avoid expensive wildcard queries.
11. Control aggregations.
12. Monitor shard sizes.

---

# 91. Mapping Principles

Example:

```json
{
  "name": {
    "type": "text",
    "fields": {
      "keyword": {
        "type": "keyword"
      }
    }
  },
  "status": {
    "type": "keyword"
  },
  "created_at": {
    "type": "date"
  },
  "location": {
    "type": "geo_point"
  },
  "id": {
    "type": "keyword"
  }
}
```

This allows:

```text
name -> full-text search
name.keyword -> exact match/sort
status -> filtering
location -> geo search
```

---

# 92. Search DSL Isolation

Only the infrastructure adapter should know OpenSearch DSL.

```text
GraphQL
   |
   v
Application Use Case
   |
   v
Search Repository Interface
   |
   v
OpenSearch Adapter
   |
   v
OpenSearch DSL
```

This keeps business/application code independent from the search engine.

---

# 93. Search Document Version

Store:

```text
source_version
indexed_at
```

Example:

```json
{
  "delivery_id": "del_123",
  "source_version": 12,
  "indexed_at": "2026-08-17T10:30:00Z"
}
```

This makes stale projections and indexing lag easier to diagnose.

---

# 94. Reconciliation

Detect:

```text
Source exists
but Search document missing
```

or:

```text
Source version = 12
Search version = 10
```

Then:

```text
enqueue repair
```

---

# 95. Search Integrity Checks

Periodically verify:

```text
Document counts
Missing IDs
Stale versions
Failed events
DLQ size
Kafka lag
Index health
```

---

# 96. DLQ Recovery

```text
DLQ
 |
 v
Inspect
 |
 +---- Retry
 |
 +---- Repair source
 |
 +---- Reindex aggregate
 |
 +---- Discard only after explicit investigation
```

---

# 97. Security / Secrets

Production communications should support secure transport where applicable:

```text
Client -> Gateway: HTTPS
Service -> OpenSearch: TLS
Service -> Kafka: TLS
Service -> Redis: TLS where configured
```

Secrets belong in Kubernetes Secrets/secret-management infrastructure.

Never commit credentials.

---

# 98. Logging

Every search operation should have:

```text
request_id
trace_id
user_id
query_hash
index
latency
result_count
cache_hit
```

Avoid logging sensitive raw search content unnecessarily.

---

# 99. Query Privacy

Search strings may contain personal information.

Prefer:

```text
query_hash
redacted query
```

when raw query logging is not required.

---

# 100. Versioning

Index:

```text
deliveries-v1
deliveries-v2
```

Events:

```text
event_version = 1
```

Mappings and event schemas should be explicitly versioned.

---

# 101. Development Phases

## Phase 1 — Basic Search

```text
Go Search Service
OpenSearch
GraphQL Subgraph
Delivery index
Full-text search
Filters
Sorting
Pagination
```

## Phase 2 — Event Driven

```text
Kafka
Outbox
Search Consumer
Idempotency
Version checking
Delete events
```

## Phase 3 — Search Quality

```text
BM25
Analyzers
Autocomplete
Fuzzy search
Synonyms
Highlighting
Relevance tests
```

## Phase 4 — Advanced Search

```text
Geo search
Aggregations
search_after
Redis cache
Query cost controls
```

## Phase 5 — Reliability

```text
Retry
Backoff
DLQ
Reconciliation
Reindex
Bulk indexing
Alias switching
Graceful shutdown
```

## Phase 6 — Observability

```text
OpenTelemetry
Prometheus
Grafana
Kafka lag
Projection lag
Search latency
OpenSearch health
```

## Phase 7 — Infrastructure

```text
Docker
Kubernetes
HPA
ConfigMaps
Secrets
Skaffold
```

## Phase 8 — System Design Scale

```text
Multi-node OpenSearch
Sharding
Replicas
Load testing
Capacity estimation
Failure injection
Disaster recovery
```

## Phase 9 — Future GenAI

```text
Embeddings
Vector search
Hybrid search
Reranking
Semantic search
RAG
```

---

# 102. OpenSearch vs Elasticsearch Decision

| Area | OpenSearch | Elasticsearch |
|---|---|---|
| Full-text search | Excellent | Excellent |
| BM25 | Yes | Yes |
| Analyzers | Yes | Yes |
| Autocomplete | Yes | Yes |
| Geo search | Yes | Yes |
| Aggregations | Yes | Yes |
| Vector search | Yes | Yes |
| Hybrid search | Yes | Yes |
| Distributed architecture | Yes | Yes |
| Portfolio learning value | Excellent | Excellent |
| Recommended for this project | **Yes** | No |
| Run both? | **No** | **No** |

OpenSearch provides BM25, vector search, AI search, search pipelines, autocomplete, filtering, aggregations, and relevance optimization. citeturn0search0turn0search5

Elasticsearch provides similarly broad search, analytics, vector, geo, and relevance capabilities. citeturn0search2turn0search4

The decision here is about **project architecture and learning focus**, not because Elasticsearch is technically inferior.

---

# 103. Final Recommended Architecture

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
                                v
                     +----------------------+
                     |    Search Subgraph   |
                     +----------+-----------+
                                |
                                v
                     +----------------------+
                     |    Search Service    |
                     |        Go            |
                     +----------+-----------+
                                |
             +------------------+------------------+
             |                  |                  |
             v                  v                  v
          Redis             OpenSearch           Kafka
          Cache             Search Index         Events
                                                   ^
                                                   |
                                          Source Services
```

---

# 104. Final Technology Rules

```text
1. OpenSearch is the only search engine in the initial architecture.
2. PostgreSQL/MongoDB remain source-of-truth databases.
3. OpenSearch is a rebuildable read projection.
4. Kafka is the durable indexing event pipeline.
5. Redis is cache/protection/temporary state.
6. NATS is transient low-latency messaging only.
7. JetStream is optional and use-case driven.
8. BullMQ is for Node-owned operational jobs.
9. Go workers process the Search Service Kafka stream.
10. WebSocket belongs to Realtime Service.
11. SSE is optional, not primary.
12. GraphQL Federation is the client-facing interface.
13. OpenSearch DSL stays inside the infrastructure adapter.
14. Search indexing is idempotent.
15. Event versions prevent stale projections.
16. DLQ handles persistent indexing failures.
17. Reindexing is a first-class operation.
18. Search is eventually consistent.
19. Authorization is enforced before returning results.
20. GenAI/vector/hybrid search is a future phase.
```

---

# 105. Implementation Checklist

- [ ] Go Search Service
- [ ] GraphQL Search Subgraph
- [ ] OpenSearch local environment
- [ ] Delivery index
- [ ] Driver index
- [ ] Media index
- [ ] Index aliases
- [ ] Mappings
- [ ] Analyzers
- [ ] BM25 search
- [ ] Filters
- [ ] Sorting
- [ ] Cursor pagination
- [ ] Autocomplete
- [ ] Fuzzy search
- [ ] Geo search
- [ ] Redis cache
- [ ] Kafka consumer
- [ ] Outbox integration
- [ ] Idempotency
- [ ] Event version checking
- [ ] Delete events
- [ ] Retry/backoff
- [ ] DLQ
- [ ] Bulk indexing
- [ ] Reindexing
- [ ] Alias switching
- [ ] Reconciliation
- [ ] OpenTelemetry
- [ ] Prometheus
- [ ] Grafana
- [ ] Structured logging
- [ ] Docker
- [ ] Kubernetes
- [ ] HPA
- [ ] Skaffold
- [ ] Unit tests
- [ ] Integration tests
- [ ] E2E tests
- [ ] Load tests
- [ ] Capacity estimation
- [ ] Failure testing
- [ ] Future GenAI plan documented
