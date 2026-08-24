package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/auth"
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
		if contains(req.Query, "searchUsers") {
			roleHeader := r.Header.Get("x-user-role")
			authHeader := r.Header.Get("Authorization")
			if !strings.EqualFold(roleHeader, "admin") {
				if _, err := auth.RequirePermission(authHeader, auth.PermissionViewUser); err != nil {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(errorResponseWithStatus(http.StatusUnauthorized, err))
					return
				}
			}
		}
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
	if contains(query, "searchUsers") {
		var input search.UserSearchQuery
		parseGraphQLInput(query, vars, "searchUsers", &input)
		res, err := s.searchService.SearchUsers(ctx, input)
		if err != nil {
			return errorResponse(err)
		}
		data["searchUsers"] = GraphQLGeneralResponse{Success: true, StatusCode: 200, Message: "Users fetched successfully", Items: res.Items, PageInfo: res.PageInfo}
	} else if contains(query, "searchDeliveries") {
		var input search.DeliverySearchQuery
		parseGraphQLInput(query, vars, "searchDeliveries", &input)
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
		parseGraphQLInput(query, vars, "searchDrivers", &input)
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
		parseGraphQLInput(query, vars, "searchMedia", &input)
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
		parseGraphQLInput(query, vars, "autocomplete", &input)
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
		parseGraphQLInput(query, vars, "nearbyDeliveries", &input)
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
		parseGraphQLInput(query, vars, "nearbyDrivers", &input)
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
		if vars != nil {
			if idx, ok := vars["index"].(string); ok && idx != "" {
				indexName = idx
			}
		}
		if indexName == "deliveries" {
			cleanQuery := stripGraphQLComments(query)
			if idx := extractInlineScalar(cleanQuery, "startReindex", "index"); idx != "" {
				indexName = idx
			}
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

func errorResponseWithStatus(status int, err error) GraphQLResponseBody {
	return GraphQLResponseBody{Errors: []map[string]interface{}{{"message": err.Error(), "statusCode": status}}}
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
  success: Boolean!
  statusCode: Int!
  message: String!
  items: [UserDocument!]
  pageInfo: PageInfo
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

input UserSearchInput {
  query: String
  role: String
  isActive: Boolean
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
  searchUsers(input: UserSearchInput!): UserSearchResponse!
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

func parseGraphQLInput(query string, vars map[string]interface{}, fieldName string, target interface{}) {
	if vars != nil && len(vars) > 0 {
		// 1. Direct "input" key
		if raw, ok := vars["input"].(map[string]interface{}); ok {
			b, _ := json.Marshal(raw)
			_ = json.Unmarshal(b, target)
			return
		}
		// 2. Check variable reference: e.g. searchUsers(input: $input_0)
		idx := strings.Index(query, fieldName)
		if idx != -1 {
			sub := query[idx:]
			inputIdx := strings.Index(sub, "input")
			if inputIdx != -1 {
				colonIdx := strings.Index(sub[inputIdx:], ":")
				if colonIdx != -1 {
					valSub := strings.TrimSpace(sub[inputIdx+colonIdx+1:])
					if strings.HasPrefix(valSub, "$") {
						endIdx := strings.IndexAny(valSub, " \t\r\n,)}")
						var varName string
						if endIdx != -1 {
							varName = valSub[1:endIdx]
						} else {
							varName = valSub[1:]
						}
						if raw, ok := vars[varName].(map[string]interface{}); ok {
							b, _ := json.Marshal(raw)
							_ = json.Unmarshal(b, target)
							return
						}
					}
				}
			}
		}
		// 3. Fallback: Search any map variable
		for _, v := range vars {
			if raw, ok := v.(map[string]interface{}); ok {
				b, _ := json.Marshal(raw)
				_ = json.Unmarshal(b, target)
				return
			}
		}
	}
	cleanQuery := stripGraphQLComments(query)
	inlineStr := extractInlineInput(cleanQuery, fieldName)
	if inlineStr == "" {
		return
	}
	jsonStr, err := graphqlToJSON(inlineStr)
	if err == nil {
		_ = json.Unmarshal([]byte(jsonStr), target)
	}
}

func stripGraphQLComments(q string) string {
	lines := strings.Split(q, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "#"); idx != -1 {
			quotes := strings.Count(line[:idx], "\"")
			if quotes%2 == 0 {
				lines[i] = line[:idx]
			}
		}
	}
	return strings.Join(lines, "\n")
}

func extractInlineInput(query, fieldName string) string {
	idx := strings.Index(query, fieldName)
	if idx == -1 {
		return ""
	}
	sub := query[idx:]
	inputIdx := strings.Index(sub, "input")
	if inputIdx == -1 {
		return ""
	}
	colonIdx := strings.Index(sub[inputIdx:], ":")
	if colonIdx == -1 {
		return ""
	}
	braceIdx := strings.Index(sub[inputIdx+colonIdx:], "{")
	if braceIdx == -1 {
		return ""
	}

	startIdx := idx + inputIdx + colonIdx + braceIdx
	braceCount := 0
	endIdx := -1
	inString := false
	escaped := false

	for i := startIdx; i < len(query); i++ {
		char := query[i]
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
		} else {
			if char == '"' {
				inString = true
			} else if char == '{' {
				braceCount++
			} else if char == '}' {
				braceCount--
				if braceCount == 0 {
					endIdx = i
					break
				}
			}
		}
	}

	if endIdx != -1 {
		return query[startIdx : endIdx+1]
	}
	return ""
}

func extractInlineScalar(query, fieldName, argName string) string {
	idx := strings.Index(query, fieldName)
	if idx == -1 {
		return ""
	}
	sub := query[idx:]
	argIdx := strings.Index(sub, argName)
	if argIdx == -1 {
		return ""
	}
	colonIdx := strings.Index(sub[argIdx:], ":")
	if colonIdx == -1 {
		return ""
	}
	valSub := strings.TrimSpace(sub[argIdx+colonIdx+1:])
	if strings.HasPrefix(valSub, "\"") {
		endQuote := strings.Index(valSub[1:], "\"")
		if endQuote != -1 {
			return valSub[1 : 1+endQuote]
		}
	}
	endIdx := strings.IndexAny(valSub, " \t\r\n,)}")
	if endIdx != -1 {
		return valSub[:endIdx]
	}
	return valSub
}

type gqlParser struct {
	src []rune
	pos int
}

func (p *gqlParser) skipWhitespaceAndCommas() {
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ',' {
			p.pos++
		} else {
			break
		}
	}
}

func (p *gqlParser) parseValue() (interface{}, error) {
	p.skipWhitespaceAndCommas()
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("unexpected EOF")
	}

	char := p.src[p.pos]

	if char == '{' {
		return p.parseObject()
	} else if char == '[' {
		return p.parseList()
	} else if char == '"' {
		return p.parseString()
	} else {
		return p.parsePrimitiveOrEnum()
	}
}

