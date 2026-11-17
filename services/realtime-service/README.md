# Realtime Service

The Realtime Service is a high-throughput, low-latency communication backbone for the Realtime Delivery microservices platform. Built with NestJS 11 and TypeScript, it orchestrates bidirectional real-time communication between customers, couriers, and platform services using WebSockets (Socket.IO and native WebSockets), Apollo Federation GraphQL Subscriptions, and distributed pub/sub fabrics (Redis, NATS JetStream, and Apache Kafka).

---

## Architecture Overview

```
                      +-----------------------------+
                      |     Client Applications     |
                      |  (Web, iOS, Android, Fleet) |
                      +--------------+--------------+
                                     |
                                     | WebSocket (WSS) / HTTP (GraphQL Subscriptions)
                                     v
                      +-----------------------------+
                      |      API Gateway / Envoy     |
                      +--------------+--------------+
                                     |
                                     | Reverse Proxy / TLS Termination (Port 3004)
                                     v
                 +---------------------------------------+
                 |            Realtime Service           |
                 |---------------------------------------|
                 |  WebSocket Gateway (Socket.IO / WS)   |
                 |  GraphQL Subscription Engine (Apollo) |
                 |  Connection Registry & Session Store  |
                 |  Driver Geolocation Processing Pipe   |
                 |  Rate Limiting & Authentication Guard |
                 +-----+-------------+-------------+-----+
                       |             |             |
                       v             v             v
                +-------------+ +----------+ +-------------+
                | Redis PubSub| |   NATS   | |    Kafka    |
                |  (Ephemeral | |JetStream | |  (Durable   |
                |   Broadcast)| | (Events) | | Domain Log) |
                +-------------+ +----------+ +-------------+
```

