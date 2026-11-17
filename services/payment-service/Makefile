# Payment Service Makefile
SHELL := cmd.exe
.DEFAULT_GOAL := help

SERVICE_NAME := payment-service
DOCKER_IMAGE := payment-service
BIN_DIR := bin
BINARY := $(BIN_DIR)/$(SERVICE_NAME).exe

.PHONY: help
help: ## Display available make targets
	@echo =====================================================================
	@echo Payment Service - Available Make Targets
	@echo =====================================================================
	@echo   make tidy              Download and tidy Go dependencies
	@echo   make fmt               Format source code with gofmt
	@echo   make vet               Run go vet static analysis
	@echo   make lint              Run golangci-lint
	@echo   make build             Build the service binary
	@echo   make run               Run service locally
	@echo   make test              Run unit tests
	@echo   make test-v            Run unit tests with verbose output
	@echo   make test-e2e          Run end-to-end integration tests
	@echo   make test-all          Run unit and e2e tests
	@echo   make coverage          Generate test coverage report
	@echo   make docker-build      Build service Docker image
	@echo   make docker-up         Start docker-compose stack
	@echo   make docker-down       Stop docker-compose stack
	@echo   make docker-logs       Tail docker-compose logs
	@echo   make clean             Remove compiled binaries and coverage output
	@echo =====================================================================

.PHONY: tidy
tidy: ## Download and tidy Go dependencies
	go mod tidy

.PHONY: fmt
fmt: ## Format Go source code
	gofmt -s -w .

.PHONY: vet
vet: ## Run go vet static analysis
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: build
build: ## Build binary
	@if not exist $(BIN_DIR) mkdir $(BIN_DIR)
	go build -ldflags="-s -w" -o $(BINARY) ./cmd/server

.PHONY: run
run: ## Run service directly
	go run ./cmd/server

.PHONY: test
test: ## Run unit tests
	go test ./internal/...

.PHONY: test-v
test-v: ## Run unit tests with verbose output
	go test -v ./internal/...

.PHONY: test-e2e
test-e2e: ## Run end-to-end integration tests
	go test -v ./test/e2e/...

.PHONY: test-all
test-all: test test-e2e ## Run all tests (unit and e2e)

.PHONY: coverage
coverage: ## Generate code coverage report
	@if not exist $(BIN_DIR) mkdir $(BIN_DIR)
	go test -coverprofile=$(BIN_DIR)/coverage.out ./internal/...
	go tool cover -html=$(BIN_DIR)/coverage.out -o $(BIN_DIR)/coverage.html
	@echo Coverage report written to $(BIN_DIR)/coverage.html

.PHONY: docker-build
docker-build: ## Build docker image
	docker build -f Dockerfile -t $(DOCKER_IMAGE):latest ../..

.PHONY: docker-up
docker-up: ## Start service and dependencies with docker compose
	docker-compose up -d

.PHONY: docker-down
docker-down: ## Stop docker compose services
	docker-compose down

.PHONY: docker-logs
docker-logs: ## Tail docker compose logs
	docker-compose logs -f

.PHONY: clean
clean: ## Clean build binaries and coverage
	@if exist $(BIN_DIR) rmdir /s /q $(BIN_DIR)

