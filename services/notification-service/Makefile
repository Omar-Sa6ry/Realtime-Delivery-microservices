# =============================================================================
# Notification Service - Makefile
# =============================================================================
# Comprehensive task runner for the Notification Service microservice.
# Works on Windows (cmd.exe via ezwinports make) and POSIX shells.
#
# Usage:
#   make help
#   make install
#   make verify          # build + unit coverage + e2e (full gate)
#   make up              # docker compose up (infra + service)
#
# All npm/docker/node commands are resolved from PATH.
# =============================================================================

.PHONY: help install install-fresh build rebuild start start-dev start-debug start-prod \
        test test-watch test-cov test-e2e test-all test-debug \
        lint format format-check typecheck \
        clean clean-dist clean-coverage clean-node-modules clean-all \
        up down up-build down-volumes logs ps restart infra-up infra-down infra-logs \
        db-up redis-up nats-up kafka-up zookeeper-up \
        health ready metrics curl-health curl-ready curl-metrics \
        proto-check env-check ports check-deps verify ci docker-build docker-shell docker-prune \
        db-shell redis-cli seed-templates

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------
SHELL := cmd.exe
MAKEFLAGS += --no-print-directory

APP_NAME          := notification-service
APP_PORT          := 4004
GRPC_PORT         := 50053
HEALTH_URL        := http://localhost:$(APP_PORT)/notification/health
READY_URL         := http://localhost:$(APP_PORT)/notification/ready
METRICS_URL       := http://localhost:$(APP_PORT)/notification/metrics
GRAPHQL_URL       := http://localhost:$(APP_PORT)/notification/graphql
ENV               ?= development
ENV_FILE          := ..\..\config\env\.env.$(ENV)
COMPOSE_FILE      := docker-compose.yml
COMPOSE           := docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE)
NPM               := npm
NODE              := node
DOCKER            := docker
PROTO_FILE        := ..\..\protos\notification.proto
USER_PROTO_FILE   := ..\..\protos\user.proto

# -----------------------------------------------------------------------------
# Help (default target)
# -----------------------------------------------------------------------------
help:
	@echo.
	@echo ============================================================
	@echo  Notification Service - Available Targets
	@echo ============================================================
	@echo.
	@echo  Setup
	@echo    make install            Install dependencies (legacy peer deps)
	@echo    make install-fresh      Remove node_modules + lockfile, then install
	@echo    make check-deps         Verify node, npm, docker are on PATH
	@echo.
	@echo  Run
	@echo    make start              Start once (nest start)
	@echo    make start-dev          Watch mode (nest start --watch)
	@echo    make start-debug        Debug + watch (inspect-brk)
	@echo    make start-prod         Run compiled dist/main
	@echo.
	@echo  Build
	@echo    make build              nest build (tsc) to dist/
	@echo    make rebuild            Clean dist then build
	@echo    make typecheck          tsc --noEmit on production sources (excludes specs)
	@echo.
	@echo  Test
	@echo    make test               Unit tests (no coverage)
	@echo    make test-watch         Unit tests watch mode
	@echo    make test-cov           Unit tests with coverage thresholds
	@echo    make test-e2e           E2E tests (supertest + GraphQL + gRPC)
	@echo    make test-all           Unit coverage + E2E (full suite)
	@echo    make test-debug         Unit tests under node --inspect-brk
	@echo.
	@echo  Quality
	@echo    make lint               ESLint with --fix
	@echo    make format             Prettier write (src + test)
	@echo    make format-check       Prettier check (no write)
	@echo    make verify             build + test-cov + test-e2e (full gate)
	@echo    make ci                 install + verify (CI pipeline)
	@echo.
	@echo  Docker / Infra
	@echo    make up                 Compose up (infra + service), detached
	@echo    make up-build           Compose up with --build
	@echo    make down               Compose down
	@echo    make down-volumes       Compose down with volumes removed
	@echo    make restart            Compose restart
	@echo    make logs               Tail service logs
	@echo    make ps                 Show running containers
	@echo    make docker-build       Build only the service image
	@echo    make docker-shell       Open a shell inside the service container
	@echo    make docker-prune       Remove dangling images/containers
	@echo    make infra-up           Start infra only (db, redis, nats, kafka, zk)
	@echo    make infra-down         Stop infra only
	@echo    make infra-logs         Tail infra logs
	@echo    make db-up              Start Postgres only
	@echo    make redis-up           Start Redis only
	@echo    make nats-up            Start NATS only
	@echo    make kafka-up           Start Kafka + Zookeeper only
	@echo    make zookeeper-up       Start Zookeeper only
	@echo    make db-shell           psql shell inside Postgres container
	@echo    make redis-cli          redis-cli inside Redis container
	@echo.
	@echo  Operations / Health
	@echo    make health             GET /notification/health
	@echo    make ready              GET /notification/ready
	@echo    make metrics            GET /notification/metrics
	@echo    make curl-health        Alias for health
	@echo    make curl-ready         Alias for ready
	@echo    make curl-metrics       Alias for metrics
	@echo.
	@echo  Checks
	@echo    make proto-check        Verify gRPC proto files exist
	@echo    make env-check          Verify env file exists for ENV=$(ENV)
	@echo    make ports              Show which ports the service uses
	@echo.
	@echo  Clean
	@echo    make clean              Remove dist + coverage
	@echo    make clean-dist         Remove dist/
	@echo    make clean-coverage     Remove coverage/
	@echo    make clean-node-modules Remove node_modules/
	@echo    make clean-all          dist + coverage + node_modules
	@echo.
	@echo  Variables: ENV=development, test, or production (default: development)
	@echo  Examples:  make up ENV=development
	@echo             make verify
	@echo             make logs
	@echo.

