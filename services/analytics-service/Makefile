# =============================================================================
# Analytics Service - Makefile
# =============================================================================
# Comprehensive task runner for the analytics-service microservice
# (Kafka event ingestion -> ClickHouse facts, Apollo Federation GraphQL
#  subgraph, internal gRPC API, NATS live ticker, Prometheus metrics).
#
# Cross-platform recipes: every recipe line is a single self-contained command
# (no shell operators such as &&, pipes or redirection), and all branching is
# expressed with make functions ($(if), $(wildcard), $(error)) instead of shell
# syntax. The blank-line separators in the help and ports output use "echo.",
# which relies on the cmd.exe shell that GNU Make for Windows selects when
# sh.exe is not on PATH.
#
# Usage:
#   make help              list every target (default goal)
#   make test              run the whole unit-test suite
#   make verify            fmt-check + vet + build + test + coverage
#   make docker-up         build and start the local compose stack
#   make health            probe the running service
# =============================================================================

.DEFAULT_GOAL := help
MAKEFLAGS    += --no-print-directory

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------
SERVICE    := analytics-service
BIN_DIR    := bin
BIN        := $(BIN_DIR)/$(SERVICE)

COVER_PROFILE := coverage.out
COVER_HTML    := coverage.html

# Legacy artifacts written by earlier ad-hoc coverage runs (also cleaned).
LEGACY_COVERAGE := coverage

GRAPHQL_PORT  := 4009
GRPC_PORT     := 50057
METRICS_PORT  := 9107

HEALTH_URL   := http://localhost:$(GRAPHQL_PORT)/health/live
READY_URL    := http://localhost:$(GRAPHQL_PORT)/health/ready
METRICS_URL  := http://localhost:$(METRICS_PORT)/metrics
GRAPHQL_URL  := http://localhost:$(GRAPHQL_PORT)/graphql
ANALYTICS_URL := http://localhost:$(GRAPHQL_PORT)/analytics/graphql

COMPOSE      := docker compose -f docker-compose.yml
INFRA_SERVICES := kafka-srv clickhouse-srv redis-srv

KAFKA_CONTAINER       := analytics-kafka-srv
CLICKHOUSE_CONTAINER  := analytics-clickhouse-srv
REDIS_CONTAINER       := analytics-redis-srv
APP_CONTAINER         := analytics-service-local

KAFKA_BOOTSTRAP := localhost:9092
CLICKHOUSE_DB   := analytics
DLQ_TOPIC       := analytics.dlq
REDIS_PREFIX    := analytics:*

# Overridable: make clickhouse-query QUERY="SELECT 1"
QUERY ?= SELECT count() FROM raw_events

# Deferred expansion: evaluated only when fmt-check runs, so plain
# "make help" never shells out to gofmt.
FMT_UNFORMATTED = $(shell gofmt -l .)

# -----------------------------------------------------------------------------
# Meta
# -----------------------------------------------------------------------------
.PHONY: help version check-deps deps \
        tidy download mod-verify \
        fmt fmt-check vet lint staticcheck vuln \
        build rebuild run \
        test test-v test-short test-race test-cover coverage coverage-func coverage-html bench verify ci \
        proto-tools proto-check proto-generate \
        docker-build docker-up docker-up-build docker-down docker-down-volumes docker-restart \
        docker-logs docker-ps docker-shell docker-prune compose-check \
        infra-up infra-down infra-logs kafka-up clickhouse-up redis-up \
        kafka-logs clickhouse-logs redis-logs \
        kafka-topics kafka-describe kafka-dlq \
        clickhouse-client clickhouse-tables clickhouse-query \
        redis-cli redis-keys redis-flush \
        health ready metrics graphql-sdl grpc-list ports \
        clean clean-bin clean-coverage

