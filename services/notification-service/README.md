# Notification Service

The **Notification Service** is one of the microservices of the realtime delivery platform. It is responsible for the **entire lifecycle of a notification**:

1. **Ingest** notification intents from many sources (Kafka domain events, gRPC `SendNotification`, GraphQL / in-process calls).
2. **Decide** which channels a user has enabled (preferences) and whether to deliver at all.
3. **Prepare** the final title/body by rendering Handlebars templates per type + channel + locale.
4. **Persist** the notification and its per-channel delivery records atomically (transaction + outbox pattern).
5. **Deliver** through background BullMQ workers per channel, delegating the actual channel send to the shared `@bts-soft/notifications` package.
6. **Observe** delivery per channel (SENT / FAILED / retries).

The service deliberately does **not** own user data. Authentication/authorization is enforced through the shared `RoleGuard` + `USER_SERVICE` (resolved over gRPC from the User Service, with a Redis cache), and the GraphQL API is exposed as a federated subgraph.

---

## Table of contents

- [Tech stack](#tech-stack)
- [How a notification flows](#how-a-notification-flows)
- [Folder structure](#folder-structure)
- [Why multiple entities?](#why-multiple-entities)
- [Key components](#key-components)
  - [app.module.ts](#appmodulets)
  - [main.ts](#maints)
  - [AuthModule (real auth)](#authmodule)
  - [KafkaModule](#kafkamodule)
  - [OutboxModule](#outboxmodule)
  - [NotificationModule](#notificationmodule)
  - [WorkersModule](#workersmodule)
  - [GrpcModule](#grpcmodule)
- [GraphQL API](#graphql-api)
- [Response format](#response-format)
- [Configuration](#configuration)
- [Important decisions & fixes](#important-decisions--fixes)

---

## Tech stack

| Concern            | Technology                                              |
| ------------------ | ------------------------------------------------------- |
| Framework          | NestJS 11                                               |
| GraphQL (public)   | Apollo Federation driver (subgraph at `/notification/graphql`) |
| Database           | PostgreSQL + TypeORM (`synchronize` for dev)            |
| Queues             | BullMQ (`notification-email | sms | push | inapp | realtime`) |
| Cache / Redis      | `@bts-soft/cache` (`RedisService`)                      |
| Notifications      | `@bts-soft/notifications` (`NotificationService.send()`) |
| Event streaming    | Kafka (consumers) via `kafkajs`                         |
| Realtime bus       | NATS (outbox → `NotificationNatsSubjects.NOTIFICATION_USER.<userId>`) |
| RPC                | gRPC (inbound `SendNotification`, outbound user lookup) |
| Shared platform    | `@delivery/common` (`file:../../packages/ts`)           |
| Auth               | JWT (`RoleGuard`, `Auth(...)`)                          |
| i18n               | `nestjs-i18n` (locales `en`, `ar`)                      |
| Observability      | `LoggingModule`, `MetricsModule`, `AutomationModule`    |

---

## How a notification flows

```
  Kafka topic (delivery.* / payment.*)          gRPC SendNotification          GraphQL Mutations
         │                                             │                                │
         ▼                                             ▼                                ▼
  KafkaConsumer ──► Inbox(idempotency) ──► EventHandlerFactory.handler
                                                          │
                                                          ▼
                                           NotificationService.createAndDispatch()
                                                          │
                                ┌─────────────────────────┼─────────────────────────┐
                                ▼                         ▼                         ▼
                     PreferenceService            TemplateService              (outbox, if
                     resolves enabled          renders Handlebars            REALTIME channel)
                     channels per user               title/body                        │
                                └─────────────────────────┼─────────────────────────┘
                                                          ▼
                              ┌── Transaction ────────────────────────────────┐
                              │  notifications (1) + notification_delivery (N) │
                              │  + notification_outbox (if realtime enabled)  │
                              └───────────────────────────────────────────────┘
                                                          │
                                                          ▼
                                        NotificationDispatcherService
                                  adds a BullMQ job per enabled channel
                                          (jobId = <notificationId>:<channel>)
                                                          │
                         ┌──────────────┬───────────────┬──┴──────────┬──────────────┐
                         ▼              ▼               ▼             ▼              ▼
                 email.worker    sms.worker     push.worker   inapp.worker   realtime.worker
                         │              │               │             │                │
                         │              │               │             │                │
                         └──────────────┴───────────────┴─────────────┘                │
                              NotificationService.send(ChannelType, NotificationMessage)│
                              (@bts-soft/notifications → per-channel providers)         │
                                                                                        │
                                            OutboxWorkerService ──► NATS notification.<userId>
```

### Inbox / Outbox pattern

- **Inbox (`notification_inbox`)**: Kafka messages are processed **at-least-once**. Before handling an event the consumer checks whether `eventId` was already processed for consumer `notification-service`; duplicate events are skipped, guaranteeing idempotency. Only after the handler succeeds is the inbox row written.
- **Outbox (`notification_outbox`)**: When a notification enables the `REALTIME` channel, a `NOTIFICATION_CREATED` outbox row is written inside the **same DB transaction** that creates the notification. `OutboxWorkerService` polls pending rows every 5s and emits them to NATS subject `notification.<userId>`, marking them `PUBLISHED` (or `FAILED` after 5 attempts). This gives reliable realtime fan-out without in-process race conditions.

---

## Folder structure

```
src/
├── app.module.ts                     # Root module: config, DB, Redis, queues, GraphQL, auth, filters/interceptors
├── app.resolver.ts                   # Health `ping` query
├── main.ts                           # Bootstrap: gRPC transport, global pipes + interceptors
├── common/
│   ├── common.module.ts              # @Global: TypeORM repos + RedisModule + SHARED_REDIS_SERVICE
│   ├── health.controller.ts          # /health endpoint (DB + Redis)
│   ├── graphql/general-response.type.ts   # GeneralResponse<T>, BooleanResponse, IntResponse
│   ├── interceptors/graphql-response.interceptor.ts  # uniform { success, statusCode, message, data }
│   ├── translation/                  # TranslationModule + locales/{en,ar}/*.json
│   └── database/entities/
│       ├── notification.entity.ts
│       ├── notification-delivery.entity.ts
│       ├── notification-template.entity.ts
│       ├── notification-preference.entity.ts
│       ├── notification-outbox.entity.ts
│       └── notification-inbox.entity.ts
└── modules/
    ├── auth/                         # grpc-clients.module, user-lookup.service, auth.module
    ├── kafka/                        # kafka.consumer, event-handlers/*
    ├── notification/                 # notification.service|resolver|dispatcher, template/*, preference/*
    ├── outbox/                       # outbox-worker.service
    ├── workers/                      # per-channel BullMQ workers + channel-message.helper
    └── grpc/                         # grpc.controller (inbound SendNotification RPC)
```

---

## Why multiple entities?

The notification domain is split into six cohesive tables on purpose:

| Entity                  | Table                    | Purpose |
| ----------------------- | ------------------------ | ------- |
| `Notification`          | `notifications`          | The **aggregate**: one logical notification for one user (type, title, body, priority, status, read state). |
| `NotificationDelivery`  | `notification_delivery`  | One row per **channel targeted** for a notification (EMAIL, SMS, ...). Tracks per-channel status, attempts, `sentAt`/`deliveredAt`. Enables retrying only the failed channel instead of re-sending everything. |
| `NotificationTemplate`  | `notification_templates` | Versioned **Handlebars templates** keyed by `type + channel + locale`. Keeps copy out of code; supports i18n (en/ar) and fast in-memory + Redis caching. |
| `NotificationPreference`| `notification_preferences`| Per-user opt-in/opt-out per `(userId, type, channel)`. `PreferenceService.getEnabledChannels()` uses this to decide which channels are allowed; if none exist, sane defaults (IN_APP + PUSH) are used. |
| `NotificationOutbox`    | `notification_outbox`    | **Transactional outbox** for realtime fan-out via NATS (see above). |
| `NotificationInbox`     | `notification_inbox`     | **Idempotency inbox** for Kafka at-least-once consumption. |

This separation keeps the aggregate clean, allows per-channel retries, isolates templating, and gives reliable, exactly-once-shipped event handling.

---

## Key components

### app.module.ts

The root module wires the platform infrastructure exactly like the User Service:

- `ConfigModule.forRoot` reads `../../.env` (repository root) — the DB password (`POSTGRES_PASSWORD`) is **never hard-coded**.
- `TranslationModule` (i18n) for localized error messages / responses.
- `RedisModule`, BullMQ `forRootAsync`, BTS `NotificationModule`.
- `JwtModule.registerAsync({ global: true })` — consumes `JWT_SECRET` / `JWT_EXPIRE`.
- `GraphQLModule.forRoot<ApolloFederationDriverConfig>` — federated subgraph at `/notification/graphql`.
- `LoggingModule`, `MetricsModule`, `AutomationModule` from `@delivery/common`.
- `AuthModule`, `CommonModule`, `KafkaModule`, `NotificationModule`, `WorkersModule`, `OutboxModule`, `GrpcModule`.
- Global `APP_FILTER` = `HttpExceptionFilter` (`@bts-soft/core`).
- Global `APP_INTERCEPTOR`s = `GraphqlResponseInterceptor` (uniform responses) + `MetricsInterceptor`.

### main.ts

- Creates the app with `StructuredLogger` (no noisy array log config).
- `setupInterceptors(app)` — class serializer + SQL injection + general response interceptors.
- Global `ValidationPipe` with `transform`, `whitelist`, `stopAtFirstError` and `I18nValidationException` factory.
- Registers the inbound **gRPC** transport (`notification` package from `protos/notification.proto`).
- Boots microservices then listens on `PORT_NOTIFICATION` (default `4004`).

### AuthModule

Replaces the previous **temporary mock** auth with a real, service-resolved implementation:

- `GrpcClientsModule` → `ClientsModule.register({ name: 'GRPC_USER_SERVICE', transport: GRPC, package: 'user', url: USER_GRPC_URL })`.
- `UserLookupService` implements the `USER_SERVICE` contract (`findById`) that `RoleGuard` requires. It:
  1. checks the Redis cache `notification:user:<id>` (TTL 300s),
  2. calls `GetUser` on the User Service over gRPC,
  3. normalizes legacy `customer` rows to `user` (roles have been merged),
  4. returns `{ id, email, role }`.
- `AuthModule` is `@Global` and registers:
  - `UserLookupService`
  - `RoleGuard`
  - `{ provide: 'USER_SERVICE', useExisting: UserLookupService }`

Every `@Auth(...)` resolver now verifies the JWT, loads the real user, and checks the role-based permission map (`rolePermissionsMap` in `packages/ts`). Note the roles have been consolidated: `CUSTOMER` was merged into `USER` (now `ADMIN | USER | DRIVER`), so no stale `customer` permissions exist anywhere.

### KafkaModule

- `KafkaConsumer` connects with `KAFKA_BROKERS` (default `kafka-srv:9092`), subscribes to `DeliveryKafkaTopics` + `PaymentKafkaTopics`.
- For each message: parses the payload as `KafkaEventPayload`, computes `eventId`, performs the **inbox idempotency check**, dispatches to `EventHandlerFactory` (a registry of `IEventHandler` keyed by event type), and records the inbox row.
- `IEventHandler.handle(payload: KafkaEventPayload)` — fully typed (`Record<string, unknown>` payloads, no `any`).
- Note: handlers can be registered with `registerHandler(eventType, handler)` for e.g. `DELIVERY_CREATED`, `PAYMENT_COMPLETED`, etc.

### OutboxModule

`OutboxWorkerService` polls `notification_outbox` (PENDING, ordered by `createdAt`, batch of 50) every 5s and emits each payload to NATS subject `notification.<userId>` via a lazily-created NATS client (`NATS_URL`, default `nats://nats-srv:4222`). Failed sends increment `attemptCount` and flip to `FAILED` after 5 attempts.

### NotificationModule

- `NotificationService` — the domain core:
  - `createAndDispatch(params)` — resolves preferences → renders template → **transactionally** creates the notification, one delivery per enabled channel, and the realtime outbox row → dispatches BullMQ jobs.
  - `findAllForUser(userId, page, limit)` — paginated list (limit clamped to 100), deliveries eager-loaded, newest first.
  - `findByIdForUser(id, userId)` — single notification **scoped to the owner**.
  - `unreadCountForUser(userId)` — unread count.
  - `markAsRead(id, userId)` — marks read (throws `NotFoundException` with i18n message if missing).
  - `markAllAsRead(userId)` — bulk update of unread rows.
  - `delete(id, userId)` — hard delete scoped to the owner.
- `NotificationDispatcherService` — enqueues one `send` job per delivery on the right queue with `jobId = <notificationId>:<deliveryChannel>` (dedup).
- `TemplateService` — Handlebars rendering with a two-level cache: in-memory compiled map (hot path) + Redis raw-template cache-aside (`template:<type>:<channel>:<locale>`, TTL 1h), falling back from requested locale to `en`.
- `PreferenceService` — resolves enabled channels for a user/type, defaulting to `IN_APP` + `PUSH`.
- `NotificationResolver` — thin GraphQL layer delegating to `NotificationService`, returning `GeneralResponse` types, decorated with `@Auth(Permission...)` and `@RedisRateLimit`.

### WorkersModule

One BullMQ worker per queue (`notification-email`, `notification-sms`, `notification-push`, `notification-inapp`, `notification-realtime`). Each worker:

1. loads the delivery row → `PROCESSING`, increments `attemptCount`,
2. loads the notification,
3. builds a proper **`NotificationMessage`** via `buildChannelMessage()` (`channel-message.helper.ts`) — maps domain priority (`LOW/NORMAL/HIGH/CRITICAL`) to the package's numeric BullMQ priorities (`10/5/2/1`) and attaches `context` from `notification.data`,
4. calls `NotificationService.send(ChannelType.X, message)` from `@bts-soft/notifications`,
5. marks the delivery `SENT` + `sentAt`, or `FAILED` + `lastError` and rethrows so BullMQ retries.

The realtime worker simply marks its delivery sent — the actual push to the client happens through the outbox/NATS pipeline.

### GrpcModule

`GrpcController` implements `NotificationService.SendNotification` (proto `notification.proto`). It accepts `{ userId, type, title, body, data(JSON string), priority }`, parses `data` safely as `Record<string, unknown>`, and calls `createAndDispatch`. Returns `{ notificationId, success }`.

---

## GraphQL API

All queries/mutations require a `Bearer` JWT and are permission/rate limited:

| Operation | Type | Permissions | Rate limit (per minute) |
| --------- | ---- | ----------- | ----------------------- |
| `myNotifications(page, limit)` | Query | `READ_NOTIFICATION` | 100 |
| `notification(id)` | Query | `READ_NOTIFICATION` | 100 |
| `unreadNotificationCount` | Query | `READ_NOTIFICATION` | 100 |
| `markNotificationAsRead(id)` | Mutation | `UPDATE_NOTIFICATION` | 60 |
| `markAllNotificationsAsRead` | Mutation | `UPDATE_NOTIFICATION` | 60 |
| `deleteNotification(id)` | Mutation | `DELETE_NOTIFICATION` | 50 |
| `ping` | Query | (public) | – |

Permissions map to roles via `rolePermissionsMap` in `packages/ts` (`USER` and `DRIVER` have `READ_NOTIFICATION`/`UPDATE_NOTIFICATION`; `DELETE_NOTIFICATION` is admin-only).

---

## Response format

Every resolver returns a `GeneralResponse<T>` / `BooleanResponse` / `IntResponse`:

```json
{
  "success": true,
  "statusCode": 200,
  "message": "Notifications retrieved successfully",
  "data": { }
}
```

- `GraphqlResponseInterceptor` passes through objects that already have `success` + `statusCode`; anything else is wrapped by `ResponseFormatter.formatSuccess(...)` from `@bts-soft/core`.
- Errors are formatted by the global `HttpExceptionFilter` (GraphQL) and i18n keys in `locales/{en,ar}/{user,notification}.json`.

---

## Configuration

From the repository-root `.env` (`envFilePath: ../../.env`):

| Variable              | Default               | Used for                       |
| --------------------- | --------------------- | ------------------------------ |
| `POSTGRES_PASSWORD`   | (required from env)   | Postgres password (`DB_USERNAME`, `DB_NAME`, `DB_HOST`, `DB_PORT` also supported) |
| `REDIS_HOST`          | `localhost`           | Redis + BullMQ + rate limit    |
| `REDIS_PORT`          | `6379`                | Redis                          |
| `REDIS_DB`            | `0`                   | Redis                          |
| `KAFKA_BROKERS`       | `kafka-srv:9092`      | Kafka consumer                 |
| `KAFKA_GROUP_ID`      | `notification-service`| Kafka consumer group           |
| `NATS_URL`            | `nats://nats-srv:4222`| Realtime outbox publisher      |
| `USER_GRPC_URL`       | `user-srv:50051`      | gRPC user lookup               |
| `JWT_SECRET`          | `default_secret`      | JWT verification (RoleGuard reads `process.env.JWT_SECRET`) |
| `JWT_EXPIRE`          | `1d`                  | JWT token expiry               |
| `PORT_NOTIFICATION`   | `4004`                | HTTP/GraphQL port              |
| `PORT_GRPC`           | `50053`               | gRPC server port               |

---

## Important decisions & fixes

- **No secrets in code** — `POSTGRES_PASSWORD` is read from config, never a literal default.
- **No mock auth** — the temporary `userService: { findById: () => mock }` was replaced by `AuthModule` with a real gRPC-backed `USER_SERVICE`.
- **Role consolidation** — `CUSTOMER` merged into `USER` (`packages/ts` `enum.constant.ts` + `rolePermissionsMap.constant.ts`); legacy `customer` rows are normalized to `user` at auth time.
- **Snowflake IDs** — every entity primary key is a 64-char string snowflake produced by `IdGenerator.generate('snowflake')` from `@bts-soft/common` (removed `@PrimaryGeneratedColumn('uuid')`).
- **No `any`** — the service is fully typed: `Record<string, unknown>` for JSON payloads, typed Kafka payloads, typed GraphQL responses, and a typed `NotificationMessage` builder for the package (no `as any` casts).
- **Uniform responses & observability** — same `GeneralResponse` + interceptor pattern, global exception filter, `MetricsInterceptor`, rate limiting (`SHARED_REDIS_SERVICE`), logging, and automation as the User Service.
- **Orphaned code removed** — auction/AI leftover DTOs (`update.dto.ts`, `aiMessageChunk.dto.ts`, stale `grpc-clients.module.ts`, unused pagination/currentUser scaffolding) deleted.

---

## Scripts

```bash
npm install --legacy-peer-deps   # install (needed due to graphql/apollo peer ranges)
npm run build                    # nest build (tsc)
npm run start:dev                # watch mode
npm run start:prod               # run compiled dist/main
```