.PHONY: help tidy fmt fmt-check vet build run test test-v test-e2e test-e2e-coverage coverage coverage-html docker-build clean

SERVICE_NAME := media-service
BIN_DIR := bin
BINARY := $(BIN_DIR)/$(SERVICE_NAME)
COVER_OUT := coverage.out
COVER_HTML := coverage.html

help:
	@echo =====================================================================
	@echo Media Service - Available Make Targets
	@echo =====================================================================
	@echo   make tidy              Download and tidy Go dependencies
	@echo   make fmt               Format source code with gofmt
	@echo   make fmt-check         Check code formatting without modifying files
	@echo   make vet               Run go vet static analysis
	@echo   make build             Compile the service binary
	@echo   make run               Run the service locally
	@echo   make test              Run all unit tests
	@echo   make test-v            Run all unit tests verbosely
	@echo   make test-e2e          Run E2E suite against live stack (MEDIA_E2E=1)
	@echo   make test-e2e-coverage Run E2E suite with coverage tracking
	@echo   make coverage          Generate test coverage report
	@echo   make coverage-html     Generate HTML coverage report
	@echo   make docker-build      Build the Docker image from monorepo root
	@echo   make clean             Remove compiled binaries and coverage outputs
	@echo =====================================================================

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files found:" && gofmt -l . && exit 1)

vet:
	go vet ./...

build:
	@if not exist $(BIN_DIR) mkdir $(BIN_DIR)
	go build -o $(BINARY) ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -count=1

test-v:
	go test ./... -v -count=1

test-e2e:
	go test ./test/e2e/ -v -count=1 -timeout 2400s

test-e2e-coverage:
	go test ./test/e2e/ -v -count=1 -timeout 2400s -coverpkg=./internal/... -covermode=atomic -coverprofile=./test/e2e/covered.out
	@echo Coverage profile: test/e2e/covered.out

coverage:
	go test -coverprofile=$(COVER_OUT) ./internal/...
	go tool cover -func=$(COVER_OUT)

coverage-html: coverage
	go tool cover -html=$(COVER_OUT) -o $(COVER_HTML)
	@echo Coverage HTML written to $(COVER_HTML)

docker-build:
	docker build -f Dockerfile -t $(SERVICE_NAME):latest ../..

clean:
	@if exist $(BIN_DIR) rmdir /s /q $(BIN_DIR)
	@if exist $(COVER_OUT) del /f /q $(COVER_OUT)
	@if exist $(COVER_HTML) del /f /q $(COVER_HTML)
	@if exist test\e2e\covered.out del /f /q test\e2e\covered.out
