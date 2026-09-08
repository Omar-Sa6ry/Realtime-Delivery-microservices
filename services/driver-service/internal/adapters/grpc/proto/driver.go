// Code generated manually from driver.proto — do NOT edit.
// This package mirrors the proto definitions for internal use.
// When protoc is available, regenerate with: protoc --go_out=. --go-grpc_out=. driver.proto

package proto

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─── Enums ─────────────────────────────────────────────────────────────────

type VehicleType int32

const (
	VehicleType_VEHICLE_TYPE_UNKNOWN VehicleType = 0
	VehicleType_CAR                  VehicleType = 1
	VehicleType_MOTORCYCLE           VehicleType = 2
	VehicleType_TRUCK                VehicleType = 3
	VehicleType_BICYCLE              VehicleType = 4
	VehicleType_VAN                  VehicleType = 5
)

// ─── Messages ──────────────────────────────────────────────────────────────

type FindAvailableDriversRequest struct {
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	RadiusKm    float64 `json:"radiusKm"`
	VehicleType VehicleType `json:"vehicleType"`
	DeliveryId  string  `json:"deliveryId"`
	CorrelationId string `json:"correlationId"`
}

type DriverCandidate struct {
	DriverId       string  `json:"driverId"`
	DistanceMeters float64 `json:"distanceMeters"`
	VehicleType    VehicleType  `json:"vehicleType"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
}

type FindAvailableDriversResponse struct {
	Candidates []*DriverCandidate `json:"candidates"`
}

type ReserveDriverRequest struct {
	DriverId       string `json:"driverId"`
	DeliveryId     string `json:"deliveryId"`
	IdempotencyKey string `json:"idempotencyKey"`
	CorrelationId  string `json:"correlationId"`
}

type ReserveDriverResponse struct {
	Reserved     bool   `json:"reserved"`
	DriverId     string `json:"driverId"`
	DeliveryId   string `json:"deliveryId"`
	AssignmentId string `json:"assignmentId"`
}

type ReleaseDriverRequest struct {
	DriverId      string `json:"driverId"`
	DeliveryId    string `json:"deliveryId"`
	Reason        string `json:"reason"`
	CorrelationId string `json:"correlationId"`
}

type ReleaseDriverResponse struct {
	Released bool   `json:"released"`
	DriverId string `json:"driverId"`
}

type AssignDriverRequest struct {
	DriverId      string `json:"driverId"`
	DeliveryId    string `json:"deliveryId"`
	CorrelationId string `json:"correlationId"`
}

type AssignDriverResponse struct {
	Assigned     bool   `json:"assigned"`
	DriverId     string `json:"driverId"`
	DeliveryId   string `json:"deliveryId"`
	AssignmentId string `json:"assignmentId"`
}

type GetDriverRequest struct {
	DriverId string `json:"driverId"`
}

type GetDriverResponse struct {
	Found       bool   `json:"found"`
	DriverId    string `json:"driverId"`
	UserId      string `json:"userId"`
	Status      string `json:"status"`
	VehicleType VehicleType `json:"vehicleType"`
}

type GetDriverStatusRequest struct {
	DriverId string `json:"driverId"`
}

type GetDriverStatusResponse struct {
	DriverId            string `json:"driverId"`
	Status              string `json:"status"`
	HasActiveAssignment bool   `json:"hasActiveAssignment"`
	ActiveDeliveryId    string `json:"activeDeliveryId"`
}

type ValidateIDRequest struct {
	Id      string `json:"id"`
	IdType  string `json:"idType"`   // "driver" | "user"
	Context string `json:"context"`
}

type ValidateIDResponse struct {
	Valid    bool   `json:"valid"`
	DriverId string `json:"driverId"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

// ─── Service Interface ─────────────────────────────────────────────────────

// DriverServiceServer is the server-side interface to implement.
type DriverServiceServer interface {
	FindAvailableDrivers(context.Context, *FindAvailableDriversRequest) (*FindAvailableDriversResponse, error)
	ReserveDriver(context.Context, *ReserveDriverRequest) (*ReserveDriverResponse, error)
	ReleaseDriver(context.Context, *ReleaseDriverRequest) (*ReleaseDriverResponse, error)
	AssignDriver(context.Context, *AssignDriverRequest) (*AssignDriverResponse, error)
	GetDriver(context.Context, *GetDriverRequest) (*GetDriverResponse, error)
	GetDriverStatus(context.Context, *GetDriverStatusRequest) (*GetDriverStatusResponse, error)
	ValidateID(context.Context, *ValidateIDRequest) (*ValidateIDResponse, error)
}

// UnimplementedDriverServiceServer provides default no-op implementations.
type UnimplementedDriverServiceServer struct{}

func (UnimplementedDriverServiceServer) FindAvailableDrivers(_ context.Context, _ *FindAvailableDriversRequest) (*FindAvailableDriversResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FindAvailableDrivers not implemented")
}
func (UnimplementedDriverServiceServer) ReserveDriver(_ context.Context, _ *ReserveDriverRequest) (*ReserveDriverResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReserveDriver not implemented")
}
func (UnimplementedDriverServiceServer) ReleaseDriver(_ context.Context, _ *ReleaseDriverRequest) (*ReleaseDriverResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReleaseDriver not implemented")
}
func (UnimplementedDriverServiceServer) AssignDriver(_ context.Context, _ *AssignDriverRequest) (*AssignDriverResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AssignDriver not implemented")
}
func (UnimplementedDriverServiceServer) GetDriver(_ context.Context, _ *GetDriverRequest) (*GetDriverResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDriver not implemented")
}
func (UnimplementedDriverServiceServer) GetDriverStatus(_ context.Context, _ *GetDriverStatusRequest) (*GetDriverStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDriverStatus not implemented")
}
func (UnimplementedDriverServiceServer) ValidateID(_ context.Context, _ *ValidateIDRequest) (*ValidateIDResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateID not implemented")
}

// ─── Server Registration ───────────────────────────────────────────────────

const _DriverService_serviceDesc_name = "driver.DriverService"

// RegisterDriverServiceServer registers the server implementation.
func RegisterDriverServiceServer(s *grpc.Server, srv DriverServiceServer) {
	s.RegisterService(&_DriverService_serviceDesc, srv)
}

var _DriverService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "driver.DriverService",
	HandlerType: (*DriverServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "FindAvailableDrivers", Handler: _DriverService_FindAvailableDrivers_Handler},
		{MethodName: "ReserveDriver", Handler: _DriverService_ReserveDriver_Handler},
		{MethodName: "ReleaseDriver", Handler: _DriverService_ReleaseDriver_Handler},
		{MethodName: "AssignDriver", Handler: _DriverService_AssignDriver_Handler},
		{MethodName: "GetDriver", Handler: _DriverService_GetDriver_Handler},
		{MethodName: "GetDriverStatus", Handler: _DriverService_GetDriverStatus_Handler},
		{MethodName: "ValidateID", Handler: _DriverService_ValidateID_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "driver.proto",
}

func _DriverService_FindAvailableDrivers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FindAvailableDriversRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DriverServiceServer).FindAvailableDrivers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/driver.DriverService/FindAvailableDrivers"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DriverServiceServer).FindAvailableDrivers(ctx, req.(*FindAvailableDriversRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DriverService_ReserveDriver_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReserveDriverRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DriverServiceServer).ReserveDriver(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/driver.DriverService/ReserveDriver"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DriverServiceServer).ReserveDriver(ctx, req.(*ReserveDriverRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DriverService_ReleaseDriver_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReleaseDriverRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DriverServiceServer).ReleaseDriver(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/driver.DriverService/ReleaseDriver"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DriverServiceServer).ReleaseDriver(ctx, req.(*ReleaseDriverRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DriverService_AssignDriver_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AssignDriverRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DriverServiceServer).AssignDriver(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/driver.DriverService/AssignDriver"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DriverServiceServer).AssignDriver(ctx, req.(*AssignDriverRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DriverService_GetDriver_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetDriverRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DriverServiceServer).GetDriver(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/driver.DriverService/GetDriver"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DriverServiceServer).GetDriver(ctx, req.(*GetDriverRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DriverService_GetDriverStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetDriverStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DriverServiceServer).GetDriverStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/driver.DriverService/GetDriverStatus"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DriverServiceServer).GetDriverStatus(ctx, req.(*GetDriverStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DriverService_ValidateID_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateIDRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DriverServiceServer).ValidateID(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/driver.DriverService/ValidateID"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DriverServiceServer).ValidateID(ctx, req.(*ValidateIDRequest))
	}
	return interceptor(ctx, in, info, handler)
}
