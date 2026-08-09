package middleware

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/constants"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	UserIDKey        contextKey = constants.HeaderXUserId
	UserRoleKey      contextKey = constants.HeaderXUserRole
	CorrelationIDKey contextKey = constants.HeaderXCorrelationId
)

// UnaryServerMetadataInterceptor extracts headers from incoming gRPC metadata and populates context.Context
func UnaryServerMetadataInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			if vals := md.Get(constants.HeaderXUserId); len(vals) > 0 {
				ctx = context.WithValue(ctx, UserIDKey, vals[0])
			}
			if vals := md.Get(constants.HeaderXUserRole); len(vals) > 0 {
				ctx = context.WithValue(ctx, UserRoleKey, vals[0])
			}
			if vals := md.Get(constants.HeaderXCorrelationId); len(vals) > 0 {
				ctx = context.WithValue(ctx, CorrelationIDKey, vals[0])
			}
		}
		return handler(ctx, req)
	}
}

// AuthInterceptor ensures that x-user-id is present in the context, returning Unauthenticated if missing.
func AuthInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		userID := GetUserID(ctx)
		if userID == "" {
			return nil, status.Error(codes.Unauthenticated, "authentication required: missing x-user-id header")
		}
		return handler(ctx, req)
	}
}

// RequireRoleInterceptor verifies that the authenticated user's role matches one of the allowed roles.
func RequireRoleInterceptor(allowedRoles ...string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		userRole := GetUserRole(ctx)
		if userRole == "" {
			return nil, status.Error(codes.Unauthenticated, "authentication required: missing x-user-role header")
		}

		for _, role := range allowedRoles {
			if userRole == role {
				return handler(ctx, req)
			}
		}

		return nil, status.Error(codes.PermissionDenied, "permission denied: insufficient role privileges")
	}
}

// GetUserID retrieves the authenticated userId from gRPC context
func GetUserID(ctx context.Context) string {
	if val, ok := ctx.Value(UserIDKey).(string); ok {
		return val
	}
	return ""
}

// GetUserRole retrieves the authenticated user's role from gRPC context
func GetUserRole(ctx context.Context) string {
	if val, ok := ctx.Value(UserRoleKey).(string); ok {
		return val
	}
	return ""
}

// GetCorrelationID retrieves the correlationId from gRPC context
func GetCorrelationID(ctx context.Context) string {
	if val, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return val
	}
	return ""
}
