# ============================================================================
# Makefile - driver-service (Go)
# ============================================================================
# Cross-platform: GNU Make on Windows (cmd.exe), Linux, and macOS.
# Override tool paths/vars on the command line, e.g.:
#   make test-pkg PKG=./internal/domain
#   make docker-up DOCKER_COMPOSE=docker-compose
# ============================================================================

PROJECT_NAME  := driver-service
GO            := go
BIN_DIR       := bin
BIN           := $(BIN_DIR)/$(PROJECT_NAME)
PKG_SERVER    := ./cmd/server
COVER_OUT     := coverage.out
COVER_HTML    := coverage.html

DOCKER         ?= docker
DOCKER_COMPOSE ?= docker compose
COMPOSE_FILE   := docker-compose.yml
E2E_COMPOSE    := docker-compose.e2e.yml
E2E_PROJECT    := driver-e2e

# Container names created by docker-compose.yml
CTR_MONGO    := driver-mongo-srv
CTR_REDIS    := driver-redis-srv
CTR_KAFKA    := driver-kafka-srv
CTR_NATS     := driver-nats-srv
CTR_SERVICE  := driver-service-local

# ----------------------------------------------------------------------------
# Platform detection - cmd.exe needs different syntax than POSIX shells
# ----------------------------------------------------------------------------
ifeq ($(OS),Windows_NT)
SHELL       := cmd.exe
.SHELLFLAGS := /c
EXE         := .exe
CLEAN_DIR   = if exist $(1) rmdir /s /q $(1)
CLEAN_FILE  = if exist $(1) del /q $(1)
MKDIRP      = if not exist $(1) mkdir $(1)
ENV_E2E     = set "DRIVER_E2E=1" &&
ENV_LINUX   = set "CGO_ENABLED=0" && set "GOOS=linux" &&
ENV_WINDOWS = set "CGO_ENABLED=0" && set "GOOS=windows" &&
ENV_DARWIN  = set "CGO_ENABLED=0" && set "GOOS=darwin" &&
ENV_RACE    =
OPEN_REPORT = start $(COVER_HTML)
else
EXE         :=
CLEAN_DIR   = rm -rf $(1)
CLEAN_FILE  = rm -f $(1)
MKDIRP      = mkdir -p $(1)
ENV_E2E     = DRIVER_E2E=1
ENV_LINUX   = CGO_ENABLED=0 GOOS=linux
ENV_WINDOWS = CGO_ENABLED=0 GOOS=windows
ENV_DARWIN  = CGO_ENABLED=0 GOOS=darwin
ENV_RACE    = -race
UNAME_S     := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
OPEN_REPORT = open $(COVER_HTML)
else
OPEN_REPORT = xdg-open $(COVER_HTML)
endif
endif

.PHONY: help deps deps-tidy deps-verify deps-update deps-graph \
        build build-linux build-windows build-darwin build-all install \
        run start \
        fmt fmt-check vet lint lint-fix staticcheck \
        test test-v test-short test-race test-cov test-cov-func test-pkg \
        test-cmd test-integration test-e2e test-all \
        coverage-report coverage-clean \
        docker-build docker-up docker-down docker-restart docker-logs docker-ps \
        docker-clean docker-shell \
        infra-up infra-down infra-logs infra-status \
        e2e-up e2e-down e2e-logs e2e-clean \
        proto-install proto-gen proto-gen-all \
        mongo-shell redis-cli kafka-topics \
        health metrics grpc-list \
        clean clean-bin clean-coverage clean-logs \
        tidy-check ci-check pre-commit version

.DEFAULT_GOAL := help

