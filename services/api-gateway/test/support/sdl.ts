/**
 * Federation v2 SDL for every subgraph the API Gateway composes.
 *
 * These definitions mirror the operations documented in /graphql-docs so the
 * gateway's `IntrospectAndCompose` builds a supergraph that accepts *every*
 * documented query & mutation. Each subgraph owns its own entity; shared
 * entities (User / Driver / Delivery) are referenced via minimal `@key`
 * stubs so cross-service entity resolution is exercised for real.
 */

const FED_LINK = `extend schema @link(
  url: "https://specs.apollo.dev/federation/v2.3"
  import: ["@key", "@external", "@shareable", "@requires", "@provides", "@tag", "@override"]
)`;

/** Minimal entity stubs used by subgraphs that *reference* an owned entity. */
export const USER_ENTITY_STUB = `type User @key(fields: "id") { id: ID! }`;
export const DRIVER_ENTITY_STUB = `type Driver @key(fields: "id") { id: ID! }`;
export const DELIVERY_ENTITY_STUB = `type Delivery @key(fields: "id") { id: ID! }`;

/** Builds a GraphQL "envelope" response field block (success/statusCode/...). */
function envelope(dataType?: string): string {
  return `
    statusCode: Int
    message: String
    success: Boolean
    timeStamp: String
    ${dataType ? `data: ${dataType}` : ''}
  `;
}

// ─────────────────────────────────────────────────────────────────────────────
// USER subgraph (owns User + Address + auth)
// ─────────────────────────────────────────────────────────────────────────────
const USER = `
  ${FED_LINK}

  type User @key(fields: "id") {
    id: ID!
    firstName: String
    lastName: String
    email: String
    isActive: Boolean
    imageUrl: String
    addresses: [Address]
    role: String
    phoneNumber: String
    createdAt: String
  }

  type Address {
    id: ID
    title: String
    street: String
    city: String
    state: String
    postalCode: String
    latitude: Float
    longitude: Float
    isDefault: Boolean
    createdAt: String
  }

  type AuthPayload {
    accessToken: String
    refreshToken: String
    user: User
  }

  input RegisterInput {
    firstName: String!
    lastName: String!
    email: String!
    password: String!
    phoneNumber: String
  }

  input LoginInput {
    email: String!
    password: String!
  }

  input UpdateProfileInput {
    firstName: String
    lastName: String
  }

  input AddAddressInput {
    title: String!
    street: String!
    city: String!
    state: String!
    postalCode: String
    latitude: Float
    longitude: Float
    isDefault: Boolean
  }

  input ChangePasswordInput {
    passwordOld: String!
    passwordNew: String!
  }

  input ForgetPasswordInput {
    email: String!
  }

  input ResetPasswordInput {
    token: String!
    passwordNew: String!
  }

  input RefreshTokenInput {
    refreshToken: String!
  }

  type UserPingResponse { ${envelope('Boolean')} }
  type UserResponse { ${envelope('User')} }
  type UserBooleanResponse { ${envelope('Boolean')} }
  type UserBasicResponse {
    statusCode: Int
    message: String
    success: Boolean
    timeStamp: String
  }
  type UserAddressesResponse { ${envelope('[Address]')} }
  type UserAddressResponse { ${envelope('Address')} }
  type AuthResponse { ${envelope('AuthPayload')} }

  type Query {
    pingForUser: UserPingResponse
    myProfile: UserResponse
    user(id: ID!): UserResponse
    myAddresses: UserAddressesResponse
  }

  type Mutation {
    updateProfile(input: UpdateProfileInput!): UserResponse
    promoteUserToAdmin(id: ID!): UserBooleanResponse
    toggleUserActive(id: ID!, isActive: Boolean!): UserBasicResponse
    deleteUser(id: ID!): UserBasicResponse
    addAddress(input: AddAddressInput!): UserAddressResponse
    deleteAddress(addressId: ID!): UserBasicResponse
    setDefaultAddress(addressId: ID!): UserBasicResponse
    register(input: RegisterInput!): AuthResponse
    login(input: LoginInput!): AuthResponse
    changePassword(input: ChangePasswordInput!): UserBasicResponse
    forgetPassword(input: ForgetPasswordInput!): UserBasicResponse
    resetPassword(input: ResetPasswordInput!): UserBasicResponse
    refreshToken(input: RefreshTokenInput!): AuthResponse
    logout: UserBasicResponse
  }
`;

