# Media Service --- System Architecture

## 1. Document Purpose

This document defines the production-grade system architecture for the
`Media Service` of the Delivery Platform.

The service is responsible for:

-   Uploading small and very large files.
-   Direct client-to-S3 uploads.
-   Resumable multipart uploads.
-   Upload validation and integrity verification.
-   Media metadata management.
-   Image processing.
-   Video transcoding.
-   Compression and optimization.
-   Malware scanning.
-   Secure downloads.
-   Media deletion.
-   Retries and failure recovery.
-   Idempotency.
-   Rate limiting and quota management.
-   Asynchronous processing.
-   Durable event publishing.
-   Reconciliation of stuck operations.
-   Observability.
-   Horizontal scaling.

The Media Service is implemented in **Go**.

------------------------------------------------------------------------

# 2. Architectural Goals

The architecture is designed around the following goals:

1.  The Media Service must never become a bottleneck for large file
    data.
2.  Files must be uploaded directly from the client to object storage.
3.  Uploads must support files from very small sizes up to
    multi-gigabyte files.
4.  Failed multipart parts must be retryable independently.
5.  Uploads must be resumable.
6.  Media state must have a reliable source of truth.
7.  Background processing must not block API requests.
8.  Processing must be horizontally scalable.
9.  Events must survive temporary Kafka outages.
10. Operations must be idempotent.
11. Temporary failures must be automatically retried.
12. Permanent failures must be observable and recoverable.
13. Users must receive realtime processing notifications.
14. Download traffic must bypass the application servers.
15. Security validation must happen both before and after upload.
16. The system must tolerate crashes, duplicate requests, worker
    failures, and partial uploads.

------------------------------------------------------------------------

# 3. High-Level Architecture

``` text
                                    CLIENT
                                      |
                                      |
                               GraphQL + JWT
                                      |
                                      v
                           +----------------------+
                           |   GraphQL Gateway    |
                           |        NestJS        |
                           +----------+-----------+
                                      |
                                     gRPC
                                      |
                                      v
                           +----------------------+
                           |    Media Service     |
                           |         Go           |
                           +----------+-----------+
                                      |
             +------------------------+------------------------+
             |                        |                        |
             v                        v                        v
       +-----------+            +-----------+            +-----------+
       | PostgreSQL|            |   Redis   |            |    S3     |
       | Metadata  |            | Ephemeral |            |  Objects  |
       +-----------+            +-----------+            +-----------+
             |                                               ^
             |                                               |
             |                                      Direct Multipart
             |                                           Upload
             |                                               |
             +-------------------- CLIENT -------------------+
                                      |
                                      |
                              Complete Upload
                                      |
                                      v
                               Media Service
                                      |
                                      v
                                  Outbox
                                      |
                                      v
                                   Kafka
                                      |
                    +-----------------+-----------------+
                    |                 |                 |
                    v                 v                 v
               Scanner          Compression       Media Processor
                    |                 |                 |
                    +-----------------+-----------------+
                                      |
                                      v
                                     S3
                                      |
                                      v
                                  Kafka Event
                                      |
                                      v
                             Realtime Service
                                  NestJS
                                      |
                                     NATS
                                      |
                                 WebSocket
                                      |
                                      v
                                    CLIENT
```

------------------------------------------------------------------------

# 4. Core Technology Responsibilities

  -----------------------------------------------------------------------
  Technology                          Responsibility
  ----------------------------------- -----------------------------------
  Go                                  Media business logic, API, workers,
                                      processing orchestration

  gRPC                                Internal communication between
                                      GraphQL Gateway and Media Service

  GraphQL Gateway                     Client-facing API, authentication,
                                      request orchestration

  S3                                  Actual file/object storage

  PostgreSQL                          Durable media metadata, state,
                                      jobs, and outbox

  Redis                               Temporary state, idempotency,
                                      locks, rate limiting, progress

  Kafka                               Durable asynchronous domain events

  NATS                                Lightweight realtime event delivery

  WebSocket                           Client realtime notifications

  FFmpeg                              Video transcoding and media
                                      processing

  ClamAV                              Malware scanning

  Kubernetes                          Deployment, scaling, and recovery

  CDN                                 High-scale file/video delivery

  OpenTelemetry                       Distributed tracing

  Prometheus-compatible metrics       Metrics collection
  -----------------------------------------------------------------------

------------------------------------------------------------------------

# 5. Why Files Do Not Pass Through the Media Service

The Media Service must not proxy large file bytes.

Bad architecture:

``` text
Client
  |
  | 10 GB File
  v
Media Service
  |
  v
S3
```

This creates:

-   Application bandwidth consumption.
-   High memory/network pressure.
-   More expensive infrastructure.
-   Longer request lifetimes.
-   More failure points.
-   Poor horizontal scalability.

Recommended architecture:

``` text
Client
  |
  | Presigned Multipart Upload
  v
S3
```

The Media Service only controls:

``` text
Authorization
Validation
Upload Session
Presigned URLs
Upload State
Completion
Processing
Metadata
Security
Events
```

------------------------------------------------------------------------

# 6. Main Media Lifecycle

The media state machine is:

