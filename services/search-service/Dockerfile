# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy shared packages
COPY packages/go /app/packages/go

# Copy service files
COPY services/search-service /app/services/search-service

WORKDIR /app/services/search-service

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /search-service ./cmd/server

# Run stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /search-service .

EXPOSE 4007 9103

ENTRYPOINT ["./search-service"]
