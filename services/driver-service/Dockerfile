# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy dependency manifests first (monorepo + local replace target) so that
# `go mod download` can run before the full source copy for better layer caching.
COPY packages/go/go.mod packages/go/go.sum ./packages/go/

COPY services/driver-service/go.mod services/driver-service/go.sum ./services/driver-service/

WORKDIR /app/services/driver-service

RUN go mod download

# Copy shared packages to workspace root for module replace
WORKDIR /app
COPY packages/go ./packages/go

# Set workdir back to service directory for build
WORKDIR /app/services/driver-service
# Copy service source
COPY services/driver-service .

# Build with optimisation flags
# CGO disabled for a fully static binary. GOARCH is left unset so the binary is
# compiled natively for the container's platform architecture (AMD64 / ARM64).
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build \
    -ldflags="-w -s" \
    -o /workspace/bin/driver-service ./cmd/server

# ─── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19 AS runtime

RUN apk --no-cache add ca-certificates tzdata

# Copy the binary
COPY --from=builder /workspace/bin/driver-service /driver-service

# Expose ports: GraphQL, gRPC, metrics
EXPOSE 4008 50055 9105

ENTRYPOINT ["/driver-service"]