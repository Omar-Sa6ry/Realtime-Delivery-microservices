# Search Service Makefile
# Cross-platform recipes: every line is a single self-contained command
# (works under Windows cmd.exe via ezwinports make and POSIX sh on Unix).

.DEFAULT_GOAL := help

SERVICE := search-service
BIN_DIR := bin
BIN     := $(BIN_DIR)/search-service

COVER_PROFILE  := coverprofile.out
SUBPROCESS_COV := $(CURDIR)/subprocess.out
COVER_MERGED   := merged.out
COVER_HTML     := coverage.html

# Exported so cmd/server TestMain writes subprocess GOCOVERDIR data to a
# deterministic path when tests run under make.
export SUBPROCESS_COV_OUT = $(SUBPROCESS_COV)

.PHONY: help tidy download mod-verify fmt fmt-check vet build run test test-v test-race coverage coverage-html ci clean docker-build docker-up docker-down

help: ## Show available targets
	@echo.
	@echo $(SERVICE) - available targets:
	@echo.
	@echo   tidy            Run go mod tidy
	@echo   download        Run go mod download
	@echo   mod-verify      Verify dependency checksums
	@echo   fmt             Format all Go sources with gofmt -w
	@echo   fmt-check       Fail if any Go file needs gofmt
	@echo   vet             Run go vet on all packages
	@echo   build           Compile the server binary to bin/
	@echo   run             Run the server with go run
	@echo   test            Run all tests once (no cache)
	@echo   test-v          Run all tests once, verbose output
	@echo   test-race       Run all tests with the race detector (requires gcc)
	@echo   coverage        Run tests with coverage (merges subprocess GOCOVERDIR) and print per-function stats
	@echo   coverage-html   Generate coverage.html from the merged profile (run coverage first)
	@echo   ci              Full gate: fmt-check, vet, test, coverage
	@echo   clean           Remove bin/ and coverage artifacts
	@echo   docker-build    Build the Docker image (context: monorepo root)
	@echo   docker-up       Start the local docker compose stack
	@echo   docker-down     Stop the local docker compose stack
	@echo.

tidy: ## go mod tidy
	go mod tidy

download: ## go mod download
	go mod download

mod-verify: ## go mod verify
	go mod verify

fmt: ## gofmt -w all sources
	gofmt -w .

fmt-check: ## fail if gofmt would change files
	go run ./scripts/checktool fmt

vet: ## go vet ./...
	go vet ./...

build: ## build server binary to bin/search-service
	go build -o $(BIN) ./cmd/server

run: ## run the server
	go run ./cmd/server

test: ## run all tests once
	go test ./... -count=1

test-v: ## run all tests once, verbose
	go test ./... -count=1 -v

test-e2e: ## run end-to-end integration tests
	go test -v ./test/e2e/...

test-race: ## run all tests with -race (requires a C compiler, e.g. gcc)
	go test ./... -count=1 -race

coverage: ## run tests with coverage and merge subprocess GOCOVERDIR data
	go test ./... -count=1 -covermode=count -coverprofile=$(COVER_PROFILE)
	go run ./scripts/checktool merge $(COVER_PROFILE) "$(SUBPROCESS_COV)" $(COVER_MERGED)
	go tool cover -func $(COVER_MERGED)

coverage-html: ## generate coverage.html from merged.out
	go tool cover -html $(COVER_MERGED) -o $(COVER_HTML)

ci: fmt-check vet test coverage ## full verification gate
	@echo ci: all checks passed

clean: ## remove build and coverage artifacts
	go run ./scripts/checktool clean

docker-build: ## build Docker image (Dockerfile lives in service dir, context is monorepo root)
	docker build -f Dockerfile -t search-service:latest ../..

docker-up: ## start local dependencies via docker compose
	docker compose up -d --build

docker-down: ## stop local docker compose stack
	docker compose down
