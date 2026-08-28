# Ø®Ø·Ø© ØªÙ†ÙÙŠØ° Delivery Service â€” Ø§Ù„Ø®Ø·Ø© Ø§Ù„Ø´Ø§Ù…Ù„Ø© ÙˆØ§Ù„Ù…ÙØµÙ„Ø©

## Ù…Ù„Ø®Øµ ØªÙ†ÙÙŠØ°ÙŠ

Ø¨Ø¹Ø¯ Ù…Ø±Ø§Ø¬Ø¹Ø© Ø´Ø§Ù…Ù„Ø© Ù„ÙƒÙ„ Ù…Ù„ÙØ§Øª Ø§Ù„Ù…Ø´Ø±ÙˆØ¹ØŒ Ù‡Ø°Ù‡ Ø§Ù„Ø®Ø·Ø© ØªØºØ·ÙŠ:
1. **ØªØ´Ø®ÙŠØµ Ø§Ù„Ø£Ø®Ø·Ø§Ø¡ Ø§Ù„Ù…ÙˆØ¬ÙˆØ¯Ø©** ÙÙŠ Ø§Ù„ÙƒÙˆØ¯ Ø§Ù„Ø­Ø§Ù„ÙŠ
2. **Ù‡ÙŠÙƒÙ„ Ø§Ù„Ù…Ù„ÙØ§Øª Ø§Ù„ÙƒØ§Ù…Ù„** Ù„Ù„Ù€ delivery-service
3. **ØªØ­Ø¯ÙŠØ«Ø§Øª Ø§Ù„Ù€ protos**
4. **Docker Compose** Ø®Ø§Øµ Ø¨Ø§Ù„Ù€ delivery-service
5. **ØªØ­Ø¯ÙŠØ«Ø§Øª infrastructure/docker/compose.yml**
6. **Ù…Ù„Ù graphql-docs/delivery.graphql**
7. **Ø¹Ù„Ø§Ù‚Ø© delivery-service Ø¨Ø¨Ù‚ÙŠØ© Ø§Ù„Ù€ services**
8. **Ù…Ø§ ÙŠÙ†Ù‚Øµ Ø§Ù„Ù€ shared package**
9. **ØªØ´Ø®ÙŠØµ port conflicts**

---

## ðŸ”´ Ø§Ù„Ø£Ø®Ø·Ø§Ø¡ Ø§Ù„Ù…ÙˆØ¬ÙˆØ¯Ø© Ø§Ù„Ø¢Ù† (ÙŠØ¬Ø¨ Ø¥ØµÙ„Ø§Ø­Ù‡Ø§ Ø£ÙˆÙ„Ø§Ù‹)

### 1. `app.module.ts` â€” Ø£Ø®Ø·Ø§Ø¡ Ø¬Ø³ÙŠÙ…Ø©

```diff
# Ø³Ø·Ø± 52-54 â€” Ø§Ø³ØªØ®Ø¯Ø§Ù… "delivery" Ø¨Ø¯Ù„ "username"
- deliveryname: config.get<string>('POSTGRES_delivery') || ...
+ username: config.get<string>('POSTGRES_USER') || config.get<string>('DB_USERNAME', 'postgres'),

# Ø³Ø·Ø± 57 â€” entities ØºÙŠØ± Ù…Ø¹Ø±ÙØ© (Delivery, Address, Outbox) ØºÙŠØ± Ù…Ø³ØªÙˆØ±Ø¯Ø©
- entities: [delivery, Address, Outbox],
+ entities: [], // Ø³ÙŠØªÙ… ØªØ¹Ø¨Ø¦ØªÙ‡Ø§ Ù„Ø§Ø­Ù‚Ø§Ù‹

# Ø³Ø·Ø± 85 â€” req.delivery Ø®Ø·Ø£ØŒ Ø§Ù„Ù…ÙØ±ÙˆØ¶ req.user
- delivery: req.delivery,
+ user: req.user,
```

### 2. `kafka.module.ts` â€” token name Ø®Ø·Ø£

```diff
# ÙŠØ³ØªÙˆØ±Ø¯ REALTIME_EVENT_HANDLERS Ù„ÙƒÙ† ÙŠØ³ØªØ®Ø¯Ù… DELIVERY_EVENT_HANDLERS
- import { REALTIME_EVENT_HANDLERS } from './handlers/base-kafka-event.handler';
+ import { DELIVERY_EVENT_HANDLERS } from './handlers/base-kafka-event.handler';
```

### 3. `kafka.consumer.ts` â€” ÙŠØ³ØªÙˆØ±Ø¯ `RealtimeMetricsService` Ù…Ù† Ù…Ø³Ø§Ø± Ø®Ø§Ø·Ø¦

