package websocket

import (
	"encoding/base64"
	"errors"
	"net/http"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/users"
)

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{
			name: "authorization header",
			req:  newRequestWithHeader("Bearer abc.def.ghi"),
			want: "abc.def.ghi",
		},
		{
			name: "query token",
			req:  newRequest("/ws?token=xyz.123.456"),
			want: "xyz.123.456",
		},
		{
			name: "no token",
			req:  newRequest("/ws"),
			want: "",
		},
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractToken(tt.req); got != tt.want {
				t.Errorf("ExtractToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWsJwtStrategy(t *testing.T) {
	payload := &users.JwtPayload{Sub: "user-7", Role: "driver"}

	strategy := NewWsJwtStrategy(func(token string) (*users.JwtPayload, error) {
		if token == "good-token" {
			return payload, nil
		}
		return nil, errors.New("invalid token")
	})

	t.Run("missing token", func(t *testing.T) {
		_, err := strategy.Authenticate(newRequest("/ws"))
		wsErr, ok := err.(*WsException)
		if !ok || wsErr.Code != WsErrorUnauthenticated {
			t.Errorf("expected WsException UNAUTHENTICATED, got %v", err)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := strategy.Authenticate(newRequest("/ws?token=bad-token"))
		wsErr, ok := err.(*WsException)
		if !ok || wsErr.Code != WsErrorUnauthenticated {
			t.Errorf("expected WsException UNAUTHENTICATED, got %v", err)
		}
	})

	t.Run("valid token normalized", func(t *testing.T) {
		got, err := strategy.Authenticate(newRequest("/ws?token=good-token"))
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if got.UserID != "user-7" {
			t.Errorf("expected normalized userId user-7, got %s", got.UserID)
		}
		if got.Role != "driver" {
			t.Errorf("expected role driver, got %s", got.Role)
		}
	})
}

func TestWsAuthGuard(t *testing.T) {
	strategy := NewWsJwtStrategy(func(token string) (*users.JwtPayload, error) {
		if token == "good-token" {
			return &users.JwtPayload{Sub: "user-7", Role: "user"}, nil
		}
		return nil, errors.New("invalid token")
	})
	guard := NewWsAuthGuard(strategy)

	t.Run("rejects and closes socket", func(t *testing.T) {
		conn := &fakeConn{}
		_, err := guard.Authenticate(conn, newRequest("/ws?token=bad"))
		if err == nil {
			t.Fatal("expected auth error")
		}
		if conn.closeCode != WsCloseUnauthorized {
			t.Errorf("expected close code %d, got %d", WsCloseUnauthorized, conn.closeCode)
		}
	})

	t.Run("accepts valid handshake", func(t *testing.T) {
		conn := &fakeConn{}
		got, err := guard.Authenticate(conn, newRequest("/ws?token=good-token"))
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if got.UserID != "user-7" {
			t.Errorf("expected userId user-7, got %s", got.UserID)
		}
		if conn.closeCode != 0 {
			t.Errorf("expected no close, got %d", conn.closeCode)
		}
	})
}

func TestSubFromJWT(t *testing.T) {
	t.Run("sub claim", func(t *testing.T) {
		token := "header." + base64url(`{"sub":"user-1","role":"admin"}`) + ".sig"
		if got := SubFromJWT(token); got != "user-1" {
			t.Errorf("expected user-1, got %q", got)
		}
	})

	t.Run("userId claim", func(t *testing.T) {
		token := "header." + base64url(`{"userId":"user-2","role":"driver"}`) + ".sig"
		if got := SubFromJWT(token); got != "user-2" {
			t.Errorf("expected user-2, got %q", got)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		if got := SubFromJWT("not-a-jwt"); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestParseJWTClaimsError(t *testing.T) {
	if _, err := ParseJWTClaims("a.b"); err == nil {
		t.Error("expected error for malformed token")
	}
	if _, err := ParseJWTClaims("a.b.c"); err == nil {
		t.Error("expected error for invalid base64 payload")
	}
}

func base64url(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func newRequest(target string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, target, nil)
	return req
}

func newRequestWithHeader(auth string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", auth)
	return req
}