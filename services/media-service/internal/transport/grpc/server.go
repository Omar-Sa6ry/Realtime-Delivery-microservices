package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	sharedmiddleware "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/middleware"
	sharedmetrics "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/metrics"
	sharedratelimiter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/ratelimiter"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/download"
	appMedia "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/media"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/upload"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
	mediav1 "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/proto/media/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Handler wires all use cases into a gRPC-compatible structure.
// It does NOT directly implement a generated proto server interface — instead it acts as
// a unified handler that the transport layer delegates to. When the proto is generated
// via `make proto`, the transport handlers embed this struct.
type Handler struct {
	mediav1.UnimplementedMediaServiceServer
	createSession   *upload.CreateSessionUseCase
	completeUpload  *upload.CompleteUploadUseCase
	abortUpload     *upload.AbortUploadUseCase
	getUploadStatus *upload.GetUploadStatusUseCase
	getMedia        *appMedia.GetMediaUseCase
	listMedia       *appMedia.ListMediaUseCase
	deleteMedia     *appMedia.DeleteMediaUseCase
	getDownloadURL  *download.GetDownloadUrlUseCase
	quotaRepo       ports.QuotaRepository
}

// NewHandler constructs the gRPC handler with all use cases.
func NewHandler(
	createSession *upload.CreateSessionUseCase,
	completeUpload *upload.CompleteUploadUseCase,
	abortUpload *upload.AbortUploadUseCase,
	getUploadStatus *upload.GetUploadStatusUseCase,
	getMedia *appMedia.GetMediaUseCase,
	listMedia *appMedia.ListMediaUseCase,
	deleteMedia *appMedia.DeleteMediaUseCase,
	getDownloadURL *download.GetDownloadUrlUseCase,
	quotaRepo ports.QuotaRepository,
) *Handler {
	return &Handler{
		createSession:   createSession,
		completeUpload:  completeUpload,
		abortUpload:     abortUpload,
		getUploadStatus: getUploadStatus,
		getMedia:        getMedia,
		listMedia:       listMedia,
		deleteMedia:     deleteMedia,
		getDownloadURL:  getDownloadURL,
		quotaRepo:       quotaRepo,
	}
}

// Server is the production gRPC server for the media service.
type Server struct {
	grpcServer *grpc.Server
	handler    *Handler
	port       string
}

// NewServer assembles a gRPC server with the full interceptor chain.
func NewServer(handler *Handler, rateLimiter *sharedratelimiter.RateLimiter, port string) *Server {
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			// 1. Extract x-user-id, x-user-role, x-correlation-id from metadata
			sharedmiddleware.UnaryServerMetadataInterceptor(),
			// 2. Logging and metrics (from shared package)
			loggingInterceptor(),
			metricsInterceptor(),
			// 3. Rate limiting (uses shared Redis sliding window)
			rateLimiter.UnaryServerInterceptor(nil),
			// 4. Authentication guard — must have x-user-id
			sharedmiddleware.AuthInterceptor(),
		),
	)

	// Register the media service handler
	mediav1.RegisterMediaServiceServer(grpcServer, handler)

	// Health check service — used by Kubernetes liveness/readiness probes
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("media.v1.MediaService", grpc_health_v1.HealthCheckResponse_SERVING)

	// Enable server reflection for debugging with grpcurl
	reflection.Register(grpcServer)

	return &Server{grpcServer: grpcServer, handler: handler, port: port}
}

// Serve starts listening on the configured port. Blocks until an error occurs.
func (s *Server) Serve(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
	if err != nil {
		return fmt.Errorf("gRPC listen on port %s: %w", s.port, err)
	}

	slog.Info("gRPC server starting", "port", s.port)

	go func() {
		<-ctx.Done()
		slog.Info("gRPC server: graceful stop initiated")
		s.grpcServer.GracefulStop()
	}()

	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("gRPC serve: %w", err)
	}
	return nil
}

// loggingInterceptor logs every incoming gRPC call with its result.
func loggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			st, _ := status.FromError(err)
			slog.Error("gRPC call failed",
				"method", info.FullMethod,
				"code", st.Code().String(),
				"message", st.Message(),
			)
		} else {
			slog.Debug("gRPC call succeeded", "method", info.FullMethod)
		}
		return resp, err
	}
}

// metricsInterceptor records request counts and durations using the shared metrics package.
func metricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		statusCode := codes.OK.String()
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code().String()
			}
		}
		sharedmetrics.RequestCounter.WithLabelValues("grpc", "UNARY", info.FullMethod, statusCode).Inc()
		return resp, err
	}
}