```diff
# Ù‡Ø°Ø§ Ù…Ù„Ù metrics Ø§Ù„Ù€ realtime-service ÙˆÙ„ÙŠØ³ delivery-service
- import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
+ import { DeliveryMetricsService } from '../../../common/metrics/delivery-metrics.service';
```

### 4. `grpc.client.ts` â€” ÙŠØ³ØªÙˆØ±Ø¯ `RealtimeConfig` Ùˆ `TIMINGS` ØºÙŠØ± Ù…ÙˆØ¬ÙˆØ¯ÙŠÙ†

```diff
- const cfg = config.get<RealtimeConfig>('realtime')!;
+ const cfg = config.get<DeliveryGrpcConfig>('grpc')!;
```

### 5. `nats.subscriber.ts` â€” Ù‡Ø°Ø§ Ù…Ù„Ù Ø§Ù„Ù€ realtime-service ÙˆÙ„ÙŠØ³ delivery-service

Ø§Ù„Ù€ nats.subscriber.ts Ø§Ù„Ù…ÙˆØ¬ÙˆØ¯ ÙÙŠ delivery-service ÙŠØ³ØªÙˆØ±Ø¯:
- `SubscriptionStore` â€” Ø®Ø§Øµ Ø¨Ù€ realtime-service
- `ConnectionService` â€” Ø®Ø§Øµ Ø¨Ù€ realtime-service
- `SocketWriter` â€” Ø®Ø§Øµ Ø¨Ù€ realtime-service
- `RealtimeMetricsService` â€” Ø®Ø§Øµ Ø¨Ù€ realtime-service

**Ø§Ù„Ù€ delivery-service Ù„Ø§ ÙŠØ­ØªØ§Ø¬ NATS subscriber** â€” ÙŠØ­ØªØ§Ø¬ ÙÙ‚Ø· **publisher** Ù„Ø¥Ø±Ø³Ø§Ù„ status updates.

### 6. `base-kafka-event.handler.ts` â€” Ù…Ù†Ø·Ù‚ Ø§Ù„Ù€ realtime-service Ù…ÙˆØ¬ÙˆØ¯ ÙÙŠ delivery

Ø§Ù„Ù€ handler ÙŠÙ€ publish Ø¹Ù„Ù‰ NATS subjects â€” Ù‡Ø°Ø§ Ø¯ÙˆØ± Ø§Ù„Ù€ realtime-service ÙˆÙ„ÙŠØ³ delivery-service.
Ø§Ù„Ù€ delivery-service ÙŠØ¬Ø¨ Ø£Ù† ÙŠÙƒØªØ¨ ÙÙŠ DB ÙÙ‚Ø· ÙˆÙŠÙ†Ø´Ø± Ø¹Ù„Ù‰ Kafka Ø¹Ø¨Ø± Ø§Ù„Ù€ Outbox.

### 7. `main.ts` â€” port ØªØ¹Ø§Ø±Ø¶

```diff
# 4003 Ù…Ø´ Ù…Ø­Ø¬ÙˆØ² Ù„ÙƒÙ† Ø§Ø³ØªØ®Ø¯Ø§Ù… 4003 Ø£ÙƒØ«Ø± Ù…Ù†Ø·Ù‚ÙŠØ© Ù…Ø¹ Ø§Ù„ØªØ±ØªÙŠØ¨ Ø§Ù„Ø­Ø§Ù„ÙŠ
- const port = process.env.PORT_DELIVERY ?? 4003;
+ const port = process.env.PORT_DELIVERY ?? 4003;
```

### 8. `package.json` â€” typeorm version Ø®Ø§Ø·Ø¦Ø©

```diff
- "typeorm": "^1.1.0"
+ "typeorm": "^0.3.21"
```

> [!CAUTION]
> TypeORM `^1.1.0` ØºÙŠØ± Ù…ÙˆØ¬ÙˆØ¯ â€” Ø§Ù„Ø¥ØµØ¯Ø§Ø± Ø§Ù„Ø­Ø§Ù„ÙŠ Ù‡Ùˆ `0.3.x`. Ù‡Ø°Ø§ Ø³ÙŠÙ…Ù†Ø¹ Ø§Ù„ØªØ´ØºÙŠÙ„ Ø§Ù„ÙƒØ§Ù…Ù„.

---

## ðŸ“Š Port Mapping Ø§Ù„Ø­Ø§Ù„ÙŠ ÙˆØ§Ù„Ù…Ù‚ØªØ±Ø­

