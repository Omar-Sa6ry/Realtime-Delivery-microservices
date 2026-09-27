# 🐇 خطة تطبيق RabbitMQ — Realtime Delivery Microservices
## Implementation Plan (مراجعة مطلوبة قبل التنفيذ)

---

## 1. الوضع الحالي — تشخيص دقيق

### 1.1 ما هو موجود الآن في RabbitMQ

| الملف | الحالة | الملاحظة |
|---|---|---|
| `infrastructure/rabbitmq/rabbitmq.conf` | موجود ومهيأ | quorum queues، prometheus، load_definitions |
| `infrastructure/rabbitmq/definitions.json` | موجود | 7 exchanges، 13 queue، 15 binding |
| `infrastructure/docker/compose.yml` | RabbitMQ container موجود | لكن لا خدمة تتصل به |
| `infrastructure/kubernetes/base/infrastructure/rabbitmq-depl.yaml` | موجود | Deployment + Service + PVC |
| `infrastructure/kubernetes/base/configmaps/rabbitmq-config.yaml` | موجود | config + enabled_plugins |
| `infrastructure/kubernetes/base/configmaps/rabbitmq-definitions.yaml` | موجود | topology كاملة |
| `infrastructure/kubernetes/base/secrets/rabbitmq-secret.yaml` | موجود | user/pass/vhost |
| `packages/ts/src/rabbitmq/` | **EMPTY** | المجلد موجود لكن **فارغ تماماً** |

### 1.2 مشاكل مكتشفة في Infrastructure

| المشكلة | الملف | الأثر |
|---|---|---|
| `enabled_plugins` غير موجود في infrastructure/rabbitmq | `compose.yml` L108 يعمل mount لملف غير موجود | Docker Compose سيفشل عند البناء |
| `quorum_cluster_size = 1` في prod | `rabbitmq.conf` L19 | OK للـ dev لكن في production يحتاج ≥ 3 |
| K8s deployment يستخدم `Recreate` strategy | `rabbitmq-depl.yaml` L11 | لا يسمح بـ rolling upgrade |
| لا يوجد `analytics-service` في definitions | `definitions.json` | Analytics لا يستلم أحداث |
| `delivery.dlx.fanout` يجمع كل DLQ messages | `definitions.json` | يصعب تصنيف وreplay الرسائل الفاشلة |
| `policies` pattern يغطي فقط 7 prefixes | `definitions.json` L11 | لا يغطي analytics و driver و user |

### 1.3 الخدمات وعلاقتها بـ RabbitMQ الحالي

| الخدمة | التقنية المستخدمة | يجب أن يستخدم RabbitMQ |
|---|---|---|
| **delivery-service** (NestJS) | Kafka Consumer + NATS | Publisher لأحداث Orders |
| **notification-service** (NestJS) | Kafka Consumer | Consumer لأحداث الإشعارات |
| **payment-service** (Go) | Kafka Publisher | Publisher + Consumer |
| **realtime-service** (NestJS) | Kafka Consumer + NATS | Consumer لـ broadcast |
| **search-service** (Go) | Kafka Consumer | Consumer لـ indexing |
| **driver-service** (Go) | Kafka Publisher | Publisher لـ dispatch |
| **user-service** (NestJS) | لا يستخدم شيئاً | Publisher لأحداث المستخدمين |
| **media-service** (Go) | Kafka Publisher | Publisher للـ media events |
| **analytics-service** | غير محدد | Consumer للتحليلات |
| **api-gateway** (NestJS) | لا messaging | لا يحتاج RabbitMQ |

---

## 2. قرار Architecture — لماذا RabbitMQ بجانب Kafka

```
Kafka    → Streaming / Event Sourcing / Log compaction / High-throughput analytics
RabbitMQ → Task queues / Guaranteed delivery / Smart routing / Request-Reply / Notifications
NATS     → Real-time pub/sub بين nodes (داخل realtime-service)
```

