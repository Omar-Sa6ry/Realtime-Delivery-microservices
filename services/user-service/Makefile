.PHONY: help install install-ci prune audit audit-fix outdated build rebuild start start-dev start-debug start-prod dev run format format-check lint lint-check typecheck verify ci test test-unit test-watch test-cov test-e2e test-e2e-cov test-all test-debug coverage-summary coverage-report smoke smoke-health smoke-ready smoke-metrics db-check wait-for-db infra-up infra-down docker-build docker-up docker-down docker-logs docker-restart docker-shell docker-status clean purge

# ----------------------------------------------------------------- Variables
# Environment selection for compose / config files: ENV=test|development|production
ENV ?= development
APP_ENV ?= $(ENV)
export APP_ENV

# Compose file that ships with this service (db + redis + nats + the app itself)
COMPOSE := docker compose -f docker-compose.yml

# HTTP base URL used by the smoke targets
USER_URL ?= http://localhost:4001
export USER_URL

# PostgreSQL endpoint used by db-check / wait-for-db. The e2e suite reads the
# same values from E2E_DB_* environment variables, so both always target one
# identical database (a pre-set E2E_DB_* wins over the default below).
# 127.0.0.1 instead of `localhost`: Node prefers the IPv6 loopback (::1) when
# resolving `localhost`, while docker publishes the port on IPv4 only, which
# makes the TCP handshake hang and fail with `read ECONNRESET`.
DB_HOST ?= $(if $(E2E_DB_HOST),$(E2E_DB_HOST),127.0.0.1)
DB_PORT ?= $(if $(E2E_DB_PORT),$(E2E_DB_PORT),5433)
DB_USER ?= $(if $(E2E_DB_USER),$(E2E_DB_USER),postgres)
DB_PASSWORD ?= $(if $(E2E_DB_PASSWORD),$(E2E_DB_PASSWORD),postgres)
DB_NAME ?= $(if $(E2E_DB_NAME),$(E2E_DB_NAME),delivery_user_db_e2e)

export E2E_DB_HOST := $(DB_HOST)
export E2E_DB_PORT := $(DB_PORT)
export E2E_DB_USER := $(DB_USER)
export E2E_DB_PASSWORD := $(DB_PASSWORD)
export E2E_DB_NAME := $(DB_NAME)

# How long wait-for-db keeps retrying (attempts x 2 seconds)
DB_ATTEMPTS ?= 60

# --------------------------------------------------------------------- Help
# Recipes run under cmd.exe on Windows, so echo strings must not use quotes,
# pipes, redirects or ampersands.
help:
	@echo User Service - Available Commands
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
	@echo.
	@echo Code quality:
	@echo   make format           - Format code with Prettier
	@echo   make format-check     - Verify Prettier formatting without writing
	@echo   make lint             - Run ESLint with auto-fix
	@echo   make lint-check       - Run ESLint without fixing
	@echo   make typecheck        - Type-check with tsc --noEmit
	@echo   make build            - Build for production
	@echo   make rebuild          - Clean then build
	@echo   make verify           - format-check, lint-check, typecheck, build, test-all
	@echo   make ci               - install-ci followed by verify
	@echo.
	@echo Dependencies:
	@echo   make audit            - npm audit, non-zero exit on high and critical
	@echo   make audit-fix        - npm audit fix, writes package-lock.json
	@echo   make outdated         - List upgradable packages
	@echo   make prune            - Remove node_modules entries not in package.json
	@echo.
	@echo Testing:
	@echo   make test             - Run unit tests
	@echo   make test-unit        - Alias of test
	@echo   make test-watch       - Run unit tests in watch mode
	@echo   make test-cov         - Run unit tests with coverage report
	@echo   make test-e2e         - Check the database, then run end-to-end tests
	@echo   make test-e2e-cov     - End-to-end tests with coverage report
	@echo   make test-all         - Unit coverage plus end-to-end tests
	@echo   make test-debug       - Run unit tests in debug mode
	@echo   make coverage-summary - Print the last coverage totals
	@echo   make coverage-report  - Print the path of the HTML coverage report
	@echo.
	@echo Operations:
	@echo   make smoke            - Probe a running service: health, ready, metrics
	@echo                         Override target: make smoke USER_URL=http://host:4001
	@echo   make smoke-health     - Probe only GET /user/health
	@echo   make smoke-ready      - Probe only GET /user/ready
	@echo   make smoke-metrics    - Probe only GET /user/metrics
	@echo.
	@echo Database:
	@echo   make db-check         - Verify PostgreSQL is reachable
	@echo   make wait-for-db      - Retry until PostgreSQL is reachable
	@echo   make infra-up         - Start this service compose infra: postgres, redis, nats
	@echo   make infra-down       - Stop this service compose infra
	@echo   Note: db-check reads DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
	@echo.
	@echo Docker:
	@echo   make docker-build     - Build the user-service image through compose
	@echo   make docker-up        - Start the stack defined in docker-compose.yml
	@echo   make docker-down      - Stop and remove the stack
	@echo   make docker-logs      - Tail the user-service logs
	@echo   make docker-restart   - Restart the user-service container
	@echo   make docker-shell     - Open a shell inside the running container
	@echo   make docker-status    - Show container status
	@echo   Use ENV=production to target another environment file
	@echo.
	@echo Maintenance:
	@echo   make clean            - Remove build output, coverage and caches
	@echo   make purge            - clean plus node_modules

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

# ------------------------------------------------------------- Code quality

format:
	npm run format