# -----------------------------------------------------------------------------
# Setup
# -----------------------------------------------------------------------------

check-deps:
	@echo [check-deps] Verifying required tools on PATH...
	@where node >nul 2>&1 || (echo   node: MISSING & exit /b 1)
	@echo   node: OK
	@where npm >nul 2>&1 || (echo   npm: MISSING & exit /b 1)
	@echo   npm: OK
	@where docker >nul 2>&1 && echo   docker: OK || echo   docker: MISSING - optional for tests
	@echo [check-deps] Done.

install:
	@echo [install] Installing dependencies (npm install --legacy-peer-deps)...
	$(NPM) install --legacy-peer-deps
	@echo [install] Done.

install-fresh:
	@echo [install-fresh] Removing node_modules and package-lock.json...
	if exist node_modules rmdir /s /q node_modules
	if exist package-lock.json del /q package-lock.json
	@echo [install-fresh] Reinstalling...
	$(NPM) install --legacy-peer-deps
	@echo [install-fresh] Done.

# -----------------------------------------------------------------------------
# Run
# -----------------------------------------------------------------------------

start:
	@echo [start] Starting $(APP_NAME) (one-shot)...
	$(NPM) run start

start-dev:
	@echo [start-dev] Starting $(APP_NAME) in watch mode...
	$(NPM) run start:dev

start-debug:
	@echo [start-debug] Starting $(APP_NAME) in debug + watch mode...
	$(NPM) run start:debug

start-prod:
	@echo [start-prod] Running compiled dist/main...
	$(NPM) run start:prod

# -----------------------------------------------------------------------------
# Build
# -----------------------------------------------------------------------------

build:
	@echo [build] Building $(APP_NAME) (nest build)...
	$(NPM) run build
	@echo [build] Done. Output in dist/

rebuild: clean-dist build

typecheck:
	@echo [typecheck] Running tsc --noEmit on production sources...
	$(NPM) exec -- tsc --noEmit -p tsconfig.build.json
	@echo [typecheck] No type errors.

