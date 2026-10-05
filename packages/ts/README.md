# @delivery-micro/shard

This package serves as the core shared library for all microservices in the Realtime Delivery architecture. It provides centralized utilities, integrations, and configurations to ensure consistency, reduce code duplication, and enforce enterprise standards across the entire system.

## Table of Contents
1. [Overview](#overview)
2. [Relationship with Microservices](#relationship-with-microservices)
3. [Architecture and Modules](#architecture-and-modules)
4. [Installation and Usage](#installation-and-usage)
5. [Testing and Coverage](#testing-and-coverage)

## Overview

The `@delivery-micro/shard` package contains highly tested and optimized shared logic. Instead of each microservice reinventing the wheel for common tasks such as logging, authentication, metrics collection, or message broker communication, they import the unified implementations from this package.

## Relationship with Microservices

This library is intricately linked to the applications located in the `services/` directory (e.g., `realtime-service`, `auth-service`, `order-service`). 

- **Dependency Injection**: Microservices import NestJS modules (e.g., `RabbitMQModule`, `KafkaModule`, `LoggerModule`) directly from this package into their root `AppModule`.
- **Standardized Communication**: Services use the provided gRPC and GraphQL filters and interceptors to ensure that all responses and exceptions conform to a strict, unified contract across the entire infrastructure.
- **Message Broker Unification**: Consumer and Publisher logic for RabbitMQ, Kafka, and NATS are abstracted here, allowing any service in the `services/` directory to communicate securely and reliably without maintaining boilerplate connection logic.
- **Shared Security**: Role-based access control, rate limiting, and JWT strategies are housed here and applied at the microservice level to protect endpoints consistently.

## Architecture and Modules

The source code (`src/`) is divided into domain-specific modules:

### 1. Automation and Health
- Provides `HealthService` and `AlertService` for infrastructure monitoring.
- Integrates with `@nestjs/terminus` for liveness and readiness probes.

### 2. Authentication and Guards
- **Decorators**: `@Auth`, `@CurrentUser`, `@RedisRateLimit`.
- **Guards**: `RoleGuard`, `RateLimiterGuard`, `RedisRateLimiterGuard`.
- Ensures zero-trust security and granular permission checks across all services.

### 3. Message Brokers
- **RabbitMQ**: Contains `RabbitMQService`, `RabbitMQConsumer`, and robust health indicators.
- **Kafka**: Features `KafkaService` and an abstract `BaseKafkaConsumer` for extending in individual microservices.
- **NATS**: Contains `NatsService` and typed event definitions.

### 4. Metrics and Logging
- **Metrics**: Exposes Prometheus histograms and counters via `MetricsService` and `MetricsInterceptor`.
- **Logging**: A unified `LoggerService` with request context tracking (`LoggerContext`) and middleware to log incoming requests across all services.

### 5. WebSockets
- Specialized `WsAdapter` to handle realtime connections.
- Contains WebSocket-specific JWT strategies, exception filters, and guard chains to bring HTTP-level security to stateful WebSocket connections.

### 6. Interceptors and Filters
- **Filters**: `GrpcExceptionFilter` and `GraphqlExceptionFilter` for converting raw backend errors into standardized front-facing error envelopes.
- **Interceptors**: `ResponseFormatter` and `GraphqlResponseInterceptor` for mapping responses into standard JSON structures.

## Installation and Usage

To use this package within any microservice under the `services/` directory, ensure it is added to the microservice's `package.json` dependencies. Since this is an internal package, it is managed in a monorepo setup.

```bash
npm install @delivery-micro/shard
```

Import modules into a microservice:

```typescript
import { LoggerModule, MetricsModule } from '@delivery-micro/shard';

@Module({
  imports: [
    LoggerModule,
    MetricsModule,
    // ...other microservice imports
  ],
})
export class AppModule {}
```

## Testing and Coverage

This package maintains a strict standard for testing to ensure that common components do not introduce regressions into the consuming microservices.

- **Unit Tests**: Powered by Jest. Every module, service, guard, and filter is accompanied by a `.spec.ts` file.
- **Code Coverage**: The package enforces a strict 90%+ code coverage threshold across lines, functions, statements, and branches. Mocks are extensively used to isolate external I/O operations (like AMQP or Kafka connections).

To run the test suite locally:

```bash
# Run all tests
npm run test

# Generate coverage report
npm run test:cov
```
