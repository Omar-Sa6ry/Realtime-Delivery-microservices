# Notification Service

The Notification Service is a standalone NestJS microservice inside the Realtime Delivery platform monorepo. It owns the complete lifecycle of user notifications: ingesting notification intents from multiple sources (Kafka, RabbitMQ, gRPC, in-process calls), deciding which channels a user has enabled, rendering localized templates, persisting notifications and per-channel delivery records transactionally, delivering through background workers, and exposing a federated GraphQL API for clients.

The service deliberately does not own user data. Authentication and authorization are delegated to the User Service over gRPC through the shared RoleGuard from `@delivery-micro/shard`.

---

## Table of Contents

- [Overview](#overview)
- [Tech Stack](#tech-stack)
- [Repository Layout](#repository-layout)
- [How a Notification Flows](#how-a-notification-flows)
- [Inbox and Outbox Patterns](#inbox-and-outbox-patterns)
- [Database Schema](#database-schema)
- [Domain Enums](#domain-enums)
- [Key Components](#key-components)
  - [AppModule](#appmodule)
  - [main.ts Bootstrap](#maints-bootstrap)
  - [CommonModule](#commonmodule)
  - [TranslationModule](#translationmodule)
  - [AuthModule](#authmodule)
  - [KafkaModule](#kafkamodule)
  - [RabbitMQNotificationModule](#rabbitmqnotificationmodule)
  - [NotificationModule](#notificationmodule)
  - [WorkersModule](#workersmodule)
  - [OutboxModule](#outboxmodule)
  - [GrpcModule](#grpcmodule)
- [GraphQL API](#graphql-api)
- [gRPC API](#grpc-api)
- [HTTP Endpoints](#http-endpoints)
- [Caching Strategy](#caching-strategy)
- [i18n](#i18n)
- [Configuration](#configuration)
- [Docker Compose](#docker-compose)
- [Makefile](#makefile)
- [Scripts](#scripts)
- [Testing](#testing)
- [Dependencies](#dependencies)
- [Design Decisions](#design-decisions)

---

## Overview

Responsibilities of this service:

1. Ingest notification intents from Kafka domain events, RabbitMQ events, the inbound gRPC `SendNotification` RPC, and internal service calls.
2. Resolve which channels (EMAIL, SMS, PUSH, IN_APP, REALTIME) a user has enabled for a given notification type through per-user preferences.
3. Render the final title and body from Handlebars templates keyed by type + channel + locale.
4. Persist the notification aggregate, its per-channel delivery rows, and (when the REALTIME channel is enabled) a transactional outbox row in a single database transaction.
5. Enqueue one BullMQ job per delivery on the correct queue.
6. Deliver through per-channel workers that delegate the actual send to the shared `@bts-soft/notifications` package.
7. Track per-channel delivery status (PENDING / PROCESSING / SENT / FAILED), retries, and expiration.
8. Fan out realtime events to NATS through the outbox worker.
9. Expose a federated GraphQL subgraph at `/notification/graphql` with JWT auth, permission checks, and Redis-backed rate limiting.
10. Expose health, readiness, and Prometheus metrics endpoints.

---

## Tech Stack

| Concern | Technology |
| --- | --- |
| Runtime | Node.js with TypeScript 5.7 |
| Framework | NestJS 11 |
| GraphQL (public) | Apollo Federation v2 subgraph driver at `/notification/graphql` |
| Database | PostgreSQL 15 + TypeORM (`synchronize` enabled for development) |
| Background jobs | BullMQ (Redis-backed queues and workers) |
| Cache | `@bts-soft/cache` (`RedisService`) |
| Channel delivery SDK | `@bts-soft/notifications` (`NotificationService.send()`) |
| Event streaming (inbound) | Kafka via `kafkajs` (`BaseKafkaConsumer` from `@delivery-micro/shard`) |
| Event streaming (inbound) | RabbitMQ via `@delivery-micro/shard` (`BaseRabbitMQConsumer`) |
| Realtime bus (outbound) | NATS via `@nestjs/microservices` NATS transport (outbox worker) |
| RPC (inbound) | gRPC server `NotificationService.SendNotification` (package `notification`) |
| RPC (outbound) | gRPC client to User Service (`GetUser`) for auth user resolution |
| Shared platform SDK | `@delivery-micro/shard` (guards, enums, Kafka/RabbitMQ base classes, metrics, logging, alerts) |
| Auth | JWT (`JwtModule` + `RoleGuard` + `@Auth(...)` decorator) |
| Templating | Handlebars |
| i18n | `nestjs-i18n` (locales `en`, `ar`) |
| Validation | `class-validator` + `class-transformer` + `I18nValidationException` |
| Observability | `LoggingModule` (structured logs), `MetricsModule` (Prometheus), `AutomationModule` (alerting), `MetricsInterceptor` |
| Health | `HealthController` (`/notification/health`, `/notification/ready`) with DB and Redis checks |
| Testing | Jest 30 + ts-jest, supertest (e2e) |

---

## Repository Layout

The service lives at `services/notification-service` inside the monorepo. The monorepo root contains shared packages and protos:

```
Realtime Delivery microservices/
├── config/env/                         # .env.development / .env.production / .env.test (shared env files)
├── packages/ts/                        # Shared TypeScript constants (roles, permissions, etc.)
├── protos/                             # Shared gRPC protobuf definitions
│   ├── notification.proto              # Inbound SendNotification RPC contract
│   └── user.proto                      # Outbound GetUser RPC contract
├── services/
│   └── notification-service/           # This service
└── docker-compose.yml (per service)    # Local infra + service container
```

Full source tree of this service:

```
services/notification-service/
├── Dockerfile
├── Makefile                                # Task runner (install/build/test/docker/health/clean)
├── README.md
├── docker-compose.yml
├── nest-cli.json
├── package.json
├── tsconfig.json
├── tsconfig.build.json
├── src/
│   ├── main.ts                                   # Bootstrap: HTTP + gRPC + validation pipe
│   ├── app.module.ts                             # Root module wiring
│   ├── app.resolver.ts                           # Public `ping` GraphQL query
│   ├── app.resolver.spec.ts
│   ├── common/
│   │   ├── common.module.ts                      # @Global TypeORM repositories + Redis + SHARED_REDIS_SERVICE
│   │   ├── health.controller.ts                  # GET /notification/health, GET /notification/ready
│   │   ├── metrics.controller.ts                 # GET /notification/metrics (Prometheus)
│   │   ├── health.controller.spec.ts
│   │   ├── metrics.controller.spec.ts
│   │   ├── database/entities/
│   │   │   ├── notification.entity.ts            # Aggregate root (notifications table)
│   │   │   ├── notification-delivery.entity.ts   # Per-channel delivery rows
│   │   │   ├── notification-template.entity.ts   # Handlebars templates
│   │   │   ├── notification-preference.entity.ts # Per-user opt-in/opt-out
│   │   │   ├── notification-outbox.entity.ts     # Transactional outbox for NATS
│   │   │   └── notification-inbox.entity.ts      # Idempotency inbox for Kafka/RabbitMQ
│   │   └── translation/
│   │       ├── translation.module.ts             # I18nModule (fallback en, x-lang + Accept-Language resolvers)
│   │       └── locales/
│   │           ├── en/{notification.json,user.json}
│   │           └── ar/{notification.json,user.json}
│   ├── modules/
│   │   ├── auth/
│   │   │   ├── auth.module.ts                    # @Global: UserLookupService + RoleGuard + USER_SERVICE token
│   │   │   ├── grpc-clients.module.ts            # GRPC_USER_SERVICE client registration (user.proto)
│   │   │   ├── user-lookup.service.ts            # findById: gRPC GetUser + role normalization
│   │   │   └── user-lookup.service.spec.ts
│   │   ├── kafka/
│   │   │   ├── kafka.module.ts
│   │   │   ├── kafka.consumer.ts                 # Extends BaseKafkaConsumer; inbox idempotency
│   │   │   ├── kafka.consumer.spec.ts
│   │   │   └── event-handlers/
│   │   │       ├── event-handler.interface.ts    # IEventHandler + KafkaEventPayload
│   │   │       ├── event-handler.factory.ts      # Registry of handlers keyed by eventType
│   │   │       ├── event-handler.factory.spec.ts
│   │   │       ├── notification-event-handler.service.ts  # EVENT_MAPPING + createAndDispatch
│   │   │       └── notification-event-handler.service.spec.ts
│   │   ├── rabbitmq/
│   │   │   ├── rabbitmq.module.ts                # RabbitMQNotificationModule
│   │   │   ├── rabbitmq-notification.mapper.ts   # Envelope -> notification view
│   │   │   ├── rabbitmq-notification.mapper.spec.ts
│   │   │   └── consumers/
│   │   │       ├── email.consumer.ts             # notifications.email.queue consumer
│   │   │       ├── sms.consumer.ts               # notifications.sms.queue consumer
│   │   │       ├── push.consumer.ts              # notifications.push.queue consumer
│   │   │       └── *.spec.ts
│   │   ├── notification/
│   │   │   ├── notification.module.ts            # Entities + all BullMQ queues + core providers
│   │   │   ├── notification.service.ts           # createAndDispatch + CRUD + mark read
│   │   │   ├── notification.service.spec.ts
│   │   │   ├── notification-dispatcher.service.ts  # enqueue per-channel jobs + outbox row
│   │   │   ├── notification-dispatcher.service.spec.ts
│   │   │   ├── notification.resolver.ts          # GraphQL queries/mutations
│   │   │   ├── notification.resolver.spec.ts
│   │   │   ├── template/
│   │   │   │   ├── template.service.ts           # Handlebars + seed defaults + 2-level cache
│   │   │   │   └── template.service.spec.ts
│   │   │   ├── preference/
│   │   │   │   ├── preference.service.ts         # getEnabledChannels / upsert / find
│   │   │   │   └── preference.service.spec.ts
│   │   │   ├── dtos/
│   │   │   │   ├── notification.dto.ts           # GraphQL response types
│   │   │   │   └── preference.dto.ts
│   │   │   └── inputs/
│   │   │       └── preference.input.ts           # GraphQL input for preference updates
│   │   ├── workers/
│   │   │   ├── workers.module.ts
│   │   │   ├── channel-message.helper.ts         # buildChannelMessage(): domain -> package message
│   │   │   ├── channel-message.helper.spec.ts
│   │   │   ├── delivery-state.service.ts         # Per-delivery + aggregate status transitions
│   │   │   ├── delivery-state.service.spec.ts
│   │   │   ├── email.worker.ts                   # @Processor('notification-email')
│   │   │   ├── sms.worker.ts                     # @Processor('notification-sms')
│   │   │   ├── push.worker.ts                    # @Processor('notification-push')
│   │   │   ├── inapp.worker.ts                   # @Processor('notification-inapp')
│   │   │   ├── realtime.worker.ts                # @Processor('notification-realtime')
│   │   │   ├── scheduled.worker.ts               # @Processor('notification-scheduled')
│   │   │   ├── retry.worker.ts                   # @Processor('notification-retry') + sweep scheduler
│   │   │   └── *.spec.ts
│   │   ├── outbox/
│   │   │   ├── outbox.module.ts
│   │   │   ├── outbox-worker.service.ts          # Polls PENDING rows every 5s -> NATS emit
│   │   │   └── outbox-worker.service.spec.ts
│   │   └── grpc/
│   │       ├── grpc.module.ts
│   │       ├── grpc.controller.ts                # @GrpcMethod NotificationService.SendNotification
│   │       └── grpc.controller.spec.ts
│   └── test-utils/
│       ├── index.ts                              # Barrel export
│       ├── mock-repository.factory.ts            # TypeORM repository mocks
│       ├── mock-query-runner.factory.ts          # QueryRunner + transaction mocks
│       └── mock-bullmq-queue.factory.ts          # BullMQ Queue mocks
└── test/
    ├── jest-e2e.json                             # E2E Jest config (ts-jest, diagnostics off)
    ├── app.e2e-spec.ts                           # HTTP endpoints: health, ready, metrics
    ├── notification-flow.e2e-spec.ts             # GraphQL federation flow with JWT + guards
    └── grpc.e2e-spec.ts                          # Real gRPC server + real client over notification.proto
```

---

## How a Notification Flows

```
 Kafka topics (delivery.* / payment.* / media.* / user.*)
 RabbitMQ (delivery.notifications.fanout -> notifications.{email,sms,push}.queue)
 gRPC SendNotification
 GraphQL mutations (read/update/delete; no create via GraphQL)
          |
          v
 +---------------------+
 | Ingest layer        |
 |  - KafkaConsumer    |  inbox idempotency check (eventId + consumer)
 |  - RabbitMQ consumers (email/sms/push) with same inbox pattern
 |  - GrpcController
 +----------+----------+
            |
            v
 EventHandlerFactory.getHandler(eventType)
            |
            v
 NotificationEventHandler.handle(payload)
   - extractUserId (userId / user_id / customerId / driverId from payload or envelope)
   - EVENT_MAPPING: topic -> NotificationType (+ priority + optional requiredChannels)
            |
            v
 NotificationService.createAndDispatch(params)
   1. Resolve channels: requiredChannels override OR PreferenceService.getEnabledChannels()
      (default IN_APP + PUSH when the user has no preference rows)
   2. TemplateService.render(type, firstChannel, 'en', data)
      - two-level cache: in-memory compiled Map, then Redis raw-template cache-aside, then DB
      - fallback locale: requested locale -> en -> generic "Notification: TYPE" format
   3. Single DB transaction (TypeORM QueryRunner):
      - INSERT notifications (1 row)
      - INSERT notification_deliveries (1 row per channel)
   4. Dispatch or schedule:
      - if scheduledAt in the future -> ScheduledWorker delayed job
      - else NotificationDispatcherService.dispatch()
            |
            v
 NotificationDispatcherService
   - for each delivery: queue.add('send', { notificationId, deliveryId }, { jobId: '<id>:<channel>' })
   - if any delivery is REALTIME: also writes a NOTIFICATION_CREATED outbox row (deduped by aggregateId)
            |
   +--------+---------+-----------+------------+--------------+
   v        v         v           v            v              v
 email    sms      push       inapp     realtime        scheduled
 worker   worker   worker     worker     worker          worker
   |        |         |           |            |              |
   +--------+---------+-----------+            |     dispatch pending deliveries
            |                                  |     at scheduledAt
            v                                  v
 @bts-soft/notifications NotificationService.send(ChannelType, NotificationMessage)
   (per-channel providers: SMTP/SendGrid/SES for email, Twilio/SMSMISR/Vonage for SMS,
    OneSignal/Firebase/Pusher for push, etc. — configured via env)
            |
            v
 DeliveryStateService
   beginProcessing -> PROCESSING, attemptCount += 1
   complete        -> SENT + sentAt (+ deliveredAt)
   fail            -> FAILED + lastError + failedAt; rethrow so BullMQ retries
            |
            v
 RetryWorker sweep (every 60s):
   - expire notifications past expiresAt (CREATED/QUEUED/PROCESSING)
   - requeue FAILED deliveries with attemptCount < 5 after 60s backoff

 OutboxWorkerService (every 5s):
   - poll notification_outbox PENDING (batch 50, oldest first)
   - NATS emit to subject `notification.user.<userId>`
   - mark PUBLISHED, or FAILED after 5 attempts + CRITICAL alert
```

---

## Inbox and Outbox Patterns

### Inbox (`notification_inbox`)

Kafka and RabbitMQ consumers process messages at-least-once. Before handling an event, the consumer checks whether `(eventId, consumer)` already exists in the inbox table:

- Kafka consumer uses consumer name `notification-service`.
- RabbitMQ consumers use consumer name `notification-service-rabbitmq`.

If the row already exists the message is skipped (idempotency). After the handler succeeds (or finds no handler), the inbox row is written. A unique constraint on `(eventId, consumer)` provides a second line of defense against race conditions.

### Outbox (`notification_outbox`)

When a notification's delivery set includes the REALTIME channel, `NotificationDispatcherService.createRealtimeOutbox` writes a `NOTIFICATION_CREATED` outbox row (deduplicated by `eventType + aggregateId`) in a separate save after dispatch. `OutboxWorkerService` polls PENDING rows every 5 seconds (batch of 50, ordered by `createdAt` ascending), emits each payload to NATS subject `notification.user.<userId>` (built from `NotificationNatsSubjects.NOTIFICATION_USER` + `.` + userId), and marks the row `PUBLISHED`. On failure it increments `attemptCount` and flips the row to `FAILED` after 5 attempts, triggering a CRITICAL alert through `AlertService`.

This gives reliable realtime fan-out without in-process races and without losing events if the process crashes between DB commit and NATS publish.

---

## Database Schema

Six cohesive tables. All primary keys are 64-character snowflake strings generated by `IdGenerator.generate('snowflake')` from `@bts-soft/common` (no UUID auto-increment columns).

### notifications (aggregate)

| Column | Type | Notes |
| --- | --- | --- |
| id | varchar(64) PK | Snowflake ID, generated in the entity field initializer |
| userId | varchar | Indexed; target user |
| type | enum NotificationType | Delivery/payment/media/user event category |
| title | varchar | Rendered from template or caller-supplied override |
| body | varchar | Rendered from template or caller-supplied override |
| data | jsonb nullable | Arbitrary context passed to templates and channel providers |
| priority | enum NotificationPriority | Default `NORMAL` |
| status | enum NotificationStatus | Default `CREATED` |
| scheduledAt | timestamp nullable | Future dispatch time |
| expiresAt | timestamp nullable | Expiration deadline (enforced by RetryWorker sweep) |
| readAt | timestamp nullable | Null = unread |
| createdAt / updatedAt | timestamp | Auto-managed |
| deliveries | OneToMany -> NotificationDelivery | Cascade delete on the delivery side |

### notification_deliveries (per-channel rows)

| Column | Type | Notes |
| --- | --- | --- |
| id | varchar(64) PK | Snowflake |
| notificationId | varchar FK | Indexed; `onDelete: CASCADE` to notifications |
| channel | enum NotificationChannel | EMAIL / SMS / PUSH / IN_APP / REALTIME |
| status | enum DeliveryChannelStatus | Default `PENDING` |
| provider | varchar nullable | Channel provider identifier (e.g. sendgrid, twilio) |
| providerMessageId | varchar nullable | External provider message id |
| attemptCount | int default 0 | Incremented by DeliveryStateService.beginProcessing |
| lastError | text nullable | Last failure message |
| scheduledAt | timestamp nullable | Mirrors notification scheduledAt |
| sentAt / deliveredAt / failedAt | timestamp nullable | Terminal timestamps |

### notification_templates

| Column | Type | Notes |
| --- | --- | --- |
| id | varchar(64) PK | Snowflake |
| type | enum NotificationType | Template category |
| channel | enum NotificationChannel | Template is channel-specific |
| locale | varchar(10) default 'en' | e.g. en, ar |
| titleTemplate | varchar | Handlebars source for title |
| bodyTemplate | text | Handlebars source for body |
| createdAt / updatedAt | timestamp | Auto-managed |

Unique constraint on `(type, channel, locale)`. Two default templates are seeded on module init for `PASSWORD_RESET_REQUESTED` on `IN_APP` in `en` and `ar`.

### notification_preferences

| Column | Type | Notes |
| --- | --- | --- |
| id | varchar(64) PK | Snowflake |
| userId | varchar | Indexed |
| type | enum NotificationType | Preference is per notification type |
| channel | enum NotificationChannel | Preference is per channel |
| enabled | boolean default true | Opt-in / opt-out flag |

Unique constraint on `(userId, type, channel)`. When a user has zero rows for a given `(userId, type)`, `PreferenceService.getEnabledChannels` returns the defaults `IN_APP + PUSH`.

### notification_outbox

| Column | Type | Notes |
| --- | --- | --- |
| id | varchar(64) PK | Snowflake |
| eventType | varchar | e.g. `NOTIFICATION_CREATED` |
| aggregateId | varchar | Notification id (dedup key with eventType) |
| payload | jsonb | Full notification payload published to NATS |
| status | enum OutboxStatus | PENDING / PUBLISHED / FAILED; indexed |
| attemptCount | int default 0 | Publish attempts |
| nextAttemptAt | timestamp nullable | Reserved for future backoff scheduling |
| publishedAt | timestamp nullable | Set on successful NATS emit |

### notification_inbox

| Column | Type | Notes |
| --- | --- | --- |
| id | varchar(64) PK | Snowflake |
| eventId | varchar | Source event id |
| eventType | varchar | Source event type |
| consumer | varchar default 'notification-service' | Consumer identity for idempotency |
| processedAt | timestamp | When the event was handled |

Unique constraint on `(eventId, consumer)`.

---

## Domain Enums

All enums are imported from `@delivery-micro/shard` (`enum.constant.ts`).

### NotificationType

| Value | Typical source event |
| --- | --- |
| DELIVERY_CREATED | Kafka `delivery.created` / RabbitMQ `delivery.created` |
| DRIVER_ASSIGNED | Kafka `delivery.driver.assigned` |
| DRIVER_ACCEPTED | Kafka `delivery.driver.accepted` |
| DELIVERY_PICKED_UP | Kafka `delivery.picked_up` |
| DELIVERY_IN_TRANSIT | Kafka `delivery.in_transit` |
| DELIVERY_COMPLETED | Kafka `delivery.completed` |
| DELIVERY_CANCELLED | Kafka `delivery.cancelled` |
| PAYMENT_COMPLETED | Kafka `payment.completed` / RabbitMQ `payment.authorized` |
| PAYMENT_FAILED | Kafka `payment.failed` / RabbitMQ `payment.failed` |
| PAYMENT_REFUNDED | Kafka `payment.refunded` / RabbitMQ `payment.refunded` |
| MEDIA_UPLOAD_COMPLETED | Kafka `media.upload.completed` |
| MEDIA_UPLOAD_FAILED | Kafka `media.upload.aborted` |
| MEDIA_SCAN_FAILED | Kafka `media.scan.failed` |
| MEDIA_PROCESSING_FAILED | Kafka `media.processing.failed` |
| MEDIA_READY | Kafka `media.ready` |
| MEDIA_DELETED | Kafka `media.deleted` |
| MEDIA_DELETE_FAILED | Kafka `media.delete.failed` |
| USER_REGISTERED | Kafka `user.created` |
| PASSWORD_RESET_REQUESTED | Kafka `user.password_reset_requested` |

### NotificationChannel

| Value | Worker queue | Delivery mechanism |
| --- | --- | --- |
| EMAIL | notification-email | `@bts-soft/notifications` email providers (SMTP / SendGrid / AWS SES) |
| SMS | notification-sms | Twilio / SMSMISR / Vonage |
| PUSH | notification-push | OneSignal / Firebase / Web Push / Pusher |
| IN_APP | notification-inapp | In-app persistence via the notifications SDK |
| REALTIME | notification-realtime | Outbox row -> NATS `notification.user.<userId>` (worker only marks delivery sent) |

### NotificationPriority

`LOW`, `NORMAL`, `HIGH`, `CRITICAL`. Mapped by `buildChannelMessage` to the numeric BullMQ priorities exposed by `@bts-soft/notifications` (`LOW / NORMAL / HIGH / CRITICAL` respectively). The event handler upgrades certain events to HIGH (DRIVER_ASSIGNED, PAYMENT_FAILED, PASSWORD_RESET_REQUESTED).

### NotificationStatus (aggregate)

`CREATED`, `QUEUED`, `PROCESSING`, `SENT`, `DELIVERED`, `FAILED`, `CANCELLED`, `EXPIRED`.

### DeliveryChannelStatus (per-channel)

`PENDING`, `QUEUED`, `PROCESSING`, `RETRYING`, `SENT`, `DELIVERED`, `FAILED`.

### OutboxStatus

`PENDING`, `PUBLISHED`, `FAILED`.

---

## Key Components

### AppModule

Root module located at `src/app.module.ts`. Wiring order:

1. `ConfigModule.forRoot` — global; env file path is `../../config/env/.env.<APP_ENV | NODE_ENV | development>` (resolved from the monorepo `config/env` directory; the DB password `POSTGRES_PASSWORD` is never hard-coded).
2. `TranslationModule` — i18n for localized messages.
3. `RedisModule` — from `@bts-soft/cache`.
4. `JwtModule.registerAsync` — global; consumes `JWT_SECRET` (fallback `default_secret`) and `JWT_EXPIRE` (fallback `1d`).
5. `TypeOrmModule.forRootAsync` — PostgreSQL; `entities` glob `__dirname + '/**/*.entity{.ts,.js}'`; `synchronize: true` for development (use migrations in production); `retryAttempts: 60`, `retryDelay: 3000` to tolerate slow DB startup in containers.
6. `BullModule.forRootAsync` — Redis connection from `REDIS_HOST` / `REDIS_PORT` / `REDIS_DB`.
7. `BtsNotificationModule` — from `@bts-soft/notifications`; provides the channel-level `NotificationService.send`.
8. `GraphQLModule.forRoot<ApolloFederationDriverConfig>` — Apollo Federation driver v2; path `/notification/graphql`; `autoSchemaFile: { federation: 2 }`; context exposes `req`, `user`, `language` (from `accept-language` header); playground enabled in dev, debug off.
9. `LoggingModule`, `MetricsModule`, `AutomationModule` — from `@delivery-micro/shard`.
10. Feature modules: `AuthModule`, `CommonModule`, `KafkaConsumerModule`, `RabbitMQNotificationModule`, `NotificationModule`, `WorkersModule`, `OutboxModule`, `GrpcModule`.

Global providers:

- `APP_FILTER` -> `GraphQLExceptionFilter` (from `@delivery-micro/shard`).
- `APP_INTERCEPTOR` -> `GraphQLResponseInterceptor` (wraps responses into the uniform `{ success, statusCode, message, data }` envelope).
- `APP_INTERCEPTOR` -> `MetricsInterceptor` (Prometheus request metrics).

`AppResolver` exposes the public `ping` GraphQL query.

### main.ts Bootstrap

1. Creates the Nest app with `StructuredLogger` and `rawBody: true`, `abortOnError: false`.
2. Enables CORS.
3. Installs a global `ValidationPipe` with `transform`, `whitelist`, `stopAtFirstError`, and an `I18nValidationException` factory (localized validation errors).
4. Connects the inbound gRPC microservice: transport `GRPC`, package `notification`, proto path `join(process.cwd(), '../../protos/notification.proto')`, bind URL `0.0.0.0:<PORT_GRPC>` (default `50053`).
5. Starts HTTP listen on `PORT_NOTIFICATION` (default `4004`), then starts all microservices (gRPC) in the background so HTTP comes up even if gRPC transport startup lags.
6. Registers process-level `uncaughtException` / `unhandledRejection` handlers that log and continue (keeps the container alive while Redis/DB/NATS/Kafka finish starting).
7. On bootstrap failure, retries after 10 seconds.

### CommonModule

`@Global` module that:

- Registers TypeORM repositories for all six entities via `TypeOrmModule.forFeature`.
- Imports `RedisModule`.
- Provides and exports the token `SHARED_REDIS_SERVICE` bound to `RedisService` (consumed by the shard's Redis rate limiter guard).
- Registers `HealthController` and `MetricsController`.
- Exports `TypeOrmModule`, `RedisModule`, and `SHARED_REDIS_SERVICE` so any module can inject repositories without re-importing.

### TranslationModule

`@Global` wrapper around `I18nModule.forRoot`:

- Fallback language: `en`.
- Locale files loaded from `src/common/translation/locales/` with `watch: true`.
- Resolvers: `HeaderResolver(['x-lang'])` then `AcceptLanguageResolver()`.
- Files: `en/notification.json`, `en/user.json`, `ar/notification.json`, `ar/user.json`.

Used by the resolver for success messages (`notification.LIST_RETRIEVED`, `notification.MARKED_READ`, etc.) and by `NotificationService.markAsRead` for the not-found error (`notification.NOT_FOUND`).

### AuthModule

`@Global` module providing real auth (no mock users):

- `GrpcClientsModule` registers the `GRPC_USER_SERVICE` gRPC client: package `user` (`USER_PACKAGE_NAME`), proto `../../protos/user.proto`, URL `USER_GRPC_URL` (default `user-srv:50051`).
- `UserLookupService` implements the `USER_SERVICE` contract (`findById`) required by `RoleGuard`:
  1. On module init, binds the `getUser` method from the gRPC client (`USER_SERVICE_NAME`).
  2. Calls `GetUser({ id })` over gRPC.
  3. Normalizes the legacy `customer` role to `Role.USER` (roles consolidated to `ADMIN | USER | DRIVER`).
  4. Returns an `IUser` with `{ id, email, role, firstName, lastName, isActive, isVerified, createdAt, updatedAt }`.
  5. Throws `NotFoundException('user.USER_NOT_FOUND')` on gRPC failure or empty response.
- `AuthModule` provides `UserLookupService`, `RoleGuard`, and the `USER_SERVICE` token (`useExisting: UserLookupService`), and exports all three.

Every `@Auth([Permission...])` decorator on a resolver method triggers `RoleGuard`, which verifies the JWT (`process.env.JWT_SECRET`), loads the user through `USER_SERVICE`, and checks the permission against `rolePermissionsMap` from `@delivery-micro/shard`.

### KafkaModule

`KafkaConsumer` extends `BaseKafkaConsumer` from `@delivery-micro/shard`:

- Subscribes to every topic in `DeliveryKafkaTopics`, `PaymentKafkaTopics`, `MediaKafkaTopics`, and `UserKafkaTopics` (see the enum tables above).
- Consumer group: `KAFKA_GROUP_ID` (default `notification-service`); the Kafka client is obtained from the injected `KafkaService`.
- Per message:
  1. Parses the JSON payload as `KafkaEventPayload` (`{ eventId, eventType, payload, data, ... }`).
  2. Resolves `eventId` from `payload.eventId || payload.id`; skips the message (with a warning) if missing.
  3. Resolves `eventType` from `payload.eventType || topic`.
  4. Inbox idempotency check on `(eventId, 'notification-service')`; skips duplicates.
  5. Looks up the handler through `EventHandlerFactory.getHandler(eventType)`; if found, awaits `handler.handle(payload)`; otherwise logs a debug line.
  6. Writes the inbox row (`eventId`, `eventType`, `consumer: 'notification-service'`, `processedAt`).
  7. Errors are caught and logged per message (the consumer does not crash on a single bad event).

`EventHandlerFactory` (in `event-handlers/event-handler.factory.ts`) is a simple registry: `registerHandler(eventType, handler)` and `getHandler(eventType)`. Handlers implement `IEventHandler` with `handle(payload: KafkaEventPayload): Promise<void>`.

`NotificationEventHandler` implements `IEventHandler` and `OnModuleInit`:

- On init it registers itself for every key in `EVENT_MAPPING` (18 mappings covering delivery, payment, media, and user topics).
- `handle` resolves the mapping, extracts the target user via `extractUserId` (checks `userId`/`user_id` at the top level, then `payload`/`data` envelope for `userId`, `user_id`, `customerId`, `customer_id`, `driverId`, `driver_id`), and calls `NotificationService.createAndDispatch` with `type`, optional `priority`, optional `requiredChannels`, and merged `data` (envelope payload + data + `eventId`).
- Special case: `PASSWORD_RESET_REQUESTED` passes `requiredChannels: [IN_APP]` so an in-app audit record is always created regardless of user preferences.

### RabbitMQNotificationModule

Located at `src/modules/rabbitmq/`. Registers `RabbitMQModule.registerAsync` with `url` from `RABBITMQ_URL` and `serviceName: 'notification-service'`.

Three consumers extend `BaseRabbitMQConsumer` from `@delivery-micro/shard`:

| Consumer | Queue | Exchange | Routing keys |
| --- | --- | --- | --- |
| EmailRabbitMQConsumer | `notifications.email.queue` | `delivery.notifications.fanout` | `['']` (fanout) |
| SmsRabbitMQConsumer | `notifications.sms.queue` | `delivery.notifications.fanout` | `['']` |
| PushRabbitMQConsumer | `notifications.push.queue` | `delivery.notifications.fanout` | `['']` |

Each consumer:

- Sets `prefetch` to 10.
- Converts the `RabbitMQEventEnvelope` into a handler payload (spreads `envelope.payload`, injects `eventId` and `eventType`).
- Checks idempotency through the inbox table with consumer name `notification-service-rabbitmq` (override of `isDuplicate`).
- Dispatches to the same `EventHandlerFactory` used by the Kafka consumer (so the same `EVENT_MAPPING` and `createAndDispatch` pipeline apply).
- Records the inbox row after handling.

`RabbitMQNotificationMapper` converts an envelope into a `RabbitMQNotificationView` `{ type, title, body, data }` with a static title map for known event types (`delivery.created` -> "Order Received", `payment.failed` -> "Payment Failed", etc.) and a generic fallback `Update: <eventType>`. It always injects `deliveryId` and `traceId` into `data`.

### NotificationModule

Registers all five entity repositories, all seven BullMQ queues (`notification-email`, `notification-sms`, `notification-push`, `notification-inapp`, `notification-realtime`, `notification-scheduled`, `notification-retry`), and exports the core services.

#### NotificationService (`notification.service.ts`)

Domain core. Public API:

| Method | Signature | Behavior |
| --- | --- | --- |
| createAndDispatch | `(params: CreateNotificationParams) => Promise<string \| null>` | Full pipeline: resolve channels -> render template -> transactional insert of notification + deliveries -> dispatch or schedule. Returns the notification id, or `null` when no channels are enabled or on transaction error (with rollback). |
| findAllForUser | `(userId, page, limit) => Promise<PaginatedNotifications>` | Paginated list (limit clamped 1..100, page min 1), deliveries eager-loaded, newest first. |
| findByIdForUser | `(id, userId) => Promise<Notification \| null>` | Single notification scoped to the owner, deliveries included. |
| unreadCountForUser | `(userId) => Promise<number>` | Count where `readAt IS NULL`. |
| markAsRead | `(id, userId) => Promise<Notification>` | Sets `readAt`; throws i18n `NotFoundException('notification.NOT_FOUND')` if missing. |
| markAllAsRead | `(userId) => Promise<number>` | Bulk update of unread rows; returns affected count. |
| delete | `(id, userId) => Promise<boolean>` | Hard delete scoped to the owner. |

`CreateNotificationParams`: `{ userId, type, title?, body?, data?, priority?, scheduledAt?, requiredChannels? }`. When `title`/`body` are supplied they override the rendered template output. Locale is currently fixed to `'en'` for rendering (extensible).

#### NotificationDispatcherService (`notification-dispatcher.service.ts`)

| Method | Behavior |
| --- | --- |
| dispatch(notification, deliveries) | For each delivery, enqueues a `send` job on the channel-specific queue with `jobId = <notificationId>-<channel>` (BullMQ dedup). Errors per queue are caught and logged. If any delivery is REALTIME, calls `createRealtimeOutbox`. |
| schedule(notification, scheduledAt) | Enqueues a `dispatch` job on `notification-scheduled` with `delay = max(0, scheduledAt - now)` and `jobId = scheduled-<notificationId>`. |
| requeue(delivery) | Enqueues a `send` job with `jobId = retry:<deliveryId>` (used by the retry sweep). |
| createRealtimeOutbox (private) | Deduped insert of a `NOTIFICATION_CREATED` outbox row with the full notification payload (`userId`, `notificationId`, `type`, `title`, `body`, `data`, `priority`). |

#### TemplateService (`template/template.service.ts`)

- On module init: seeds two default templates (`PASSWORD_RESET_REQUESTED` / IN_APP / en and ar) if missing, then loads all templates into an in-memory `Map<string, CompiledTemplate>` keyed by `<type>:<channel>:<locale>`.
- `render(type, channel, locale, data)`:
  1. `resolveTemplate`: in-memory compiled map -> Redis raw cache (`template:<type>:<channel>:<locale>`, TTL 3600s) -> DB entity (then populates both caches).
  2. If the requested locale misses and locale is not `en`, retries with `en`.
  3. If still missing, returns the generic fallback `{ title: 'Notification: <type>', body: JSON.stringify(data) }`.
  4. Compiles and executes Handlebars; on render error, logs and returns the same generic fallback.
- Handlebars context is the `data` record (e.g. `{{firstName}}` in the seeded password-reset templates).

#### PreferenceService (`preference/preference.service.ts`)

| Method | Behavior |
| --- | --- |
| getEnabledChannels(userId, type) | Loads preference rows for `(userId, type)`. If none exist, returns the defaults `[IN_APP, PUSH]`. Otherwise returns only the channels with `enabled = true`. |
| upsertPreferences(userId, type, channels) | For each `{ channel, enabled }`: find existing row or create, then save; returns the saved rows. |
| findForUser(userId, type?) | Lists preference rows ordered by `(type ASC, channel ASC)`; optional type filter. |

#### NotificationResolver (`notification.resolver.ts`)

Thin GraphQL layer delegating to the services. Every operation is decorated with `@RedisRateLimit` (fixed-window counter) and `@Auth([Permission...])`. See the GraphQL API section for the full operation table.

### WorkersModule

Imports the two entity repositories, `BtsNotificationModule`, and `NotificationModule`. Provides seven workers plus `DeliveryStateService`.

#### Shared worker pattern (email / sms / push / inapp)

Each channel worker (`EmailWorker`, `SmsWorker`, `PushWorker`, `InAppWorker`) follows the same `process(job)` shape:

1. Destructures `{ notificationId, deliveryId }` from `job.data`.
2. Loads the delivery row and the notification row; returns silently if either is missing.
3. `DeliveryStateService.beginProcessing(delivery)` — sets `PROCESSING` and increments `attemptCount`.
4. Builds the package message via `buildChannelMessage(notification, idempotencyKey)` where the idempotency key is `<channel>:<notificationId>:<deliveryId>`.
5. Calls `this.notificationService.send(ChannelType.<CHANNEL>, message)` from `@bts-soft/notifications`.
6. On success: `DeliveryStateService.complete(delivery, notification)`.
7. On failure: `DeliveryStateService.fail(delivery, notification, error)` then rethrows so BullMQ applies its own retry policy.

#### RealtimeWorker

Marks its delivery as sent (via `DeliveryStateService`). The actual client push happens through the outbox/NATS pipeline, not through the notifications SDK.

#### ScheduledWorker

Consumes `dispatch` jobs from `notification-scheduled`. Loads the notification, loads its PENDING deliveries, and calls `dispatcherService.dispatch(notification, deliveries)` so delayed notifications are fanned out at the scheduled time.

#### RetryWorker

- On module init, upserts a BullMQ job scheduler (`notification-sweep`, every 60 seconds) on the `notification-retry` queue.
- The `process` method only handles the `sweep` job name and runs two sweeps:
  1. `expireDueNotifications`: finds notifications in `CREATED / QUEUED / PROCESSING` with `expiresAt < now` and marks them (and their non-terminal deliveries) as expired via `DeliveryStateService.expire`.
  2. `requeueFailedDeliveries`: finds deliveries in `FAILED` with `attemptCount < 5` and `failedAt` older than 60 seconds; flips them to `PENDING` and calls `dispatcherService.requeue(delivery)`.

#### DeliveryStateService

Central state machine for delivery and aggregate status:

| Method | Effect |
| --- | --- |
| beginProcessing(delivery) | `PROCESSING`, `attemptCount += 1`, save. |
| complete(delivery, notification, { delivered? }) | `SENT` + `sentAt` (+ `deliveredAt` when `delivered: true`), then transition the aggregate. |
| fail(delivery, notification, error) | `FAILED` + `lastError` + `failedAt`, then transition the aggregate. |
| expire(notification) | Aggregate -> `EXPIRED`; non-terminal deliveries (PENDING / PROCESSING / RETRYING) -> `FAILED` with `lastError = 'Expired'`. |

`transitionNotification` (private) recomputes the aggregate status from all its deliveries:

- No-op if the notification is already terminal (SENT / DELIVERED / FAILED / EXPIRED / CANCELLED) or has no deliveries.
- If any delivery is still inflight (PENDING / PROCESSING / QUEUED / RETRYING): bump `CREATED` to `QUEUED` and stop.
- Otherwise: `FAILED` if any delivery failed, else the terminal status passed by the caller.

#### channel-message.helper.ts

`buildChannelMessage(notification, idempotencyKey)` produces a `NotificationMessage` for `@bts-soft/notifications`:

- `recipientId` = `notification.userId`
- `body` / `title` / `subject` = notification title and body (subject mirrors title for email providers).
- `priority` = mapped from the shard enum to the package enum (`LOW / NORMAL / HIGH / CRITICAL`).
- `idempotencyKey` = caller-supplied key.
- `context` = `notification.data ?? undefined` (passed through to provider templates).

### OutboxModule

`OutboxWorkerService` (`OnModuleInit` + `OnModuleDestroy`):

- Creates a NATS `ClientProxy` (transport `NATS`, server `NATS_URL` default `nats://nats-srv:4222`, queue `notification-service`) in the constructor and connects on module init.
- Starts a `setInterval` every 5000 ms calling `processOutbox`.
- `processOutbox`:
  1. Loads up to 50 PENDING outbox rows ordered by `createdAt ASC`.
  2. For each row: builds subject `notification.user.<payload.userId>` and `natsClient.emit(subject, payload)`; on success sets `PUBLISHED` + `publishedAt` and saves.
  3. On per-row failure: increments `attemptCount`; after more than 5 attempts sets `FAILED` and triggers a CRITICAL alert via `AlertService.triggerAlert`.
  4. Outer errors (e.g. DB down) are logged and trigger a WARNING alert.
- On destroy: clears the interval and closes the NATS client.

### GrpcModule

`GrpcController` implements the inbound RPC defined in `protos/notification.proto`:

- RPC: `NotificationService.SendNotification` (decorated with `@GrpcMethod('NotificationService', 'SendNotification')`).
- Request fields (proto uses snake_case; NestJS/proto-loader maps them to camelCase in the handler): `user_id`, `type`, `title`, `body`, `data` (JSON string), `priority`.
- The controller parses `data` defensively (`JSON.parse` inside try/catch, empty object on failure), casts `type`/`priority` to the shard enums, and calls `createAndDispatch`.
- Response: `{ notification_id, success }` (camelCase `notificationId` / `success` in the handler).

Proto contract:

```protobuf
syntax = "proto3";
package notification;

service NotificationService {
  rpc SendNotification(SendNotificationRequest) returns (SendNotificationResponse);
}

message SendNotificationRequest {
  string user_id = 1;
  string type = 2;
  string title = 3;
  string body = 4;
  string data = 5;    // JSON string
  string priority = 6;
}

message SendNotificationResponse {
  string notification_id = 1;
  bool success = 2;
}
```

---

## GraphQL API

Endpoint: `POST /notification/graphql` (Apollo Federation v2 subgraph). Playground is enabled.

All operations require a `Bearer` JWT in the `Authorization` header, pass through `RoleGuard` (JWT verify -> gRPC user lookup -> permission check), and are rate limited with a Redis fixed-window counter.

| Operation | Kind | Arguments | Permission | Rate limit (per 60s) | Returns |
| --- | --- | --- | --- | --- | --- |
| ping | Query | none | public (no @Auth) | none | string pong |
| myNotifications | Query | `page: Int = 1`, `limit: Int = 20` | READ_NOTIFICATION | 100 | PaginatedNotificationResponse `{ items, totalCount }` |
| notification | Query | `id: ID!` | READ_NOTIFICATION | 100 | NotificationResponse (owner-scoped) |
| unreadNotificationCount | Query | none | READ_NOTIFICATION | 100 | IntResponse |
| markNotificationAsRead | Mutation | `id: ID!` | UPDATE_NOTIFICATION | 60 | NotificationResponse |
| markAllNotificationsAsRead | Mutation | none | UPDATE_NOTIFICATION | 60 | BooleanResponse |
| deleteNotification | Mutation | `id: ID!` | UPDATE_NOTIFICATION | 50 | BooleanResponse |
| myNotificationPreferences | Query | `type: String` (optional) | READ_NOTIFICATION | 100 | NotificationPreferenceResponse `{ items }` |
| updateNotificationPreferences | Mutation | `preferences: [NotificationPreferenceInput!]!` | UPDATE_NOTIFICATION | 60 | NotificationPreferenceResponse `{ items }` |

Permission-to-role mapping (from `rolePermissionsMap` in `@delivery-micro/shard`):

| Permission | ADMIN | USER | DRIVER |
| --- | --- | --- | --- |
| READ_NOTIFICATION | yes | yes | yes |
| UPDATE_NOTIFICATION | yes | yes | yes |
| DELETE_NOTIFICATION | yes | no | no (defined but not used on any resolver method; deleteNotification currently requires UPDATE_NOTIFICATION) |
| MANAGE_NOTIFICATION_PREFERENCES | yes | yes | yes |

`NotificationPreferenceInput` (GraphQL input): `{ type: NotificationType, channels: [{ channel: NotificationChannel, enabled: Boolean }] }`.

Every resolver returns the uniform envelope produced by `GraphQLResponseInterceptor`:

```json
{
  "success": true,
  "statusCode": 200,
  "message": "Notifications retrieved successfully",
  "data": {}
}
```

Preference operations return the saved/listed rows in an `items` field alongside `success` / `statusCode` / `message` (see `NotificationPreferenceResponse` in `dtos/preference.dto.ts`).

Errors are formatted by the global `GraphQLExceptionFilter`; validation errors surface as `I18nValidationException` with localized messages from the locale files.

---

## gRPC API

Inbound server:

- Package: `notification`
- Service: `NotificationService`
- Method: `SendNotification(SendNotificationRequest) returns (SendNotificationResponse)`
- Bind URL: `0.0.0.0:<PORT_GRPC>` (default `50053`)
- Proto file: `protos/notification.proto` at the monorepo root

Outbound client (auth):

- Package: `user`
- Service: `UserService` (`USER_SERVICE_NAME`)
- Method: `GetUser({ id })`
- Default URL: `user-srv:50051` (`USER_GRPC_URL`)
- Proto file: `protos/user.proto`

---

## HTTP Endpoints

All routes are prefixed with `/notification` (controller prefix).

| Method | Path | Behavior |
| --- | --- | --- |
| GET | `/notification/health` | Always 200 `{ status: 'UP', service, timestamp, system }` with system stats from `HealthService`. |
| GET | `/notification/ready` | Runs `HealthService.checkDatabase(dataSource)` and `checkRedis(redisService)`. Returns 200 with `status: 'UP'` when both are UP, otherwise 503 with `status: 'DOWN'` and the failing check details. |
| GET | `/notification/metrics` | Streams Prometheus metrics from `MetricsService.getMetrics()` with the correct `Content-Type`. |

These endpoints are unauthenticated (used by orchestrators and load balancers).

---

## Caching Strategy

1. **Template in-memory cache** — compiled Handlebars functions in a `Map` keyed by `<type>:<channel>:<locale>`. Hot path; avoids recompilation.
2. **Template Redis cache** — raw template rows under key `template:<type>:<channel>:<locale>` with a 1-hour TTL. Cache-aside: populated on DB read, consulted before hitting the DB.
3. **User lookup** — performed live over gRPC on each authenticated request (the User Service and/or intermediate layers may apply their own caching; this service does not add a local Redis cache for users).
4. **Rate limiting** — `RedisRateLimit` decorator backed by `SHARED_REDIS_SERVICE` (Redis) with a fixed-window counter algorithm.
5. **BullMQ** — Redis is also the queue backend (job dedup via deterministic `jobId`s).

---

## i18n

- Module: `TranslationModule` (global), wrapper around `nestjs-i18n`.
- Fallback language: `en`.
- Header-based resolution: `x-lang` first, then `Accept-Language`.
- Locale files: `src/common/translation/locales/{en,ar}/{notification.json,user.json}`.
- Key namespaces used by this service: `notification.*` for success/error messages (e.g. `LIST_RETRIEVED`, `GET_RETRIEVED`, `UNREAD_COUNT`, `MARKED_READ`, `ALL_MARKED_READ`, `DELETED`, `PREFERENCES_RETRIEVED`, `PREFERENCES_UPDATED`, `NOT_FOUND`), `user.*` for auth errors (e.g. `USER_NOT_FOUND`).

---

## Configuration

Environment variables are loaded from the monorepo `config/env/.env.<APP_ENV|NODE_ENV|development>` file and/or process environment (Docker Compose injects additional provider secrets). Important variables:

### Core

| Variable | Default | Used for |
| --- | --- | --- |
| APP_ENV / NODE_ENV | development | Selects the env file suffix |
| PORT_NOTIFICATION | 4004 | HTTP + GraphQL port |
| PORT_GRPC | 50053 | gRPC server port |
| JWT_SECRET | default_secret | JWT signing/verification (also read directly by RoleGuard via `process.env.JWT_SECRET`) |
| JWT_EXPIRE | 1d | JWT expiry |
| LOG_LEVEL | info | Structured logger level |

### Database

| Variable | Default | Used for |
| --- | --- | --- |
| DB_HOST | localhost | Postgres host |
| DB_PORT | 5432 | Postgres port |
| DB_USERNAME | postgres | Postgres user |
| DB_NAME / NOTIFICATION_DB_NAME | delivery_notification_db | Postgres database |
| POSTGRES_PASSWORD | (required) | Postgres password (never hard-coded) |

### Redis

| Variable | Default | Used for |
| --- | --- | --- |
| REDIS_HOST | localhost | Redis host (BullMQ + cache + rate limit) |
| REDIS_PORT | 6379 | Redis port |
| REDIS_DB | 0 | Redis database index |

### Kafka

| Variable | Default | Used for |
| --- | --- | --- |
| KAFKA_BROKERS | kafka-srv:9092 | Kafka broker list |
| KAFKA_GROUP_ID | notification-service | Consumer group |

### NATS

| Variable | Default | Used for |
| --- | --- | --- |
| NATS_URL | nats://nats-srv:4222 | Outbox worker NATS server URL |

### RabbitMQ

| Variable | Default | Used for |
| --- | --- | --- |
| RABBITMQ_URL | (from env file) | RabbitMQ connection URL for the shard RabbitMQ module |
| RABBITMQ_USER / RABBITMQ_PASSWORD / RABBITMQ_VHOST | (from env file) | Credentials (used by compose / infra) |

### gRPC

| Variable | Default | Used for |
| --- | --- | --- |
| USER_GRPC_URL | user-srv:50051 | User Service gRPC address for auth lookups |

### Channel provider credentials (consumed by `@bts-soft/notifications`)

Email: `EMAIL_SERVICE`, `EMAIL_HOST`, `EMAIL_PORT`, `EMAIL_SECURE`, `EMAIL_USER`, `EMAIL_PASS`, `EMAIL_SENDER`, `EMAIL_PROVIDER`, `SENDGRID_API_KEY`, `AWS_SES_ACCESS_KEY_ID`, `AWS_SES_SECRET_ACCESS_KEY`, `AWS_SES_REGION`.

SMS: `SMS_PROVIDER`, `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_FROM_NUMBER`, `TWILIO_WHATSAPP_NUMBER`, `TWILIO_SMS_NUMBER`, `SMSMISR_USERNAME`, `SMSMISR_PASSWORD`, `SMSMISR_SENDER`, `SMSMISR_ENVIRONMENT`, `VONAGE_API_KEY`, `VONAGE_API_SECRET`, `VONAGE_SENDER`.

Push: `ONESIGNAL_APP_ID`, `ONESIGNAL_REST_API_KEY`, `WEB_PUSH_PUBLIC_KEY`, `WEB_PUSH_PRIVATE_KEY`, `WEB_PUSH_SUBJECT`, `FIREBASE_PROJECT_ID` (`PROJECT_ID`), `FIREBASE_CLIENT_EMAIL` (`CLIENT_EMAIL`), `FIREBASE_PRIVATE_KEY` (`PRIVATE_KEY`), `PUSHER_APP_ID`, `PUSHER_KEY`, `PUSHER_SECRET`, `PUSHER_CLUSTER`, `IN_APP_PROVIDER`.

Chat/alerts (available via the notifications SDK / shard): `TELEGRAM_BOT_TOKEN`, `ENABLE_TELEGRAM_BOT`, `DISCORD_WEBHOOK_URL`, `SLACK_WEBHOOK_URL`, `SLACK_BOT_TOKEN`, `SLACK_DEFAULT_CHANNEL`.

Observability: `ALERT_WEBHOOK_URL`, `ALERTMANAGER_URL`.

---

## Docker Compose

`docker-compose.yml` (service-local) provisions:

| Service | Image | Ports | Notes |
| --- | --- | --- | --- |
| notification-db-srv | postgres:15 | 5434:5432 | DB from env file; `POSTGRES_DB` defaults to `delivery_notification_db` |
| redis-srv | redis:alpine | 6379:6379 | BullMQ + cache + rate limit |
| nats-srv | nats-streaming:0.17.0 | 4222, 8222 | Streaming ID `ticketing`; heartbeat flags enabled |
| kafka-srv | bitnami/kafka:3.6 | 9092 | Advertised listener `PLAINTEXT://kafka-srv:9092`; depends on zookeeper |
| zookeeper-srv | bitnami/zookeeper:3.9 | 2181 | Kafka dependency; anonymous login allowed |
| notification-service | built from `services/notification-service/Dockerfile` | 4004, 50053 | Mounts `src`, `packages/ts`, `protos`; injects all env vars; depends on db, redis, nats, kafka |

Typical local startup:

```bash
cd services/notification-service
docker compose up --build
```

Or from the monorepo tooling (`Makefile` / `Taskfile.yml` at the root) if task runners are configured.

---

## Makefile

The service ships with a comprehensive, Windows-compatible GNU Makefile at `services/notification-service/Makefile` (uses `SHELL := cmd.exe`; also works under POSIX shells). It wraps every operational task so developers and CI do not need to remember raw npm/docker commands.

Run `make help` from `services/notification-service` to list all targets.

### Quick reference

| Target | What it does |
| --- | --- |
| `make help` | Print the full target catalog (default target). |
| `make check-deps` | Verify `node`, `npm`, `docker` are on PATH. |
| `make install` | `npm install --legacy-peer-deps`. |
| `make install-fresh` | Delete `node_modules` + lockfile, then reinstall. |
| `make start` / `start-dev` / `start-debug` / `start-prod` | Run the service once / watch / debug+watch / compiled `dist/main`. |
| `make build` | `nest build` to `dist/`. |
| `make rebuild` | `clean-dist` then `build`. |
| `make typecheck` | `tsc --noEmit -p tsconfig.build.json` (production sources only; specs and test-utils excluded). |
| `make test` | Unit tests without coverage. |
| `make test-watch` | Unit tests in watch mode. |
| `make test-cov` | Unit tests with coverage thresholds enforced. |
| `make test-e2e` | E2E tests via `test/jest-e2e.json`. |
| `make test-all` | `test-cov` then `test-e2e` (full local suite). |
| `make test-debug` | Unit tests under `node --inspect-brk`. |
| `make lint` | ESLint with `--fix`. |
| `make format` | Prettier write on `src` and `test`. |
| `make format-check` | Prettier check (no write; fails if unformatted). |
| `make verify` | `build` + `test-cov` + `test-e2e` (full quality gate). |
| `make ci` | `check-deps` + `install` + `verify` (CI pipeline). |
| `make up` | `docker compose up -d` (infra + service) using `ENV` env file. |
| `make up-build` | Compose up with `--build`. |
| `make down` / `down-volumes` | Compose down (optionally remove volumes). |
| `make restart` | Compose restart. |
| `make logs` | Tail `notification-service` container logs. |
| `make ps` | Show compose service status. |
| `make docker-build` | Build only the service image (`notification-service:local`). |
| `make docker-shell` | `docker exec -it notification-service sh`. |
| `make docker-prune` | Prune dangling containers/images. |
| `make infra-up` / `infra-down` / `infra-logs` | Manage infrastructure containers only (db, redis, nats, kafka, zookeeper). |
| `make db-up` / `redis-up` / `nats-up` / `kafka-up` / `zookeeper-up` | Start a single infra service. |
| `make db-shell` | `psql` shell inside the Postgres container. |
| `make redis-cli` | `redis-cli` shell inside the Redis container. |
| `make health` / `ready` / `metrics` | `curl` the HTTP health, readiness, and metrics endpoints. |
| `make curl-health` / `curl-ready` / `curl-metrics` | Aliases for the above. |
| `make proto-check` | Verify `protos/notification.proto` and `protos/user.proto` exist. |
| `make env-check` | Verify `config/env/.env.$(ENV)` exists. |
| `make ports` | Print all ports used by the service and its infra. |
| `make clean` | Remove `dist/` and `coverage/`. |
| `make clean-dist` / `clean-coverage` / `clean-node-modules` | Remove individual artifacts. |
| `make clean-all` | Remove `dist/`, `coverage/`, and `node_modules/`. |

### Variables

| Variable | Default | Description |
| --- | --- | --- |
| `ENV` | `development` | Selects `../../config/env/.env.$(ENV)` for compose and env checks. Override with `make up ENV=production`. |
| `APP_PORT` | `4004` | HTTP/GraphQL port used in health URLs. |
| `GRPC_PORT` | `50053` | gRPC port shown by `make ports`. |

### Typical workflows

First-time setup and full local verification:

```bash
make check-deps
make install
make verify          # build + unit coverage + e2e
```

Day-to-day development:

```bash
make start-dev       # watch mode
make test-cov        # in another terminal
```

Bring up the full stack (Postgres, Redis, NATS, Kafka, Zookeeper, service):

```bash
make up ENV=development
make health          # once the service is listening
make logs            # follow logs
make down            # tear down
```

CI pipeline (install + full gate):

```bash
make ci
```

### Windows compatibility notes

- The Makefile sets `SHELL := cmd.exe` so recipes run under the Windows command interpreter (verified with ezwinports `make` on Windows).
- Path variables use backslashes (`..\..\config\env\.env.$(ENV)`, `..\..\protos\notification.proto`).
- Cleanup recipes use `if exist ... rmdir /s /q ...` instead of POSIX `rm -rf`.
- `curl` targets avoid `|` and `%{...}` format strings because `cmd.exe` interprets `|` as a pipe and mishandles lone `%` sequences; they use `curl -sf --max-time 5` and print a friendly message when the service is not running.
- `typecheck` intentionally targets `tsconfig.build.json` (excludes `*.spec.ts`, `test/`, `test-utils/`); the full `tsconfig.json` type surface of specs is intentionally loose (ts-jest runs with `diagnostics: false`).

---

## Scripts

Run from `services/notification-service`:

| Script | Command | Purpose |
| --- | --- | --- |
| build | `npm run build` | `nest build` (tsc) to `dist/` |
| start | `npm start` | One-shot start |
| start:dev | `npm run start:dev` | Watch mode |
| start:debug | `npm run start:debug` | Debug + watch |
| start:prod | `npm run start:prod` | Run compiled `dist/main` |
| lint | `npm run lint` | ESLint with `--fix` (note: `eslint.config.mjs` currently exports an empty config; fix the config before relying on lint) |
| format | `npm run format` | Prettier write on `src` and `test` |
| test | `npm test` | Jest unit tests (no coverage) |
| test:watch | `npm run test:watch` | Jest watch mode |
| test:cov | `npm run test:cov` | Jest with coverage thresholds |
| test:debug | `npm run test:debug` | Jest under `node --inspect-brk` |
| test:e2e | `npm run test:e2e` | Jest with `test/jest-e2e.json` |

Install (first run):

```bash
npm install --legacy-peer-deps
```

`--legacy-peer-deps` is required due to peer range conflicts among `graphql` / `@apollo/*` packages; `graphql` is pinned to `16.10.0` via `overrides` in `package.json`.

---

## Testing

### Unit tests

- Configured inline in `package.json` under the `jest` key.
- `rootDir: src`, `testRegex: .*\.spec\.ts$`, `testEnvironment: node`.
- Transform: `ts-jest` with `diagnostics: false` (type errors do not fail the suite; the build step is the type gate).
- `collectCoverageFrom` includes all TS/JS under `src` except `*.module.ts`, `*.entity.ts`, `*.dto.ts`, `*.input.ts`, `main.ts`, `test-utils/**`, and `node_modules/**`.
- Coverage thresholds (global): branches 85, functions 85, lines 90, statements 90.
- Coverage output directory: `../coverage` (i.e. `services/notification-service/coverage`).

Current results (verified): 27 suites / 210 tests, all passing; statements 98.02%, branches 85.59%, functions 86.66%, lines 97.83%.

Spec files cover:

- Helpers: `channel-message.helper.spec.ts`
- Core services: `notification.service.spec.ts`, `notification-dispatcher.service.spec.ts`, `template.service.spec.ts`, `preference.service.spec.ts`
- Workers: `email/sms/push/inapp/realtime.worker.spec.ts`, `retry.worker.spec.ts`, `scheduled.worker.spec.ts`, `delivery-state.service.spec.ts`
- Ingestion: `outbox-worker.service.spec.ts` (mocks `@nestjs/microservices` ClientProxyFactory + fake timers), `kafka.consumer.spec.ts`, `event-handler.factory.spec.ts`, `notification-event-handler.service.spec.ts`
- RabbitMQ: `rabbitmq-notification.mapper.spec.ts`, `email/sms/push.consumer.spec.ts`
- Presentation: `notification.resolver.spec.ts`, `grpc.controller.spec.ts`, `app.resolver.spec.ts`, `health.controller.spec.ts`, `metrics.controller.spec.ts`
- Auth: `user-lookup.service.spec.ts`

Shared mocks live in `src/test-utils/`:

- `mock-repository.factory.ts` — TypeORM repository mocks (find, findOne, save, create, count, update, delete, findAndCount).
- `mock-query-runner.factory.ts` — QueryRunner + transaction mocks (connect, startTransaction, commitTransaction, rollbackTransaction, release, manager).
- `mock-bullmq-queue.factory.ts` — BullMQ Queue mocks (add, upsertJobScheduler).

### E2E tests

- Config: `test/jest-e2e.json` (`rootDir: .`, `testRegex: .e2e-spec.ts$`, ts-jest with `diagnostics: false`).
- Current results (verified): 3 suites / 19 tests, all passing.

| File | Scope |
| --- | --- |
| `test/app.e2e-spec.ts` | Real Nest app via supertest: `/notification/health`, `/notification/ready`, `/notification/metrics`. |
| `test/notification-flow.e2e-spec.ts` | GraphQL over HTTP with real Apollo Federation driver, real `ValidationPipe`, real `RoleGuard` (JWT signed with a test secret; Redis falls back to the in-memory store when `SHARED_REDIS_SERVICE` is not a real Redis client). Covers `ping`, `myNotifications`, `notification`, `unreadNotificationCount`, `markNotificationAsRead`, `markAllNotificationsAsRead`, `deleteNotification`, preferences queries/mutations, and the full `createAndDispatch` -> worker happy path with mocked repositories/queues. |
| `test/grpc.e2e-spec.ts` | Real gRPC server (`NestFactory.createMicroservice` with Transport.GRPC and the real `protos/notification.proto`) plus a real `@grpc/grpc-js` client. Covers valid JSON payloads, invalid JSON `data`, missing fields, and response shape (`notificationId`, `success`). |

Infrastructure (Postgres, Redis, BullMQ, NATS, Kafka, RabbitMQ) is mocked in unit tests and replaced with in-memory/test doubles in e2e tests so the suite runs without Docker. For full end-to-end verification against real infrastructure, run the service via `docker compose up` and exercise the endpoints manually or with an external integration suite.

---

## Dependencies

### Internal / shared packages

| Package | Role in this service |
| --- | --- |
| `@delivery-micro/shard` | Enums (NotificationType, NotificationChannel, NotificationPriority, NotificationStatus, DeliveryChannelStatus, Role, Permission), `RoleGuard`, `@Auth` / `@RedisRateLimit` / `@CurrentUser` decorators, `rolePermissionsMap`, `BaseKafkaConsumer`, `KafkaService`, topic enums, `BaseRabbitMQConsumer`, `RabbitMQService`, RabbitMQ constants, `NatsService`-related enums (`NotificationNatsSubjects`), `HealthService`, `MetricsService`, `MetricsModule`, `MetricsInterceptor`, `LoggingModule` / `StructuredLogger`, `AutomationModule` / `AlertService`, `GraphQLExceptionFilter`, `GraphQLResponseInterceptor`, `IntResponse` / `BooleanResponse`, `IUser`, `USER_SERVICE_NAME`, `USER_PACKAGE_NAME`, `GetUserResponse`. |
| `@bts-soft/notifications` | Channel-level delivery: `NotificationService.send(ChannelType, NotificationMessage)`, `ChannelType`, `NotificationMessage`, `NotificationPriority` (package enum). Provider configuration is env-driven. |
| `@bts-soft/cache` | `RedisModule` + `RedisService` (template cache, `SHARED_REDIS_SERVICE` for rate limiting). |
| `@bts-soft/common` | `IdGenerator.generate('snowflake')` for all entity primary keys. |
| `@bts-soft/core` | Transitive platform utilities (error/response helpers used by the shard filters). |

### External highlights

NestJS 11 packages (`@nestjs/common`, `core`, `config`, `typeORM`, `bullmq`, `graphql`, `apollo`, `jwt`, `microservices`, `websockets`, `platform-express`, `platform-ws`, `terminus`), `typeorm`, `pg`, `bullmq`, `kafkajs`, `nats`, `graphql` 16.10.0, `@apollo/server`, `@apollo/subgraph`, `handlebars`, `nestjs-i18n`, `class-validator`, `class-transformer`, `rxjs`, `reflect-metadata`.

Dev: Jest 30, ts-jest, supertest, `@types/*`, ESLint 9 + typescript-eslint, Prettier, ts-node, tsconfig-paths, `@nestjs/cli` + schematics.

---

## Design Decisions

1. **No secrets in code** — `POSTGRES_PASSWORD` and all provider credentials come from the shared env file / process environment. Compose injects them at runtime.
2. **No mock auth** — a temporary in-process mock user service was replaced by the real `AuthModule` + gRPC-backed `USER_SERVICE` so `@Auth` enforces real JWT verification and real role checks in every environment.
3. **Role consolidation** — `CUSTOMER` was merged into `USER` across the platform; legacy `customer` rows are normalized to `user` at auth time inside `UserLookupService.normalizeRole`.
4. **Snowflake IDs** — every entity uses `IdGenerator.generate('snowflake')` string PKs for sortable, collision-resistant ids without a central UUID service.
5. **Strong typing, no `any`** — JSON payloads use `Record<string, unknown>`; Kafka payloads are typed via `KafkaEventPayload`; GraphQL responses are typed DTOs; the channel message builder produces a typed `NotificationMessage`.
6. **Inbox idempotency for both buses** — Kafka and RabbitMQ share the same inbox table with distinct consumer names, so at-least-once delivery from either bus cannot create duplicate notifications.
7. **Transactional writes for the aggregate** — notification + all delivery rows are inserted in one QueryRunner transaction with rollback on failure; dispatch happens only after commit.
8. **Job id determinism** — BullMQ `jobId`s are deterministic per notification+channel (and `retry:<deliveryId>` for requeues), preventing accidental duplicate jobs on producer retries.
9. **Sweep-based retry and expiry** — a single scheduled `sweep` job (60s) handles both expiration of stale notifications and requeueing of failed deliveries under a max-attempt and backoff policy, instead of per-job cron noise.
10. **Two-level template cache** — in-memory compiled templates for the hot path, Redis for cross-instance raw template sharing, DB as the source of truth, with locale fallback (`ar` -> `en`) and a safe generic fallback when no template exists or rendering throws.
11. **Security events bypass preferences** — `PASSWORD_RESET_REQUESTED` always creates an IN_APP record via `requiredChannels`, guaranteeing an audit trail even if the user disabled other channels.
12. **Realtime via outbox, not in-process emit** — the NATS publish path is decoupled from the request path through the outbox table so a crash between commit and publish cannot lose realtime events.
13. **Uniform GraphQL envelope** — every response is wrapped in `{ success, statusCode, message, data }` by the global interceptor, matching the platform convention used by the User Service and other microservices.
14. **Federation-ready GraphQL** — the subgraph uses Apollo Federation v2 with `autoSchemaFile: { federation: 2 }` so the gateway can compose it with other subgraphs.
15. **Graceful startup** — TypeORM retries for up to 3 minutes, bootstrap retries after 10s on failure, and process-level exception handlers keep the container alive while dependent infrastructure starts.
16. **Coverage thresholds enforced in CI** — Jest fails the build if coverage drops below statements 90 / lines 90 / branches 85 / functions 85; the function threshold accounts for GraphQL decorator arrow functions (`() => Type`) that execute only at schema build time (exercised in e2e, not in unit tests).
