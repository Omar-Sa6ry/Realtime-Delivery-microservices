SHELL := cmd.exe
.SHELLFLAGS := /c

PROJECT_NAME := delivery-service
NPM := npm
NEST := npx nest
DOCKER := docker
DOCKER_COMPOSE := docker-compose

.PHONY: help install install-ci build build-dev start start-dev start-debug start-prod lint lint-check format format-check test test-watch test-cov test-e2e test-debug coverage-report clean clean-dist clean-coverage docker-build docker-up docker-down docker-restart docker-logs docker-ps migrate-create migrate-run migrate-revert migrate-status seed db-reset outdated update-deps security-audit sonarqube ci-check pre-commit

help:
	@echo Available targets:
	@echo.
	@echo Setup ^& Installation:
	@echo   install              - Install all dependencies
	@echo   install-ci           - Install dependencies for CI (no optional deps)
	@echo.
	@echo Build:
	@echo   build                - Build the project for production
	@echo   build-dev            - Build the project in development mode
	@echo.
	@echo Run:
	@echo   start                - Start the application
	@echo   start-dev            - Start in development mode with watch
	@echo   start-debug          - Start in debug mode with watch
	@echo   start-prod           - Start production build
	@echo.
	@echo Code Quality:
	@echo   lint                 - Run ESLint with auto-fix
	@echo   lint-check           - Run ESLint without fixing
	@echo   format               - Format code with Prettier
	@echo   format-check         - Check code formatting without fixing
	@echo.
	@echo Testing:
	@echo   test                 - Run unit tests
	@echo   test-watch           - Run unit tests in watch mode
	@echo   test-cov             - Run unit tests with coverage
	@echo   test-e2e             - Run e2e tests
	@echo   test-debug           - Run tests in debug mode
	@echo   coverage-report      - Generate and open coverage report
	@echo.
	@echo Docker:
	@echo   docker-build         - Build Docker image
	@echo   docker-up            - Start services with docker-compose
	@echo   docker-down          - Stop services with docker-compose
	@echo   docker-logs          - View docker-compose logs
	@echo   docker-ps            - List running containers
	@echo   docker-restart       - Restart services
	@echo.
	@echo Database:
	@echo   migrate-create       - Create new migration (usage: make migrate-create NAME=migration_name)
	@echo   migrate-run          - Run pending migrations
	@echo   migrate-revert       - Revert last migration
	@echo   migrate-status       - Show migration status
	@echo   seed                 - Run database seeds
	@echo   db-reset             - Reset database (drop, create, migrate, seed)
	@echo.
	@echo Utilities:
	@echo   clean                - Clean build artifacts and node_modules
	@echo   clean-dist           - Clean only dist folder
	@echo   clean-coverage       - Clean only coverage folder
	@echo   outdated             - Check for outdated dependencies
	@echo   update-deps          - Update dependencies to latest versions
	@echo   security-audit       - Run npm audit
	@echo   sonarqube            - Run SonarQube scanner
	@echo.
	@echo CI/CD:
	@echo   ci-check             - Run full CI pipeline locally
	@echo   pre-commit           - Run pre-commit checks

install:
	$(NPM) install

install-ci:
	$(NPM) ci

build:
	$(NEST) build

build-dev:
	$(NEST) build --watch

start:
	$(NPM) run start

start-dev:
	$(NPM) run start:dev

start-debug:
	$(NPM) run start:debug

start-prod:
	$(NPM) run start:prod

lint:
	$(NPM) run lint

lint-check:
	$(NPM) run lint -- --max-warnings=0

format:
	$(NPM) run format

format-check:
	$(NPM) run format -- --check

test:
	$(NPM) run test

test-watch:
	$(NPM) run test:watch

test-cov:
	$(NPM) run test:cov

test-e2e:
	$(NPM) run test:e2e

test-debug:
	$(NPM) run test:debug

coverage-report:
	$(NPM) run test:cov && start coverage/lcov-report/index.html

docker-build:
	$(DOCKER) build -t $(PROJECT_NAME):latest .

docker-up:
	$(DOCKER_COMPOSE) up -d

docker-down:
	$(DOCKER_COMPOSE) down

docker-restart:
	$(DOCKER_COMPOSE) restart

docker-logs:
	$(DOCKER_COMPOSE) logs -f

docker-ps:
	$(DOCKER_COMPOSE) ps

migrate-create:
	@if not defined NAME (echo Error: NAME is required. Usage: make migrate-create NAME=migration_name ^& exit /b 1)
	$(NEST) g migration $(NAME)

migrate-run:
	$(NPM) run migration:run

migrate-revert:
	$(NPM) run migration:revert

migrate-status:
	$(NPM) run migration:status

seed:
	$(NPM) run seed

db-reset:
	$(NPM) run db:reset

clean:
	if exist dist rmdir /s /q dist
	if exist coverage rmdir /s /q coverage
	if exist node_modules rmdir /s /q node_modules
	if exist .turbo rmdir /s /q .turbo

clean-dist:
	if exist dist rmdir /s /q dist

clean-coverage:
	if exist coverage rmdir /s /q coverage

outdated:
	$(NPM) outdated

update-deps:
	$(NPM) update

security-audit:
	$(NPM) audit
	$(NPM) audit fix

sonarqube:
	npx sonarqube-scanner

ci-check: install lint format-check test-cov test-e2e

pre-commit: lint format-check test

.DEFAULT_GOAL := help