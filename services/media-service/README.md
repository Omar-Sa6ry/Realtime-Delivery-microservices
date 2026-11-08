# Media Service

A production-grade, distributed media processing and asset storage microservice built with Go. The service implements clean hexagonal architecture, handling large multipart uploads directly to AWS S3, asynchronous background processing pipelines, virus scanning, media transcoding, metadata extraction, distributed quotas, and realtime client notifications via WebSockets and NATS.

---

## Table of Contents

1. Architecture and Design Principles
2. Directory Structure
3. System Components and Communication Protocols
4. Media Processing Pipeline and Lifecycle
5. Data Models and Storage
6. Security and Validation Mechanisms
7. Configuration and Environment Variables
8. API Reference
   - GraphQL Federation Subgraph
   - gRPC Inter-Service Interface
   - WebSocket Realtime Events
9. Background Workers and Scheduling
10. Testing Strategy and Quality Assurance
11. Local Development and Build Commands

---

## 1. Architecture and Design Principles

The service strictly adheres to Hexagonal (Ports and Adapters) Architecture and Domain-Driven Design (DDD) principles:

- **Domain Layer (`internal/domain`)**: Contains enterprise business rules, entity models (`Media`, `UploadSession`, `MediaJob`, `MediaVersion`, `QuotaUsage`, `OutboxEvent`), state transitions, and domain-specific errors. It has zero external dependencies.
- **Ports Layer (`internal/ports`)**: Defines input and output interfaces for storage, persistence, cache, messaging, and dead-letter queue management.
- **Application Layer (`internal/application`)**: Implements use cases for upload orchestration, file lifecycle management, download link generation, and distributed quota accounting.
- **Adapters Layer (`internal/adapters`)**: Concrete implementations of ports:
  - AWS S3 for object storage and presigned multipart URLs.
  - AWS DynamoDB for entity persistence, transactions, and single-table / multi-table querying.
  - Redis for distributed locks, idempotency keys, and rate limiting.
  - Apache Kafka for event-driven asynchronous processing queues.
  - NATS Core for low-latency pub/sub progress streaming.
- **Transport Layer (`internal/transport`)**:
  - GraphQL Federation Subgraph using `graphql-go` for client and gateway operations.
  - gRPC server for low-latency inter-service resolution.
  - WebSocket Hub for bidirectional streaming of upload progress and processing stages.
- **Transactional Outbox Pattern**: Ensures dual-write safety between DynamoDB state updates and Kafka event publishing.

---

## 2. Directory Structure

```
services/media-service/
├── cmd/
│   └── server/
│       └── main.go                 # Service entrypoint, dependency wiring, and graceful shutdown
├── internal/
│   ├── adapters/
│   │   ├── dynamodb/               # DynamoDB client and repository implementations
│   │   ├── kafka/                  # Kafka producer, consumer groups, and envelope marshaling
│   │   ├── nats/                   # NATS realtime event publisher
│   │   ├── redis/                  # Redis caching, distributed locking, and rate limiting
│   │   └── s3/                     # AWS S3 client, multipart upload handling, and presigning
│   ├── application/
│   │   ├── download/               # Get download URL use case
│   │   ├── media/                  # Media query and deletion use cases
│   │   └── upload/                 # Upload session creation, completion, abortion, and renewal
│   ├── config/                     # Configuration loading, validation, and defaults
│   ├── domain/                     # Entities, domain errors, state machine, and retry policies
│   ├── i18n/                       # Localization messages (Arabic and English)
│   ├── observability/              # Prometheus metrics and instrumentation
│   ├── ports/                      # Port interfaces for repositories, storage, and messaging
│   ├── scheduler/                  # Cron scheduler for periodic background jobs
│   ├── telemetry/                  # OpenTelemetry tracing integration
│   ├── testutil/                   # In-memory doubles and mocks for tests
│   ├── transport/
│   │   ├── graphql/                # GraphQL schema, queries, mutations, and DLQ resolvers
│   │   ├── grpc/                   # gRPC service implementation and protobuf definitions
│   │   └── ws/                     # WebSocket hub, connection lifecycles, and pump loops
│   ├── validation/                 # Magic bytes detection, MIME verification, and SHA-256 checks
│   └── workers/                    # Pipeline workers (scan, image, video, metadata, outbox, etc.)
├── test/
│   └── e2e/                        # End-to-end integration test suite against live infrastructure
├── Dockerfile                      # Multi-stage production container build
├── Makefile                        # Automation targets for build, test, and vet
└── go.mod                          # Go module dependencies
```

