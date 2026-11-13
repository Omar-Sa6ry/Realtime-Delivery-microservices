ENV ?= development
ENV_FILE := config/env/.env.$(ENV)
COMPOSE := docker compose --env-file $(ENV_FILE)
SERVICE ?= all

.PHONY: help \
	infra-up infra-down infra-logs infra-ps \
	dev-up dev-down dev-logs dev-ps \
	integration-up integration-down \
	k8s-validate k8s-apply k8s-delete \
	skaffold-dev skaffold-run skaffold-build skaffold-logs skaffold-delete skaffold-diagnose \
	minikube-start minikube-stop minikube-status minikube-tunnel minikube-env \
	docker-ps docker-clean \
	proto-generate \
	build build-all lint format \
	test-unit test-e2e test-cov

# Help
help:
	@echo ==============================================================================
	@echo Realtime Delivery Microservices - Master Makefile
	@echo ==============================================================================
	@echo Usage: make ^<target^> [ENV=development^|test^|production] [SERVICE=^<name^>]
	@echo.
	@echo Docker Compose Targets:
	@echo   make infra-up              - Start shared infrastructure (DBs, Redis, Kafka, NATS, etc.)
	@echo   make infra-down            - Stop infrastructure containers
	@echo   make infra-logs            - Tail infrastructure containers logs
	@echo   make infra-ps              - Show infrastructure containers status
	@echo   make dev-up                - Start full dev stack with hot reload / builds
	@echo   make dev-down              - Stop full dev stack
	@echo   make dev-logs              - Tail full dev stack logs
	@echo   make dev-ps                - Show running stack containers
	@echo   make integration-up        - Start integration stack (observability ^& selected services)
	@echo   make integration-down      - Stop integration stack
	@echo   make docker-ps             - List all running project containers
	@echo   make docker-clean          - Clean unused Docker containers, networks, and images
	@echo.
	@echo Kubernetes ^& Minikube Targets:
	@echo   make k8s-validate          - Validate Kubernetes manifests via kustomize
	@echo   make k8s-apply             - Apply development overlay manifests to current cluster
	@echo   make k8s-delete            - Delete development overlay manifests from cluster
	@echo   make minikube-start        - Start local Minikube cluster (docker driver)
	@echo   make minikube-stop         - Stop Minikube cluster
	@echo   make minikube-status       - Display Minikube status
	@echo   make minikube-tunnel       - Run Minikube tunnel for ingress and loadbalancers
	@echo   make minikube-env          - Export Minikube docker environment
	@echo.
	@echo Skaffold Targets:
	@echo   make skaffold-dev          - Start Skaffold dev loop (auto-build, deploy, port-forward)
	@echo   make skaffold-run          - One-shot deploy via Skaffold without dev loop
	@echo   make skaffold-build        - Build container images via Skaffold
	@echo   make skaffold-logs         - Tail application logs via Skaffold
	@echo   make skaffold-delete       - Clean up Skaffold deployed resources
	@echo   make skaffold-diagnose     - Check Skaffold configuration health
	@echo.
	@echo Code Generation ^& Protobuf:
	@echo   make proto-generate        - Compile all Protobuf definitions for Go and NestJS services
	@echo.
	@echo Build ^& Quality Targets:
	@echo   make build                 - Build a service (or SERVICE=user-service)
	@echo   make build-all             - Build all Go and TypeScript services and packages
	@echo   make lint                  - Lint services and packages
	@echo   make format                - Format Go and TypeScript code
	@echo.
	@echo Testing Targets (3 Unified Commands):
	@echo   make test-unit             - Run unit tests across all services ^& packages (or SERVICE=user-service)
	@echo   make test-e2e              - Run E2E tests across all services (or SERVICE=payment-service)
	@echo   make test-cov              - Run test coverage across all services ^& packages (or SERVICE=delivery-service)
	@echo ==============================================================================

# Docker Compose Management
infra-up:
	$(COMPOSE) -f infrastructure/docker/compose.infra.yml up -d

infra-down:
	$(COMPOSE) -f infrastructure/docker/compose.infra.yml down

infra-logs:
	$(COMPOSE) -f infrastructure/docker/compose.infra.yml logs -f

infra-ps:
	$(COMPOSE) -f infrastructure/docker/compose.infra.yml ps

dev-up:
	$(COMPOSE) -f infrastructure/docker/compose.yml -f infrastructure/docker/compose.integration.yml up --build -d

dev-down:
	$(COMPOSE) -f infrastructure/docker/compose.yml -f infrastructure/docker/compose.integration.yml down

dev-logs:
	$(COMPOSE) -f infrastructure/docker/compose.yml -f infrastructure/docker/compose.integration.yml logs -f

dev-ps:
	$(COMPOSE) -f infrastructure/docker/compose.yml -f infrastructure/docker/compose.integration.yml ps