help: ## List every available target
	@echo  ============================================================
	@echo   Analytics Service - Available Targets
	@echo  ============================================================
	@echo.
	@echo  Meta
	@echo    make help                 Show this help (default goal)
	@echo    make version              Print the Go toolchain version
	@echo    make check-deps           Verify go, docker and docker compose
	@echo    make deps                 List all resolved Go modules
	@echo.
	@echo  Modules
	@echo    make tidy                 go mod tidy
	@echo    make download             go mod download
	@echo    make mod-verify           go mod verify (checksums)
	@echo.
	@echo  Quality
	@echo    make fmt                  gofmt -w . (rewrite sources in place)
	@echo    make fmt-check            Fail when gofmt -l reports files
	@echo    make vet                  go vet ./...
	@echo    make lint                 golangci-lint run ./... (optional tool)
	@echo    make staticcheck          staticcheck ./... (optional tool)
	@echo    make vuln                 govulncheck ./... (optional tool)
	@echo.
	@echo  Build and run
	@echo    make build                Compile bin/analytics-service
	@echo    make rebuild              Clean bin/ then build
	@echo    make run                  go run ./cmd/server
	@echo.
	@echo  Test
	@echo    make test                 Run all tests once
	@echo    make test-v               Run all tests once, verbose
	@echo    make test-short           Run tests flagged as short
	@echo    make test-race            Run tests with the race detector
	@echo    make test-cover           Run tests with a per-package summary
	@echo    make coverage             Write coverage.out profile
	@echo    make coverage-func        Per-function totals from coverage.out
	@echo    make coverage-html        Render coverage.html report
	@echo    make bench                Run Go benchmarks with allocations
	@echo    make verify               Full gate: fmt-check vet build test coverage
	@echo    make ci                   tidy then verify (CI pipeline)
	@echo.
	@echo  Protobuf
	@echo    make proto-tools          Install protoc-gen-go and protoc-gen-go-grpc
	@echo    make proto-check          Verify ../../protos/analytics.proto exists
	@echo    make proto-generate       Regenerate internal/adapters/grpc/proto
	@echo.
	@echo  Docker
	@echo    make docker-build         Build the service image (context: repo root)
	@echo    make docker-up            Compose up (infra + service), detached
	@echo    make docker-up-build       Compose up with --build
	@echo    make docker-down          Compose down
	@echo    make docker-down-volumes  Compose down and delete volumes
	@echo    make docker-restart       Restart the compose stack
	@echo    make docker-logs          Tail the service logs
	@echo    make docker-ps            Show compose containers
	@echo    make docker-shell         Open a shell in the service container
	@echo    make docker-prune         Remove dangling containers and images
	@echo    make compose-check        Validate docker-compose.yml
	@echo.
	@echo  Infrastructure
	@echo    make infra-up             Start kafka, clickhouse and redis
	@echo    make infra-down           Stop kafka, clickhouse and redis
	@echo    make infra-logs           Tail infrastructure logs
	@echo    make kafka-up             Start kafka only
	@echo    make clickhouse-up        Start clickhouse only
	@echo    make redis-up             Start redis only
	@echo    make kafka-logs           Tail kafka logs
	@echo    make clickhouse-logs      Tail clickhouse logs
	@echo    make redis-logs           Tail redis logs
	@echo.
	@echo  Kafka operations
	@echo    make kafka-topics         List all topics on the local broker
	@echo    make kafka-describe       Describe topics, partitions and ISRs
	@echo    make kafka-dlq            Consume the analytics dead-letter topic
	@echo.
	@echo  ClickHouse operations
	@echo    make clickhouse-client    Interactive clickhouse-client shell
	@echo    make clickhouse-tables    List tables in the analytics database
	@echo    make clickhouse-query     Run QUERY (make ... QUERY=...)
	@echo.
	@echo  Redis operations
	@echo    make redis-cli            Interactive redis-cli shell
	@echo    make redis-keys           Scan keys in the analytics cache namespace
	@echo    make redis-flush          Flush the current redis database
	@echo.
	@echo  Health and endpoints
	@echo    make health               GET /health/live
	@echo    make ready                GET /health/ready
	@echo    make metrics              GET /metrics on the metrics port
	@echo    make graphql-sdl          POST _service sdl federation smoke test
	@echo    make grpc-list            grpcurl list of gRPC methods (optional)
	@echo    make ports                Print the port map of this service
	@echo.
	@echo  Clean
	@echo    make clean                Remove bin/ and coverage artifacts
	@echo    make clean-bin            Remove bin/
	@echo    make clean-coverage       Remove coverage.out, coverage.html
	@echo.
	@echo  Variables: QUERY=sql (clickhouse-query), override GRAPHQL_PORT if needed
	@echo  Examples:  make verify
	@echo             make docker-up
	@echo             make clickhouse-query QUERY="SELECT count() FROM raw_events"
	@echo.

