package e2e

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// newID returns a fresh unique identifier for use in test payloads.
func newID() string { return uuid.New().String() }

// The helpers below turn malformed or failed GraphQL responses into ordinary
// test failures. Using them instead of bare type assertions keeps a single
// failing assertion from panicking and taking down the rest of the suite.

// rootField returns resp.data[name] as an object.
func rootField(t *testing.T, resp map[string]interface{}, name string) map[string]interface{} {
	t.Helper()
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response has no data object: %v", resp)
	}
	obj, ok := data[name].(map[string]interface{})
	if !ok {
		t.Fatalf("field %q missing or not an object (data=%v)", name, data)
	}
	return obj
}

// nested returns obj[key] as an object.
func nested(t *testing.T, obj map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	child, ok := obj[key].(map[string]interface{})
	if !ok {
		t.Fatalf("field %q missing or not an object (obj=%v)", key, obj)
	}
	return child
}

// wantSuccess fails the test unless obj["success"] is true, reporting the
// service-provided message and status code on failure.
func wantSuccess(t *testing.T, obj map[string]interface{}) {
	t.Helper()
	if ok, cast := obj["success"].(bool); !cast || !ok {
		t.Fatalf("operation failed (status=%v message=%v)", obj["statusCode"], obj["message"])
	}
}

// wantStr fails unless obj[key] is a string equal to want.
func wantStr(t *testing.T, obj map[string]interface{}, key, want string) {
	t.Helper()
	got, ok := obj[key].(string)
	if !ok {
		t.Fatalf("field %q is not a string (obj=%v)", key, obj)
	}
	if got != want {
		t.Errorf("%s = %q, want %q", key, got, want)
	}
}

// wantBool fails unless obj[key] is a bool equal to want.
func wantBool(t *testing.T, obj map[string]interface{}, key string, want bool) {
	t.Helper()
	got, ok := obj[key].(bool)
	if !ok {
		t.Fatalf("field %q is not a bool (obj=%v)", key, obj)
	}
	if got != want {
		t.Errorf("%s = %v, want %v", key, got, want)
	}
}

// driverIDOf extracts the driver id from a successful operation payload.
func driverIDOf(t *testing.T, payload map[string]interface{}) string {
	t.Helper()
	id, ok := payload["id"].(string)
	if !ok || id == "" {
		t.Fatalf("payload has no driver id: %v", payload)
	}
	return id
}

// registerDriver performs the register mutation and returns the new driver id
// plus the user it was registered for. It fails the test on any error.
func registerDriver(t *testing.T, client *GraphQLClient, role string) (driverID, userID string) {
	t.Helper()
	userID = fmt.Sprintf("e2e-user-%s", newID())
	resp, err := client.exec(t.Context(), registerDriverMutation, map[string]interface{}{
		"input": map[string]interface{}{
			"userId":      userID,
			"vehicleType": "CAR",
			"plateNumber": fmt.Sprintf("E2E-%s", newID()[:8]),
			"capacityKg":  100,
		},
	}, map[string]string{"x-user-id": userID, "x-user-role": role})
	if err != nil {
		t.Fatalf("register driver: %v", err)
	}
	op := rootField(t, resp, "registerDriver")
	wantSuccess(t, op)
	return driverIDOf(t, nested(t, op, "data")), userID
}