**المبدأ:** RabbitMQ يُستخدم للـ Business Events ذات الأهمية العالية التي تتطلب:
- Guaranteed delivery مع acknowledgements
- Dead Letter Queue مُنظَّمة per-domain
- Routing ذكي بـ topic/fanout/direct exchanges
- Priority queues للإشعارات العاجلة
- Request-Reply pattern (لـ dispatch)

---

## 3. التصميم الكامل للـ Topology

### 3.1 Exchanges (مُدمَجة مع الموجودة)

| Exchange | Type | الاستخدام |
|---|---|---|
| `delivery.orders.topic` | topic | أحداث الطلبات من delivery-service (موجود) |
| `delivery.notifications.fanout` | fanout | fan-out للإشعارات (موجود) |
| `delivery.payments.direct` | direct | أحداث الدفع من payment-service (موجود) |
| `delivery.dispatch.direct` | direct | طلبات التعيين بين delivery/driver (موجود) |
| `delivery.media.topic` | topic | أحداث الوسائط من media-service (موجود) |
| `delivery.realtime.fanout` | fanout | broadcast للـ WebSocket clients (موجود) |
| `delivery.dlx.fanout` | fanout | Dead Letter Exchange الرئيسي (موجود) |
| `delivery.users.topic` | topic | أحداث المستخدمين من user-service (**جديد**) |
| `delivery.analytics.fanout` | fanout | نسخ أحداث للتحليلات (**جديد**) |
| `delivery.drivers.topic` | topic | أحداث السائقين من driver-service (**جديد**) |
| `delivery.dlx.direct` | direct | Per-domain DLQ routing (**جديد**) |

### 3.2 Queues الجديدة والمُعدَّلة

| Queue | Type | Consumer | جديد؟ |
|---|---|---|---|
| `orders.created.queue` | quorum | delivery + notification | موجود |
| `orders.status.queue` | quorum | realtime + search | موجود |
| `notifications.email.queue` | quorum | notification-service | موجود |
| `notifications.sms.queue` | quorum | notification-service | موجود |
| `notifications.push.queue` | quorum | notification-service | موجود |
| `payments.authorized.queue` | quorum | delivery-service | موجود |
| `payments.failed.queue` | quorum | delivery + notification | موجود |
| `dispatch.requests.queue` | quorum | driver-service | موجود |
| `dispatch.responses.queue` | quorum | delivery-service | موجود |
| `media.uploaded.queue` | quorum | notification-service | موجود |
| `media.processed.queue` | quorum | search-service | موجود |
| `realtime.broadcast.queue` | quorum | realtime-service | موجود |
| `search.index.queue` | quorum | search-service | موجود |
| `delivery.dlq.queue` | quorum | DLQ handler | موجود |
| `users.created.queue` | quorum | notification + search | **جديد** |
| `users.updated.queue` | quorum | search-service | **جديد** |
| `analytics.events.queue` | quorum | analytics-service | **جديد** |
| `drivers.events.queue` | quorum | search + notification | **جديد** |
| `payments.refunded.queue` | quorum | notification-service | **جديد** |
| `orders.dlq.queue` | quorum | manual review | **جديد** |
| `payments.dlq.queue` | quorum | manual review | **جديد** |
| `notifications.dlq.queue` | quorum | manual review | **جديد** |
| `drivers.dlq.queue` | quorum | manual review | **جديد** |

### 3.3 DLQ Strategy المُحسَّنة (Per-Domain DLQ)

بدلاً من DLQ واحد لكل شيء، نستخدم per-domain routing:

```
delivery.dlx.direct (جديد)
  routing_key: "orders.dlq"        → orders.dlq.queue
  routing_key: "payments.dlq"      → payments.dlq.queue
  routing_key: "notifications.dlq" → notifications.dlq.queue
  routing_key: "drivers.dlq"       → drivers.dlq.queue
```

كل queue تضيف `x-dead-letter-exchange` + `x-dead-letter-routing-key`.

