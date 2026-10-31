# Media Frontend (Media Service Test Lab)

A React + TypeScript test application for the media subsystem of the Realtime
Delivery microservices project. It is not the production storefront/driver app;
it is a lab harness whose purpose is to exercise, visualize and debug the full
media lifecycle against the real backend:

- register / login through the API Gateway (user-service subgraph)
- avatar upload that is linked to the profile over gRPC (user-service asks
  media-service to verify ownership and resolve the URL)
- generic media upload with multipart presigned URLs, resume after
  interruption, offline recovery, pause / cancel
- live processing status over the media-service WebSocket (scan, processing,
  ready, delete)
- download with HTTP Range resume, copy of the short-lived download URL,
  media deletion
- an optional dashboard that can trigger the media-service e2e test runner and
  stream results live (the runner itself is a separate backend, see below)

The app talks only to the API Gateway (GraphQL over HTTP, JWT bearer tokens)
and to short-lived presigned URLs / the media WebSocket. It never receives AWS
credentials. Parts of the codebase (for example video transcoding UI, CDN
delivery) exercise backend capabilities that exist in the media-service design;
the visual reference for the whole system is in [FULL_FLOW.md](./FULL_FLOW.md)
and its infographic [image.png](./image.png).

## Tech stack

| Layer | Choice |
|---|---|
| UI | React 18 |
| Language | TypeScript 5 (strict, `tsc -b`) |
| Build tool | Vite 5 |
| GraphQL client | Apollo Client 3 (imperative `mutate` / `query`, no hooks-based queries) |
| Realtime | native `WebSocket` (single shared `MediaSocket` singleton) |
| Resume storage | IndexedDB (`MediaUploadDB` database, `sessions` object store) |
| Upload transport | `XMLHttpRequest` for parts (only API that reports upload progress), `fetch` for GraphQL and downloads |

## End-to-end flow implemented by this app

```text
Register / Login (public)
  └─► register() / login() ─► AuthResponse { accessToken, refreshToken, user }
        tokens stored in localStorage; every GraphQL request carries
        Authorization: Bearer <accessToken> (authLink in api/client.ts)
        │
        ├─► [optional avatar at register time]
        │     createUploadSession()            media subgraph (JWT) ─► x-user-id
        │     uploadFileParts()                DIRECT PUT to S3 (presigned URLs)
        │     completeUpload()                 media subgraph
        │     updateProfile(avatarMediaId)     user-service resolves the URL over
        │                                      gRPC ─► media-service (ownership check)
        │     myProfile() ─► user.imageUrl
        │
        └─► [Media Lab Pro panel — any file type]
              createUploadSession ─► presigned parts ─► parallel PUT to S3
              ─► completeUpload ─► Kafka outbox ─► scan / video / image / metadata
                 workers ─► media.ready ─► NATS ─► media WebSocket ─► UI status
              downloadUrl ─► Range download (206) ─► deleteMedia ─► DELETED event
```

The backend owns all authentication and authorization:

- The API Gateway verifies the JWT (`JWT_SECRET`) and forwards the trusted user
  identity (`x-user-id`) to the subgraphs.
- media-service checks ownership on every upload / complete / download /
  delete call.
- user-service links an avatar only when media-service (over gRPC) confirms the
  caller owns that media item.

## Features of each screen

The app has a single page with four panels after login:

1. Profile (left column) — shows the current user, role and `imageUrl`, plus an
   avatar picker that uploads an image and links it via `updateProfile`.
2. Media Lab Pro (center) — the main test surface: drag & drop or file picker,
   upload queue with per-file progress bar, speed, phase machine
   (`IDLE → CREATING_SESSION → UPLOADING → COMPLETING → DONE / PAUSED / ERROR`),
   start / pause / resume / cancel per file, and after completion: download,
   copy download link, delete. Processing status arrives over WebSocket and is
   shown per item (SCANNING, PROCESSING with percent, READY, FAILED, DELETING).
3. Realtime events (right) — raw log of every media WebSocket event
   (`SCAN_STARTED`, `PROCESSING_PROGRESS`, `READY`, `DELETE_COMPLETED`, ...),
   with connection state, mediaId filter and clear button.
4. Testing dashboard — triggers the media-service e2e test runner and renders
   per-suite / per-case results streamed over WebSocket. Optional: see
   "Testing dashboard backend" below.

## Project structure

