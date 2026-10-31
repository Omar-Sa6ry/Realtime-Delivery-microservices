# Unified Media Upload Flow — Flow Only

Scope of this document: the end-to-end flow of the media subsystem (upload,
scan, process, deliver, delete) as implemented by `services/media-service` and
its infrastructure, plus the parts exercised by the React test app in this
folder (`media-frontend/`). The React app implements the frontend boxes of this
diagram (register/login, multipart upload with resume, WebSocket status,
download with Range, delete). Sections marked as target architecture (CDN
delivery layer, Kubernetes runtime, OTLP tracing from the browser) describe the
intended production design and are not fully wired in the test app yet.

```text
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                           USER                                                 │
│                    [100 GB Video]  [5 KB Video]  [Image]                                     │
└──────────────────────────────────────────────┬───────────────────────────────────────────────┘
                                               │
                                               │ 1. Select file(s)
                                               ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                   FRONTEND / REACT                                              │
│                                                                                              │
│  File Picker → Upload Queue → Preview                                                        │
│  Client Validation: MIME whitelist + per-kind size cap (image 10MB / video 50GB / doc 500MB)  │
│  SHA-256 Checksum (crypto.subtle) → sent with createUploadSession                              │
│  Bearer JWT → GraphQL operations (Apollo Client)                                               │
│                                                                                              │
│  Branch A: 100 GB Video → Multipart Upload (many parts)                                       │
│  Branch B: 5 KB Video  → one-part upload (server returns a single part)                       │
│  Branch C: Image       → one-part or multipart according to server part-size policy           │
│                                                                                              │
│  Resume: IndexedDB / uploadId / completedParts / ETags (identifier for resource/file)        │
│  Parallel parts: bounded concurrency 4 (worker pool over presigned parts)                     │
│  Failed part: up to 5 attempts + exponential backoff (1s base) + jitter                       │
│  Offline: pause new parts → reconnect → resume missing parts                                  │
└───────────────────────────────┬──────────────────────────────┬───────────────────────────────┘
                                │                              │
                                │ 2. GraphQL: createUploadSession                          │
                                ▼                              │
┌──────────────────────────────────────────────────────────────┐│
│                         API GATEWAY                           ││
│                                                              ││
│  JWT Authentication: signature / issuer / audience / expiry  ││
│  Authorization guard: ACL / role / permission                 ││
│  Correlation ID / requestId / traceId                         ││
│  Routing: GraphQL → Media Service                             ││
│  Rate limit at edge when configured                            ││
└───────────────────────────────┬──────────────────────────────┘│
                                │ 3. trusted user identity + request metadata
                                ▼                              │
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                  MEDIA SERVICE / GO                                            │
│                                                                                              │
│  Authentication context: requireUser                                                        │
│  Authorization: owner / ACL / role                                                          │
│  MIME whitelist + extension validation + size validation                                    │
│  Checksum metadata validation                                                               │
│  Redis rate limit: user/IP operations                                                       │
│  DynamoDB quota: used bytes + concurrent uploads                                            │
│  Conditional quota reservation: race protection                                             │
│  Idempotency: create / complete / delete / process                                           │
│                                                                                              │
│  Create: mediaId + uploadId + s3UploadId                                                    │
│  Metadata: owner / fileName / contentType / size / checksum / objectKey / timestamps        │
│  State: PENDING → UPLOADING                                                                  │
│                                                                                              │
│  Large file: calculate partSize + totalParts                                                │
│  Small file: single object / one-part path                                                  │
└───────────────┬──────────────────────────────┬───────────────────────────┬─────────────────┘
                │                              │                           │
                │ 4. durable metadata         │ 5. coordination            │ 6. storage setup
                ▼                              ▼                           ▼
┌───────────────────────────┐  ┌───────────────────────────┐  ┌─────────────────────────────┐
│        DYNAMODB            │  │           REDIS             │  │            S3               │
│                            │  │                             │  │                             │
│ Media metadata             │  │ Rate limiting               │  │ Bytes storage               │
│ Upload Session             │  │ Distributed lock            │  │ Original object             │
│ Ownership / ACL reference  │  │ Idempotency window          │  │ Multipart upload            │
│ State / version / TTL      │  │ Temporary progress/state   │  │ Temporary objects           │
│ Quota / jobs / outbox      │  │ Concurrent coordination     │  │ Presigned PUT/GET URLs      │
│ Conditional writes         │  │                             │  │ S3 Lifecycle cleanup        │
└───────────────────────────┘  └───────────────────────────┘  └──────────────┬──────────────┘
                                                                                │
                                                                                │ 7. return short-lived
                                                                                │    presigned URLs
                                                                                ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                   FRONTEND / REACT                                              │
│                                                                                              │
│  100 GB Video:                                                                            │
│    File.slice → P1, P2, P3 ... Pn                                                        │
│    4 bounded concurrent PUT requests (XMLHttpRequest for upload progress)                 │
│    each successful part → ETag                                                            │
│    failed part → retry (up to 5 attempts) + exponential backoff + jitter                  │
│    pause/resume/offline recovery → IndexedDB                                              │
│                                                                                              │
│  5 KB Video:                                                                            │
│    one short PUT / one part → ETag                                                       │
│                                                                                              │
│  Image:                                                                                  │
│    one PUT or multipart → ETag                                                           │
│                                                                                              │
│  IMPORTANT: Frontend never receives AWS credentials.                                      │
└─────────────────────────────────────────────┬────────────────────────────────────────────┘
                                              │ 8. direct upload — bytes do not pass through Gateway
                                              ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                      S3 STORE                                                   │
│                                                                                              │
│  Multipart state: initiated → uploading → completed                                         │
│  Stores parts and returns ETags                                                              │
│  Handles transient failure through client retry                                              │
│  Permanent failure: invalid URL / policy / object mismatch → stop and report                 │
│  Incomplete multipart uploads → S3 Lifecycle cleanup                                          │
└─────────────────────────────────────────────┬────────────────────────────────────────────────┘
                                              │ 9. Frontend sends uploadId + ordered parts + ETags
                                              ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                  MEDIA SERVICE / GO                                            │
│                              CompleteUploadUseCase                                             │
│                                                                                              │
│  Verify uploadId + owner + expiry + allowed state                                           │
│  Acquire Redis distributed lock                                                             │
│  Verify part numbers and ETags                                                              │
│  S3 CompleteMultipartUpload / finalize single object                                       │
│  S3 HeadObject: compare actual size with declared size                                     │
│  Post-upload validation: magic bytes + optional SHA-256 checksum                            │
│                                                                                              │
│  Invalid size/checksum/magic bytes → FAILED                                                 │
│  Valid object → state UPLOADED                                                              │
│  Create outbox event before/with state transition                                            │
│  Event: media.upload.completed                                                             │
└─────────────────────────────────────────────┬────────────────────────────────────────────────┘
                                              │ 10. durable event
                                              ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                  KAFKA EVENT BUS                                               │
│                                                                                              │
│  Topics: media.upload.completed / media.scan.completed / media.ready / media.delete.requested│
│  Partitioning: mediaId as key to preserve per-media ordering                                 │
│  Consumer groups: scanner / image / video / metadata / delete                               │
│  Event envelope: eventId / eventType / traceId / occurredAt / schemaVersion / payload        │
│  Retry: transient failures only                                                             │
│  Permanent failures after max retries → DLQ                                                 │
│  DLQ logic: preserve payload + error + attempt + traceId → investigate → safe replay         │
└───────────────────────┬──────────────────────┬──────────────────────┬───────────────────────┘
                        │                      │                      │
                        │ 11A. consume        │ 11B. consume          │ 11C. consume
                        ▼                      ▼                      ▼
┌─────────────────────────────┐ ┌─────────────────────────────┐ ┌─────────────────────────────┐
│        SCAN WORKER           │ │        VIDEO WORKER          │ │        IMAGE WORKER          │
│                             │ │                             │ │                             │
│ All uploaded file types     │ │ 100 GB + 5 KB videos        │ │ Images                      │
│ Read object from S3         │ │ Read object from S3         │ │ Read object from S3         │
│ Isolated scratch directory  │ │ Isolated scratch directory  │ │ Isolated scratch directory  │
│ subprocess timeout          │ │ subprocess timeout          │ │ subprocess timeout          │
│ No path traversal            │ │ No path traversal            │ │ No path traversal            │
│ No unsafe shell interpolation││ No unsafe shell interpolation││ No unsafe shell interpolation│
└───────────────┬─────────────┘ └───────────────┬─────────────┘ └───────────────┬─────────────┘
                │                               │                               │
                │ 12. malware scan              │                               │
                ▼                               │                               │
┌───────────────────────────────┐              │                               │
│             ClamAV             │              │                               │
│                               │              │                               │
│ Clean → continue processing   │              │                               │
│ Infected → QUARANTINED        │              │                               │
│ EICAR → integration test only │              │                               │
│ ClamAV unreachable → fail-open│              │                               │
│   treated clean + logged      │              │                               │
└───────────────┬───────────────┘              │                               │
                │                               │                               │
        ┌───────┴────────┐                      │                               │
        │                │                      │                               │
        │ clean          │ infected             │                               │
        ▼                ▼                      │                               │
┌───────────────┐ ┌────────────────┐            │                               │
│ PROCESSING     │ │ QUARANTINED   │            │                               │
│ continue       │ │ stop pipeline │            │                               │
└──────┬────────┘ │ no download   │            │                               │
       │          │ URL           │            │                               │
       └──────────┴────────────────────────────┘                               │
                                                                                │
                                                                                │ 13. video processing
                                                                                ▼
                                                       ┌───────────────────────────────┐
                                                       │           FFmpeg               │
                                                       │                               │
                                                       │ codec detection/validation    │
                                                       │ dimensions / resolution       │
                                                       │ duration extraction            │
                                                       │ FFmpeg-backed renditions       │
                                                       │ 360p / 720p / 1080p            │
                                                       │ HLS playlist + segments        │
                                                       │ subprocess timeout             │
                                                       │ output validation + cleanup    │
                                                       └───────────────┬───────────────┘
                                                                       │
                                                                       │ video renditions
                                                                       ▼
                                                        ┌───────────────────────────────┐
                                                        │             S3                │
                                                        │ original + renditions/HLS     │
                                                        │  movie_1080p.mp4              │
                                                        │  movie_720p.mp4               │
                                                        │  movie_480p.mp4               │
                                                        │  movie_360p.mp4 + HLS         │
                                                        └───────────────┬───────────────┘
                                                                        │
                                                                        │ 14. image processing (parallel branch)
                                                                        ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                 IMAGE PROCESSING                                               │
│                                                                                              │
│  ImageMagick / FFmpeg (1080p, 720p, 480p, 360p)                                              │
│  dimensions limits                                                                           │
│  thumbnail generation                                                                         │
│  FFmpeg-backed rendition generation                                                          │
│  WebP output                                                                                  │
│  AVIF output                                                                                  │
│  correct MIME for each rendition                                                             │
│  codec/output validation                                                                     │
│  isolated scratch directory + cleanup                                                        │
└──────────────────────────────────────────────┬───────────────────────────────────────────────┘
                                               │ 15. metadata extraction for all valid media
                                               ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                  METADATA WORKER                                                │
│                                                                                              │
│  Image: width / height / dimensions                                                          │
│  Video: duration / codec / resolution / frame information                                    │
│  Metadata is derived from actual bytes, not trusted from Frontend                            │
│  Save metadata and rendition versions in DynamoDB                                            │
└──────────────────────────────────────────────┬───────────────────────────────────────────────┘
                                               │ 16. processing result
                                               ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                  MEDIA SERVICE / GO                                            │
│                                                                                              │
│  Update DynamoDB conditionally: PROCESSING → READY                                           │
│  Save original/rendition object keys, versions, metadata                                     │
│  Publish media.ready event                                                                   │
│  Processing failure: FAILED or retry                                                        │
│  Permanent processing failure: DLQ                                                         │
└───────────────────────────────┬──────────────────────────────┬───────────────────────────────┘
                                │                              │
                                │ 17. durable media.ready      │ 18. fast realtime event
                                ▼                              ▼
┌───────────────────────────────┐              ┌───────────────────────────────────────────────┐
│            KAFKA                │              │                    NATS                       │
│ media.ready / delete events    │              │ low-latency realtime fan-out                 │
│ replayable business event      │              │ not the durable source of truth              │
└───────────────┬───────────────┘              └───────────────────────────────┬───────────────┘
                │                                                               │
                │ 19. notification event                                       │ 20. realtime event
                ▼                                                               ▼
┌───────────────────────────────────────────────┐   ┌───────────────────────────────────────────┐
│            NOTIFICATION SERVICE                │   │             REALTIME SERVICE              │
│                                               │   │                                           │
│ Notification record                           │   │ JWT WebSocket authentication              │
│ NotificationDelivery record                   │   │ user/channel authorization                │
│ delivery state: pending/processing/completed  │   │ WebSocket connection management           │
│ realtime worker                                │   │ NATS fan-out                               │
│ outbox worker                                  │   │ reconnect support                          │
│ retry/expire delivery                         │   │ bounded payload / WS_MAX_PAYLOAD           │
│ BullMQ + TypeORM                              │   │                                           │
└───────────────────────────────┬───────────────┘   └──────────────────────┬────────────────────┘
                                │                                           │
                                └──────────────────────┬────────────────────┘
                                                       │ 21. WebSocket update
                                                       ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                   FRONTEND / REACT                                              │
│                                                                                              │
│  Receives: SCANNING / PROCESSING / PROCESSING_PROGRESS / READY / FAILED                    │
│  Clean file: show READY ✓                                                                   │
│  Failed processing: show FAILED                                                             │
│  Quarantined: show blocked/quarantined and hide download                                    │
│  WebSocket disconnect: reconnect with backoff + refetch status from API                     │
└──────────────────────────────────────────────────────────────────────────────────────────────┘


DELETE FLOW

Frontend
  |
  | deleteMedia(mediaId, idempotencyKey)
  v
API Gateway
  | JWT authentication + correlation ID
  v
Media Service
  | authorization: owner / ACL / role
  | conditional DynamoDB transition: READY → DELETING
  | Redis idempotency + lock
  | outbox: media.delete.requested
  v
Kafka
  |
  v
Delete Worker
  | delete original object
  | delete thumbnails / WebP / AVIF / renditions / HLS
  | retry transient S3 failures
  | permanent failure → DLQ
  v
S3 DeleteObject
  v
DynamoDB
  | conditional state: DELETING → DELETED
  v
Kafka / NATS
  v
Notification Service + Realtime Service
  | delivery state + WebSocket fan-out
  v
Frontend displays DELETE_COMPLETED or DELETE_FAILED


DOWNLOAD FLOW

Frontend
  |
  | request downloadUrl(mediaId, versionType)
  v
API Gateway
  | JWT authentication
  v
Media Service
  | owner / ACL / role authorization
  | reject FAILED / QUARANTINED / DELETING / DELETED
  | issue short-lived presigned GET URL or configured CDN URL
  v
S3 / CDN
  | direct download
  | large file: HTTP Range bytes=start-
  | response: 206 Partial Content + Content-Range
  v
Frontend
  | append bytes for resume; never overwrite completed bytes


RECONCILIATION AND CLEANUP FLOW

Scheduler / Reconciliation Worker
  |
  | find stale UPLOADING / PROCESSING / DELETING records
  v
Media Service
  | check DynamoDB state + S3 existence + Kafka/outbox status
  | retry safe transient operations
  | repair missing event or mark FAILED
  v
DynamoDB + S3 + Kafka/DLQ

S3 Lifecycle
  | remove incomplete multipart uploads and temporary objects after policy duration

DynamoDB TTL
  | asynchronously remove expired temporary sessions/idempotency records


OBSERVABILITY FLOW

Frontend requestId / traceId
  |
  v
API Gateway span
  |
  v
Media Service span
  |
  +--> Redis span
  +--> DynamoDB span
  +--> S3 presign/complete span
  +--> Kafka producer span
              |
              v
       Worker consumer span
              |
              +--> ClamAV span
              +--> FFmpeg/ImageMagick span
              +--> S3 rendition span
              +--> DynamoDB update span
              +--> NATS/WebSocket span
              v
       OTLP tracing exporter → OpenTelemetry Collector / Jaeger


KUBERNETES / RUNTIME SUPPORT

ConfigMaps
  | non-secret configuration: ports / topic names / feature flags / limits
  v
Deployments and Services
  | frontend / gateway / media / realtime / notification / workers
  v
NetworkPolicies
  | least-privilege pod-to-pod communication
  v
Secrets
  | JWT signing keys / AWS credentials / database passwords
  v
HPA
  | scale replicas using CPU / memory / queue or Kafka lag metrics
  v
PDB
  | keep minimum replicas during voluntary disruption


SECURITY GUARDRAILS INSIDE THE FLOW

No AWS credentials in Frontend
No trust in extension or browser MIME alone
Magic bytes + checksum after upload
ClamAV scan before READY (fail-open fallback when ClamAV is unreachable)
EICAR only as harmless integration test
No download URL for QUARANTINED
No path traversal or symlink escape in scratch directory
No unsafe shell interpolation
Subprocess timeout for FFmpeg and ClamAV
Limits for image dimensions and video duration/resolution
Protection against zip bombs/decompression bombs
Conditional writes for state transitions
Idempotency for create/complete/delete/process
Short-lived presigned URLs
Never log JWT, secrets, or complete presigned URLs
```