---

## 4. الخطة التفصيلية — Phase by Phase

---

### PHASE 1 — إصلاح Infrastructure الموجودة

**المشاكل الواجب إصلاحها قبل أي كود:**

#### 1.1 إنشاء ملف enabled_plugins المفقود

**الملف:** `infrastructure/rabbitmq/enabled_plugins` (جديد)

```
[rabbitmq_management, rabbitmq_prometheus, rabbitmq_shovel, rabbitmq_federation].
```

> سبب المشكلة: compose.yml:108 يعمل volume mount لهذا الملف لكنه غير موجود

#### 1.2 تحديث definitions.json

التغييرات المطلوبة:
1. إضافة exchanges جديدة: `delivery.users.topic`, `delivery.analytics.fanout`, `delivery.drivers.topic`, `delivery.dlx.direct`
2. إضافة queues جديدة: per-domain DLQs + analytics + users + drivers
3. تحديث policies pattern ليشمل: `^(orders|notifications|payments|dispatch|media|realtime|search|users|analytics|drivers)[\\.-]`
4. إضافة `x-dead-letter-routing-key` لكل queue
5. إضافة priority support: `x-max-priority: 10`

#### 1.3 تحديث rabbitmq.conf

التغييرات الطفيفة:
- إضافة `consumer_timeout = 1800000`
- إضافة `management.cors.allow_origins = *` (للـ dev)

#### 1.4 تحويل K8s من Deployment إلى StatefulSet

```yaml
# infrastructure/kubernetes/base/infrastructure/rabbitmq-depl.yaml
apiVersion: apps/v1
kind: StatefulSet   # كان Deployment
metadata:
  name: rabbitmq
spec:
  serviceName: rabbitmq-headless
  replicas: 1       # 3 في production
```

---

### PHASE 2 — بناء الـ Shared RabbitMQ Package (TypeScript)

**الهدف:** بناء package كامل في `packages/ts/src/rabbitmq/`

#### هيكل الملفات

```
packages/ts/src/rabbitmq/
├── rabbitmq.constants.ts    # Exchanges, Queues, Routing Keys
├── rabbitmq.module.ts       # Dynamic NestJS Module
├── rabbitmq.service.ts      # Publisher Service
├── rabbitmq.consumer.ts     # Base Consumer Class (Abstract)
├── rabbitmq.health.ts       # Health indicator لـ @nestjs/terminus
└── index.ts                 # Re-exports
```

#### rabbitmq.constants.ts — الثوابت الكاملة

