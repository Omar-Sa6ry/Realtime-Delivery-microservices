package graphql

// DriverSubgraphSDL is the GraphQL SDL for the driver subgraph (Apollo Federation v2).
const DriverSubgraphSDL = `extend schema
  @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])

type Driver @key(fields: "id") {
  id: ID!
  userId: String!
  status: String!
  vehicleType: String
  plateNumber: String
  capacityKg: Int
  capabilities: [String!]
  serviceArea: String
  rating: Float
  createdAt: String
  updatedAt: String
}

type Assignment {
  id: ID!
  deliveryId: String!
  driverId: String!
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
  driverId: String!
  status: String!
  hasActiveAssignment: Boolean!
  activeDeliveryId: String
  lastSeenAt: String
}

type NearbyDriverItem {
  driverId: String!
  distanceMeters: Float!
  status: String!
  vehicleType: String
  latitude: Float!
  longitude: Float!
}

type NearbyDriversResult {
  items: [NearbyDriverItem!]!
  total: Int!
}

type DispatchAttemptItem {
  id: ID!
  deliveryId: String!
  driverId: String!
  distanceMeters: Float!
  attemptNumber: Int!
  result: String!
  reason: String
  createdAt: String!
}

type DispatchAttemptsResult {
  items: [DispatchAttemptItem!]!
  total: Int!
}

type NearbyDriversInput {
  latitude: Float!
  longitude: Float!
  radiusKm: Float!
  vehicleType: String
  limit: Int
}

input RegisterDriverInput {
  userId: String!
  vehicleType: String!
  plateNumber: String!
  capacityKg: Int!
  capabilities: [String!]
  serviceArea: String
}

input UpdateDriverProfileInput {
  vehicleType: String
  plateNumber: String
  capacityKg: Int
  capabilities: [String!]
  serviceArea: String
}

type Query {
  _service: _Service!
  driverServiceInfo: DriverServiceInfoResponse
  driver(id: ID!): Driver
  myDriverProfile: Driver
  driverActiveAssignment(driverId: ID!): Assignment
  driverStatus(driverId: ID!): DriverStatus
  nearbyDrivers(input: NearbyDriversInput!): NearbyDriversResult
  assignment(id: ID!): Assignment
  dispatchAttempts(deliveryId: ID!): DispatchAttemptsResult
}

type Mutation {
  goOnline(idempotencyKey: String!): Driver
  goOffline(idempotencyKey: String!): Driver
  acceptAssignment(assignmentId: ID!, idempotencyKey: String!): Assignment
  rejectAssignment(assignmentId: ID!, reason: String, idempotencyKey: String!): Assignment
  registerDriver(input: RegisterDriverInput!): Driver
  updateDriverProfile(driverId: ID!, input: UpdateDriverProfileInput!): Driver
  suspendDriver(driverId: ID!, reason: String!): Driver
  activateDriver(driverId: ID!): Driver
}

type Subscription {
  driverLocationUpdated: Driver
  driverAssignmentOffered(assignmentId: ID!): Assignment
  driverAssignmentAccepted(assignmentId: ID!): Assignment
  driverStatusUpdated(driverId: ID!): DriverStatus
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
