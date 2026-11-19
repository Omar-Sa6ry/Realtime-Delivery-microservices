# User Service

## Overview

The User Service is a microservice in the Real-time Delivery platform architecture. It is built with NestJS 11 and TypeScript, functioning as an Apollo Federation Subgraph for the API Gateway and exposing gRPC, REST health/metrics endpoints, BullMQ outbox event processing, and event-driven integration over RabbitMQ and Kafka.

The service manages the full user lifecycle, including user registration, authentication, role-based access control (RBAC), profile maintenance, address book operations, session lifecycle in Redis, transactional outbox pattern dispatching, and integration with the Media service for avatars.

---

## Architectural Characteristics

- Framework: NestJS 11 on Node.js
- API Protocol: GraphQL (Apollo Federation 2 subgraph) via Apollo Server Fastify/Express integration
- Inter-Service RPC: gRPC for high-performance internal communication (User service proto definitions and Media service client)
- Primary Database: PostgreSQL managed via TypeORM with entity relations and transactional support
- Cache and Session Store: Redis via @bts-soft/core and redis-session.repository
- Asynchronous Messaging:
  - Transactional Outbox pattern backed by PostgreSQL `outbox` table
  - Queue processing via BullMQ
  - Event streaming via Apache Kafka (`@delivery-micro/shard` topics)
  - Broadcast and domain event fan-out via RabbitMQ
- Security:
  - Passwords hashed using bcrypt (10 rounds)
  - Access and refresh tokens generated via JSON Web Tokens (JWT)
  - Role-based authorization (`ADMIN`, `USER`, `DRIVER`, etc.)
- Localization: Multi-language error messages and responses via `nestjs-i18n` (supporting English and Arabic)
- Observability: Prometheus metrics (`/user/metrics`), Liveness probe (`/user/health`), Readiness probe (`/user/ready`)

---

## Project Structure

```
services/user-service/
├── src/
│   ├── app.module.ts                         # Root application module with Federation and config setup
│   ├── app.resolver.ts                       # Root-level GraphQL queries (e.g. pingForUser)
│   ├── graphql-schema.spec.ts                # Schema federation tests
│   ├── main.ts                               # Service bootstrap (Fastify/Express, gRPC microservice listener)
│   ├── common/
│   │   ├── common.module.ts                  # Core shared module
│   │   ├── database/
│   │   │   ├── entities/
│   │   │   │   ├── user.entity.ts            # User entity definition, roles, statuses, and relations
│   │   │   │   ├── address.entity.ts         # User address book entity
│   │   │   │   ├── outbox.entity.ts          # Transactional outbox entity for event reliability
│   │   │   │   └── entities.meta.ts          # TypeORM entities metadata collector
│   │   │   └── repositories/
│   │   │       └── redis-session.repository.ts # Redis session tracking and token management
│   │   ├── messaging/
│   │   │   ├── outbox.queue.ts               # BullMQ queue provider for outbox events
│   │   │   └── outbox-worker.service.ts      # Worker consuming outbox table and publishing downstream
│   │   ├── security/
│   │   │   ├── bcrypt-password.hasher.ts     # Bcrypt implementation for password hashing and verification
│   │   │   └── jwt-token.provider.ts         # JWT signer and verification provider
│   │   └── translation/
│   │       ├── translation.module.ts         # NestJS i18n module registration
│   │       └── locales/
│   │           ├── en/user.json              # English localized strings and validation errors
│   │           └── ar/user.json              # Arabic localized strings and validation errors
│   └── modules/
│       ├── auth/
│       │   ├── auth.module.ts                # Authentication module configuration
│       │   ├── auth.resolver.ts              # GraphQL mutations (register, login, refreshToken, etc.)
│       │   ├── auth.service.ts               # Core authentication business logic and workflows
│       │   ├── user.factory.ts               # Factory creating user aggregates and initial state
│       │   └── dto/
│       │       └── auth.types.ts             # GraphQL inputs, args, and response envelopes for Auth
│       ├── media/
│       │   ├── media.module.ts               # Media gRPC client module
│       │   └── grpc-media.service.ts         # Client communicating with Media microservice for avatar URLs
│       ├── rabbitmq/
│       │   ├── rabbitmq.module.ts            # RabbitMQ producer module
│       │   └── rabbitmq.publisher.ts         # RabbitMQ topic and exchange publisher
│       └── user/
│           ├── user.module.ts                # User domain module
│           ├── user.resolver.ts              # GraphQL queries and mutations (profiles, addresses, admin ops)
│           ├── user.service.ts               # Service layer coordinating repositories, caching, and events
│           ├── db-user.service.ts            # Direct PostgreSQL data access layer
│           ├── user-grpc.controller.ts       # gRPC endpoint controller for user queries across microservices
│           ├── health.controller.ts          # REST liveness and readiness probes
│           ├── metrics.controller.ts         # Prometheus metrics exposition endpoint
│           └── dto/
│               └── user.types.ts             # GraphQL inputs, objects, and types for User and Address
├── test/
│   ├── auth.e2e-spec.ts                      # End-to-end suite covering full authentication workflows
│   ├── health.e2e-spec.ts                    # End-to-end suite covering health, ready, metrics, and boot
│   ├── user.e2e-spec.ts                      # End-to-end suite covering user profiles, addresses, and admin ops
│   ├── jest-e2e.json                         # Jest configuration for E2E testing
│   ├── test-app.module.ts                    # NestJS testing root module configuration
│   └── support/
│       ├── harness.ts                        # E2E test harness, database bootstrap, and client helpers
│       ├── test-env.ts                       # Environment variable defaults for test execution
│       ├── fake-redis.ts                     # In-memory Redis mock for isolated integration tests
│       └── fakes.ts                          # Test doubles for Kafka, RabbitMQ, Media gRPC, and Notifications
├── Dockerfile                                # Multi-stage container build configuration
├── docker-compose.yml                        # Local dependencies stack (PostgreSQL, Redis, NATS, App)
├── Makefile                                  # Automation targets for development, testing, and operations
├── package.json                              # Package definitions, scripts, and dependencies
└── tsconfig.json                             # TypeScript compiler configuration
```

