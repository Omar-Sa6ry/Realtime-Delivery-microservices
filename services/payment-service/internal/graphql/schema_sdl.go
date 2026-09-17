package graphql

const PaymentSubgraphSDL = `directive @key(fields: String!) repeatable on OBJECT | INTERFACE
directive @shareable on OBJECT | FIELD_DEFINITION
directive @external on FIELD_DEFINITION

extend schema
	@link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@external"])

enum Currency {
	EGP
	USD
	EUR
	SAR
	AED
}

enum PaymentStatus {
	PENDING
	AUTHORIZED
	CAPTURED
	CANCELLED
	FAILED
	REFUNDED
}

enum RefundStatus {
	REFUND_PENDING
	REFUNDED
	REFUND_FAILED
}

type User @key(fields: "id") {
	id: ID!
}

type Delivery @key(fields: "id") {
	id: ID!
}

type PaymentServiceInfo @shareable {
	name: String!
	version: String!
	status: String!
}

type PaymentServiceInfoResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: PaymentServiceInfo
}

type PaginationInfo @shareable {
	totalItems: Int!
	currentPage: Int!
	nextPage: Int
}

type Payment @key(fields: "id") {
	id: ID!
	deliveryId: String!
	userId: String!
	delivery: Delivery
	user: User
	amountMinor: Int!
	currency: Currency!
	status: PaymentStatus!
	provider: String!
	authorizedAmountMinor: Int
	capturedAmountMinor: Int
	refundedAmountMinor: Int
	clientSecret: String
	checkoutUrl: String
	correlationId: String
	causationId: String
	createdAt: String!
	updatedAt: String!
	authorizedAt: String
	capturedAt: String
	cancelledAt: String
	failedAt: String
}

type Refund {
	id: ID!
	paymentId: String!
	deliveryId: String!
	amountMinor: Int!
	currency: Currency!
	status: RefundStatus!
	reason: String!
	providerRefundId: String
	correlationId: String
	causationId: String
	createdAt: String!
	updatedAt: String!
}

type PaymentResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: Payment
}

type PaymentListData {
	paginationInfo: PaginationInfo!
	items: [Payment!]!
}

type PaymentListResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: PaymentListData
}

type RefundResponse {
	success: Boolean!
	statusCode: Int!
	message: String!
	timeStamp: String!
	data: Refund
}

input CreatePaymentInput {
	deliveryId: String!
	amountMinor: Int!
	currency: Currency!
}

input AuthorizePaymentInput {
	paymentId: ID!
	correlationId: String
	causationId: String
}

input CapturePaymentInput {
	paymentId: ID!
	amountMinor: Int!
	correlationId: String
	causationId: String
}

input CancelPaymentInput {
	paymentId: ID!
	correlationId: String
}

input CreateRefundInput {
	paymentId: ID!
	amountMinor: Int!
	reason: String!
	correlationId: String
	causationId: String
}

type Query {
	_service: _Service!
	paymentServiceInfo: PaymentServiceInfoResponse!
	payment(id: ID!): PaymentResponse
	payments(page: Int, limit: Int, userId: String): PaymentListResponse
}

type Mutation {
	createPayment(input: CreatePaymentInput!): PaymentResponse
	authorizePayment(input: AuthorizePaymentInput!): PaymentResponse
	capturePayment(input: CapturePaymentInput!): PaymentResponse
	cancelAuthorization(input: CancelPaymentInput!): PaymentResponse
	createRefund(input: CreateRefundInput!): RefundResponse
}

type _Service {
	sdl: String!
}
`