``` text
PENDING
   |
   v
UPLOADING
   |
   v
UPLOADED
   |
   v
SCANNING
   |
   v
PROCESSING
   |
   v
READY
```

Failure paths:

``` text
PENDING
   |
   v
FAILED

UPLOADING
   |
   v
ABORTED

SCANNING
   |
   v
QUARANTINED

PROCESSING
   |
   v
FAILED

READY
   |
   v
DELETING
   |
   v
DELETED
```

Recommended states:

``` text
PENDING
UPLOADING
UPLOADED
SCANNING
PROCESSING
READY
FAILED
QUARANTINED
DELETING
DELETED
ABORTED
```

State transitions must be validated so that workers cannot arbitrarily
overwrite each other's state.

------------------------------------------------------------------------

# 7. Upload Flow

## Step 1 --- Client Requests an Upload Session

The client sends:

``` text
createUploadSession(
    fileName,
    contentType,
    size,
    checksum?
)
```

The GraphQL Gateway:

1.  Authenticates the user.
2.  Validates the request.
3.  Applies API rate limits.
4.  Extracts the authenticated user ID.
5.  Calls the Media Service through gRPC.

------------------------------------------------------------------------

# 8. Upload Validation

The Media Service performs server-side validation.

Validation includes:

-   File size.
-   Content type.
-   File extension.
-   Allowed media type.
-   User permissions.
-   Delivery ownership.
-   Storage quota.
-   Concurrent upload limits.
-   Filename normalization.
-   Object key generation.
-   Optional client-provided checksum.

The client validation is only a UX optimization.

The backend remains authoritative.

------------------------------------------------------------------------

# 9. Upload Session Creation

The Media Service creates:

``` text
mediaId
uploadId
objectKey
status
expiration
```

Example object key:

``` text
users/{userId}/deliveries/{deliveryId}/media/{mediaId}/original/{fileName}
```

The object key should not be controlled directly by the client.

------------------------------------------------------------------------

# 10. Multipart Upload

For large files:

``` text
Client
   |
   +---- Part 1 ----+
   +---- Part 2 ----+
   +---- Part 3 ----+----> S3
   +---- Part 4 ----+
   +---- Part N ----+
```

The Media Service:

1.  Creates an S3 multipart upload.
2.  Determines the number of parts.
3.  Generates presigned URLs.
4.  Returns the upload information to the client.

The client uploads each part directly to S3.

------------------------------------------------------------------------

# 11. Multipart Retry

If one part fails:

``` text
Part 1  OK
Part 2  OK
Part 3  FAILED
Part 4  OK
```

Only Part 3 is retried.

The entire upload is not restarted.

Recommended client retry strategy:

``` text
Attempt 1
   |
   +-- failure
   |
Attempt 2
   |
   +-- failure
   |
Attempt 3
   |
   +-- failure
   |
Abort or report failure
```

Use exponential backoff with jitter.

------------------------------------------------------------------------

# 12. Resumable Upload

The client can resume an interrupted upload.

The Media Service can track:

``` text
uploadId
s3UploadId
totalParts
completedParts
status
expiresAt
```

The client can ask:

``` text
getUploadStatus(uploadId)
```

The service checks S3 for the current multipart state and returns
missing parts.

------------------------------------------------------------------------

# 13. Complete Upload

After all parts are uploaded:

``` text
Client
   |
   | completeUpload(uploadId)
   v
GraphQL Gateway
   |
   | gRPC
   v
Media Service
   |
   v
S3 CompleteMultipartUpload
```

The Media Service then verifies:

-   Upload session exists.
-   User owns the session.
-   Upload has not expired.
-   Correct state.
-   Required parts exist.
-   Expected size matches.
-   Optional checksum matches.

------------------------------------------------------------------------

# 14. Post-Upload Verification

Do not trust only:

``` text
Content-Type: video/mp4
```

The service can validate the actual object using:

-   S3 metadata.
-   Object size.
-   Magic bytes.
-   File signature.
-   MIME detection.
-   Checksum.
-   Media metadata extraction.

Example:

``` text
Extension: .mp4
Client MIME: video/mp4
Actual signature: valid MP4
Size: valid
Checksum: valid
```

Only then should the media proceed to processing.

------------------------------------------------------------------------

# 15. Source of Truth

For this architecture:

``` text
PostgreSQL = Source of Truth
Redis       = Temporary/Fast State
S3          = File Storage
Kafka       = Event Log / Event Transport
```

Redis must never be the only place containing critical media state.

If Redis goes down:

``` text
Redis DOWN
   |
   v
Media Service
   |
   v
PostgreSQL
```

The system can recover.

------------------------------------------------------------------------

# 16. PostgreSQL Data Model

## media

``` text
id
owner_id
delivery_id
file_name
content_type
media_type
size
checksum
object_key
status
created_at
updated_at
```

## upload_sessions

``` text
id
media_id
user_id
s3_upload_id
total_parts
completed_parts
status
expires_at
created_at
updated_at
```

## media_versions

``` text
id
media_id
version_type
object_key
content_type
size
checksum
width
height
duration
created_at
```

Examples:

``` text
original
thumbnail
medium
optimized
360p
720p
1080p
```

## media_jobs