---

## Environment Configuration

Configure the service by providing the following environment variables (or by using the project configuration files located under `config/env/.env.<ENV>`):

| Variable | Description | Default / Example |
| :--- | :--- | :--- |
| `NODE_ENV` | Runtime environment mode | `development` |
| `PORT` | HTTP/GraphQL listening port | `4001` |
| `GRPC_PORT` | gRPC server listening port | `50051` |
| `DB_HOST` | PostgreSQL host | `127.0.0.1` |
| `DB_PORT` | PostgreSQL port | `5433` |
| `DB_USER` | PostgreSQL username | `postgres` |
| `DB_PASSWORD` | PostgreSQL password | `postgres` |
| `DB_NAME` | PostgreSQL database name | `delivery_user_db` |
| `REDIS_HOST` | Redis host | `127.0.0.1` |
| `REDIS_PORT` | Redis port | `6379` |
| `JWT_SECRET` | Secret key for JWT signing | `change-me-in-production` |
| `JWT_ACCESS_EXPIRES_IN` | Access token expiration period | `15m` |
| `JWT_REFRESH_EXPIRES_IN` | Refresh token expiration period | `7d` |
| `KAFKA_BROKERS` | Kafka broker endpoints | `localhost:9092` |
| `RABBITMQ_URL` | RabbitMQ AMQP connection URI | `amqp://guest:guest@localhost:5672` |
| `MEDIA_GRPC_URL` | Media service gRPC address | `localhost:50052` |

---

## GraphQL API Schema and Capabilities

The service integrates into Apollo Federation 2. Key operations include:

### Authentication Queries and Mutations
- `register(input: RegisterInput!)`: Registers a new user. Automatically designates the very first registered user in an empty database as `ADMIN`. Generates access and refresh tokens, creates an outbox event, and stores the active session in Redis.
- `login(input: LoginInput!)`: Validates user credentials, updates session tokens, and returns user profile data.
- `refreshToken(refreshToken: String!)`: Validates and rotates refresh tokens, invalidating expired tokens.
- `logout`: Revokes the current session and purges the token from Redis.
- `requestPasswordReset(email: String!)`: Generates a password reset token and sends an email notification.
- `resetPassword(input: ResetPasswordInput!)`: Verifies the reset token and updates the password hash.

### User and Profile Queries and Mutations
- `myProfile`: Retrieves the authenticated user's profile and addresses (backed by a read-through Redis cache).
- `updateProfile(input: UpdateProfileInput!)`: Updates user details. If an avatar media ID is provided, coordinates with the Media gRPC service to resolve the storage URL.
- `user(id: ID!)`: Admin-only query to inspect any registered user account.
- `users(page: Int, limit: Int)`: Admin-only paginated listing of users.
- `changeUserStatus(userId: ID!, isActive: Boolean!)`: Admin mutation to activate or deactivate user accounts.

