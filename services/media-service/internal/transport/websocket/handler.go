package websocket

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	token := ""

	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token = strings.TrimPrefix(auth, "Bearer ")
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		return "", false
	}

	userID := subFromJWT(token)
	if userID == "" {
		return "", false
	}
	return userID, true
}

// subFromJWT extracts the `sub` or `userId` claim from a JWT without signature verification.
// Full verification is performed by the API Gateway before traffic reaches this service.
func subFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}

	// JWT payload is base64url-encoded without padding.
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(
		strings.NewReplacer("-", "+", "_", "/").Replace(payload),
	)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return ""
		}
	}

	s := string(decoded)
	for _, field := range []string{"sub", "userId", "user_id"} {
		if v := jsonStringField(s, field); v != "" {
			return v
		}
	}
	return ""
}

// jsonStringField extracts a string JSON field without a full unmarshal.
func jsonStringField(s, field string) string {
	needle := `"` + field + `":"`
	idx := strings.Index(s, needle)
	if idx < 0 {
		return ""
	}
	start := idx + len(needle)
	end := strings.Index(s[start:], `"`)
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}
