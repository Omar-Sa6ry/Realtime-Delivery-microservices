# Driver & Dispatch Service — خطة التنفيذ الشاملة

## نظرة عامة

بعد مراجعة شاملة لكل ملفات المشروع، هذه الخطة المفصّلة لبناء `driver-service` بالكامل وتكاملها مع باقي الخدمات.

---

## ما تم مراجعته

| الملف / المجلد | الملاحظة |
|---|---|
| `system_articture.md` | المرجع الأساسي للمعمارية الكاملة |
| `driver_dispatch_service_system_architecture.md` | المرجع التفصيلي للـ driver service |
| `packages/go/` | الباكدجات المشتركة: kafka, nats, events, auth, logging, metrics |
| `protos/driver.proto` | **ناقص** — يحتاج تحديث جذري |
| `infrastructure/docker/compose.yml` | لا يوجد driver service — يحتاج إضافة |
| `infrastructure/kubernetes/base/` | لا يوجد driver-depl.yaml — يحتاج إضافة |
| `infrastructure/skaffold/skaffold.yaml` | لا يوجد driver artifact — يحتاج إضافة |
| `infrastructure/kubernetes/base/kustomization.yaml` | لا يوجد driver — يحتاج إضافة |
| `graphql-docs/` | لا يوجد driver.graphql — يحتاج إنشاء |
| `services/delivery-service/src/modules/infrastructure/grpc/` | يستخدم gRPC لكن بدون driver client |
| `packages/go/events/driver_events.go` | موجود لكن ناقص (لا يشمل assignment events) |
| `packages/go/nats/events.go` | موجود ويشمل NATS subjects للـ driver |

---

## مشاكل مكتشفة تحتاج إصلاح

> [!WARNING]
> هذه مشاكل موجودة حالياً في المشروع يجب إصلاحها

### 1. `protos/driver.proto` — ناقص جداً

الملف الحالي:
```proto
rpc IsAssignedDriver
rpc AssignDriver
rpc ReleaseDriver
```

المطلوب حسب الأرشيتكتشر:
```proto
rpc FindAvailableDrivers  ← ناقص
rpc ReserveDriver         ← ناقص
rpc ReleaseDriver
rpc GetDriver             ← ناقص
rpc GetDriverStatus       ← ناقص
rpc AssignDriver
```

### 2. `packages/go/events/driver_events.go` — ناقص

يحتوي فقط على `DriverCreated`, `DriverUpdated`, `DriverDeleted`.

الناقص:
- `DriverAssignmentOffered`
- `DriverAssignmentAccepted`
- `DriverAssignmentRejected`
- `DriverAssignmentExpired`
- `DriverAssignmentReleased`
- `DriverAvailable`
- `DriverUnavailable`

### 3. `infrastructure/docker/compose.yml` — لا يوجد driver service أو MongoDB

### 4. `infrastructure/kubernetes/base/` — لا يوجد driver deployment أو MongoDB

### 5. `infrastructure/skaffold/skaffold.yaml` — لا يوجد driver artifact

### 6. تعارض Ports حقيقي

> [!CAUTION]
> **تعارض Port 4003:** `media-service` يستخدم `4003:4003` لـ WebSocket، و`delivery-service` يستخدم `4003:4003` لـ HTTP في نفس الـ compose.yml — هذا تعارض host port حقيقي!

---

## Port Map الكامل للمشروع (بعد الإصلاح)

| Service | Host HTTP | Container HTTP | gRPC Host | gRPC Container | Metrics Host | Metrics Container |
|---|---|---|---|---|---|---|
| api-gateway | 4000 | 4000 | — | — | — | — |
| user-service | 4001 | 4001 | 50051 | 50051 | — | — |
| media-service GraphQL | 4005 | 4005 | 50052 | 50052 | 9102 | 9102 |
| media-service WS | **4009** | 4003 | — | — | — | — |
| notification-service | 4004 | 4004 | 50053 | 50053 | — | — |
| delivery-service | 4003 | 4003 | 50054 | 50054 | 9104 | 9104 |
| realtime-service | 4006 | 4006 | — | — | — | — |
| search-service | 4007 | 4007 | — | — | 9103 | 9103 |
| **driver-service** | **4008** | **4008** | **50055** | **50055** | **9105** | **9105** |

---

## علاقة driver-service بباقي الخدمات

### جدول العلاقات الكامل

