# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy dependency manifests first (monorepo + local replace target) so that
COPY packages/go/go.mod packages/go/go.sum ./packages/go/
COPY services/payment-service/go.mod services/payment-service/go.sum ./services/payment-service/

# Resolve dependencies (module cache is persisted across builds via BuildKit)
WORKDIR /app/services/payment-service
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy shared packages to workspace root for module replace
WORKDIR /app
COPY packages/go ./packages/go
# Copy payment-service source
WORKDIR /app/services/payment-service
COPY services/payment-service .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-w -s" -o /workspace/bin/payment-service ./cmd/server

# Runtime stage
FROM alpine:3.20 AS runtime

RUN apk --no-cache add ca-certificates tzdata

# Copy the binary
COPY --from=builder /workspace/bin/payment-service /payment-service

# Expose ports: GraphQL, gRPC, metrics, webhook
EXPOSE 4002 50056 9106 4012

ENTRYPOINT ["/payment-service"]