// ─────────────────────────────────────────────────────────────────────────────
// DELIVERY subgraph (owns Delivery)
// ─────────────────────────────────────────────────────────────────────────────
const DELIVERY = `
  ${FED_LINK}
  ${DRIVER_ENTITY_STUB}

  """
  The delivery service owns the "deliveries of a user" edge, so User is
  extended here with it (this also gives the depth-limit tests a real cycle:
  Delivery → User → Delivery → ...).
  """
  type User @key(fields: "id") {
    id: ID!
    deliveries: [Delivery]
  }

  type Delivery @key(fields: "id") {
    id: ID!
    driver: Driver
    customer: User
    status: String
    paymentStatus: String
    checkoutUrl: String
    amount: String
    currency: String
    pickupAddress: DeliveryAddress
    dropoffAddress: DeliveryAddress
    statusHistory: [StatusHistoryEntry]
    pickedUpAt: String
    completedAt: String
    cancelledAt: String
    createdAt: String
    updatedAt: String
  }

  type DeliveryAddress {
    line1: String
    line2: String
    city: String
    state: String
    postalCode: String
    countryCode: String
    latitude: Float
    longitude: Float
  }

  type StatusHistoryEntry {
    status: String
    changedBy: String
    note: String
    createdAt: String
  }

  enum DeliveryStatus {
    PENDING_PAYMENT
    PAYMENT_CONFIRMED
    DRIVER_ASSIGNED
    DRIVER_ACCEPTED
    PICKED_UP
    IN_TRANSIT
    DELIVERED
    COMPLETED
    CANCELLED
  }

  input DeliveryAddressInput {
    line1: String!
    line2: String
    city: String!
    state: String
    postalCode: String
    countryCode: String
    latitude: Float
    longitude: Float
  }

  input CreateDeliveryInput {
    amount: String!
    currency: String!
    pickupAddress: DeliveryAddressInput!
    dropoffAddress: DeliveryAddressInput!
    idempotencyKey: String
  }

  input TransitionDeliveryInput {
    deliveryId: ID!
    status: DeliveryStatus!
    note: String
  }

  type DeliveryPaginationInfo {
    totalItems: Int
    currentPage: Int
    nextPage: Int
  }

  type DeliveryData {
    paginationInfo: DeliveryPaginationInfo
    items: [Delivery]
  }

  type OpenDeliveriesData {
    items: [Delivery]
    paginationInfo: DeliveryPaginationInfo
  }

  type ServiceInfoData @shareable {
    name: String
    version: String
    status: String
  }

  type NextStatusesData {
    statuses: [String]
  }

  type CancelDeliveryData {
    id: ID
    status: String
    cancelledAt: String
    updatedAt: String
  }

  type DeliveryServiceInfoResponse { ${envelope('ServiceInfoData')} }
  type DeliveryResponse { ${envelope('Delivery')} }
  type MyDeliveriesResponse { ${envelope('DeliveryData')} }
  type OpenDeliveriesResponse { ${envelope('OpenDeliveriesData')} }
  type DeliveryNextStatusesResponse { ${envelope('NextStatusesData')} }
  type CancelDeliveryResponse { ${envelope('CancelDeliveryData')} }

  type Query {
    deliveryServiceInfo: DeliveryServiceInfoResponse
    deliveryNextStatuses(id: ID!): DeliveryNextStatusesResponse
    myDeliveries(page: Int, pageSize: Int): MyDeliveriesResponse
    delivery(id: ID!): DeliveryResponse
    openDeliveries(page: Int, pageSize: Int): OpenDeliveriesResponse
  }

  type Mutation {
    createDelivery(input: CreateDeliveryInput!): DeliveryResponse
    transitionDelivery(input: TransitionDeliveryInput!): DeliveryResponse
    cancelDelivery(deliveryId: ID!, reason: String): CancelDeliveryResponse
  }
`;

