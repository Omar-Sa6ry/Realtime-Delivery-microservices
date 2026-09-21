package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/adapters/grpc/proto"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/analytics"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type Server struct {
	pb.UnimplementedAnalyticsServiceServer
	grpcServer       *grpc.Server
	platformService  *analytics.PlatformOverviewService
	driverService    *analytics.DriverAnalyticsService
	port             string
}

func NewServer(
	port string,
	platformService *analytics.PlatformOverviewService,
	driverService *analytics.DriverAnalyticsService,
) *Server {
	s := &Server{
		platformService: platformService,
		driverService:   driverService,
		port:            port,
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAnalyticsServiceServer(grpcServer, s)
	reflection.Register(grpcServer)
	s.grpcServer = grpcServer

	return s
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port %s: %w", s.port, err)
	}

	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

func (s *Server) GetPlatformOverview(ctx context.Context, req *pb.PlatformOverviewRequest) (*pb.PlatformOverviewResponse, error) {
	from, err := parseTime(req.From, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid from time: %v", err)
	}
	to, err := parseTime(req.To, time.Now().UTC())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid to time: %v", err)
	}

	overview, err := s.platformService.GetPlatformOverview(ctx, ports.TimeRange{
		From:        from,
		To:          to,
		Granularity: ports.GranularityDay,
	}, "grpc")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get platform overview: %v", err)
	}

	completionRate := 0.0
	if overview.TotalDeliveries > 0 {
		completionRate = float64(overview.CompletedDeliveries) / float64(overview.TotalDeliveries)
	}

	return &pb.PlatformOverviewResponse{
		TotalDeliveries:     overview.TotalDeliveries,
		CompletedDeliveries: overview.CompletedDeliveries,
		CompletionRate:      completionRate,
		DataAsOf:            time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *Server) GetDriverPerformance(ctx context.Context, req *pb.DriverPerformanceRequest) (*pb.DriverPerformanceResponse, error) {
	if req.DriverId == "" {
		return nil, status.Error(codes.InvalidArgument, "driver_id is required")
	}

	from, err := parseTime(req.From, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid from time: %v", err)
	}
	to, err := parseTime(req.To, time.Now().UTC())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid to time: %v", err)
	}

	perf, err := s.driverService.GetDriverAnalytics(ctx, ports.DriverAnalyticsFilter{
		DriverID: req.DriverId,
		Range: ports.TimeRange{
			From:        from,
			To:          to,
			Granularity: ports.GranularityDay,
		},
	}, "grpc")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get driver performance: %v", err)
	}

	return &pb.DriverPerformanceResponse{
		DriverId:            perf.DriverID,
		Offers:              perf.Offers,
		Accepted:            perf.Accepted,
		AcceptanceRate:      perf.AcceptanceRate,
		CompletedDeliveries: perf.CompletedDeliveries,
	}, nil
}

func parseTime(raw string, def time.Time) (time.Time, error) {
	if raw == "" {
		return def, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}