| Service              | HTTP Port    | gRPC Port | Metrics Port |
|----------------------|--------------|-----------|--------------|
| api-gateway          | 4000         | â€”         | â€”            |
| user-service         | 4001         | 50051     | â€”            |
| **delivery-service** | **4003**     | **50054** | **9104**     |
| media-service        | 4005         | 50052     | 9102         |
| notification-service | 4004         | 50053     | â€”            |
| realtime-service     | 4006         | â€”         | â€”            |
| search-service       | 4007         | â€”         | 9103         |
| user-db              | 5433:5432    | â€”         | â€”            |
| notification-db      | 5434:5432    | â€”         | â€”            |
| **delivery-db**      | **5435:5432**| â€”         | â€”            |

> [!NOTE]
> Ù„Ø§ ÙŠÙˆØ¬Ø¯ ØªØ¹Ø§Ø±Ø¶ ÙÙŠ Ø§Ù„Ù€ ports â€” port 4003 ÙˆÙ‚Øª Ø§Ù„ØªØ¹Ø§Ø±Ø¶ Ù„Ù… ÙŠÙƒÙ† Ù…Ø­Ø¬ÙˆØ²Ø§Ù‹ Ù„ÙƒÙ† 4003 Ù‡Ùˆ Ø§Ù„ØªØ±ØªÙŠØ¨ Ø§Ù„Ù…Ù†Ø·Ù‚ÙŠ.
> gRPC ports: user=50051, media=50052, notification=50053, **delivery=50054**, driver=50055 (future), payment=50056 (future)

---

## ðŸ—ï¸ Ù‡ÙŠÙƒÙ„ Ø§Ù„Ù…Ù„ÙØ§Øª Ø§Ù„Ù…Ø·Ù„ÙˆØ¨ (Final Target Structure)

Ù…Ù‚Ø§Ø±Ù†Ø©Ù‹ Ø¨Ù€ `realtime-service`, `user-service`, `notification-service`:

