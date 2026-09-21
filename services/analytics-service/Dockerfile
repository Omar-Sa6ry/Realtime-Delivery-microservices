# ─── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# Install build tools (git guards VCS-hosted Go modules).
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /workspace

# Copy dependency manifests first (monorepo + local replace target) so that
# `go mod download` can run before the full source copy for better layer caching.
COPY packages/go/go.mod packages/go/go.sum ./packages/go/
COPY services/analytics-service/go.mod services/analytics-service/go.sum ./services/analytics-service/

# Resolve dependencies (module cache is persisted across builds via BuildKit).
WORKDIR /workspace/services/analytics-service
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy shared packages to workspace root for module replace.
WORKDIR /workspace
COPY packages/go ./packages/go
# Copy analytics-service source.
WORKDIR /workspace/services/analytics-service
COPY services/analytics-service .

# Build with optimisation flags (build cache persisted via BuildKit).
# CGO disabled for a fully static binary.
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build \
    -ldflags="-w -s" \
    -o /workspace/bin/analytics-service \
    ./cmd/server

# ─── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.20 AS runtime

RUN apk --no-cache add ca-certificates tzdata

# Copy the binary.
COPY --from=builder /workspace/bin/analytics-service /analytics-service

# Expose ports: GraphQL (4009), gRPC (50057, future use), metrics (9107).
EXPOSE 4009 50057 9107

ENTRYPOINT ["/analytics-service"]
