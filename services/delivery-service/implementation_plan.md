# خطة تنفيذ Delivery Service — الخطة الشاملة والمفصلة

## ملخص تنفيذي

بعد مراجعة شاملة لكل ملفات المشروع، هذه الخطة تغطي:
1. **تشخيص الأخطاء الموجودة** في الكود الحالي
2. **هيكل الملفات الكامل** للـ delivery-service
3. **تحديثات الـ protos**
4. **Docker Compose** خاص بالـ delivery-service
5. **تحديثات infrastructure/docker/compose.yml**
6. **ملف graphql-docs/delivery.graphql**
7. **علاقة delivery-service ببقية الـ services**
8. **ما ينقص الـ shared package**
9. **تشخيص port conflicts**

---

## 🔴 الأخطاء الموجودة الآن (يجب إصلاحها أولاً)

### 1. `app.module.ts` — أخطاء جسيمة

```diff
# سطر 52-54 — استخدام "delivery" بدل "username"
- deliveryname: config.get<string>('POSTGRES_delivery') || ...
+ username: config.get<string>('POSTGRES_USER') || config.get<string>('DB_USERNAME', 'postgres'),

# سطر 57 — entities غير معرفة (Delivery, Address, Outbox) غير مستوردة
- entities: [delivery, Address, Outbox],
+ entities: [], // سيتم تعبئتها لاحقاً

# سطر 85 — req.delivery خطأ، المفروض req.user
- delivery: req.delivery,
+ user: req.user,
```

### 2. `kafka.module.ts` — token name خطأ

```diff
# يستورد REALTIME_EVENT_HANDLERS لكن يستخدم DELIVERY_EVENT_HANDLERS
- import { REALTIME_EVENT_HANDLERS } from './handlers/base-kafka-event.handler';
+ import { DELIVERY_EVENT_HANDLERS } from './handlers/base-kafka-event.handler';
```

### 3. `kafka.consumer.ts` — يستورد `RealtimeMetricsService` من مسار خاطئ

```diff
# هذا ملف metrics الـ realtime-service وليس delivery-service
- import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
+ import { DeliveryMetricsService } from '../../../common/metrics/delivery-metrics.service';
```

### 4. `grpc.client.ts` — يستورد `RealtimeConfig` و `TIMINGS` غير موجودين

```diff
- const cfg = config.get<RealtimeConfig>('realtime')!;
+ const cfg = config.get<DeliveryGrpcConfig>('grpc')!;
```

### 5. `nats.subscriber.ts` — هذا ملف الـ realtime-service وليس delivery-service

الـ nats.subscriber.ts الموجود في delivery-service يستورد:
- `SubscriptionStore` — خاص بـ realtime-service
- `ConnectionService` — خاص بـ realtime-service
- `SocketWriter` — خاص بـ realtime-service
- `RealtimeMetricsService` — خاص بـ realtime-service

**الـ delivery-service لا يحتاج NATS subscriber** — يحتاج فقط **publisher** لإرسال status updates.

### 6. `base-kafka-event.handler.ts` — منطق الـ realtime-service موجود في delivery

الـ handler يـ publish على NATS subjects — هذا دور الـ realtime-service وليس delivery-service.
الـ delivery-service يجب أن يكتب في DB فقط وينشر على Kafka عبر الـ Outbox.

### 7. `main.ts` — port تعارض

```diff
# 4003 مش محجوز لكن استخدام 4003 أكثر منطقية مع الترتيب الحالي
- const port = process.env.PORT_DELIVERY ?? 4003;
+ const port = process.env.PORT_DELIVERY ?? 4003;
```

### 8. `package.json` — typeorm version خاطئة

```diff
- "typeorm": "^1.1.0"
+ "typeorm": "^0.3.21"
```

> [!CAUTION]
> TypeORM `^1.1.0` غير موجود — الإصدار الحالي هو `0.3.x`. هذا سيمنع التشغيل الكامل.

---

## 📊 Port Mapping الحالي والمقترح

| Service              | HTTP Port    | gRPC Port | Metrics Port |
|----------------------|--------------|-----------|--------------|
| api-gateway          | 4000         | —         | —            |
| user-service         | 4001         | 50051     | —            |
| **delivery-service** | **4003**     | **50054** | **9104**     |
| media-service        | 4005         | 50052     | 9102         |
| notification-service | 4004         | 50053     | —            |
| realtime-service     | 4006         | —         | —            |
| search-service       | 4007         | —         | 9103         |
| user-db              | 5433:5432    | —         | —            |
| notification-db      | 5434:5432    | —         | —            |
| **delivery-db**      | **5435:5432**| —         | —            |