// ─────────────────────────────────────────────────────────────────────────────
// DRIVER subgraph (owns Driver + Assignment)
// ─────────────────────────────────────────────────────────────────────────────
const DRIVER = `
  ${FED_LINK}
  ${USER_ENTITY_STUB}
  ${DELIVERY_ENTITY_STUB}

  type Driver @key(fields: "id") {
    id: ID!
    user: User
    status: String
    vehicleType: String
    plateNumber: String
    capacityKg: Float
    capabilities: [String]
    serviceArea: String
    isActive: Boolean
    isBlocked: Boolean
    rating: Float
    createdAt: String
    updatedAt: String
  }

  enum VehicleType {
    MOTORCYCLE
    CAR
    VAN
    TRUCK
  }

  type Assignment {
    id: ID
    delivery: Delivery
    driver: Driver
    status: String
    attemptNumber: Int
    offeredAt: String
    expiresAt: String
    acceptedAt: String
    rejectedAt: String
    completedAt: String
    createdAt: String
    updatedAt: String
  }

  type DispatchAttempt {
    id: ID
    delivery: Delivery
    driver: Driver
    distanceMeters: Float
    attemptNumber: Int
    result: String
    reason: String
    createdAt: String
  }

  type NearbyDriverEntry {
    driver: Driver
    distanceMeters: Float
    status: String
    vehicleType: String
    latitude: Float
    longitude: Float
    rating: Float
    isBlocked: Boolean
  }

  type AvailableDelivery {
    id: ID
    customerId: ID
    status: String
    pickupCity: String
    pickupCountry: String
    dropoffCity: String
    dropoffCountry: String
    amount: String
    currency: String
    createdAt: String
  }

  type DriverReview {
    id: ID
    driver: Driver
    user: User
    delivery: Delivery
    rating: Float
    comment: String
    createdAt: String
  }

  type SelfAcceptData {
    id: ID
    deliveryId: ID
    driverId: ID
    status: String
    offeredAt: String
    acceptedAt: String
    expiresAt: String
  }

  type DriverStatusData {
    status: String
    hasActiveAssignment: Boolean
    activeDelivery: Delivery
    lastSeenAt: String
  }

  type DriverPaginationInfo {
    totalItems: Int
    currentPage: Int
    nextPage: Int
  }

  type NearbyDriversData {
    paginationInfo: DriverPaginationInfo
    items: [NearbyDriverEntry]
  }

  type DispatchAttemptsData {
    paginationInfo: DriverPaginationInfo
    items: [DispatchAttempt]
  }

  type AvailableDeliveriesData {
    totalItems: Int
    items: [AvailableDelivery]
  }

  type DriverReviewsData {
    paginationInfo: DriverPaginationInfo
    items: [DriverReview]
    averageRating: Float
    totalReviews: Int
  }

  type ServiceInfoData @shareable {
    name: String
    version: String
    status: String
  }

  input RegisterDriverInput {
    userId: ID!
    plateNumber: String!
    capacityKg: Float
    vehicleType: VehicleType!
    capabilities: [String]
    serviceArea: String
  }

  input UpdateDriverProfileInput {
    driverId: ID!
    vehicleType: VehicleType
    plateNumber: String
    capacityKg: Float
    serviceArea: String
  }

  input NearbyDriversInput {
    latitude: Float!
    longitude: Float!
    radiusKm: Float
    vehicleType: VehicleType
    limit: Int
  }

  input RateDriverInput {
    driverId: ID!
    deliveryId: ID!
    rating: Float!
    comment: String
  }

  type DriverServiceInfoResponse { ${envelope('ServiceInfoData')} }
  type DriverResponse { ${envelope('Driver')} }
  type DriverStatusResponse { ${envelope('DriverStatusData')} }
  type NearbyDriversResponse { ${envelope('NearbyDriversData')} }
  type AssignmentResponse { ${envelope('Assignment')} }
  type DispatchAttemptsResponse { ${envelope('DispatchAttemptsData')} }
  type AvailableDeliveriesResponse { ${envelope('AvailableDeliveriesData')} }
  type SelfAcceptResponse { ${envelope('SelfAcceptData')} }
  type RateDriverResponse { ${envelope('DriverReview')} }
  type DriverReviewsResponse { ${envelope('DriverReviewsData')} }

  type Query {
    driverServiceInfo: DriverServiceInfoResponse
    driver(id: ID!): DriverResponse
    myDriverProfile: DriverResponse
    driverStatus(driverId: ID!): DriverStatusResponse
    nearbyDrivers(input: NearbyDriversInput!): NearbyDriversResponse
    driverActiveAssignment(driverId: ID!): AssignmentResponse
    dispatchAttempts(deliveryId: ID!): DispatchAttemptsResponse
    assignment(id: ID!): AssignmentResponse
    availableDeliveries: AvailableDeliveriesResponse
    driverReviews(driverId: ID!, page: Int, limit: Int): DriverReviewsResponse
  }

  type Mutation {
    registerDriver(input: RegisterDriverInput!): DriverResponse
    updateDriverProfile(input: UpdateDriverProfileInput!): DriverResponse
    activateDriver(driverId: ID!): DriverResponse
    goOnline(idempotencyKey: String): DriverResponse
    goOffline(idempotencyKey: String): DriverResponse
    blockDriver(driverId: ID!, reason: String): DriverResponse
    unblockDriver(driverId: ID!, reason: String): DriverResponse
    suspendDriver(driverId: ID!, reason: String): DriverResponse
    deactivateDriver(driverId: ID!): DriverResponse
    acceptAssignment(assignmentId: ID!, idempotencyKey: String): AssignmentResponse
    rejectAssignment(assignmentId: ID!, reason: String, idempotencyKey: String): AssignmentResponse
    selfAcceptDelivery(deliveryId: ID!, idempotencyKey: String): SelfAcceptResponse
    rateDriver(input: RateDriverInput!): RateDriverResponse
  }
`;

