package grpc

import (
	"context"
	"log"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/i18n"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// extractLanguage extracts the client's language preference from gRPC metadata.
func extractLanguage(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if langs := md.Get("x-lang"); len(langs) > 0 && langs[0] != "" {
			return i18n.NormalizeLang(langs[0])
		}
		if langs := md.Get("accept-language"); len(langs) > 0 && langs[0] != "" {
			return i18n.NormalizeLang(langs[0])
		}
	}
	return i18n.FromContext(ctx)
}

// DriverGRPCServer implements the DriverService gRPC service.
type DriverGRPCServer struct {
	pb.UnimplementedDriverServiceServer
	dispatch   *services.DispatchService
	driverRepo ports.DriverRepository
}

// NewDriverGRPCServer creates a new DriverGRPCServer.
func NewDriverGRPCServer(dispatch *services.DispatchService, driverRepo ports.DriverRepository) *DriverGRPCServer {
	return &DriverGRPCServer{
		dispatch:   dispatch,
		driverRepo: driverRepo,
	}
}

// translateError maps domain errors to translated gRPC status errors.
func translateError(lang string, err error) error {
	if err == nil {
		return nil
	}
	if domErr, ok := err.(*domain.Error); ok {
		key := "error.internal"
		code := codes.Internal
		switch domErr.Code {
		case "driver_not_found":
			key = "driver.not_found"
			code = codes.NotFound
		case "driver_not_available_for_assignment", "driver_not_available":
			key = "driver.not_available"
			code = codes.FailedPrecondition
		case "driver_already_reserved":
			key = "driver.already_reserved"
			code = codes.FailedPrecondition
		case "driver_already_online":
			key = "driver.already_online"
			code = codes.FailedPrecondition
		case "driver_already_offline":
			key = "driver.already_offline"
			code = codes.FailedPrecondition
		case "assignment_not_found":
			key = "assignment.not_found"
			code = codes.NotFound
		case "assignment_invalid_state":
			key = "assignment.already_handled"
			code = codes.FailedPrecondition
		case "assignment_expired":
			key = "assignment.expired"
			code = codes.FailedPrecondition
		case "invalid_argument":
			key = "error.internal"
			code = codes.InvalidArgument
		}
		return status.Error(code, i18n.T(lang, key))
	}
	return status.Errorf(codes.Internal, "%s: %v", i18n.T(lang, "error.internal"), err)
}

func toProtoVehicleType(vt domain.VehicleType) pb.VehicleType {
	switch vt {
	case domain.VehicleTypeCar:
		return pb.VehicleType_CAR
	case domain.VehicleTypeMotorcycle:
		return pb.VehicleType_MOTORCYCLE
	case domain.VehicleTypeTruck:
		return pb.VehicleType_TRUCK
	case domain.VehicleTypeBicycle:
		return pb.VehicleType_BICYCLE
	case domain.VehicleTypeVan:
		return pb.VehicleType_VAN
	default:
		return pb.VehicleType_VEHICLE_TYPE_UNKNOWN
	}
}

// FindAvailableDrivers finds available drivers near a pickup point.
func (s *DriverGRPCServer) FindAvailableDrivers(ctx context.Context, req *pb.FindAvailableDriversRequest) (*pb.FindAvailableDriversResponse, error) {
	lang := extractLanguage(ctx)
	if req.DeliveryId == "" {
		return nil, status.Error(codes.InvalidArgument, i18n.T(lang, "validation.delivery_id_required"))
	}
	var vType domain.VehicleType
	switch req.VehicleType {
	case pb.VehicleType_CAR:
		vType = domain.VehicleTypeCar
	case pb.VehicleType_MOTORCYCLE:
		vType = domain.VehicleTypeMotorcycle
	case pb.VehicleType_TRUCK:
		vType = domain.VehicleTypeTruck
	case pb.VehicleType_BICYCLE:
		vType = domain.VehicleTypeBicycle
	case pb.VehicleType_VAN:
		vType = domain.VehicleTypeVan
	default:
		vType = ""
	}

	candidates, err := s.dispatch.FindAvailableDrivers(ctx, req.Latitude, req.Longitude, req.RadiusKm, vType, req.DeliveryId)
	if err != nil {
		log.Printf("grpc: FindAvailableDrivers error: %v", err)
		return nil, translateError(lang, err)
	}
	var pbCandidates []*pb.DriverCandidate
	for _, c := range candidates {
		pbCandidates = append(pbCandidates, &pb.DriverCandidate{
			DriverId:       c.DriverID,
			DistanceMeters: c.DistanceMeters,
			VehicleType:    toProtoVehicleType(c.VehicleType),
		})
	}
	return &pb.FindAvailableDriversResponse{Candidates: pbCandidates}, nil
}

