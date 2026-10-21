# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY packages/go/go.mod packages/go/go.sum ./packages/go/
COPY services/driver-service/go.mod services/driver-service/go.sum ./services/driver-service/

WORKDIR /app/services/driver-service
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

WORKDIR /app
COPY packages/go ./packages/go

WORKDIR /app/services/driver-service
COPY services/driver-service .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-w -s" -o /workspace/bin/driver-service ./cmd/server

# ─── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.20 AS runtime

RUN apk --no-cache add ca-certificates tzdata

# Copy the binary
COPY --from=builder /workspace/bin/driver-service /driver-service

# Expose ports: GraphQL, gRPC, metrics
EXPOSE 4008 50055 9105

ENTRYPOINT ["/driver-service"]