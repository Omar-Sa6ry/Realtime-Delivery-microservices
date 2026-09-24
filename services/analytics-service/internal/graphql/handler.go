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
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func LanguageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.Header.Get("x-lang")
		if lang == "" {
			lang = r.Header.Get("X-Lang")
		}
		if lang == "" {
			lang = r.URL.Query().Get("lang")
		}
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
	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"statusCode": 200,
		"message":    i18n.T(lang, "server.healthy"),
		"timeStamp":  time.Now().UTC().Format(time.RFC3339),
		"data": map[string]any{
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
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"success":    false,
				"statusCode": 503,
				"message":    i18n.T(lang, "error.internal"),
				"timeStamp":  time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success":    true,
			"statusCode": 200,
			"message":    i18n.T(lang, "server.healthy"),
			"timeStamp":  time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func GraphQLHandler(resolver *Resolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusOK, map[string]any{
				"message": "Analytics GraphQL. Use POST with query.",
			})
			return
		}

		var req gqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			lang := i18n.FromContext(r.Context())
			writeJSON(w, http.StatusBadRequest, map[string]any{
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
			vars = map[string]any{}
		}
		scope := validation.ScopeFromRequest(r)
		query := req.Query

		// 1. Apollo Federation SDL introspection (gateway composition).
		if strings.Contains(query, "_service") {
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"_service": map[string]any{"sdl": AnalyticsSubgraphSDL},
				},
			})
			return
		}

		// 2. Gateway readiness probe: query { __typename }.
		if strings.Contains(query, "__typename") && !strings.Contains(query, "analyticsServiceInfo") {
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{"__typename": "Query"},
			})
			return
		}

		// 3. Generic introspection stub.
		if strings.Contains(query, "__schema") || strings.Contains(query, "__type") {
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"__schema": map[string]any{"types": []any{}},
				},
			})
			return
		}

		// 4. analyticsServiceInfo.
		if strings.Contains(query, "analyticsServiceInfo") {
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"analyticsServiceInfo": resolver.ServiceInfo(ctx),
				},
			})
			return
		}

		role := strings.ToLower(strings.TrimSpace(r.Header.Get("x-user-role")))
		if role == "" {
			role = strings.ToLower(strings.TrimSpace(r.Header.Get("X-User-Role")))
		}
		if role != "admin" {
			lang := i18n.FromContext(ctx)
			writeJSON(w, http.StatusForbidden, map[string]any{
				"errors": []map[string]any{
					{
						"message": i18n.T(lang, "error.unauthorized"),
						"extensions": map[string]any{
							"code": "FORBIDDEN",
						},
					},
				},
			})
			return
		}

		// 5. platformOverview.
		if strings.Contains(query, "platformOverview") {
			tr, err := validation.ParseRange(query, vars)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"platformOverview": resolver.PlatformOverview(ctx, tr, scope),
				},
			})
			return
		}

		// 6. deliveryAnalytics.
		if strings.Contains(query, "deliveryAnalytics") {
			f, err := validation.ParseDeliveryFilter(query, vars)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"deliveryAnalytics": resolver.DeliveryAnalytics(ctx, f, scope),
				},
			})
			return
		}

		// 7. driverAnalytics (singular; topDrivers contains "topDrivers" too
		// so check the plural first below via separate branch order).
		if strings.Contains(query, "driverAnalytics") && !strings.Contains(query, "topDrivers") {
			f, err := validation.ParseDriverFilter(query, vars)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"driverAnalytics": resolver.DriverAnalytics(ctx, f, scope),
				},
			})
			return
		}

		// 8. topDrivers.
		if strings.Contains(query, "topDrivers") {
			tr, err := validation.ParseRange(query, vars)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			limit := validation.ParseLimit(query, vars, 10)
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"topDrivers": resolver.TopDrivers(ctx, tr, limit, scope),
				},
			})
			return
		}

		// 9. paymentAnalytics.
		if strings.Contains(query, "paymentAnalytics") {
			f, err := validation.ParsePaymentFilter(query, vars)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"paymentAnalytics": resolver.PaymentAnalytics(ctx, f, scope),
				},
			})
			return
		}

		// 10. rawAnalyticsEvents (admin/debug).
		if strings.Contains(query, "rawAnalyticsEvents") {
			page, limit := validation.ParsePage(query, vars)
			rawEventType := validation.ExtractQueryField(query, "eventType", vars)
			eventType, err := validation.NormalizeEventType(rawEventType)
			if err != nil {
				writeDataError(w, ctx, err)
				return
			}
			var from, to *time.Time
			fromStr := validation.ExtractQueryField(query, "from", vars)
			if fromStr != "" {
				if tm, err := time.Parse(time.RFC3339, fromStr); err == nil {
					from = &tm
				}
			}
			toStr := validation.ExtractQueryField(query, "to", vars)
			if toStr != "" {
				if tm, err := time.Parse(time.RFC3339, toStr); err == nil {
					to = &tm
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"rawAnalyticsEvents": resolver.RawAnalyticsEvents(ctx, page, limit, eventType, from, to),
				},
			})
			return
		}

		// 11. dataQualityIssues (admin/debug).
		if strings.Contains(query, "dataQualityIssues") {
			page, limit := validation.ParsePage(query, vars)
			severity := validation.ExtractQueryField(query, "severity", vars)
			if severity != "" {
				var err error
				severity, err = validation.ValidateSeverity(severity)
				if err != nil {
					writeDataError(w, ctx, err)
					return
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"dataQualityIssues": resolver.DataQualityIssues(ctx, page, limit, severity),
				},
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"errors": []map[string]any{{"message": "operation not recognized"}},
		})
	}
}

func writeDataError(w http.ResponseWriter, ctx context.Context, err error) {
	lang := i18n.FromContext(ctx)
	writeJSON(w, http.StatusOK, map[string]any{
		"errors": []map[string]any{{"message": i18n.T(lang, "error.internal") + ": " + err.Error()}},
	})
}
