# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy dependency manifests first (monorepo + local replace target) so that
COPY packages/go/go.mod packages/go/go.sum ./packages/go/
COPY services/search-service/go.mod services/search-service/go.sum ./services/search-service/

# Resolve dependencies (module cache is persisted across builds via BuildKit)
WORKDIR /app/services/search-service
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy shared packages to workspace root for module replace
WORKDIR /app
COPY packages/go /app/packages/go
# Copy search-service source
WORKDIR /app/services/search-service
COPY services/search-service /app/services/search-service

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-w -s" -o /search-service ./cmd/server

# Run stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /search-service .

EXPOSE 4007 9103

ENTRYPOINT ["./search-service"]
