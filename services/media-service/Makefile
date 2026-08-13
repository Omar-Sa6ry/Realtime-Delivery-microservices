.PHONY: build tidy run help

# ── Dependencies ───────────────────────────────────────────────────────────────
tidy:
	go mod tidy

# ── Build ──────────────────────────────────────────────────────────────────────
build:
	CGO_ENABLED=0 go build -o bin/media-service ./cmd/server

# ── Run ────────────────────────────────────────────────────────────────────────
run:
	go run ./cmd/server

# ── Test ───────────────────────────────────────────────────────────────────────
test:
	go test ./... -v -race

# ── Vet ────────────────────────────────────────────────────────────────────────
vet:
	go vet ./...

# ── Docker ─────────────────────────────────────────────────────────────────────
# Build from the monorepo root (Dockerfile uses relative paths to packages/go)
docker-build:
	docker build \
		-f deployments/docker/Dockerfile \
		-t media-service:latest \
		../../

# ── Help ───────────────────────────────────────────────────────────────────────
help:
	@echo "make tidy        Download and tidy dependencies"
	@echo "make build       Compile the binary"
	@echo "make run         Run the service locally"
	@echo "make test        Run all tests"
	@echo "make vet         Run go vet"
	@echo "make docker-build  Build the Docker image"