// ─────────────────────────────────────────────────────────────────────────────
// PAYMENT subgraph (owns Payment)
// ─────────────────────────────────────────────────────────────────────────────
const PAYMENT = `
  ${FED_LINK}
  ${DELIVERY_ENTITY_STUB}
  ${USER_ENTITY_STUB}

  type Payment @key(fields: "id") {
    id: ID!
    deliveryId: ID
    delivery: Delivery
    userId: ID
    user: User
    amountMinor: Float
    currency: String
    status: String
    provider: String
    correlationId: String
    causationId: String
    createdAt: String
    updatedAt: String
    authorizedAt: String
    capturedAt: String
  }

  type PaymentPaginationInfo {
    totalItems: Int
    currentPage: Int
    nextPage: Int
  }

  type PaymentsData {
    paginationInfo: PaymentPaginationInfo
    items: [Payment]
  }

  type ServiceInfoData @shareable {
    name: String
    version: String
    status: String
  }

  type PaymentServiceInfoResponse { ${envelope('ServiceInfoData')} }
  type PaymentsResponse { ${envelope('PaymentsData')} }
  type PaymentResponse { ${envelope('Payment')} }

  type Query {
    paymentServiceInfo: PaymentServiceInfoResponse
    payments(page: Int, limit: Int): PaymentsResponse
    payment(id: ID!): PaymentResponse
  }
`;

// ─────────────────────────────────────────────────────────────────────────────
// NOTIFICATION subgraph
// ─────────────────────────────────────────────────────────────────────────────
const NOTIFICATION = `
  ${FED_LINK}

  enum NotificationType {
    DELIVERY_CREATED
    DELIVERY_ASSIGNED
    DRIVER_LOCATION
    PAYMENT_CONFIRMED
    PASSWORD_RESET
  }

  enum NotificationChannel {
    SMS
    EMAIL
    PUSH
    IN_APP
    WEBHOOK
  }

  enum NotificationPriority {
    LOW
    NORMAL
    HIGH
  }

  type NotificationDelivery {
    id: ID
    channel: String
    status: String
    sentAt: String
    deliveredAt: String
  }

  type Notification {
    id: ID
    type: String
    title: String
    body: String
    priority: String
    createdAt: String
    readAt: String
    deliveries: [NotificationDelivery]
  }

  type NotificationPreference {
    id: ID
    type: String
    channel: String
    enabled: Boolean
  }

  input NotificationChannelSettingInput {
    channel: NotificationChannel!
    enabled: Boolean!
  }

  input NotificationPreferenceInput {
    type: NotificationType!
    channels: [NotificationChannelSettingInput!]!
  }

  type NotificationsData {
    totalCount: Int
    items: [Notification]
  }

  type PingResponse @shareable { ${envelope('Boolean')} }
  type NotificationsResponse { ${envelope('NotificationsData')} }
  type NotificationResponse { ${envelope('Notification')} }
  type UnreadCountResponse { ${envelope('Int')} }
  type NotificationBasicResponse {
    statusCode: Int
    message: String
    success: Boolean
    timeStamp: String
  }

  type NotificationPreferencesResponse {
    statusCode: Int
    message: String
    success: Boolean
    timeStamp: String
    items: [NotificationPreference]
  }

  type Query {
    pingForNotification: PingResponse
    myNotifications: NotificationsResponse
    notification(id: ID!): NotificationResponse
    unreadNotificationCount: UnreadCountResponse
    myNotificationPreferences: NotificationPreferencesResponse
  }

  type Mutation {
    markNotificationAsRead(id: ID!): NotificationBasicResponse
    markAllNotificationsAsRead: NotificationBasicResponse
    deleteNotification(id: ID!): NotificationBasicResponse
    updateNotificationPreferences(preferences: [NotificationPreferenceInput!]!): NotificationPreferencesResponse
  }
`;

