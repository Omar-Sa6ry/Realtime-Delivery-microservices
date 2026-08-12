PHONY: proto build tidy run help

# ── Generate proto ─────────────────────────────────────────────────────────────
# Run once to generate gRPC Go code from the proto definition.
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
# Install deps: make proto-deps
proto-deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	protoc \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		proto/media/v1/media.proto

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
	@echo "make proto-deps  Install protoc plugins"
	@echo "make proto       Generate gRPC Go code"
	@echo "make tidy        Download and tidy dependencies"
	@echo "make build       Compile the binary"
	@echo "make run         Run the service locally"
	@echo "make test        Run all tests"
	@echo "make vet         Run go vet"
	@echo "make docker-build  Build the Docker image"