---

## 3. System Components and Communication Protocols

The service exposes four network ports and runs background consumers:

1. **GraphQL Federation Subgraph (Default Port: `:4005`)**: Exposed through the API Gateway. Implements Apollo Federation specification (`_service { sdl }`) for federated schema stitching.
2. **WebSocket Realtime Hub (Default Port: `:4003`)**: Direct client connection using token-based authentication for real-time progress events.
3. **Internal gRPC Server (Default Port: `:50052`)**: Fast, binary RPC used by peer microservices (such as Delivery Service and User Service) to resolve media download URLs without gateway overhead.
4. **Prometheus Metrics Server (Default Port: `:9102`)**: Scraped by Prometheus for monitoring upload rates, durations, failure counters, and queue lags.
5. **Kafka Consumer Groups**: 6 isolated consumer groups processing media pipelines concurrently.

---

## 4. Media Processing Pipeline and Lifecycle

Media items follow a deterministic state machine:

```
[ PENDING ] ──► [ UPLOADING ] ──► [ UPLOADED ] ──► [ SCANNING ] ──► [ PROCESSING ] ──► [ READY ]
     │                │                                │                    │             │
     ▼                ▼                                ▼                    ▼             ▼
[ ABORTED ]      [ ABORTED ]                     [ QUARANTINED ]         [ FAILED ]   [ DELETING ]
                                                       │                                  │
                                                       ▼                                  ▼
                                                  [ DELETING ]                       [ DELETED ]
```

### Upload Workflow:
1. **Session Creation**: Client requests an upload session with file metadata (name, size, MIME type, optional SHA-256). The service verifies user storage quota, concurrent upload limits, rate limits, reserves bytes, initiates an S3 Multipart Upload, and returns presigned PUT URLs for each chunk.
2. **Direct Chunk Upload**: The client uploads parts directly to S3 without passing bytes through the API gateway or backend servers.
3. **Session Completion**: Client submits completion request with the list of part numbers and ETags. The service:
   - Acquires a distributed lock in Redis for the media ID.
   - Signals S3 to assemble the multipart upload.
   - Reads the initial bytes directly from S3 to verify the magic bytes against declared MIME type.
   - Persists a `TopicUploadCompleted` event in the Outbox table.
   - Atomically updates media status from `UPLOADING` to `UPLOADED`.
   - Decrements active upload counters and updates total stored bytes.

### Background Processing Workflow:
1. **Outbox Publisher**: Poller picks pending outbox entries and publishes `media.upload.completed` events to Kafka.
2. **Scanner Worker (`media-scanner`)**: Inspects file via ClamAV daemon. If infected or signature mismatched, sets state to `QUARANTINED`. If clean, publishes `media.scan.completed`.
3. **Metadata Worker (`media-metadata-worker`)**: Reads stream headers to extract dimensions, duration, bitrate, and codec details. Saves a `metadata` version record and publishes `media.processing.completed`.
4. **Image Worker (`media-image-worker`)**: For image media types, generates standardized thumbnails and optimized WebP variants, uploads them to S3, and records `thumbnail` / `webp` versions in DynamoDB.
5. **Video Worker (`media-video-worker`)**: For video media types, runs FFmpeg transcoding pipelines to produce standardized MP4 and HLS streams.
6. **Realtime Notifications**: Emits intermediate progress and final `media_ready` notifications via NATS, which the WebSocket Hub delivers to connected clients.

---

## 5. Data Models and Storage

The service persists entities using AWS DynamoDB:

- **Media Records**: Stores file identity, ownership (`ownerId`), context (`deliveryId`), object key, content type, media category, byte size, status, and creation timestamps. Supported with Global Secondary Indexes (GSIs) for querying by owner and status.
- **Upload Sessions**: Tracks active multipart sessions, S3 upload IDs, part sizes, total parts, completed part lists, and session expiration timestamps.
- **Media Versions**: Stores derived asset records (e.g., thumbnail, compressed, webp, transcoded resolutions) referencing the parent `mediaId`.
- **User Quotas**: Tracks cumulative byte consumption and concurrent upload session counts per user with atomic counters.
- **Transactional Outbox**: Maintains pending events with aggregate ID, topic, trace ID, creation timestamp, and TTL to prevent dual-write loss.

---

## 6. Security and Validation Mechanisms