// ReserveDriver reserves a driver for a delivery using distributed lock.
func (s *DriverGRPCServer) ReserveDriver(ctx context.Context, req *pb.ReserveDriverRequest) (*pb.ReserveDriverResponse, error) {
	lang := extractLanguage(ctx)
	if req.DriverId == "" {
		return nil, status.Error(codes.InvalidArgument, i18n.T(lang, "validation.driver_id_required"))
	}
	if req.DeliveryId == "" {
		return nil, status.Error(codes.InvalidArgument, i18n.T(lang, "validation.delivery_id_required"))
	}

	reserved, err := s.dispatch.ReserveDriver(ctx, req.DriverId, req.DeliveryId)
	if err != nil {
		log.Printf("grpc: ReserveDriver error: %v", err)
		return nil, translateError(lang, err)
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
	lang := extractLanguage(ctx)
	if req.DriverId == "" {
		return nil, status.Error(codes.InvalidArgument, i18n.T(lang, "validation.driver_id_required"))
	}
	err := s.dispatch.ReleaseDriver(ctx, req.DriverId, req.DeliveryId)
	if err != nil {
		if domErr, ok := err.(*domain.Error); ok && domErr.Code == "assignment_not_found" {
			// Idempotent — already released
			return &pb.ReleaseDriverResponse{Released: true, DriverId: req.DriverId}, nil
		}
		log.Printf("grpc: ReleaseDriver error: %v", err)
		return nil, translateError(lang, err)
	}
	return &pb.ReleaseDriverResponse{Released: true, DriverId: req.DriverId}, nil
}

// AssignDriver directly assigns a driver to a delivery (after acceptance).
func (s *DriverGRPCServer) AssignDriver(ctx context.Context, req *pb.AssignDriverRequest) (*pb.AssignDriverResponse, error) {
	lang := extractLanguage(ctx)
	if req.DriverId == "" {
		return nil, status.Error(codes.InvalidArgument, i18n.T(lang, "validation.driver_id_required"))
	}
	if req.DeliveryId == "" {
		return nil, status.Error(codes.InvalidArgument, i18n.T(lang, "validation.delivery_id_required"))
	}

	reserved, err := s.dispatch.ReserveDriver(ctx, req.DriverId, req.DeliveryId)
	if err != nil {
		log.Printf("grpc: AssignDriver error: %v", err)
		return nil, translateError(lang, err)
	}

	assignmentID := req.DeliveryId + "-" + req.DriverId
	return &pb.AssignDriverResponse{
		Assigned:     reserved,
		DriverId:     req.DriverId,
		DeliveryId:   req.DeliveryId,
		AssignmentId: assignmentID,
	}, nil
}

// GetDriver returns driver info by driver ID.
func (s *DriverGRPCServer) GetDriver(ctx context.Context, req *pb.GetDriverRequest) (*pb.GetDriverResponse, error) {
	lang := extractLanguage(ctx)
	if req.DriverId == "" {
		return nil, status.Error(codes.InvalidArgument, i18n.T(lang, "validation.driver_id_required"))
	}
	driver, err := s.driverRepo.FindByID(ctx, req.DriverId)
	if err != nil || driver == nil {
		return &pb.GetDriverResponse{Found: false, DriverId: req.DriverId}, nil
	}
	return &pb.GetDriverResponse{
		Found:       true,
		DriverId:    driver.ID,
		UserId:      driver.UserID,
		Status:      string(driver.Status),
		VehicleType: toProtoVehicleType(driver.Vehicle.Type),
		IsBlocked:   driver.IsBlocked,
	}, nil
}

// GetDriverStatus returns the current operational status of a driver.
func (s *DriverGRPCServer) GetDriverStatus(ctx context.Context, req *pb.GetDriverStatusRequest) (*pb.GetDriverStatusResponse, error) {
	lang := extractLanguage(ctx)
	if req.DriverId == "" {
		return nil, status.Error(codes.InvalidArgument, i18n.T(lang, "validation.driver_id_required"))
	}
	driver, err := s.driverRepo.FindByID(ctx, req.DriverId)
	if err != nil || driver == nil {
		return nil, status.Error(codes.NotFound, i18n.T(lang, "driver.not_found"))
	}
	return &pb.GetDriverStatusResponse{
		DriverId: driver.ID,
		Status:   string(driver.Status),
	}, nil
}

// ValidateID validates a driver ID or user ID and returns existence + status.
func (s *DriverGRPCServer) ValidateID(ctx context.Context, req *pb.ValidateIDRequest) (*pb.ValidateIDResponse, error) {
	lang := extractLanguage(ctx)
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, i18n.T(lang, "validation.id_required"))
	}

	switch strings.ToLower(req.IdType) {
	case "driver":
		driver, err := s.driverRepo.FindByID(ctx, req.Id)
		if err != nil || driver == nil {
			return &pb.ValidateIDResponse{
				Valid:   false,
				Message: i18n.T(lang, "driver.not_found"),
			}, nil
		}
		return &pb.ValidateIDResponse{
			Valid:    true,
			DriverId: driver.ID,
			Status:   string(driver.Status),
			Message:  i18n.T(lang, "driver.found"),
		}, nil

	case "user":
		driver, err := s.driverRepo.FindByUserID(ctx, req.Id)
		if err != nil || driver == nil {
			return &pb.ValidateIDResponse{
				Valid:   false,
				Message: i18n.T(lang, "driver.no_profile_for_user"),
			}, nil
		}
		return &pb.ValidateIDResponse{
			Valid:    true,
			DriverId: driver.ID,
			Status:   string(driver.Status),
			Message:  i18n.T(lang, "driver.profile_found"),
		}, nil

	default:
		return nil, status.Error(codes.InvalidArgument, i18n.T(lang, "validation.invalid_id_type"))
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

func (g *GRPCServer) Start(lis net.Listener) error {
	return g.server.Serve(lis)
}

func (g *GRPCServer) Stop() {
	g.server.GracefulStop()
}