> [!NOTE]
> لا يوجد تعارض في الـ ports — port 4003 وقت التعارض لم يكن محجوزاً لكن 4003 هو الترتيب المنطقي.
> gRPC ports: user=50051, media=50052, notification=50053, **delivery=50054**, driver=50055 (future), payment=50056 (future)

---

## 🏗️ هيكل الملفات المطلوب (Final Target Structure)

مقارنةً بـ `realtime-service`, `user-service`, `notification-service`:

```
services/delivery-service/
├── .dockerignore                                    [NEW]
├── .prettierrc                                      [EXISTS]
├── Dockerfile                                       [EXISTS]
├── docker-compose.yml                               [NEW]
├── eslint.config.mjs                                [EXISTS]
├── nest-cli.json                                    [EXISTS]
├── package.json                                     [MODIFY]
├── tsconfig.json                                    [EXISTS]
├── tsconfig.build.json                              [EXISTS]
└── src/
    ├── main.ts                                      [MODIFY]
    ├── app.module.ts                                [MODIFY]
    ├── app.resolver.ts                              [NEW]
    ├── health.controller.ts                         [NEW]
    ├── schema.gql                                   [AUTO-GENERATED]
    │
    ├── common/
    │   ├── config/
    │   │   └── delivery.config.ts                   [NEW]
    │   ├── translation/
    │   │   └── translation.module.ts                [EXISTS]
    │   └── metrics/
    │       ├── delivery-metrics.service.ts           [NEW]
    │       └── delivery-metrics.module.ts            [NEW]
    │
    └── modules/
        ├── infrastructure/
        │   ├── grpc/
        │   │   ├── grpc.module.ts                   [MODIFY]
        │   │   ├── grpc.client.ts                   [REWRITE]
        │   │   └── grpc.server.ts                   [NEW]
        │   ├── kafka/
        │   │   ├── kafka.module.ts                  [MODIFY]
        │   │   ├── kafka.producer.ts                [NEW]
        │   │   ├── kafka.consumer.ts                [MODIFY]
        │   │   └── handlers/
        │   │       ├── base-delivery-handler.ts      [NEW]
        │   │       ├── payment-completed.handler.ts  [MODIFY]
        │   │       └── payment-failed.handler.ts     [MODIFY]
        │   ├── nats/
        │   │   ├── nats.module.ts                   [MODIFY - إزالة subscriber]
        │   │   ├── nats.service.ts                  [KEEP]
        │   │   └── nats.publisher.ts                [KEEP]
        │   ├── outbox/
        │   │   ├── outbox.module.ts                 [NEW]
        │   │   ├── outbox.entity.ts                 [NEW]
        │   │   ├── outbox.repository.ts             [NEW]
        │   │   └── outbox-publisher.service.ts      [NEW]
        │   └── redis/
        │       ├── redis.module.ts                  [NEW]
        │       └── idempotency.service.ts           [NEW]
        │
        ├── delivery/
        │   ├── delivery.module.ts                   [NEW]
        │   ├── entities/
        │   │   ├── delivery.entity.ts               [NEW]
        │   │   ├── address.entity.ts                [NEW]
        │   │   └── delivery-status-history.entity.ts[NEW]
        │   ├── enums/
        │   │   ├── delivery-status.enum.ts          [NEW]
        │   │   └── payment-status.enum.ts           [NEW]
        │   ├── dto/
        │   │   ├── create-delivery.input.ts         [NEW]
        │   │   ├── cancel-delivery.input.ts         [NEW]
        │   │   ├── delivery.type.ts                 [NEW]
        │   │   ├── address.type.ts                  [NEW]
        │   │   └── delivery-connection.type.ts      [NEW]
        │   ├── state-machine/
        │   │   └── delivery.state-machine.ts        [NEW]
        │   ├── commands/
        │   │   ├── create-delivery.command.ts       [NEW]
        │   │   ├── cancel-delivery.command.ts       [NEW]
        │   │   ├── accept-delivery.command.ts       [NEW]
        │   │   ├── reject-delivery.command.ts       [NEW]
        │   │   ├── start-pickup.command.ts          [NEW]
        │   │   ├── mark-picked-up.command.ts        [NEW]
        │   │   ├── start-transit.command.ts         [NEW]
        │   │   └── complete-delivery.command.ts     [NEW]
        │   ├── queries/
        │   │   ├── get-delivery.query.ts            [NEW]
        │   │   ├── get-active-delivery.query.ts     [NEW]
        │   │   ├── get-my-deliveries.query.ts       [NEW]
        │   │   └── get-delivery-history.query.ts    [NEW]
        │   ├── resolvers/
        │   │   ├── delivery.resolver.ts             [NEW]
        │   │   └── delivery.query.resolver.ts       [NEW]
        │   ├── services/
        │   │   ├── delivery-command.service.ts      [NEW]
        │   │   └── delivery-query.service.ts        [NEW]
        │   └── repositories/
        │       ├── delivery.repository.ts           [NEW]
        │       └── delivery-history.repository.ts   [NEW]
        │
        └── saga/
            ├── saga.module.ts                       [NEW]
            ├── delivery-saga.orchestrator.ts        [NEW]
            ├── delivery-saga-state.entity.ts        [NEW]
            └── steps/
                ├── reserve-driver.step.ts           [NEW]
                ├── process-payment.step.ts          [NEW]
                ├── confirm-delivery.step.ts         [NEW]
                ├── release-driver.step.ts           [NEW - compensation]
                └── refund-payment.step.ts           [NEW - compensation]
```