version:
	@echo version: Go toolchain used by this service
	go version

check-deps:
	@echo check-deps: verifying required tools on PATH
	go version
	docker --version
	docker compose version
	@echo check-deps: OK

deps:
	@echo deps: resolved Go modules
	go list -m all

# -----------------------------------------------------------------------------
# Modules
# -----------------------------------------------------------------------------

tidy:
	@echo tidy: running go mod tidy
	go mod tidy
	@echo tidy: done

download:
	@echo download: fetching module dependencies
	go mod download
	@echo download: done

mod-verify:
	@echo mod-verify: verifying dependency checksums
	go mod verify

# -----------------------------------------------------------------------------
# Quality
# -----------------------------------------------------------------------------

fmt:
	@echo fmt: rewriting sources with gofmt -w
	gofmt -w .

# Fails when gofmt -l reports anything. Branching happens in make itself so
# the recipe stays valid under cmd.exe and sh alike.
fmt-check:
	$(if $(FMT_UNFORMATTED),$(error fmt-check: these files need 'make fmt':$(FMT_UNFORMATTED)),@echo fmt-check: OK - all files are gofmt-clean)

vet:
	@echo vet: running go vet
	go vet ./...
	@echo vet: OK

lint:
	@echo lint: running golangci-lint
	golangci-lint run ./...

staticcheck:
	@echo staticcheck: running staticcheck
	staticcheck ./...

vuln:
	@echo vuln: running govulncheck
	govulncheck ./...

# -----------------------------------------------------------------------------
# Build and run
# -----------------------------------------------------------------------------

build:
	@echo build: compiling bin/$(SERVICE) from ./cmd/server
	go build -o $(BIN) ./cmd/server
	@echo build: done - $(BIN)

rebuild: clean-bin build

run:
	@echo run: starting the analytics service (Ctrl+C to stop)
	go run ./cmd/server

# -----------------------------------------------------------------------------
# Test
# -----------------------------------------------------------------------------

test:
	@echo test: running the full unit-test suite
	go test ./... -count=1

test-v:
	@echo test-v: running the full unit-test suite (verbose)
	go test ./... -count=1 -v

test-short:
	@echo test-short: running short tests only
	go test ./... -short -count=1

test-race:
	@echo test-race: running tests with the race detector (requires gcc)
	go test ./... -count=1 -race

test-e2e: ## run end-to-end integration tests
	go test -v ./test/e2e/...

test-cover:
	@echo test-cover: per-package coverage summary
	go test ./... -cover -count=1

coverage:
	@echo coverage: writing $(COVER_PROFILE)
	go test ./... -count=1 -covermode=atomic -coverprofile=$(COVER_PROFILE)

coverage-func: coverage
	@echo coverage-func: per-function totals
	go tool cover -func=$(COVER_PROFILE)

coverage-html: coverage
	@echo coverage-html: rendering $(COVER_HTML)
	go tool cover -html=$(COVER_PROFILE) -o $(COVER_HTML)
	@echo coverage-html: open $(COVER_HTML) in a browser

bench:
	@echo bench: running Go benchmarks
	go test ./... -count=1 -run=XXX -bench=. -benchmem

verify: fmt-check vet build test coverage
	@echo verify: fmt-check + vet + build + test + coverage all passed

ci: tidy verify
	@echo ci: tidy + verify all passed

# -----------------------------------------------------------------------------
# Protobuf
# -----------------------------------------------------------------------------

proto-tools:
	@echo proto-tools: installing protoc-gen-go and protoc-gen-go-grpc
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo proto-tools: done