``` text
id
media_id
job_type
status
attempts
max_attempts
last_error
started_at
completed_at
created_at
updated_at
```

Job types:

``` text
SCAN
COMPRESS
THUMBNAIL
TRANSCODE
METADATA_EXTRACTION
DELETE
```

## outbox_events

``` text
id
aggregate_id
event_type
payload
status
attempts
created_at
published_at
```

------------------------------------------------------------------------

# 17. Redis Responsibilities

Redis is used for fast, temporary operations.

## Upload Progress

``` text
media:upload:{uploadId}:progress
```

## Idempotency

``` text
idempotency:{userId}:{key}
```

## Distributed Locks

``` text
lock:media:{mediaId}
```

## Rate Limiting

``` text
rate-limit:user:{userId}
rate-limit:ip:{ip}
```

## Temporary Processing Progress

``` text
processing:{mediaId}:progress
```

Redis keys must have appropriate TTLs.

------------------------------------------------------------------------

# 18. Idempotency

Every externally triggered operation that can be retried should be
idempotent.

Examples:

``` text
createUploadSession
completeUpload
abortUpload
deleteMedia
processMedia
```

Example:

``` text
Idempotency-Key: 7f2c...
```

If the same request arrives twice:

``` text
Request 1
   |
   v
Create Media

Request 2
   |
   v
Return existing result
```

No duplicate media should be created.

------------------------------------------------------------------------

# 19. Concurrency Control

Multiple workers may process the same media.

Example:

``` text
Worker A
   |
   v
PROCESSING

Worker B
   |
   v
PROCESSING
```

Use:

-   Database conditional updates.
-   Optimistic locking.
-   Redis distributed locks when appropriate.
-   Idempotent workers.

The database remains the final authority.

------------------------------------------------------------------------

# 20. Quota Management

The system must protect storage and concurrent operations.

Examples:

``` text
User Storage Quota
Daily Upload Quota
Maximum File Size
Maximum Concurrent Uploads
Maximum Processing Jobs
```

Before creating an upload:

``` text
Current Usage
     +
Requested File Size
     <=
User Quota
```

For concurrent uploads:

``` text
active_uploads < max_concurrent_uploads
```

The quota operation must be concurrency-safe.

------------------------------------------------------------------------

# 21. Rate Limiting

Rate limiting is applied at multiple levels.

``` text
IP
User
API
Operation
```

Examples:

``` text
createUploadSession
completeUpload
deleteMedia
download URL generation
```

Redis can implement distributed rate limiting.

------------------------------------------------------------------------

# 22. Transactional Outbox

The Media Service should not do:

``` text
UPDATE media
   |
   v
Kafka.publish()
```

because the database update could succeed while Kafka publishing fails.

Instead:

``` text
Database Transaction
        |
        +---- Update Media
        |
        +---- Insert Outbox Event
        |
       COMMIT
```

Then:

``` text
Outbox Publisher
       |
       v
Kafka
```

This prevents losing important events.

------------------------------------------------------------------------

# 23. Outbox Failure Recovery

If Kafka is unavailable:

``` text
PostgreSQL
    |
    v
Outbox Event
    |
    X Kafka unavailable
```

The event remains persisted.

Later:

``` text
Outbox Publisher
      |
      v
Retry
      |
      v
Kafka
```

The publisher must be idempotent.

------------------------------------------------------------------------

# 24. Kafka Events

Recommended events:

``` text
media.upload.created
media.upload.completed
media.scan.started
media.scan.completed
media.scan.failed
media.processing.started
media.processing.completed
media.processing.failed
media.ready
media.delete.requested
media.deleted
media.delete.failed
```

Events should contain:

``` text
eventId
eventType
aggregateId
mediaId
userId
timestamp
version
traceId
payload
```

------------------------------------------------------------------------

# 25. Kafka Consumer Groups

Use independent consumer groups for independent responsibilities.

Example:

``` text
media-scanner
media-processor
media-notification
media-analytics
```

This allows each subsystem to scale independently.

------------------------------------------------------------------------

# 26. Dead Letter Queue

If a job repeatedly fails:

``` text
Kafka
  |
  v
Consumer
  |
  +-- retry 1
  +-- retry 2
  +-- retry 3
  |
  v
DLQ
```

The DLQ allows:

-   Investigation.
-   Manual replay.
-   Alerting.
-   Root-cause analysis.

------------------------------------------------------------------------

# 27. Worker Architecture

Workers should use a controlled worker pool.

``` text
Kafka
  |
  v
Consumer
  |
  v
Job Queue
  |
  +---- Worker 1
  +---- Worker 2
  +---- Worker 3
  +---- Worker N
```

Do not create unlimited goroutines for expensive processing.

Concurrency must be controlled because video processing can be
CPU-intensive.

------------------------------------------------------------------------

# 28. Malware Scanning

After upload:

``` text
UPLOADED
    |
    v
SCANNING
    |
    +---- clean ------> PROCESSING
    |
    +---- infected ---> QUARANTINED
```

A quarantined object must not be publicly downloadable.

------------------------------------------------------------------------

# 29. Image Processing

For images:

``` text
original.jpg
      |
      +---- thumbnail
      +---- medium
      +---- optimized
      +---- WebP/AVIF if required
```