---

## 🔗 علاقة delivery-service ببقية الـ services

### 1. delivery-service ↔ api-gateway
- **الاتجاه:** api-gateway → delivery-service
- **البروتوكول:** HTTP (GraphQL Federation Subgraph)
- **الـ URL:** `http://delivery-srv:4003/delivery/graphql`
- **يجب إضافته في compose.yml:** `DELIVERY_SERVICE_URL: "http://delivery-srv:4003/delivery/graphql"`

### 2. delivery-service → driver-service (Go — مستقبلي)
- **البروتوكول:** gRPC
- **الـ Calls المطلوبة للـ Saga:**
  - `FindAvailableDriver(pickup_location)` → يُعيد driverId
  - `AssignDriver(driverId, deliveryId)` → يحجز السائق
  - `ReleaseDriver(driverId)` → compensation عند الفشل
- **الـ proto الحالي:** يحتوي فقط على `IsAssignedDriver` — **ناقص**

### 3. delivery-service → payment-service (Go — مستقبلي)
- **البروتوكول:** gRPC
- **الـ Calls:**
  - `CreatePayment(deliveryId, customerId, amount)` → Saga step
  - `RefundPayment(deliveryId)` → compensation
- **الـ proto:** **غير موجود — يجب إنشاء `payment.proto`**

### 4. delivery-service → notification-service
- **البروتوكول:** gRPC
- **الـ Call:** `SendNotification(userId, type, title, body, data)`
- **الـ proto:** `notification.proto` — موجود وكامل ✅
- **متى:** عند تعيين سائق، قبول/رفض، بدء الاستلام، الإلغاء، الاكتمال

### 5. delivery-service → realtime-service
- **البروتوكول:** NATS publish
- **Subjects:**
  - `realtime.delivery.status.updated` → عند أي تغيير في الـ status
  - `realtime.driver.assignment.updated` → عند تعيين/رفض سائق
- **الـ realtime-service** يستهلك ويبث عبر WebSocket

### 6. delivery-service ← realtime-service
- **البروتوكول:** gRPC (delivery كـ server)
- **الـ Call:** `IsParticipant(userId, deliveryId)` → authorization check
- **الـ proto:** `delivery.proto` — موجود ✅

### 7. delivery-service → user-service
- **البروتوكول:** gRPC
- **الـ Call:** `GetUser(userId)` → التحقق من وجود المستخدم
- **الـ proto:** `user.proto` — موجود وكامل ✅

### 8. delivery-service → Kafka (producer)
الـ events التي يُصدرها عبر الـ Transactional Outbox:

| Event | المستهلكون |
|-------|-----------|
| `delivery.created` | notification, analytics, realtime |
| `delivery.driver.assigned` | notification, realtime |
| `delivery.driver.accepted` | notification, realtime |
| `delivery.picked_up` | notification, realtime, analytics |
| `delivery.in_transit` | notification, realtime |
| `delivery.completed` | notification, analytics |
| `delivery.cancelled` | notification, analytics |

### 9. delivery-service ← Kafka (consumer)
| Event | الإجراء |
|-------|---------|
| `payment.completed` | تحديث `paymentStatus = PAID`، إكمال الـ Saga |
| `payment.failed` | compensation: release driver + cancel delivery |

---

