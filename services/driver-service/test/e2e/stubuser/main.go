// Command stubuser is a minimal stand-in for the user-service gRPC API used by
// the driver-service end-to-end suite. It answers GetUser / UpdateUserRole so
// the register-driver flow can complete without a live user-service.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto"
	"google.golang.org/grpc"
)

type stubServer struct {
	proto.UnimplementedUserServiceServer

	mu    sync.Mutex
	roles map[string]string
}

func (s *stubServer) GetUser(_ context.Context, req *proto.GetUserRequest) (*proto.GetUserResponse, error) {
	if req.GetId() == "" {
		return nil, fmt.Errorf("missing user id")
	}
	s.mu.Lock()
	role := s.roles[req.GetId()]
	s.mu.Unlock()
	if role == "" {
		role = "customer"
	}
	return &proto.GetUserResponse{
		Id:       req.GetId(),
		Email:    req.GetId() + "@e2e.test",
		Role:     role,
		FirstName: "E2E",
		LastName:  "Driver",
		IsActive:  true,
	}, nil
}

func (s *stubServer) GetUserByEmail(_ context.Context, req *proto.GetUserByEmailRequest) (*proto.GetUserResponse, error) {
	return &proto.GetUserResponse{Id: "user-from-email", Email: req.GetEmail(), Role: "customer", IsActive: true}, nil
}

func (s *stubServer) UpdateUserRole(_ context.Context, req *proto.UpdateUserRoleRequest) (*proto.UpdateUserRoleResponse, error) {
	if req.GetUserId() == "" {
		return &proto.UpdateUserRoleResponse{Success: false, Message: "missing user id"}, nil
	}
	s.mu.Lock()
	s.roles[req.GetUserId()] = req.GetRole()
	s.mu.Unlock()
	return &proto.UpdateUserRoleResponse{Success: true, Message: "role updated"}, nil
}

func (s *stubServer) ValidateToken(_ context.Context, req *proto.ValidateTokenRequest) (*proto.ValidateTokenResponse, error) {
	return &proto.ValidateTokenResponse{Valid: true, UserId: "e2e-user", Role: "customer"}, nil
}

func (s *stubServer) GetUserPermissions(_ context.Context, _ *proto.GetUserPermissionsRequest) (*proto.GetUserPermissionsResponse, error) {
	return &proto.GetUserPermissionsResponse{Permissions: []string{"driver:read"}}, nil
}

// deliveryStub answers the delivery-service RPCs the driver-service calls so
// flows like rate-driver and available-deliveries can be exercised end to end.
type deliveryStub struct {
	proto.UnimplementedDeliveryServiceServer
}

func (d *deliveryStub) IsParticipant(_ context.Context, _ *proto.ParticipantRequest) (*proto.ParticipantResponse, error) {
	return &proto.ParticipantResponse{IsParticipant: true}, nil
}

func (d *deliveryStub) GetDelivery(_ context.Context, req *proto.GetDeliveryRequest) (*proto.GetDeliveryResponse, error) {
	// End-to-end delivery ids are encoded as "e2e:<driverID>:<customerID>:<n>"
	// so ownership checks can be exercised without a real delivery-service.
	customerID, driverID := "e2e-customer", ""
	if parts := strings.SplitN(req.GetDeliveryId(), ":", 4); len(parts) == 4 && parts[0] == "e2e" {
		driverID, customerID = parts[1], parts[2]
	}
	return &proto.GetDeliveryResponse{
		Found:      true,
		DeliveryId: req.GetDeliveryId(),
		CustomerId: customerID,
		DriverId:   driverID,
		Status:     "DELIVERED",
		Amount:     42,
		Currency:   "EGP",
	}, nil
}

func (d *deliveryStub) RefundPayment(_ context.Context, req *proto.RefundPaymentRequest) (*proto.RefundPaymentResponse, error) {
	return &proto.RefundPaymentResponse{Accepted: true, DeliveryId: req.GetDeliveryId(), RefundId: "refund-e2e"}, nil
}

func (d *deliveryStub) GetOpenDeliveries(_ context.Context, _ *proto.GetOpenDeliveriesRequest) (*proto.GetOpenDeliveriesResponse, error) {
	return &proto.GetOpenDeliveriesResponse{Items: nil, TotalItems: 0}, nil
}

func main() {
	addr := flag.String("addr", "127.0.0.1:50051", "listen address")
	flag.Parse()

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("stubuser: listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterUserServiceServer(grpcServer, &stubServer{roles: map[string]string{}})
	proto.RegisterDeliveryServiceServer(grpcServer, &deliveryStub{})

	log.Printf("stubuser: listening on %s", *addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("stubuser: serve: %v", err)
	}
}