Store generated versions in S3.

Metadata can include:

``` text
width
height
format
size
checksum
```

------------------------------------------------------------------------

# 30. Video Processing

For videos, use FFmpeg-based workers.

Example:

``` text
original.mp4
     |
     +---- 360p
     +---- 720p
     +---- 1080p
     +---- thumbnail
     +---- preview
```

For streaming workloads, the service can generate HLS assets:

``` text
master.m3u8
360p/
720p/
1080p/
```

Video processing should be asynchronous.

Never perform heavy transcoding inside a synchronous gRPC request.

------------------------------------------------------------------------

# 31. Compression

Compression should be content-aware.

Good candidates:

``` text
TXT
JSON
CSV
LOG
Some documents
```

Already-compressed formats generally should not be blindly recompressed:

``` text
JPEG
PNG
WebP
MP4
MKV
ZIP
RAR
7Z
```

For video, transcoding/encoding is usually more useful than generic
compression.

------------------------------------------------------------------------

# 32. Download Flow

The Media Service should not stream large downloads through itself.

Flow:

``` text
Client
  |
  | getDownloadUrl(mediaId)
  v
GraphQL Gateway
  |
  | gRPC
  v
Media Service
  |
  | Authorization
  |
  | Generate Presigned GET URL
  v
Client
  |
  v
S3 / CDN
```

This keeps download traffic away from application servers.

------------------------------------------------------------------------

# 33. CDN

For frequently accessed media:

``` text
Client
   |
   v
CDN
   |
   +-- cache hit ---> Client
   |
   +-- cache miss
          |
          v
         S3
```

The Media Service remains responsible for authorization and URL
generation.

------------------------------------------------------------------------

# 34. Delete Flow

Deletion should be asynchronous for large or multi-version media.

``` text
Client
   |
   | deleteMedia
   v
Media Service
   |
   v
Mark DELETING
   |
   v
Outbox
   |
   v
Kafka
   |
   v
Delete Worker
   |
   +---- original
   +---- thumbnail
   +---- optimized
   +---- video variants
   |
   v
S3
   |
   v
Mark DELETED
```

------------------------------------------------------------------------

# 35. Delete Retry

If S3 deletion fails:

``` text
Delete Worker
      |
      X
      |
      v
Retry with backoff
```

After the maximum number of attempts:

``` text
FAILED
   |
   v
DLQ / Reconciliation
```

Delete operations must be idempotent.

------------------------------------------------------------------------

# 36. Reconciliation Worker

The system needs a periodic reconciliation process.

Examples:

``` text
UPLOADING for too long
PROCESSING for too long
DELETING for too long
Outbox events not published
Jobs stuck in RUNNING
Expired upload sessions
```

The reconciliation worker can:

``` text
Retry
Resume
Abort
Repair state
Publish missing events
Mark permanently failed
```

Do not rely exclusively on Redis polling.

------------------------------------------------------------------------

# 37. S3 Lifecycle Policies

S3 should automatically clean storage that the application does not need
to manage synchronously.

Examples:

``` text
Incomplete multipart uploads
Temporary processing objects
Old temporary versions
```

Example policy:

``` text
Abort incomplete multipart uploads after N days.
Delete temporary objects after N days.
```

S3 Lifecycle is a second line of defense.

------------------------------------------------------------------------

# 38. Scheduled Jobs

The scheduler can run:

``` text
Reconciliation
Expired upload cleanup
Stuck job detection
Outbox recovery
Orphan media detection
Temporary state cleanup
```

The scheduler should be safe to run on multiple replicas.

Use distributed locking or a scheduler designed for clustered
deployments.

------------------------------------------------------------------------

# 39. Observability

Every request/job should carry:

``` text
requestId
traceId
userId
mediaId
uploadId
jobId
eventId
```

------------------------------------------------------------------------

# 40. Structured Logging

Example:

``` json
{
  "level": "INFO",
  "service": "media-service",
  "operation": "complete_upload",
  "mediaId": "media_123",
  "uploadId": "upload_123",
  "userId": "user_123",
  "traceId": "trace_123",
  "duration_ms": 142,
  "status": "success"
}
```

Never log:

-   Presigned URLs.
-   Authentication tokens.
-   Secrets.
-   Sensitive file contents.

------------------------------------------------------------------------

# 41. Metrics

Important metrics include:

## Upload Metrics

``` text
upload_started_total
upload_completed_total
upload_failed_total
upload_bytes_total
upload_duration_seconds
active_uploads
```

## Processing Metrics

``` text
processing_started_total
processing_completed_total
processing_failed_total
processing_duration_seconds
active_processing_jobs
```

## Reliability Metrics

``` text
retry_total
dead_letter_total
reconciliation_total
idempotency_hits_total
```

## Infrastructure Metrics

``` text
s3_errors_total
redis_errors_total
database_errors_total
kafka_consumer_lag
kafka_publish_failures
```

## Realtime Metrics

``` text
websocket_connections
notification_failures
nats_publish_failures
```

------------------------------------------------------------------------

# 42. Distributed Tracing

Trace propagation should work across:

