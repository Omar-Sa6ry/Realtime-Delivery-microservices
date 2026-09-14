package graphql

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	pkgauth "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/auth"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/ports"
)

type HealthResponse struct {
	Success    bool   `json:"success"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	TimeStamp  string `json:"timeStamp"`
	Data       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Status  string `json:"status"`
	} `json:"data"`
}

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

func ReadyHandler(db interface{ Ping() error }) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			slog.Error("health/ready: DB ping failed", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "db unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		claims, err := pkgauth.Authenticate(authHeader)
		if err != nil {
			slog.Warn("auth_middleware: invalid token", "error", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set("userID", claims.UserID())
		c.Set("userRole", claims.Role)
		c.Next()
	}
}

func DataLoaderMiddleware(paymentRepo ports.PaymentRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		loaders := NewLoaders(paymentRepo)
		ctx := WithLoaders(c.Request.Context(), loaders)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func GraphQLHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Query         string      `json:"query"`
			OperationName string      `json:"operationName"`
			Variables     interface{} `json:"variables"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if strings.Contains(req.Query, "_service") {
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"_service": gin.H{"sdl": PaymentSubgraphSDL},
				},
			})
			return
		}

		if strings.Contains(req.Query, "__typename") {
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{"__typename": "Query"},
			})
			return
		}

		if strings.Contains(req.Query, "paymentServiceInfo") {
			HealthHandler()(c)
			return
		}

		if strings.Contains(req.Query, "payment(") && strings.Contains(req.Query, "id:") {
			handlePaymentQuery(c, req.Variables)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"errors": []gin.H{
				{"message": "query not yet implemented in payment-service GraphQL handler"},
			},
		})
	}
}

func handlePaymentQuery(c *gin.Context, variables interface{}) {
	vars, ok := variables.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, gin.H{"errors": []gin.H{{"message": "variables must be an object"}}})
		return
	}
	id, ok := vars["id"].(string)
	if !ok || id == "" {
		c.JSON(http.StatusOK, gin.H{"errors": []gin.H{{"message": "id is required"}}})
		return
	}

	payment, err := LoadPayment(c.Request.Context(), id)
	if err != nil {
		slog.Error("graphql: payment query failed", "id", id, "error", err)
		c.JSON(http.StatusOK, gin.H{"errors": []gin.H{{"message": err.Error()}}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"payment": gin.H{
				"id":          payment.ID,
				"deliveryId":  payment.DeliveryID,
				"userId":      payment.UserID,
				"amountMinor": payment.AmountMinor,
				"currency":    payment.Currency,
				"status":      string(payment.Status),
				"createdAt":   payment.CreatedAt.Format(time.RFC3339),
				"updatedAt":   payment.UpdatedAt.Format(time.RFC3339),
			},
		},
	})
}
