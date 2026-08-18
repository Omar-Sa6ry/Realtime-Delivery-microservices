package websocket

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	sharedws "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/websocket"
	"github.com/gorilla/websocket"
)

// upgrader configures the gorilla/websocket HTTP upgrader.
// CheckOrigin should restrict to your frontend domain in production.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// TODO: restrict origins in production — e.g. return origin == "https://yourapp.com"
	CheckOrigin:      func(r *http.Request) bool { return true },
	HandshakeTimeout: 10 * time.Second,
}

// Handler upgrades HTTP connections to WebSocket and registers them with the Hub.
//
// Authentication:
//   - Authorization: Bearer <JWT>   (preferred)
//   - ?token=<JWT>                  (browser fallback — cannot set WS headers)
//
// The API Gateway is responsible for full JWT signature verification.
// This handler only extracts the `sub` / `userId` claim from the payload.
type Handler struct {
	hub *Hub
}

// NewHandler creates a new WebSocket HTTP handler backed by the given Hub.
func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

// ServeHTTP upgrades the connection and hands it off to the Hub.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := extractUserID(r)
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("WebSocket: upgrade failed", "error", err, "remoteAddr", r.RemoteAddr)
		return
	}

	// Register blocks until the connection closes (readPump exits).
	h.hub.Register(r.Context(), userID, wsConn)
}

// NewServer builds an http.Server that exposes only the WebSocket endpoint.
// Run this on its own port (WSPort from config, default :4003).
func NewServer(addr string, hub *Hub) *http.Server {
	mux := http.NewServeMux()
	h := NewHandler(hub)

	mux.HandleFunc("/ws", h.ServeHTTP)

	// Kubernetes liveness probe
	mux.HandleFunc("/ws/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"UP","connections":` + strconv.Itoa(hub.ActiveUsers()) + `}`))
	})

	return &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// extractUserID extracts the user ID from the JWT in the Authorization header or token query param.
func extractUserID(r *http.Request) (string, bool) {
	token := sharedws.ExtractToken(r)
	if token == "" {
		return "", false
	}

	userID := sharedws.SubFromJWT(token)
	if userID == "" {
		return "", false
	}
	return userID, true
}
