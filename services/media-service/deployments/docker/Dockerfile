# ─── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# Install build tools
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /workspace

# Copy shared packages first for layer caching
COPY packages/go ./packages/go

# Copy media-service source
COPY services/media-service ./services/media-service

# Resolve dependencies
WORKDIR /workspace/services/media-service
RUN go mod download

# Build with optimisation flags
# CGO disabled for a fully static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /workspace/bin/media-service \
    ./cmd/server

# ─── Stage 2: Runtime ──────────────────────────────────────────────────────────
# Use distroless for minimal attack surface — no shell, no package manager.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# Copy timezone data and CA certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /workspace/bin/media-service /media-service

# gRPC port and metrics port
EXPOSE 4002 9102

# Run as nonroot user (distroless default)
USER nonroot:nonroot

ENTRYPOINT ["/media-service"]
