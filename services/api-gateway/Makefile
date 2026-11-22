.PHONY: help install install-ci prune audit audit-fix outdated build rebuild start start-dev start-debug start-prod dev run format format-check lint lint-check lint-fix typecheck verify ci all test unit test-watch test-cov cov coverage test-e2e e2e test-e2e-cov test-all test-debug coverage-summary coverage-report coverage-open smoke health smoke-graphql smoke-all env-check versions docker-build docker-up docker-down docker-logs docker-restart docker-shell docker-status up down logs restart clean user-help user-install user-install-ci user-ci user-prune user-audit user-audit-fix user-outdated user-build user-rebuild user-start user-run user-start-dev user-start-debug user-start-prod user-dev user-format user-format-check user-lint user-lint-check user-typecheck user-verify user-test user-test-unit user-test-watch user-test-cov user-test-e2e user-test-e2e-cov user-test-all user-test-debug user-coverage-summary user-coverage-report user-smoke user-smoke-health user-smoke-ready user-smoke-metrics user-db-check user-wait-for-db user-infra-up user-infra-down user-docker-build user-docker-up user-docker-down user-docker-logs user-docker-restart user-docker-shell user-docker-status user-clean user-purge smoke-user verify-all test-all-services

# --------------------------------------------------------------- Variables

# Runtime environment used by the docker-* targets (ENV=production etc.)
ENV ?= development
ENV_FILE := ../../config/env/.env.$(ENV)
COMPOSE := docker compose -f ../../infrastructure/docker/compose.yml --env-file $(ENV_FILE)

# Base URL probed by `make smoke` against an already running gateway
GATEWAY_URL ?= http://localhost:4000
export GATEWAY_URL

# Base URL probed by `make smoke-user` against a running user-service
USER_URL ?= http://localhost:4001
export USER_URL

# Every user-service command below delegates to the service Makefile
USER_MAKE := $(MAKE) -C ../user-service

