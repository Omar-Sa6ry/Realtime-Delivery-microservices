package metrics

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerMetricsInterceptor captures and logs prometheus metrics for unary gRPC requests
func UnaryServerMetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		startTime := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(startTime).Seconds()

		st, ok := status.FromError(err)
		statusCode := "UNKNOWN"
		if ok {
			statusCode = st.Code().String()
		}

		// Record request count and duration
		RequestCounter.WithLabelValues("gRPC", "RPC", info.FullMethod, statusCode).Inc()
		RequestDuration.WithLabelValues("gRPC", "RPC", info.FullMethod, statusCode).Observe(duration)

		// Record error metric if there is an error
		if err != nil {
			ErrorCounter.WithLabelValues("gRPC:"+info.FullMethod, statusCode).Inc()
		}

		return resp, err
	}
}

// StreamServerMetricsInterceptor captures and logs prometheus metrics for stream gRPC requests
func StreamServerMetricsInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		startTime := time.Now()

		err := handler(srv, ss)

		duration := time.Since(startTime).Seconds()

		st, ok := status.FromError(err)
		statusCode := "UNKNOWN"
		if ok {
			statusCode = st.Code().String()
		}

		// Record request count and duration
		RequestCounter.WithLabelValues("gRPC", "STREAM", info.FullMethod, statusCode).Inc()
		RequestDuration.WithLabelValues("gRPC", "STREAM", info.FullMethod, statusCode).Observe(duration)

		// Record error metric if there is an error
		if err != nil {
			ErrorCounter.WithLabelValues("gRPC:"+info.FullMethod, statusCode).Inc()
		}

		return err
	}
}
