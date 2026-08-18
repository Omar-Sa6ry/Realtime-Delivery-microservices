package websocket

import (
	"log/slog"
	"net/http"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/users"
)

// WsConnLike is the minimal socket surface needed to reject a handshake.
// Services adapt their WebSocket library (e.g. gorilla/websocket) to this
// interface — mirrors the TS client surface used by WsAuthGuard.
type WsConnLike interface {
	// CloseWith closes the connection with the given WebSocket close code and reason.
	CloseWith(code int, reason string) error
}

// WsAuthGuard authenticates WebSocket handshakes and closes the socket with
// 4401 on failure — mirrors the TS WsAuthGuard class.
type WsAuthGuard struct {
	strategy *WsJwtStrategy
}

// NewWsAuthGuard creates an auth guard backed by the given JWT strategy.
func NewWsAuthGuard(strategy *WsJwtStrategy) *WsAuthGuard {
	return &WsAuthGuard{strategy: strategy}
}

// Authenticate verifies the handshake request. On failure the socket is
// closed with 4401 (UNAUTHENTICATED) and the error is returned.
func (g *WsAuthGuard) Authenticate(conn WsConnLike, req *http.Request) (*users.JwtPayload, error) {
	payload, err := g.strategy.Authenticate(req)
	if err != nil {
		slog.Warn("WebSocket handshake rejected", "error", err.Error())
		if conn != nil {
			_ = conn.CloseWith(WsCloseUnauthorized, "UNAUTHENTICATED")
		}
		return nil, err
	}
	return payload, nil
}