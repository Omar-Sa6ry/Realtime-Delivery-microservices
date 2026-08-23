package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	sharedauth "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/auth"
	sharednats "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/nats"
	sharedws "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/websocket"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/config"
	gorilla "github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"golang.org/x/time/rate"
)

// MediaWSMessage represents a message sent over media WebSocket
type MediaWSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// UploadProgressData represents upload progress payload
type UploadProgressData struct {
	MediaID    string  `json:"mediaId"`
	UploadID   string  `json:"uploadId"`
	UploadedMB float64 `json:"uploadedMB"`
	TotalMB    float64 `json:"totalMB"`
	SpeedMBps  float64 `json:"speedMBps"`
	Percent    int     `json:"percent"`
	PartsDone  int     `json:"partsDone"`
	PartsTotal int     `json:"partsTotal"`
	Status     string  `json:"status"`
}

// ProcessingProgressData represents processing progress payload
type ProcessingProgressData struct {
	MediaID string `json:"mediaId"`
	Stage   string `json:"stage"` // scanning, image, video, compression, metadata
	Percent int    `json:"percent"`
	Message string `json:"message,omitempty"`
}

// MediaReadyData represents media ready payload
type MediaReadyData struct {
	MediaID     string   `json:"mediaId"`
	MediaType   string   `json:"mediaType"`
	Versions    []string `json:"versions"`
	DownloadURL string   `json:"downloadUrl,omitempty"`
}

// MediaDeletedData represents media deleted payload
type MediaDeletedData struct {
	MediaID string `json:"mediaId"`
}

// MediaFailedData represents media failed payload
type MediaFailedData struct {
	MediaID string `json:"mediaId"`
	Reason  string `json:"reason"`
}

// Connection represents a WebSocket connection with user context
type Connection struct {
	Conn     *gorilla.Conn
	UserID   string
	Send     chan []byte
	Cancel   context.CancelFunc
	RateLim  *rate.Limiter
	LastPing time.Time
}

// Hub manages WebSocket connections and NATS subscriptions
type Hub struct {
	mu          sync.RWMutex
	connections map[string]map[*Connection]bool // userID -> connections
	natsClient  *sharednats.NatsClient
	logger      *slog.Logger
	upgrader    gorilla.Upgrader
	rateLimit   rate.Limit
	rateBurst   int
}

// NewHub creates a new WebSocket hub
func NewHub(natsClient *sharednats.NatsClient, logger *slog.Logger, cfg *config.Config) *Hub {
	return &Hub{
		connections: make(map[string]map[*Connection]bool),
		natsClient:  natsClient,
		logger:      logger.With("component", "media-ws-hub"),
		upgrader: gorilla.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		rateLimit: rate.Limit(cfg.WSRateLimitPerSecond),
		rateBurst: cfg.WSRateLimitBurst,
	}
}

// Run starts the hub: subscribes to NATS and handles message routing
func (h *Hub) Run(ctx context.Context) error {
	h.logger.Info("Starting media WebSocket hub", "port", "4003")

	// Subscribe to media realtime NATS subjects
	subjects := []string{
		sharednats.RealtimeMediaUploadProgress,
		sharednats.RealtimeMediaProcessingProgress,
		sharednats.RealtimeMediaReady,
		sharednats.RealtimeMediaDeleted,
		sharednats.RealtimeMediaFailed,
	}

	for _, subject := range subjects {
		if err := h.subscribeToSubject(ctx, subject); err != nil {
			h.logger.Error("Failed to subscribe to NATS subject", "subject", subject, "error", err)
			return err
		}
		h.logger.Info("Subscribed to NATS subject", "subject", subject)
	}

	// Start ping/pong cleanup routine
	go h.cleanupStaleConnections(ctx)

	<-ctx.Done()
	h.logger.Info("Media WebSocket hub stopped")
	return nil
}

func (h *Hub) subscribeToSubject(ctx context.Context, subject string) error {
	sub, err := h.natsClient.Conn().Subscribe(subject, func(msg *nats.Msg) {
		h.handleNATSMessage(subject, msg.Data)
	})
	if err != nil {
		return err
	}
	// Keep subscription alive
	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
	}()
	return nil
}

func (h *Hub) handleNATSMessage(subject string, data []byte) {
	var msg sharedws.RealtimeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		h.logger.Warn("Failed to unmarshal NATS message", "subject", subject, "error", err)
		return
	}

	// Extract userID from message data for targeted delivery
	userID := h.extractUserID(msg.Data)
	if userID == "" {
		h.logger.Debug("No userID in message, broadcasting to all (should not happen)", "subject", subject)
		return
	}

	// Convert to server message format
	serverMsg := sharedws.ServerMessage{
		Type:      sharedws.ServerMessageType(msg.Type),
		Data:      msg.Data,
		RequestID: msg.MessageID,
	}

	msgBytes, err := serverMsg.Marshal()
	if err != nil {
		h.logger.Error("Failed to marshal server message", "error", err)
		return
	}

	h.sendToUser(userID, msgBytes)
}

