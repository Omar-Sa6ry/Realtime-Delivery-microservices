package graphql

// DriverSubgraphSDL is the GraphQL SDL for the driver subgraph (Apollo Federation v2).
const DriverSubgraphSDL = `directive @key(fields: String!) repeatable on OBJECT | INTERFACE
directive @shareable on OBJECT | FIELD_DEFINITION
directive @external on FIELD_DEFINITION

extend schema
  @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@external"])

enum VehicleType {
  CAR
  MOTORCYCLE
  TRUCK
  BICYCLE
  VAN
}

type User @key(fields: "id") {
  id: ID!
}

type Delivery @key(fields: "id") {
  id: ID!
}

type Driver @key(fields: "id") {
  id: ID!
  userId: String!
  user: User
  status: String!
  vehicleType: VehicleType
  plateNumber: String
  capacityKg: Int
  capabilities: [String!]
  serviceArea: String
  rating: Float
  isBlocked: Boolean!
  createdAt: String
  updatedAt: String
}

type Assignment {
  id: ID!
  deliveryId: String!
  driverId: String!
  delivery: Delivery!
  driver: Driver!
  status: String!
  attemptNumber: Int
  offeredAt: String
  expiresAt: String
  acceptedAt: String
  rejectedAt: String
  completedAt: String
  createdAt: String
  updatedAt: String
}

type DriverStatus {
  driver: Driver!
  status: String!
  hasActiveAssignment: Boolean!
  activeDelivery: Delivery
  lastSeenAt: String
}

type NearbyDriverItem {
  driverId: String!
  driver: Driver
  distanceMeters: Float!
  status: String!
  vehicleType: VehicleType
  latitude: Float!
  longitude: Float!
  rating: Float!
  isBlocked: Boolean!
}

type NearbyDriversResult {
  items: [NearbyDriverItem!]!
  total: Int!
}

type DispatchAttemptItem {
  id: ID!
  deliveryId: String!
  driverId: String!
  delivery: Delivery!
  driver: Driver!
  distanceMeters: Float!
  attemptNumber: Int!
  result: String!
  reason: String
  createdAt: String!
}

type DispatchAttemptsData {
  paginationInfo: PaginationInfo!
  items: [DispatchAttemptItem!]!
}

type PaginationInfo @shareable {
  totalItems: Int!
  currentPage: Int!
  nextPage: Int
}

input NearbyDriversInput {
  latitude: Float!
  longitude: Float!
  radiusKm: Float!
  vehicleType: VehicleType
  limit: Int
}

input RegisterDriverInput {
  userId: String!
  vehicleType: VehicleType!
  plateNumber: String!
  capacityKg: Int!
  capabilities: [String!]
  serviceArea: String
}

input UpdateDriverProfileInput {
  driverId: ID!
  vehicleType: VehicleType
  plateNumber: String
  capacityKg: Int
	serviceArea: String
}

input RateDriverInput {
  driverId: ID!
  deliveryId: ID!
  rating: Float!
  comment: String
}

type DriverResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: Driver
}

type AssignmentResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: Assignment
}

type DriverStatusResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: DriverStatus
}

type NearbyDriversData {
  paginationInfo: PaginationInfo!
  items: [NearbyDriverItem!]!
}

type NearbyDriversResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: NearbyDriversData
}

type DispatchAttemptsResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: DispatchAttemptsData
}

type DriverLocationUpdatedResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  data: Driver
}

type Review {
  id: ID!
  driverId: String!
  userId: String!
  deliveryId: String!
  driver: Driver!
  user: User!
  delivery: Delivery!
  rating: Float!
  comment: String
  createdAt: String!
}

type ReviewResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: Review
}

type DriverReviewsData {
  paginationInfo: PaginationInfo!
  items: [Review!]!
  averageRating: Float!
  totalReviews: Int!
}

type DriverReviewsResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: DriverReviewsData
}

type Query {
  _service: _Service!
  driverServiceInfo: DriverServiceInfoResponse
  driver(id: ID!): DriverResponse
  myDriverProfile: DriverResponse
  driverActiveAssignment(driverId: ID): AssignmentResponse
  driverStatus(driverId: ID!): DriverStatusResponse
  nearbyDrivers(input: NearbyDriversInput!): NearbyDriversResponse
  assignment(id: ID!): AssignmentResponse
  dispatchAttempts(deliveryId: ID!): DispatchAttemptsResponse
  driverReviews(driverId: ID!, page: Int, limit: Int): DriverReviewsResponse
}

type Mutation {
  goOnline(idempotencyKey: String!): DriverResponse
  goOffline(idempotencyKey: String!): DriverResponse
  acceptAssignment(assignmentId: ID!, idempotencyKey: String!): AssignmentResponse
  rejectAssignment(assignmentId: ID!, reason: String, idempotencyKey: String!): AssignmentResponse
  registerDriver(input: RegisterDriverInput!): DriverResponse
  updateDriverProfile(input: UpdateDriverProfileInput!): DriverResponse
  suspendDriver(driverId: ID!, reason: String!): DriverResponse
  activateDriver(driverId: ID!): DriverResponse
  blockDriver(driverId: ID!, reason: String): DriverResponse
  unblockDriver(driverId: ID!, reason: String): DriverResponse
  rateDriver(input: RateDriverInput!): ReviewResponse
}

type _Service {
  sdl: String!
}

type DriverServiceInfoResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: DriverServiceInfo
}

type DriverServiceInfo {
  name: String!
  version: String!
  status: String!
}`