```text
CDN FLOW — DELIVERY LAYER

MEDIA SERVICE / GO
  |
  | verify owner / ACL / role + state READY + version
  | generate signed CDN URL or signed cookie
  | include mediaId / rendition / expiry / cache policy
  v
CDN / EDGE DISTRIBUTION
  |-- origin: S3 bucket or private origin endpoint
  |-- cache original and renditions according to policy
  |-- cache key: mediaId + version + rendition + format
  |-- private origin: CDN uses Origin Access Control / private access
  |-- HTTPS/TLS + WAF/rate protection when configured
  |-- cache hit -> serve from edge
  |-- cache miss -> fetch from S3 -> store at edge -> return to client
  |-- invalid/expired signature -> 401/403
  |-- origin unavailable -> 5xx / retry or controlled fallback
  v
FRONTEND / REACT
  |-- receives signed CDN URL, never permanent S3 credentials
  |-- video: play original/rendition/HLS through CDN
  |-- image: load thumbnail/WebP/AVIF through CDN
  |-- large download: send Range: bytes=start-
  |-- CDN returns 206 Partial Content + Content-Range
  |-- resume download by requesting the missing byte range

CDN CACHE POLICY
  |
  |-- READY public/authorized rendition -> cacheable with versioned URL
  |-- QUARANTINED / FAILED / DELETING / DELETED -> never serve
  |-- private user media -> signed URL/cookie + short authorization lifetime
  |-- immutable rendition version -> long cache-control max-age
  |-- new rendition/version -> new cache key, no stale old content
  |-- deleted object -> CDN invalidation/purge + S3 deletion
  v
S3 ORIGIN
  |-- private bucket; direct public bucket access disabled
  |-- original bytes / renditions / HLS segments
  |-- CDN Origin Access Control
  |-- S3 Lifecycle cleans incomplete multipart/temp objects

CDN FAILURE AND FALLBACK

Frontend
  |
  | CDN 5xx / timeout / invalid cache response
  v
Media Service
  |-- validate media state and permission again
  |-- issue fresh CDN URL
  |-- optionally issue short-lived S3 presigned GET fallback
  |-- log requestId + traceId + mediaId + CDN status
  v
CDN retry or S3 fallback
  |
  v
Frontend displays download/playback or controlled failure

CDN OBSERVABILITY

Frontend requestId / traceId
  -> API Gateway span
  -> Media Service download-url span
  -> CDN request ID / cache status: HIT or MISS
  -> Origin S3 span
  -> OTLP tracing exporter -> OpenTelemetry Collector / Jaeger

CDN PLACEMENT SUMMARY

UPLOAD:
  Frontend -> API Gateway -> Media Service -> presigned S3 PUT
  CDN is not in the upload byte path unless an upload-enabled edge is explicitly configured.

PROCESSING:
  S3 origin -> Scan/Video/Image/Metadata Workers -> S3 renditions

DOWNLOAD:
  Frontend -> API Gateway -> Media Service authorization
  -> signed CDN URL -> CDN edge -> private S3 origin

DELETE:
  Media Service -> S3 DeleteObject + CDN invalidation/purge
  -> Kafka/NATS -> Notification/Realtime -> Frontend
```