// ─────────────────────────────────────────────────────────────────────────────
// REALTIME subgraph
// ─────────────────────────────────────────────────────────────────────────────
const REALTIME = `
  ${FED_LINK}

  type ConnectionStatusData {
    isConnected: Boolean
    lastSeen: String
    connectionCount: Int
  }

  type ByRole {
    customers: Int
    drivers: Int
    admins: Int
  }

  type ActiveConnectionsData {
    total: Int
    byRole: ByRole
  }

  type PingResponse @shareable { ${envelope('Boolean')} }
  type ConnectionStatusResponse { ${envelope('ConnectionStatusData')} }
  type ActiveConnectionsResponse { ${envelope('ActiveConnectionsData')} }

  type Query {
    pingForRealtime: PingResponse
    getConnectionStatus(userId: ID!): ConnectionStatusResponse
    getActiveConnections: ActiveConnectionsResponse
  }
`;

// ─────────────────────────────────────────────────────────────────────────────
// MEDIA subgraph
// ─────────────────────────────────────────────────────────────────────────────
const MEDIA = `
  ${FED_LINK}

  type MediaVersion {
    versionType: String
    objectKey: String
    contentType: String
    size: Float
    width: Int
    height: Int
    durationMs: Int
  }

  type MediaItem {
    mediaId: ID
    ownerId: ID
    fileName: String
    contentType: String
    mediaType: String
    size: Float
    objectKey: String
    versions: [MediaVersion]
    status: String
    createdAt: String
  }

  type MediaPaginationInfo {
    totalItems: Int
    currentPage: Int
    nextPage: Int
  }

  type MediaListData {
    paginationInfo: MediaPaginationInfo
    items: [MediaItem]
  }

  type ServiceInfoData @shareable {
    name: String
    version: String
    status: String
  }

  type UploadStatusData {
    uploadId: ID
    status: String
    totalParts: Int
    completedParts: Int
    missingParts: [Int]
    expiresAt: String
  }

  type DownloadUrlData {
    url: String
    expiresAt: String
    contentType: String
  }

  type QuotaData {
    usedBytes: Float
    quotaBytes: Float
    remainingBytes: Float
    activeUploads: Int
    maxConcurrentUploads: Int
  }

  type DlqStatData {
    topic: String
    messageCount: Int
  }

  type PresignedPart {
    partNumber: Int
    presignedUrl: String
  }

  type UploadSessionData {
    mediaId: ID
    uploadId: ID
    s3UploadId: String
    presignedParts: [PresignedPart]
    partSize: Float
    totalParts: Int
    expiresAt: String
  }

  type CompleteUploadData {
    mediaId: ID
    status: String
  }

  type DlqReplayData {
    success: Boolean
    replayedCount: Int
    errors: [String]
  }

  input CreateUploadSessionInput {
    fileName: String!
    contentType: String!
    size: Float!
    checksum: String
    deliveryId: ID
    idempotencyKey: String
  }

  input UploadPartInput {
    partNumber: Int!
    eTag: String
  }

  input CompleteUploadInput {
    uploadId: ID!
    parts: [UploadPartInput!]!
    idempotencyKey: String
  }

  input RenewPresignedInput {
    uploadId: ID!
    expirySeconds: Int
  }

  type ListMediaResponse { ${envelope('MediaListData')} }
  type MediaResponse { ${envelope('MediaItem')} }
  type UploadStatusResponse { ${envelope('UploadStatusData')} }
  type DownloadUrlResponse { ${envelope('DownloadUrlData')} }
  type QuotaResponse { ${envelope('QuotaData')} }
  type DlqStatsResponse { ${envelope('DlqStatData')} }
  type DeleteMediaResponse { ${envelope('Boolean')} }
  type AbortUploadResponse { ${envelope('Boolean')} }
  type UploadSessionResponse { ${envelope('UploadSessionData')} }
  type CompleteUploadResponse { ${envelope('CompleteUploadData')} }
  type DlqReplayResponse { ${envelope('DlqReplayData')} }

  type Query {
    listMedia(ownerId: ID, limit: Int, cursor: String): ListMediaResponse
    media(mediaId: ID!): MediaResponse
    uploadStatus(uploadId: ID!): UploadStatusResponse
    downloadUrl(mediaId: ID!, versionType: String, expirySeconds: Int): DownloadUrlResponse
    quota: QuotaResponse
    dlqTopics: [String]
    dlqStats(topics: [String!]): DlqStatsResponse
  }

  type Mutation {
    deleteMedia(mediaId: ID!, idempotencyKey: String): DeleteMediaResponse
    abortUpload(uploadId: ID!): AbortUploadResponse
    createUploadSession(input: CreateUploadSessionInput!): UploadSessionResponse
    completeUpload(input: CompleteUploadInput!): CompleteUploadResponse
    dlqReplay(topic: String!, maxMessages: Int): DlqReplayResponse
    renewPresigned(input: RenewPresignedInput!): UploadSessionResponse
  }
`;