### Core Responsibilities
- **Driver Telemetry Streaming**: Ingests high-frequency GPS coordinate streams from mobile driver clients, processes bearing/speed updates, and performs geofence calculations.
- **Order Lifecycle Broadcasting**: Pushes instant state transitions (`PENDING`, `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `DELIVERED`) to active customer tracking interfaces.
- **Clustered Pub/Sub Fabric**: Uses Redis Pub/Sub and NATS JetStream to broadcast events across multiple horizontally scaled service instances without sticky session dependencies.
- **Connection Registry & Health**: Maintains real-time tracking of active socket connections, user device mappings, and room subscriptions with automatic reconnection handling and dead-socket eviction.

---

## Directory Structure

```
services/realtime-service/
|-- src/
|   |-- common/
|   |   |-- config/               Configuration service and environment validation schema
|   |   |-- constants/            Event names, channel definitions, and error codes
|   |   |-- filters/              WebSocket and HTTP exception filters
|   |   |-- guards/               JWT authentication and role-based authorization guards
|   |   |-- interceptors/         Structured logging, tracing, and metric collection
|   |   `-- metrics/              Prometheus metrics definitions and registry
|   |-- modules/
|   |   |-- gateway/              Socket.IO and WebSocket connection lifecycle management
|   |   |   |-- authorization/    Connection handshake token validation and RBAC checks
|   |   |   |-- connection/       Connection registry, distributed session state store
|   |   |   |-- heartbeat/        Periodic ping/pong monitor and stale socket cleanup
|   |   |   `-- websocket/        WebSocket event router, rate limiting, and handlers
|   |   |-- driver-location/      Driver coordinates ingestion, filtering, and room broadcast
|   |   |-- delivery-stream/      Order room management and state change propagation
|   |   |-- graphql/              Apollo Federation schema, subscription resolvers, PubSub
|   |   `-- infrastructure/       Message broker integration layer
|   |       |-- grpc/             gRPC client for driver and delivery verification
|   |       |-- kafka/            Kafka consumers for durable delivery domain events
|   |       |-- nats/             NATS JetStream publishers and subscribers
|   |       `-- rabbitmq/         RabbitMQ broadcast consumer for platform-wide alerts
|   |-- app.module.ts             Root application module
|   `-- main.ts                   Application bootstrap, CORS configuration, listener startup
|-- test/
|   |-- app.e2e-spec.ts           Health check and HTTP API end-to-end integration tests
|   |-- graphql.e2e-spec.ts       GraphQL subscription lifecycle end-to-end tests
|   |-- websocket.e2e-spec.ts     Socket.IO connection, room join, and message flow tests
|   `-- jest-e2e.json             Jest end-to-end test suite runner configuration
|-- Dockerfile                    Multi-stage production container build
|-- docker-compose.yml            Local infrastructure dependencies (Redis, NATS, Kafka)
|-- Makefile                      Automation targets for build, test, lint, and run
|-- package.json                  Dependencies and lifecycle scripts
`-- tsconfig.json                 TypeScript compiler settings
```

---

## Connection Lifecycle & Authentication

### 1. Connection Handshake
Clients initiate connections via `wss://<host>/socket.io/?token=<JWT>` or by sending the authorization token in the Socket.IO `auth` object:

```json
{
  "auth": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

### 2. Guard Authentication & Authorization
The `RealtimeAuthorizationService` decodes and verifies the token against the configured public key or `JWT_SECRET`. Unauthorized connections are immediately disconnected with error code `4403 (FORBIDDEN)`.

### 3. Room Membership
Upon successful verification, the socket joins:
- `user:<userId>`: Individual user private notification channel.
- `role:<driver|customer|admin>`: Role-specific broadcast channel.
- Dynamic delivery rooms: `delivery:<deliveryId>` when assigned to or tracking an order.

### 4. Heartbeat & Health Monitoring
- Heartbeat Interval: 25 seconds.
- Disconnect Timeout: 60 seconds.
- The `HeartbeatService` regularly sends ping frames. If a client fails to reply within the timeout threshold, its resources and subscriptions are cleaned up.

---

## WebSocket Protocol Specification

### Client to Server Events

#### `driver:location:update`
Sent by driver applications to stream current location:
```json
{
  "latitude": 30.0444,
  "longitude": 31.2357,
  "heading": 182.5,
  "speed": 42.0,
  "accuracy": 4.5,
  "timestamp": 1730847600000
}
```

#### `delivery:subscribe`
Subscribes a customer or support agent to live tracking updates for a delivery:
```json
{
  "deliveryId": "del_01HXYZ789ABC"
}
```

#### `delivery:unsubscribe`
Removes the socket from the delivery room:
```json
{
  "deliveryId": "del_01HXYZ789ABC"
}
```

### Server to Client Events

#### `delivery:location:stream`
Broadcasts updated driver coordinates to everyone in `delivery:<deliveryId>`:
```json
{
  "deliveryId": "del_01HXYZ789ABC",
  "driverId": "drv_9921",
  "latitude": 30.0448,
  "longitude": 31.2361,
  "heading": 185.0,
  "speed": 40.2,
  "estimatedArrival": "2026-11-05T21:45:00Z"
}
```

#### `delivery:status:changed`
Pushed when delivery state transitions occur:
```json
{
  "deliveryId": "del_01HXYZ789ABC",
  "previousStatus": "ASSIGNED",
  "currentStatus": "PICKED_UP",
  "updatedAt": "2026-11-05T21:30:00Z"
}
```

---

## GraphQL Subscriptions Reference

The service exports federated GraphQL subscriptions consumed by clients via the API Gateway:

```graphql
type Subscription {
  deliveryStatusUpdated(deliveryId: ID!): DeliveryEvent!
  driverLocationUpdated(deliveryId: ID!): DriverLocationPayload!
  userNotifications(userId: ID!): NotificationEvent!
}

type DriverLocationPayload {
  deliveryId: ID!
  driverId: ID!
  latitude: Float!
  longitude: Float!
  heading: Float
  speed: Float
  updatedAt: String!
}

type DeliveryEvent {
  deliveryId: ID!
  status: String!
  timestamp: String!
}
```

---

## Distributed Pub/Sub & Clustered Messaging

The service utilizes multiple broker backbones:

| Broker | Usage | Topic / Exchange Pattern | Persistence |
| :--- | :--- | :--- | :--- |
| **Redis** | High-velocity ephemereal broadcast across nodes | `realtime:channel:*` | In-memory |
| **NATS JetStream** | Cross-microservice event delivery & replay | `delivery.events.>`, `driver.location.>` | File-backed stream |
| **Apache Kafka** | Audit log ingestion and state machine events | `delivery.status.v1`, `payment.completed.v1` | Durable log |
| **RabbitMQ** | Asynchronous system-wide broadcast alerts | `broadcast.exchange` (fanout) | Durable queue |

---

## Environment Variables

| Variable | Description | Default | Required |
| :--- | :--- | :--- | :--- |
| `NODE_ENV` | Application environment (`development`, `production`, `test`) | `development` | Yes |
| `PORT` | HTTP and WebSocket server port | `3004` | No |
| `METRICS_PORT` | Prometheus metrics scrape port | `9104` | No |
| `JWT_SECRET` | Secret key for verifying JWT tokens | - | Yes |
| `REDIS_HOST` | Redis host for Pub/Sub and session cache | `localhost` | Yes |
| `REDIS_PORT` | Redis port | `6379` | No |
| `REDIS_PASSWORD` | Optional Redis authentication password | - | No |
| `NATS_URL` | NATS cluster connection string | `nats://localhost:4222` | Yes |
| `KAFKA_BROKERS` | Comma-separated Kafka broker addresses | `localhost:9092` | Yes |
| `KAFKA_GROUP_ID` | Kafka consumer group name | `realtime-service-group` | No |
| `RABBITMQ_URL` | RabbitMQ connection URI | `amqp://guest:guest@localhost:5672` | No |
| `WS_MAX_CONNECTIONS` | Maximum simultaneous open connections per instance | `10000` | No |

---

## Local Development & Operations

### Prerequisites
- Node.js 20+ and npm 10+
- Docker and Docker Compose
- GNU Make

### Setup and Start
```bash
# 1. Start required infrastructure (Redis, NATS, Kafka)
make infra-up

# 2. Install dependencies
make install

# 3. Start development server with hot-reload
make start-dev
```

### Production Build
```bash
make build
make start-prod
```

---

## Testing

The service maintains full unit and end-to-end test suites:

```bash
# Run unit tests
make test

# Run unit tests with code coverage
make test-cov

# Run end-to-end integration tests
make test-e2e

# Run all test suites
make test-all
```

---

## Makefile Targets

| Target | Description |
| :--- | :--- |
| `make help` | Display all available targets with descriptions |
| `make install` | Install project dependencies |
| `make build` | Compile the TypeScript project to JavaScript in `dist/` |
| `make start-dev` | Start the service in development watch mode |
| `make start-prod` | Execute production compiled code from `dist/main` |
| `make test` | Execute Jest unit tests |
| `make test-cov` | Execute unit tests and output coverage summary |
| `make test-e2e` | Run end-to-end integration tests |
| `make lint` | Run ESLint and apply automatic fixes |
| `make format` | Check and format code with Prettier |
| `make typecheck` | Validate TypeScript types without emit |
| `make infra-up` | Launch local Docker containers for Redis, NATS, and Kafka |
| `make infra-down` | Stop and remove local Docker containers |
| `make clean` | Delete build outputs, test coverage, and temporary caches |