```
services/delivery-service/
â”œâ”€â”€ .dockerignore                                    [NEW]
â”œâ”€â”€ .prettierrc                                      [EXISTS]
â”œâ”€â”€ Dockerfile                                       [EXISTS]
â”œâ”€â”€ docker-compose.yml                               [NEW]
â”œâ”€â”€ eslint.config.mjs                                [EXISTS]
â”œâ”€â”€ nest-cli.json                                    [EXISTS]
â”œâ”€â”€ package.json                                     [MODIFY]
â”œâ”€â”€ tsconfig.json                                    [EXISTS]
â”œâ”€â”€ tsconfig.build.json                              [EXISTS]
â””â”€â”€ src/
    â”œâ”€â”€ main.ts                                      [MODIFY]
    â”œâ”€â”€ app.module.ts                                [MODIFY]
    â”œâ”€â”€ app.resolver.ts                              [NEW]
    â”œâ”€â”€ health.controller.ts                         [NEW]
    â”œâ”€â”€ schema.gql                                   [AUTO-GENERATED]
    â”‚
    â”œâ”€â”€ common/
    â”‚   â”œâ”€â”€ config/
    â”‚   â”‚   â””â”€â”€ delivery.config.ts                   [NEW]
    â”‚   â”œâ”€â”€ translation/
    â”‚   â”‚   â””â”€â”€ translation.module.ts                [EXISTS]
    â”‚   â””â”€â”€ metrics/
    â”‚       â”œâ”€â”€ delivery-metrics.service.ts           [NEW]
    â”‚       â””â”€â”€ delivery-metrics.module.ts            [NEW]
    â”‚
    â””â”€â”€ modules/
        â”œâ”€â”€ infrastructure/
        â”‚   â”œâ”€â”€ grpc/
        â”‚   â”‚   â”œâ”€â”€ grpc.module.ts                   [MODIFY]
        â”‚   â”‚   â”œâ”€â”€ grpc.client.ts                   [REWRITE]
        â”‚   â”‚   â””â”€â”€ grpc.server.ts                   [NEW]
        â”‚   â”œâ”€â”€ kafka/
        â”‚   â”‚   â”œâ”€â”€ kafka.module.ts                  [MODIFY]
        â”‚   â”‚   â”œâ”€â”€ kafka.producer.ts                [NEW]
        â”‚   â”‚   â”œâ”€â”€ kafka.consumer.ts                [MODIFY]
        â”‚   â”‚   â””â”€â”€ handlers/
        â”‚   â”‚       â”œâ”€â”€ base-delivery-handler.ts      [NEW]
        â”‚   â”‚       â”œâ”€â”€ payment-completed.handler.ts  [MODIFY]
        â”‚   â”‚       â””â”€â”€ payment-failed.handler.ts     [MODIFY]
        â”‚   â”œâ”€â”€ nats/
        â”‚   â”‚   â”œâ”€â”€ nats.module.ts                   [MODIFY - Ø¥Ø²Ø§Ù„Ø© subscriber]
        â”‚   â”‚   â”œâ”€â”€ nats.service.ts                  [KEEP]
        â”‚   â”‚   â””â”€â”€ nats.publisher.ts                [KEEP]
        â”‚   â”œâ”€â”€ outbox/
        â”‚   â”‚   â”œâ”€â”€ outbox.module.ts                 [NEW]
        â”‚   â”‚   â”œâ”€â”€ outbox.entity.ts                 [NEW]
        â”‚   â”‚   â”œâ”€â”€ outbox.repository.ts             [NEW]
        â”‚   â”‚   â””â”€â”€ outbox-publisher.service.ts      [NEW]
        â”‚   â””â”€â”€ redis/
        â”‚       â”œâ”€â”€ redis.module.ts                  [NEW]
        â”‚       â””â”€â”€ idempotency.service.ts           [NEW]
        â”‚
        â”œâ”€â”€ delivery/
        â”‚   â”œâ”€â”€ delivery.module.ts                   [NEW]
        â”‚   â”œâ”€â”€ entities/
        â”‚   â”‚   â”œâ”€â”€ delivery.entity.ts               [NEW]
        â”‚   â”‚   â”œâ”€â”€ address.entity.ts                [NEW]
        â”‚   â”‚   â””â”€â”€ delivery-status-history.entity.ts[NEW]
        â”‚   â”œâ”€â”€ enums/
        â”‚   â”‚   â”œâ”€â”€ delivery-status.enum.ts          [NEW]
        â”‚   â”‚   â””â”€â”€ payment-status.enum.ts           [NEW]
        â”‚   â”œâ”€â”€ dto/
        â”‚   â”‚   â”œâ”€â”€ create-delivery.input.ts         [NEW]
        â”‚   â”‚   â”œâ”€â”€ cancel-delivery.input.ts         [NEW]
        â”‚   â”‚   â”œâ”€â”€ delivery.type.ts                 [NEW]
        â”‚   â”‚   â”œâ”€â”€ address.type.ts                  [NEW]
        â”‚   â”‚   â””â”€â”€ delivery-connection.type.ts      [NEW]
        â”‚   â”œâ”€â”€ state-machine/
        â”‚   â”‚   â””â”€â”€ delivery.state-machine.ts        [NEW]
        â”‚   â”œâ”€â”€ commands/
        â”‚   â”‚   â”œâ”€â”€ create-delivery.command.ts       [NEW]
        â”‚   â”‚   â”œâ”€â”€ cancel-delivery.command.ts       [NEW]
        â”‚   â”‚   â”œâ”€â”€ accept-delivery.command.ts       [NEW]
        â”‚   â”‚   â”œâ”€â”€ reject-delivery.command.ts       [NEW]
        â”‚   â”‚   â”œâ”€â”€ start-pickup.command.ts          [NEW]
        â”‚   â”‚   â”œâ”€â”€ mark-picked-up.command.ts        [NEW]
        â”‚   â”‚   â”œâ”€â”€ start-transit.command.ts         [NEW]
        â”‚   â”‚   â””â”€â”€ complete-delivery.command.ts     [NEW]
        â”‚   â”œâ”€â”€ queries/
        â”‚   â”‚   â”œâ”€â”€ get-delivery.query.ts            [NEW]
        â”‚   â”‚   â”œâ”€â”€ get-active-delivery.query.ts     [NEW]
        â”‚   â”‚   â”œâ”€â”€ get-my-deliveries.query.ts       [NEW]
        â”‚   â”‚   â””â”€â”€ get-delivery-history.query.ts    [NEW]
        â”‚   â”œâ”€â”€ resolvers/
        â”‚   â”‚   â”œâ”€â”€ delivery.resolver.ts             [NEW]
        â”‚   â”‚   â””â”€â”€ delivery.query.resolver.ts       [NEW]
        â”‚   â”œâ”€â”€ services/
        â”‚   â”‚   â”œâ”€â”€ delivery-command.service.ts      [NEW]
        â”‚   â”‚   â””â”€â”€ delivery-query.service.ts        [NEW]
        â”‚   â””â”€â”€ repositories/
        â”‚       â”œâ”€â”€ delivery.repository.ts           [NEW]
        â”‚       â””â”€â”€ delivery-history.repository.ts   [NEW]
        â”‚
        â””â”€â”€ saga/
            â”œâ”€â”€ saga.module.ts                       [NEW]
            â”œâ”€â”€ delivery-saga.orchestrator.ts        [NEW]
            â”œâ”€â”€ delivery-saga-state.entity.ts        [NEW]
            â””â”€â”€ steps/
                â”œâ”€â”€ reserve-driver.step.ts           [NEW]
                â”œâ”€â”€ process-payment.step.ts          [NEW]
                â”œâ”€â”€ confirm-delivery.step.ts         [NEW]
                â”œâ”€â”€ release-driver.step.ts           [NEW - compensation]
                â””â”€â”€ refund-payment.step.ts           [NEW - compensation]
```

