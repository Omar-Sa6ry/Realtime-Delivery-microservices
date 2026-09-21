package graphql

const AnalyticsSubgraphSDL = `directive @key(fields: String!) repeatable on OBJECT | INTERFACE
directive @shareable on OBJECT | FIELD_DEFINITION
directive @external on FIELD_DEFINITION

extend schema
	@link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@external"])

scalar DateTime
scalar Decimal
scalar Long

enum AnalyticsGranularity {
	HOUR
	DAY
	WEEK
	MONTH
}

type User @key(fields: "id") {
	id: ID!
}

type Delivery @key(fields: "id") {
	id: ID!
}

type Driver @key(fields: "id") {
	id: ID!
}

type AnalyticsServiceInfo @shareable {
	name: String!
	version: String!
	status: String!
}

type AnalyticsServiceInfoResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: AnalyticsServiceInfo
}

type PaginationInfo @shareable {
	totalItems: Int!
	currentPage: Int!
	nextPage: Int
}

input AnalyticsRange {
	from: DateTime!
	to: DateTime!
	granularity: AnalyticsGranularity!
}

input DeliveryAnalyticsFilter {
	range: AnalyticsRange!
	cityId: String
	driverId: ID
}

input DriverAnalyticsFilter {
	range: AnalyticsRange!
	driverId: ID
}

input PaymentAnalyticsFilter {
	range: AnalyticsRange!
	provider: String
}

type PlatformOverview {
	totalDeliveries: Long!
	completedDeliveries: Long!
	cancelledDeliveries: Long!
	failedDeliveries: Long!
	completionRate: Float!
	averageDeliveryDurationSeconds: Float!
	paymentCapturedAmount: Decimal!
	refundedAmount: Decimal!
	driverAcceptanceRate: Float!
	dataAsOf: DateTime!
}

type DeliveryMetricBucket {
	bucket: DateTime!
	total: Long!
	completed: Long!
	cancelled: Long!
	failed: Long!
	avgDurationSeconds: Float!
}

type DeliveryAnalytics {
	total: Long!
	completed: Long!
	cancelled: Long!
	failed: Long!
	completionRate: Float!
	averageDurationSeconds: Float!
	p50DurationSeconds: Float!
	p95DurationSeconds: Float!
	p99DurationSeconds: Float!
	averageAssignmentTimeSeconds: Float!
	buckets: [DeliveryMetricBucket!]!
	dataAsOf: DateTime!
}

type DriverAnalytics {
	driverId: ID
	driver: Driver
	offers: Long!
	accepted: Long!
	rejected: Long!
	expired: Long!
	acceptanceRate: Float!
	averageResponseTimeMs: Float!
	completedDeliveries: Long!
	dataAsOf: DateTime!
}

type DriverAnalyticsListData {
	paginationInfo: PaginationInfo!
	items: [DriverAnalytics!]!
}

type PaymentAnalytics {
	authorizationCount: Long!
	authorizationSuccessRate: Float!
	captureCount: Long!
	capturedAmount: Decimal!
	refundCount: Long!
	refundedAmount: Decimal!
	refundRate: Float!
	averageProviderLatencyMs: Float!
	dataAsOf: DateTime!
}

type RawAnalyticsEvent {
	eventId: ID!
	eventType: String!
	eventVersion: Int!
	aggregateType: String!
	aggregateId: String!
	producer: String!
	occurredAt: DateTime!
	ingestedAt: DateTime!
	correlationId: String
	sourceTopic: String!
}

type RawAnalyticsEventsData {
	paginationInfo: PaginationInfo!
	items: [RawAnalyticsEvent!]!
}

type DataQualityIssue {
	issueId: ID!
	eventId: String!
	issueType: String!
	aggregateType: String!
	aggregateId: String!
	detectedAt: DateTime!
	severity: String!
	details: String
}

type DataQualityIssuesData {
	paginationInfo: PaginationInfo!
	items: [DataQualityIssue!]!
}

type PlatformOverviewResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: PlatformOverview
}

type DeliveryAnalyticsResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: DeliveryAnalytics
}

type DriverAnalyticsResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: DriverAnalytics
}

type DriverAnalyticsListResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: DriverAnalyticsListData
}

type PaymentAnalyticsResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: PaymentAnalytics
}

type RawAnalyticsEventsResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: RawAnalyticsEventsData
}

type DataQualityIssuesResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: DataQualityIssuesData
}

type Query {
	_service: _Service!
	analyticsServiceInfo: AnalyticsServiceInfoResponse!
	platformOverview(range: AnalyticsRange!): PlatformOverviewResponse!
	deliveryAnalytics(filter: DeliveryAnalyticsFilter!): DeliveryAnalyticsResponse!
	driverAnalytics(filter: DriverAnalyticsFilter!): DriverAnalyticsResponse!
	topDrivers(range: AnalyticsRange!, limit: Int): DriverAnalyticsListResponse!
	paymentAnalytics(filter: PaymentAnalyticsFilter!): PaymentAnalyticsResponse!
	rawAnalyticsEvents(page: Int, limit: Int, eventType: String, from: DateTime, to: DateTime): RawAnalyticsEventsResponse!
	dataQualityIssues(page: Int, limit: Int, severity: String): DataQualityIssuesResponse!
}

type _Service {
	sdl: String!
}
`