```typescript
export enum RabbitMQExchanges {
  ORDERS        = 'delivery.orders.topic',
  NOTIFICATIONS = 'delivery.notifications.fanout',
  PAYMENTS      = 'delivery.payments.direct',
  DISPATCH      = 'delivery.dispatch.direct',
  MEDIA         = 'delivery.media.topic',
  REALTIME      = 'delivery.realtime.fanout',
  USERS         = 'delivery.users.topic',
  ANALYTICS     = 'delivery.analytics.fanout',
  DRIVERS       = 'delivery.drivers.topic',
  DLX           = 'delivery.dlx.fanout',
  DLX_DIRECT    = 'delivery.dlx.direct',
}

export enum OrdersRoutingKeys {
  CREATED      = 'order.created',
  STATUS_ALL   = 'order.#',
  CANCELLED    = 'order.cancelled',
  COMPLETED    = 'order.completed',
  PICKED_UP    = 'order.picked_up',
  IN_TRANSIT   = 'order.in_transit',
  DRIVER_ASSGN = 'order.driver.assigned',
}

export enum PaymentsRoutingKeys {
  AUTHORIZED = 'payment.authorized',
  FAILED     = 'payment.failed',
  REFUNDED   = 'payment.refunded',
  COMPLETED  = 'payment.completed',
}

export enum DispatchRoutingKeys {
  REQUEST  = 'dispatch.request',
  RESPONSE = 'dispatch.response',
}

export enum RabbitMQQueues {
  ORDERS_CREATED      = 'orders.created.queue',
  NOTIFICATIONS_EMAIL = 'notifications.email.queue',
  NOTIFICATIONS_SMS   = 'notifications.sms.queue',
  NOTIFICATIONS_PUSH  = 'notifications.push.queue',
  PAYMENTS_AUTHORIZED = 'payments.authorized.queue',
  PAYMENTS_FAILED     = 'payments.failed.queue',
  PAYMENTS_REFUNDED   = 'payments.refunded.queue',
  DISPATCH_REQUESTS   = 'dispatch.requests.queue',
  DISPATCH_RESPONSES  = 'dispatch.responses.queue',
  REALTIME_BROADCAST  = 'realtime.broadcast.queue',
  SEARCH_INDEX        = 'search.index.queue',
  USERS_CREATED       = 'users.created.queue',
  USERS_UPDATED       = 'users.updated.queue',
  ANALYTICS_EVENTS    = 'analytics.events.queue',
  DRIVERS_EVENTS      = 'drivers.events.queue',
  DLQ_ORDERS          = 'orders.dlq.queue',
  DLQ_PAYMENTS        = 'payments.dlq.queue',
  DLQ_NOTIFICATIONS   = 'notifications.dlq.queue',
  DLQ_DRIVERS         = 'drivers.dlq.queue',
}
```

#### rabbitmq.service.ts — Publisher مع Circuit Breaker

```typescript
@Injectable()
export class RabbitMQService implements OnModuleInit, OnModuleDestroy {
  // Connection واحدة + channels متعددة (AMQP multiplexing)
  // publisher confirms لضمان الوصول
  // reconnect تلقائي مع exponential backoff
  // buildEnvelope() — نفس EventEnvelope الموجود في الـ Kafka
  // metrics: published_total, failed_total, retry_total
}
```

**Pattern:** يُستخدم Outbox Pattern — كل publisher يحفظ في DB أولاً، worker منفصل يُرسل.

#### rabbitmq.consumer.ts — Base Consumer Abstract

```typescript
export abstract class BaseRabbitMQConsumer implements OnModuleInit {
  protected abstract queue: string;
  protected abstract exchange: string;
  protected abstract routingKeys: string[];
  protected prefetch = 10;

  // channel.prefetch(this.prefetch) لـ back-pressure
  // Idempotency check قبل المعالجة (eventId + consumerName)
  // ack() عند النجاح
  // nack(false, false) + dead letter عند الفشل النهائي
  // Retry مع x-retry-count header
  // Tracing propagation (x-trace-id)
}
```

#### تحديث packages/ts/src/index.ts

إضافة exports:
```typescript
export * from './rabbitmq/rabbitmq.constants';
export * from './rabbitmq/rabbitmq.module';
export * from './rabbitmq/rabbitmq.service';
export * from './rabbitmq/rabbitmq.consumer';
export * from './rabbitmq/rabbitmq.health';
```

#### تحديث packages/ts/package.json

```json
"dependencies": {
  "amqplib": "^0.10.4",
  "@types/amqplib": "^0.10.5"
}
```

---

### PHASE 3 — تطبيق على خدمات NestJS

#### 3.1 delivery-service — Publisher

**الدور:** ينشر أحداث Orders إلى `delivery.orders.topic`

**الملفات الجديدة:**
```
services/delivery-service/src/modules/infrastructure/rabbitmq/
├── rabbitmq.module.ts       # DeliveryRabbitMQModule
└── rabbitmq.publisher.ts    # DeliveryRabbitMQPublisher
```

**rabbitmq.publisher.ts:**
```typescript
@Injectable()
export class DeliveryRabbitMQPublisher {
  publishOrderCreated(delivery): Promise<void>
  publishOrderStatusChanged(delivery): Promise<void>
  publishDriverAssigned(deliveryId, driverId): Promise<void>
  publishOrderCompleted(delivery): Promise<void>
  publishOrderCancelled(delivery): Promise<void>
}
```