---

## ðŸ”— Ø¹Ù„Ø§Ù‚Ø© delivery-service Ø¨Ø¨Ù‚ÙŠØ© Ø§Ù„Ù€ services

### 1. delivery-service â†” api-gateway
- **Ø§Ù„Ø§ØªØ¬Ø§Ù‡:** api-gateway â†’ delivery-service
- **Ø§Ù„Ø¨Ø±ÙˆØªÙˆÙƒÙˆÙ„:** HTTP (GraphQL Federation Subgraph)
- **Ø§Ù„Ù€ URL:** `http://delivery-srv:4003/delivery/graphql`
- **ÙŠØ¬Ø¨ Ø¥Ø¶Ø§ÙØªÙ‡ ÙÙŠ compose.yml:** `DELIVERY_SERVICE_URL: "http://delivery-srv:4003/delivery/graphql"`

### 2. delivery-service â†’ driver-service (Go â€” Ù…Ø³ØªÙ‚Ø¨Ù„ÙŠ)
- **Ø§Ù„Ø¨Ø±ÙˆØªÙˆÙƒÙˆÙ„:** gRPC
- **Ø§Ù„Ù€ Calls Ø§Ù„Ù…Ø·Ù„ÙˆØ¨Ø© Ù„Ù„Ù€ Saga:**
  - `FindAvailableDriver(pickup_location)` â†’ ÙŠÙØ¹ÙŠØ¯ driverId
  - `AssignDriver(driverId, deliveryId)` â†’ ÙŠØ­Ø¬Ø² Ø§Ù„Ø³Ø§Ø¦Ù‚
  - `ReleaseDriver(driverId)` â†’ compensation Ø¹Ù†Ø¯ Ø§Ù„ÙØ´Ù„
- **Ø§Ù„Ù€ proto Ø§Ù„Ø­Ø§Ù„ÙŠ:** ÙŠØ­ØªÙˆÙŠ ÙÙ‚Ø· Ø¹Ù„Ù‰ `IsAssignedDriver` â€” **Ù†Ø§Ù‚Øµ**

### 3. delivery-service â†’ payment-service (Go â€” Ù…Ø³ØªÙ‚Ø¨Ù„ÙŠ)
- **Ø§Ù„Ø¨Ø±ÙˆØªÙˆÙƒÙˆÙ„:** gRPC
- **Ø§Ù„Ù€ Calls:**
  - `CreatePayment(deliveryId, customerId, amount)` â†’ Saga step
  - `RefundPayment(deliveryId)` â†’ compensation
- **Ø§Ù„Ù€ proto:** **ØºÙŠØ± Ù…ÙˆØ¬ÙˆØ¯ â€” ÙŠØ¬Ø¨ Ø¥Ù†Ø´Ø§Ø¡ `payment.proto`**

### 4. delivery-service â†’ notification-service
- **Ø§Ù„Ø¨Ø±ÙˆØªÙˆÙƒÙˆÙ„:** gRPC
- **Ø§Ù„Ù€ Call:** `SendNotification(userId, type, title, body, data)`
- **Ø§Ù„Ù€ proto:** `notification.proto` â€” Ù…ÙˆØ¬ÙˆØ¯ ÙˆÙƒØ§Ù…Ù„ âœ…
- **Ù…ØªÙ‰:** Ø¹Ù†Ø¯ ØªØ¹ÙŠÙŠÙ† Ø³Ø§Ø¦Ù‚ØŒ Ù‚Ø¨ÙˆÙ„/Ø±ÙØ¶ØŒ Ø¨Ø¯Ø¡ Ø§Ù„Ø§Ø³ØªÙ„Ø§Ù…ØŒ Ø§Ù„Ø¥Ù„ØºØ§Ø¡ØŒ Ø§Ù„Ø§ÙƒØªÙ…Ø§Ù„

