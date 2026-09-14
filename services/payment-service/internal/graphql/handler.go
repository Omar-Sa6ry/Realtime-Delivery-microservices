package graphql

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	pkgauth "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/auth"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/i18n"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/ports"
)

func ExtractLanguage(c *gin.Context) string {
	if lang := c.Query("lang"); lang != "" {
		return i18n.NormalizeLang(lang)
	}
	accept := c.GetHeader("Accept-Language")
	return i18n.NormalizeLang(accept)
}

func LanguageMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := ExtractLanguage(c)
		ctx := i18n.WithLanguage(c.Request.Context(), lang)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func HealthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := ExtractLanguage(c)
		now := time.Now().UTC().Format(time.RFC3339)
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"statusCode": 200,
			"message":    i18n.T(lang, "server.healthy"),
			"timeStamp":  now,
			"data": gin.H{
				"name":    "payment-service",
				"version": "1.0.0",
				"status":  "healthy",
			},
		})
	}
}

func ReadyHandler(db interface{ Ping() error }) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := ExtractLanguage(c)
		if err := db.Ping(); err != nil {
			slog.Error("health/ready: DB ping failed", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success":    false,
				"statusCode": 503,
				"message":    i18n.T(lang, "error.internal"),
				"timeStamp":  time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"statusCode": 200,
			"message":    i18n.T(lang, "server.healthy"),
			"timeStamp":  time.Now().UTC().Format(time.RFC3339),
		})
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
			lang := ExtractLanguage(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success":    false,
				"statusCode": 401,
				"message":    i18n.T(lang, "error.unauthorized"),
				"timeStamp":  time.Now().UTC().Format(time.RFC3339),
			})
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

func formatPayment(p *domain.Payment) gin.H {
	if p == nil {
		return nil
	}

	var authAt, capAt, canAt, failAt *string
	if p.AuthorizedAt != nil {
		t := p.AuthorizedAt.Format(time.RFC3339)
		authAt = &t
	}
	if p.CapturedAt != nil {
		t := p.CapturedAt.Format(time.RFC3339)
		capAt = &t
	}
	if p.CancelledAt != nil {
		t := p.CancelledAt.Format(time.RFC3339)
		canAt = &t
	}
	if p.FailedAt != nil {
		t := p.FailedAt.Format(time.RFC3339)
		failAt = &t
	}

	return gin.H{
		"id":                    p.ID,
		"deliveryId":            p.DeliveryID,
		"userId":                p.UserID,
		"delivery": gin.H{
			"id": p.DeliveryID,
		},
		"user": gin.H{
			"id": p.UserID,
		},
		"amountMinor":           p.AmountMinor,
		"currency":              p.Currency,
		"status":                string(p.Status),
		"provider":              p.Provider,
		"authorizedAmountMinor": p.AuthorizedAmountMinor,
		"capturedAmountMinor":   p.CapturedAmountMinor,
		"refundedAmountMinor":   p.RefundedAmountMinor,
		"correlationId":         p.CorrelationID,
		"causationId":           p.CausationID,
		"createdAt":             p.CreatedAt.Format(time.RFC3339),
		"updatedAt":             p.UpdatedAt.Format(time.RFC3339),
		"authorizedAt":          authAt,
		"capturedAt":            capAt,
		"cancelledAt":           canAt,
		"failedAt":              failAt,
	}
}

func formatRefund(r *domain.Refund) gin.H {
	if r == nil {
		return nil
	}
	return gin.H{
		"id":               r.ID,
		"paymentId":        r.PaymentID,
		"deliveryId":       r.DeliveryID,
		"amountMinor":      r.AmountMinor,
		"currency":         r.Currency,
		"status":           string(r.Status),
		"reason":           r.Reason,
		"providerRefundId": r.ProviderRefundID,
		"correlationId":    r.CorrelationID,
		"causationId":      r.CausationID,
		"createdAt":        r.CreatedAt.Format(time.RFC3339),
		"updatedAt":        r.UpdatedAt.Format(time.RFC3339),
	}
}

