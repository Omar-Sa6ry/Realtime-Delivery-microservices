package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "super-secret"

func b64(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func sign(header, payload, secret string) string {
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validToken() string {
	header := b64(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload := b64(map[string]interface{}{
		"sub":  "user-1",
		"id":   "user-1",
		"role": RoleAdmin,
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	return sign(header, payload, testSecret)
}

func TestClaimsUserID(t *testing.T) {
	tests := []struct {
		name   string
		claims Claims
		want   string
	}{
		{"subject wins", Claims{Subject: "s", ID: "i"}, "s"},
		{"falls back to id", Claims{ID: "i"}, "i"},
		{"empty", Claims{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.claims.UserID())
		})
	}
}

func TestAuthenticateErrors(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	header := b64(map[string]string{"alg": "HS256"})
	payload := b64(map[string]interface{}{"sub": "user-1", "exp": time.Now().Add(time.Hour).Unix()})
	otherPayload := b64(map[string]interface{}{"sub": "user-2", "exp": time.Now().Add(time.Hour).Unix()})

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"empty header", "", "missing bearer token"},
		{"not bearer", "Basic abc", "missing bearer token"},
		{"too many fields", "Bearer a b", "missing bearer token"},
		{"too few segments", "Bearer " + header + "." + payload, "invalid token"},
		{"unsupported algorithm", "Bearer " + sign(b64(map[string]string{"alg": "RS256"}), payload, testSecret),
			"unsupported token algorithm"},
		{"garbage header segment", "Bearer not-base64." + payload + ".sig", "unsupported token algorithm"},
		{"invalid signature", "Bearer " + header + "." + payload + ".bm90LXRoZS1zaWduYXR1cmU",
			"invalid token signature"},
		{"tampered payload", "Bearer " + header + "." + otherPayload + "." +
			sign(header, payload, testSecret)[len(header)+1+len(payload)+1:], "invalid token signature"},
		{"invalid claims payload", "Bearer " + sign(header, "%%%not-json%%%", testSecret),
			"invalid token claims"},
		{"expired token", "Bearer " + sign(header,
			b64(map[string]interface{}{"sub": "user-1", "exp": time.Now().Add(-time.Minute).Unix()}),
			testSecret), "token expired"},
		{"missing subject", "Bearer " + sign(header,
			b64(map[string]interface{}{"role": "admin", "exp": time.Now().Add(time.Hour).Unix()}),
			testSecret), "token subject missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Authenticate(tt.header)
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

func TestAuthenticateMissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	_, err := Authenticate("Bearer " + validToken())
	require.Error(t, err)
	assert.Equal(t, "JWT_SECRET is not configured", err.Error())
}

func TestAuthenticateSuccess(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	claims, err := Authenticate("Bearer " + validToken())
	require.NoError(t, err)

	assert.Equal(t, "user-1", claims.UserID())
	assert.Equal(t, RoleAdmin, claims.Role)
	assert.Greater(t, claims.ExpiresAt, time.Now().Unix())
}

func TestAuthenticateAcceptsCaseInsensitiveBearer(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	token := validToken()
	claims, err := Authenticate("bearer " + token)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID())

	claims, err = Authenticate("BEARER " + token)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID())
}

func TestAuthenticateAcceptsTokenWithoutExpiry(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	header := b64(map[string]string{"alg": "HS256"})
	payload := b64(map[string]interface{}{"id": "only-id"})

	claims, err := Authenticate("Bearer " + sign(header, payload, testSecret))
	require.NoError(t, err)
	assert.Equal(t, "only-id", claims.UserID())
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		permission string
		allowed    bool
	}{
		{"admin can view users", RoleAdmin, PermissionViewUser, true},
		{"case insensitive", "ADMIN", "VIEW_USER", true},
		{"user cannot", "user", PermissionViewUser, false},
		{"admin cannot other permission", RoleAdmin, "delete_user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.allowed, HasPermission(tt.role, tt.permission))
		})
	}
}

func TestRequirePermission(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	t.Run("grants admin", func(t *testing.T) {
		claims, err := RequirePermission("Bearer "+validToken(), PermissionViewUser)
		require.NoError(t, err)
		assert.Equal(t, RoleAdmin, claims.Role)
	})

	t.Run("denies missing permission", func(t *testing.T) {
		_, err := RequirePermission("Bearer "+validToken(), "delete_user")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "permission delete_user denied")
	})

	t.Run("propagates authentication failure", func(t *testing.T) {
		_, err := RequirePermission("broken", PermissionViewUser)
		require.Error(t, err)
		assert.Equal(t, "missing bearer token", err.Error())
	})
}

func TestDecodeRejectsInvalidBase64(t *testing.T) {
	var out map[string]interface{}
	err := decode("!!!not-base64!!!", &out)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "illegal") || err != nil)
}