**Integration:** في `delivery-command.service.ts` بعد كل state transition.

**تحديث app.module.ts:** إضافة `DeliveryRabbitMQModule`

**تحديث compose.yml:**
```yaml
delivery-service:
  environment:
    RABBITMQ_URL: "amqp://${RABBITMQ_USER:-delivery}:${RABBITMQ_PASSWORD}@rabbitmq-srv:5672/"
  depends_on:
    rabbitmq-srv:
      condition: service_healthy
```

#### 3.2 notification-service — Consumer (3 consumers)

**الدور:** يستهلك من email + sms + push queues

**الملفات الجديدة:**
```
services/notification-service/src/modules/rabbitmq/
├── rabbitmq.module.ts
├── consumers/
│   ├── email.consumer.ts      (prefetch: 10)
│   ├── sms.consumer.ts        (prefetch: 20)
│   └── push.consumer.ts       (prefetch: 50)
└── rabbitmq-notification.mapper.ts
```

**Idempotency:** استخدام `NotificationInbox` الموجودة مع إضافة `source: 'rabbitmq'`

**ملاحظة مهمة:** KafkaConsumerModule يبقى — لا يُحذف (Dual publishing strategy)

#### 3.3 realtime-service — Consumer

**الدور:** يستهلك من `realtime.broadcast.queue` ويُذيع عبر WebSocket

**الملفات الجديدة:**
```
services/realtime-service/src/modules/infrastructure/rabbitmq/
├── rabbitmq.module.ts
└── broadcast.consumer.ts
```

**Integration:** ينادي على `EventsService` الموجود لتوزيع الرسالة على WebSocket clients.

---

### PHASE 4 — تطبيق على خدمات Go

#### 4.1 Go RabbitMQ Shared Package

**الملف الجديد:** `packages/go/rabbitmq/rabbitmq.go`

```go
package rabbitmq

// Publisher: AMQP connection + channel + publisher confirms
// Consumer: manual ack + prefetch + DLQ routing
// Reconnect: automatic با exponential backoff
// Metrics: Prometheus counters
// Tracing: x-trace-id header propagation

type Publisher struct {}
type Consumer struct {}
type Connection struct {}
```

**تحديث packages/go/go.mod:**
```
require (
  github.com/rabbitmq/amqp091-go v1.10.0
)
```

#### 4.2 payment-service (Go) — Publisher + Consumer

**Publisher:** ينشر payment.authorized, payment.failed, payment.refunded
**Consumer:** يستقبل من dispatch.responses.queue

**الملفات الجديدة:**
```
services/payment-service/internal/adapters/rabbitmq/
├── publisher.go
└── consumer.go
```

**تحديث:** `services/payment-service/go.mod`

#### 4.3 driver-service (Go) — Consumer + Publisher

**Consumer:** يستقبل `dispatch.requests.queue` (يُعالج طلبات التعيين)
**Publisher:** ينشر ردود التعيين إلى `delivery.dispatch.direct`

**الملفات الجديدة:**
```
services/driver-service/internal/adapters/rabbitmq/
├── publisher.go
└── dispatch_consumer.go
```

**Integration:** يُكمل (لا يستبدل) `delivery_consumer.go` الـ Kafka الموجود

**تحديث:** `services/driver-service/go.mod`

#### 4.4 search-service (Go) — Consumer

**الدور:** يستهلك من `search.index.queue` لتحديث OpenSearch

**الملفات الجديدة:**
```
services/search-service/internal/infrastructure/rabbitmq/
├── consumer.go
└── index_handler.go   (يُعيد استخدام indexing.Service الموجود)
```

**تحديث:** `services/search-service/go.mod`

#### 4.5 media-service (Go) — Publisher

**الدور:** ينشر media.uploaded.* و media.processed.*