## 📋 التحديثات المطلوبة على الـ `.proto` files

### [MODIFY] `delivery.proto` — إضافة RPC

```proto
syntax = "proto3";
package delivery;

service DeliveryService {
  rpc IsParticipant (ParticipantRequest) returns (ParticipantResponse);
  rpc GetDeliveryStatus (GetDeliveryStatusRequest) returns (GetDeliveryStatusResponse);
}

message ParticipantRequest { string userId = 1; string deliveryId = 2; }
message ParticipantResponse { bool isParticipant = 1; }

message GetDeliveryStatusRequest { string deliveryId = 1; }
message GetDeliveryStatusResponse {
  string deliveryId = 1; string status = 2;
  string driverId = 3; string customerId = 4;
}
```

### [MODIFY] `driver.proto` — إضافة RPCs للـ Saga

```proto
syntax = "proto3";
package driver;

service DriverService {
  rpc IsAssignedDriver (DriverAssignmentRequest) returns (DriverAssignmentResponse);
  rpc FindAvailableDriver (FindAvailableDriverRequest) returns (FindAvailableDriverResponse);
  rpc AssignDriver (AssignDriverRequest) returns (AssignDriverResponse);
  rpc ReleaseDriver (ReleaseDriverRequest) returns (ReleaseDriverResponse);
  rpc GetDriver (GetDriverRequest) returns (GetDriverResponse);
}

message DriverAssignmentRequest { string driverId = 1; string deliveryId = 2; }
message DriverAssignmentResponse { bool isAssigned = 1; }

message FindAvailableDriverRequest {
  double pickupLat = 1; double pickupLon = 2;
  string vehicleType = 3; double radiusKm = 4;
}
message FindAvailableDriverResponse { bool found = 1; string driverId = 2; double distanceKm = 3; }

message AssignDriverRequest { string driverId = 1; string deliveryId = 2; }
message AssignDriverResponse { bool success = 1; string message = 2; }

message ReleaseDriverRequest { string driverId = 1; string deliveryId = 2; }
message ReleaseDriverResponse { bool success = 1; }

message GetDriverRequest { string driverId = 1; }
message GetDriverResponse {
  string id = 1; string userId = 2; string vehicleType = 3;
  string status = 4; string firstName = 5; string lastName = 6; string phone = 7;
}
```

### [NEW] `payment.proto` — للـ Saga payment step

```proto
syntax = "proto3";
package payment;

service PaymentService {
  rpc CreatePayment (CreatePaymentRequest) returns (CreatePaymentResponse);
  rpc GetPaymentStatus (GetPaymentStatusRequest) returns (GetPaymentStatusResponse);
  rpc RefundPayment (RefundPaymentRequest) returns (RefundPaymentResponse);
}

message CreatePaymentRequest {
  string deliveryId = 1; string customerId = 2;
  double amount = 3; string currency = 4; string idempotencyKey = 5;
}
message CreatePaymentResponse { bool success = 1; string paymentId = 2; string status = 3; }

message GetPaymentStatusRequest { string deliveryId = 1; }
message GetPaymentStatusResponse { string paymentId = 1; string status = 2; double amount = 3; }

message RefundPaymentRequest { string deliveryId = 1; string reason = 2; }
message RefundPaymentResponse { bool success = 1; string refundId = 2; }
```

---

## ⚠️ ما ينقص الـ shared package `@delivery/common`

### الموجود ✅
- `DeliveryEventType` enum, `DeliveryKafkaTopics`, `PaymentKafkaTopics`
- `RealtimeNatsSubjects`, `NotificationNatsSubjects`
- `KafkaModule`, `KafkaService`, `NatsModule`, `NatsService`
- Auth guards, decorators, filters, interceptors
- `DeliveryCreatedPayload`, `DeliveryUpdatedPayload`, `DeliveryDriverAssignedPayload`

### الناقص — يجب إضافته 🔴

#### [NEW] `src/events/payment.events.ts`

```typescript
export interface PaymentCompletedPayload {
  paymentId: string;
  deliveryId: string;
  customerId: string;
  amount: number;
  currency: string;
  completedAt: string;
}

export interface PaymentFailedPayload {
  paymentId: string;
  deliveryId: string;
  reason: string;
  failedAt: string;
}

export interface PaymentRefundedPayload {
  paymentId: string;
  deliveryId: string;
  refundId: string;
  amount: number;
  refundedAt: string;
}
```

#### [ADD to kafka.service.ts] `KafkaEventEnvelope` interface