// ─────────────────────────────────────────────────────────────────────────────
// SEARCH subgraph
// ─────────────────────────────────────────────────────────────────────────────
const SEARCH = `
  ${FED_LINK}

  input SearchPaginationInput {
    limit: Int
    offset: Int
    cursor: String
  }

  input AutocompleteInput {
    prefix: String!
    index: String
    limit: Int
  }

  input SearchMediaInput {
    query: String!
    pagination: SearchPaginationInput
  }

  input SearchUsersInput {
    query: String!
    role: String
    isActive: Boolean
    pagination: SearchPaginationInput
  }

  input SearchDeliveriesInput {
    query: String!
    status: String
    pagination: SearchPaginationInput
  }

  input NearbyDeliveriesInput {
    lat: Float!
    lon: Float!
    radiusKm: Float
    pagination: SearchPaginationInput
  }

  input SearchGeoInput {
    lat: Float!
    lon: Float!
    radiusKm: Float
  }

  input SearchDriversInput {
    query: String!
    geo: SearchGeoInput
    vehicleType: String
    status: String
    minRating: Float
    pagination: SearchPaginationInput
  }

  input SearchNearbyDriversInput {
    lat: Float!
    lon: Float!
    radiusKm: Float
    pagination: SearchPaginationInput
  }

  type SearchPaginationInfo {
    totalItems: Int
    currentPage: Int
    nextPage: Int
  }

  type SearchCoords {
    lat: Float
    lon: Float
  }

  type SearchGeoLocation {
    city: String
    country: String
    location: SearchCoords
  }

  type SearchMediaItem {
    media_id: ID
    file_name: String
    mime_type: String
    owner_id: ID
    size: Float
    status: String
    created_at: String
  }

  type SearchUserItem {
    id: ID
    first_name: String
    last_name: String
    email: String
    is_active: Boolean
    created_at: String
  }

  type SearchDeliveryItem {
    delivery_id: ID
    customer_id: ID
    driver_id: ID
    status: String
    pickup: SearchGeoLocation
    dropoff: SearchGeoLocation
    created_at: String
    updated_at: String
    source_version: String
  }

  type NearbyDeliveryItem {
    delivery_id: ID
    customer_id: ID
    status: String
    pickup: SearchGeoLocation
  }

  type SearchDriverItem {
    driver_id: ID
    name: String
    status: String
    vehicle_type: String
    rating: Float
    location: SearchCoords
    updated_at: String
    source_version: String
  }

  type SearchNearbyDriverItem {
    driver_id: ID
    name: String
    status: String
    vehicle_type: String
    rating: Float
    location: SearchCoords
  }

  type SearchHealthData {
    status: String
    timestamp: String
  }

  type AutocompleteData {
    suggestions: [String]
  }

  type SearchMediaData {
    paginationInfo: SearchPaginationInfo
    items: [SearchMediaItem]
  }

  type SearchUsersData {
    paginationInfo: SearchPaginationInfo
    items: [SearchUserItem]
  }

  type SearchDeliveriesData {
    paginationInfo: SearchPaginationInfo
    items: [SearchDeliveryItem]
  }

  type NearbyDeliveriesData {
    paginationInfo: SearchPaginationInfo
    items: [NearbyDeliveryItem]
  }

  type SearchDriversData {
    paginationInfo: SearchPaginationInfo
    items: [SearchDriverItem]
  }

  type SearchNearbyDriversData {
    paginationInfo: SearchPaginationInfo
    items: [SearchNearbyDriverItem]
  }

  type ReindexData {
    jobId: ID
    index: String
    status: String
    startedAt: String
    completedAt: String
    error: String
  }

  type SearchHealthResponse { ${envelope('SearchHealthData')} }
  type AutocompleteResponse { ${envelope('AutocompleteData')} }
  type SearchMediaResponse { ${envelope('SearchMediaData')} }
  type SearchUsersResponse { ${envelope('SearchUsersData')} }
  type SearchDeliveriesResponse { ${envelope('SearchDeliveriesData')} }
  type NearbyDeliveriesResponse { ${envelope('NearbyDeliveriesData')} }
  type SearchDriversResponse { ${envelope('SearchDriversData')} }
  type SearchNearbyDriversResponse { ${envelope('SearchNearbyDriversData')} }
  type ReindexResponse { ${envelope('ReindexData')} }

  type Query {
    searchHealth: SearchHealthResponse
    autocomplete(input: AutocompleteInput!): AutocompleteResponse
    searchMedia(input: SearchMediaInput!): SearchMediaResponse
    searchUsers(input: SearchUsersInput!): SearchUsersResponse
    searchDeliveries(input: SearchDeliveriesInput!): SearchDeliveriesResponse
    nearbyDeliveries(input: NearbyDeliveriesInput!): NearbyDeliveriesResponse
    searchDrivers(input: SearchDriversInput!): SearchDriversResponse
    searchNearbyDrivers(input: SearchNearbyDriversInput!): SearchNearbyDriversResponse
  }

  type Mutation {
    startReindex(index: String!): ReindexResponse
  }
`;