```text
media-frontend/
├── index.html                     Vite entry
├── vite.config.ts                 dev server + /graphql and /ws proxies
├── tsconfig.json                  TypeScript project references
├── package.json                   scripts and dependencies
├── image.png                      infographic of the unified upload flow
├── FULL_FLOW.md                   full-system flow document (upload/scan/process/deliver/delete)
├── scripts/
│   ├── cors.json                  LocalStack bucket CORS (exposes ETag)
│   └── init-localstack.sh         one-time bucket creation + CORS apply
└── src/
    ├── main.tsx                   React root (StrictMode)
    ├── App.tsx                    session gate: AuthFlow or Dashboard
    ├── styles.css                 design tokens + all styling
    ├── api/
    │   ├── client.ts              Apollo Client, authLink, env-based endpoint
    │   ├── operations.ts          all 10 GraphQL documents + typed wrappers
    │   └── testingApi.ts          REST client for the e2e test runner (:8090)
    ├── auth/
    │   └── AuthContext.tsx        register/login/logout/refreshProfile, localStorage tokens
    ├── components/
    │   ├── AuthFlow.tsx           register + login forms, optional avatar at register
    │   ├── AvatarPicker.tsx       avatar pick/preview/validate (10MB, jpeg/png/webp/gif)
    │   ├── Dashboard.tsx          4-panel layout after login
    │   ├── MediaLabPro.tsx        upload queue, WS status, download/copy/delete
    │   ├── RealtimePanel.tsx      raw WebSocket event log
    │   ├── TestingDashboard.tsx   e2e test runner panel (optional backend)
    │   ├── UploadProgress.tsx     progress bar for the avatar flow
    │   ├── StatusBadge.tsx        colored status badge (currently unused, kept as reference)
    │   └── FileUploadLab.tsx      earlier simpler upload UI (currently unused, kept as reference)
    ├── hooks/
    │   └── useTestingWS.ts        WebSocket client for the test runner
    └── upload/
        ├── types.ts               MediaStatus, session/part/user shapes, WS event types
        ├── operations helpers...  (see api/operations.ts)
        ├── presignedUpload.ts     simple part uploader (fetch) + client validation + sha256
        ├── advancedUploader.ts    full engine: chunking, pool, retry, resume, pause/offline
        ├── useFileUpload.ts       React hook wrapping the simple uploader + MIME/size table
        ├── useUploadAvatar.ts     avatar flow orchestration (session ─► parts ─► complete ─► profile)
        ├── indexedDB.ts           persistent upload sessions for resume
        ├── resumableDownload.ts   Range-based download with retry and progress
        └── websocket.ts           MediaSocket singleton: connect, reconnect, listeners
```

## Upload engine details

Two uploaders exist; both are correct, the advanced one is used by the Media
Lab Pro panel:

- `presignedUpload.ts` — minimal path used by the avatar flow. Slices the file,
  PUTs every part with `fetch` (4 workers in parallel), 3 attempts per part with
  exponential backoff + jitter, reads the `ETag` from each PUT response.
- `advancedUploader.ts` — the full engine:
  - computes a SHA-256 checksum of the whole file (`crypto.subtle.digest`) and
    sends it to `createUploadSession` for server-side verification after upload
  - reuses a previously stored session from IndexedDB when the same file
    (name + size + lastModified) is re-selected, and reconciles it with the
    backend `uploadStatus` query (completed vs missing parts) before resuming
  - part size comes from the server response (`partSize`, minimum 5 MB per
    `S3_MULTIPART_MIN_PART_SIZE`); the file is cut with `File.slice`
  - uploads with a bounded worker pool of 4 concurrent parts, using
    `XMLHttpRequest` because only `xhr.upload.onprogress` reports true upload
    progress
  - up to 5 attempts per part, exponential backoff from 1 s plus jitter
  - every completed part (number + ETag) is persisted to IndexedDB immediately,
    so closing the tab and coming back resumes from the first missing part
  - `AbortController` powers cancel; `navigator.onLine` plus `online` /
    `offline` events pause the queue when the network drops and resume it when
    it returns
  - `completeUpload` sends the ordered `{partNumber, eTag}` list; `cancel` calls
    `abortUpload` so the server and S3 multipart state are cleaned up
  - strips the quotes S3 puts around ETags before storing them

Client-side validation (advisory only, the server always re-validates):

| Kind | Allowed types | Max size |
|---|---|---|
| image | jpeg, png, webp, gif, avif | 10 MB |
| video | mp4, quicktime, x-msvideo, x-matroska, webm | 50 GB |
| document | pdf, docx, xlsx, txt, zip | 500 MB |

## GraphQL operations