| من | إلى | البروتوكول | الغرض |
|---|---|---|---|
| delivery-service | **driver-service** | **gRPC** | FindAvailableDrivers, ReserveDriver, ReleaseDriver, GetDriver |
| **driver-service** | realtime-service | **NATS** | driver.assignment.offered — إشعار السائق عبر WebSocket |
| realtime-service | **driver-service** | **NATS** | driver.location.updated — Location من الـ WebSocket |
| **driver-service** | Kafka | **Kafka** | driver.created, driver.assignment.accepted/rejected/expired |
| notification-service | Kafka | **consume** | يستهلك driver.assignment.accepted/rejected |
| search-service | Kafka | **consume** | يستهلك driver.created/updated لتحديث OpenSearch |
| api-gateway | **driver-service** | **GraphQL Federation** | الـ Driver subgraph |
| **driver-service** | Kafka (delivery) | **consume** | يستهلك delivery.completed/cancelled لتحرير السائق |

### الخدمات المستقبلية

| الخدمة | علاقتها بـ driver-service |
|---|---|
| payment-service | تؤثر غير مباشرة عبر Saga (delivery-service → driver-service ReleaseDriver) |
| analytics-service | تستهلك كل Kafka events من driver-service لـ ClickHouse |
| AI-service (FastAPI) | ستستدعي driver-service gRPC لـ dispatch optimization |

---

## هيكل ملفات driver-service

مبني على نفس نمط `media-service` و`search-service`:

```
services/driver-service/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── domain/
│   │   ├── driver.go          # Driver aggregate + state machine
│   │   ├── assignment.go      # Assignment aggregate
│   │   ├── dispatch.go        # Dispatch rules
│   │   ├── state.go           # State transitions validation
│   │   └── errors.go          # Domain errors
│   │
│   ├── ports/
│   │   ├── driver_repository.go
│   │   ├── assignment_repository.go
│   │   ├── location_store.go       # Redis GEO interface
│   │   ├── lock_manager.go         # Redis lock interface
│   │   ├── event_publisher.go      # Kafka interface
│   │   └── idempotency_store.go
│   │
│   ├── adapters/
│   │   ├── mongodb/
│   │   │   ├── driver_repository.go
│   │   │   ├── assignment_repository.go
│   │   │   ├── dispatch_attempt_repository.go
│   │   │   └── idempotency_repository.go
│   │   ├── redis/
│   │   │   ├── geo_store.go        # GEOADD / GEOSEARCH
│   │   │   ├── lock_manager.go     # Distributed lock
│   │   │   └── cache.go
│   │   ├── kafka/
│   │   │   ├── publisher.go
│   │   │   └── consumer.go         # Consumes delivery events
│   │   ├── nats/
│   │   │   ├── publisher.go
│   │   │   └── subscriber.go       # Location updates
│   │   └── grpc/
│   │       └── server.go
│   │
│   ├── application/
│   │   ├── commands/
│   │   │   ├── go_online.go
│   │   │   ├── go_offline.go
│   │   │   ├── reserve_driver.go
│   │   │   ├── accept_assignment.go
│   │   │   ├── reject_assignment.go
│   │   │   └── release_driver.go
│   │   ├── queries/
│   │   │   ├── find_available_drivers.go
│   │   │   ├── get_driver.go
│   │   │   └── get_assignment.go
│   │   └── services/
│   │       └── dispatch_service.go
│   │
│   ├── transport/
│   │   └── graphql/
│   │       ├── server.go
│   │       ├── schema.go
│   │       └── handler.go
│   │
│   ├── workers/
│   │   ├── pool.go
│   │   ├── assignment_expiry.go     # Expires OFFERED past expiresAt
│   │   ├── reconciliation.go        # Repairs inconsistent state
│   │   └── heartbeat_monitor.go     # Detects stale drivers
│   │
│   ├── observability/
│   │   ├── metrics.go               # Prometheus
│   │   └── tracing.go               # OpenTelemetry
│   │
│   └── validation/
│       └── location.go
│
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── Makefile
```

---

## كل الملفات المطلوب تعديلها أو إنشاؤها

### المرحلة 1 — إصلاحات عاجلة

---

#### [MODIFY] [protos/driver.proto](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/protos/driver.proto)