# -----------------------------------------------------------------------------
# Test
# -----------------------------------------------------------------------------

test:
	@echo [test] Running unit tests...
	$(NPM) run test

test-watch:
	@echo [test-watch] Running unit tests in watch mode...
	$(NPM) run test:watch

test-cov:
	@echo [test-cov] Running unit tests with coverage thresholds...
	$(NPM) run test:cov

test-e2e:
	@echo [test-e2e] Running E2E tests...
	$(NPM) run test:e2e

test-all: test-cov test-e2e
	@echo [test-all] Full suite passed (unit coverage + e2e).

test-debug:
	@echo [test-debug] Running unit tests under inspector...
	$(NPM) run test:debug

# -----------------------------------------------------------------------------
# Quality
# -----------------------------------------------------------------------------

lint:
	@echo [lint] Running ESLint with --fix...
	$(NPM) run lint
	@echo [lint] Done.

format:
	@echo [format] Running Prettier write...
	$(NPM) run format
	@echo [format] Done.

format-check:
	@echo [format-check] Running Prettier check...
	$(NPM) exec -- prettier --check "src/**/*.ts" "test/**/*.ts"
	@echo [format-check] Done.

verify: build test-cov test-e2e
	@echo.
	@echo ============================================================
	@echo  VERIFY PASSED: build + unit coverage + e2e all green
	@echo ============================================================

ci: check-deps install verify
	@echo.
	@echo ============================================================
	@echo  CI PASSED: deps + build + coverage + e2e all green
	@echo ============================================================

# -----------------------------------------------------------------------------
# Docker / Infra
# -----------------------------------------------------------------------------

up:
	@echo [up] docker compose up -d (infra + service)...
	$(COMPOSE) up -d
	@echo [up] Done. Service: $(GRAPHQL_URL) -- Health: $(HEALTH_URL)

up-build:
	@echo [up-build] docker compose up --build -d...
	$(COMPOSE) up --build -d
	@echo [up-build] Done.

down:
	@echo [down] docker compose down...
	$(COMPOSE) down
	@echo [down] Done.

down-volumes:
	@echo [down-volumes] docker compose down -v...
	$(COMPOSE) down -v
	@echo [down-volumes] Done.

restart:
	@echo [restart] docker compose restart...
	$(COMPOSE) restart
	@echo [restart] Done.

logs:
	@echo [logs] Tailing service logs (Ctrl+C to stop)...
	$(COMPOSE) logs -f notification-service

ps:
	@echo [ps] Running containers:
	$(COMPOSE) ps

docker-build:
	@echo [docker-build] Building $(APP_NAME) image...
	$(DOCKER) build -t $(APP_NAME):local -f Dockerfile ../..
	@echo [docker-build] Done. Image: $(APP_NAME):local

docker-shell:
	@echo [docker-shell] Opening shell inside service container...
	$(DOCKER) exec -it notification-service sh

docker-prune:
	@echo [docker-prune] Removing dangling docker resources...
	$(DOCKER) container prune -f
	$(DOCKER) image prune -f
	@echo [docker-prune] Done.

infra-up:
	@echo [infra-up] Starting infrastructure only (db, redis, nats, kafka, zookeeper)...
	$(COMPOSE) up -d notification-db-srv redis-srv nats-srv kafka-srv zookeeper-srv
	@echo [infra-up] Done.

infra-down:
	@echo [infra-down] Stopping infrastructure...
	$(COMPOSE) stop notification-db-srv redis-srv nats-srv kafka-srv zookeeper-srv
	@echo [infra-down] Done.

infra-logs:
	@echo [infra-logs] Tailing infra logs (Ctrl+C to stop)...
	$(COMPOSE) logs -f notification-db-srv redis-srv nats-srv kafka-srv zookeeper-srv

db-up:
	@echo [db-up] Starting Postgres...
	$(COMPOSE) up -d notification-db-srv

