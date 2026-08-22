package graphql

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/reindex"
	appSearch "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
)

type Server struct {
	searchService  *appSearch.Service
	reindexService *reindex.Service
}

func NewServer(searchService *appSearch.Service, reindexService *reindex.Service) *Server {
	return &Server{
		searchService:  searchService,
		reindexService: reindexService,
	}
}

// Handler handles incoming GraphQL queries and mutations
func (s *Server) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Return schema or simple federation metadata / playground
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"service": "search-service",
				"status":  "healthy",
				"graphql": "Federation Subgraph active on POST /search/graphql",
			})
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Query         string                 `json:"query"`
			OperationName string                 `json:"operationName"`
			Variables     map[string]interface{} `json:"variables"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		res := s.executeGraphQL(ctx, req.Query, req.Variables)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	}
}

// GeneralResponse models matching standard schema:
type GraphQLGeneralResponse struct {
	Success    bool        `json:"success"`
	StatusCode int         `json:"statusCode"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data,omitempty"`
	Items      interface{} `json:"items,omitempty"`
	PageInfo   interface{} `json:"pageInfo,omitempty"`
}

type GraphQLResponseBody struct {
	Data   map[string]interface{}   `json:"data,omitempty"`
	Errors []map[string]interface{} `json:"errors,omitempty"`
}

