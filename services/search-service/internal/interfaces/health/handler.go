package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/redis"
)

type Handler struct {
	repo  search.SearchRepository
	cache *redis.Cache
}

func NewHandler(repo search.SearchRepository, cache *redis.Cache) *Handler {
	return &Handler{
		repo:  repo,
		cache: cache,
	}
}

func (h *Handler) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    "UP",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func (h *Handler) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		osErr := h.repo.Ping(ctx)
		redisErr := h.cache.Ping(ctx)

		status := "UP"
		code := http.StatusOK
		if osErr != nil || redisErr != nil {
			status = "DOWN"
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": status,
			"dependencies": map[string]string{
				"opensearch": formatStatus(osErr),
				"redis":      formatStatus(redisErr),
			},
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func formatStatus(err error) string {
	if err == nil {
		return "UP"
	}
	return "DOWN: " + err.Error()
}
