# Realtime Delivery Microservices - API Documentation

This directory contains the GraphQL schema operation examples and HTTP/REST observability documentation for the Realtime Delivery Microservices platform.

All client-facing operations are routed through the API Gateway, which aggregates downstream microservices into a unified entry point, while observability endpoints are accessible via direct REST/HTTP probes.

---

## Directory Overview

The folder is structured by microservice domain. Each `.graphql` file contains sample queries, mutations, and subscriptions with documented headers, variables, and permissions. The `observability.md` file documents health checks and metric endpoints.

| File | Type | Target Microservice | Primary Purpose |
| :--- | :--- | :--- | :--- |
| `apiGW.graphql` | REST / HTTP | API Gateway | Gateway ping and connectivity test. |
| `auth.graphql` | GraphQL | Authentication Service | User registration, login, token refresh, and logout. |
| `user.graphql` | GraphQL | User Profile Service | User details retrieval, profile updates, and address management. |
| `driver.graphql` | GraphQL | Driver Service | Driver registration, vehicle management, shift status, and geolocation updates. |
| `delivery.graphql` | GraphQL | Delivery Service | Delivery creation, status transitions, assignment tracking, and price calculation. |
| `payment.graphql` | GraphQL | Payment Service | Payment processing, transaction verification, and refund requests. |
| `realtime.graphql` | GraphQL / WS | Realtime Service | Live driver tracking, delivery state subscriptions, and WebSocket connection status. |
| `search.graphql` | GraphQL | Search Service | Geospatial search, nearby driver discovery, and order filtering. |
| `notification.graphql` | GraphQL | Notification Service | In-app alerts, push notifications, read states, and delivery status notifications. |
| `analytics.graphql` | GraphQL | Analytics Service | Business intelligence queries, platform overview metrics, and driver performance aggregations. |
| `media.graphql` | GraphQL / REST | Media Service | File upload pre-signed URLs, media asset retrieval, and attachment verification. |
| `observability.md` | Markdown | All Services | Complete reference for REST health checks (`/health`), readiness probes (`/ready`), and Prometheus metric scrape endpoints (`/metrics`). |

---

## Architecture & Gateway Routing

1. **GraphQL Operations**: All queries, mutations, and subscriptions are sent to the central API Gateway endpoint:
   - HTTP URL: `http://localhost:4000/graphql`
   - WebSocket URL (Subscriptions): `ws://localhost:4000/graphql`

2. **Authentication Flow**:
   - Public operations (`register`, `login`) return an `accessToken` (JWT) and a `refreshToken`.
   - Protected operations require an HTTP authorization header:
     ```http
     Authorization: Bearer <YOUR_ACCESS_TOKEN>
     ```
   - Downstream services receive decoded user claims passed down by the API Gateway.

3. **Service Details & Port Reference**:
   - `API Gateway`: Port 4000 (Liveness check: Port 4099)
   - Downstream microservices expose GraphQL APIs internally or via the gateway, and expose standard REST observability endpoints on their respective ports.

---

## Detailed Service Breakdown

### 1. Authentication (`auth.graphql`)
Handles identity lifecycle and token management.
- **Register**: Creates new user records with password hashing.
- **Login**: Validates credentials and returns JWT session tokens.
- **Refresh Token**: Renews expired access tokens without re-authenticating.
- **Logout / Revocation**: Invalidates active refresh tokens.

### 2. User Profile (`user.graphql`)
Manages account profiles and saved addresses.
- **Get Current User**: Retrieves logged-in profile data.
- **Update Profile**: Updates personal metadata, phone number, and avatar reference.
- **Address Book**: Adds, lists, updates, and deletes pickup/dropoff addresses with coordinate metadata.

### 3. Driver Management (`driver.graphql`)
Manages the lifecycle and state of fleet drivers.
- **Registration & Verification**: Onboards drivers with vehicle type, license plate, capacity, and service area.
- **Shift & Availability**: Toggles driver status between `ONLINE`, `OFFLINE`, and `BUSY`.
- **Location Updates**: Ingests real-time latitude/longitude coordinates and heading for dispatch matching.
- **Driver Profiles & Metrics**: Fetches driver status, rating, and accepted orders history.

### 4. Delivery Operations (`delivery.graphql`)
Core domain orchestration for order dispatch and delivery lifecycles.
- **Create Delivery**: Initializes delivery orders with pickup/dropoff details and idempotency protection.
- **Status Progression**: Handles workflow state transitions (`PENDING`, `ASSIGNED`, `PICKED_UP`, `DELIVERED`, `CANCELLED`).
- **Estimation & Pricing**: Estimates shipping costs based on distance, package size, and vehicle capacity.

### 5. Payments (`payment.graphql`)
Handles financial transactions associated with deliveries.
- **Initialize Payment**: Creates transaction intents for card or wallet payments.
- **Confirm Payment**: Processes webhooks and client confirmations.
- **Refunds & Disputes**: Issues full or partial refunds for cancelled deliveries.

### 6. Realtime Tracking (`realtime.graphql`)
Low-latency state streaming and WebSocket management.
- **Location Streaming**: Subscribes to driver coordinates during active deliveries.
- **Order State Notifications**: Real-time push updates for delivery status changes.
- **Connection Diagnostics**: Queries active connection metrics and client socket state.

### 7. Search & Discovery (`search.graphql`)
Geospatial and text search powered by dedicated search indexes.
- **Nearby Driver Lookup**: Locates available drivers within a specified geographic radius.
- **Order & History Search**: Multi-filter queries for deliveries based on dates, statuses, and customer IDs.

### 8. Notifications (`notification.graphql`)
Multi-channel notification management.
- **In-App Inbox**: Lists customer and driver alerts.
- **State Updates**: Marks notifications as read or archived.
- **Preferences**: Configures push and SMS notification triggers.

### 9. Analytics & Reporting (`analytics.graphql`)
Aggregated business and system telemetry queries for administrative dashboards.
- **Platform Overview**: Orders volume, gross merchandise value (GMV), and completion rates aggregated by time interval (`DAY`, `WEEK`, `MONTH`).
- **Performance Reports**: Driver utilization, average transit duration, and delivery heatmaps.

### 10. Media Management (`media.graphql`)
Handles file and binary storage operations.
- **Presigned URLs**: Requests secure upload URLs for package photos, receipts, and driver documents.
- **Asset Metadata**: Validates file MIME types, dimensions, and persistence links.

### 11. Observability & Monitoring (`observability.md`)
Dedicated documentation for platform health and metrics.
- **Health Checks**: `/health` endpoints for basic liveness.
- **Readiness Probes**: `/health/ready` endpoints verifying database, Redis, and RabbitMQ dependencies.
- **Prometheus Metrics**: `/metrics` endpoints for OpenTelemetry and Prometheus scraping across all microservices.

---

## How to Execute the GraphQL Files

### Using an HTTP Client or Terminal
Use `curl` or PowerShell `Invoke-RestMethod` against the gateway:

```powershell
# Example: Sending a query to API Gateway
curl -X POST http://localhost:4000/graphql `
  -H "Content-Type: application/json" `
  -H "Authorization: Bearer <TOKEN_HERE>" `
  -d '{"query": "query { me { id firstName email } }"}'
```

### Using GraphQL GUI Clients
You can copy queries from any `.graphql` file into tools like Apollo Studio, Postman, Insomnia, or Altair GraphQL Client pointing to `http://localhost:4000/graphql`. Set authorization tokens in the request header tab.