// ─────────────────────────────────────────────────────────────────────────────
// ANALYTICS subgraph
// ─────────────────────────────────────────────────────────────────────────────
const ANALYTICS = `
  ${FED_LINK}
  ${DRIVER_ENTITY_STUB}

  enum Granularity {
    HOUR
    DAY
    WEEK
    MONTH
  }

  enum PaymentProvider {
    STRIPE
    PAYPAL
    MOCK
  }

  enum AnalyticsEventType {
    DELIVERY_CREATED
    DELIVERY_COMPLETED
    PAYMENT_CAPTURED
    DRIVER_ASSIGNED
  }

  enum DataQualitySeverity {
    INFO
    WARNING
    ERROR
  }

  input AnalyticsRangeInput {
    from: String!
    to: String!
    granularity: Granularity!
  }

  input DeliveryAnalyticsFilterInput {
    range: AnalyticsRangeInput!
    driverId: ID
    status: String
  }

  input DriverAnalyticsFilterInput {
    range: AnalyticsRangeInput!
    driverId: ID!
  }

  input PaymentAnalyticsFilterInput {
    range: AnalyticsRangeInput!
    provider: PaymentProvider
  }

  type ServiceInfoData @shareable {
    name: String
    version: String
    status: String
  }

  type PlatformOverviewData {
    totalDeliveries: Int
    completedDeliveries: Int
    cancelledDeliveries: Int
    failedDeliveries: Int
    completionRate: Float
    averageDeliveryDurationSeconds: Float
    paymentCapturedAmount: Float
    refundedAmount: Float
    driverAcceptanceRate: Float
    dataAsOf: String
  }

  type DeliveryBucket {
    bucket: String
    total: Int
    completed: Int
    cancelled: Int
    failed: Int
    avgDurationSeconds: Float
  }

  type DeliveryAnalyticsData {
    total: Int
    completed: Int
    cancelled: Int
    failed: Int
    completionRate: Float
    averageDurationSeconds: Float
    p50DurationSeconds: Float
    p95DurationSeconds: Float
    p99DurationSeconds: Float
    averageAssignmentTimeSeconds: Float
    buckets: [DeliveryBucket]
    dataAsOf: String
  }

  type DriverAnalyticsData {
    driverId: ID
    driver: Driver
    offers: Int
    accepted: Int
    rejected: Int
    expired: Int
    acceptanceRate: Float
    averageResponseTimeMs: Float
    completedDeliveries: Int
    dataAsOf: String
  }

  type AnalyticsPaginationInfo {
    totalItems: Int
    currentPage: Int
    nextPage: Int
  }

  type TopDriverItem {
    driverId: ID
    driver: Driver
    offers: Int
    accepted: Int
    acceptanceRate: Float
    completedDeliveries: Int
    dataAsOf: String
  }

  type TopDriversData {
    paginationInfo: AnalyticsPaginationInfo
    items: [TopDriverItem]
  }

  type PaymentAnalyticsData {
    authorizationCount: Int
    authorizationSuccessRate: Float
    captureCount: Int
    capturedAmount: Float
    refundCount: Int
    refundedAmount: Float
    refundRate: Float
    averageProviderLatencyMs: Float
    dataAsOf: String
  }

  type RawEventItem {
    eventId: ID
    eventType: String
    eventVersion: String
    aggregateType: String
    aggregateId: ID
    producer: String
    occurredAt: String
    ingestedAt: String
    correlationId: String
    sourceTopic: String
  }

  type RawEventsData {
    paginationInfo: AnalyticsPaginationInfo
    items: [RawEventItem]
  }

  type DataQualityIssueItem {
    issueId: ID
    eventId: ID
    issueType: String
    aggregateType: String
    aggregateId: ID
    detectedAt: String
    severity: String
    details: String
  }

  type DataQualityData {
    paginationInfo: AnalyticsPaginationInfo
    items: [DataQualityIssueItem]
  }

  type AnalyticsServiceInfoResponse { ${envelope('ServiceInfoData')} }
  type PlatformOverviewResponse { ${envelope('PlatformOverviewData')} }
  type DeliveryAnalyticsResponse { ${envelope('DeliveryAnalyticsData')} }
  type DriverAnalyticsResponse { ${envelope('DriverAnalyticsData')} }
  type TopDriversResponse { ${envelope('TopDriversData')} }
  type PaymentAnalyticsResponse { ${envelope('PaymentAnalyticsData')} }
  type RawAnalyticsEventsResponse { ${envelope('RawEventsData')} }
  type DataQualityIssuesResponse { ${envelope('DataQualityData')} }

  type Query {
    analyticsServiceInfo: AnalyticsServiceInfoResponse
    platformOverview(range: AnalyticsRangeInput!): PlatformOverviewResponse
    deliveryAnalytics(filter: DeliveryAnalyticsFilterInput!): DeliveryAnalyticsResponse
    driverAnalytics(filter: DriverAnalyticsFilterInput!): DriverAnalyticsResponse
    topDrivers(range: AnalyticsRangeInput!, limit: Int): TopDriversResponse
    paymentAnalytics(filter: PaymentAnalyticsFilterInput!): PaymentAnalyticsResponse
    rawAnalyticsEvents(page: Int, limit: Int, eventType: AnalyticsEventType): RawAnalyticsEventsResponse
    dataQualityIssues(page: Int, limit: Int, severity: DataQualitySeverity): DataQualityIssuesResponse
  }
`;

