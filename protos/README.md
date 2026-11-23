# Protocol Buffers (gRPC Definitions)

This directory contains the Protocol Buffer (`proto3`) interface definitions for synchronous inter-service communication across the Realtime Delivery Microservices platform.

While public-facing client requests are served via the GraphQL API Gateway, internal microservices communicate directly over low-latency, strongly typed gRPC channels defined by these proto schemas.

---

## Directory Overview

| File | Package Name | Service Defined | Core Responsibility |
| :--- | :--- | :--- | :--- |
| `user.proto` | `user` | `UserService` | User profile retrieval, permission checks, role updates, and JWT validation. |
| `driver.proto` | `driver` | `DriverService` | Driver discovery, reservation, assignment, and status retrieval. |
| `delivery.proto` | `delivery` | `DeliveryService` | Delivery validation, participant verification, order summaries, and payment refund triggers. |
| `payment.proto` | `payment.v1` | `PaymentService` | Transaction lifecycle (creation, authorization, capture, cancellation, and refund). |
| `analytics.proto` | `analytics` | `AnalyticsService` | High-level metrics aggregation, platform performance overview, and driver statistics. |
| `media.proto` | `media` | `MediaService` | Resolution of secure pre-signed media URLs and metadata verification. |
| `notification.proto`| `notification`| `NotificationService` | Direct delivery of internal transactional alerts and notifications. |
| `payment.pb.go` | `paymentpb` | Generated Code | Generated Go structures and serialization helpers for `payment.proto`. |
| `payment_grpc.pb.go`| `paymentpb`| Generated Code | Generated gRPC client stubs and server interface handlers for `payment.proto`. |

---

## Service Contracts & RPC Definitions

### 1. User Service (`user.proto`)
Manages authentication token verification, user retrieval, and authorization permissions for internal microservices.

- `GetUser(GetUserRequest) returns (GetUserResponse)`: Fetches user profile attributes by unique user ID.
- `GetUserByEmail(GetUserByEmailRequest) returns (GetUserResponse)`: Lookups user by email address.
- `ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse)`: Validates JWT access tokens and extracts user ID and role.
- `GetUserPermissions(GetUserPermissionsRequest) returns (GetUserPermissionsResponse)`: Retrieves the list of permission strings granted to a user.
- `UpdateUserRole(UpdateUserRoleRequest) returns (UpdateUserRoleResponse)`: Elevates or updates the user role (for example, promoting a customer to a `DRIVER`).

### 2. Driver Service (`driver.proto`)
Coordinates fleet dispatching, geographic search, and driver allocation.

- `FindAvailableDrivers(FindAvailableDriversRequest) returns (FindAvailableDriversResponse)`: Finds driver candidates within a given radius based on coordinates and vehicle type (`CAR`, `MOTORCYCLE`, `TRUCK`, `BICYCLE`, `VAN`).
- `ReserveDriver(ReserveDriverRequest) returns (ReserveDriverResponse)`: Temporarily locks a driver for a delivery attempt with an idempotency key.
- `ReleaseDriver(ReleaseDriverRequest) returns (ReleaseDriverResponse)`: Releases a reserved driver back into the available pool if an offer expires or is rejected.
- `AssignDriver(AssignDriverRequest) returns (AssignDriverResponse)`: Finalizes driver assignment for an active delivery.
- `GetDriver(GetDriverRequest) returns (GetDriverResponse)`: Returns driver registration details and vehicle specification.
- `GetDriverStatus(GetDriverStatusRequest) returns (GetDriverStatusResponse)`: Checks current state (`ONLINE`, `OFFLINE`, `BUSY`) and current active delivery ID.
- `ValidateID(ValidateIDRequest) returns (ValidateIDResponse)`: Verifies if a given ID corresponds to an active, valid driver.

### 3. Delivery Service (`delivery.proto`)
Internal contract for fetching delivery state, participant relationships, and open order pools.