func (s *Server) executeGraphQL(ctx context.Context, query string, vars map[string]interface{}) GraphQLResponseBody {
	data := make(map[string]interface{})

	// Handle standard query routing
	if contains(query, "searchDeliveries") {
		var input search.DeliverySearchQuery
		if raw, ok := vars["input"].(map[string]interface{}); ok {
			b, _ := json.Marshal(raw)
			_ = json.Unmarshal(b, &input)
		}
		res, err := s.searchService.SearchDeliveries(ctx, input)
		if err != nil {
			return errorResponse(err)
		}
		data["searchDeliveries"] = GraphQLGeneralResponse{
			Success:    true,
			StatusCode: 200,
			Message:    "Deliveries fetched successfully",
			Items:      res.Items,
			PageInfo:   res.PageInfo,
		}
	} else if contains(query, "searchDrivers") {
		var input search.DriverSearchQuery
		if raw, ok := vars["input"].(map[string]interface{}); ok {
			b, _ := json.Marshal(raw)
			_ = json.Unmarshal(b, &input)
		}
		res, err := s.searchService.SearchDrivers(ctx, input)
		if err != nil {
			return errorResponse(err)
		}
		data["searchDrivers"] = GraphQLGeneralResponse{
			Success:    true,
			StatusCode: 200,
			Message:    "Drivers fetched successfully",
			Items:      res.Items,
			PageInfo:   res.PageInfo,
		}
	} else if contains(query, "searchMedia") {
		var input search.MediaSearchQuery
		if raw, ok := vars["input"].(map[string]interface{}); ok {
			b, _ := json.Marshal(raw)
			_ = json.Unmarshal(b, &input)
		}
		res, err := s.searchService.SearchMedia(ctx, input)
		if err != nil {
			return errorResponse(err)
		}
		data["searchMedia"] = GraphQLGeneralResponse{
			Success:    true,
			StatusCode: 200,
			Message:    "Media fetched successfully",
			Items:      res.Items,
			PageInfo:   res.PageInfo,
		}
	} else if contains(query, "autocomplete") {
		var input search.AutocompleteQuery
		if raw, ok := vars["input"].(map[string]interface{}); ok {
			b, _ := json.Marshal(raw)
			_ = json.Unmarshal(b, &input)
		}
		res, err := s.searchService.Autocomplete(ctx, input)
		if err != nil {
			return errorResponse(err)
		}
		data["autocomplete"] = GraphQLGeneralResponse{
			Success:    true,
			StatusCode: 200,
			Message:    "Suggestions fetched successfully",
			Data:       res,
		}
	} else if contains(query, "nearbyDeliveries") {
		var input search.GeoSearchQuery
		if raw, ok := vars["input"].(map[string]interface{}); ok {
			b, _ := json.Marshal(raw)
			_ = json.Unmarshal(b, &input)
		}
		res, err := s.searchService.NearbyDeliveries(ctx, input)
		if err != nil {
			return errorResponse(err)
		}
		data["nearbyDeliveries"] = GraphQLGeneralResponse{
			Success:    true,
			StatusCode: 200,
			Message:    "Nearby deliveries fetched successfully",
			Items:      res.Items,
			PageInfo:   res.PageInfo,
		}
	} else if contains(query, "nearbyDrivers") {
		var input search.GeoSearchQuery
		if raw, ok := vars["input"].(map[string]interface{}); ok {
			b, _ := json.Marshal(raw)
			_ = json.Unmarshal(b, &input)
		}
		res, err := s.searchService.NearbyDrivers(ctx, input)
		if err != nil {
			return errorResponse(err)
		}
		data["nearbyDrivers"] = GraphQLGeneralResponse{
			Success:    true,
			StatusCode: 200,
			Message:    "Nearby drivers fetched successfully",
			Items:      res.Items,
			PageInfo:   res.PageInfo,
		}
	} else if contains(query, "startReindex") {
		indexName := "deliveries"
		if idx, ok := vars["index"].(string); ok && idx != "" {
			indexName = idx
		}
		job, err := s.reindexService.StartReindex(ctx, indexName)
		if err != nil {
			return errorResponse(err)
		}
		data["startReindex"] = GraphQLGeneralResponse{
			Success:    true,
			StatusCode: 200,
			Message:    "Reindex job started",
			Data:       job,
		}
	} else if contains(query, "_service") {
		// Apollo Federation Subgraph SDL query
		data["_service"] = map[string]string{
			"sdl": FederationSDL,
		}
	} else if contains(query, "__typename") {
		data["__typename"] = "Query"
	} else {
		data["searchHealth"] = map[string]interface{}{
			"status":    "UP",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
	}

	return GraphQLResponseBody{Data: data}
}

func errorResponse(err error) GraphQLResponseBody {
	return GraphQLResponseBody{
		Errors: []map[string]interface{}{
			{
				"message": err.Error(),
			},
		},
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstr(s, substr))
}

func findSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

const FederationSDL = `
extend schema @link(url: "https://specs.apollo.dev/federation/v2.0", import: ["@key", "@shareable"])

type GeoPoint {
  lat: Float!
  lon: Float!
}

type GeoAddress {
  city: String!
  country: String!
  location: GeoPoint!
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

type PageInfo {
  hasNextPage: Boolean!
  cursor: String
  total: Int!
}

type AutocompleteResult {
  suggestions: [String!]!
}

type DeliverySearchResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  items: [DeliveryDocument!]
  pageInfo: PageInfo
}

type DriverSearchResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  items: [DriverDocument!]
  pageInfo: PageInfo
}

type MediaSearchResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  items: [MediaDocument!]
  pageInfo: PageInfo
}

type AutocompleteResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  data: AutocompleteResult
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
  pagination: PaginationInput
}

input DriverSearchInput {
  query: String
  status: String
  vehicleType: String
  minRating: Float
  pagination: PaginationInput
}

input MediaSearchInput {
  query: String
  mediaType: String
  mimeType: String
  pagination: PaginationInput
}

input AutocompleteInput {
  prefix: String!
  index: String!
  limit: Int
}

type Query {
  searchDeliveries(input: DeliverySearchInput!): DeliverySearchResponse!
  searchDrivers(input: DriverSearchInput!): DriverSearchResponse!
  searchMedia(input: MediaSearchInput!): MediaSearchResponse!
  autocomplete(input: AutocompleteInput!): AutocompleteResponse!
  nearbyDeliveries(input: GeoSearchInput!): DeliverySearchResponse!
  nearbyDrivers(input: GeoSearchInput!): DriverSearchResponse!
}

type Mutation {
  startReindex(index: String!): Boolean!
}
`