# --------------------------------------------------------------------- Help
# Recipes run under cmd.exe on Windows, so echo strings must not use quotes,
# pipes, redirects or ampersands.
help:
	@echo API Gateway - Available Commands
	@echo.
	@echo Usage: make ^<target^> ENV=test^|development^|production
	@echo.
	@echo Development:
	@echo   make install          - Install dependencies: npm install --legacy-peer-deps
	@echo   make install-ci       - Reproducible install from package-lock.json: npm ci
	@echo   make start            - Build and run once
	@echo   make start-dev        - Start in development mode with hot reload
	@echo   make start-debug      - Start in debug mode
	@echo   make start-prod       - Run the production build
	@echo   make dev              - Alias of start-dev
	@echo   make run              - Alias of start
	@echo   make format           - Format code with Prettier
	@echo   make lint             - Run ESLint with auto-fix
	@echo.
	@echo Code quality:
	@echo   make lint-check       - Run ESLint without fixing
	@echo   make lint-fix         - Alias of lint
	@echo   make format-check     - Check Prettier formatting
	@echo   make typecheck        - Type-check with tsc --noEmit
	@echo   make build            - Build for production
	@echo   make rebuild          - Clean then build
	@echo   make verify           - Full pipeline: format, lint, typecheck, build, tests
	@echo   make ci               - install-ci followed by verify
	@echo.
	@echo Dependencies:
	@echo   make audit            - npm audit, exits non-zero on high and critical findings
	@echo   make audit-fix        - npm audit fix, writes package-lock.json
	@echo   make outdated         - List upgradable packages, exits non-zero if any exist
	@echo   make prune            - Remove node_modules entries not listed in package.json
	@echo.
	@echo Testing:
	@echo   make test             - Run unit tests
	@echo   make unit             - Alias of test
	@echo   make test-watch       - Run unit tests in watch mode
	@echo   make test-cov         - Unit tests with coverage report
	@echo   make cov              - Alias of test-cov
	@echo   make test-e2e         - Run end-to-end tests
	@echo   make e2e              - Alias of test-e2e
	@echo   make test-e2e-cov     - E2E tests with coverage report
	@echo   make test-all         - Unit coverage plus end-to-end tests
	@echo   make all              - Alias of test-all
	@echo   make test-debug       - Run unit tests in debug mode
	@echo   make coverage-summary - Print the last coverage summary
	@echo   make coverage-report  - Print the path of the HTML coverage report
	@echo   make coverage-open    - Open the HTML coverage report in a browser
	@echo.
	@echo Operations:
	@echo   make smoke            - Probe a running gateway: GET /ping and GET /health
	@echo                         Override target: make smoke GATEWAY_URL=http://host:4000
	@echo   make health           - Alias of smoke
	@echo   make smoke-graphql    - POST an introspection query to /graphql
	@echo   make smoke-all        - smoke plus smoke-graphql
	@echo   make env-check        - Verify config/env/.env.ENV exists
	@echo   make versions         - Print node, npm, docker and make versions
	@echo.
	@echo Docker:
	@echo   make docker-build     - Build the api-gateway image
	@echo   make docker-up        - Start api-gateway in compose
	@echo   make docker-down      - Stop and remove the api-gateway container
	@echo   make docker-logs      - Tail api-gateway logs
	@echo   make docker-restart   - Restart the api-gateway container
	@echo   make docker-shell     - Open a shell inside the running container
	@echo   make docker-status    - Show the api-gateway container status
	@echo   make up, down, logs, restart - Short aliases of the docker targets
	@echo   Use ENV=production to target another environment file
	@echo.
	@echo User service commands, delegated to the user-service Makefile:
	@echo   make user-help            - Print the user-service command list
	@echo   make user-install         - npm install for user-service
	@echo   make user-install-ci      - npm ci for user-service
	@echo   make user-ci              - npm ci followed by user-service verify
	@echo   make user-build           - Build user-service for production
	@echo   make user-start           - Build and run user-service once
	@echo   make user-run             - Alias of user-start
	@echo   make user-start-dev       - Start user-service with hot reload
	@echo   make user-start-debug     - Start user-service in debug mode
	@echo   make user-start-prod      - Run the user-service production build
	@echo   make user-format          - Format user-service code
	@echo   make user-lint            - Lint user-service code with auto-fix
	@echo   make user-lint-check      - Lint user-service code without fixing
	@echo   make user-format-check    - Check user-service formatting
	@echo   make user-typecheck       - Type-check user-service
	@echo   make user-test            - user-service unit tests
	@echo   make user-test-unit       - Alias of user-test
	@echo   make user-test-watch      - user-service unit tests in watch mode
	@echo   make user-test-cov        - user-service unit tests with coverage
	@echo   make user-test-e2e        - user-service end-to-end tests
	@echo   make user-test-e2e-cov    - user-service e2e tests with coverage
	@echo   make user-test-all        - user-service unit coverage plus e2e
	@echo   make user-test-debug      - user-service unit tests in debug mode
	@echo   make user-coverage-summary - Print the last user-service coverage totals
	@echo   make user-coverage-report - Print the path of the user-service HTML report
	@echo   make user-verify          - user-service format, lint, typecheck, build, tests
	@echo   make user-audit           - npm audit for user-service
	@echo   make user-audit-fix       - npm audit fix for user-service
	@echo   make user-outdated        - Upgradable packages for user-service
	@echo   make user-prune           - npm prune for user-service
	@echo   make user-smoke           - Probe a running user-service: health, ready, metrics
	@echo   make user-smoke-health    - Probe GET /user/health only
	@echo   make user-smoke-ready     - Probe GET /user/ready only
	@echo   make user-smoke-metrics   - Probe GET /user/metrics only
	@echo   make smoke-user           - Alias of user-smoke
	@echo   make user-db-check        - Verify the PostgreSQL used by user-service e2e
	@echo   make user-wait-for-db     - Retry until that PostgreSQL is reachable
	@echo   make user-infra-up        - Start the user-service db, redis and nats containers
	@echo   make user-infra-down      - Stop those containers
	@echo   make user-docker-build    - Build the user-service image
	@echo   make user-docker-up       - Start the user-service compose stack
	@echo   make user-docker-down     - Stop the user-service compose stack
	@echo   make user-docker-logs     - Tail user-service container logs
	@echo   make user-docker-restart  - Restart the user-service container
	@echo   make user-docker-shell    - Open a shell inside user-service
	@echo   make user-docker-status   - Show user-service container status
	@echo   make user-clean           - Remove user-service build output and caches
	@echo   make user-purge           - user-clean plus node_modules
	@echo   make verify-all           - Verify api-gateway and user-service
	@echo   make test-all-services    - Run both test suites: unit coverage plus e2e
	@echo   Override the probe target: make user-smoke USER_URL=http://host:4001
	@echo.
	@echo Maintenance:
	@echo   make clean            - Remove build output, coverage and caches

# --------------------------------------------------------------- Development

install:
	npm install --legacy-peer-deps

install-ci:
	npm ci --legacy-peer-deps

start: build
	npm run start

start-dev:
	npm run start:dev

start-debug:
	npm run start:debug

start-prod:
	npm run start:prod

dev: start-dev

run: start

format:
	npm run format

lint:
	npm run lint

# ------------------------------------------------------------- Code quality

lint-check:
	npm run lint:check

lint-fix: lint

