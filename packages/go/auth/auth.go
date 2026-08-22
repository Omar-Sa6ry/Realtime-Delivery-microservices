package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const PermissionViewUser = "VIEW_USER"

// Claims is the subset of the shared JWT contract required by search-service.
type Claims struct {
	Subject   string `json:"sub"`
	ID        string `json:"id"`
	Role      string `json:"role"`
	ExpiresAt int64  `json:"exp"`
}

func (c Claims) UserID() string {
	if c.Subject != "" {
		return c.Subject
	}
	return c.ID
}

// Authenticate verifies a bearer JWT using the same JWT_SECRET used by the Nest services.
func Authenticate(header string) (Claims, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return Claims{}, errors.New("missing bearer token")
	}
	segments := strings.Split(parts[1], ".")
	if len(segments) != 3 {
		return Claims{}, errors.New("invalid token")
	}

	var head struct {
		Algorithm string `json:"alg"`
	}
	if err := decode(segments[0], &head); err != nil || head.Algorithm != "HS256" {
		return Claims{}, errors.New("unsupported token algorithm")
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return Claims{}, errors.New("JWT_SECRET is not configured")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(segments[0] + "." + segments[1]))
	signature, err := base64.RawURLEncoding.DecodeString(segments[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return Claims{}, errors.New("invalid token signature")
	}

	var claims Claims
	if err := decode(segments[1], &claims); err != nil {
		return Claims{}, errors.New("invalid token claims")
	}
	if claims.ExpiresAt > 0 && time.Now().Unix() >= claims.ExpiresAt {
		return Claims{}, errors.New("token expired")
	}
	if claims.UserID() == "" {
		return Claims{}, errors.New("token subject missing")
	}
	return claims, nil
}

func RequirePermission(header, permission string) (Claims, error) {
	claims, err := Authenticate(header)
	if err != nil {
		return Claims{}, err
	}
	if !HasPermission(claims.Role, permission) {
		return Claims{}, fmt.Errorf("permission %s denied", permission)
	}
	return claims, nil
}

// Keep this map aligned with packages/ts/src/constants/rolePermissionsMap.constant.ts.
func HasPermission(role, permission string) bool {
	return role == "ADMIN" && permission == PermissionViewUser
}

func decode(value string, target interface{}) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, target)
}