proto-check:
	$(if $(wildcard ../../protos/analytics.proto),@echo proto-check: ../../protos/analytics.proto found,$(error proto-check: ../../protos/analytics.proto is missing))

proto-generate: proto-check
	@echo proto-generate: regenerating internal/adapters/grpc/proto
	protoc -I../../protos --go_out=internal/adapters/grpc/proto --go_opt=paths=source_relative --go-grpc_out=internal/adapters/grpc/proto --go-grpc_opt=paths=source_relative analytics.proto
	@echo proto-generate: done

# -----------------------------------------------------------------------------
# Docker
# -----------------------------------------------------------------------------

docker-build:
	@echo docker-build: building $(SERVICE):latest (context: ../..)
	docker build -f Dockerfile -t $(SERVICE):latest ../..
	@echo docker-build: done - image $(SERVICE):latest

docker-up:
	@echo docker-up: compose up (kafka, clickhouse, redis, service)
	$(COMPOSE) up -d
	@echo docker-up: done - GraphQL $(GRAPHQL_URL) - Health $(HEALTH_URL)

docker-up-build:
	@echo docker-up-build: compose up --build
	$(COMPOSE) up -d --build
	@echo docker-up-build: done

docker-down:
	@echo docker-down: compose down
	$(COMPOSE) down
	@echo docker-down: done

docker-down-volumes:
	@echo docker-down-volumes: compose down -v (destroys clickhouse data)
	$(COMPOSE) down -v
	@echo docker-down-volumes: done

docker-restart:
	@echo docker-restart: compose restart
	$(COMPOSE) restart

docker-logs:
	@echo docker-logs: tailing service logs (Ctrl+C to stop)
	$(COMPOSE) logs -f analytics-service

docker-ps:
	@echo docker-ps: compose containers
	$(COMPOSE) ps

docker-shell:
	@echo docker-shell: opening a shell in $(APP_CONTAINER)
	docker exec -it $(APP_CONTAINER) sh

docker-prune:
	@echo docker-prune: removing dangling containers
	docker container prune -f
	@echo docker-prune: removing dangling images
	docker image prune -f

compose-check:
	@echo compose-check: validating docker-compose.yml
	$(COMPOSE) config --quiet
	@echo compose-check: OK

# -----------------------------------------------------------------------------
# Infrastructure
# -----------------------------------------------------------------------------

infra-up:
	@echo infra-up: starting kafka, clickhouse and redis
	$(COMPOSE) up -d $(INFRA_SERVICES)
	@echo infra-up: done

infra-down:
	@echo infra-down: stopping kafka, clickhouse and redis
	$(COMPOSE) stop $(INFRA_SERVICES)
	@echo infra-down: done

infra-logs:
	@echo infra-logs: tailing infrastructure logs (Ctrl+C to stop)
	$(COMPOSE) logs -f $(INFRA_SERVICES)

kafka-up:
	@echo kafka-up: starting kafka-srv
	$(COMPOSE) up -d kafka-srv

clickhouse-up:
	@echo clickhouse-up: starting clickhouse-srv
	$(COMPOSE) up -d clickhouse-srv

redis-up:
	@echo redis-up: starting redis-srv
	$(COMPOSE) up -d redis-srv

kafka-logs:
	@echo kafka-logs: tailing kafka logs (Ctrl+C to stop)
	$(COMPOSE) logs -f kafka-srv

clickhouse-logs:
	@echo clickhouse-logs: tailing clickhouse logs (Ctrl+C to stop)
	$(COMPOSE) logs -f clickhouse-srv

redis-logs:
	@echo redis-logs: tailing redis logs (Ctrl+C to stop)
	$(COMPOSE) logs -f redis-srv

# -----------------------------------------------------------------------------
# Kafka operations
# -----------------------------------------------------------------------------

kafka-topics:
	@echo kafka-topics: listing topics on $(KAFKA_BOOTSTRAP)
	docker exec $(KAFKA_CONTAINER) /opt/kafka/bin/kafka-topics.sh --bootstrap-server $(KAFKA_BOOTSTRAP) --list