```proto
syntax = "proto3";
package driver;
option go_package = "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/protos/driver";

service DriverService {
  rpc FindAvailableDrivers (FindAvailableDriversRequest) returns (FindAvailableDriversResponse);
  rpc ReserveDriver        (ReserveDriverRequest)        returns (ReserveDriverResponse);
  rpc ReleaseDriver        (ReleaseDriverRequest)        returns (ReleaseDriverResponse);
  rpc AssignDriver         (AssignDriverRequest)         returns (AssignDriverResponse);
  rpc GetDriver            (GetDriverRequest)            returns (GetDriverResponse);
  rpc GetDriverStatus      (GetDriverStatusRequest)      returns (GetDriverStatusResponse);
}

message FindAvailableDriversRequest {
  double latitude      = 1;
  double longitude     = 2;
  double radiusKm      = 3;
  string vehicleType   = 4;
  string deliveryId    = 5;
  string correlationId = 6;
}
message DriverCandidate {
  string driverId       = 1;
  double distanceMeters = 2;
  string vehicleType    = 3;
  double latitude       = 4;
  double longitude      = 5;
}
message FindAvailableDriversResponse {
  repeated DriverCandidate candidates = 1;
}

message ReserveDriverRequest {
  string driverId       = 1;
  string deliveryId     = 2;
  string idempotencyKey = 3;
  string correlationId  = 4;
}
message ReserveDriverResponse {
  bool   reserved     = 1;
  string driverId     = 2;
  string deliveryId   = 3;
  string assignmentId = 4;
}

message ReleaseDriverRequest {
  string driverId      = 1;
  string deliveryId    = 2;
  string reason        = 3;
  string correlationId = 4;
}
message ReleaseDriverResponse {
  bool   released = 1;
  string driverId = 2;
}

message AssignDriverRequest {
  string driverId      = 1;
  string deliveryId    = 2;
  string correlationId = 3;
}
message AssignDriverResponse {
  bool   assigned     = 1;
  string driverId     = 2;
  string deliveryId   = 3;
  string assignmentId = 4;
}

message GetDriverRequest  { string driverId = 1; }
message GetDriverResponse {
  bool   found       = 1;
  string driverId    = 2;
  string userId      = 3;
  string status      = 4;
  string vehicleType = 5;
}

message GetDriverStatusRequest  { string driverId = 1; }
message GetDriverStatusResponse {
  string driverId            = 1;
  string status              = 2;
  bool   hasActiveAssignment = 3;
  string activeDeliveryId    = 4;
}
```

---

#### [MODIFY] [packages/go/events/driver_events.go](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/packages/go/events/driver_events.go)

إضافة Event Types وPayloads ناقصة:
```go
const (
    DriverCreated             DriverEventType = "driver.created"
    DriverUpdated             DriverEventType = "driver.updated"
    DriverDeleted             DriverEventType = "driver.deleted"
    DriverActivated           DriverEventType = "driver.activated"
    DriverDeactivated         DriverEventType = "driver.deactivated"
    DriverAvailable           DriverEventType = "driver.available"
    DriverUnavailable         DriverEventType = "driver.unavailable"
    DriverAssignmentOffered   DriverEventType = "driver.assignment.offered"
    DriverAssignmentAccepted  DriverEventType = "driver.assignment.accepted"
    DriverAssignmentRejected  DriverEventType = "driver.assignment.rejected"
    DriverAssignmentExpired   DriverEventType = "driver.assignment.expired"
    DriverAssignmentReleased  DriverEventType = "driver.assignment.released"
    DriverAssignmentCompleted DriverEventType = "driver.assignment.completed"
)
```

---

#### [MODIFY] [infrastructure/docker/compose.yml](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/infrastructure/docker/compose.yml)

1. إصلاح تعارض port 4003 (media WS → 4009:4003)
2. إضافة `driver-db-srv` (MongoDB)
3. إضافة `driver-service`
4. إضافة `DRIVER_SERVICE_URL` لـ api-gateway
5. إضافة `driver_db_data` لـ volumes

---

### المرحلة 2 — البنية التحتية

---

#### [NEW] `infrastructure/kubernetes/base/services/driver-depl.yaml`

```yaml
# Deployment + HPA + PDB + NetworkPolicy + ServiceAccount + Service
# Image: delivery/driver
# Ports: 4008 (graphql), 50055 (grpc), 9105 (metrics)
# initContainers: wait-for-kafka, wait-for-nats, wait-for-mongodb
# readinessProbe / livenessProbe على /health/ready و /health/live
```

