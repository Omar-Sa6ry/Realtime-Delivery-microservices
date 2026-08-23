package auth

import "testing"

func TestHasPermissionUsesSharedRoleAndPermissionValues(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		permission string
		allowed    bool
	}{
		{name: "shared lowercase values", role: "admin", permission: "view_user", allowed: true},
		{name: "uppercase values remain compatible", role: "ADMIN", permission: "VIEW_USER", allowed: true},
		{name: "regular user is denied", role: "user", permission: "view_user", allowed: false},
		{name: "unrelated permission is denied", role: "admin", permission: "delete_user", allowed: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := HasPermission(test.role, test.permission); got != test.allowed {
				t.Fatalf("HasPermission(%q, %q) = %v, want %v", test.role, test.permission, got, test.allowed)
			}
		})
	}
}