```typescript
export interface KafkaEventEnvelope<T = unknown> {
  eventId: string;
  eventType: string;
  aggregateId: string;
  aggregateType: string;
  version: number;
  producer: string;
  traceId?: string;
  timestamp: number;
  payload: T;
}
```

#### [MODIFY] `src/index.ts` — إضافة export

```typescript
export * from './events/payment.events';
```

---

## 🐳 [NEW] `services/delivery-service/docker-compose.yml`

```yaml
version: '3.8'

services:
  delivery-db-srv:
    image: postgres:15
    pull_policy: missing
    container_name: delivery-db-srv
    env_file:
      - ../../config/env/.env.${APP_ENV:-development}
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${DELIVERY_DB_NAME:-delivery_delivery_db}
      POSTGRES_HOST_AUTH_METHOD: trust
    ports:
      - "5435:5432"
    volumes:
      - delivery_db_data:/var/lib/postgresql/data

  redis-srv:
    image: redis:alpine
    pull_policy: missing
    container_name: redis-srv
    ports:
      - "6379:6379"

  nats-srv:
    image: nats:2.10-alpine
    pull_policy: missing
    container_name: nats-srv
    command: ["-p", "4222", "-m", "8222"]
    ports:
      - "4222:4222"
      - "8222:8222"

  kafka-srv:
    image: apache/kafka:3.8.0
    container_name: kafka-srv
    ports:
      - "9092:9092"
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: CONTROLLER://:9093,PLAINTEXT://:9092
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka-srv:9092
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka-srv:9093
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_MIN_ISR: 1
      KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS: 0
      CLUSTER_ID: 4L6g3nShT-eMCtK--X86sw
      KAFKA_HEAP_OPTS: "-Xms256m -Xmx512m"

  delivery-service:
    build:
      context: ../..
      dockerfile: services/delivery-service/Dockerfile
    container_name: delivery-service
    restart: always
    env_file:
      - ../../config/env/.env.${APP_ENV:-development}
    ports:
      - "4003:4003"
      - "50054:50054"
      - "9104:9104"
    networks:
      default:
        aliases:
          - delivery-srv
    environment:
      PORT_DELIVERY: ${PORT_DELIVERY:-4003}
      PORT_DELIVERY_GRPC: ${PORT_DELIVERY_GRPC:-50054}
      PORT_DELIVERY_METRICS: ${PORT_DELIVERY_METRICS:-9104}
      DB_HOST: "delivery-db-srv"
      DB_PORT: ${DB_PORT:-5432}
      DB_NAME: ${DELIVERY_DB_NAME:-delivery_delivery_db}
      DB_USERNAME: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      NODE_ENV: ${NODE_ENV:-development}
      JWT_SECRET: ${JWT_SECRET}
      JWT_EXPIRE: ${JWT_EXPIRE:-36000s}
      REDIS_HOST: "redis-srv"
      REDIS_PORT: ${REDIS_PORT:-6379}
      REDIS_DB: ${REDIS_DB:-0}
      NATS_URL: ${NATS_URL:-nats://nats-srv:4222}
      KAFKA_BROKERS: ${KAFKA_BROKERS:-kafka-srv:9092}
      KAFKA_CLIENT_ID: "delivery-service"
      KAFKA_GROUP_ID: "delivery-service-group"
      USER_SERVICE_GRPC_URL: ${USER_SERVICE_GRPC_URL:-user-srv:50051}
      NOTIFICATION_SERVICE_GRPC_URL: ${NOTIFICATION_SERVICE_GRPC_URL:-notification-srv:50053}
      DRIVER_SERVICE_GRPC_URL: ${DRIVER_SERVICE_GRPC_URL:-driver-srv:50055}
      PAYMENT_SERVICE_GRPC_URL: ${PAYMENT_SERVICE_GRPC_URL:-payment-srv:50056}
      OUTBOX_POLLING_INTERVAL_MS: ${OUTBOX_POLLING_INTERVAL_MS:-1000}
      OUTBOX_BATCH_SIZE: ${OUTBOX_BATCH_SIZE:-100}
      SAGA_DRIVER_TIMEOUT_MS: ${SAGA_DRIVER_TIMEOUT_MS:-30000}
      SAGA_PAYMENT_TIMEOUT_MS: ${SAGA_PAYMENT_TIMEOUT_MS:-60000}
      IDEMPOTENCY_TTL_SECONDS: ${IDEMPOTENCY_TTL_SECONDS:-86400}
    depends_on:
      - delivery-db-srv
      - redis-srv
      - nats-srv
      - kafka-srv
    volumes:
      - ./src:/usr/src/app/services/delivery-service/src
      - ../../packages/ts:/usr/src/app/packages/ts
      - ../../protos:/usr/src/app/protos

volumes:
  delivery_db_data:
```