#### [NEW] `infrastructure/kubernetes/base/infrastructure/driver-db-depl.yaml`

```yaml
# MongoDB StatefulSet/Deployment
# Port: 27017 (ClusterIP only — لا يُعرض خارجياً)
# PersistentVolumeClaim
# Service: driver-db-srv
```

#### [NEW] `infrastructure/kubernetes/base/configmaps/driver-service-config.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: driver-service-config
data:
  PORT_DRIVER_GRAPHQL:          "4008"
  PORT_DRIVER_GRPC:             "50055"
  PORT_DRIVER_METRICS:          "9105"
  MONGODB_DATABASE:             "driver_db"
  DISPATCH_SEARCH_RADIUS_KM:    "5"
  ASSIGNMENT_TIMEOUT_SECONDS:   "20"
  LOCATION_STALE_SECONDS:       "30"
  LOCK_TTL_MS:                  "5000"
  MAX_DISPATCH_ATTEMPTS:        "5"
  KAFKA_GROUP_ID:               "driver-service"
  KAFKA_CLIENT_ID:              "driver-service"
```

#### [NEW] `infrastructure/kubernetes/base/secrets/driver-secrets.yaml`

```yaml
# MONGODB_URI, JWT_SECRET, etc.
```

#### [MODIFY] [infrastructure/kubernetes/base/kustomization.yaml](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/infrastructure/kubernetes/base/kustomization.yaml)

إضافة:
```yaml
- configmaps/driver-service-config.yaml
- secrets/driver-secrets.yaml
- services/driver-depl.yaml
- infrastructure/driver-db-depl.yaml
```

#### [MODIFY] [infrastructure/skaffold/skaffold.yaml](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/infrastructure/skaffold/skaffold.yaml)

إضافة:
```yaml
- image: delivery/driver
  context: .
  docker:
    dockerfile: services/driver-service/Dockerfile
```

---

### المرحلة 3 — ملفات driver-service

---

#### [NEW] `services/driver-service/docker-compose.yml`

على نفس نمط [search-service/docker-compose.yml](file:///d:/projects/Back-End/Realtime%20Delivery%20microservices/services/search-service/docker-compose.yml):

```yaml
services:
  driver-db-srv:
    image: mongo:7
    container_name: driver-mongo-srv
    ports: ["27017:27017"]
    volumes:
      - driver_db_data:/data/db

  redis-srv:
    image: redis:alpine
    container_name: driver-redis-srv
    ports: ["6379:6379"]

  kafka-srv:
    image: apache/kafka:3.8.0
    container_name: driver-kafka-srv
    ports: ["9092:9092"]
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: CONTROLLER://:9093,PLAINTEXT://:9092
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka-srv:9092
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka-srv:9093
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      CLUSTER_ID: 4L6g3nShT-eMCtK--X86sw

  nats-srv:
    image: nats:2-alpine
    container_name: driver-nats-srv
    ports: ["4222:4222", "8222:8222"]

  driver-service:
    build:
      context: ../..
      dockerfile: services/driver-service/Dockerfile
    container_name: driver-service-local
    restart: unless-stopped
    ports:
      - "4008:4008"
      - "50055:50055"
      - "9105:9105"
    environment:
      PORT_DRIVER_GRAPHQL:        "4008"
      PORT_DRIVER_GRPC:           "50055"
      PORT_DRIVER_METRICS:        "9105"
      MONGODB_URI:                "mongodb://driver-db-srv:27017"
      MONGODB_DATABASE:           "driver_db"
      REDIS_HOST:                 "redis-srv"
      REDIS_PORT:                 "6379"
      KAFKA_BROKERS:              "kafka-srv:9092"
      KAFKA_GROUP_ID:             "driver-service"
      KAFKA_CLIENT_ID:            "driver-service"
      NATS_URL:                   "nats://nats-srv:4222"
      DISPATCH_SEARCH_RADIUS_KM:  "5"
      ASSIGNMENT_TIMEOUT_SECONDS: "20"
      LOCATION_STALE_SECONDS:     "30"
      LOCK_TTL_MS:                "5000"
      MAX_DISPATCH_ATTEMPTS:      "5"
    depends_on:
      - driver-db-srv
      - redis-srv
      - kafka-srv
      - nats-srv
    volumes:
      - ./internal:/workspace/services/driver-service/internal
      - ../../packages/go:/workspace/packages/go