func (h *Hub) extractUserID(data json.RawMessage) string {
	var payload struct {
		UserID  string `json:"userId"`
		MediaID string `json:"mediaId"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return payload.UserID
}

func (h *Hub) sendToUser(userID string, data []byte) {
	h.mu.RLock()
	conns := h.connections[userID]
	h.mu.RUnlock()

	for conn := range conns {
		select {
		case conn.Send <- data:
		default:
			// Connection buffer full, close it
			h.closeConnection(conn, "send buffer full")
		}
	}
}

// ServeWS handles WebSocket upgrade and connection lifecycle
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	// Browsers cannot attach arbitrary Authorization headers to WebSocket
	// handshakes, so accept a Bearer header or a short-lived query token.
	// Identity always comes from verified claims; user_id is ignored.
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		if token := r.URL.Query().Get("access_token"); token != "" {
			authHeader = "Bearer " + token
		} else if token := r.URL.Query().Get("token"); token != "" {
			authHeader = "Bearer " + token
		}
	}
	claims, err := sharedauth.Authenticate(authHeader)
	if err != nil {
		h.logger.Warn("WebSocket connection rejected: invalid JWT", "error", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := claims.UserID()

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}

	_, cancel := context.WithCancel(context.Background())
	c := &Connection{
		Conn:     conn,
		UserID:   userID,
		Send:     make(chan []byte, 256),
		Cancel:   cancel,
		RateLim:  rate.NewLimiter(h.rateLimit, h.rateBurst),
		LastPing: time.Now(),
	}

	h.registerConnection(c)
	h.logger.Info("WebSocket connected", "userID", userID, "remote", r.RemoteAddr)

	// Start read/write pumps
	go c.readPump(h)
	go c.writePump(h)
}

func (h *Hub) registerConnection(c *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.connections[c.UserID] == nil {
		h.connections[c.UserID] = make(map[*Connection]bool)
	}
	h.connections[c.UserID][c] = true
}

func (h *Hub) unregisterConnection(c *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns := h.connections[c.UserID]; conns != nil {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.connections, c.UserID)
		}
	}
}

func (h *Hub) closeConnection(c *Connection, reason string) {
	h.unregisterConnection(c)
	close(c.Send)
	c.Cancel()
	c.Conn.Close()
	h.logger.Info("WebSocket closed", "userID", c.UserID, "reason", reason)
}

func (h *Hub) cleanupStaleConnections(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.mu.RLock()
			var stale []*Connection
			for _, conns := range h.connections {
				for c := range conns {
					if time.Since(c.LastPing) > 90*time.Second {
						stale = append(stale, c)
					}
				}
			}
			h.mu.RUnlock()
			for _, c := range stale {
				h.closeConnection(c, "stale connection (no ping)")
			}
		}
	}
}

// Connection read pump
func (c *Connection) readPump(h *Hub) {
	defer func() {
		h.closeConnection(c, "read pump ended")
	}()

	c.Conn.SetReadLimit(16384)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.LastPing = time.Now()
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			if err != gorilla.ErrCloseSent && gorilla.IsCloseError(err, gorilla.CloseGoingAway, gorilla.CloseAbnormalClosure) {
				h.logger.Error("WebSocket read error", "userID", c.UserID, "error", err)
			}
			break
		}

		// Rate limit incoming messages
		if !c.RateLim.Allow() {
			h.logger.Warn("Rate limit exceeded", "userID", c.UserID)
			continue
		}

		var msg sharedws.ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			h.logger.Warn("Invalid message format", "userID", c.UserID, "error", err)
			continue
		}

		c.handleMessage(h, msg)
	}
}

func (c *Connection) handleMessage(h *Hub, msg sharedws.ClientMessage) {
	switch msg.Type {
	case sharedws.ClientMessagePing:
		// Respond with pong
		pong := sharedws.ServerMessage{
			Type:      sharedws.ServerMessagePong,
			RequestID: msg.RequestID,
			Data:      json.RawMessage(`{"timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `"}`),
		}
		if bytes, err := pong.Marshal(); err == nil {
			select {
			case c.Send <- bytes:
			default:
			}
		}

	case sharedws.ClientMessageSubscribeDelivery:
		// For media, we could subscribe to specific media IDs
		// For now, just ack
		ack := sharedws.ServerMessage{
			Type:      sharedws.ServerMessageSubscribed,
			RequestID: msg.RequestID,
			Data:      json.RawMessage(`{"subscribed":true}`),
		}
		if bytes, err := ack.Marshal(); err == nil {
			select {
			case c.Send <- bytes:
			default:
			}
		}

	default:
		h.logger.Debug("Unknown message type", "type", msg.Type, "userID", c.UserID)
	}
}

// Connection write pump
func (c *Connection) writePump(h *Hub) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		h.closeConnection(c, "write pump ended")
	}()

	for {
		select {
		case data, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(gorilla.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(gorilla.TextMessage, data); err != nil {
				h.logger.Error("WebSocket write error", "userID", c.UserID, "error", err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(gorilla.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
