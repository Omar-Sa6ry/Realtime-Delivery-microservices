# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy shared packages
COPY packages/go ./packages/go

# Copy service files
COPY services/payment-service ./services/payment-service

WORKDIR /app/services/payment-service

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /workspace/bin/payment-service ./cmd/server

# Runtime stage
FROM alpine:3.19 AS runtime

RUN apk --no-cache add ca-certificates tzdata

# Copy the binary
COPY --from=builder /workspace/bin/payment-service /payment-service

# Expose ports: GraphQL, gRPC, metrics, webhook
EXPOSE 4002 50056 9106 4012

ENTRYPOINT ["/payment-service"]