package grpc

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/realtime-delivery/payment-service/internal/application/services"
	"github.com/realtime-delivery/payment-service/internal/ports"
)

// GRPCServer wraps the gRPC server.
type GRPCServer struct {
	server   *grpc.Server
	listener net.Listener
	port     string
	service  *services.PaymentService
}

// NewGRPCServer creates a new gRPC server.
func NewGRPCServer(port string, service *services.PaymentService, interceptor grpc.UnaryServerInterceptor) *GRPCServer {
	var opts []grpc.ServerOption
	if interceptor != nil {
		opts = append(opts, grpc.UnaryInterceptor(interceptor))
	}

	server := grpc.NewServer(opts...)
	// payment.RegisterPaymentServiceServer(server, newPaymentServer())
	reflection.Register(server)

	return &GRPCServer{
		server:  server,
		port:    port,
		service: service,
	}
}

// Start starts the gRPC server.
func (s *GRPCServer) Start() error {
	listener, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", s.port, err)
	}
	s.listener = listener
	return s.server.Serve(listener)
}

// Stop stops the gRPC server gracefully.
func (s *GRPCServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

// paymentServer implements the payment protobuf service.
type paymentServer struct {
	// payment.UnimplementedPaymentServiceServer
}

// newPaymentServer creates a new paymentServer.
func newPaymentServer() *paymentServer {
	return &paymentServer{}
}

// CreatePayment creates a new payment.
func (s *paymentServer) CreatePayment(ctx context.Context, req interface{}) (interface{}, error) {
	// TODO: Implement with actual service
	return map[string]interface{}{
		"payment_id": "mock_payment_id",
		"status":     "PENDING",
	}, nil
}

// AuthorizePayment authorizes a payment.
func (s *paymentServer) AuthorizePayment(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"payment_id": "mock_payment_id",
		"status":     "AUTHORIZED",
	}, nil
}

// CapturePayment captures a payment.
func (s *paymentServer) CapturePayment(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"payment_id": "mock_payment_id",
		"status":     "CAPTURED",
	}, nil
}

// CancelAuthorization cancels an authorization.
func (s *paymentServer) CancelAuthorization(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"payment_id": "mock_payment_id",
		"status":     "CANCELLED",
	}, nil
}

// CreateRefund creates a refund.
func (s *paymentServer) CreateRefund(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"refund_id": "mock_refund_id",
		"status":    "SUCCEEDED",
	}, nil
}

// GetPayment gets a payment by ID.
func (s *paymentServer) GetPayment(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"payment": map[string]interface{}{
			"payment_id": "mock_payment_id",
			"status":     "PENDING",
		},
	}, nil
}

// GetPaymentStatus gets a payment status.
func (s *paymentServer) GetPaymentStatus(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"payment_id": "mock_payment_id",
		"status":     "PENDING",
	}, nil
}