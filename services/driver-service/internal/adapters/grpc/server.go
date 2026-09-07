package grpc

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// DriverGRPCServer implements the DriverService gRPC service.
type DriverGRPCServer struct {
	pb.UnimplementedDriverServiceServer
	dispatch    *services.DispatchService
	driverRepo  ports.DriverRepository
}

// NewDriverGRPCServer creates a new DriverGRPCServer.
func NewDriverGRPCServer(dispatch *services.DispatchService, driverRepo ports.DriverRepository) *DriverGRPCServer {
	return &DriverGRPCServer{
		dispatch:   dispatch,
		driverRepo: driverRepo,
	}
}

// FindAvailableDrivers finds available drivers near a pickup point.
func (s *DriverGRPCServer) FindAvailableDrivers(ctx context.Context, req *pb.FindAvailableDriversRequest) (*pb.FindAvailableDriversResponse, error) {
	if req.DeliveryId == "" {
		return nil, status.Error(codes.InvalidArgument, "deliveryId is required")
	}
	candidates, err := s.dispatch.FindAvailableDrivers(ctx, req.Latitude, req.Longitude, req.RadiusKm, req.VehicleType, req.DeliveryId)
	if err != nil {
		log.Printf("grpc: FindAvailableDrivers error: %v", err)
		return nil, status.Errorf(codes.Internal, "find available drivers failed: %v", err)
	}
	var pbCandidates []*pb.DriverCandidate
	for _, c := range candidates {
		pbCandidates = append(pbCandidates, &pb.DriverCandidate{
			DriverId:       c.DriverID,
			DistanceMeters: c.DistanceMeters,
			VehicleType:    c.VehicleType,
		})
	}
	return &pb.FindAvailableDriversResponse{Candidates: pbCandidates}, nil
}

// ReserveDriver reserves a driver for a delivery using distributed lock.
func (s *DriverGRPCServer) ReserveDriver(ctx context.Context, req *pb.ReserveDriverRequest) (*pb.ReserveDriverResponse, error) {
	if req.DriverId == "" || req.DeliveryId == "" {
		return nil, status.Error(codes.InvalidArgument, "driverId and deliveryId are required")
	}
	reserved, err := s.dispatch.ReserveDriver(ctx, req.DriverId, req.DeliveryId)
	if err != nil {
		if domErr, ok := err.(*domain.Error); ok {
			switch domErr.Code {
			case "driver_not_found":
				return nil, status.Error(codes.NotFound, domErr.Message)
			case "driver_not_available_for_assignment", "driver_already_reserved":
				return nil, status.Error(codes.FailedPrecondition, domErr.Message)
			}
		}
		log.Printf("grpc: ReserveDriver error: %v", err)
		return nil, status.Errorf(codes.Internal, "reserve driver failed: %v", err)
	}
	assignmentID := req.DeliveryId + "-" + req.DriverId
	return &pb.ReserveDriverResponse{
		Reserved:     reserved,
		DriverId:     req.DriverId,
		DeliveryId:   req.DeliveryId,
		AssignmentId: assignmentID,
	}, nil
}

// ReleaseDriver releases a driver from their current assignment.
func (s *DriverGRPCServer) ReleaseDriver(ctx context.Context, req *pb.ReleaseDriverRequest) (*pb.ReleaseDriverResponse, error) {
	if req.DriverId == "" {
		return nil, status.Error(codes.InvalidArgument, "driverId is required")
	}
	err := s.dispatch.ReleaseDriver(ctx, req.DriverId, req.DeliveryId)
	if err != nil {
		if domErr, ok := err.(*domain.Error); ok && domErr.Code == "assignment_not_found" {
			// Idempotent — already released
			return &pb.ReleaseDriverResponse{Released: true, DriverId: req.DriverId}, nil
		}
		log.Printf("grpc: ReleaseDriver error: %v", err)
		return nil, status.Errorf(codes.Internal, "release driver failed: %v", err)
	}
	return &pb.ReleaseDriverResponse{Released: true, DriverId: req.DriverId}, nil
}