``` text
GraphQL Gateway
      |
     gRPC
      |
Media Service
      |
     S3
      |
    Kafka
      |
   Worker
      |
     S3
      |
Realtime Service
      |
     NATS
      |
 WebSocket
```

Use OpenTelemetry-compatible tracing.

For asynchronous messaging, propagate trace context through Kafka event
headers.

------------------------------------------------------------------------

# 43. Security Architecture

Security must exist at multiple layers.

## Authentication

Handled at the Gateway.

## Authorization

Enforced again inside the Media Service.

Never trust the Gateway blindly for ownership-sensitive operations.

The Media Service should verify:

``` text
user owns media
user owns delivery
user has required permission
```

------------------------------------------------------------------------

# 44. S3 Security

Use:

``` text
Private Bucket
Block Public Access
Server-side Encryption
Short-lived Presigned URLs
Least-privilege IAM
```

The client should never receive AWS credentials.

------------------------------------------------------------------------

# 45. Presigned URL Security

Presigned URLs should:

-   Have short expiration times.
-   Be generated only after authorization.
-   Be scoped to the required object.
-   Be generated with the required operation only.

Example:

``` text
PUT presigned URL
GET presigned URL
```

Do not reuse a PUT URL for downloading.

------------------------------------------------------------------------

# 46. Object Key Security

Never allow the client to decide arbitrary S3 paths.

Bad:

``` text
objectKey = client.filePath
```

Good:

``` text
users/{userId}/deliveries/{deliveryId}/media/{mediaId}/...
```

The server owns object-key generation.

------------------------------------------------------------------------

# 47. Failure Scenarios

## Client crashes during upload

``` text
Client crashes
     |
     v
Upload remains resumable
     |
     v
Client reconnects
     |
     v
Resume upload
```

## Media Service crashes

``` text
Media Service DOWN
     |
     v
S3 upload continues
```

The client can continue uploading because the file data does not pass
through the service.

## Worker crashes

Kafka does not acknowledge the message.

The event can be redelivered.

## Kafka crashes

Outbox events remain persisted.

## Redis crashes

Critical state remains in PostgreSQL.

## S3 temporarily fails

Retry using exponential backoff.

## Processing permanently fails

Move the job to DLQ and mark the media appropriately.

------------------------------------------------------------------------

# 48. Retry Strategy

Use exponential backoff with jitter.

Conceptually:

``` text
Retry 1 → short delay
Retry 2 → larger delay
Retry 3 → larger delay
Retry N → DLQ
```

Do not retry permanent errors such as:

``` text
Unauthorized
Invalid media
Unsupported format
Quota exceeded
```

Retry transient errors such as:

``` text
S3 timeout
Network failure
Kafka unavailable
Temporary dependency failure
```

------------------------------------------------------------------------

# 49. API Boundaries

The client-facing API belongs to the GraphQL Gateway.

Example:

``` text
createUploadSession
getUploadStatus
completeUpload
abortUpload
resumeUpload
getMedia
listMedia
getDownloadUrl
deleteMedia
```

The Media Service exposes internal gRPC methods corresponding to these
operations.

------------------------------------------------------------------------

# 50. gRPC Responsibility

Example service:

``` proto
service MediaService {
    rpc CreateUploadSession(CreateUploadSessionRequest)
        returns (CreateUploadSessionResponse);

    rpc CompleteUpload(CompleteUploadRequest)
        returns (CompleteUploadResponse);

    rpc AbortUpload(AbortUploadRequest)
        returns (AbortUploadResponse);

    rpc GetUploadStatus(GetUploadStatusRequest)
        returns (GetUploadStatusResponse);

    rpc GetMedia(GetMediaRequest)
        returns (GetMediaResponse);

    rpc GetDownloadUrl(GetDownloadUrlRequest)
        returns (GetDownloadUrlResponse);

    rpc DeleteMedia(DeleteMediaRequest)
        returns (DeleteMediaResponse);
}
```

The gRPC API should carry metadata and control operations, not large
file bytes.

------------------------------------------------------------------------

# 51. Realtime Architecture

The Media Service should not own WebSocket connections.

Recommended architecture:

``` text
Media Service
     |
     v
Kafka
     |
     v
Realtime Service
     |
     v
NATS
     |
     v
WebSocket
     |
     v
Client
```

The Realtime Service owns:

-   WebSocket connections.
-   Client subscriptions.
-   Connection lifecycle.
-   Notification fan-out.

------------------------------------------------------------------------

# 52. Upload Progress vs Processing Progress

Do not send every upload percentage through Kafka.

During direct S3 upload:

``` text
Client
   |
   +---- Part 1
   +---- Part 2
   +---- Part 3
   |
   v
S3
```

The client can calculate upload progress locally.

The backend should publish meaningful lifecycle events:

``` text
UPLOAD_COMPLETED
SCAN_COMPLETED
PROCESSING_STARTED
PROCESSING_COMPLETED
READY
FAILED
DELETED
```

Processing progress can be published through Redis/NATS when required.

------------------------------------------------------------------------

# 53. Horizontal Scaling

The Media API is stateless.

``` text
                 Load Balancer
                      |
        +-------------+-------------+
        |             |             |
        v             v             v
   Media API 1   Media API 2   Media API N
```