- **Deep Magic Bytes Inspection**: Validates file content using raw signature headers rather than relying on client-supplied extensions. Covers JPEG, PNG, GIF, WebP, AVIF, MP4 (ftyp boxes), QuickTime, WebM, PDF, and ZIP.
- **Strict Ownership Checks**: Every mutation and query validates the authenticated user ID against resource ownership (`ErrUnauthorized`), unless initiated by an administrator.
- **Quarantine Enforcement**: Any media item flagged as `QUARANTINED` cannot be read or resolved for download URLs.
- **Distributed Idempotency**: All mutation endpoints accept idempotency keys stored in Redis with 24-hour TTL to prevent double charges, duplicate sessions, or duplicate multipart completions.
- **Rate Limiting**: Sliding window rate limiters in Redis protect upload creation, renewal, and download resolution endpoints against abuse.

---

## 7. Configuration and Environment Variables

Configuration is loaded from environment variables:

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT_MEDIA_GRAPHQL` | `4005` | GraphQL server HTTP port |
| `PORT_MEDIA_WS` | `4003` | WebSocket server port |
| `PORT_MEDIA_GRPC` | `50052` | Internal gRPC server port |
| `PORT_MEDIA_METRICS` | `9102` | Prometheus metrics scrape port |
| `NODE_ENV` | `development` | Deployment environment (`development`, `staging`, `production`) |
| `AWS_REGION` | `us-east-1` | AWS region |
| `AWS_ENDPOINT` | *empty* | Custom endpoint (e.g., `http://localstack-srv:4566` for local testing) |
| `DYNAMODB_ENDPOINT` | *empty* | Custom DynamoDB endpoint for local testing |
| `DYNAMODB_TABLE_NAME` | `media-service` | DynamoDB table name |
| `S3_BUCKET_NAME` | `delivery-micro` | AWS S3 bucket name |
| `S3_PUBLIC_ENDPOINT` | *empty* | Externally reachable S3 URL for presigned URLs in local development |
| `S3_PRESIGNED_URL_EXPIRY`| `3600` | Expiration time for presigned URLs in seconds |
| `S3_MULTIPART_MIN_PART_SIZE` | `5242880` | Minimum size per upload part (default 5 MB) |
| `REDIS_HOST` | `redis-srv` | Redis server hostname |
| `REDIS_PORT` | `6379` | Redis server port |
| `NATS_URL` | `nats://nats-srv:4222` | NATS connection string |
| `KAFKA_BROKERS` | `kafka-srv:9092` | Comma-separated list of Kafka broker addresses |
| `KAFKA_GROUP_ID` | `media-service` | Base consumer group ID prefix |
| `MEDIA_MAX_FILE_SIZE_BYTES` | `53687091200` | Maximum file size allowed (default 50 GB) |
| `MEDIA_MAX_CONCURRENT_UPLOADS` | `5` | Maximum concurrent active uploads per user |
| `MEDIA_STORAGE_QUOTA_BYTES` | `53687091200` | Default storage quota per user (50 GB) |
| `CLAMAV_HOST` | `clamav-srv` | Hostname of the ClamAV daemon |
| `CLAMAV_PORT` | `3310` | Port of the ClamAV daemon |
| `FFMPEG_BINARY` | `ffmpeg` | Path to the FFmpeg executable |

---

## 8. API Reference

### GraphQL Federation Subgraph (`POST /media/graphql`)

Headers required:
- `x-user-id`: User ID passed by the API Gateway.
- `x-user-role`: User role (`user`, `driver`, `admin`).
- `x-correlation-id`: Trace and telemetry context.

#### Mutations
- `createUploadSession(input: CreateUploadInput!)`: Initializes a multipart upload and returns chunk upload URLs.
- `completeUpload(input: CompleteUploadInput!)`: Finalizes upload, verifies signatures, and queues processing.
- `abortUpload(input: AbortUploadInput!)`: Cancels upload, releases allocated quota, and aborts S3 session.
- `renewPresignedUrls(input: RenewPresignedUrlsInput!)`: Generates fresh URLs for pending chunks if expired.
- `deleteMedia(mediaId: String!)`: Flags item as `DELETING` and queues background asset cleanup.
- `replayDLQ(topic: String!, maxMessages: Int!)`: Re-queues failed dead-letter messages (Admin only).