format-check:
	npx prettier --check "src/**/*.ts" "test/**/*.ts"

lint:
	npm run lint

lint-check:
	npx eslint "{src,apps,libs,test}/**/*.ts"

typecheck:
	npx tsc -p tsconfig.json --noEmit

build:
	npm run build

rebuild: clean build

verify: format-check lint-check typecheck build test-all

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

test-unit: test

test-watch:
	npm run test:watch

test-cov:
	npm run test:cov

test-e2e: db-check
	npm run test:e2e

test-e2e-cov: db-check
	npx jest --config ./test/jest-e2e.json --coverage --coverageDirectory=../coverage/e2e

test-all: test-cov test-e2e

test-debug:
	npm run test:debug

coverage-summary:
	node -e "const fs=require('fs');const p='coverage/coverage-summary.json';if(!fs.existsSync(p)){console.error('No coverage/coverage-summary.json - run make test-cov first');process.exit(1)}const s=JSON.parse(fs.readFileSync(p,'utf8')).total;const pct=(m)=>s[m].pct+'% ('+s[m].covered+'/'+s[m].total+')';console.log('statements:',pct('statements'));console.log('branches:  ',pct('branches'));console.log('functions: ',pct('functions'));console.log('lines:     ',pct('lines'));"

coverage-report:
	node -e "const fs=require('fs');const p='coverage/lcov-report/index.html';if(!fs.existsSync(p)){console.error('No coverage/lcov-report/index.html - run make test-cov first');process.exit(1)}console.log('HTML report: '+require('path').resolve(p));"

# ----------------------------------------------------------------- Operations

smoke: smoke-health smoke-ready smoke-metrics

smoke-health:
	@node -e "const base=process.env.USER_URL||'http://localhost:4001';(async()=>{try{const r=await fetch(base+'/user/health');const t=await r.text();console.log('GET /user/health -> '+r.status+' '+t.slice(0,160));process.exit(r.ok?0:1)}catch(e){console.error('GET /user/health -> FAILED: '+e.message);process.exit(1)}})()"

smoke-ready:
	@node -e "const base=process.env.USER_URL||'http://localhost:4001';(async()=>{try{const r=await fetch(base+'/user/ready');const t=await r.text();console.log('GET /user/ready -> '+r.status+' '+t.slice(0,160));process.exit(r.ok?0:1)}catch(e){console.error('GET /user/ready -> FAILED: '+e.message);process.exit(1)}})()"

smoke-metrics:
	@node -e "const base=process.env.USER_URL||'http://localhost:4001';(async()=>{try{const r=await fetch(base+'/user/metrics');const t=await r.text();console.log('GET /user/metrics -> '+r.status+' '+t.slice(0,160));process.exit(r.ok?0:1)}catch(e){console.error('GET /user/metrics -> FAILED: '+e.message);process.exit(1)}})()"

# ------------------------------------------------------------------- Database

db-check:
	@node -e "const {Client}=require('pg');const host='$(DB_HOST)';const port=Number('$(DB_PORT)');const user='$(DB_USER)';const password='$(DB_PASSWORD)';const database='$(DB_NAME)';(async()=>{const c=new Client({host,port,user,password,database:'postgres'});try{await c.connect();const r=await c.query('select datname from pg_database where datname=$$1',[database]);const state=r.rowCount?'exists':'will be created by the e2e harness';console.log('PostgreSQL reachable at '+host+':'+port+' - database '+database+' '+state);process.exit(0)}catch(e){console.error('PostgreSQL NOT reachable at '+host+':'+port+' - '+e.message);console.error('Start it first with: make infra-up');process.exit(1)}finally{await c.end().catch(()=>{})}})()"

wait-for-db:
	@node -e "const {Client}=require('pg');const host='$(DB_HOST)';const port=Number('$(DB_PORT)');const user='$(DB_USER)';const password='$(DB_PASSWORD)';const attempts=Number('$(DB_ATTEMPTS)');(async()=>{for(let i=1;i<=attempts;i++){const c=new Client({host,port,user,password,database:'postgres'});try{await c.connect();await c.end();console.log('PostgreSQL reachable at '+host+':'+port);process.exit(0)}catch(e){try{await c.end()}catch{}if(i===attempts){console.error('PostgreSQL still unreachable after '+attempts+' attempts: '+e.message);process.exit(1)}console.log('waiting for PostgreSQL at '+host+':'+port+' ('+i+'/'+attempts+')');await new Promise(r=>setTimeout(r,2000))}}})()"

infra-up:
	$(COMPOSE) up -d user-db-srv redis-srv nats-srv

infra-down:
	$(COMPOSE) rm -sf user-db-srv redis-srv nats-srv

# ---------------------------------------------------------------------- Docker

docker-build:
	$(COMPOSE) build user-service

docker-up:
	$(COMPOSE) up -d

docker-down:
	$(COMPOSE) down

docker-logs:
	$(COMPOSE) logs -f user-service

docker-restart:
	$(COMPOSE) restart user-service

docker-shell:
	$(COMPOSE) exec user-service sh

docker-status:
	$(COMPOSE) ps

# --------------------------------------------------------------- Maintenance

clean:
	@node -e "const fs=require('fs');['dist','coverage','.tsbuildinfo','tsconfig.build.tsbuildinfo','node_modules/.cache'].forEach((p)=>fs.rmSync(p,{recursive:true,force:true}));console.log('cleaned dist, coverage and caches');"

purge: clean
	@node -e "require('fs').rmSync('node_modules',{recursive:true,force:true});console.log('removed node_modules');"