format-check:
	npm run format:check

typecheck:
	npm run typecheck

build:
	npm run build

rebuild: clean build

verify:
	npm run verify

ci: install-ci verify

# -------------------------------------------------------------- Dependencies

audit:
	npm audit --audit-level=high

audit-fix:
	npm audit fix --legacy-peer-deps

outdated:
	npm outdated

prune:
	npm prune --legacy-peer-deps

# -------------------------------------------------------------------- Testing

test:
	npm run test

unit: test

test-watch:
	npm run test:watch

test-cov:
	npm run test:cov

cov: test-cov

coverage: test-cov

test-e2e:
	npm run test:e2e

e2e: test-e2e

test-e2e-cov:
	npm run test:e2e:cov

test-all:
	npm run test:all

all: test-all

test-debug:
	npm run test:debug

coverage-summary:
	node -e "const fs=require('fs');const p='coverage/coverage-summary.json';if(!fs.existsSync(p)){console.error('No coverage/coverage-summary.json - run make test-cov first');process.exit(1)}const s=JSON.parse(fs.readFileSync(p,'utf8')).total;const pct=(m)=>s[m].pct+'% ('+s[m].covered+'/'+s[m].total+')';console.log('statements:',pct('statements'));console.log('branches:  ',pct('branches'));console.log('functions: ',pct('functions'));console.log('lines:     ',pct('lines'));"

coverage-report:
	node -e "const fs=require('fs');const p='coverage/lcov-report/index.html';if(!fs.existsSync(p)){console.error('No coverage/lcov-report/index.html - run make test-cov first');process.exit(1)}console.log('HTML report: '+require('path').resolve(p));"

coverage-open:
	@node -e "const fs=require('fs');const path=require('path');const p='coverage/lcov-report/index.html';if(!fs.existsSync(p)){console.error('No coverage/lcov-report/index.html - run make test-cov first');process.exit(1)}const url=path.resolve(p);const {spawn}=require('child_process');const child=process.platform==='win32'?spawn('cmd',['/c','start',String.fromCharCode(34)+String.fromCharCode(34),url],{stdio:'ignore',detached:true}):spawn(process.platform==='darwin'?'open':'xdg-open',[url],{stdio:'ignore',detached:true});child.unref();console.log('opening '+url);"

# ----------------------------------------------------------------- Operations

smoke:
	@node -e "const base=process.env.GATEWAY_URL||'http://localhost:4000';const paths=['/ping','/health'];(async()=>{let bad=false;for(const p of paths){try{const r=await fetch(base+p);const t=await r.text();console.log('GET '+p+' -> '+r.status+' '+t.slice(0,160));if(!r.ok){bad=true}}catch(e){console.error('GET '+p+' -> FAILED: '+e.message);bad=true}}if(bad){console.error('smoke failed against '+base);process.exit(1)}console.log('smoke passed against '+base)})()"

health: smoke

smoke-graphql:
	@node -e "const base=process.env.GATEWAY_URL||'http://localhost:4000';(async()=>{try{const r=await fetch(base+'/graphql',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({query:'query { __typename }'})});const t=await r.text();console.log('POST /graphql -> '+r.status+' '+t.slice(0,160));const ok=r.ok&&t.includes('__typename');if(!ok){console.error('smoke-graphql failed against '+base);process.exit(1)}console.log('smoke-graphql passed against '+base)}catch(e){console.error('POST /graphql -> FAILED: '+e.message);process.exit(1)}})()"

smoke-all: smoke smoke-graphql

env-check:
	@node -e "const fs=require('fs');const p='$(ENV_FILE)';if(!fs.existsSync(p)){console.error('Missing environment file: '+p);process.exit(1)}console.log('environment file OK: '+p);"

versions:
	@node --version
	@npm --version
	@docker --version
	@make --version

# ---------------------------------------------------------------------- Docker

docker-build:
	$(COMPOSE) build api-gateway

docker-up:
	$(COMPOSE) up -d api-gateway

docker-down:
	$(COMPOSE) rm -sf api-gateway

docker-logs:
	$(COMPOSE) logs -f api-gateway

docker-restart:
	$(COMPOSE) restart api-gateway

docker-shell:
	$(COMPOSE) exec api-gateway sh

docker-status:
	$(COMPOSE) ps api-gateway

up: docker-up

down: docker-down

logs: docker-logs

restart: docker-restart

# ---------------------------------------------------- Maintenance

clean:
	npm run clean

# ---------------------------------------------------- User service delegation

user-clean:
	$(MAKE) -C ../user-service clean

user-purge:
	$(MAKE) -C ../user-service purge

verify-all: verify user-verify

test-all-services: test-all user-test-all

