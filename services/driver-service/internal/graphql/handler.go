package graphql

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

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

// Handler handles Apollo Federation GraphQL subgraph requests.
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	lang := ExtractLanguage(r)

	if r.Method == http.MethodGet {
		resp := map[string]interface{}{"data": map[string]interface{}{"__typename": "Query"}}
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

	var req struct {
		Query string `json:"query"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if strings.Contains(req.Query, "_service") {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"_service": map[string]interface{}{"sdl": DriverSubgraphSDL},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	if strings.Contains(req.Query, "driverServiceInfo") {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"driverServiceInfo": map[string]interface{}{
					"success":    true,
					"statusCode": 200,
					"message":    i18n.T(lang, "server.running"),
					"timeStamp":  time.Now().UTC().Format(time.RFC3339),
					"data": map[string]interface{}{
						"name":    "driver-service",
						"version": "1.0.0",
						"status":  i18n.T(lang, "server.healthy"),
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp := map[string]interface{}{
		"data": map[string]interface{}{"__typename": "Query"},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
