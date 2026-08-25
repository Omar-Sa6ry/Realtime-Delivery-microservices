package graphql

import (
	"fmt"

	gql "github.com/graphql-go/graphql"
)

// subgraphSDL defines the Apollo Federation 2.3 subgraph schema
const subgraphSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])

type GeoPoint {
  lat: Float!
  lon: Float!
}

type GeoAddress {
  city: String!
  country: String!
  location: GeoPoint!
}

type UserDocument {
  id: String!
  first_name: String!
  last_name: String!
  email: String!
  role: String!
  is_active: Boolean!
  created_at: String!
}

type UserSearchResponse {
  items: [UserDocument!]!
  pageInfo: PageInfo!
}

type DeliveryDocument {
  delivery_id: String!
  customer_id: String!
  driver_id: String
  status: String!
  pickup: GeoAddress!
  dropoff: GeoAddress!
  created_at: String!
  updated_at: String!
  source_version: Int!
}

type DeliverySearchResponse {
  items: [DeliveryDocument!]!
  pageInfo: PageInfo!
}

type DriverDocument {
  driver_id: String!
  name: String!
  status: String!
  vehicle_type: String!
  rating: Float!
  location: GeoPoint
  updated_at: String!
  source_version: Int!
}

type DriverSearchResponse {
  items: [DriverDocument!]!
  pageInfo: PageInfo!
}

type MediaDocument {
  media_id: String!
  owner_id: String!
  file_name: String!
  mime_type: String!
  media_type: String!
  status: String!
  size: Int!
  created_at: String!
}

type MediaSearchResponse {
  items: [MediaDocument!]!
  pageInfo: PageInfo!
}

type PageInfo {
  hasNextPage: Boolean!
  cursor: String
  total: Int!
}

type AutocompleteResult {
  suggestions: [String!]!
}

type AutocompleteResponse {
  data: AutocompleteResult!
}

type ReindexJob {
  jobId: String!
  index: String!
  status: String!
  startedAt: String!
  completedAt: String
  error: String
}

input PaginationInput {
  limit: Int
  cursor: String
}

input GeoSearchInput {
  lat: Float!
  lon: Float!
  radiusKm: Float!
  pagination: PaginationInput
}

input DeliverySearchInput {
  query: String
  status: String
  city: String
  driverId: String
  customerId: String
  fromDate: String
  toDate: String
  geo: GeoSearchInput
  sort: [DeliverySearchSortInput]
  pagination: PaginationInput
}

input DeliverySearchSortInput {
  field: String!
  order: String!
}

input DriverSearchInput {
  query: String
  status: String
  vehicleType: String
  minRating: Float
  geo: GeoSearchInput
  sort: [DriverSearchSortInput]
  pagination: PaginationInput
}

input DriverSearchSortInput {
  field: String!
  order: String!
}

input MediaSearchInput {
  query: String
  mediaType: String
  mimeType: String
  ownerId: String
  pagination: PaginationInput
}

input UserSearchInput {
  query: String
  role: String
  isActive: Boolean
  pagination: PaginationInput
}

input AutocompleteInput {
  prefix: String!
  index: String!
  limit: Int
}

type Query {
  _service: _Service!
  searchDeliveries(input: DeliverySearchInput!): DeliverySearchResponse!
  searchDrivers(input: DriverSearchInput!): DriverSearchResponse!
  searchMedia(input: MediaSearchInput!): MediaSearchResponse!
  searchUsers(input: UserSearchInput!): UserSearchResponse!
  autocomplete(input: AutocompleteInput!): AutocompleteResponse!
  nearbyDeliveries(input: GeoSearchInput!): DeliverySearchResponse!
  nearbyDrivers(input: GeoSearchInput!): DriverSearchResponse!
  searchHealth: SearchHealth!
}

type Mutation {
  startReindex(index: String!): ReindexJob!
}

type _Service {
  sdl: String!
}