integration-up:
	$(COMPOSE) -f infrastructure/docker/compose.integration.yml up -d

integration-down:
	$(COMPOSE) -f infrastructure/docker/compose.integration.yml down

docker-ps:
	docker ps --format "table {{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Ports}}"

docker-clean:
	docker system prune -f

# Kubernetes & Minikube
k8s-validate:
	kubectl kustomize infrastructure/kubernetes/overlays/development

k8s-apply:
	kubectl apply -k infrastructure/kubernetes/overlays/development

k8s-delete:
	kubectl delete -k infrastructure/kubernetes/overlays/development

minikube-start:
	minikube start --driver=docker --cpus 6 --memory 15000

minikube-stop:
	minikube stop

minikube-status:
	minikube status

minikube-tunnel:
	minikube tunnel

minikube-env:
	powershell -NoProfile -Command "& minikube -p minikube docker-env | Invoke-Expression"

# Skaffold
skaffold-dev:
	skaffold dev -f infrastructure/skaffold/skaffold.yaml --cache-artifacts=true --build-concurrency=0

skaffold-run:
	skaffold run -f infrastructure/skaffold/skaffold.yaml --cache-artifacts=true

skaffold-build:
	skaffold build -f infrastructure/skaffold/skaffold.yaml

skaffold-logs:
	skaffold logs -f infrastructure/skaffold/skaffold.yaml

skaffold-delete:
	skaffold delete -f infrastructure/skaffold/skaffold.yaml

skaffold-diagnose:
	skaffold diagnose -f infrastructure/skaffold/skaffold.yaml

# Protobuf Code Generation
proto-generate:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "if (!(Test-Path services/delivery-service/internal/protos/user)) { New-Item -ItemType Directory -Force -Path services/delivery-service/internal/protos/user | Out-Null }; protoc --proto_path=. --proto_path=protos --go_out=services/delivery-service/internal/protos --go-grpc_out=services/delivery-service/internal/protos protos/user.proto; protoc --proto_path=. --proto_path=protos --go_out=services/delivery-service/internal/protos --go-grpc_out=services/delivery-service/internal/protos protos/delivery.proto; protoc --proto_path=. --proto_path=protos --go_out=services/payment-service/internal/adapters/grpc/proto --go-grpc_out=services/payment-service/internal/adapters/grpc/proto protos/payment.proto"

# Build & Quality Targets
build:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "if ('$(SERVICE)' -eq 'all') { npm --prefix packages/ts run build } else { if (Test-Path 'services/$(SERVICE)/package.json') { npm --prefix services/$(SERVICE) run build } elseif (Test-Path 'services/$(SERVICE)/go.mod') { Push-Location services/$(SERVICE); go build ./...; Pop-Location } elseif (Test-Path '$(SERVICE)/package.json') { npm --prefix $(SERVICE) run build } elseif (Test-Path '$(SERVICE)/go.mod') { Push-Location $(SERVICE); go build ./...; Pop-Location } }"

build-all:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "npm --prefix packages/ts run build; @('api-gateway','delivery-service','notification-service','realtime-service','user-service') | ForEach-Object { npm --prefix services/$$_ run build }; @('analytics-service','driver-service','media-service','payment-service','search-service') | ForEach-Object { Push-Location services/$$_; go build ./...; Pop-Location }"

lint:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "if ('$(SERVICE)' -eq 'all') { @('packages/ts','services/api-gateway','services/delivery-service','services/notification-service','services/realtime-service','services/user-service') | ForEach-Object { if (Test-Path \"$$_/package.json\") { npm --prefix $$_ run lint } } } else { if (Test-Path 'services/$(SERVICE)') { npm --prefix services/$(SERVICE) run lint } else { npm --prefix $(SERVICE) run lint } }"

format:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "if ('$(SERVICE)' -eq 'all') { @('packages/ts','services/api-gateway','services/delivery-service','services/notification-service','services/realtime-service','services/user-service') | ForEach-Object { if (Test-Path \"$$_/package.json\") { npm --prefix $$_ run format } }; @('packages/go','services/analytics-service','services/driver-service','services/media-service','services/payment-service','services/search-service') | ForEach-Object { Push-Location $$_; go fmt ./...; Pop-Location } } else { if (Test-Path 'services/$(SERVICE)/package.json') { npm --prefix services/$(SERVICE) run format } elseif (Test-Path 'services/$(SERVICE)/go.mod') { Push-Location services/$(SERVICE); go fmt ./...; Pop-Location } }"

# Unified Testing Suite (3 Commands)
test-unit:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/test-unit.ps1 -Target "$(SERVICE)"

test-e2e:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/test-e2e.ps1 -Target "$(SERVICE)"

test-cov:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/test-cov.ps1 -Target "$(SERVICE)"
