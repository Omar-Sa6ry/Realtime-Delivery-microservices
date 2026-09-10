package graphql

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	gql "github.com/graph-gophers/graphql-go"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/i18n"
)

// ExtractLanguage determines the user language preference from Accept-Language or query parameter.
func ExtractLanguage(r *http.Request) string {
	if lang := r.URL.Query().Get("lang"); lang != "" {
		return i18n.NormalizeLang(lang)
	}
	accept := r.Header.Get("Accept-Language")
	return i18n.NormalizeLang(accept)
}

// ExtractUserID retrieves the user ID from headers (e.g., set by API gateway/ingress).
func ExtractUserID(r *http.Request) string {
	if uid := r.Header.Get("x-user-id"); uid != "" {
		return uid
	}
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		return uid
	}
	return ""
}

// ExtractUserRole retrieves the user role from headers.
func ExtractUserRole(r *http.Request) string {
	if role := r.Header.Get("x-user-role"); role != "" {
		return role
	}
	if role := r.Header.Get("X-User-Role"); role != "" {
		return role
	}
	return ""
}

// HealthHandler serves liveness and readiness probe requests.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	lang := ExtractLanguage(r)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"service": "driver-service",
		"message": i18n.T(lang, "server.healthy"),
	})
}

// GraphQLHandler wraps the parsed GraphQL schema into an http.HandlerFunc.
type GraphQLHandler struct {
	schema  *gql.Schema
	loaders *Loaders
}

// NewHandler creates a real Apollo Federation-compatible GraphQL subgraph HTTP handler.
func NewHandler(rootResolver *RootResolver, loaders *Loaders) http.HandlerFunc {
	// Parse schema with Federation options (schema definition)
	schema, err := gql.ParseSchema(DriverSubgraphSDL, rootResolver, gql.UseStringDescriptions())
	if err != nil {
		log.Fatalf("Failed to parse GraphQL schema: %v", err)
	}

	h := &GraphQLHandler{schema: schema, loaders: loaders}
	return h.ServeHTTP
}

func (h *GraphQLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	lang := ExtractLanguage(r)

	if r.Method == http.MethodGet {
		// Respond with basic service information on GET
		resp := map[string]interface{}{
			"status":  "healthy",
			"service": "driver-subgraph",
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": i18n.T(lang, "server.method_not_allowed"),
		})
		return
	}

	var params struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []map[string]string{
				{"message": "Invalid JSON body"},
			},
		})
		return
	}

	// Attach request contextual metadata (e.g., user ID, language, role)
	ctx := context.WithValue(r.Context(), "userID", ExtractUserID(r))
	ctx = context.WithValue(ctx, "role", ExtractUserRole(r))
	ctx = context.WithValue(ctx, "lang", lang)
	if h.loaders != nil {
		ctx = WithLoaders(ctx, h.loaders)
	}

	response := h.schema.Exec(ctx, params.Query, params.OperationName, params.Variables)
	_ = json.NewEncoder(w).Encode(response)
}