---

## 🔧 تحديثات `infrastructure/docker/compose.yml`

### إضافة delivery-db-srv

```yaml
delivery-db-srv:
  image: postgres:15
  pull_policy: missing
  container_name: delivery-db-srv
  env_file:
    - ../../config/env/.env.${APP_ENV:-development}
  environment:
    POSTGRES_USER: ${POSTGRES_USER}
    POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    POSTGRES_DB: ${DELIVERY_DB_NAME:-delivery_delivery_db}
    POSTGRES_HOST_AUTH_METHOD: trust
  ports:
    - "5435:5432"
  volumes:
    - delivery_db_data:/var/lib/postgresql/data
```

### إضافة delivery-service

```yaml
delivery-service:
  build:
    context: ../..
    dockerfile: services/delivery-service/Dockerfile
  container_name: delivery-service
  restart: always
  env_file:
    - ../../config/env/.env.${APP_ENV:-development}
  ports:
    - "4003:4003"
    - "50054:50054"
    - "9104:9104"
  networks:
    default:
      aliases:
        - delivery-srv
  environment:
    PORT_DELIVERY: ${PORT_DELIVERY:-4003}
    PORT_DELIVERY_GRPC: ${PORT_DELIVERY_GRPC:-50054}
    DB_HOST: "delivery-db-srv"
    DB_PORT: ${DB_PORT:-5432}
    DB_NAME: ${DELIVERY_DB_NAME:-delivery_delivery_db}
    DB_USERNAME: ${POSTGRES_USER}
    POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    NODE_ENV: ${NODE_ENV:-development}
    JWT_SECRET: ${JWT_SECRET}
    JWT_EXPIRE: ${JWT_EXPIRE:-36000s}
    REDIS_HOST: "redis-srv"
    REDIS_PORT: ${REDIS_PORT:-6379}
    NATS_URL: ${NATS_URL:-nats://nats-srv:4222}
    KAFKA_BROKERS: ${KAFKA_BROKERS:-kafka-srv:9092}
    KAFKA_CLIENT_ID: "delivery-service"
    KAFKA_GROUP_ID: "delivery-service-group"
    USER_SERVICE_GRPC_URL: "user-srv:50051"
    NOTIFICATION_SERVICE_GRPC_URL: "notification-srv:50053"
    DRIVER_SERVICE_GRPC_URL: "driver-srv:50055"
    PAYMENT_SERVICE_GRPC_URL: "payment-srv:50056"
    OUTBOX_POLLING_INTERVAL_MS: "1000"
    OUTBOX_BATCH_SIZE: "100"
  depends_on:
    - delivery-db-srv
    - redis-srv
    - nats-srv
    - kafka-srv
  volumes:
    - ../../services/delivery-service/src:/usr/src/app/services/delivery-service/src
    - ../../packages/ts:/usr/src/app/packages/ts
    - ../../protos:/usr/src/app/protos
```

### تحديث api-gateway environment

```yaml
DELIVERY_SERVICE_URL: "http://delivery-srv:4003/delivery/graphql"
```

### إضافة volumes

```yaml
volumes:
  delivery_db_data:
```

---

## 📄 [NEW] `graphql-docs/delivery.graphql`