# ----------------------------------------------------------------------------
# Help
# ----------------------------------------------------------------------------
help:
	@echo ================================================================================
	@echo  driver-service - available targets
	@echo ================================================================================
	@echo.
	@echo  Setup and dependencies:
	@echo    deps                 - Download Go module dependencies
	@echo    deps-tidy            - Tidy and verify go.mod / go.sum
	@echo    deps-verify          - Verify dependency checksums
	@echo    deps-update          - Update dependencies to latest versions
	@echo    deps-graph           - Print the module dependency graph
	@echo.
	@echo  Build:
	@echo    build                - Build the service binary into bin/
	@echo    build-linux          - Cross-compile a static Linux binary
	@echo    build-windows        - Cross-compile a Windows binary
	@echo    build-darwin         - Cross-compile a macOS binary
	@echo    build-all            - Cross-compile for all three platforms
	@echo    install              - Install the binary into GOBIN
	@echo.
	@echo  Run:
	@echo    run                  - Run the service locally with go run
	@echo    start                - Alias for run
	@echo.
	@echo  Code quality:
	@echo    fmt                  - Format all Go sources with gofmt -w
	@echo    fmt-check            - List unformatted files without changing them
	@echo    vet                  - Run go vet on all packages
	@echo    lint                 - Run golangci-lint
	@echo    lint-fix             - Run golangci-lint with auto-fix
	@echo    staticcheck          - Run staticcheck
	@echo.
	@echo  Testing:
	@echo    test                 - Run the full unit test suite
	@echo    test-v               - Run unit tests verbosely
	@echo    test-short           - Run unit tests in short mode
	@echo    test-race            - Run unit tests with the race detector
	@echo    test-cov             - Run tests with coverage and emit HTML report
	@echo    test-cov-func        - Run tests with coverage and print per-function totals
	@echo    test-pkg             - Test a single package, e.g. make test-pkg PKG=./internal/domain
	@echo    test-cmd             - Run cmd/server integration-style tests, needs live infra
	@echo    test-integration     - Alias for test-cmd
	@echo    test-e2e             - Run the end-to-end suite against the e2e infra, needs DRIVER_E2E=1
	@echo    test-all             - Run unit tests plus the end-to-end suite
	@echo    coverage-report      - Generate and open the HTML coverage report
	@echo    coverage-clean       - Remove coverage artifacts
	@echo.
	@echo  Docker:
	@echo    docker-build         - Build the production image from the repo root context
	@echo    docker-up            - Build and start the full dev stack
	@echo    docker-down          - Stop and remove the dev stack
	@echo    docker-restart       - Restart the dev stack containers
	@echo    docker-logs          - Tail the dev stack logs
	@echo    docker-ps            - Show running containers
	@echo    docker-clean         - Stop the stack and remove the image
	@echo    docker-shell         - Open a shell inside the running service container
	@echo.
	@echo  Infrastructure only, without the service:
	@echo    infra-up             - Start MongoDB, Redis, Kafka, and NATS
	@echo    infra-down           - Stop the infrastructure containers
	@echo    infra-logs           - Tail the infrastructure logs
	@echo    infra-status         - Show infrastructure container status
	@echo.
	@echo  End-to-end infrastructure, offset ports, project driver-e2e:
	@echo    e2e-up               - Start the e2e stack and wait for healthchecks
	@echo    e2e-down             - Stop the e2e stack
	@echo    e2e-logs             - Tail the e2e stack logs
	@echo    e2e-clean            - Stop the e2e stack and delete its volumes
	@echo.
	@echo  Protobuf:
	@echo    proto-install        - Install protoc-gen-go and protoc-gen-go-grpc
	@echo    proto-gen            - Regenerate driver.proto bindings
	@echo    proto-gen-all        - Regenerate driver, delivery, and user proto bindings
	@echo.
	@echo  Operations helpers:
	@echo    mongo-shell          - Open mongosh against the dev MongoDB
	@echo    redis-cli            - Open redis-cli against the dev Redis
	@echo    kafka-topics         - List Kafka topics in the dev broker
	@echo    health               - Curl the liveness endpoint
	@echo    metrics              - Curl the Prometheus metrics endpoint
	@echo    grpc-list            - List gRPC services via grpcurl reflection
	@echo.
	@echo  Maintenance:
	@echo    clean                - Remove binary, coverage, and stray artifacts
	@echo    clean-bin            - Remove only the bin directory
	@echo    clean-coverage       - Remove only coverage artifacts
	@echo    clean-logs           - Remove stray log and exe artifacts
	@echo    tidy-check           - Fail if go mod tidy would change go.mod or go.sum
	@echo    ci-check             - Run the full local CI pipeline
	@echo    pre-commit           - Run fast pre-commit checks
	@echo    version              - Print the active Go toolchain version
	@echo.
	@echo  Notes:
	@echo    The Docker build context is the repository root, two levels up,
	@echo    because the image also copies the shared packages/go module.
	@echo    On Windows the race detector is skipped because it needs cgo.