/** Subgraph name → Federation v2 SDL. */
export const SUBGRAPH_SDL: Record<string, string> = {
  user: USER,
  delivery: DELIVERY,
  driver: DRIVER,
  payment: PAYMENT,
  notification: NOTIFICATION,
  realtime: REALTIME,
  media: MEDIA,
  search: SEARCH,
  analytics: ANALYTICS,
};

/** The env var each subgraph URL is read from in app.module.ts. */
export const SUBGRAPH_ENV_VAR: Record<string, string> = {
  user: 'USER_SERVICE_URL',
  delivery: 'DELIVERY_SERVICE_URL',
  driver: 'DRIVER_SERVICE_URL',
  payment: 'PAYMENT_SUBGRAPH_URL',
  notification: 'NOTIFICATION_SERVICE_URL',
  realtime: 'REALTIME_SERVICE_URL',
  media: 'MEDIA_SUBGRAPH_URL',
  search: 'SEARCH_SERVICE_URL',
  analytics: 'ANALYTICS_SERVICE_URL',
};

/** Path each subgraph serves GraphQL on (matches gateway defaults). */
export const SUBGRAPH_PATH: Record<string, string> = {
  user: '/user/graphql',
  delivery: '/delivery/graphql',
  driver: '/driver/graphql',
  payment: '/payment/graphql',
  notification: '/notification/graphql',
  realtime: '/realtime/graphql',
  media: '/media/graphql',
  search: '/search/graphql',
  analytics: '/analytics/graphql',
};