**الملف الجديد:** `services/media-service/internal/rabbitmq/publisher.go`

---

### PHASE 5 — Outbox Pattern للـ Publishers

**المبدأ:** لتحقيق Exactly-Once Delivery مع ACID consistency.

**لخدمات NestJS (delivery, user):**
```typescript
// في delivery-command.service.ts:
await queryRunner.transaction(async (em) => {
  await em.save(delivery);    // تحديث الـ delivery
  await em.save(outboxEntry); // حفظ في Outbox
});
// RabbitMQOutboxWorker يُرسل من outbox إلى RabbitMQ
```

**لخدمات Go (payment, driver, media):**
استخدام نفس pattern الـ outbox workers الموجودة في payment-service + إضافة `rabbitmq_outbox_worker.go`

---

### PHASE 6 — تحديث Docker Compose الكامل

**الملف:** `infrastructure/docker/compose.yml`

إضافة لكل خدمة:
```yaml
environment:
  RABBITMQ_URL: "amqp://${RABBITMQ_USER:-delivery}:${RABBITMQ_PASSWORD:-delivery-dev-password}@rabbitmq-srv:5672/${RABBITMQ_VHOST:-/}"
depends_on:
  rabbitmq-srv:
    condition: service_healthy
```

الخدمات المتأثرة: delivery, notification, realtime, payment, driver, search, media

---

### PHASE 7 — Kubernetes Manifests

#### إضافة RABBITMQ_URL لكل service deployment

في كل `*-depl.yaml`:
```yaml
env:
  - name: RABBITMQ_URL
    valueFrom:
      secretKeyRef:
        name: rabbitmq-secret
        key: RABBITMQ_URL
```

#### تحديث rabbitmq-secret.yaml

إضافة:
```yaml
data:
  RABBITMQ_URL: <base64 encoded amqp URL>
```

---

### PHASE 8 — Observability

**Prometheus Metrics:**
```
rabbitmq_messages_published_total{service, exchange, routing_key}
rabbitmq_messages_consumed_total{service, queue}
rabbitmq_messages_failed_total{service, queue, reason}
rabbitmq_messages_dlq_total{service, domain}
rabbitmq_consumer_lag{queue}
rabbitmq_connection_errors_total{service}
rabbitmq_circuit_breaker_state{service}
```

**Grafana:** إضافة dashboard في `infrastructure/observability/grafana/`

---

## 5. ملخص الملفات التي ستتغير

### ملفات Infrastructure (إصلاح عاجل)

| الملف | التغيير |
|---|---|
| `infrastructure/rabbitmq/enabled_plugins` | **إنشاء** (مفقود — يُسبب Docker failure) |
| `infrastructure/rabbitmq/definitions.json` | **تحديث** (exchanges + queues + policies جديدة) |
| `infrastructure/rabbitmq/rabbitmq.conf` | **تحديث طفيف** |
| `infrastructure/kubernetes/base/infrastructure/rabbitmq-depl.yaml` | **تحويل لـ StatefulSet** |
| `infrastructure/kubernetes/base/configmaps/rabbitmq-config.yaml` | **إضافة enabled_plugins** |

### ملفات packages/ts الجديدة

| الملف | نوع |
|---|---|
| `packages/ts/src/rabbitmq/rabbitmq.constants.ts` | جديد |
| `packages/ts/src/rabbitmq/rabbitmq.module.ts` | جديد |
| `packages/ts/src/rabbitmq/rabbitmq.service.ts` | جديد |
| `packages/ts/src/rabbitmq/rabbitmq.consumer.ts` | جديد |
| `packages/ts/src/rabbitmq/rabbitmq.health.ts` | جديد |
| `packages/ts/src/index.ts` | تحديث |
| `packages/ts/package.json` | تحديث (amqplib) |

### ملفات packages/go الجديدة

| الملف | نوع |
|---|---|
| `packages/go/rabbitmq/rabbitmq.go` | جديد |
| `packages/go/go.mod` | تحديث |

