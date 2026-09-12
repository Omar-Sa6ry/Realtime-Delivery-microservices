package graphql

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/graphql-go/graphql"
)

// HealthResponse represents the health check response.
type HealthResponse struct {
	Success    bool     `json:"success"`
	StatusCode int      `json:"statusCode"`
	Message    string   `json:"message"`
	TimeStamp  string   `json:"timeStamp"`
	Data       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Status  string `json:"status"`
	} `json:"data"`
}

// handlerData holds the context for GraphQL execution.
type handlerData struct {
	schema     *graphql.Schema
	dataloader *dataloader.Map
}

// NewHandlerData creates new handler data with schema and dataloader.
func NewHandlerData(schema *graphql.Schema, dataloader *dataloader.Map) *handlerData {
	return &handlerData{
		schema:     schema,
		dataloader: dataloader,
	}
}

// HealthHandler returns a Gin handler for the /health/live endpoint.
func HealthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now().UTC().Format(time.RFC3339)
		resp := HealthResponse{
			Success:    true,
			StatusCode: 200,
			Message:    "Payment service is healthy",
			TimeStamp:  now,
		}
		resp.Data.Name = "payment-service"
		resp.Data.Version = "1.0.0"
		resp.Data.Status = "healthy"
		c.JSON(http.StatusOK, map[string]interface{}{
			"paymentServiceInfo": resp,
		})
	}
}

// GraphQLHandler returns a Gin handler for the /payment/graphql endpoint.
func GraphQLHandler(schema *graphql.Schema, dataloader *dataloader.Map) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Query         string `json:"query"`
			OperationName string `json:"operationName"`
			Variables     string `json:"variables"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check for health query
		if strings.Contains(req.Query, "paymentServiceInfo") || strings.Contains(req.Query, "paymentService_info") {
			HealthHandler()(c)
			return
		}

		// Execute GraphQL query
		params := graphql.Params{
			Schema:        schema,
			Query:         req.Query,
			OperationName: req.OperationName,
			VariableValues: parseVariables(req.Variables),
			Context:       context.Background(),
		}

		result := graphql.Do(params)
		if result.HasErrors() {
			c.JSON(http.StatusBadRequest, gin.H{"errors": result.Errors})
		} else {
			c.JSON(http.StatusOK, result.Data)
		}
	}
}

// parseVariables parses the variables string into a map.
func parseVariables(variables string) map[string]interface{} {
	if variables == "" || variables == "null" {
		return nil
	}

	var result map[string]interface{}
	if err := fmt.Unmarshal([]byte(variables), &result); err != nil {
		return nil
	}
	return result
}

// Schema returns the GraphQL schema.
func GetSchema() *graphql.Schema {
	// This would be populated with the full schema definition
	// For now, return a basic schema
	return nil
}