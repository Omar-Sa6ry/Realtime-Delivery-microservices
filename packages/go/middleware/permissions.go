package middleware

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/constants"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RequirePermissionInterceptor verifies the authenticated user has at least one of the required
// permissions (derived from their role via constants.RolePermissionsMap). Mirrors the
// NestJS RoleGuard permission validation.
func RequirePermissionInterceptor(required ...constants.Permission) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if len(required) == 0 {
			return handler(ctx, req)
		}

		userRole := GetUserRole(ctx)
		if userRole == "" {
			return nil, status.Error(codes.Unauthenticated, "authentication required: missing x-user-role header")
		}

		perms := constants.RolePermissionsMap[constants.Role(userRole)]
		for _, reqPerm := range required {
			for _, have := range perms {
				if have == reqPerm {
					return handler(ctx, req)
				}
			}
		}

		return nil, status.Error(codes.PermissionDenied, "permission denied: insufficient privileges")
	}
}
