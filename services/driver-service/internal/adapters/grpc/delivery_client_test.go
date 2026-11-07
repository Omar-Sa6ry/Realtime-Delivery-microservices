package grpc_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	drivergrpc "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc"
	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// fakeDeliveryServer is an in-process DeliveryService used to exercise the
// adapter through a real gRPC connection.
type fakeDeliveryServer struct {
	pb.UnimplementedDeliveryServiceServer

	mu sync.Mutex

	getDeliveryResp    *pb.GetDeliveryResponse
	getDeliveryErr     error
	openDeliveriesResp *pb.GetOpenDeliveriesResponse
	openDeliveriesErr  error

	lastGetDeliveryReq *pb.GetDeliveryRequest
	lastOpenReq        *pb.GetOpenDeliveriesRequest
}

func (s *fakeDeliveryServer) GetDelivery(_ context.Context, req *pb.GetDeliveryRequest) (*pb.GetDeliveryResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastGetDeliveryReq = req
	if s.getDeliveryErr != nil {
		return nil, s.getDeliveryErr
	}
	return s.getDeliveryResp, nil
}

func (s *fakeDeliveryServer) GetOpenDeliveries(_ context.Context, req *pb.GetOpenDeliveriesRequest) (*pb.GetOpenDeliveriesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastOpenReq = req
	if s.openDeliveriesErr != nil {
		return nil, s.openDeliveriesErr
	}
	return s.openDeliveriesResp, nil
}

// startDeliveryBackend registers the fake on a bufconn server and wraps a
// connected client in the production adapter.
func startDeliveryBackend(t *testing.T, srv *fakeDeliveryServer) ports.DeliveryServiceClient {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	grpcServer := grpc.NewServer()
	pb.RegisterDeliveryServiceServer(grpcServer, srv)

	serveErr := make(chan error, 1)
	go func() { serveErr <- grpcServer.Serve(lis) }()

	conn, err := grpc.NewClient(
		"passthrough:///delivery-bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient failed: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		grpcServer.Stop()
		select {
		case <-serveErr:
		case <-time.After(10 * time.Second):
			t.Errorf("Serve did not return after Stop")
		}
	})

	client := pb.NewDeliveryServiceClient(conn)
	return drivergrpc.NewDeliveryServiceClientAdapter(client)
}

func deliveryCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestDeliveryServiceClientAdapter_GetDelivery(t *testing.T) {
	t.Run("maps request and response fields", func(t *testing.T) {
		backend := &fakeDeliveryServer{
			getDeliveryResp: &pb.GetDeliveryResponse{
				Found:      true,
				DeliveryId: "del-1",
				CustomerId: "cust-1",
				DriverId:   "drv-1",
				Status:     "ASSIGNED",
				Amount:     42.5,
				Currency:   "EGP",
			},
		}
		adapter := startDeliveryBackend(t, backend)

		info, err := adapter.GetDelivery(deliveryCtx(t), "del-1")
		if err != nil {
			t.Fatalf("GetDelivery() error = %v", err)
		}

		backend.mu.Lock()
		gotReq := backend.lastGetDeliveryReq
		backend.mu.Unlock()
		if gotReq == nil || gotReq.DeliveryId != "del-1" {
			t.Errorf("backend received request %+v, want DeliveryId=del-1", gotReq)
		}

		if info == nil {
			t.Fatal("GetDelivery() info = nil, want mapped delivery")
		}
		if info.DeliveryID != "del-1" {
			t.Errorf("DeliveryID = %q, want %q", info.DeliveryID, "del-1")
		}
		if info.CustomerID != "cust-1" {
			t.Errorf("CustomerID = %q, want %q", info.CustomerID, "cust-1")
		}
		if info.DriverID != "drv-1" {
			t.Errorf("DriverID = %q, want %q", info.DriverID, "drv-1")
		}
		if info.Status != "ASSIGNED" {
			t.Errorf("Status = %q, want %q", info.Status, "ASSIGNED")
		}
		if info.Amount != 42.5 {
			t.Errorf("Amount = %v, want %v", info.Amount, 42.5)
		}
		if info.Currency != "EGP" {
			t.Errorf("Currency = %q, want %q", info.Currency, "EGP")
		}
	})

	t.Run("propagates backend errors", func(t *testing.T) {
		backend := &fakeDeliveryServer{
			getDeliveryErr: status.Error(codes.NotFound, "delivery not found"),
		}
		adapter := startDeliveryBackend(t, backend)

		info, err := adapter.GetDelivery(deliveryCtx(t), "ghost")
		if info != nil {
			t.Errorf("GetDelivery() info = %+v, want nil on error", info)
		}
		if err == nil {
			t.Fatal("GetDelivery() error = nil, want NotFound from backend")
		}
		if status.Code(err) != codes.NotFound {
			t.Errorf("error code = %v, want %v", status.Code(err), codes.NotFound)
		}
		if status.Convert(err).Message() != "delivery not found" {
			t.Errorf("error message = %q, want %q", status.Convert(err).Message(), "delivery not found")
		}
	})
}