type SearchHealth {
  status: String!
  timestamp: String!
}`

var geoPointType = gql.NewObject(gql.ObjectConfig{
	Name: "GeoPoint",
	Fields: gql.Fields{
		"lat": &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"lon": &gql.Field{Type: gql.NewNonNull(gql.Float)},
	},
})

var geoAddressType = gql.NewObject(gql.ObjectConfig{
	Name: "GeoAddress",
	Fields: gql.Fields{
		"city":     &gql.Field{Type: gql.NewNonNull(gql.String)},
		"country":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"location": &gql.Field{Type: gql.NewNonNull(geoPointType)},
	},
})

var userDocumentType = gql.NewObject(gql.ObjectConfig{
	Name: "UserDocument",
	Fields: gql.Fields{
		"id":         &gql.Field{Type: gql.NewNonNull(gql.String)},
		"first_name": &gql.Field{Type: gql.NewNonNull(gql.String)},
		"last_name":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"email":      &gql.Field{Type: gql.NewNonNull(gql.String)},
		"role":       &gql.Field{Type: gql.NewNonNull(gql.String)},
		"is_active":  &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"created_at": &gql.Field{Type: gql.NewNonNull(gql.String)},
	},
})

var userSearchResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "UserSearchResponse",
	Fields: gql.Fields{
		"items":    &gql.Field{Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(userDocumentType)))},
		"pageInfo": &gql.Field{Type: gql.NewNonNull(pageInfoType)},
	},
})

var deliveryDocumentType = gql.NewObject(gql.ObjectConfig{
	Name: "DeliveryDocument",
	Fields: gql.Fields{
		"delivery_id":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"customer_id":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"driver_id":      &gql.Field{Type: gql.String},
		"status":         &gql.Field{Type: gql.NewNonNull(gql.String)},
		"pickup":         &gql.Field{Type: gql.NewNonNull(geoAddressType)},
		"dropoff":        &gql.Field{Type: gql.NewNonNull(geoAddressType)},
		"created_at":     &gql.Field{Type: gql.NewNonNull(gql.String)},
		"updated_at":     &gql.Field{Type: gql.NewNonNull(gql.String)},
		"source_version": &gql.Field{Type: gql.NewNonNull(gql.Int)},
	},
})

var deliverySearchResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "DeliverySearchResponse",
	Fields: gql.Fields{
		"items":    &gql.Field{Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(deliveryDocumentType)))},
		"pageInfo": &gql.Field{Type: gql.NewNonNull(pageInfoType)},
	},
})

var driverDocumentType = gql.NewObject(gql.ObjectConfig{
	Name: "DriverDocument",
	Fields: gql.Fields{
		"driver_id":      &gql.Field{Type: gql.NewNonNull(gql.String)},
		"name":           &gql.Field{Type: gql.NewNonNull(gql.String)},
		"status":         &gql.Field{Type: gql.NewNonNull(gql.String)},
		"vehicle_type":   &gql.Field{Type: gql.NewNonNull(gql.String)},
		"rating":         &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"location":       &gql.Field{Type: geoPointType},
		"updated_at":     &gql.Field{Type: gql.NewNonNull(gql.String)},
		"source_version": &gql.Field{Type: gql.NewNonNull(gql.Int)},
	},
})

var driverSearchResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "DriverSearchResponse",
	Fields: gql.Fields{
		"items":    &gql.Field{Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(driverDocumentType)))},
		"pageInfo": &gql.Field{Type: gql.NewNonNull(pageInfoType)},
	},
})

var mediaDocumentType = gql.NewObject(gql.ObjectConfig{
	Name: "MediaDocument",
	Fields: gql.Fields{
		"media_id":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"owner_id":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"file_name":   &gql.Field{Type: gql.NewNonNull(gql.String)},
		"mime_type":   &gql.Field{Type: gql.NewNonNull(gql.String)},
		"media_type":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"status":      &gql.Field{Type: gql.NewNonNull(gql.String)},
		"size":        &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"created_at":  &gql.Field{Type: gql.NewNonNull(gql.String)},
	},
})

var mediaSearchResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "MediaSearchResponse",
	Fields: gql.Fields{
		"items":    &gql.Field{Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(mediaDocumentType)))},
		"pageInfo": &gql.Field{Type: gql.NewNonNull(pageInfoType)},
	},
})

var pageInfoType = gql.NewObject(gql.ObjectConfig{
	Name: "PageInfo",
	Fields: gql.Fields{
		"hasNextPage": &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"cursor":      &gql.Field{Type: gql.String},
		"total":       &gql.Field{Type: gql.NewNonNull(gql.Int)},
	},
})

var autocompleteResultType = gql.NewObject(gql.ObjectConfig{
	Name: "AutocompleteResult",
	Fields: gql.Fields{
		"suggestions": &gql.Field{Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(gql.String)))},
	},
})

var autocompleteResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "AutocompleteResponse",
	Fields: gql.Fields{
		"data": &gql.Field{Type: gql.NewNonNull(autocompleteResultType)},
	},
})

var reindexJobType = gql.NewObject(gql.ObjectConfig{
	Name: "ReindexJob",
	Fields: gql.Fields{
		"jobId":       &gql.Field{Type: gql.NewNonNull(gql.String)},
		"index":       &gql.Field{Type: gql.NewNonNull(gql.String)},
		"status":      &gql.Field{Type: gql.NewNonNull(gql.String)},
		"startedAt":   &gql.Field{Type: gql.NewNonNull(gql.String)},
		"completedAt": &gql.Field{Type: gql.String},
		"error":       &gql.Field{Type: gql.String},
	},
})

var searchHealthType = gql.NewObject(gql.ObjectConfig{
	Name: "SearchHealth",
	Fields: gql.Fields{
		"status":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timestamp": &gql.Field{Type: gql.NewNonNull(gql.String)},
	},
})

var serviceType = gql.NewObject(gql.ObjectConfig{
	Name: "_Service",
	Fields: gql.Fields{
		"sdl": &gql.Field{Type: gql.NewNonNull(gql.String)},
	},
})

var paginationInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "PaginationInput",
	Fields: gql.InputObjectConfigFieldMap{
		"limit":  &gql.InputObjectFieldConfig{Type: gql.Int},
		"cursor": &gql.InputObjectFieldConfig{Type: gql.String},
	},
})

var geoSearchInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "GeoSearchInput",
	Fields: gql.InputObjectConfigFieldMap{
		"lat":       &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.Float)},
		"lon":       &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.Float)},
		"radiusKm":  &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.Float)},
		"pagination": &gql.InputObjectFieldConfig{Type: paginationInput},
	},
})

var deliverySearchSortInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "DeliverySearchSortInput",
	Fields: gql.InputObjectConfigFieldMap{
		"field": &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
		"order": &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
	},
})

var deliverySearchInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "DeliverySearchInput",
	Fields: gql.InputObjectConfigFieldMap{
		"query":      &gql.InputObjectFieldConfig{Type: gql.String},
		"status":     &gql.InputObjectFieldConfig{Type: gql.String},
		"city":       &gql.InputObjectFieldConfig{Type: gql.String},
		"driverId":   &gql.InputObjectFieldConfig{Type: gql.String},
		"customerId": &gql.InputObjectFieldConfig{Type: gql.String},
		"fromDate":   &gql.InputObjectFieldConfig{Type: gql.String},
		"toDate":     &gql.InputObjectFieldConfig{Type: gql.String},
		"geo":        &gql.InputObjectFieldConfig{Type: geoSearchInput},
		"sort":       &gql.InputObjectFieldConfig{Type: gql.NewList(deliverySearchSortInput)},
		"pagination": &gql.InputObjectFieldConfig{Type: paginationInput},
	},
})

var driverSearchSortInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "DriverSearchSortInput",
	Fields: gql.InputObjectConfigFieldMap{
		"field": &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
		"order": &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
	},
})

var driverSearchInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "DriverSearchInput",
	Fields: gql.InputObjectConfigFieldMap{
		"query":       &gql.InputObjectFieldConfig{Type: gql.String},
		"status":      &gql.InputObjectFieldConfig{Type: gql.String},
		"vehicleType": &gql.InputObjectFieldConfig{Type: gql.String},
		"minRating":   &gql.InputObjectFieldConfig{Type: gql.Float},
		"geo":         &gql.InputObjectFieldConfig{Type: geoSearchInput},
		"sort":        &gql.InputObjectFieldConfig{Type: gql.NewList(driverSearchSortInput)},
		"pagination":  &gql.InputObjectFieldConfig{Type: paginationInput},
	},
})

var mediaSearchInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "MediaSearchInput",
	Fields: gql.InputObjectConfigFieldMap{
		"query":      &gql.InputObjectFieldConfig{Type: gql.String},
		"mediaType":  &gql.InputObjectFieldConfig{Type: gql.String},
		"mimeType":   &gql.InputObjectFieldConfig{Type: gql.String},
		"ownerId":    &gql.InputObjectFieldConfig{Type: gql.String},
		"pagination": &gql.InputObjectFieldConfig{Type: paginationInput},
	},
})

var userSearchInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "UserSearchInput",
	Fields: gql.InputObjectConfigFieldMap{
		"query":      &gql.InputObjectFieldConfig{Type: gql.String},
		"role":       &gql.InputObjectFieldConfig{Type: gql.String},
		"isActive":   &gql.InputObjectFieldConfig{Type: gql.Boolean},
		"pagination": &gql.InputObjectFieldConfig{Type: paginationInput},
	},
})

var autocompleteInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "AutocompleteInput",
	Fields: gql.InputObjectConfigFieldMap{
		"prefix":   &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
		"index":    &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
		"limit":    &gql.InputObjectFieldConfig{Type: gql.Int},
	},
})

func buildSchema(r *Resolver) (gql.Schema, error) {
	query := gql.NewObject(gql.ObjectConfig{
		Name: "Query",
		Fields: gql.Fields{
			"_service": &gql.Field{
				Type: gql.NewNonNull(serviceType),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					return map[string]interface{}{"sdl": subgraphSDL}, nil
				},
			},
			"searchDeliveries": &gql.Field{
				Type: gql.NewNonNull(deliverySearchResponseType),
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(deliverySearchInput)},
				},
				Resolve: r.ResolveSearchDeliveries,
			},
			"searchDrivers": &gql.Field{
				Type: gql.NewNonNull(driverSearchResponseType),
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(driverSearchInput)},
				},
				Resolve: r.ResolveSearchDrivers,
			},
			"searchMedia": &gql.Field{
				Type: gql.NewNonNull(mediaSearchResponseType),
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(mediaSearchInput)},
				},
				Resolve: r.ResolveSearchMedia,
			},
			"searchUsers": &gql.Field{
				Type: gql.NewNonNull(userSearchResponseType),
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(userSearchInput)},
				},
				Resolve: r.ResolveSearchUsers,
			},
			"autocomplete": &gql.Field{
				Type: gql.NewNonNull(autocompleteResponseType),
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(autocompleteInput)},
				},
				Resolve: r.ResolveAutocomplete,
			},
			"nearbyDeliveries": &gql.Field{
				Type: gql.NewNonNull(deliverySearchResponseType),
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(geoSearchInput)},
				},
				Resolve: r.ResolveNearbyDeliveries,
			},
			"nearbyDrivers": &gql.Field{
				Type: gql.NewNonNull(driverSearchResponseType),
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(geoSearchInput)},
				},
				Resolve: r.ResolveNearbyDrivers,
			},
			"searchHealth": &gql.Field{
				Type: gql.NewNonNull(searchHealthType),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					return map[string]interface{}{
						"status":    "UP",
						"timestamp": p.Context.Value("timestamp"),
					}, nil
				},
			},
		},
	})

	mutation := gql.NewObject(gql.ObjectConfig{
		Name: "Mutation",
		Fields: gql.Fields{
			"startReindex": &gql.Field{
				Type: gql.NewNonNull(reindexJobType),
				Args: gql.FieldConfigArgument{
					"index": &gql.ArgumentConfig{Type: gql.NewNonNull(gql.String)},
				},
				Resolve: r.ResolveStartReindex,
			},
		},
	})

	schema, err := gql.NewSchema(gql.SchemaConfig{Query: query, Mutation: mutation})
	if err != nil {
		return gql.Schema{}, fmt.Errorf("new schema: %w", err)
	}
	return schema, nil
}