volumes:
  driver_db_data:
```

#### [NEW] `services/driver-service/Dockerfile`

```dockerfile
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /workspace
COPY packages/go/go.mod packages/go/go.sum ./packages/go/
COPY services/driver-service/go.mod services/driver-service/go.sum ./services/driver-service/
WORKDIR /workspace/services/driver-service
RUN --mount=type=cache,target=/go/pkg/mod go mod download
WORKDIR /workspace
COPY packages/go ./packages/go
WORKDIR /workspace/services/driver-service
COPY services/driver-service .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-w -s" \
    -o /workspace/bin/driver-service ./cmd/server

FROM alpine:3.19 AS runtime
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /workspace/bin/driver-service /driver-service
USER 10001:10001
EXPOSE 4008 50055 9105
ENTRYPOINT ["/driver-service"]
```

#### [NEW] `services/driver-service/go.mod`

```go
module github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service

go 1.25.0

require (
    github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go v0.0.0
    go.mongodb.org/mongo-driver/v2 v2.0.0
    github.com/redis/go-redis/v9 v9.22.0
    github.com/graph-gophers/graphql-go v1.5.0
    google.golang.org/grpc v1.65.0
    google.golang.org/protobuf v1.36.11
    github.com/segmentio/kafka-go v0.4.47
    github.com/nats-io/nats.go v1.38.0
    go.opentelemetry.io/otel v1.28.0
    go.opentelemetry.io/otel/exporters/prometheus v0.50.0
    github.com/prometheus/client_golang v1.24.1
    go.uber.org/zap v1.27.0
    github.com/google/uuid v1.6.0
)

replace github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go => ../../packages/go
```

#### [NEW] كود Go (كل الملفات بترتيب التنفيذ)

1. `internal/domain/errors.go` — Domain errors
2. `internal/domain/driver.go` — Driver struct + state machine
3. `internal/domain/assignment.go` — Assignment struct + transitions
4. `internal/domain/dispatch.go` — Dispatch policy
5. `internal/domain/state.go` — ValidateTransition()
6. `internal/ports/driver_repository.go`
7. `internal/ports/assignment_repository.go`
8. `internal/ports/location_store.go` — GeoAdd, GeoSearch
9. `internal/ports/lock_manager.go` — Acquire, Release
10. `internal/ports/event_publisher.go` — Kafka, NATS
11. `internal/ports/idempotency_store.go`
12. `internal/adapters/mongodb/driver_repository.go`
13. `internal/adapters/mongodb/assignment_repository.go`
14. `internal/adapters/mongodb/dispatch_attempt_repository.go`
15. `internal/adapters/mongodb/idempotency_repository.go`
16. `internal/adapters/redis/geo_store.go`
17. `internal/adapters/redis/lock_manager.go`
18. `internal/adapters/redis/cache.go`
19. `internal/adapters/kafka/publisher.go`
20. `internal/adapters/kafka/consumer.go`
21. `internal/adapters/nats/publisher.go`
22. `internal/adapters/nats/subscriber.go`
23. `internal/adapters/grpc/server.go`
24. `internal/application/services/dispatch_service.go`
25. `internal/application/commands/go_online.go`
26. `internal/application/commands/go_offline.go`
27. `internal/application/commands/reserve_driver.go`
28. `internal/application/commands/accept_assignment.go`
29. `internal/application/commands/reject_assignment.go`
30. `internal/application/commands/release_driver.go`
31. `internal/application/queries/find_available_drivers.go`
32. `internal/application/queries/get_driver.go`
33. `internal/application/queries/get_assignment.go`
34. `internal/workers/pool.go`
35. `internal/workers/assignment_expiry.go`
36. `internal/workers/reconciliation.go`
37. `internal/workers/heartbeat_monitor.go`
38. `internal/transport/graphql/schema.go`
39. `internal/transport/graphql/handler.go`
40. `internal/transport/graphql/server.go`
41. `internal/observability/metrics.go`
42. `internal/observability/tracing.go`
43. `internal/validation/location.go`
44. `internal/config/config.go`
45. `cmd/server/main.go`

---

### المرحلة 4 — التكامل

---

#### [MODIFY] delivery-service — إضافة Driver gRPC Client

في `services/delivery-service/src/modules/infrastructure/grpc/`:
- إنشاء `grpc.client.driver.ts` — يستدعي `FindAvailableDrivers`, `ReserveDriver`, `ReleaseDriver`, `GetDriverStatus`
- تعديل `grpc.module.ts` لتسجيل الـ driver client

#### [NEW] `graphql-docs/driver.graphql`

```graphql
# ============================================================
# DRIVER SERVICE — GraphQL Operations Reference
# Port: 4008 | Subgraph: /driver/graphql
# ============================================================