State is externalized to:

``` text
PostgreSQL
Redis
S3
Kafka
```

Workers can scale independently:

``` text
Scanner Workers
       x N

Image Workers
       x N

Video Workers
       x N
```

------------------------------------------------------------------------

# 54. Kubernetes Deployment

Recommended workloads:

``` text
media-api
media-scanner-worker
media-image-worker
media-video-worker
media-compression-worker
media-delete-worker
media-reconciliation-worker
media-outbox-worker
```

Heavy workers should scale independently from the API.

------------------------------------------------------------------------

# 55. Autoscaling

API scaling can depend on:

``` text
CPU
Memory
Request rate
Latency
```

Worker scaling can depend on:

``` text
Kafka consumer lag
Queue depth
CPU
Processing latency
```

Video workers should have their own scaling policy because they are
CPU-intensive.

------------------------------------------------------------------------

# 56. Media Service File Structure

``` text
media-service/
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── domain/
│   │   ├── media.go
│   │   ├── media_version.go
│   │   ├── upload.go
│   │   ├── processing.go
│   │   ├── status.go
│   │   └── events.go
│   │
│   ├── application/
│   │   ├── upload/
│   │   │   ├── create.go
│   │   │   ├── complete.go
│   │   │   ├── cancel.go
│   │   │   └── resume.go
│   │   │
│   │   ├── download/
│   │   │   └── download.go
│   │   │
│   │   ├── delete/
│   │   │   └── delete.go
│   │   │
│   │   └── processing/
│   │       └── processing.go
│   │
│   ├── transport/
│   │   └── grpc/
│   │       ├── server.go
│   │       ├── upload_handler.go
│   │       ├── media_handler.go
│   │       └── download_handler.go
│   │
│   ├── storage/
│   │   └── s3/
│   │       ├── client.go
│   │       ├── multipart.go
│   │       ├── presigned.go
│   │       ├── objects.go
│   │       └── checksum.go
│   │
│   ├── database/
│   │   └── postgres/
│   │       ├── client.go
│   │       ├── media.repository.go
│   │       ├── upload.repository.go
│   │       ├── job.repository.go
│   │       └── outbox.repository.go
│   │
│   ├── cache/
│   │   └── redis/
│   │       ├── client.go
│   │       ├── progress.go
│   │       ├── lock.go
│   │       ├── rate_limit.go
│   │       └── idempotency.go
│   │
│   ├── messaging/
│   │   └── kafka/
│   │       ├── producer.go
│   │       ├── consumer.go
│   │       ├── topics.go
│   │       └── events.go
│   │
│   ├── workers/
│   │   ├── pool.go
│   │   ├── outbox/
│   │   ├── scan/
│   │   ├── compression/
│   │   ├── image/
│   │   ├── video/
│   │   ├── delete/
│   │   └── reconciliation/
│   │
│   ├── processing/
│   │   ├── compression/
│   │   ├── image/
│   │   └── video/
│   │
│   ├── validation/
│   │   ├── file_type.go
│   │   ├── mime.go
│   │   ├── size.go
│   │   ├── checksum.go
│   │   └── magic_bytes.go
│   │
│   ├── security/
│   │   ├── authorization.go
│   │   └── antivirus.go
│   │
│   ├── scheduler/
│   │   ├── cron.go
│   │   └── jobs.go
│   │
│   └── observability/
│       ├── logger.go
│       ├── metrics.go
│       └── tracing.go
│
├── proto/
│   └── media.proto
│
├── migrations/
│
├── deployments/
│   ├── docker/
│   └── kubernetes/
│
├── configs/
│   ├── config.yaml
│   └── config.example.yaml
│
├── scripts/
│
├── tests/
│   ├── integration/
│   ├── e2e/
│   └── fixtures/
│
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

------------------------------------------------------------------------

# 57. Complete Upload Sequence

``` text
Client
  |
  | GraphQL createUploadSession
  v
Gateway
  |
  | gRPC
  v
Media Service
  |
  +--> Authentication context
  +--> Authorization
  +--> Quota
  +--> Rate Limit
  +--> Validation
  +--> Idempotency
  |
  +--> PostgreSQL
  |       |
  |       +--> Create Media
  |       +--> Create Upload Session
  |
  +--> S3
  |       |
  |       +--> Create Multipart Upload
  |
  +--> Presigned URLs
          |
          v
        Client
          |
          +---- Part 1 ----+
          +---- Part 2 ----+
          +---- Part N ----+----> S3
          |
          v
      Complete Upload
          |
          v
      Media Service
          |
          +--> Verify S3 object
          +--> Verify checksum
          +--> Update state
          +--> Create Outbox Event
          |
          v
         Kafka
          |
          +--> Scanner
          +--> Processor
          +--> Notification
```

------------------------------------------------------------------------

# 58. Complete Processing Sequence

``` text
Kafka
  |
  | media.upload.completed
  v