# ----------------------------------------------------------------------------
# Setup and dependencies
# ----------------------------------------------------------------------------
deps:
	$(GO) mod download

deps-tidy:
	$(GO) mod tidy
	$(GO) mod verify

deps-verify:
	$(GO) mod verify

deps-update:
	$(GO) get -u ./...
	$(GO) mod tidy

deps-graph:
	$(GO) list -m all

# ----------------------------------------------------------------------------
# Build
# ----------------------------------------------------------------------------
build:
	$(call MKDIRP,$(BIN_DIR))
	$(GO) build -trimpath -ldflags "-w -s" -o $(BIN)$(EXE) $(PKG_SERVER)

build-linux:
	$(call MKDIRP,$(BIN_DIR))
	$(ENV_LINUX) $(GO) build -trimpath -ldflags "-w -s" -o $(BIN)-linux $(PKG_SERVER)

build-windows:
	$(call MKDIRP,$(BIN_DIR))
	$(ENV_WINDOWS) $(GO) build -trimpath -ldflags "-w -s" -o $(BIN)-windows.exe $(PKG_SERVER)

build-darwin:
	$(call MKDIRP,$(BIN_DIR))
	$(ENV_DARWIN) $(GO) build -trimpath -ldflags "-w -s" -o $(BIN)-darwin $(PKG_SERVER)

build-all: build-linux build-windows build-darwin

install:
	$(GO) install $(PKG_SERVER)

# ----------------------------------------------------------------------------
# Run
# ----------------------------------------------------------------------------
run:
	$(GO) run $(PKG_SERVER)

start: run

# ----------------------------------------------------------------------------
# Code quality
# ----------------------------------------------------------------------------
fmt:
	gofmt -w .

fmt-check:
	gofmt -l .

vet:
	$(GO) vet ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

staticcheck:
	staticcheck ./...

# ----------------------------------------------------------------------------
# Testing
# ----------------------------------------------------------------------------
test:
	$(GO) test ./...

test-v:
	$(GO) test ./... -v

test-short:
	$(GO) test ./... -short

test-race:
	$(GO) test ./... $(ENV_RACE)

test-cov:
	$(GO) test ./... -coverprofile=$(COVER_OUT) -covermode=atomic
	$(GO) tool cover -html=$(COVER_OUT) -o $(COVER_HTML)

test-cov-func:
	$(GO) test ./... -coverprofile=$(COVER_OUT) -covermode=atomic
	$(GO) tool cover -func=$(COVER_OUT)

test-pkg:
	$(GO) test $(if $(PKG),$(PKG),./internal/...) -v

test-cmd:
	$(GO) test $(PKG_SERVER)/... -count=1 -v

test-integration: test-cmd

test-e2e:
	$(ENV_E2E) $(GO) test ./test/e2e/... -count=1 -v

test-all: test test-e2e

coverage-report: test-cov
	$(OPEN_REPORT)

coverage-clean:
	$(call CLEAN_FILE,$(COVER_OUT))
	$(call CLEAN_FILE,$(COVER_HTML))
	$(call CLEAN_DIR,coverage)

# ----------------------------------------------------------------------------
# Docker - the dev stack from docker-compose.yml
# ----------------------------------------------------------------------------
docker-build:
	$(DOCKER) build -f Dockerfile -t $(PROJECT_NAME):latest ../..

docker-up:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up -d --build

docker-down:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) down

docker-restart:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) restart

docker-logs:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) logs -f

docker-ps:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) ps

docker-clean:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) down --rmi local --volumes --remove-orphans

