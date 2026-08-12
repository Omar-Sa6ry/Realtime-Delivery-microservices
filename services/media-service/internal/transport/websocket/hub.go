// Package websocket implements the real-time WebSocket notification hub.
//
// Architecture:
//
//	Browser ──── WS connection ────► Handler.ServeHTTP
//	                                        │
//	                              Hub.Register(conn)
//	                                        │
//	              Redis SUBSCRIBE ws:user:{userId}
//	                                        │
//	              goroutine reads Redis messages
//	                                        │
//	              conn.WriteMessage  ──────► Browser
package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/gorilla/websocket"

	wsadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/ws"
)

const (
	// writeWait is the time allowed to write a message to a peer.
	writeWait = 10 * time.Second

	// pongWait is the time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// pingPeriod sends pings to peer at this period — must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// maxMessageSize is the maximum message size allowed from peer.
	maxMessageSize = 512
)

// Conn wraps a gorilla WebSocket connection with thread-safe writes.
type Conn struct {
	ws   *websocket.Conn
	send chan []byte
}

// Hub maintains active WebSocket connections indexed by userID.
// Multiple connections per user are supported (multiple browser tabs).
type Hub struct {
	mu          sync.RWMutex
	connections map[string][]*Conn // userID → active connections
	redis       *goredis.Client
	subscribed  map[string]context.CancelFunc // userID → cancel for Redis sub goroutine
}

// NewHub creates a new Hub.
func NewHub(redisClient *goredis.Client) *Hub {
	return &Hub{
		connections: make(map[string][]*Conn),
		redis:       redisClient,
		subscribed:  make(map[string]context.CancelFunc),
	}
}

// Register adds a new WebSocket connection for a user.
// It also starts a Redis Pub/Sub subscription for that user if not already running.
func (h *Hub) Register(ctx context.Context, userID string, wsConn *websocket.Conn) {
	conn := &Conn{
		ws:   wsConn,
		send: make(chan []byte, 256),
	}

	h.mu.Lock()
	h.connections[userID] = append(h.connections[userID], conn)
	firstConn := len(h.connections[userID]) == 1
	h.mu.Unlock()

	slog.Info("WebSocket: client connected", "userId", userID)

	// Start Redis subscription only for the first connection of this user.
	if firstConn {
		subCtx, cancel := context.WithCancel(ctx)
		h.mu.Lock()
		h.subscribed[userID] = cancel
		h.mu.Unlock()
		go h.subscribeRedis(subCtx, userID)
	}

	// Start writer goroutine — serialises all writes to this connection.
	go h.writePump(conn)

	// Block in reader goroutine — handles pong and detects disconnect.
	h.readPump(userID, conn)
}

// readPump reads from the WebSocket connection, keeping it alive via pong handling.
// When the connection closes it deregisters the conn from the Hub.
func (h *Hub) readPump(userID string, conn *Conn) {
	defer func() {
		h.deregister(userID, conn)
		conn.ws.Close()
	}()

	conn.ws.SetReadLimit(maxMessageSize)
	_ = conn.ws.SetReadDeadline(time.Now().Add(pongWait))
	conn.ws.SetPongHandler(func(string) error {
		return conn.ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		// We only accept pings/control frames from the client — no application messages.
		_, _, err := conn.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("WebSocket: unexpected close", "userId", userID, "error", err)
			}
			break
		}
	}
}

// writePump serialises outgoing messages to the WebSocket connection.
func (h *Hub) writePump(conn *Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.ws.Close()
	}()

	for {
		select {
		case msg, ok := <-conn.send:
			_ = conn.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel.
				_ = conn.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Warn("WebSocket: write error", "error", err)
				return
			}

		case <-ticker.C:
			_ = conn.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// subscribeRedis subscribes to Redis Pub/Sub for a user and forwards messages
// to all active WebSocket connections for that user.
func (h *Hub) subscribeRedis(ctx context.Context, userID string) {
	channel := wsadapter.ChannelForUser(userID)
	sub := h.redis.Subscribe(ctx, channel)
	defer sub.Close()

	slog.Info("WebSocket: Redis subscription started", "userId", userID, "channel", channel)

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			slog.Info("WebSocket: Redis subscription cancelled", "userId", userID)
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			h.broadcast(userID, []byte(msg.Payload))
		}
	}
}

// broadcast sends a raw JSON payload to all connections for a given user.
func (h *Hub) broadcast(userID string, data []byte) {
	h.mu.RLock()
	conns := make([]*Conn, len(h.connections[userID]))
	copy(conns, h.connections[userID])
	h.mu.RUnlock()

	for _, conn := range conns {
		select {
		case conn.send <- data:
		default:
			// Buffer full — drop message and log; do not block worker goroutines.
			slog.Warn("WebSocket: send buffer full, dropping message", "userId", userID)
		}
	}
}

// deregister removes a specific connection from the Hub.
// If no connections remain for a user, the Redis subscription is cancelled.
func (h *Hub) deregister(userID string, conn *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns := h.connections[userID]
	for i, c := range conns {
		if c == conn {
			close(c.send)
			h.connections[userID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}

	if len(h.connections[userID]) == 0 {
		delete(h.connections, userID)
		if cancel, ok := h.subscribed[userID]; ok {
			cancel()
			delete(h.subscribed, userID)
		}
		slog.Info("WebSocket: last connection closed, Redis subscription cancelled", "userId", userID)
	}
}

// BroadcastJSON is a helper for broadcasting any JSON-serialisable value.
func (h *Hub) BroadcastJSON(userID string, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Warn("WebSocket: BroadcastJSON marshal error", "error", err)
		return
	}
	h.broadcast(userID, data)
}

// ActiveUsers returns the number of users with at least one active connection.
func (h *Hub) ActiveUsers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}