# ─── QUERIES ───────────────────────────────────────────────

# Get Driver Service Health
query GetDriverServiceInfo {
  driverServiceInfo {
    success
    statusCode
    message
    timeStamp
    data { name  version  status }
  }
}

# Get Driver by ID
query GetDriver {
  driver(id: "driver-123") {
    success
    statusCode
    message
    timeStamp
    data {
      id
      userId
      status        # OFFLINE | AVAILABLE | BUSY
      vehicle {
        type         # CAR | MOTORCYCLE | TRUCK
        plateNumber
        capacityKg
      }
      capabilities
      serviceArea
      rating
      createdAt
      updatedAt
    }
  }
}

# Get My Driver Profile (authenticated driver)
query GetMyDriverProfile {
  myDriverProfile {
    success
    statusCode
    message
    timeStamp
    data {
      id
      userId
      status
      vehicle { type  plateNumber  capacityKg }
      capabilities
      serviceArea
      rating
      createdAt
      updatedAt
    }
  }
}

# Get Driver Active Assignment
query GetDriverActiveAssignment {
  driverActiveAssignment(driverId: "driver-123") {
    success
    statusCode
    message
    timeStamp
    data {
      id
      deliveryId
      driverId
      status        # OFFERED|ACCEPTED|ACTIVE|COMPLETED|REJECTED|EXPIRED|CANCELLED
      attemptNumber
      offeredAt
      expiresAt
      acceptedAt
      rejectedAt
      completedAt
      createdAt
      updatedAt
    }
  }
}

# Get Driver Status
query GetDriverStatus {
  driverStatus(driverId: "driver-123") {
    success
    statusCode
    message
    timeStamp
    data {
      driverId
      status
      hasActiveAssignment
      activeDeliveryId
      lastSeenAt
    }
  }
}

# Get Nearby Drivers (Admin only)
query GetNearbyDrivers {
  nearbyDrivers(input: {
    latitude: 30.0444
    longitude: 31.2357
    radiusKm: 5.0
    vehicleType: "CAR"
    limit: 10
  }) {
    success
    statusCode
    message
    timeStamp
    data {
      items {
        driverId
        distanceMeters
        status
        vehicleType
        latitude
        longitude
      }
      total
    }
  }
}

# Get Assignment by ID
query GetAssignment {
  assignment(id: "assignment-123") {
    success
    statusCode
    message
    timeStamp
    data {
      id
      deliveryId
      driverId
      status
      attemptNumber
      offeredAt
      expiresAt
      acceptedAt
      rejectedAt
      completedAt
      createdAt
      updatedAt
    }
  }
}

# Get Dispatch Attempts for a Delivery (Admin/Debug)
query GetDispatchAttempts {
  dispatchAttempts(deliveryId: "delivery-456") {
    success
    statusCode
    message
    timeStamp
    data {
      items {
        id
        deliveryId
        driverId
        distanceMeters
        attemptNumber
        result          # ACCEPTED | REJECTED | TIMEOUT | NO_DRIVER
        reason
        createdAt
      }
      total
    }
  }
}

# ─── MUTATIONS ──────────────────────────────────────────────

# Go Online
mutation GoOnline {
  goOnline(idempotencyKey: "go-online-driver-123-1234567890") {
    success
    statusCode
    message
    timeStamp
    data { driverId  status  updatedAt }
  }
}

# Go Offline
mutation GoOffline {
  goOffline(idempotencyKey: "go-offline-driver-123-1234567890") {
    success
    statusCode
    message
    timeStamp
    data { driverId  status  updatedAt }
  }
}

# Accept Assignment
mutation AcceptAssignment {
  acceptAssignment(
    assignmentId: "assignment-123"
    idempotencyKey: "accept-assignment-123-driver-456"
  ) {
    success
    statusCode
    message
    timeStamp
    data {
      id
      deliveryId
      driverId
      status
      acceptedAt
      updatedAt
    }
  }
}