- `IsParticipant(ParticipantRequest) returns (ParticipantResponse)`: Validates whether a specific user is the customer or assigned driver of a delivery.
- `GetDelivery(GetDeliveryRequest) returns (GetDeliveryResponse)`: Returns the current delivery record, financial amount, and fulfillment state.
- `RefundPayment(RefundPaymentRequest) returns (RefundPaymentResponse)`: Triggers a refund request within the delivery domain workflow.
- `GetOpenDeliveries(GetOpenDeliveriesRequest) returns (GetOpenDeliveriesResponse)`: Fetches paginated lists of unassigned or active deliveries for dispatching.

### 4. Payment Service (`payment.proto`)
Financial workflow engine operating on minor currency units (e.g., cents) with correlation tracking and idempotency.

- `CreatePayment(CreatePaymentRequest) returns (CreatePaymentResponse)`: Initializes a payment record and returns payment links or gateway references.
- `AuthorizePayment(AuthorizePaymentRequest) returns (AuthorizePaymentResponse)`: Holds authorization on customer payment methods.
- `CapturePayment(CapturePaymentRequest) returns (CapturePaymentResponse)`: Captures previously authorized funds upon successful delivery completion.
- `CancelAuthorization(CancelAuthorizationRequest) returns (CancelAuthorizationResponse)`: Cancels holds on authorizations for failed or rejected orders.
- `CreateRefund(CreateRefundRequest) returns (CreateRefundResponse)`: Executes a full or partial refund against a completed payment.
- `GetPayment(GetPaymentRequest) returns (GetPaymentResponse)`: Returns transaction records and gateway IDs.
- `GetPaymentStatus(GetPaymentStatusRequest) returns (GetPaymentStatusResponse)`: Retrieves the real-time settlement status or failure reasons.

### 5. Analytics Service (`analytics.proto`)
Provides metrics aggregation for dashboards and reporting systems.

- `GetPlatformOverview(PlatformOverviewRequest) returns (PlatformOverviewResponse)`: Aggregates total deliveries, completed orders, and completion percentages for a given time window (`from` / `to`).
- `GetDriverPerformance(DriverPerformanceRequest) returns (DriverPerformanceResponse)`: Computes individual driver metrics including delivery offers received, acceptance rates, and completed deliveries.

### 6. Media Service (`media.proto`)
Asset verification and time-limited pre-signed URL generation.

- `ResolveMediaUrl(ResolveMediaUrlRequest) returns (ResolveMediaUrlResponse)`: Produces authenticated, short-lived URLs (with customizable expiry) for media display (such as avatars, delivery proof images).
- `GetMedia(GetMediaRequest) returns (GetMediaResponse)`: Validates owner authorization and returns file metadata (size, content type, storage object key).

### 7. Notification Service (`notification.proto`)
Direct messaging channel for high-priority inter-service notifications.

- `SendNotification(SendNotificationRequest) returns (SendNotificationResponse)`: Transmits notifications across internal channels with custom payload data, title, body, and priority levels.

---

## Compilation and Code Generation

### Prerequisites
To compile these `.proto` files into Go code, install the `protoc` compiler and Go plugins:

```powershell
# Install protoc-gen-go and protoc-gen-go-grpc
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Ensure `$GOPATH/bin` is added to your system `PATH`.

### Generating Code Example

To generate Go files for any proto file (e.g., `driver.proto`), execute:

```powershell
protoc --proto_path=protos `
       --go_out=protos --go_opt=paths=source_relative `
       --go-grpc_out=protos --go-grpc_opt=paths=source_relative `
       protos/driver.proto
```

For Node.js or TypeScript services, you can compile these files using `@grpc/proto-loader` or `ts-proto` depending on the service setup.

---

## Best Practices & Guidelines

1. **Backward Compatibility**: Do not change existing field numbers or rename serialized enum values. Use `reserved` statements when deleting fields.
2. **Idempotency**: All mutation-oriented RPCs (such as payments, reservations, and updates) accept an `idempotency_key` to avoid duplicate operations during network retries.
3. **Traceability**: Pass `correlation_id` across RPC calls to preserve request traces throughout the distributed system.
4. **Data Isolation**: Services must not query external databases directly; all cross-domain operations must pass through these gRPC contracts.