```graphql
# ===========================================
# DELIVERY SERVICE — GraphQL Documentation
# ===========================================
# Base URL: http://localhost:4000/graphql (via API Gateway)
# Auth: Bearer <JWT_TOKEN> required for all
# Roles: CUSTOMER | DRIVER | ADMIN
# ===========================================

# ── QUERIES ────────────────────────────────

# Get a single delivery by ID
# Roles: CUSTOMER (own), DRIVER (assigned), ADMIN (any)
query GetDelivery {
  delivery(id: "delivery-uuid-here") {
    statusCode
    message
    success
    data {
      id
      status
      paymentStatus
      price
      currency
      customerId
      driverId
      pickupAddress {
        city
        country
        street
        location { lat lon }
      }
      dropoffAddress {
        city
        country
        street
        location { lat lon }
      }
      statusHistory {
        fromStatus
        toStatus
        changedBy
        createdAt
      }
      createdAt
      updatedAt
    }
  }
}

# Get paginated deliveries — Customer: own orders | Driver: assigned
# Roles: CUSTOMER, DRIVER
query GetMyDeliveries {
  myDeliveries(page: 1, limit: 10) {
    statusCode
    message
    success
    data {
      items {
        id
        status
        paymentStatus
        price
        pickupAddress { city country }
        dropoffAddress { city country }
        createdAt
        updatedAt
      }
      total
      page
      limit
      totalPages
    }
  }
}

# Get the currently active delivery (non-terminal status)
# Roles: CUSTOMER, DRIVER
query GetActiveDelivery {
  activeDelivery {
    statusCode
    message
    success
    data {
      id
      status
      paymentStatus
      price
      driverId
      pickupAddress { city country street location { lat lon } }
      dropoffAddress { city country street location { lat lon } }
      createdAt
      updatedAt
    }
  }
}

# Get delivery history (DELIVERED, CANCELLED, FAILED)
# Roles: CUSTOMER, ADMIN
query GetDeliveryHistory {
  deliveryHistory(page: 1, limit: 20) {
    statusCode
    message
    success
    data {
      items {
        id
        status
        paymentStatus
        price
        pickupAddress { city country }
        dropoffAddress { city country }
        createdAt
        updatedAt
      }
      total
      page
      limit
      totalPages
    }
  }
}

# ── MUTATIONS ──────────────────────────────

# Create a new delivery — initiates the Delivery Saga
# Roles: CUSTOMER
# State machine: PENDING → SEARCHING_DRIVER → DRIVER_ASSIGNED → ...
mutation CreateDelivery {
  createDelivery(
    input: {
      pickupAddress: {
        city: "Cairo"
        country: "EG"
        street: "123 Tahrir Square"
        location: { lat: 30.0444, lon: 31.2357 }
      }
      dropoffAddress: {
        city: "Giza"
        country: "EG"
        street: "456 Pyramids Road"
        location: { lat: 29.9773, lon: 31.1325 }
      }
      vehicleType: "MOTORCYCLE"
      notes: "Handle with care"
    }
  ) {
    statusCode
    message
    success
    data {
      id
      status
      price
      currency
      estimatedDurationMinutes
      createdAt
    }
  }
}

# Cancel delivery
# Roles: CUSTOMER (own), ADMIN
# Allowed from: PENDING | SEARCHING_DRIVER | DRIVER_ASSIGNED | DRIVER_ACCEPTED
mutation CancelDelivery {
  cancelDelivery(id: "delivery-uuid-here") {
    statusCode
    message
    success
    data { id status updatedAt }
  }
}

# Driver accepts assigned delivery
# Roles: DRIVER
# Transition: DRIVER_ASSIGNED → DRIVER_ACCEPTED
mutation AcceptDelivery {
  acceptDelivery(id: "delivery-uuid-here") {
    statusCode
    message
    success
    data { id status updatedAt }
  }
}

# Driver rejects assigned delivery (triggers re-assignment or CANCELLED)
# Roles: DRIVER
# Transition: DRIVER_ASSIGNED → SEARCHING_DRIVER (retry) | CANCELLED
mutation RejectDelivery {
  rejectDelivery(id: "delivery-uuid-here") {
    statusCode
    message
    success
    data { id status updatedAt }
  }
}

# Driver starts moving to pickup location
# Roles: DRIVER
# Transition: DRIVER_ACCEPTED → PICKUP_STARTED
mutation StartPickup {
  startPickup(id: "delivery-uuid-here") {
    statusCode
    message
    success
    data { id status updatedAt }
  }
}

# Driver confirms item has been picked up
# Roles: DRIVER
# Transition: PICKUP_STARTED → PICKED_UP
mutation MarkPickedUp {
  markPickedUp(id: "delivery-uuid-here") {
    statusCode
    message
    success
    data { id status updatedAt }
  }
}

# Driver starts transit to dropoff location
# Roles: DRIVER
# Transition: PICKED_UP → IN_TRANSIT
mutation StartTransit {
  startTransit(id: "delivery-uuid-here") {
    statusCode
    message
    success
    data { id status updatedAt }
  }
}

# Driver marks delivery as completed (terminal state)
# Roles: DRIVER
# Transition: IN_TRANSIT → DELIVERED
mutation CompleteDelivery {
  completeDelivery(id: "delivery-uuid-here") {
    statusCode
    message
    success
    data { id status paymentStatus updatedAt }
  }
}
```

---