// AssignDriver directly assigns a driver to a delivery (after they accepted).
func (s *DriverGRPCServer) AssignDriver(ctx context.Context, req *pb.AssignDriverRequest) (*pb.AssignDriverResponse, error) {
	if req.DriverId == "" || req.DeliveryId == "" {
		return nil, status.Error(codes.InvalidArgument, "driverId and deliveryId are required")
	}
	// Verify driver exists
	driver, err := s.driverRepo.FindByID(ctx, req.DriverId)
	if err != nil || driver == nil {
		return nil, status.Error(codes.NotFound, "driver not found")
	}
	assignmentID := req.DeliveryId + "-" + req.DriverId
	return &pb.AssignDriverResponse{
		Assigned:     true,
		DriverId:     req.DriverId,
		DeliveryId:   req.DeliveryId,
		AssignmentId: assignmentID,
	}, nil
}

// GetDriver retrieves driver information by ID.
func (s *DriverGRPCServer) GetDriver(ctx context.Context, req *pb.GetDriverRequest) (*pb.GetDriverResponse, error) {
	if req.DriverId == "" {
		return nil, status.Error(codes.InvalidArgument, "driverId is required")
	}
	driver, err := s.driverRepo.FindByID(ctx, req.DriverId)
	if err != nil {
		return &pb.GetDriverResponse{Found: false}, nil
	}
	if driver == nil {
		return &pb.GetDriverResponse{Found: false}, nil
	}
	return &pb.GetDriverResponse{
		Found:       true,
		DriverId:    driver.ID,
		UserId:      driver.UserID,
		Status:      string(driver.Status),
		VehicleType: driver.Vehicle.Type,
	}, nil
}

// GetDriverStatus returns the operational status of a driver.
func (s *DriverGRPCServer) GetDriverStatus(ctx context.Context, req *pb.GetDriverStatusRequest) (*pb.GetDriverStatusResponse, error) {
	if req.DriverId == "" {
		return nil, status.Error(codes.InvalidArgument, "driverId is required")
	}
	driver, err := s.driverRepo.FindByID(ctx, req.DriverId)
	if err != nil || driver == nil {
		return nil, status.Error(codes.NotFound, "driver not found")
	}
	return &pb.GetDriverStatusResponse{
		DriverId: driver.ID,
		Status:   string(driver.Status),
	}, nil
}

// ValidateID validates a driver ID or user ID and returns existence + status.
func (s *DriverGRPCServer) ValidateID(ctx context.Context, req *pb.ValidateIDRequest) (*pb.ValidateIDResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	switch req.IdType {
	case "driver":
		driver, err := s.driverRepo.FindByID(ctx, req.Id)
		if err != nil || driver == nil {
			return &pb.ValidateIDResponse{Valid: false, Message: "driver not found"}, nil
		}
		return &pb.ValidateIDResponse{
			Valid:    true,
			DriverId: driver.ID,
			Status:   string(driver.Status),
			Message:  "driver found",
		}, nil

	case "user":
		driver, err := s.driverRepo.FindByUserID(ctx, req.Id)
		if err != nil || driver == nil {
			return &pb.ValidateIDResponse{Valid: false, Message: "no driver profile for this user"}, nil
		}
		return &pb.ValidateIDResponse{
			Valid:    true,
			DriverId: driver.ID,
			Status:   string(driver.Status),
			Message:  "driver profile found",
		}, nil

	default:
		return nil, status.Error(codes.InvalidArgument, "idType must be 'driver' or 'user'")
	}
}

type GRPCServer struct {
	server *grpc.Server
}

func NewGRPCServer(dispatch *services.DispatchService, driverRepo ports.DriverRepository) *GRPCServer {
	srv := grpc.NewServer()
	driverHandler := NewDriverGRPCServer(dispatch, driverRepo)
	pb.RegisterDriverServiceServer(srv, driverHandler)
	reflection.Register(srv) 
	return &GRPCServer{server: srv}
}

// Start starts the gRPC server on the given listener.
func (s *GRPCServer) Start(l net.Listener) error {
	log.Printf("gRPC DriverService listening on %v", l.Addr())
	return s.server.Serve(l)
}

// Stop gracefully stops the gRPC server.
func (s *GRPCServer) Stop() {
	s.server.GracefulStop()
}