docker-shell:
	$(DOCKER) exec -it $(CTR_SERVICE) sh

# ----------------------------------------------------------------------------
# Infrastructure only, without the service container
# ----------------------------------------------------------------------------
infra-up:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up -d $(CTR_MONGO) $(CTR_REDIS) $(CTR_KAFKA) $(CTR_NATS)

infra-down:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) stop $(CTR_MONGO) $(CTR_REDIS) $(CTR_KAFKA) $(CTR_NATS)

infra-logs:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) logs -f $(CTR_MONGO) $(CTR_REDIS) $(CTR_KAFKA) $(CTR_NATS)

infra-status:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) ps $(CTR_MONGO) $(CTR_REDIS) $(CTR_KAFKA) $(CTR_NATS)

# ----------------------------------------------------------------------------
# End-to-end infrastructure - offset ports so it can run beside the dev stack
# ----------------------------------------------------------------------------
e2e-up:
	$(DOCKER_COMPOSE) -f $(E2E_COMPOSE) -p $(E2E_PROJECT) up -d --wait

e2e-down:
	$(DOCKER_COMPOSE) -f $(E2E_COMPOSE) -p $(E2E_PROJECT) down

e2e-logs:
	$(DOCKER_COMPOSE) -f $(E2E_COMPOSE) -p $(E2E_PROJECT) logs -f

e2e-clean:
	$(DOCKER_COMPOSE) -f $(E2E_COMPOSE) -p $(E2E_PROJECT) down --volumes --remove-orphans

# ----------------------------------------------------------------------------
# Protobuf - sources live in ../../protos, generated Go lands in
# internal/adapters/grpc/proto
# ----------------------------------------------------------------------------
proto-install:
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto-gen:
	protoc --proto_path=../../protos \
		--go_out=internal/adapters/grpc/proto --go_opt=paths=source_relative \
		--go-grpc_out=internal/adapters/grpc/proto --go-grpc_opt=paths=source_relative \
		driver.proto

proto-gen-all:
	protoc --proto_path=../../protos \
		--go_out=internal/adapters/grpc/proto --go_opt=paths=source_relative \
		--go-grpc_out=internal/adapters/grpc/proto --go-grpc_opt=paths=source_relative \
		--go_opt=Muser.proto=github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto \
		--go-grpc_opt=Muser.proto=github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto \
		driver.proto delivery.proto user.proto

# ----------------------------------------------------------------------------
# Operations helpers against the dev stack
# ----------------------------------------------------------------------------
mongo-shell:
	$(DOCKER) exec -it $(CTR_MONGO) mongosh driver_db

redis-cli:
	$(DOCKER) exec -it $(CTR_REDIS) redis-cli

kafka-topics:
	$(DOCKER) exec $(CTR_KAFKA) /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list

health:
	curl -s http://localhost:4008/health/live

metrics:
	curl -s http://localhost:9105/metrics

grpc-list:
	grpcurl -plaintext localhost:50055 list

# ----------------------------------------------------------------------------
# Maintenance
# ----------------------------------------------------------------------------
clean: clean-bin clean-coverage clean-logs

clean-bin:
	$(call CLEAN_DIR,$(BIN_DIR))

clean-logs:
	$(call CLEAN_FILE,service.log)
	$(call CLEAN_FILE,service.err)
	$(call CLEAN_FILE,stub.log)
	$(call CLEAN_FILE,stub.err)
	$(call CLEAN_FILE,coverage_run.log)
	$(call CLEAN_FILE,commands.test.exe)
	$(call CLEAN_FILE,stub-user.exe)
	$(call CLEAN_FILE,driver-service-e2e)

tidy-check:
	$(GO) mod tidy -diff

ci-check: fmt-check vet lint test-cov build

pre-commit: fmt vet test-short

version:
	$(GO) version

# ----------------------------------------------------------------------------
# Internal helpers - CLEAN_DIR, CLEAN_FILE, and MKDIRP are defined per platform
# near the top of this file.
# ----------------------------------------------------------------------------
