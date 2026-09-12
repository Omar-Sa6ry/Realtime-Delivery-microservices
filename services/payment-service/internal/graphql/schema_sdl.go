package graphql

// Payment subgraph Schema Definition Language
// This file defines the GraphQL schema for the payment service subgraph.

const PaymentSubgraphSDL = `
	type PaymentServiceInfo {
		success: Boolean!
		statusCode: Int!
		message: String!
		timeStamp: String!
		data: ServiceData!
	}

	type ServiceData {
		name: String!
		version: String!
		status: String!
	}

	type Payment {
		id: ID!
		deliveryId: String!
		userId: String!
		amountMinor: Int!
		currency: String!
		status: String!
		correlationId: String
		causationId: String
		createdAt: String
	}

	type PaymentCreatedPayload {
		id: ID!
		status: String!
		correlationId: String
		causationId: String
		createdAt: String
	}

	type PaymentAuthorizedPayload {
		id: ID!
		authorizationId: String!
		status: String!
		correlationId: String
		causationId: String
		createdAt: String
	}

	type PaymentCapturedPayload {
		id: ID!
		captureId: String!
		status: String!
		correlationId: String
		causationId: String
		createdAt: String
	}

	type PaymentCancelledPayload {
		id: ID!
		status: String!
		correlationId: String
		causationId: String
		createdAt: String
	}

	type PaymentRefundStartedPayload {
		id: ID!
		status: String!
		correlationId: String
		causationId: String
		createdAt: String
	}

	type PaymentRefundedPayload {
		id: ID!
		refundId: String!
		status: String!
		correlationId: String
		causationId: String
		createdAt: String
	}

	type PaymentRefundFailedPayload {
		id: ID!
		status: String!
		error: String
		correlationId: String
		causationId: String
		createdAt: String
	}

	type PaymentFailedPayload {
		id: ID!
		status: String!
		error: String
		correlationId: String
		causationId: String
		createdAt: String
	}

	input PaymentCreatedInput {
		deliveryId: String!
		userId: String!
		amountMinor: Int!
		currency: String!
		correlationId: String
		causationId: String
	}

	input PaymentAuthorizationInput {
		paymentId: String!
		correlationId: String
		causationId: String
	}

	input PaymentCaptureInput {
		paymentId: String!
		amountMinor: Int!
		correlationId: String
		causationId: String
	}

	input PaymentCancelInput {
		paymentId: String!
		correlationId: String
	}

	input PaymentRefundInput {
		paymentId: String!
		amountMinor: Int!
		reason: String!
		correlationId: String
		causationId: String
	}

	type Query {
		paymentServiceInfo: PaymentServiceInfo!
		payment(id: ID!): Payment
	}

	type Mutation {
		createPayment(input: PaymentCreatedInput): PaymentCreatedPayload
		authorizePayment(input: PaymentAuthorizationInput): PaymentAuthorizedPayload
		capturePayment(input: PaymentCaptureInput): PaymentCapturedPayload
		cancelAuthorization(input: PaymentCancelInput): PaymentCancelledPayload
		createRefund(input: PaymentRefundInput): PaymentRefundedPayload
	}

	schema {
		query: Query
	}
`
`