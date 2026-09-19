# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy shared packages
COPY packages/go ./packages/go

# Copy service files
COPY services/analytics-service ./services/analytics-service

WORKDIR /app/services/analytics-service

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /workspace/bin/analytics-service ./cmd/server

# Runtime stage
FROM alpine:3.20 AS runtime

RUN apk --no-cache add ca-certificates tzdata

# Copy the binary
COPY --from=builder /workspace/bin/analytics-service /analytics-service

# Expose ports: GraphQL, gRPC, metrics
EXPOSE 4009 50057 9107

ENTRYPOINT ["/analytics-service"]