All documents live in `src/api/operations.ts`, every response uses the backend
envelope `{ success, statusCode, message, data }` and errors are unwrapped from
`errors[0].extensions.exception.response.message` when present:

| Operation | Type | Purpose |
|---|---|---|
| `register` | mutation | create account, returns tokens + user |
| `login` | mutation | authenticate, returns tokens + user |
| `createUploadSession` | mutation | returns `mediaId`, `uploadId`, `s3UploadId`, `partSize`, `totalParts`, `expiresAt`, `presignedParts[]` |
| `completeUpload` | mutation | finalize multipart with ordered parts + ETags |
| `abortUpload` | mutation | cancel a session server-side |
| `uploadStatus` | query | backend view of a session: status, completed parts, missing parts |
| `downloadUrl` | query | short-lived presigned GET URL (optional `versionType`, `expirySeconds`) |
| `deleteMedia` | mutation | request deletion (optional idempotency key) |
| `updateProfile` | mutation | link `avatarMediaId` to the profile |
| `myProfile` | query | current user incl. resolved `imageUrl` |

## WebSocket (realtime media events)

`src/upload/websocket.ts` exports a shared `MediaSocket`:

- URL resolution: `VITE_MEDIA_WS_URL` if set, otherwise same host as the page on
  path `/ws` (which in dev goes through the Vite proxy).
- The JWT is passed as a query parameter: `?token=<accessToken>`.
- Reconnection uses exponential backoff with jitter, capped at 30 s.
- Subscribers register with `onEvent` / `onStateChange` and unsubscribe in
  React cleanups.
- Handled events: `SCAN_STARTED`, `SCAN_COMPLETED`, `SCAN_FAILED`,
  `PROCESSING_STARTED`, `PROCESSING_PROGRESS`, `PROCESSING_COMPLETED`,
  `PROCESSING_FAILED`, `READY`, `DELETE_STARTED`, `DELETE_COMPLETED`,
  `DELETE_FAILED` (mapped to `MediaStatus` in `types.ts`).

Important: the media-service WebSocket is a dedicated server inside the
media-service container (internal port 4003, published on the host as 4009).
The API Gateway `/ws` route proxies to the realtime-service, not to
media-service. Therefore, to receive media events in local docker, set
`VITE_MEDIA_WS_URL=ws://localhost:4009` (see environment variables below).

## Prerequisites

- Docker with the compose plugin
- Node.js 20+ and npm
- AWS CLI v2 (only to create the LocalStack bucket and apply CORS once)

## Running the stack

All compose files live in `infrastructure/docker/`.

1. Start infrastructure (Redis, NATS, Kafka, LocalStack, Postgres, RabbitMQ):

   ```bash
   docker compose -f infrastructure/docker/compose.infra.yml up -d
   ```

2. Start the services needed by this app (this compose file also starts their
   dependencies):

   ```bash
   docker compose -f infrastructure/docker/compose.yml up -d --build \
     user-service media-service api-gateway
   ```

3. Create the S3 bucket in LocalStack and apply the CORS rule that exposes the
   `ETag` header to the browser (required to complete multipart uploads):

   ```bash
   aws --endpoint-url http://localhost:4566 s3 mb s3://delivery-media-development
   aws --endpoint-url http://localhost:4566 s3api put-bucket-cors \
     --bucket delivery-media-development \
     --cors-configuration file://media-frontend/scripts/cors.json
   ```

   Or run the helper script from this folder:

   ```bash
   ./scripts/init-localstack.sh
   ```

4. Configure the frontend environment. Create `media-frontend/.env.local`:

   ```bash
   VITE_GATEWAY_URL=/graphql
   VITE_MEDIA_WS_URL=ws://localhost:4009
   ```

   `.env.local` is git-ignored. Without `VITE_MEDIA_WS_URL` the app still runs
   but the realtime panel will connect to the wrong backend (see limitations).

5. Install and start the dev server:

   ```bash
   cd media-frontend
   npm install
   npm run dev
   ```

   Open http://localhost:5173, register a user (a random Egyptian phone number
   is generated per mount to avoid clashes), optionally pick an avatar, then use
   the Media Lab Pro panel to upload files.

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `VITE_GATEWAY_URL` | `/graphql` (Vite proxies it to `http://localhost:4000`) | Apollo Client endpoint |
| `VITE_MEDIA_WS_URL` | `ws(s)://<page host>/ws` | media-service WebSocket; use `ws://localhost:4009` with the docker setup |

## npm scripts