### 5. delivery-service â†’ realtime-service
- **Ø§Ù„Ø¨Ø±ÙˆØªÙˆÙƒÙˆÙ„:** NATS publish
- **Subjects:**
  - `realtime.delivery.status.updated` â†’ Ø¹Ù†Ø¯ Ø£ÙŠ ØªØºÙŠÙŠØ± ÙÙŠ Ø§Ù„Ù€ status
  - `realtime.driver.assignment.updated` â†’ Ø¹Ù†Ø¯ ØªØ¹ÙŠÙŠÙ†/Ø±ÙØ¶ Ø³Ø§Ø¦Ù‚
- **Ø§Ù„Ù€ realtime-service** ÙŠØ³ØªÙ‡Ù„Ùƒ ÙˆÙŠØ¨Ø« Ø¹Ø¨Ø± WebSocket

### 6. delivery-service â† realtime-service
- **Ø§Ù„Ø¨Ø±ÙˆØªÙˆÙƒÙˆÙ„:** gRPC (delivery ÙƒÙ€ server)
- **Ø§Ù„Ù€ Call:** `IsParticipant(userId, deliveryId)` â†’ authorization check
- **Ø§Ù„Ù€ proto:** `delivery.proto` â€” Ù…ÙˆØ¬ÙˆØ¯ âœ…

### 7. delivery-service â†’ user-service
- **Ø§Ù„Ø¨Ø±ÙˆØªÙˆÙƒÙˆÙ„:** gRPC
- **Ø§Ù„Ù€ Call:** `GetUser(userId)` â†’ Ø§Ù„ØªØ­Ù‚Ù‚ Ù…Ù† ÙˆØ¬ÙˆØ¯ Ø§Ù„Ù…Ø³ØªØ®Ø¯Ù…
- **Ø§Ù„Ù€ proto:** `user.proto` â€” Ù…ÙˆØ¬ÙˆØ¯ ÙˆÙƒØ§Ù…Ù„ âœ…

### 8. delivery-service â†’ Kafka (producer)
Ø§Ù„Ù€ events Ø§Ù„ØªÙŠ ÙŠÙØµØ¯Ø±Ù‡Ø§ Ø¹Ø¨Ø± Ø§Ù„Ù€ Transactional Outbox:

| Event | Ø§Ù„Ù…Ø³ØªÙ‡Ù„ÙƒÙˆÙ† |
|-------|-----------|
| `delivery.created` | notification, analytics, realtime |
| `delivery.driver.assigned` | notification, realtime |
| `delivery.driver.accepted` | notification, realtime |
| `delivery.picked_up` | notification, realtime, analytics |
| `delivery.in_transit` | notification, realtime |
| `delivery.completed` | notification, analytics |
| `delivery.cancelled` | notification, analytics |

### 9. delivery-service â† Kafka (consumer)
| Event | Ø§Ù„Ø¥Ø¬Ø±Ø§Ø¡ |
|-------|---------|
| `payment.completed` | ØªØ­Ø¯ÙŠØ« `paymentStatus = PAID`ØŒ Ø¥ÙƒÙ…Ø§Ù„ Ø§Ù„Ù€ Saga |
| `payment.failed` | compensation: release driver + cancel delivery |

---

## ðŸ“‹ Ø§Ù„ØªØ­Ø¯ÙŠØ«Ø§Øª Ø§Ù„Ù…Ø·Ù„ÙˆØ¨Ø© Ø¹Ù„Ù‰ Ø§Ù„Ù€ `.proto` files

### [MODIFY] `delivery.proto` â€” Ø¥Ø¶Ø§ÙØ© RPC

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

### [MODIFY] `driver.proto` â€” Ø¥Ø¶Ø§ÙØ© RPCs Ù„Ù„Ù€ Saga

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

### [NEW] `payment.proto` â€” Ù„Ù„Ù€ Saga payment step

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

## âš ï¸ Ù…Ø§ ÙŠÙ†Ù‚Øµ Ø§Ù„Ù€ shared package `@delivery/common`

### Ø§Ù„Ù…ÙˆØ¬ÙˆØ¯ âœ…
- `DeliveryEventType` enum, `DeliveryKafkaTopics`, `PaymentKafkaTopics`
- `RealtimeNatsSubjects`, `NotificationNatsSubjects`
- `KafkaModule`, `KafkaService`, `NatsModule`, `NatsService`
- Auth guards, decorators, filters, interceptors
- `DeliveryCreatedPayload`, `DeliveryUpdatedPayload`, `DeliveryDriverAssignedPayload`

### Ø§Ù„Ù†Ø§Ù‚Øµ â€” ÙŠØ¬Ø¨ Ø¥Ø¶Ø§ÙØªÙ‡ ðŸ”´

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