#### Queries
- `media(mediaId: String!)`: Fetches media details, technical metadata, and available versions.
- `myMedia(limit: Int, cursor: String)`: Paginated list of media items owned by the authenticated caller.
- `uploadStatus(uploadId: String!)`: Current status of an in-flight upload session.
- `downloadUrl(mediaId: String!, versionType: String, expirySeconds: Int)`: Generates presigned GET URL.
- `quota`: Retrieves storage quota limits and current usage for caller.
- `dlqStats(topics: [String!])`: Message counts in dead-letter queues (Admin only).

### gRPC Interface (`:50052`)

Defined in `protos/media.proto` and implemented in `internal/transport/grpc`:

```protobuf
service MediaService {
  rpc ResolveMediaUrl(ResolveMediaUrlRequest) returns (ResolveMediaUrlResponse);
  rpc GetMedia(GetMediaRequest) returns (GetMediaResponse);
}
```

- Handled with interceptors for correlation ID logging and status error mapping (maps `ErrMediaNotFound` to `codes.NotFound`, `ErrUnauthorized` to `codes.PermissionDenied`, and `ErrMediaQuarantined` to `codes.FailedPrecondition`).

### WebSocket Interface (`/ws`)

Connect query parameter: `?token=<JWT_TOKEN>`

Incoming client messages:
- `ping`: Keepalive heartbeat.
- `subscribe`: Subscribes connection to a specific media ID topic.

Outgoing server messages:
- `upload_progress`: Broadcasts parts uploaded, total parts, bytes transferred, speed MB/s, and percent.
- `processing_progress`: Broadcasts processing stage (`scanning`, `metadata`, `image`, `video`) and completion percent.
- `media_ready`: Emitted when all variants are processed and media status transitions to `READY`.
- `media_failed`: Emitted if scanning or processing encounters a permanent error.

---

## 9. Background Workers and Scheduling

- **Worker Pools (`internal/workers/pool.go`)**: Each processing domain uses a dedicated worker pool with configurable concurrency limits, semaphore slot acquisition, and panic recovery.
- **Reconciliation Worker (`internal/workers/reconciliation`)**: Periodically finds abandoned or expired multipart sessions, cancels them in S3, and frees user quota.
- **Dead Letter Queue (DLQ)**: Every Kafka consumer uses exponential backoff retries. After exhausting retry attempts, messages are directed to `{topic}.dlq` to prevent consumer head-of-line blocking.
- **Cron Scheduler (`internal/scheduler`)**: Lightweight wrapper around `robfig/cron/v3` orchestrating cleanup routines at configurable intervals.

---

## 10. Testing Strategy and Quality Assurance

The codebase maintains full test suites covering both isolated units and end-to-end integration:

### Unit Testing
- Covers all business logic across `domain`, `validation`, `application`, `workers`, `transport`, `config`, and `adapters`.
- Mocks and in-memory test doubles reside in `internal/testutil/`.
- Executed via:
  ```bash
  go test ./... -v
  ```

### End-to-End (E2E) Testing
- Resides in `test/e2e/`.
- Executes real multipart upload workflows, verifies S3 storage, executes Kafka pipelines, streams NATS progress events, and asserts DynamoDB records.
- Guarded by the `MEDIA_E2E=1` environment variable so unit test runs in standard CI pipelines remain fast and isolated.
- Executed via:
  ```bash
  make test-e2e
  ```

---

## 11. Local Development and Build Commands

### Prerequisites
- Go 1.22+
- Docker and Docker Compose
- LocalStack (for local S3 and DynamoDB emulation)
- ClamAV daemon (optional, falls back gracefully in development)

### Environment Setup
Create a local `.env` or set the required environment variables:
```bash
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
S3_ENDPOINT=http://localhost:4566
DDB_ENDPOINT=http://localhost:4566
REDIS_HOST=localhost
REDIS_PORT=6379
KAFKA_BROKERS=localhost:9092
NATS_URL=nats://localhost:4222
```

### Build and Run Targets
Use the provided `Makefile` targets to build and test:

```bash
# Display help and all available targets
make help

# Download and tidy Go dependencies
make tidy

# Format source code
make fmt

# Run static code analysis and syntax verification
make vet

# Execute all unit tests
make test

# Execute live E2E integration test suite (requires live stack)
make test-e2e

# Execute E2E suite and produce code coverage report
make test-e2e-coverage

# Generate unit test coverage report
make coverage

# Compile the standalone production binary
make build

# Run the service locally
make run

# Clean build artifacts and coverage files
make clean

# Build the Docker container image
make docker-build
```