### ملفات Services الجديدة

| الملف | نوع |
|---|---|
| `services/delivery-service/src/modules/infrastructure/rabbitmq/rabbitmq.module.ts` | جديد |
| `services/delivery-service/src/modules/infrastructure/rabbitmq/rabbitmq.publisher.ts` | جديد |
| `services/delivery-service/src/app.module.ts` | تحديث |
| `services/notification-service/src/modules/rabbitmq/rabbitmq.module.ts` | جديد |
| `services/notification-service/src/modules/rabbitmq/consumers/email.consumer.ts` | جديد |
| `services/notification-service/src/modules/rabbitmq/consumers/sms.consumer.ts` | جديد |
| `services/notification-service/src/modules/rabbitmq/consumers/push.consumer.ts` | جديد |
| `services/notification-service/src/app.module.ts` | تحديث |
| `services/realtime-service/src/modules/infrastructure/rabbitmq/rabbitmq.module.ts` | جديد |
| `services/realtime-service/src/modules/infrastructure/rabbitmq/broadcast.consumer.ts` | جديد |
| `services/realtime-service/src/app.module.ts` | تحديث |
| `services/payment-service/internal/adapters/rabbitmq/publisher.go` | جديد |
| `services/payment-service/internal/adapters/rabbitmq/consumer.go` | جديد |
| `services/payment-service/go.mod` | تحديث |
| `services/driver-service/internal/adapters/rabbitmq/publisher.go` | جديد |
| `services/driver-service/internal/adapters/rabbitmq/dispatch_consumer.go` | جديد |
| `services/driver-service/go.mod` | تحديث |
| `services/search-service/internal/infrastructure/rabbitmq/consumer.go` | جديد |
| `services/search-service/go.mod` | تحديث |
| `services/media-service/internal/rabbitmq/publisher.go` | جديد |
| `services/media-service/go.mod` | تحديث |

### ملفات Docker/K8s تحتاج تحديث

| الملف | التغيير |
|---|---|
| `infrastructure/docker/compose.yml` | RABBITMQ_URL + depends_on لكل service (7 خدمات) |
| `infrastructure/kubernetes/base/secrets/rabbitmq-secret.yaml` | إضافة RABBITMQ_URL |
| كل `*-depl.yaml` في k8s/services/ | إضافة RABBITMQ_URL env var |

---

## 6. Design Patterns المستخدمة

| Pattern | أين يُطبَّق |
|---|---|
| **Outbox Pattern** | delivery, user, payment, media publishers |
| **Dead Letter Queue (per-domain)** | كل consumer |
| **Circuit Breaker** | RabbitMQService في packages/ts |
| **Idempotency / Inbox Pattern** | كل consumer (eventId + consumerName) |
| **Publisher Confirms** | RabbitMQService.publish() |
| **Prefetch + Back-pressure** | كل consumer يضبط prefetch حسب capacity |
| **Retry with Exponential Backoff** | BaseRabbitMQConsumer |
| **Message Envelope** | نفس EventEnvelope الموجود (eventId, eventType, payload, traceId) |
| **Adapter Pattern** | Go services: RabbitMQPublisherAdapter تغلف amqp091 |
| **Factory Pattern** | EventHandlerFactory في notification-service |
| **Strategy Pattern** | كل handler يطبق interface موحد |

---

## 7. تسلسل Events (Flow Diagrams)

### Order Flow عبر RabbitMQ

```
Customer → API Gateway
  → delivery-service (mutation)
    → DB: save delivery
    → Outbox: save event
    → [Worker] publish → delivery.orders.topic
      routing_key: "order.created"
        → orders.created.queue
            → notification-service: "طلبك تم استلامه"
            → realtime-service: WebSocket update
            → search-service: index new order

  → delivery.dispatch.direct
    routing_key: "dispatch.request"
      → dispatch.requests.queue
          → driver-service: find available driver
              → dispatch.responses.queue
                  → delivery-service: update driverId
                      → delivery.orders.topic
                          routing_key: "order.driver.assigned"
                            → notification-service: "تم تعيين سائق"
                            → realtime-service: driver tracking starts
```