### Address Book Mutations
- `addAddress(input: CreateAddressInput!)`: Adds an address to the user profile. Supports setting default delivery addresses.
- `updateAddress(id: ID!, input: UpdateAddressInput!)`: Modifies an existing address belonging to the user.
- `deleteAddress(id: ID!)`: Removes an address from the user profile.

### Apollo Federation Entity Resolvers
- `_entities`: Resolves federated User entity representations across microservices (such as Order, Delivery, and Analytics services).

---

## gRPC Services

The service implements the user gRPC proto definition for synchronous cross-service queries:
- `FindUserById(UserLookupRequest)`: Returns user identity, contact details, role, and status.
- `ValidateToken(TokenRequest)`: Internal validation service for microservices that do not verify tokens independently.

---

## Background Processing and Reliable Messaging

### Transactional Outbox Pattern
1. State changes and domain events are persisted in the same PostgreSQL transaction within the `outbox` table.
2. An event is enqueued into BullMQ (`outbox-worker.service.ts`).
3. The worker picks up the job, retrieves the unhandled event from PostgreSQL, and broadcasts it to:
   - Kafka topic: `user.created`, `user.updated`, `user.deleted`
   - RabbitMQ exchange: domain event fan-out
4. Upon successful delivery, the worker marks the outbox record as `processed = true`.

---

## Testing and Quality Assurance

The service is engineered with high test coverage standards covering 100% of statements, functions, and lines across core domain logic.

### Unit Tests
The test suite includes 30 test suites with 406 test cases covering:
- Authentication, token rotation, and password hashing
- Validation pipes, input decorators, and internationalization in English and Arabic
- User resolvers, mutations, queries, and role-based guards
- TypeORM database services, transaction rollback handling, and retry logic
- Media gRPC client integration and transient failure handling
- Redis session caching and BullMQ outbox worker execution

Run unit tests with coverage:
```bash
npm test -- --coverage
```

### End-to-End (E2E) Tests
Located in the `test/` directory:
- `auth.e2e-spec.ts`: Complete authentication, token refresh, and registration flows against real PostgreSQL and mocked messaging backends.
- `user.e2e-spec.ts`: Profile management, address book operations, admin role checks, and Apollo Federation entity resolution.
- `health.e2e-spec.ts`: System liveness (`/user/health`), readiness (`/user/ready`), metrics (`/user/metrics`), and GraphQL health ping.

Run E2E tests:
```bash
npm run test:e2e
```

---

## Makefile Automation Commands

The service contains a dedicated `Makefile` to streamline local development, Docker operations, and CI workflows:

```bash
make help                 # Display all available targets
make install              # Install dependencies with npm install --legacy-peer-deps
make install-ci           # Reproducible dependency installation via npm ci
make build                # Compile TypeScript source code into dist/
make start-dev            # Launch development server with hot-reload
make test                 # Run all Jest unit tests
make test-cov             # Run unit tests and generate coverage report
make test-e2e             # Execute end-to-end test suite
make verify               # Execute linting, formatting check, type checking, build, and all tests
make infra-up             # Start local dependencies via Docker (PostgreSQL, Redis, NATS)
make infra-down           # Stop local dependencies
make docker-build         # Build the user-service Docker image
make docker-up            # Run user-service in Docker Compose
make docker-down          # Stop and remove Docker Compose containers
make smoke                # Perform HTTP health, readiness, and metrics smoke tests
make clean                # Remove build artifacts, coverage reports, and caches
```

---

## API Gateway Delegation

All user-service targets can also be invoked directly from the root API Gateway via `services/api-gateway/Makefile`:

```bash
make user-test            # Run user-service unit tests from api-gateway
make user-test-cov        # Run user-service coverage from api-gateway
make user-test-e2e        # Run user-service e2e tests from api-gateway
make user-verify          # Run full user-service verification pipeline from api-gateway
make user-smoke           # Probe running user-service health endpoints
make verify-all           # Verify both api-gateway and user-service in a single command
```

---

## Observability and Operations

### Health Probes
- `GET /user/health`: Liveness probe. Returns HTTP 200 with service uptime, memory allocations (RSS and Heap), and CPU load.
- `GET /user/ready`: Readiness probe. Returns HTTP 200 when PostgreSQL and Redis are both reachable; returns HTTP 503 if any required subsystem is disconnected.

### Metrics
- `GET /user/metrics`: Prometheus metric exposition endpoint detailing request rates, latency histograms, database query duration, and session metrics.

---

## License

UNLICENSED - Proprietary internal delivery microservice.