#### [MODIFY] `src/index.ts` â€” Ø¥Ø¶Ø§ÙØ© export

```typescript
export * from './events/payment.events';
```

---

## ðŸ³ [NEW] `services/delivery-service/docker-compose.yml`

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

## ðŸ”§ ØªØ­Ø¯ÙŠØ«Ø§Øª `infrastructure/docker/compose.yml`

### Ø¥Ø¶Ø§ÙØ© delivery-db-srv

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

### Ø¥Ø¶Ø§ÙØ© delivery-service

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

### ØªØ­Ø¯ÙŠØ« api-gateway environment

```yaml
DELIVERY_SERVICE_URL: "http://delivery-srv:4003/delivery/graphql"
```

### Ø¥Ø¶Ø§ÙØ© volumes

```yaml
volumes:
  delivery_db_data:
```

---

## ðŸ“„ [NEW] `graphql-docs/delivery.graphql`

```graphql
# ===========================================
# DELIVERY SERVICE â€” GraphQL Documentation
# ===========================================
# Base URL: http://localhost:4000/graphql (via API Gateway)
# Auth: Bearer <JWT_TOKEN> required for all
# Roles: CUSTOMER | DRIVER | ADMIN
# ===========================================

# â”€â”€ QUERIES â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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

# Get paginated deliveries â€” Customer: own orders | Driver: assigned
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

# â”€â”€ MUTATIONS â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

# Create a new delivery â€” initiates the Delivery Saga
# Roles: CUSTOMER
# State machine: PENDING â†’ SEARCHING_DRIVER â†’ DRIVER_ASSIGNED â†’ ...
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
# Transition: DRIVER_ASSIGNED â†’ DRIVER_ACCEPTED
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
# Transition: DRIVER_ASSIGNED â†’ SEARCHING_DRIVER (retry) | CANCELLED
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
# Transition: DRIVER_ACCEPTED â†’ PICKUP_STARTED
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
# Transition: PICKUP_STARTED â†’ PICKED_UP
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
# Transition: PICKED_UP â†’ IN_TRANSIT
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
# Transition: IN_TRANSIT â†’ DELIVERED
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

## ðŸ—ºï¸ Ø®Ø·Ø© Ø§Ù„ØªÙ†ÙÙŠØ° Ø§Ù„Ù…Ø±Ø­Ù„ÙŠØ©

### Ø§Ù„Ù…Ø±Ø­Ù„Ø© 1 â€” Ø¥ØµÙ„Ø§Ø­ Ø§Ù„Ø£Ø®Ø·Ø§Ø¡ Ø§Ù„Ø­Ø§Ù„ÙŠØ© ðŸ”´

- [ ] Ø¥ØµÙ„Ø§Ø­ `package.json` â€” typeorm version Ø¥Ù„Ù‰ `^0.3.21`
- [ ] Ø¥ØµÙ„Ø§Ø­ `app.module.ts` â€” username, entities, context
- [ ] Ø¥ØµÙ„Ø§Ø­ `kafka.module.ts` â€” token name
- [ ] Ø¥ØµÙ„Ø§Ø­ `kafka.consumer.ts` â€” metrics import
- [ ] Ø¥ØµÙ„Ø§Ø­ `grpc.client.ts` â€” config type Ùˆ TIMINGS
- [ ] Ø¥ØµÙ„Ø§Ø­ `nats.module.ts` â€” Ø¥Ø²Ø§Ù„Ø© subscriber Ø§Ù„Ø®Ø§Ø·Ø¦
- [ ] Ø¥ØµÙ„Ø§Ø­ `main.ts` â€” port 4003

### Ø§Ù„Ù…Ø±Ø­Ù„Ø© 2 â€” Ø§Ù„Ù…Ù„ÙØ§Øª ÙˆØ§Ù„Ø¥Ø¹Ø¯Ø§Ø¯ ðŸŸ 

- [ ] Ø¥Ù†Ø´Ø§Ø¡ `docker-compose.yml` Ù„Ù„Ù€ delivery-service
- [ ] ØªØ­Ø¯ÙŠØ« `infrastructure/docker/compose.yml`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery.config.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery-metrics.service.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `health.controller.ts`

### Ø§Ù„Ù…Ø±Ø­Ù„Ø© 3 â€” Database Layer

- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery.entity.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `address.entity.ts` (embedded)
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery-status-history.entity.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery-status.enum.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `payment-status.enum.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `outbox.entity.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery-saga-state.entity.ts`

### Ø§Ù„Ù…Ø±Ø­Ù„Ø© 4 â€” Business Logic

- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery.state-machine.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery-command.service.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery-query.service.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery.repository.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `idempotency.service.ts`

### Ø§Ù„Ù…Ø±Ø­Ù„Ø© 5 â€” GraphQL Layer

- [ ] Ø¥Ù†Ø´Ø§Ø¡ DTOs Ùˆ Types
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery.resolver.ts` (mutations)
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery.query.resolver.ts` (queries)
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `app.resolver.ts`

### Ø§Ù„Ù…Ø±Ø­Ù„Ø© 6 â€” Outbox + Kafka Producer

- [ ] Ø¥Ù†Ø´Ø§Ø¡ `outbox.repository.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `outbox-publisher.service.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `kafka.producer.ts`

### Ø§Ù„Ù…Ø±Ø­Ù„Ø© 7 â€” Saga + gRPC

- [ ] Ø¥Ù†Ø´Ø§Ø¡ `delivery-saga.orchestrator.ts`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ saga steps
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `grpc.server.ts` (expose IsParticipant)
- [ ] ØªÙØ¹ÙŠÙ„ gRPC transport ÙÙŠ `main.ts`

### Ø§Ù„Ù…Ø±Ø­Ù„Ø© 8 â€” Protos + Shared Package + Docs

- [ ] ØªØ­Ø¯ÙŠØ« `delivery.proto`
- [ ] ØªØ­Ø¯ÙŠØ« `driver.proto`
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `payment.proto`
- [ ] Ø¥Ø¶Ø§ÙØ© `payment.events.ts` ÙÙŠ shared package
- [ ] Ø¥Ø¶Ø§ÙØ© `KafkaEventEnvelope` ÙÙŠ shared package
- [ ] Ø¥Ù†Ø´Ø§Ø¡ `graphql-docs/delivery.graphql`

---

## âœ… Ø¬Ø¯ÙˆÙ„ Ø§Ù„Ø£ÙˆÙ„ÙˆÙŠØ§Øª Ø§Ù„ÙÙˆØ±ÙŠØ©

| # | Ø§Ù„Ø¥Ø¬Ø±Ø§Ø¡ | Ø§Ù„Ø£ÙˆÙ„ÙˆÙŠØ© | Ø§Ù„Ù…Ù„Ù |
|---|---------|----------|-------|
| 1 | Ø¥ØµÙ„Ø§Ø­ typeorm version | ðŸ”´ Critical | `package.json` |
| 2 | Ø¥ØµÙ„Ø§Ø­ TypeORM config (username, entities) | ðŸ”´ Critical | `app.module.ts` |
| 3 | Ø¥ØµÙ„Ø§Ø­ kafka token name | ðŸ”´ Critical | `kafka.module.ts` |
| 4 | Ø¥ØµÙ„Ø§Ø­ metrics import | ðŸ”´ Critical | `kafka.consumer.ts` |
| 5 | Ø¥ØµÙ„Ø§Ø­ grpc config + TIMINGS | ðŸ”´ Critical | `grpc.client.ts` |
| 6 | Ø¥Ø²Ø§Ù„Ø© nats subscriber Ø§Ù„Ø®Ø§Ø·Ø¦ | ðŸ”´ Critical | `nats.module.ts` + `nats.subscriber.ts` |
| 7 | Ø¥ØµÙ„Ø§Ø­ port Ø¥Ù„Ù‰ 4003 | ðŸŸ  High | `main.ts` |
| 8 | Ø¥Ù†Ø´Ø§Ø¡ `docker-compose.yml` | ðŸŸ  High | `delivery-service/` |
| 9 | ØªØ­Ø¯ÙŠØ« `compose.yml` Ø§Ù„Ù…Ø´ØªØ±Ùƒ | ðŸŸ  High | `infrastructure/docker/` |
| 10 | ØªØ­Ø¯ÙŠØ« `driver.proto` | ðŸŸ¡ Medium | `protos/` |
| 11 | Ø¥Ù†Ø´Ø§Ø¡ `payment.proto` | ðŸŸ¡ Medium | `protos/` |
| 12 | Ø¥Ø¶Ø§ÙØ© `payment.events.ts` | ðŸŸ¡ Medium | `packages/ts/` |
| 13 | Ø¥Ù†Ø´Ø§Ø¡ `delivery.graphql` docs | ðŸŸ¡ Medium | `graphql-docs/` |
| 14 | Ø¨Ù†Ø§Ø¡ Ù‡ÙŠÙƒÙ„ Ø§Ù„Ù…Ù„ÙØ§Øª Ø§Ù„ÙƒØ§Ù…Ù„ | ðŸŸ¢ Standard | `delivery-service/src/` |