### Payment Flow عبر RabbitMQ

```
Stripe Webhook → payment-service
  → DB: update payment
  → Outbox: save event
  → [Worker] publish → delivery.payments.direct
    routing_key: "payment.authorized"
      → payments.authorized.queue
          → delivery-service: transition to PAYMENT_CONFIRMED
    routing_key: "payment.failed"
      → payments.failed.queue
          → delivery-service: cancel delivery
          → notification-service: "فشل الدفع"
```

### Notification Fan-out

```
Any Service → delivery.notifications.fanout
  → notifications.email.queue (prefetch: 10)
  → notifications.sms.queue   (prefetch: 20)
  → notifications.push.queue  (prefetch: 50)
```

---

## 8. قرارات معمارية مهمة

### Kafka vs RabbitMQ — القاعدة المحددة

| الحالة | استخدم |
|---|---|
| Event Sourcing / Log replay | Kafka |
| Analytics / ClickHouse ingestion | Kafka |
| High-throughput media processing | Kafka |
| **Business workflow events** | **RabbitMQ** |
| **Notifications (email, sms, push)** | **RabbitMQ** |
| **Request-Reply (dispatch)** | **RabbitMQ** |
| Real-time WebSocket fan-out | RabbitMQ + NATS |

### Dual Publishing Strategy

لا نحذف Kafka من الخدمات فوراً — RabbitMQ يعمل بجانبه:
1. Delivery Events: Kafka (للـ analytics/search) + RabbitMQ (للـ notifications/realtime)
2. Payment Events: Kafka (للـ audit log) + RabbitMQ (للـ workflow)
3. بعد التحقق من الاستقرار، يمكن إزالة Kafka consumers للـ notifications تدريجياً.

### Connection Strategy

كل خدمة تفتح **connection واحدة** فقط مع RabbitMQ وتُنشئ channels متعددة منها.

---

## 9. المخاطر والتحذيرات

> [!CAUTION]
> ملف `enabled_plugins` المفقود يجعل Docker Compose يفشل تماماً. يجب إنشاؤه كأول خطوة.

> [!WARNING]
> لا تحذف Kafka consumers قبل التأكد من عمل RabbitMQ consumers. يجب أن يعمل الاثنان بالتوازي.

> [!IMPORTANT]
> Quorum Queues تتطلب odd number of nodes في production. Dev يعمل بـ node واحدة، لكن production يحتاج 3 replicas.

> [!NOTE]
> analytics-service غير موجود في compose.yml — يُضاف لاحقاً أو يُستبعد من definitions مؤقتاً.

---

## 10. ترتيب التنفيذ المقترح

```
Phase 1: إصلاح Infrastructure (enabled_plugins + definitions.json + rabbitmq.conf)
Phase 2: بناء packages/ts/src/rabbitmq/ (shared TypeScript package)
Phase 3: بناء packages/go/rabbitmq/rabbitmq.go (shared Go package)
Phase 4: تطبيق على delivery-service (Publisher)
Phase 5: تطبيق على notification-service (3 Consumers)
Phase 6: تطبيق على payment-service (Publisher + Consumer)
Phase 7: تطبيق على driver-service (Consumer + Publisher)
Phase 8: تطبيق على realtime-service (Consumer)
Phase 9: تطبيق على search-service (Consumer)
Phase 10: تطبيق على media-service (Publisher)
Phase 11: تحديث Docker Compose الكامل
Phase 12: تحديث Kubernetes Manifests
Phase 13: Observability (Grafana Dashboard)
Phase 14: إنشاء ملف system_architecture_rabbitmq.md في الـ root
```

---

**في انتظار موافقتك للبدء في التنفيذ.**