## 🗺️ خطة التنفيذ المرحلية

### المرحلة 1 — إصلاح الأخطاء الحالية 🔴

- [ ] إصلاح `package.json` — typeorm version إلى `^0.3.21`
- [ ] إصلاح `app.module.ts` — username, entities, context
- [ ] إصلاح `kafka.module.ts` — token name
- [ ] إصلاح `kafka.consumer.ts` — metrics import
- [ ] إصلاح `grpc.client.ts` — config type و TIMINGS
- [ ] إصلاح `nats.module.ts` — إزالة subscriber الخاطئ
- [ ] إصلاح `main.ts` — port 4003

### المرحلة 2 — الملفات والإعداد 🟠

- [ ] إنشاء `docker-compose.yml` للـ delivery-service
- [ ] تحديث `infrastructure/docker/compose.yml`
- [ ] إنشاء `delivery.config.ts`
- [ ] إنشاء `delivery-metrics.service.ts`
- [ ] إنشاء `health.controller.ts`

### المرحلة 3 — Database Layer

- [ ] إنشاء `delivery.entity.ts`
- [ ] إنشاء `address.entity.ts` (embedded)
- [ ] إنشاء `delivery-status-history.entity.ts`
- [ ] إنشاء `delivery-status.enum.ts`
- [ ] إنشاء `payment-status.enum.ts`
- [ ] إنشاء `outbox.entity.ts`
- [ ] إنشاء `delivery-saga-state.entity.ts`

### المرحلة 4 — Business Logic

- [ ] إنشاء `delivery.state-machine.ts`
- [ ] إنشاء `delivery-command.service.ts`
- [ ] إنشاء `delivery-query.service.ts`
- [ ] إنشاء `delivery.repository.ts`
- [ ] إنشاء `idempotency.service.ts`

### المرحلة 5 — GraphQL Layer

- [ ] إنشاء DTOs و Types
- [ ] إنشاء `delivery.resolver.ts` (mutations)
- [ ] إنشاء `delivery.query.resolver.ts` (queries)
- [ ] إنشاء `app.resolver.ts`

### المرحلة 6 — Outbox + Kafka Producer

- [ ] إنشاء `outbox.repository.ts`
- [ ] إنشاء `outbox-publisher.service.ts`
- [ ] إنشاء `kafka.producer.ts`

### المرحلة 7 — Saga + gRPC

- [ ] إنشاء `delivery-saga.orchestrator.ts`
- [ ] إنشاء saga steps
- [ ] إنشاء `grpc.server.ts` (expose IsParticipant)
- [ ] تفعيل gRPC transport في `main.ts`

### المرحلة 8 — Protos + Shared Package + Docs

- [ ] تحديث `delivery.proto`
- [ ] تحديث `driver.proto`
- [ ] إنشاء `payment.proto`
- [ ] إضافة `payment.events.ts` في shared package
- [ ] إضافة `KafkaEventEnvelope` في shared package
- [ ] إنشاء `graphql-docs/delivery.graphql`

---

## ✅ جدول الأولويات الفورية

| # | الإجراء | الأولوية | الملف |
|---|---------|----------|-------|
| 1 | إصلاح typeorm version | 🔴 Critical | `package.json` |
| 2 | إصلاح TypeORM config (username, entities) | 🔴 Critical | `app.module.ts` |
| 3 | إصلاح kafka token name | 🔴 Critical | `kafka.module.ts` |
| 4 | إصلاح metrics import | 🔴 Critical | `kafka.consumer.ts` |
| 5 | إصلاح grpc config + TIMINGS | 🔴 Critical | `grpc.client.ts` |
| 6 | إزالة nats subscriber الخاطئ | 🔴 Critical | `nats.module.ts` + `nats.subscriber.ts` |
| 7 | إصلاح port إلى 4003 | 🟠 High | `main.ts` |
| 8 | إنشاء `docker-compose.yml` | 🟠 High | `delivery-service/` |
| 9 | تحديث `compose.yml` المشترك | 🟠 High | `infrastructure/docker/` |
| 10 | تحديث `driver.proto` | 🟡 Medium | `protos/` |
| 11 | إنشاء `payment.proto` | 🟡 Medium | `protos/` |
| 12 | إضافة `payment.events.ts` | 🟡 Medium | `packages/ts/` |
| 13 | إنشاء `delivery.graphql` docs | 🟡 Medium | `graphql-docs/` |
| 14 | بناء هيكل الملفات الكامل | 🟢 Standard | `delivery-service/src/` |
