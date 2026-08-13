# syntax=docker/dockerfile:1.4

# ─── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder


# Install build tools
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /workspace

# Copy dependency manifests first (monorepo + local replace target) so that
# `go mod download` can run before the full source copy for better layer caching.
COPY packages/go/go.mod packages/go/go.sum ./packages/go/
COPY services/media-service/go.mod services/media-service/go.sum ./services/media-service/

# Resolve dependencies (module cache is persisted across builds via BuildKit)
WORKDIR /workspace/services/media-service
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy shared packages to workspace root for module replace
WORKDIR /workspace
COPY packages/go ./packages/go
# Set workdir back to service directory for build
WORKDIR /workspace/services/media-service
# Copy media-service source
COPY services/media-service .

# Build with optimisation flags
# CGO disabled for a fully static binary. GOARCH is left unset so the binary is
# compiled natively for the container's platform architecture (AMD64 / ARM64).
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build \
    -ldflags="-w -s -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /workspace/bin/media-service \
    ./cmd/server

# ─── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19 AS runtime

# Install ffmpeg, ffprobe, and dependencies
RUN apk add --no-cache ffmpeg ca-certificates tzdata

# Copy timezone data and CA certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /workspace/bin/media-service /media-service

# GraphQL federation subgraph port, websocket port, and metrics port
EXPOSE 4005 4003 9102

ENTRYPOINT ["/media-service"]