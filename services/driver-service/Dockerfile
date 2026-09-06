# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy shared packages
COPY packages/go ./packages/go

# Copy service files
COPY services/driver-service ./services/driver-service

WORKDIR /app/services/driver-service

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /workspace/bin/driver-service ./cmd/server

# ─── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19 AS runtime

RUN apk --no-cache add ca-certificates tzdata

# Copy the binary
COPY --from=builder /workspace/bin/driver-service /driver-service

# Expose ports: GraphQL, gRPC, metrics
EXPOSE 4008 50055 9105

ENTRYPOINT ["/driver-service"]