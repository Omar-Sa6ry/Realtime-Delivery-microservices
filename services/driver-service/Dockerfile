# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy shared packages
COPY packages/go /app/packages/go

# Copy service files
COPY services/driver-service /app/services/driver-service

WORKDIR /app/services/driver-service

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /driver-service ./cmd/server

# Run stage
FROM alpine:3.19 AS runtime

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root

COPY --from=builder /driver-service .

EXPOSE 4008 50055 9105

ENTRYPOINT ["/driver-service"]