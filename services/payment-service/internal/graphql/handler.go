package graphql

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
func GraphQLHandler() gin.HandlerFunc {
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

		c.JSON(http.StatusOK, gin.H{
			"errors": []map[string]string{
				{"message": "Query not supported in minimal setup"},
			},
		})
	}
}