package grpc

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	paymentpb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/grpc/proto"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
)

type GRPCServer struct {
	server   *grpc.Server
	port     string
	service  *services.PaymentService
}

func NewGRPCServer(port string, service *services.PaymentService, interceptor grpc.UnaryServerInterceptor) *GRPCServer {
	var opts []grpc.ServerOption
	if interceptor != nil {
		opts = append(opts, grpc.UnaryInterceptor(interceptor))
	}

	server := grpc.NewServer(opts...)
	paymentpb.RegisterPaymentServiceServer(server, newPaymentServer(service))
	reflection.Register(server)

	return &GRPCServer{
		server:  server,
		port:    port,
		service: service,
	}
}

func (s *GRPCServer) Start(lis net.Listener) error {
	return s.server.Serve(lis)
}

func (s *GRPCServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

type paymentServer struct {
	paymentpb.UnimplementedPaymentServiceServer
	service *services.PaymentService
}

func newPaymentServer(service *services.PaymentService) *paymentServer {
	return &paymentServer{service: service}
}

func (s *paymentServer) CreatePayment(ctx context.Context, req *paymentpb.CreatePaymentRequest) (*paymentpb.CreatePaymentResponse, error) {
	res, err := s.service.CreatePayment(ctx, services.CreatePaymentInput{
		DeliveryID:     req.GetDeliveryId(),
		UserID:         req.GetUserId(),
		AmountMinor:    req.GetAmountMinor(),
		Currency:       req.GetCurrency(),
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create payment failed: %v", err)
	}

	return &paymentpb.CreatePaymentResponse{
		PaymentId:        &paymentpb.PaymentID{Value: res.PaymentID},
		Status:           string(res.Status),
		PaymentLink:      res.CheckoutURL,
		GatewayReference: res.ClientSecret,
	}, nil
}

func (s *paymentServer) AuthorizePayment(ctx context.Context, req *paymentpb.AuthorizePaymentRequest) (*paymentpb.AuthorizePaymentResponse, error) {
	payment, err := s.service.AuthorizePayment(ctx, services.AuthorizePaymentInput{
		PaymentID:      req.GetPaymentId(),
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "authorize payment failed: %v", err)
	}

	return &paymentpb.AuthorizePaymentResponse{
		PaymentId:       &paymentpb.PaymentID{Value: payment.ID},
		AuthorizationId: payment.ProviderPaymentID,
		Status:          string(payment.Status),
	}, nil
}

func (s *paymentServer) CapturePayment(ctx context.Context, req *paymentpb.CapturePaymentRequest) (*paymentpb.CapturePaymentResponse, error) {
	payment, err := s.service.CapturePayment(ctx, services.CapturePaymentInput{
		PaymentID:      req.GetPaymentId(),
		AmountMinor:    req.GetAmountMinor(),
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "capture payment failed: %v", err)
	}

	return &paymentpb.CapturePaymentResponse{
		PaymentId: &paymentpb.PaymentID{Value: payment.ID},
		CaptureId: payment.ProviderPaymentID,
		Status:    string(payment.Status),
	}, nil
}

func (s *paymentServer) CancelAuthorization(ctx context.Context, req *paymentpb.CancelAuthorizationRequest) (*paymentpb.CancelAuthorizationResponse, error) {
	payment, err := s.service.CancelAuthorization(ctx, req.GetPaymentId(), req.GetIdempotencyKey())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cancel authorization failed: %v", err)
	}

	return &paymentpb.CancelAuthorizationResponse{
		PaymentId: &paymentpb.PaymentID{Value: payment.ID},
		Status:    string(payment.Status),
	}, nil
}

func (s *paymentServer) CreateRefund(ctx context.Context, req *paymentpb.CreateRefundRequest) (*paymentpb.CreateRefundResponse, error) {
	refund, err := s.service.CreateRefund(ctx, services.CreateRefundInput{
		PaymentID:      req.GetPaymentId(),
		AmountMinor:    req.GetAmountMinor(),
		Reason:         req.GetReason(),
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create refund failed: %v", err)
	}

	return &paymentpb.CreateRefundResponse{
		PaymentId: &paymentpb.PaymentID{Value: refund.PaymentID},
		RefundId:  refund.ID,
		Status:    string(refund.Status),
	}, nil
}

func (s *paymentServer) GetPayment(ctx context.Context, req *paymentpb.GetPaymentRequest) (*paymentpb.GetPaymentResponse, error) {
	payment, err := s.service.GetPayment(ctx, req.GetPaymentId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "payment not found: %v", err)
	}

	var completedAt string
	if payment.CapturedAt != nil {
		completedAt = payment.CapturedAt.Format(time.RFC3339)
	}

	return &paymentpb.GetPaymentResponse{
		PaymentId:        &paymentpb.PaymentID{Value: payment.ID},
		DeliveryId:       payment.DeliveryID,
		UserId:           payment.UserID,
		AmountMinor:      payment.AmountMinor,
		Currency:         payment.Currency,
		Status:           string(payment.Status),
		GatewayPaymentId: payment.ProviderPaymentID,
		CreatedAt:        payment.CreatedAt.Format(time.RFC3339),
		CompletedAt:      completedAt,
	}, nil
}

func (s *paymentServer) GetPaymentStatus(ctx context.Context, req *paymentpb.GetPaymentStatusRequest) (*paymentpb.GetPaymentStatusResponse, error) {
	statusVal, err := s.service.GetPaymentStatus(ctx, req.GetPaymentId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "payment not found: %v", err)
	}

	return &paymentpb.GetPaymentStatusResponse{
		PaymentId: &paymentpb.PaymentID{Value: req.GetPaymentId()},
		Status:    string(statusVal),
	}, nil
}