redis-up:
	@echo [redis-up] Starting Redis...
	$(COMPOSE) up -d redis-srv

nats-up:
	@echo [nats-up] Starting NATS...
	$(COMPOSE) up -d nats-srv

kafka-up:
	@echo [kafka-up] Starting Kafka + Zookeeper...
	$(COMPOSE) up -d zookeeper-srv kafka-srv

zookeeper-up:
	@echo [zookeeper-up] Starting Zookeeper...
	$(COMPOSE) up -d zookeeper-srv

db-shell:
	@echo [db-shell] Opening psql shell inside Postgres...
	$(DOCKER) exec -it notification-db-srv psql -U delivery -d delivery_notification_db

redis-cli:
	@echo [redis-cli] Opening redis-cli shell...
	$(DOCKER) exec -it redis-srv redis-cli

# -----------------------------------------------------------------------------
# Operations / Health
# -----------------------------------------------------------------------------

health:
	@echo [health] GET $(HEALTH_URL)
	@curl -sf --max-time 5 $(HEALTH_URL) && echo. && echo [health] OK || echo [health] Service not reachable on port $(APP_PORT) - is it running?

ready:
	@echo [ready] GET $(READY_URL)
	@curl -sf --max-time 5 $(READY_URL) && echo. && echo [ready] OK || echo [ready] Service not reachable on port $(APP_PORT) - is it running?

metrics:
	@echo [metrics] GET $(METRICS_URL)
	@curl -sf --max-time 5 $(METRICS_URL) >NUL && echo [metrics] OK - metrics endpoint reachable || echo [metrics] Service not reachable on port $(APP_PORT) - is it running?

curl-health: health
curl-ready: ready
curl-metrics: metrics

# -----------------------------------------------------------------------------
# Checks
# -----------------------------------------------------------------------------

proto-check:
	@echo [proto-check] Verifying gRPC proto files...
	@if exist $(PROTO_FILE) (echo   notification.proto: OK) else (echo   notification.proto: MISSING & exit /b 1)
	@if exist $(USER_PROTO_FILE) (echo   user.proto: OK) else (echo   user.proto: MISSING & exit /b 1)
	@echo [proto-check] Done.

env-check:
	@echo [env-check] Verifying env file for ENV=$(ENV)...
	@if exist $(ENV_FILE) (echo   $(ENV_FILE): OK) else (echo   $(ENV_FILE): MISSING & exit /b 1)
	@echo [env-check] Done.

ports:
	@echo.
	@echo  $(APP_NAME) ports:
	@echo    HTTP / GraphQL : $(APP_PORT)  --  $(GRAPHQL_URL)
	@echo    gRPC           : $(GRPC_PORT)
	@echo    Health         : $(HEALTH_URL)
	@echo    Ready          : $(READY_URL)
	@echo    Metrics        : $(METRICS_URL)
	@echo    Postgres (map) : 5434
	@echo    Redis (map)    : 6379
	@echo    NATS (map)     : 4222 / 8222
	@echo    Kafka (map)    : 9092
	@echo    Zookeeper      : 2181
	@echo.

# -----------------------------------------------------------------------------
# Clean
# -----------------------------------------------------------------------------

clean: clean-dist clean-coverage
	@echo [clean] dist and coverage removed.

clean-dist:
	@echo [clean-dist] Removing dist/...
	if exist dist rmdir /s /q dist
	@echo [clean-dist] Done.

clean-coverage:
	@echo [clean-coverage] Removing coverage/...
	if exist coverage rmdir /s /q coverage
	@echo [clean-coverage] Done.

clean-node-modules:
	@echo [clean-node-modules] Removing node_modules/...
	if exist node_modules rmdir /s /q node_modules
	@echo [clean-node-modules] Done.

clean-all: clean-dist clean-coverage clean-node-modules
	@echo [clean-all] All build artifacts removed.

# Default target
.DEFAULT_GOAL := help