func (p *gqlParser) parseObject() (map[string]interface{}, error) {
	p.pos++ // skip '{'
	result := make(map[string]interface{})

	for {
		p.skipWhitespaceAndCommas()
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("unclosed '{'")
		}
		if p.src[p.pos] == '}' {
			p.pos++ // skip '}'
			break
		}

		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}

		p.skipWhitespaceAndCommas()
		if p.pos >= len(p.src) || p.src[p.pos] != ':' {
			return nil, fmt.Errorf("expected ':' after key %s", key)
		}
		p.pos++ // skip ':'

		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		result[key] = val
	}

	return result, nil
}

func (p *gqlParser) parseList() ([]interface{}, error) {
	p.pos++ // skip '['
	result := make([]interface{}, 0)

	for {
		p.skipWhitespaceAndCommas()
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("unclosed '['")
		}
		if p.src[p.pos] == ']' {
			p.pos++ // skip ']'
			break
		}

		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}

	return result, nil
}

func (p *gqlParser) parseKey() (string, error) {
	p.skipWhitespaceAndCommas()
	if p.pos >= len(p.src) {
		return "", fmt.Errorf("unexpected EOF reading key")
	}

	if p.src[p.pos] == '"' {
		s, err := p.parseString()
		return s.(string), err
	}

	start := p.pos
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		if r == ':' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ',' || r == '}' || r == '{' {
			break
		}
		p.pos++
	}
	key := string(p.src[start:p.pos])
	if key == "" {
		return "", fmt.Errorf("empty key")
	}
	return key, nil
}

func (p *gqlParser) parseString() (interface{}, error) {
	p.pos++ // skip opening '"'
	var sb strings.Builder
	escaped := false

	for p.pos < len(p.src) {
		char := p.src[p.pos]
		p.pos++
		if escaped {
			sb.WriteRune(char)
			escaped = false
		} else if char == '\\' {
			escaped = true
		} else if char == '"' {
			return sb.String(), nil
		} else {
			sb.WriteRune(char)
		}
	}
	return nil, fmt.Errorf("unclosed string literal")
}

func (p *gqlParser) parsePrimitiveOrEnum() (interface{}, error) {
	start := p.pos
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ',' || r == '}' || r == ']' || r == ':' {
			break
		}
		p.pos++
	}
	token := string(p.src[start:p.pos])
	if token == "true" {
		return true, nil
	}
	if token == "false" {
		return false, nil
	}
	if token == "null" {
		return nil, nil
	}

	var intVal int64
	if _, err := fmt.Sscanf(token, "%d", &intVal); err == nil && !strings.Contains(token, ".") {
		return intVal, nil
	}

	var floatVal float64
	if _, err := fmt.Sscanf(token, "%f", &floatVal); err == nil && strings.Contains(token, ".") {
		return floatVal, nil
	}

	return token, nil
}

func graphqlToJSON(gql string) (string, error) {
	parser := &gqlParser{src: []rune(gql)}
	val, err := parser.parseValue()
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(val)
	if err != nil {
		return "", err
	}
	return string(b), nil
}