# Reject Assignment
mutation RejectAssignment {
  rejectAssignment(
    assignmentId: "assignment-123"
    reason: "Too far from pickup"
    idempotencyKey: "reject-assignment-123-driver-456"
  ) {
    success
    statusCode
    message
    timeStamp
    data {
      id
      deliveryId
      driverId
      status
      rejectedAt
      updatedAt
    }
  }
}

# Register Driver (Admin / onboarding)
mutation RegisterDriver {
  registerDriver(input: {
    userId: "user-789"
    vehicle: {
      type: CAR
      plateNumber: "ABC-123"
      capacityKg: 50
    }
    capabilities: ["STANDARD"]
    serviceArea: "Cairo"
  }) {
    success
    statusCode
    message
    timeStamp
    data {
      id
      userId
      status
      vehicle { type  plateNumber  capacityKg }
      capabilities
      serviceArea
      createdAt
    }
  }
}

# Update Driver Profile (Admin)
mutation UpdateDriverProfile {
  updateDriverProfile(
    driverId: "driver-123"
    input: {
      vehicle: { type: MOTORCYCLE  plateNumber: "XYZ-456"  capacityKg: 20 }
      capabilities: ["STANDARD", "FRAGILE"]
      serviceArea: "Giza"
    }
  ) {
    success
    statusCode
    message
    timeStamp
    data {
      id
      status
      vehicle { type  plateNumber  capacityKg }
      capabilities
      serviceArea
      updatedAt
    }
  }
}

# Suspend Driver (Admin)
mutation SuspendDriver {
  suspendDriver(
    driverId: "driver-123"
    reason: "Policy violation"
  ) {
    success
    statusCode
    message
    timeStamp
    data { driverId  status  updatedAt }
  }
}

# Activate Driver (Admin)
mutation ActivateDriver {
  activateDriver(driverId: "driver-123") {
    success
    statusCode
    message
    timeStamp
    data { driverId  status  updatedAt }
  }
}
```

---

## خلاصة الملفات المطلوبة

| الملف | النوع | الملاحظة |
|---|---|---|
| `protos/driver.proto` | MODIFY | إضافة 4 RPCs + messages جديدة |
| `packages/go/events/driver_events.go` | MODIFY | إضافة 10 event types + payloads |
| `packages/go/nats/events.go` | MODIFY | إضافة NATS subjects للـ driver |
| `infrastructure/docker/compose.yml` | MODIFY | إصلاح port conflict + إضافة driver |
| `infrastructure/kubernetes/base/kustomization.yaml` | MODIFY | إضافة driver resources |
| `infrastructure/skaffold/skaffold.yaml` | MODIFY | إضافة driver artifact |
| `infrastructure/kubernetes/base/services/driver-depl.yaml` | **NEW** | |
| `infrastructure/kubernetes/base/infrastructure/driver-db-depl.yaml` | **NEW** | MongoDB |
| `infrastructure/kubernetes/base/configmaps/driver-service-config.yaml` | **NEW** | |
| `infrastructure/kubernetes/base/secrets/driver-secrets.yaml` | **NEW** | |
| `services/driver-service/docker-compose.yml` | **NEW** | Local dev compose |
| `services/driver-service/Dockerfile` | **NEW** | Multi-stage Go build |
| `services/driver-service/go.mod` | **NEW** | |
| `services/driver-service/cmd/server/main.go` | **NEW** | |
| `services/driver-service/internal/config/config.go` | **NEW** | |
| `services/driver-service/internal/domain/*.go` | **NEW** | 5 files |
| `services/driver-service/internal/ports/*.go` | **NEW** | 6 files |
| `services/driver-service/internal/adapters/**/*.go` | **NEW** | ~12 files |
| `services/driver-service/internal/application/**/*.go` | **NEW** | ~10 files |
| `services/driver-service/internal/workers/*.go` | **NEW** | 4 files |
| `services/driver-service/internal/transport/graphql/*.go` | **NEW** | 3 files |
| `services/driver-service/internal/observability/*.go` | **NEW** | 2 files |
| `services/driver-service/internal/validation/location.go` | **NEW** | |
| `graphql-docs/driver.graphql` | **NEW** | |
| `services/delivery-service/.../grpc/grpc.client.driver.ts` | **NEW** | |
| `services/delivery-service/.../grpc/grpc.module.ts` | MODIFY | تسجيل driver client |
