# Delivery Microservices Go Shared Library

This repository contains the shared Go packages used across the Realtime Delivery microservices ecosystem. It provides common utilities, middleware, and integrations to ensure consistency, security, and performance across all Go-based microservices.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Packages](#packages)
  - [auth](#auth)
  - [automation](#automation)
  - [constants](#constants)
  - [env](#env)
  - [events](#events)
  - [kafka](#kafka)
  - [logging](#logging)
  - [metrics](#metrics)
  - [middleware](#middleware)
  - [nats](#nats)
  - [rabbitmq](#rabbitmq)
  - [ratelimiter](#ratelimiter)
  - [snowflake](#snowflake)
  - [users](#users)
  - [websocket](#websocket)
- [Testing and Coverage](#testing-and-coverage)
- [Contributing](#contributing)

## Overview

The `packages/go` module acts as the core library for all backend microservices written in Go. By centralizing logic such as authentication, logging, metrics, and message broker integrations, we prevent code duplication and enforce architectural standards. 

## Installation

To use this shared library within another microservice, import it using its module path:

```go
import "delivery-micro/packages/go/<package_name>"
```

Ensure your `go.mod` file correctly references this local workspace or remote repository.

## Packages

### auth

Provides authentication and authorization utilities, including JWT token parsing, validation, and role-based access control (RBAC) mechanisms. It includes generic interfaces that allow services to verify user identity securely.

### automation

Contains health check services, background task scheduling, and automated scripts used for maintenance and monitoring of microservices.

### constants

A centralized repository for system-wide constants. This includes error codes, configuration keys, and domain-specific enumerations used across multiple services to avoid hardcoding.

### env

A configuration management package responsible for parsing, validating, and loading environment variables. It ensures that services fail fast on startup if required configuration values are missing.

### events

Defines standard event structures and payloads (e.g., Domain Events, Integration Events). It acts as the contract for asynchronous communication between microservices.

### kafka

A wrapper and abstraction over Apache Kafka client libraries. It provides interfaces and implementations for producing and consuming messages, handling retries, and managing partitions securely. 

### logging

Provides a standardized structured logging solution (e.g., utilizing Zap or Logrus). It supports context propagation, allowing distributed tracing IDs and metadata to be injected automatically into log entries.

### metrics

Provides instrumentation tools for Prometheus and Grafana. It includes helper functions for recording HTTP request durations, custom counters, and infrastructure gauges.

### middleware

HTTP middleware components for web servers (e.g., Gin, Fiber, or standard net/http). Includes middlewares for logging, error handling, CORS, rate limiting integration, and tracing.

### nats

Integration package for NATS message broker. It provides robust client abstractions for publish-subscribe and request-reply messaging patterns used for high-throughput microservice communication.

### rabbitmq

Provides AMQP protocol integration via RabbitMQ. Includes wrappers for reliable connection management, channel pooling, and handling dead-letter queues.

### ratelimiter

Implements API rate limiting algorithms (such as Token Bucket or Fixed Window) utilizing Redis. It prevents abuse and ensures fair usage of service resources.

### snowflake

Generates distributed, time-sortable, and collision-resistant unique IDs based on the Twitter Snowflake algorithm. Essential for database primary keys and message tracking IDs.

### users

Contains shared data transfer objects (DTOs), models, and standard validation logic specifically related to User domains across the platform.

### websocket

Provides real-time, full-duplex communication infrastructure over WebSockets. Includes connection management, broadcast mechanisms, and client session tracking.

## Testing and Coverage

This repository adheres to a strict 100% unit testing and coverage masterplan. All business logic, interfaces, and shared utilities must have accompanying tests. Mock implementations are utilized for external systems like Redis, Kafka, RabbitMQ, and NATS.

To run tests and check coverage:

```bash
go test ./... -covermode=atomic -coverprofile=coverage.out
go tool cover -func=coverage
```

Our CI pipeline enforces an 85% minimum total coverage threshold via GitHub Actions (`common-unit-tests.yml`).

## Contributing

When adding new packages or modifying existing ones:

1. Maintain high cohesion and low coupling.
2. Ensure you depend on interfaces rather than concrete implementations for external dependencies to facilitate unit testing.
3. Keep the code free of unnecessary comments.
4. Always provide 100% unit test coverage for new logic.
