.PHONY: help infra-up infra-down dev-up dev-down k8s-validate skaffold-dev skaffold-delete proto-generate

ENV ?= development
ENV_FILE := config/env/.env.$(ENV)
COMPOSE := docker-compose --env-file $(ENV_FILE)

help:
	@echo "Usage: make <target> ENV=test|development|production"
	@echo "  make infra-up ENV=development"
	@echo "  make infra-down ENV=development"
	@echo "  make dev-up ENV=development"
	@echo "  make dev-down ENV=development"
	@echo "  make k8s-validate"
	@echo "  make skaffold-dev"
	@echo "  make skaffold-delete"
	@echo "  make proto-generate"

infra-up:
	$(COMPOSE) -f infrastructure/docker/compose.infra.yml up -d

infra-down:
	$(COMPOSE) -f infrastructure/docker/compose.infra.yml down

dev-up:
	$(COMPOSE) -f infrastructure/docker/compose.yml -f infrastructure/docker/compose.integration.yml up --build -d

dev-down:
	$(COMPOSE) -f infrastructure/docker/compose.yml -f infrastructure/docker/compose.integration.yml down

k8s-validate:
	kubectl kustomize infrastructure/kubernetes/overlays/development

skaffold-dev:
	skaffold dev -f infrastructure/skaffold/skaffold.yaml --cache-artifacts=true --build-concurrency=0

skaffold-delete:
	skaffold delete -f infrastructure/skaffold/skaffold.yaml

proto-generate:
	@echo "Run the protobuf generation commands documented in infrastructure/commands.txt"