Scanner
  |
  +---- infected ----> QUARANTINED
  |
  +---- clean
          |
          v
      Processor
          |
          +---- Image
          |      |
          |      +--> Thumbnail
          |      +--> Optimization
          |
          +---- Video
          |      |
          |      +--> FFmpeg
          |      +--> 360p
          |      +--> 720p
          |      +--> 1080p
          |
          +---- Document
                 |
                 +--> Preview/Optimization

          |
          v
         S3
          |
          v
      Update Metadata
          |
          v
      media.ready
          |
          v
        Kafka
          |
          v
    Realtime Service
          |
          v
       WebSocket
          |
          v
        Client
```

------------------------------------------------------------------------

# 59. Complete Delete Sequence

``` text
Client
  |
  | deleteMedia
  v
Gateway
  |
  | gRPC
  v
Media Service
  |
  +--> Authorization
  +--> Idempotency
  +--> Mark DELETING
  +--> Outbox Event
  |
  v
Kafka
  |
  v
Delete Worker
  |
  +--> Delete original
  +--> Delete thumbnails
  +--> Delete optimized versions
  +--> Delete video variants
  |
  v
S3
  |
  v
Mark DELETED
  |
  v
Kafka
  |
  v
Realtime Service
  |
  v
Client
```

------------------------------------------------------------------------

# 60. Reliability Principles

The Media Service follows these principles:

### Never trust the client

Validate everything server-side.

### Never send large file bytes through the application

Use direct S3 upload/download.

### Never use Redis as the source of truth

Keep durable state in PostgreSQL.

### Never depend on a single synchronous request for processing

Use asynchronous workers.

### Never publish an event without reliable persistence

Use an Outbox.

### Never assume a worker runs only once

Make processing idempotent.

### Never retry permanent errors

Retry only transient failures.

### Never let one worker type consume unlimited resources

Use controlled worker pools.

### Never expose private S3 objects publicly

Use IAM and presigned URLs/CDN.

------------------------------------------------------------------------

# 61. Final Architecture

``` text
                                  CLIENT
                                    |
                              GraphQL + JWT
                                    |
                                    v
                           +------------------+
                           | GraphQL Gateway  |
                           |     NestJS       |
                           +--------+---------+
                                    |
                                   gRPC
                                    |
                                    v
                         +----------------------+
                         |    Media Service     |
                         |         Go           |
                         +----------+-----------+
                                    |
             +----------------------+----------------------+
             |                      |                      |
             v                      v                      v
       +-----------+          +-----------+          +-----------+
       | PostgreSQL|          |   Redis   |          |    S3     |
       |   State   |          |  Ephemeral|          |  Storage  |
       +-----+-----+          +-----------+          +-----+-----+
             |                                          ^
             |                                          |
             |                                   Direct Upload
             |                                          |
             +---------------- CLIENT ------------------+
                                    |
                              Complete Upload
                                    |
                                    v
                                Outbox
                                    |
                                    v
                                  Kafka
                                    |
                +-------------------+-------------------+
                |                   |                   |
                v                   v                   v
             Scanner           Compression          Processor
                |                   |                   |
                +-------------------+-------------------+
                                    |
                                    v
                                   S3
                                    |
                                    v
                                  Kafka
                                    |
                                    v
                           Realtime Service
                                NestJS
                                    |
                                   NATS
                                    |
                               WebSocket
                                    |
                                    v
                                  CLIENT
```

------------------------------------------------------------------------

# 62. Final Technology Map

``` text
CLIENT
  |
  +-- GraphQL
  +-- Multipart Upload
  +-- Upload Progress
  +-- WebSocket
  |
  v
GRAPHQL GATEWAY
  |
  +-- Authentication
  +-- Request Validation
  +-- Rate Limiting
  +-- gRPC
  |
  v
MEDIA SERVICE — GO
  |
  +-- Upload Management
  +-- Validation
  +-- Authorization
  +-- Idempotency
  +-- Quotas
  +-- Presigned URLs
  +-- Media Lifecycle
  +-- Processing Orchestration
  |
  +------------------+
  |                  |
  v                  v
PostgreSQL          Redis
  |                  |
  +-- Metadata       +-- Progress
  +-- Jobs           +-- Locks
  +-- Outbox         +-- Idempotency
                     +-- Rate Limits
  |
  v
Kafka
  |
  +-- Scanner
  +-- Compression
  +-- Image Processing
  +-- Video Processing
  +-- Delete Workers
  +-- Notifications
  |
  v
S3
  |
  +-- Original Files
  +-- Processed Files
  +-- Thumbnails
  +-- Video Variants
  |
  v
CDN
  |
  v
CLIENT

Realtime:

Kafka
  |
  v
Realtime Service
  |
  v
NATS
  |
  v
WebSocket
  |
  v
CLIENT
```

------------------------------------------------------------------------

# 63. Architectural Decision Summary

  Decision             Choice
  -------------------- --------------------------------------
  Main language        Go
  Client API           GraphQL through Gateway
  Internal API         gRPC
  File storage         S3
  Database             PostgreSQL
  Cache                Redis
  Durable events       Kafka
  Realtime messaging   NATS
  Client realtime      WebSocket
  Upload strategy      Direct-to-S3 Multipart
  Large file support   Resumable Multipart Upload
  Download strategy    Presigned GET + CDN
  Processing           Async Workers
  Video processing     FFmpeg
  Malware scanning     ClamAV
  Reliability          Retry + Backoff + DLQ
  Event reliability    Transactional Outbox
  Concurrency          Conditional Updates + Locks
  Cleanup              Reconciliation + S3 Lifecycle
  Rate limiting        Redis
  Idempotency          Redis + persistent state
  Observability        Logs + Metrics + OpenTelemetry
  Deployment           Docker + Kubernetes
  Scaling              Horizontal API + independent workers

------------------------------------------------------------------------

# 64. Recommended Implementation Order

Build the system in this order:

``` text
Phase 1
Go Service
    ↓