kafka-describe:
	@echo kafka-describe: describing topics on $(KAFKA_BOOTSTRAP)
	docker exec $(KAFKA_CONTAINER) /opt/kafka/bin/kafka-topics.sh --bootstrap-server $(KAFKA_BOOTSTRAP) --describe

kafka-dlq:
	@echo kafka-dlq: consuming $(DLQ_TOPIC) from the beginning (Ctrl+C to stop)
	docker exec $(KAFKA_CONTAINER) /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server $(KAFKA_BOOTSTRAP) --topic $(DLQ_TOPIC) --from-beginning

# -----------------------------------------------------------------------------
# ClickHouse operations
# -----------------------------------------------------------------------------

clickhouse-client:
	@echo clickhouse-client: interactive shell in database $(CLICKHOUSE_DB)
	docker exec -it $(CLICKHOUSE_CONTAINER) clickhouse-client --database $(CLICKHOUSE_DB)

clickhouse-tables:
	@echo clickhouse-tables: tables in database $(CLICKHOUSE_DB)
	docker exec $(CLICKHOUSE_CONTAINER) clickhouse-client --database $(CLICKHOUSE_DB) --query "SHOW TABLES"

clickhouse-query:
	@echo clickhouse-query: $(QUERY)
	docker exec $(CLICKHOUSE_CONTAINER) clickhouse-client --database $(CLICKHOUSE_DB) --query "$(QUERY)"

# -----------------------------------------------------------------------------
# Redis operations
# -----------------------------------------------------------------------------

redis-cli:
	@echo redis-cli: interactive redis shell
	docker exec -it $(REDIS_CONTAINER) redis-cli

redis-keys:
	@echo redis-keys: scanning $(REDIS_PREFIX)
	docker exec $(REDIS_CONTAINER) redis-cli --scan --pattern "$(REDIS_PREFIX)"

redis-flush:
	@echo redis-flush: flushing the current redis database
	docker exec $(REDIS_CONTAINER) redis-cli FLUSHDB

# -----------------------------------------------------------------------------
# Health and endpoints
# -----------------------------------------------------------------------------

health:
	@echo health: GET $(HEALTH_URL)
	curl -sf --max-time 5 $(HEALTH_URL)

ready:
	@echo ready: GET $(READY_URL)
	curl -sf --max-time 5 $(READY_URL)

metrics:
	@echo metrics: GET $(METRICS_URL)
	curl -sf --max-time 5 $(METRICS_URL)

graphql-sdl:
	@echo graphql-sdl: POST federation _service sdl to $(GRAPHQL_URL)
	curl -s -X POST -H "Content-Type: application/json" -d "{\"query\":\"{_service{sdl}}\"}" $(GRAPHQL_URL)

grpc-list:
	@echo grpc-list: listing gRPC methods on localhost:$(GRPC_PORT)
	grpcurl -plaintext localhost:$(GRPC_PORT) list

ports:
	@echo.
	@echo  $(SERVICE) ports:
	@echo    GraphQL HTTP : $(GRAPHQL_PORT)  -  $(GRAPHQL_URL)
	@echo    GraphQL alias: $(GRAPHQL_PORT)  -  $(ANALYTICS_URL)
	@echo    Liveness     : $(HEALTH_URL)
	@echo    Readiness    : $(READY_URL)
	@echo    Metrics      : $(METRICS_URL)
	@echo    gRPC         : $(GRPC_PORT)  -  analytics.AnalyticsService
	@echo    Kafka        : 9092  (container $(KAFKA_CONTAINER))
	@echo    ClickHouse   : 8123 HTTP / 9000 native
	@echo    Redis        : 6379
	@echo.

# -----------------------------------------------------------------------------
# Clean
# -----------------------------------------------------------------------------

clean: clean-bin clean-coverage
	@echo clean: build and coverage artifacts removed

clean-bin:
	@echo clean-bin: removing $(BIN_DIR)/
	git clean -fdx $(BIN_DIR)

clean-coverage:
	@echo clean-coverage: removing coverage artifacts
	git clean -fdx $(COVER_PROFILE) $(COVER_HTML) $(LEGACY_COVERAGE)
