package graphql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	sharedconstants "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/constants"
	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	appSearch "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/reindex"
	gql "github.com/graphql-go/graphql"
)

const graphqlPath = "/search/graphql"

type graphqlRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

type Server struct {
	schema gql.Schema
}

func NewServer(searchService *appSearch.Service, reindexService *reindex.Service, port string) (*Server, error) {
	resolver := NewResolver(searchService, reindexService)
	schema, err := buildSchema(resolver)
	if err != nil {
		return nil, fmt.Errorf("build graphql schema: %w", err)
	}
	return &Server{schema: schema}, nil
}

// Handler returns an http.HandlerFunc that handles GraphQL requests.
// This allows the caller to mount it on their own HTTP server/mux.
func (s *Server) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.handleGraphQL(w, r)
	}
}

func (s *Server) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)

	var params gql.Params
	switch r.Method {
	case http.MethodPost:
		var body graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid GraphQL request: %w", err))
			return
		}
		params = gql.Params{
			Schema:         s.schema,
			RequestString:  body.Query,
			VariableValues: body.Variables,
			OperationName:  body.OperationName,
			Context:        ctx,
		}
	case http.MethodGet:
		variables, err := parseVariables(r.URL.Query().Get("variables"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		params = gql.Params{
			Schema:         s.schema,
			RequestString:  r.URL.Query().Get("query"),
			VariableValues: variables,
			OperationName:  r.URL.Query().Get("operationName"),
			Context:        ctx,
		}
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, errors.New("only GET and POST requests are supported"))
		return
	}

	result := gql.Do(params)
	writeJSON(w, result)
}

func requestContext(r *http.Request) context.Context {
	ctx := r.Context()
	userID := r.Header.Get(sharedconstants.HeaderXUserId)
	userRole := r.Header.Get(sharedconstants.HeaderXUserRole)
	traceID := r.Header.Get(sharedconstants.HeaderXCorrelationId)
	authHeader := r.Header.Get("Authorization")

	ctx = sharedlogging.WithLogContext(ctx, sharedlogging.LogContext{
		TraceID: traceID,
		UserID:  userID,
		Method:  r.Method,
		Path:    r.URL.Path,
	})

	if userID != "" {
		ctx = context.WithValue(ctx, "x-user-id", userID)
	}
	if userRole != "" {
		ctx = context.WithValue(ctx, "x-user-role", userRole)
	}
	if authHeader != "" {
		ctx = context.WithValue(ctx, "authorization", authHeader)
	}

	ctx = context.WithValue(ctx, "timestamp", time.Now().UTC().Format(time.RFC3339))

	return ctx
}

func parseVariables(raw string) (map[string]interface{}, error) {
	if raw == "" {
		return nil, nil
	}
	var variables map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &variables); err != nil {
		return nil, fmt.Errorf("invalid GraphQL variables: %w", err)
	}
	return variables, nil
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to write GraphQL response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"errors": []interface{}{
			map[string]interface{}{"message": err.Error()},
		},
	})
}