gRPC
    ↓
PostgreSQL
    ↓
Redis
```

``` text
Phase 2
Upload Session
    ↓
S3 Multipart
    ↓
Presigned URLs
    ↓
Complete Upload
    ↓
Resume / Abort
```

``` text
Phase 3
Validation
    ↓
Checksum
    ↓
Idempotency
    ↓
Quota
    ↓
Rate Limiting
```

``` text
Phase 4
Outbox
    ↓
Kafka
    ↓
Consumer Groups
    ↓
Retry
    ↓
DLQ
```

``` text
Phase 5
Scanner
    ↓
Compression
    ↓
Image Processing
    ↓
Video Processing
```

``` text
Phase 6
Download
    ↓
Presigned GET
    ↓
CDN
    ↓
Delete
```

``` text
Phase 7
Reconciliation
    ↓
S3 Lifecycle
    ↓
Observability
    ↓
Metrics
    ↓
Tracing
```

``` text
Phase 8
Realtime Service
    ↓
Kafka Consumer
    ↓
NATS
    ↓
WebSocket
    ↓
Client Notifications
```

This ordering keeps the Media Service independently functional before
realtime capabilities are added.



1. Authenticate
        ↓
2. Authorize
        ↓
3. Validate Request
        ↓
4. Check Quota
        ↓
5. Check Rate Limit
        ↓
6. Create Upload Session
        ↓
7. Generate Presigned URLs
        ↓
8. Client uploads directly to S3
        ↓
9. Multipart / Resume / Retry
        ↓
10. Complete Upload
        ↓
11. Verify S3 object
        ↓
12. Checksum / Magic Bytes
        ↓
13. Virus Scan
        ↓
14. Metadata Extraction
        ↓
15. Processing
        ↓
16. Store processed versions
        ↓
17. Mark MEDIA READY
        ↓
18. Publish event
        ↓
19. Notify through Realtime Service
        ↓
20. CDN delivery


8. State Machine

دي ممتازة، لكن أنا أعدلها قليلًا.

INITIATED
    ↓
UPLOADING
    ↓
UPLOADED
    ↓
SCANNING
    ↓
PROCESSING
    ↓
READY

وممكن branches:

       ┌──────────────→ FAILED
       |
       ↓
SCANNING
       |
       ├──────────────→ QUARANTINED
       |
       ↓
PROCESSING
       |
       └──────────────→ READY

وللحذف:

READY
  ↓
DELETING
  ↓
DELETED

والـ transitions نفسها لازم تكون guarded.



8. State Machine

دي ممتازة، لكن أنا أعدلها قليلًا.

INITIATED
    ↓
UPLOADING
    ↓
UPLOADED
    ↓
SCANNING
    ↓
PROCESSING
    ↓
READY

وممكن branches:

       ┌──────────────→ FAILED
       |
       ↓
SCANNING
       |
       ├──────────────→ QUARANTINED
       |
       ↓
PROCESSING
       |
       └──────────────→ READY

وللحذف:

READY
  ↓
DELETING
  ↓
DELETED

والـ transitions نفسها لازم تكون guarded.

PHASE 1 — Foundation
├── Go service
├── gRPC
├── PostgreSQL
├── Redis
├── S3
├── Docker
└── Configuration

PHASE 2 — Basic Upload
├── Authentication
├── Authorization
├── Validation
├── Quota
├── Create Upload Session
├── Presigned URLs
└── Single PUT Upload

PHASE 3 — Large Files
├── Multipart Upload
├── Parallel Parts
├── Retry Parts
├── Resume Upload
├── Pause
└── Cancel

PHASE 4 — Reliability
├── Idempotency
├── Checksum
├── Conditional State Transitions
├── Distributed Locks
├── Reconciliation
└── S3 Lifecycle

PHASE 5 — Event Driven
├── Transactional Outbox
├── Kafka
├── Events
├── Consumers
├── Retry
└── DLQ

PHASE 6 — Media Processing
├── Virus Scan
├── Metadata Extraction
├── Image Processing
├── Compression
├── Thumbnails
└── Video Processing / FFmpeg

PHASE 7 — Realtime
├── Realtime Service
├── NATS
├── WebSocket
├── Upload Progress
└── Processing Progress

PHASE 8 — Delivery
├── Download Authorization
├── Presigned GET
├── CDN
├── Range Requests
└── Cache Headers

PHASE 9 — Observability
├── Structured Logging
├── Metrics
├── Distributed Tracing
├── Alerts
└── Kafka Lag Monitoring

PHASE 10 — Advanced
├── Versioning
├── Deduplication
├── Advanced Quota
├── Storage Optimization
├── Advanced Reconciliation
└── Multi-region considerations