| Script | Command | Effect |
|---|---|---|
| `npm run dev` | `vite` | dev server on port 5173 with proxies |
| `npm run build` | `tsc -b && vite build` | type-check then production bundle into `dist/` |
| `npm run preview` | `vite preview` | serve the production bundle locally |
| `npm run typecheck` | `tsc -b --noEmit` | type-check only |

## Ports used by the local backend

| Port | Service |
|---|---|
| 4000 | API Gateway (GraphQL + `/ws` proxy to realtime-service) |
| 4005 | media-service GraphQL subgraph |
| 4009 | media-service WebSocket (host mapping of container port 4003) |
| 4566 | LocalStack (S3 / DynamoDB) |
| 5173 | Vite dev server |

## Testing dashboard backend (optional)

The fourth panel (`TestingDashboard`) calls a separate REST + WebSocket API:

- `POST http://localhost:8090/api/runs` — start a run, returns `{ runId }`
- `GET http://localhost:8090/api/runs` — list runs
- `GET http://localhost:8090/api/suites` — list suites
- `ws://localhost:8090/ws` — streams `TEST_RESULT` and `RUN_COMPLETE` frames

This runner is not part of `infrastructure/docker` (the e2e suite itself lives
in `services/media-service/test/e2e`, see the plan document at the repository
root). If nothing listens on port 8090 the rest of the app works normally; the
panel just shows a disconnected dot and "run" will log an error to the console.

## Known limitations and notes

- The default WebSocket URL (no `VITE_MEDIA_WS_URL`) resolves to the page host
  on `/ws`, which the Vite proxy forwards to the API Gateway, and the gateway
  forwards to the realtime-service. Media scan/processing events are published
  by the media-service WebSocket (port 4009 locally), so set the variable.
- `FileUploadLab.tsx` and `StatusBadge.tsx` are not mounted anywhere; they are
  kept as a reference implementation of the simpler upload path and of a status
  badge component.
- Client-side validation is a convenience only. Every limit (MIME allowlist,
  magic bytes, size, checksum, quota, rate limit) is enforced again by
  media-service.
- Access and refresh tokens are stored in `localStorage`. This is acceptable
  for a lab app; a production app should prefer httpOnly cookies or an in-memory
  token with a refresh rotation flow. `refreshToken` is returned by the backend
  but this app does not implement token renewal yet.
- Presigned URLs are signed by media-service with
  `AWS_ENDPOINT=http://host.docker.internal:4566` so the browser can reach
  LocalStack. The uploader additionally rewrites the internal host
  `http://localstack-srv:4566` to `http://<page hostname>:4566`. If you run the
  Go service natively instead of docker, use `AWS_ENDPOINT=http://localhost:4566`.
- One `MediaSocket` instance is shared by the avatar flow, Media Lab Pro and the
  Realtime panel; they all call `connect` / `disconnect` on it. If a component
  unmounts and disconnects while another still needs events, reconnect happens
  on the next `connect` call. Acceptable for a lab app, worth revisiting in a
  production shell.
- Video upload / download / renditions in this app are generic file flows; the
  heavy backend work (ClamAV scan, FFmpeg renditions, HLS, WebP/AVIF, metadata)
  happens in the media-service workers, driven by Kafka events, and is observed
  through the WebSocket panel rather than dedicated video UI.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Part PUT fails with a CORS error in the browser | bucket CORS missing or ETag not exposed | re-apply `scripts/cors.json` (step 3 above) |
| Part PUT fails with 403 | presigned URL expired (1 h) or host mismatch | restart the upload; verify `AWS_ENDPOINT` is `host.docker.internal:4566` |
| `completeUpload` fails on ETag mismatch | ETag quotes were kept or a part was overwritten | ensure the stored ETag has no surrounding quotes (advancedUploader strips them) |
| Realtime panel stays disconnected | wrong WS URL | set `VITE_MEDIA_WS_URL=ws://localhost:4009` and restart `npm run dev` |
| Upload rejected with bucket not found | LocalStack bucket missing | run step 3 |
| Testing dashboard shows nothing | no runner on port 8090 | start the e2e runner or ignore the panel |

## Related documents

- [FULL_FLOW.md](./FULL_FLOW.md) — full media subsystem flow (upload, scan,
  processing, delivery, delete, CDN, observability), and [image.png](./image.png)
  its infographic.
- `../services/media-service/README.md` — the media-service itself.
- `../MEDIA_FRONTEND_TERMS_AR.md` — Arabic glossary mapping every term used in
  this app and the media-service to concrete file paths.
- `../media_service_e2e_test_plan.md` — plan for the e2e suite that the testing
  dashboard can trigger.
