package graphql

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/i18n"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/validation"
)

type gqlRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func LanguageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		if lang == "" {
			lang = r.Header.Get("Accept-Language")
		}
		ctx := i18n.WithLanguage(r.Context(), i18n.NormalizeLang(lang))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func DataLoaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithLoaders(r.Context(), NewLoaders())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func HealthLiveHandler(w http.ResponseWriter, r *http.Request) {
	lang := i18n.FromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"statusCode": 200,
		"message":    i18n.T(lang, "server.healthy"),
		"timeStamp":  time.Now().UTC().Format(time.RFC3339),
		"data": map[string]interface{}{
			"name":    "analytics-service",
			"version": "1.0.0",
			"status":  "healthy",
		},
	})
}

func HealthReadyHandler(ping func(ctx context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := i18n.FromContext(r.Context())
		if err := ping(r.Context()); err != nil {
			slog.Error("health/ready: dependency ping failed", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"success":    false,
				"statusCode": 503,
				"message":    i18n.T(lang, "error.internal"),
				"timeStamp":  time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":    true,
			"statusCode": 200,
			"message":    i18n.T(lang, "server.healthy"),
			"timeStamp":  time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// GraphQLHandler serves POST /graphql and POST /analytics/graphql following
// the payment-service pattern: string dispatch into the resolver layer.
func GraphQLHandler(resolver *Resolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"message": "Analytics GraphQL. Use POST with query.",
			})
			return
		}

		var req gqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			lang := i18n.FromContext(r.Context())
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success":    false,
				"statusCode": 400,
				"message":    i18n.T(lang, "error.internal"),
				"timeStamp":  time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		ctx := r.Context()
		vars := req.Variables
		if vars == nil {
			vars = map[string]interface{}{}
		}
		scope := validation.ScopeFromRequest(r)
		query := req.Query

		// 1. Apollo Federation SDL introspection (gateway composition).
		if strings.Contains(query, "_service") {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"_service": map[string]interface{}{"sdl": AnalyticsSubgraphSDL},
				},
			})
			return
		}

		// 2. Gateway readiness probe: query { __typename }.
		if strings.Contains(query, "__typename") && !strings.Contains(query, "analyticsServiceInfo") {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{"__typename": "Query"},
			})
			return
		}

		// 3. Generic introspection stub.
		if strings.Contains(query, "__schema") || strings.Contains(query, "__type") {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"__schema": map[string]interface{}{"types": []interface{}{}},
				},
			})
			return
		}

		// 4. analyticsServiceInfo.
		if strings.Contains(query, "analyticsServiceInfo") {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"analyticsServiceInfo": resolver.ServiceInfo(ctx),
				},
			})
			return
		}

		// 5. platformOverview.
		if strings.Contains(query, "platformOverview") {
			tr, err := validation.ParseRange(vars)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"platformOverview": resolver.PlatformOverview(ctx, tr, scope),
				},
			})
			return
		}

		// 6. deliveryAnalytics.
		if strings.Contains(query, "deliveryAnalytics") {
			f, err := validation.ParseDeliveryFilter(vars)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"deliveryAnalytics": resolver.DeliveryAnalytics(ctx, f, scope),
				},
			})
			return
		}

		// 7. driverAnalytics (singular; topDrivers contains "topDrivers" too
		// so check the plural first below via separate branch order).
		if strings.Contains(query, "driverAnalytics") && !strings.Contains(query, "topDrivers") {
			f, err := validation.ParseDriverFilter(vars)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"driverAnalytics": resolver.DriverAnalytics(ctx, f, scope),
				},
			})
			return
		}

		// 8. topDrivers.
		if strings.Contains(query, "topDrivers") {
			tr, err := validation.ParseRange(vars)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			limit := validation.ClampTopLimit(0)
			if v, ok := vars["limit"].(float64); ok {
				limit = validation.ClampTopLimit(int(v))
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"topDrivers": resolver.TopDrivers(ctx, tr, limit, scope),
				},
			})
			return
		}

		// 9. paymentAnalytics.
		if strings.Contains(query, "paymentAnalytics") {
			f, err := validation.ParsePaymentFilter(vars)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"paymentAnalytics": resolver.PaymentAnalytics(ctx, f, scope),
				},
			})
			return
		}

		// 10. rawAnalyticsEvents (admin/debug).
		if strings.Contains(query, "rawAnalyticsEvents") {
			page, limit := validation.ParsePage(vars)
			eventType := ""
			if v, ok := vars["eventType"].(string); ok {
				eventType = v
			}
			var from, to *time.Time
			if v, ok := vars["from"].(string); ok && v != "" {
				if tm, err := time.Parse(time.RFC3339, v); err == nil {
					from = &tm
				}
			}
			if v, ok := vars["to"].(string); ok && v != "" {
				if tm, err := time.Parse(time.RFC3339, v); err == nil {
					to = &tm
				}
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"rawAnalyticsEvents": resolver.RawAnalyticsEvents(ctx, page, limit, eventType, from, to),
				},
			})
			return
		}

		// 11. dataQualityIssues (admin/debug).
		if strings.Contains(query, "dataQualityIssues") {
			page, limit := validation.ParsePage(vars)
			severity := ""
			if v, ok := vars["severity"].(string); ok {
				var err error
				severity, err = validation.ValidateSeverity(v)
				if err != nil {
					writeDataError(w, ctx, err)
					return
				}
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"data": map[string]interface{}{
					"dataQualityIssues": resolver.DataQualityIssues(ctx, page, limit, severity),
				},
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"errors": []map[string]interface{}{{"message": "operation not recognized"}},
		})
	}
}

func writeDataError(w http.ResponseWriter, ctx context.Context, err error) {
	lang := i18n.FromContext(ctx)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"errors": []map[string]interface{}{{"message": i18n.T(lang, "error.internal") + ": " + err.Error()}},
	})
}
