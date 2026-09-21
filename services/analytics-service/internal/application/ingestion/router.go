package ingestion

import (
	"fmt"
	"strings"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion/handlers"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

type Router struct {
	routes map[string]handlers.Handler
}

func NewRouter() *Router {
	return &Router{routes: map[string]handlers.Handler{}}
}

func (r *Router) Register(prefix string, h handlers.Handler) {
	r.routes[prefix] = h
}

func (r *Router) Route(eventType string) (handlers.Handler, error) {
	prefix := eventType
	if i := strings.Index(eventType, "."); i != -1 {
		prefix = eventType[:i]
	}
	h, ok := r.routes[prefix]
	if !ok {
		return nil, fmt.Errorf("%w: no handler for %s", domain.ErrUnknownEventType, eventType)
	}
	if !h.Handles(eventType) {
		return nil, fmt.Errorf("%w: %s", domain.ErrUnknownEventType, eventType)
	}
	return h, nil
}
