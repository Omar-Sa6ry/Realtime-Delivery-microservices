package middleware

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/constants"
	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestHTTPStatusToGRPC(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       codes.Code
	}{
		{"bad request", 400, codes.InvalidArgument},
		{"unauthorized", 401, codes.Unauthenticated},
		{"forbidden", 403, codes.PermissionDenied},
		{"not found", 404, codes.NotFound},
		{"conflict", 409, codes.AlreadyExists},
		{"too many requests", 429, codes.ResourceExhausted},
		{"internal default", 500, codes.Internal},
		{"unknown default", 418, codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HTTPStatusToGRPC(tt.statusCode))
		})
	}
}

func metadataContext(t *testing.T, md metadata.MD) context.Context {
	t.Helper()
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestUnaryServerMetadataInterceptor(t *testing.T) {
	tests := []struct {
		name       string
		md         metadata.MD
		wantUser   string
		wantRole   string
		wantCorrID string
	}{
		{
			name:       "all headers present",
			md:         metadata.Pairs(constants.HeaderXUserId, "u-1", constants.HeaderXUserRole, "driver", constants.HeaderXCorrelationId, "c-1"),
			wantUser:   "u-1",
			wantRole:   "driver",
			wantCorrID: "c-1",
		},
		{
			name: "partial headers",
			md:   metadata.Pairs(constants.HeaderXUserId, "u-2"),
			// Correlation id is not set when the header is absent.
			wantUser: "u-2",
			wantRole: "",
		},
		{
			name:       "no metadata",
			md:         metadata.MD{},
			wantUser:   "",
			wantRole:   "",
			wantCorrID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := UnaryServerMetadataInterceptor()

			var gotUser, gotRole, gotCorr string
			_, err := interceptor(metadataContext(t, tt.md), struct{}{},
				&grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
				func(ctx context.Context, req interface{}) (interface{}, error) {
					gotUser = GetUserID(ctx)
					gotRole = GetUserRole(ctx)
					gotCorr = GetCorrelationID(ctx)
					return "ok", nil
				})

			require.NoError(t, err)
			assert.Equal(t, tt.wantUser, gotUser)
			assert.Equal(t, tt.wantRole, gotRole)
			if tt.wantCorrID != "" {
				assert.Equal(t, tt.wantCorrID, gotCorr)
			}
		})
	}
}

func TestUnaryServerMetadataInterceptorWithoutMetadata(t *testing.T) {
	interceptor := UnaryServerMetadataInterceptor()

	var called bool
	_, err := interceptor(context.Background(), struct{}{},
		&grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			called = true
			return nil, nil
		})

	require.NoError(t, err)
	assert.True(t, called)
}

func TestUnaryServerMetadataInterceptorSeedsLoggingContext(t *testing.T) {
	interceptor := UnaryServerMetadataInterceptor()

	var gotTraceID, gotUserID, gotMethod string
	_, err := interceptor(
		metadataContext(t, metadata.Pairs(
			constants.HeaderXCorrelationId, "corr-9",
			constants.HeaderXUserId, "u-9",
		)),
		struct{}{},
		&grpc.UnaryServerInfo{FullMethod: "/driver.DriverService/ReserveDriver"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			gotTraceID = sharedlogging.GetTraceID(ctx)
			gotUserID = sharedlogging.GetUserID(ctx)
			gotMethod = sharedlogging.GetMethod(ctx)
			return "ok", nil
		})

	require.NoError(t, err)
	assert.Equal(t, "corr-9", gotTraceID)
	assert.Equal(t, "u-9", gotUserID)
	assert.Equal(t, "/driver.DriverService/ReserveDriver", gotMethod)
}

func TestUnaryServerMetadataInterceptorGeneratesTraceID(t *testing.T) {
	interceptor := UnaryServerMetadataInterceptor()

	var gotTraceID string
	_, err := interceptor(metadataContext(t, metadata.MD{}), struct{}{},
		&grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			gotTraceID = sharedlogging.GetTraceID(ctx)
			return "ok", nil
		})

	require.NoError(t, err)
	assert.Len(t, gotTraceID, 32)
}

