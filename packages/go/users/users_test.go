package users

import (
	"encoding/json"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPaginationInput(t *testing.T) {
	tests := []struct {
		name        string
		page, limit int
		wantPage    int
		wantLimit   int
	}{
		{"valid values are kept", 4, 25, 4, 25},
		{"zero page falls back to default", 0, 25, constants.DefaultPage, 25},
		{"negative page falls back to default", -3, 25, constants.DefaultPage, 25},
		{"zero limit falls back to default", 4, 0, 4, constants.DefaultLimit},
		{"negative limit falls back to default", 4, -1, 4, constants.DefaultLimit},
		{"both invalid fall back", 0, 0, constants.DefaultPage, constants.DefaultLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPaginationInput(tt.page, tt.limit)
			assert.Equal(t, tt.wantPage, got.Page)
			assert.Equal(t, tt.wantLimit, got.Limit)
		})
	}
}

func TestPaginationInputOffset(t *testing.T) {
	tests := []struct {
		name string
		p    PaginationInput
		want int
	}{
		{"first page", PaginationInput{Page: 1, Limit: 10}, 0},
		{"second page", PaginationInput{Page: 2, Limit: 10}, 10},
		{"fifth page", PaginationInput{Page: 5, Limit: 25}, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.p.Offset())
		})
	}
}

func TestJwtPayloadUserIDOrEmpty(t *testing.T) {
	tests := []struct {
		name    string
		payload JwtPayload
		want    string
	}{
		{"userId wins", JwtPayload{UserID: "u-1", Sub: "s-1", ID: "i-1"}, "u-1"},
		{"falls back to sub", JwtPayload{Sub: "s-1", ID: "i-1"}, "s-1"},
		{"falls back to id", JwtPayload{ID: "i-1"}, "i-1"},
		{"empty when no identity", JwtPayload{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.payload.UserIDOrEmpty())
		})
	}
}

func TestPackageIdentityConstants(t *testing.T) {
	assert.Equal(t, "user", PackageName)
	assert.Equal(t, "UserService", ServiceName)
	assert.Equal(t, "/user.UserService", FullService)
	assert.Equal(t, "jwt-payload", JwtPayloadKey)
}

func TestJwtPayloadJSONRoundTrip(t *testing.T) {
	in := JwtPayload{
		UserID:    "u-1",
		Role:      "driver",
		Email:     "d@example.com",
		SessionID: "sess-1",
		Iat:       100,
		Exp:       200,
	}

	raw, err := json.Marshal(in)
	require.NoError(t, err)

	var out JwtPayload
	require.NoError(t, json.Unmarshal(raw, &out))

	assert.Equal(t, in, out)
	assert.NotContains(t, string(raw), `"userId":""`, "omitempty fields should be dropped")
}

func TestGetUserResponseJSONTags(t *testing.T) {
	raw, err := json.Marshal(GetUserResponse{ID: "1", FirstName: "A"})
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, "1", decoded["id"])
	assert.Equal(t, "A", decoded["first_name"])
	assert.Equal(t, "is_active", keysOf(decoded)["is_active"])
}

func keysOf(m map[string]interface{}) map[string]string {
	out := make(map[string]string, len(m))
	for k := range m {
		out[k] = k
	}
	return out
}

func TestValidateTokenResponseJSONTags(t *testing.T) {
	raw, err := json.Marshal(ValidateTokenResponse{Valid: true, UserID: "u-1", Role: "admin"})
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, true, decoded["valid"])
	assert.Equal(t, "u-1", decoded["user_id"])
	assert.Equal(t, "admin", decoded["role"])
}