func TestDeliveryServiceClientAdapter_GetOpenDeliveries(t *testing.T) {
	t.Run("maps request and response fields", func(t *testing.T) {
		backend := &fakeDeliveryServer{
			openDeliveriesResp: &pb.GetOpenDeliveriesResponse{
				Items: []*pb.OpenDeliverySummary{
					{
						Id:             "del-1",
						CustomerId:     "cust-1",
						Status:         "PENDING",
						PickupCity:     "Cairo",
						PickupCountry:  "EG",
						DropoffCity:    "Giza",
						DropoffCountry: "EG",
						Amount:         "100.5",
						Currency:       "EGP",
						CreatedAt:      "2026-01-01T00:00:00Z",
					},
					{
						Id:             "del-2",
						CustomerId:     "cust-2",
						Status:         "PENDING",
						PickupCity:     "Alex",
						PickupCountry:  "EG",
						DropoffCity:    "Luxor",
						DropoffCountry: "EG",
						Amount:         "250",
						Currency:       "EGP",
						CreatedAt:      "2026-01-02T00:00:00Z",
					},
				},
				TotalItems: 7,
			},
		}
		adapter := startDeliveryBackend(t, backend)

		items, total, err := adapter.GetOpenDeliveries(deliveryCtx(t), 2, 25)
		if err != nil {
			t.Fatalf("GetOpenDeliveries() error = %v", err)
		}

		backend.mu.Lock()
		gotReq := backend.lastOpenReq
		backend.mu.Unlock()
		if gotReq == nil || gotReq.Page != 2 || gotReq.Limit != 25 {
			t.Errorf("backend received request %+v, want Page=2 Limit=25", gotReq)
		}

		if total != 7 {
			t.Errorf("total = %d, want 7", total)
		}
		if len(items) != 2 {
			t.Fatalf("got %d items, want 2", len(items))
		}

		first := items[0]
		if first.ID != "del-1" {
			t.Errorf("ID = %q, want %q", first.ID, "del-1")
		}
		if first.CustomerID != "cust-1" {
			t.Errorf("CustomerID = %q, want %q", first.CustomerID, "cust-1")
		}
		if first.Status != "PENDING" {
			t.Errorf("Status = %q, want %q", first.Status, "PENDING")
		}
		if first.PickupCity != "Cairo" {
			t.Errorf("PickupCity = %q, want %q", first.PickupCity, "Cairo")
		}
		if first.PickupCountry != "EG" {
			t.Errorf("PickupCountry = %q, want %q", first.PickupCountry, "EG")
		}
		if first.DropoffCity != "Giza" {
			t.Errorf("DropoffCity = %q, want %q", first.DropoffCity, "Giza")
		}
		if first.DropoffCountry != "EG" {
			t.Errorf("DropoffCountry = %q, want %q", first.DropoffCountry, "EG")
		}
		if first.Amount != "100.5" {
			t.Errorf("Amount = %q, want %q", first.Amount, "100.5")
		}
		if first.Currency != "EGP" {
			t.Errorf("Currency = %q, want %q", first.Currency, "EGP")
		}
		if first.CreatedAt != "2026-01-01T00:00:00Z" {
			t.Errorf("CreatedAt = %q, want %q", first.CreatedAt, "2026-01-01T00:00:00Z")
		}
	})

	t.Run("empty list maps to an empty slice", func(t *testing.T) {
		backend := &fakeDeliveryServer{
			openDeliveriesResp: &pb.GetOpenDeliveriesResponse{Items: nil, TotalItems: 0},
		}
		adapter := startDeliveryBackend(t, backend)

		items, total, err := adapter.GetOpenDeliveries(deliveryCtx(t), 1, 10)
		if err != nil {
			t.Fatalf("GetOpenDeliveries() error = %v", err)
		}
		if len(items) != 0 {
			t.Errorf("got %d items, want 0", len(items))
		}
		if total != 0 {
			t.Errorf("total = %d, want 0", total)
		}
	})

	t.Run("propagates backend errors", func(t *testing.T) {
		backend := &fakeDeliveryServer{
			openDeliveriesErr: errors.New("delivery service unavailable"),
		}
		adapter := startDeliveryBackend(t, backend)

		items, total, err := adapter.GetOpenDeliveries(deliveryCtx(t), 1, 10)
		if items != nil {
			t.Errorf("items = %+v, want nil on error", items)
		}
		if total != 0 {
			t.Errorf("total = %d, want 0 on error", total)
		}
		if err == nil {
			t.Fatal("error = nil, want error from backend")
		}
	})
}
