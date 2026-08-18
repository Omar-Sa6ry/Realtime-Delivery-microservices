package websocket

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/users"
)

// WsJwtVerifyFunc verifies a JWT and returns the parsed payload.
// Signature verification stays in the owning service (e.g. a user-service
// gRPC call or a local secret check) — mirrors the WS_JWT_SERVICE injection
// in the TS package.
type WsJwtVerifyFunc func(token string) (*users.JwtPayload, error)

// WsJwtStrategy authenticates WebSocket handshakes —
// mirrors the TS WsJwtStrategy class.
type WsJwtStrategy struct {
	verify WsJwtVerifyFunc
}

// NewWsJwtStrategy creates a strategy backed by the given verifier.
func NewWsJwtStrategy(verify WsJwtVerifyFunc) *WsJwtStrategy {
	return &WsJwtStrategy{verify: verify}
}

// Authenticate extracts and verifies the token from the handshake request,
// returning the canonical payload or a WsException(UNAUTHENTICATED).
func (s *WsJwtStrategy) Authenticate(req *http.Request) (*users.JwtPayload, error) {
	token := ExtractToken(req)
	if token == "" {
		return nil, NewWsException(WsErrorUnauthenticated, "Missing token", false)
	}

	payload, err := s.verify(token)
	if err != nil {
		return nil, NewWsException(WsErrorUnauthenticated, "Invalid or expired token", false)
	}
	return NormalizePayload(payload), nil
}

// ExtractToken extracts the JWT from the Authorization header or the ?token= query param.
func ExtractToken(req *http.Request) string {
	if req == nil {
		return ""
	}
	if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if token := req.URL.Query().Get("token"); token != "" {
		return token
	}
	return ""
}

// NormalizePayload maps any JWT claim layout onto the canonical payload —
// mirrors the TS WsJwtStrategy.normalize().
func NormalizePayload(p *users.JwtPayload) *users.JwtPayload {
	if p == nil {
		return nil
	}
	cp := *p
	cp.UserID = p.UserIDOrEmpty()
	return &cp
}

// ParseJWTClaims decodes the payload segment of a JWT WITHOUT signature
// verification. Verification must be performed by the caller.
func ParseJWTClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("jwt: invalid token structure")
	}

	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(strings.NewReplacer("-", "+", "_", "/").Replace(payload))
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("jwt: invalid payload encoding: %w", err)
		}
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("jwt: invalid claims JSON: %w", err)
	}
	return claims, nil
}

// SubFromJWT extracts the sub / userId / user_id claim without signature
// verification. Used where the API Gateway already verified the token.
func SubFromJWT(token string) string {
	claims, err := ParseJWTClaims(token)
	if err != nil {
		return ""
	}
	for _, field := range []string{"sub", "userId", "user_id"} {
		if v, ok := claims[field].(string); ok && v != "" {
			return v
		}
	}
	return ""
}