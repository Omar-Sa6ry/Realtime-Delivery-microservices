package grpc

import (
	"context"
	"net"

	sharedconstants "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/constants"
	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryInterceptor extracts identity + correlation headers from gRPC metadata into
// the request context so that logging/metrics and use-cases keep the same contract
// as the GraphQL path (which reads x-user-id from HTTP headers).
func UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	requestID := ""
	userID := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(metadataRequesterID); len(vals) > 0 {
			userID = vals[0]
		}
		if vals := md.Get(sharedconstants.HeaderXUserId); len(vals) > 0 {
			userID = vals[0]
		}
		if vals := md.Get(metadataTraceID); len(vals) > 0 {
			requestID = vals[0]
		}
		if vals := md.Get(sharedconstants.HeaderXCorrelationId); len(vals) > 0 {
			requestID = vals[0]
		}
	}

	ctx = sharedlogging.WithLogContext(ctx, sharedlogging.LogContext{
		TraceID: requestID,
		UserID:  userID,
		Method:  info.FullMethod,
		Path:    info.FullMethod,
	})

	return handler(ctx, req)
}

// NewListener binds a plain TCP listener for the gRPC server on addr.
func NewListener(addr string) (net.Listener, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return lis, nil
}

// NewServerOptions returns the shared gRPC server options (unary interceptor enabled).
func NewServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(UnaryInterceptor),
	}
}