func GraphQLHandler(paymentSvc *services.PaymentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Query         string                 `json:"query"`
			OperationName string                 `json:"operationName"`
			Variables     map[string]interface{} `json:"variables"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			lang := ExtractLanguage(c)
			c.JSON(http.StatusBadRequest, gin.H{
				"success":    false,
				"statusCode": 400,
				"message":    i18n.T(lang, "validation.id_required"),
				"timeStamp":  time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		ctx := c.Request.Context()
		lang := i18n.FromContext(ctx)
		now := time.Now().UTC().Format(time.RFC3339)

		// 1. Apollo Federation SDL Introspection
		if strings.Contains(req.Query, "_service") {
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"_service": gin.H{"sdl": PaymentSubgraphSDL},
				},
			})
			return
		}

		// 1.5. Gateway readiness probe: query { __typename }
		if strings.Contains(req.Query, "__typename") {
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"__typename": "Query",
				},
			})
			return
		}

		// 2. Introspection queries
		if strings.Contains(req.Query, "__schema") || strings.Contains(req.Query, "__type") {
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{"__schema": gin.H{"types": []interface{}{}}},
			})
			return
		}

		// 3. Payment Service Info
		if strings.Contains(req.Query, "paymentServiceInfo") {
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"paymentServiceInfo": gin.H{
						"success":    true,
						"statusCode": 200,
						"message":    i18n.T(lang, "server.healthy"),
						"timeStamp":  now,
						"data": gin.H{
							"name":    "payment-service",
							"version": "1.0.0",
							"status":  "healthy",
						},
					},
				},
			})
			return
		}

		// 4. Query: payment(id: ID!)
		if strings.Contains(req.Query, "payment(") && !strings.Contains(req.Query, "payments(") {
			paymentID := extractIDFromQueryOrVars(req.Query, req.Variables)
			if paymentID == "" {
				c.JSON(http.StatusOK, gin.H{
					"data": gin.H{
						"payment": gin.H{
							"success":    false,
							"statusCode": 400,
							"message":    i18n.T(lang, "validation.id_required"),
							"timeStamp":  now,
							"data":       nil,
						},
					},
				})
				return
			}

			payment, err := LoadPayment(ctx, paymentID)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"data": gin.H{
						"payment": gin.H{
							"success":    false,
							"statusCode": 404,
							"message":    i18n.T(lang, "payment.not_found"),
							"timeStamp":  now,
							"data":       nil,
						},
					},
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"payment": gin.H{
						"success":    true,
						"statusCode": 200,
						"message":    i18n.T(lang, "payment.found"),
						"timeStamp":  now,
						"data":       formatPayment(payment),
					},
				},
			})
			return
		}

		// 5. Query: payments(page: Int, limit: Int, userId: String)
		if strings.Contains(req.Query, "payments(") || strings.Contains(req.Query, "payments {") {
			page := 1
			limit := 20
			var userID string

			if p, ok := req.Variables["page"].(float64); ok && p > 0 {
				page = int(p)
			}
			if l, ok := req.Variables["limit"].(float64); ok && l > 0 {
				limit = int(l)
			}
			if u, ok := req.Variables["userId"].(string); ok {
				userID = u
			}

			payments, total, err := paymentSvc.ListPayments(ctx, page, limit, userID)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"data": gin.H{
						"payments": gin.H{
							"success":    false,
							"statusCode": 500,
							"message":    i18n.T(lang, "error.internal"),
							"timeStamp":  now,
							"data":       nil,
						},
					},
				})
				return
			}

			items := make([]gin.H, 0, len(payments))
			for _, p := range payments {
				items = append(items, formatPayment(p))
			}

			var nextPage *int
			if page*limit < total {
				np := page + 1
				nextPage = &np
			}

			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"payments": gin.H{
						"success":    true,
						"statusCode": 200,
						"message":    i18n.T(lang, "payment.list_found"),
						"timeStamp":  now,
						"data": gin.H{
							"paginationInfo": gin.H{
								"totalItems":  total,
								"currentPage": page,
								"nextPage":    nextPage,
							},
							"items": items,
						},
					},
				},
			})
			return
		}

		// 6. Mutation: createPayment(input: CreatePaymentInput!)
		if strings.Contains(req.Query, "createPayment") {
			inputMap := extractInputMap(req.Variables)
			delID, _ := inputMap["deliveryId"].(string)
			usrID, _ := inputMap["userId"].(string)
			curr, _ := inputMap["currency"].(string)
			corrID, _ := inputMap["correlationId"].(string)
			causID, _ := inputMap["causationId"].(string)

			var amountMinor int64
			if a, ok := inputMap["amountMinor"].(float64); ok {
				amountMinor = int64(a)
			}

			res, err := paymentSvc.CreatePayment(ctx, services.CreatePaymentInput{
				DeliveryID:     delID,
				UserID:         usrID,
				AmountMinor:    amountMinor,
				Currency:       curr,
				IdempotencyKey: fmt.Sprintf("create-%s", delID),
				CorrelationID:  corrID,
				CausationID:    causID,
			})
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"data": gin.H{
						"createPayment": gin.H{
							"success":    false,
							"statusCode": 400,
							"message":    err.Error(),
							"timeStamp":  now,
							"data":       nil,
						},
					},
				})
				return
			}

			createdPayment, _ := paymentSvc.GetPayment(ctx, res.PaymentID)
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"createPayment": gin.H{
						"success":    true,
						"statusCode": 201,
						"message":    i18n.T(lang, "payment.created"),
						"timeStamp":  now,
						"data":       formatPayment(createdPayment),
					},
				},
			})
			return
		}

		// 7. Mutation: authorizePayment
		if strings.Contains(req.Query, "authorizePayment") {
			inputMap := extractInputMap(req.Variables)
			paymentID, _ := inputMap["paymentId"].(string)
			corrID, _ := inputMap["correlationId"].(string)

			payment, err := paymentSvc.AuthorizePayment(ctx, services.AuthorizePaymentInput{
				PaymentID:      paymentID,
				IdempotencyKey: fmt.Sprintf("auth-%s", paymentID),
				CorrelationID:  corrID,
			})
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"data": gin.H{
						"authorizePayment": gin.H{
							"success":    false,
							"statusCode": 400,
							"message":    err.Error(),
							"timeStamp":  now,
							"data":       nil,
						},
					},
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"authorizePayment": gin.H{
						"success":    true,
						"statusCode": 200,
						"message":    i18n.T(lang, "payment.authorized"),
						"timeStamp":  now,
						"data":       formatPayment(payment),
					},
				},
			})
			return
		}

		// 8. Mutation: capturePayment
		if strings.Contains(req.Query, "capturePayment") {
			inputMap := extractInputMap(req.Variables)
			paymentID, _ := inputMap["paymentId"].(string)
			corrID, _ := inputMap["correlationId"].(string)
			var amountMinor int64
			if a, ok := inputMap["amountMinor"].(float64); ok {
				amountMinor = int64(a)
			}

			payment, err := paymentSvc.CapturePayment(ctx, services.CapturePaymentInput{
				PaymentID:      paymentID,
				AmountMinor:    amountMinor,
				IdempotencyKey: fmt.Sprintf("cap-%s", paymentID),
				CorrelationID:  corrID,
			})
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"data": gin.H{
						"capturePayment": gin.H{
							"success":    false,
							"statusCode": 400,
							"message":    err.Error(),
							"timeStamp":  now,
							"data":       nil,
						},
					},
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"capturePayment": gin.H{
						"success":    true,
						"statusCode": 200,
						"message":    i18n.T(lang, "payment.captured"),
						"timeStamp":  now,
						"data":       formatPayment(payment),
					},
				},
			})
			return
		}

		// 9. Mutation: cancelAuthorization
		if strings.Contains(req.Query, "cancelAuthorization") {
			inputMap := extractInputMap(req.Variables)
			paymentID, _ := inputMap["paymentId"].(string)

			payment, err := paymentSvc.CancelAuthorization(ctx, paymentID, fmt.Sprintf("cancel-%s", paymentID))
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"data": gin.H{
						"cancelAuthorization": gin.H{
							"success":    false,
							"statusCode": 400,
							"message":    err.Error(),
							"timeStamp":  now,
							"data":       nil,
						},
					},
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"cancelAuthorization": gin.H{
						"success":    true,
						"statusCode": 200,
						"message":    i18n.T(lang, "payment.cancelled"),
						"timeStamp":  now,
						"data":       formatPayment(payment),
					},
				},
			})
			return
		}

		// 10. Mutation: createRefund
		if strings.Contains(req.Query, "createRefund") {
			inputMap := extractInputMap(req.Variables)
			paymentID, _ := inputMap["paymentId"].(string)
			reason, _ := inputMap["reason"].(string)
			corrID, _ := inputMap["correlationId"].(string)
			var amountMinor int64
			if a, ok := inputMap["amountMinor"].(float64); ok {
				amountMinor = int64(a)
			}

			refund, err := paymentSvc.CreateRefund(ctx, services.CreateRefundInput{
				PaymentID:      paymentID,
				AmountMinor:    amountMinor,
				Reason:         reason,
				IdempotencyKey: fmt.Sprintf("refund-%s-%d", paymentID, time.Now().UnixNano()),
				CorrelationID:  corrID,
			})
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"data": gin.H{
						"createRefund": gin.H{
							"success":    false,
							"statusCode": 400,
							"message":    err.Error(),
							"timeStamp":  now,
							"data":       nil,
						},
					},
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"createRefund": gin.H{
						"success":    true,
						"statusCode": 200,
						"message":    i18n.T(lang, "payment.refunded"),
						"timeStamp":  now,
						"data":       formatRefund(refund),
					},
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"errors": []gin.H{
				{"message": "operation not recognized"},
			},
		})
	}
}

func extractInputMap(vars map[string]interface{}) map[string]interface{} {
	if vars == nil {
		return map[string]interface{}{}
	}
	if input, ok := vars["input"].(map[string]interface{}); ok {
		return input
	}
	return vars
}

func extractIDFromQueryOrVars(query string, vars map[string]interface{}) string {
	if vars != nil {
		if id, ok := vars["id"].(string); ok && id != "" {
			return id
		}
	}

	idx := strings.Index(query, "id:")
	if idx == -1 {
		return ""
	}
	sub := query[idx+3:]
	sub = strings.TrimSpace(sub)
	if strings.HasPrefix(sub, "\"") {
		sub = sub[1:]
		end := strings.Index(sub, "\"")
		if end != -1 {
			return sub[:end]
		}
	}
	return ""
}
