package graphql

const AnalyticsSubgraphSDL = `directive @key(fields: String!) repeatable on OBJECT | INTERFACE
directive @shareable on OBJECT | FIELD_DEFINITION
directive @external on FIELD_DEFINITION

extend schema
	@link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@external"])

scalar DateTime
scalar Decimal
scalar Long

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

type Query {
	_service: _Service!
	analyticsServiceInfo: AnalyticsServiceInfoResponse!
}

type _Service {
	sdl: String!
}
`