func TestAuthInterceptor(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}

	t.Run("rejects missing user id", func(t *testing.T) {
		interceptor := AuthInterceptor()
		called := false

		_, err := interceptor(context.Background(), struct{}{}, info,
			func(ctx context.Context, req interface{}) (interface{}, error) {
				called = true
				return nil, nil
			})

		require.Error(t, err)
		assert.False(t, called)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("passes with user id", func(t *testing.T) {
		interceptor := AuthInterceptor()
		ctx := context.WithValue(context.Background(), UserIDKey, "u-1")
		called := false

		resp, err := interceptor(ctx, struct{}{}, info,
			func(ctx context.Context, req interface{}) (interface{}, error) {
				called = true
				return "handled", nil
			})

		require.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, "handled", resp)
	})
}

func TestRequireRoleInterceptor(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}

	tests := []struct {
		name     string
		role     string
		allowed  []string
		wantCode codes.Code
		wantOK   bool
	}{
		{"missing role", "", []string{"admin"}, codes.Unauthenticated, false},
		{"role allowed", "admin", []string{"admin", "user"}, codes.OK, true},
		{"role not allowed", "driver", []string{"admin"}, codes.PermissionDenied, false},
		{"no allowed roles configured", "admin", nil, codes.PermissionDenied, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := RequireRoleInterceptor(tt.allowed...)
			ctx := context.Background()
			if tt.role != "" {
				ctx = context.WithValue(ctx, UserRoleKey, tt.role)
			}

			called := false
			_, err := interceptor(ctx, struct{}{}, info,
				func(ctx context.Context, req interface{}) (interface{}, error) {
					called = true
					return nil, nil
				})

			if tt.wantOK {
				require.NoError(t, err)
				assert.True(t, called)
				return
			}
			require.Error(t, err)
			assert.False(t, called)
			assert.Equal(t, tt.wantCode, status.Code(err))
		})
	}
}

func TestRequirePermissionInterceptor(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}
	ctxWithRole := func(role string) context.Context {
		return context.WithValue(context.Background(), UserRoleKey, role)
	}

	t.Run("no required permissions short circuits", func(t *testing.T) {
		interceptor := RequirePermissionInterceptor()
		called := false

		_, err := interceptor(context.Background(), struct{}{}, info,
			func(ctx context.Context, req interface{}) (interface{}, error) {
				called = true
				return nil, nil
			})

		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("missing role is unauthenticated", func(t *testing.T) {
		interceptor := RequirePermissionInterceptor(constants.PermissionViewDelivery)

		_, err := interceptor(context.Background(), struct{}{}, info,
			func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, nil
			})

		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("granted permission passes", func(t *testing.T) {
		interceptor := RequirePermissionInterceptor(constants.PermissionViewDelivery)
		called := false

		_, err := interceptor(ctxWithRole("admin"), struct{}{}, info,
			func(ctx context.Context, req interface{}) (interface{}, error) {
				called = true
				return nil, nil
			})

		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("missing permission is denied", func(t *testing.T) {
		interceptor := RequirePermissionInterceptor(constants.PermissionDeleteUser)

		_, err := interceptor(ctxWithRole("driver"), struct{}{}, info,
			func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, nil
			})

		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("any of several required permissions is enough", func(t *testing.T) {
		interceptor := RequirePermissionInterceptor(
			constants.PermissionDeleteUser,
			constants.PermissionViewDelivery,
		)
		called := false

		_, err := interceptor(ctxWithRole("driver"), struct{}{}, info,
			func(ctx context.Context, req interface{}) (interface{}, error) {
				called = true
				return nil, nil
			})

		require.NoError(t, err)
		assert.True(t, called)
	})
}

func TestContextGettersWithoutValues(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", GetUserID(ctx))
	assert.Equal(t, "", GetUserRole(ctx))
	assert.Equal(t, "", GetCorrelationID(ctx))
}
