package grpc_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	drivergrpc "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc"
	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/i18n"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

type harness struct {
	driverRepo     *mocks.MockDriverRepository
	assignmentRepo *mocks.MockAssignmentRepository
	locationStore  *mocks.MockLocationStore
	lockManager    *mocks.MockLockManager
	eventPub       *mocks.MockEventPublisher
	dispatch       *services.DispatchService
	handler        *drivergrpc.DriverGRPCServer
}

// newHarness wires the DispatchService with in-memory mocks only — no
// infrastructure is required.
func newHarness() *harness {
	driverRepo := mocks.NewMockDriverRepository()
	assignmentRepo := mocks.NewMockAssignmentRepository()
	locationStore := mocks.NewMockLocationStore()
	lockManager := mocks.NewMockLockManager()
	eventPub := mocks.NewMockEventPublisher()

	dispatch := services.NewDispatchService(
		driverRepo,
		assignmentRepo,
		locationStore,
		lockManager,
		eventPub,
		domain.NewDispatchPolicy(),
		nil, // metrics
	)

	return &harness{
		driverRepo:     driverRepo,
		assignmentRepo: assignmentRepo,
		locationStore:  locationStore,
		lockManager:    lockManager,
		eventPub:       eventPub,
		dispatch:       dispatch,
		handler:        drivergrpc.NewDriverGRPCServer(dispatch, driverRepo),
	}
}

func newDriver(id, userID string, vt domain.VehicleType) *domain.Driver {
	return &domain.Driver{
		ID:       id,
		UserID:   userID,
		Status:   domain.DriverStatusAvailable,
		IsActive: true,
		Vehicle:  domain.VehicleInfo{Type: vt},
	}
}

// startBufConnServer starts the production GRPCServer (with its interceptors
// and reflection) on a bufconn listener and returns a connected client.
func startBufConnServer(t *testing.T, h *harness) pb.DriverServiceClient {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := drivergrpc.NewGRPCServer(h.dispatch, h.driverRepo)

	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start(lis) }()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
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
		srv.Stop()
		select {
		case err := <-startErr:
			if err != nil {
				t.Errorf("Start returned %v after Stop, want nil", err)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("Start did not return after Stop")
		}
	})

	return pb.NewDriverServiceClient(conn)
}

// rpcCtx returns a context with a deadline for RPC calls; it is cancelled when
// the (sub)test finishes.
func rpcCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func requireStatus(t *testing.T, err error, wantCode codes.Code, wantMsg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %v, got nil", wantCode)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != wantCode {
		t.Errorf("status code = %v, want %v (message %q)", st.Code(), wantCode, st.Message())
	}
	if st.Message() != wantMsg {
		t.Errorf("status message = %q, want %q", st.Message(), wantMsg)
	}
}

// vehicleFixtures are used both by FindAvailableDrivers (request-side enum
// mapping) and GetDriver (response-side toProtoVehicleType mapping).
var vehicleFixtures = []struct {
	pbType   pb.VehicleType
	domainT  domain.VehicleType
	id       string
	distance float64
}{
	{pb.VehicleType_CAR, domain.VehicleTypeCar, "drv-car", 100},
	{pb.VehicleType_MOTORCYCLE, domain.VehicleTypeMotorcycle, "drv-moto", 200},
	{pb.VehicleType_TRUCK, domain.VehicleTypeTruck, "drv-truck", 300},
	{pb.VehicleType_BICYCLE, domain.VehicleTypeBicycle, "drv-bike", 400},
	{pb.VehicleType_VAN, domain.VehicleTypeVan, "drv-van", 500},
}

// seedVehicles stores one available driver per vehicle type plus a geo
// location for each of them.
func seedVehicles(t *testing.T, h *harness) {
	t.Helper()
	for _, f := range vehicleFixtures {
		if err := h.driverRepo.Save(context.Background(), newDriver(f.id, "user-"+f.id, f.domainT)); err != nil {
			t.Fatalf("saving driver %s: %v", f.id, err)
		}
		h.locationStore.SetLocation(f.id, 31.0, 30.0, f.distance)
	}
}

// ---------------------------------------------------------------------------
// extractLanguage (driven through the handler because the context-value
// fallback branch cannot be reached over the wire)
// ---------------------------------------------------------------------------

func TestExtractLanguage(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "x-lang header wins and is normalized",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-lang", "AR")),
			want: "ar",
		},
		{
			name: "empty x-lang falls through to accept-language",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-lang", "", "accept-language", "ar-EG")),
			want: "ar",
		},
		{
			name: "accept-language used when x-lang absent",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("accept-language", "ar")),
			want: "ar",
		},
		{
			name: "unsupported language normalizes to english",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-lang", "fr")),
			want: "en",
		},
		{
			name: "metadata without language keys falls back to context value",
			ctx:  metadata.NewIncomingContext(i18n.WithLanguage(context.Background(), "ar"), metadata.MD{}),
			want: "ar",
		},
		{
			name: "no metadata falls back to context value",
			ctx:  i18n.WithLanguage(context.Background(), "ar"),
			want: "ar",
		},
		{
			name: "no metadata and no context value defaults to english",
			ctx:  context.Background(),
			want: "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness()
			_, err := h.handler.ReserveDriver(tt.ctx, &pb.ReserveDriverRequest{})
			requireStatus(t, err, codes.InvalidArgument, i18n.T(tt.want, "validation.driver_id_required"))
		})
	}
}

// TestTranslateErrorNilGuard covers translateError's nil-error guard; no RPC
// handler can reach it because they only call it when err != nil.
func TestTranslateErrorNilGuard(t *testing.T) {
	if got := drivergrpc.TranslateError("en", nil); got != nil {
		t.Errorf("TranslateError(\"en\", nil) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// translateError matrix (every domain code it maps + fallback branches)
// ---------------------------------------------------------------------------

var translateErrorCases = []struct {
	name     string
	err      error
	wantCode codes.Code
	wantMsg  string
}{
	{"domain/driver_not_found", domain.ErrDriverNotFound, codes.NotFound, i18n.T("en", "driver.not_found")},
	{"domain/driver_not_available_for_assignment", domain.ErrDriverNotAvailableForAssignment, codes.FailedPrecondition, i18n.T("en", "driver.not_available")},
	{"domain/driver_not_available", domain.ErrDriverNotAvailable, codes.FailedPrecondition, i18n.T("en", "driver.not_available")},
	{"domain/driver_already_reserved", domain.ErrDriverAlreadyReserved, codes.FailedPrecondition, i18n.T("en", "driver.already_reserved")},
	{"domain/driver_already_online", domain.ErrDriverAlreadyOnline, codes.FailedPrecondition, i18n.T("en", "driver.already_online")},
	{"domain/driver_already_offline", domain.ErrDriverAlreadyOffline, codes.FailedPrecondition, i18n.T("en", "driver.already_offline")},
	{"domain/assignment_not_found", domain.ErrAssignmentNotFound, codes.NotFound, i18n.T("en", "assignment.not_found")},
	{"domain/assignment_invalid_state", domain.ErrAssignmentInvalidState, codes.FailedPrecondition, i18n.T("en", "assignment.already_handled")},
	{"domain/assignment_expired", domain.ErrAssignmentExpired, codes.FailedPrecondition, i18n.T("en", "assignment.expired")},
	{"domain/invalid_argument", domain.ErrInvalidArgument, codes.InvalidArgument, i18n.T("en", "error.internal")},
	{"domain/unmapped_code_falls_back_to_internal", &domain.Error{Code: "some_other_code", Message: "boom"}, codes.Internal, i18n.T("en", "error.internal")},
	{"plain_error_falls_back_to_internal", errors.New("boom"), codes.Internal, i18n.T("en", "error.internal") + ": boom"},
}

func TestReserveDriver_TranslateErrorMatrix(t *testing.T) {
	for _, tc := range translateErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			// The mock repository returns this error from FindByID, which is
			// the first domain call inside dispatch.ReserveDriver.
			h.driverRepo.SetError(tc.err)

			_, err := h.handler.ReserveDriver(context.Background(), &pb.ReserveDriverRequest{
				DriverId:   "d1",
				DeliveryId: "del-1",
			})
			requireStatus(t, err, tc.wantCode, tc.wantMsg)
		})
	}
}

func TestAssignDriver_TranslateErrorMatrix(t *testing.T) {
	for _, tc := range translateErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			h.driverRepo.SetError(tc.err)

			_, err := h.handler.AssignDriver(context.Background(), &pb.AssignDriverRequest{
				DriverId:   "d1",
				DeliveryId: "del-1",
			})
			requireStatus(t, err, tc.wantCode, tc.wantMsg)
		})
	}
}

// ---------------------------------------------------------------------------
// FindAvailableDrivers
// ---------------------------------------------------------------------------

func TestFindAvailableDrivers_Validation(t *testing.T) {
	h := newHarness()
	client := startBufConnServer(t, h)

	t.Run("missing deliveryId in english", func(t *testing.T) {
		_, err := client.FindAvailableDrivers(rpcCtx(t), &pb.FindAvailableDriversRequest{
			Latitude:    30,
			Longitude:   31,
			RadiusKm:    10,
			VehicleType: pb.VehicleType_CAR,
		})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("en", "validation.delivery_id_required"))
	})

	t.Run("missing deliveryId in arabic via x-lang", func(t *testing.T) {
		ctx := metadata.AppendToOutgoingContext(rpcCtx(t), "x-lang", "ar")
		_, err := client.FindAvailableDrivers(ctx, &pb.FindAvailableDriversRequest{})
		want := i18n.T("ar", "validation.delivery_id_required")
		requireStatus(t, err, codes.InvalidArgument, want)
		if want == i18n.T("en", "validation.delivery_id_required") {
			t.Fatal("arabic and english messages are identical; localization is not applied")
		}
	})
}

func TestFindAvailableDrivers_VehicleTypeMapping(t *testing.T) {
	h := newHarness()
	seedVehicles(t, h)
	client := startBufConnServer(t, h)
	ctx := rpcCtx(t)

	for _, f := range vehicleFixtures {
		t.Run("maps_"+f.pbType.String(), func(t *testing.T) {
			resp, err := client.FindAvailableDrivers(ctx, &pb.FindAvailableDriversRequest{
				Latitude:    30.0,
				Longitude:   31.0,
				RadiusKm:    10,
				VehicleType: f.pbType,
				DeliveryId:  "del-1",
			})
			if err != nil {
				t.Fatalf("FindAvailableDrivers() error = %v", err)
			}
			if len(resp.Candidates) != 1 {
				t.Fatalf("got %d candidates, want exactly 1 (%s)", len(resp.Candidates), f.id)
			}
			c := resp.Candidates[0]
			if c.DriverId != f.id {
				t.Errorf("candidate DriverId = %q, want %q", c.DriverId, f.id)
			}
			if c.DistanceMeters != f.distance {
				t.Errorf("candidate DistanceMeters = %v, want %v", c.DistanceMeters, f.distance)
			}
			if c.VehicleType != f.pbType {
				t.Errorf("candidate VehicleType = %v, want %v", c.VehicleType, f.pbType)
			}
		})
	}

	t.Run("VEHICLE_TYPE_UNKNOWN returns every vehicle type", func(t *testing.T) {
		resp, err := client.FindAvailableDrivers(ctx, &pb.FindAvailableDriversRequest{
			Latitude:    30.0,
			Longitude:   31.0,
			RadiusKm:    10,
			VehicleType: pb.VehicleType_VEHICLE_TYPE_UNKNOWN,
			DeliveryId:  "del-1",
		})
		if err != nil {
			t.Fatalf("FindAvailableDrivers() error = %v", err)
		}
		if len(resp.Candidates) != len(vehicleFixtures) {
			t.Fatalf("got %d candidates, want %d", len(resp.Candidates), len(vehicleFixtures))
		}
		byID := make(map[string]*pb.DriverCandidate, len(resp.Candidates))
		for _, c := range resp.Candidates {
			byID[c.DriverId] = c
		}
		for _, f := range vehicleFixtures {
			c, ok := byID[f.id]
			if !ok {
				t.Errorf("missing candidate %q", f.id)
				continue
			}
			if c.VehicleType != f.pbType {
				t.Errorf("candidate %q VehicleType = %v, want %v", f.id, c.VehicleType, f.pbType)
			}
		}
	})

	t.Run("an unrecognized enum value hits the default branch", func(t *testing.T) {
		resp, err := client.FindAvailableDrivers(ctx, &pb.FindAvailableDriversRequest{
			Latitude:    30.0,
			Longitude:   31.0,
			RadiusKm:    10,
			VehicleType: pb.VehicleType(42),
			DeliveryId:  "del-1",
		})
		if err != nil {
			t.Fatalf("FindAvailableDrivers() error = %v", err)
		}
		if len(resp.Candidates) != len(vehicleFixtures) {
			t.Fatalf("got %d candidates, want %d (unfiltered)", len(resp.Candidates), len(vehicleFixtures))
		}
	})
}

func TestFindAvailableDrivers_NoCandidates(t *testing.T) {
	h := newHarness()
	client := startBufConnServer(t, h)

	resp, err := client.FindAvailableDrivers(rpcCtx(t), &pb.FindAvailableDriversRequest{
		Latitude:    30,
		Longitude:   31,
		RadiusKm:    10,
		VehicleType: pb.VehicleType_CAR,
		DeliveryId:  "del-1",
	})
	if err != nil {
		t.Fatalf("FindAvailableDrivers() error = %v", err)
	}
	if len(resp.Candidates) != 0 {
		t.Errorf("got %d candidates, want 0", len(resp.Candidates))
	}
}

func TestFindAvailableDrivers_ErrorPath(t *testing.T) {
	h := newHarness()
	h.driverRepo.SetError(domain.ErrInternal)
	client := startBufConnServer(t, h)

	_, err := client.FindAvailableDrivers(rpcCtx(t), &pb.FindAvailableDriversRequest{
		Latitude:    30,
		Longitude:   31,
		RadiusKm:    10,
		DeliveryId:  "del-1",
		VehicleType: pb.VehicleType_CAR,
	})
	requireStatus(t, err, codes.Internal, i18n.T("en", "error.internal"))
}

// ---------------------------------------------------------------------------
// ReserveDriver
// ---------------------------------------------------------------------------

func TestReserveDriver_Validation(t *testing.T) {
	h := newHarness()
	client := startBufConnServer(t, h)

	tests := []struct {
		name    string
		req     *pb.ReserveDriverRequest
		wantMsg string
	}{
		{"missing driverId", &pb.ReserveDriverRequest{DeliveryId: "del-1"}, i18n.T("en", "validation.driver_id_required")},
		{"missing deliveryId", &pb.ReserveDriverRequest{DriverId: "d1"}, i18n.T("en", "validation.delivery_id_required")},
		{"missing both", &pb.ReserveDriverRequest{}, i18n.T("en", "validation.driver_id_required")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ReserveDriver(rpcCtx(t), tt.req)
			requireStatus(t, err, codes.InvalidArgument, tt.wantMsg)
		})
	}

	t.Run("localized via accept-language", func(t *testing.T) {
		ctx := metadata.AppendToOutgoingContext(rpcCtx(t), "accept-language", "ar-SA")
		_, err := client.ReserveDriver(ctx, &pb.ReserveDriverRequest{DeliveryId: "del-1"})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("ar", "validation.driver_id_required"))
	})
}

func TestReserveDriver_Success(t *testing.T) {
	h := newHarness()
	h.driverRepo.SetDriver(newDriver("d1", "u1", domain.VehicleTypeCar))
	client := startBufConnServer(t, h)

	resp, err := client.ReserveDriver(rpcCtx(t), &pb.ReserveDriverRequest{
		DriverId:   "d1",
		DeliveryId: "del-1",
	})
	if err != nil {
		t.Fatalf("ReserveDriver() error = %v", err)
	}
	if !resp.Reserved {
		t.Error("Reserved = false, want true")
	}
	if resp.DriverId != "d1" {
		t.Errorf("DriverId = %q, want %q", resp.DriverId, "d1")
	}
	if resp.DeliveryId != "del-1" {
		t.Errorf("DeliveryId = %q, want %q", resp.DeliveryId, "del-1")
	}
	if resp.AssignmentId != "del-1-d1" {
		t.Errorf("AssignmentId = %q, want %q", resp.AssignmentId, "del-1-d1")
	}

	// The dispatch service should have transitioned the driver to BUSY and
	// created the assignment record.
	d, err := h.driverRepo.FindByID(context.Background(), "d1")
	if err != nil || d == nil {
		t.Fatalf("FindByID(d1) = %v, %v", d, err)
	}
	if d.Status != domain.DriverStatusBusy {
		t.Errorf("driver status = %v, want BUSY", d.Status)
	}
	a, err := h.assignmentRepo.FindByDeliveryID(context.Background(), "del-1")
	if err != nil || a == nil {
		t.Fatalf("FindByDeliveryID(del-1) = %v, %v", a, err)
	}
	if a.Status != domain.AssignmentStatusOffered {
		t.Errorf("assignment status = %v, want OFFERED", a.Status)
	}
}

func TestReserveDriver_NaturalFailures(t *testing.T) {
	t.Run("driver not found", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		_, err := client.ReserveDriver(rpcCtx(t), &pb.ReserveDriverRequest{DriverId: "ghost", DeliveryId: "del-1"})
		requireStatus(t, err, codes.NotFound, i18n.T("en", "driver.not_found"))
	})

	t.Run("driver not available", func(t *testing.T) {
		h := newHarness()
		d := newDriver("d1", "u1", domain.VehicleTypeCar)
		d.Status = domain.DriverStatusOffline
		h.driverRepo.SetDriver(d)
		client := startBufConnServer(t, h)
		_, err := client.ReserveDriver(rpcCtx(t), &pb.ReserveDriverRequest{DriverId: "d1", DeliveryId: "del-1"})
		requireStatus(t, err, codes.FailedPrecondition, i18n.T("en", "driver.not_available"))
	})

	t.Run("lock already held means already reserved", func(t *testing.T) {
		h := newHarness()
		h.driverRepo.SetDriver(newDriver("d1", "u1", domain.VehicleTypeCar))
		if _, err := h.lockManager.Acquire(context.Background(), "d1", time.Minute); err != nil {
			t.Fatalf("pre-acquiring lock: %v", err)
		}
		client := startBufConnServer(t, h)
		_, err := client.ReserveDriver(rpcCtx(t), &pb.ReserveDriverRequest{DriverId: "d1", DeliveryId: "del-1"})
		requireStatus(t, err, codes.FailedPrecondition, i18n.T("en", "driver.already_reserved"))
	})

	t.Run("repository error falls back to internal", func(t *testing.T) {
		h := newHarness()
		h.driverRepo.SetError(errors.New("mongo down"))
		client := startBufConnServer(t, h)
		_, err := client.ReserveDriver(rpcCtx(t), &pb.ReserveDriverRequest{DriverId: "d1", DeliveryId: "del-1"})
		requireStatus(t, err, codes.Internal, i18n.T("en", "error.internal")+": mongo down")
	})
}

// ---------------------------------------------------------------------------
// AssignDriver
// ---------------------------------------------------------------------------

func TestAssignDriver_Validation(t *testing.T) {
	h := newHarness()
	client := startBufConnServer(t, h)

	tests := []struct {
		name    string
		req     *pb.AssignDriverRequest
		wantMsg string
	}{
		{"missing driverId", &pb.AssignDriverRequest{DeliveryId: "del-1"}, i18n.T("en", "validation.driver_id_required")},
		{"missing deliveryId", &pb.AssignDriverRequest{DriverId: "d1"}, i18n.T("en", "validation.delivery_id_required")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.AssignDriver(rpcCtx(t), tt.req)
			requireStatus(t, err, codes.InvalidArgument, tt.wantMsg)
		})
	}

	t.Run("localized via x-lang", func(t *testing.T) {
		ctx := metadata.AppendToOutgoingContext(rpcCtx(t), "x-lang", "ar")
		_, err := client.AssignDriver(ctx, &pb.AssignDriverRequest{DeliveryId: "del-1"})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("ar", "validation.driver_id_required"))
	})
}

func TestAssignDriver_Success(t *testing.T) {
	h := newHarness()
	h.driverRepo.SetDriver(newDriver("d1", "u1", domain.VehicleTypeCar))
	client := startBufConnServer(t, h)

	resp, err := client.AssignDriver(rpcCtx(t), &pb.AssignDriverRequest{
		DriverId:   "d1",
		DeliveryId: "del-1",
	})
	if err != nil {
		t.Fatalf("AssignDriver() error = %v", err)
	}
	if !resp.Assigned {
		t.Error("Assigned = false, want true")
	}
	if resp.DriverId != "d1" {
		t.Errorf("DriverId = %q, want %q", resp.DriverId, "d1")
	}
	if resp.DeliveryId != "del-1" {
		t.Errorf("DeliveryId = %q, want %q", resp.DeliveryId, "del-1")
	}
	if resp.AssignmentId != "del-1-d1" {
		t.Errorf("AssignmentId = %q, want %q", resp.AssignmentId, "del-1-d1")
	}
}

// ---------------------------------------------------------------------------
// ReleaseDriver
// ---------------------------------------------------------------------------

func TestReleaseDriver(t *testing.T) {
	t.Run("missing driverId", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		_, err := client.ReleaseDriver(rpcCtx(t), &pb.ReleaseDriverRequest{DeliveryId: "del-1"})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("en", "validation.driver_id_required"))
	})

	t.Run("no active assignment is idempotent", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		resp, err := client.ReleaseDriver(rpcCtx(t), &pb.ReleaseDriverRequest{DriverId: "d1", DeliveryId: "del-1"})
		if err != nil {
			t.Fatalf("ReleaseDriver() error = %v", err)
		}
		if !resp.Released {
			t.Error("Released = false, want true")
		}
		if resp.DriverId != "d1" {
			t.Errorf("DriverId = %q, want %q", resp.DriverId, "d1")
		}
	})

	t.Run("success releases an active assignment", func(t *testing.T) {
		h := newHarness()
		h.driverRepo.SetDriver(newDriver("d1", "u1", domain.VehicleTypeCar))
		h.assignmentRepo.SetAssignment(&domain.Assignment{
			ID:         "del-1-d1",
			DriverID:   "d1",
			DeliveryID: "del-1",
			Status:     domain.AssignmentStatusOffered,
		})
		client := startBufConnServer(t, h)

		resp, err := client.ReleaseDriver(rpcCtx(t), &pb.ReleaseDriverRequest{DriverId: "d1", DeliveryId: "del-1"})
		if err != nil {
			t.Fatalf("ReleaseDriver() error = %v", err)
		}
		if !resp.Released {
			t.Error("Released = false, want true")
		}

		d, err := h.driverRepo.FindByID(context.Background(), "d1")
		if err != nil || d == nil {
			t.Fatalf("FindByID(d1) = %v, %v", d, err)
		}
		if d.Status != domain.DriverStatusAvailable {
			t.Errorf("driver status = %v, want AVAILABLE", d.Status)
		}
		a, err := h.assignmentRepo.FindByID(context.Background(), "del-1-d1")
		if err != nil || a == nil {
			t.Fatalf("FindByID(del-1-d1) = %v, %v", a, err)
		}
		if a.Status != domain.AssignmentStatusCompleted {
			t.Errorf("assignment status = %v, want COMPLETED", a.Status)
		}
	})

	t.Run("non-domain error goes through translateError", func(t *testing.T) {
		h := newHarness()
		h.assignmentRepo.SetAssignment(&domain.Assignment{
			ID:         "del-1-d1",
			DriverID:   "d1",
			DeliveryID: "del-1",
			Status:     domain.AssignmentStatusOffered,
		})
		h.assignmentRepo.SetError(errors.New("mongo down"))
		client := startBufConnServer(t, h)

		_, err := client.ReleaseDriver(rpcCtx(t), &pb.ReleaseDriverRequest{DriverId: "d1", DeliveryId: "del-1"})
		requireStatus(t, err, codes.Internal, i18n.T("en", "error.internal")+": mongo down")
	})

	t.Run("domain error other than assignment_not_found goes through translateError", func(t *testing.T) {
		h := newHarness()
		h.assignmentRepo.SetAssignment(&domain.Assignment{
			ID:         "del-1-d1",
			DriverID:   "d1",
			DeliveryID: "del-1",
			Status:     domain.AssignmentStatusOffered,
		})
		h.assignmentRepo.SetError(domain.ErrAssignmentExpired)
		client := startBufConnServer(t, h)

		_, err := client.ReleaseDriver(rpcCtx(t), &pb.ReleaseDriverRequest{DriverId: "d1", DeliveryId: "del-1"})
		requireStatus(t, err, codes.FailedPrecondition, i18n.T("en", "assignment.expired"))
	})
}

// ---------------------------------------------------------------------------
// GetDriver
// ---------------------------------------------------------------------------

func TestGetDriver(t *testing.T) {
	t.Run("missing driverId", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		_, err := client.GetDriver(rpcCtx(t), &pb.GetDriverRequest{})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("en", "validation.driver_id_required"))
	})

	t.Run("found maps all fields", func(t *testing.T) {
		h := newHarness()
		d := newDriver("d1", "u1", domain.VehicleTypeTruck)
		d.Status = domain.DriverStatusOffline
		d.IsBlocked = true
		h.driverRepo.SetDriver(d)
		client := startBufConnServer(t, h)

		resp, err := client.GetDriver(rpcCtx(t), &pb.GetDriverRequest{DriverId: "d1"})
		if err != nil {
			t.Fatalf("GetDriver() error = %v", err)
		}
		if !resp.Found {
			t.Error("Found = false, want true")
		}
		if resp.DriverId != "d1" {
			t.Errorf("DriverId = %q, want %q", resp.DriverId, "d1")
		}
		if resp.UserId != "u1" {
			t.Errorf("UserId = %q, want %q", resp.UserId, "u1")
		}
		if resp.Status != string(domain.DriverStatusOffline) {
			t.Errorf("Status = %q, want %q", resp.Status, domain.DriverStatusOffline)
		}
		if resp.VehicleType != pb.VehicleType_TRUCK {
			t.Errorf("VehicleType = %v, want TRUCK", resp.VehicleType)
		}
		if !resp.IsBlocked {
			t.Error("IsBlocked = false, want true")
		}
	})

	t.Run("vehicle type mapping including unknown values", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		ctx := rpcCtx(t)

		cases := []struct {
			domainT domain.VehicleType
			want    pb.VehicleType
		}{
			{domain.VehicleTypeCar, pb.VehicleType_CAR},
			{domain.VehicleTypeMotorcycle, pb.VehicleType_MOTORCYCLE},
			{domain.VehicleTypeTruck, pb.VehicleType_TRUCK},
			{domain.VehicleTypeBicycle, pb.VehicleType_BICYCLE},
			{domain.VehicleTypeVan, pb.VehicleType_VAN},
			{"", pb.VehicleType_VEHICLE_TYPE_UNKNOWN},
			{domain.VehicleType("HOVERCRAFT"), pb.VehicleType_VEHICLE_TYPE_UNKNOWN},
		}

		for i, tc := range cases {
			id := "drv-" + string(rune('a'+i))
			if err := h.driverRepo.Save(ctx, newDriver(id, "user-"+id, tc.domainT)); err != nil {
				t.Fatalf("saving driver %s: %v", id, err)
			}
			resp, err := client.GetDriver(ctx, &pb.GetDriverRequest{DriverId: id})
			if err != nil {
				t.Fatalf("GetDriver(%s) error = %v", id, err)
			}
			if resp.VehicleType != tc.want {
				t.Errorf("GetDriver(%s: domain %q).VehicleType = %v, want %v", id, tc.domainT, resp.VehicleType, tc.want)
			}
		}
	})

	t.Run("not found", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		resp, err := client.GetDriver(rpcCtx(t), &pb.GetDriverRequest{DriverId: "ghost"})
		if err != nil {
			t.Fatalf("GetDriver() error = %v", err)
		}
		if resp.Found {
			t.Error("Found = true, want false")
		}
		if resp.DriverId != "ghost" {
			t.Errorf("DriverId = %q, want %q", resp.DriverId, "ghost")
		}
	})

	t.Run("repository error reports not found", func(t *testing.T) {
		h := newHarness()
		h.driverRepo.SetError(errors.New("mongo down"))
		client := startBufConnServer(t, h)
		resp, err := client.GetDriver(rpcCtx(t), &pb.GetDriverRequest{DriverId: "d1"})
		if err != nil {
			t.Fatalf("GetDriver() error = %v", err)
		}
		if resp.Found {
			t.Error("Found = true, want false on repository error")
		}
	})
}

// ---------------------------------------------------------------------------
// GetDriverStatus
// ---------------------------------------------------------------------------

func TestGetDriverStatus(t *testing.T) {
	t.Run("missing driverId", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		_, err := client.GetDriverStatus(rpcCtx(t), &pb.GetDriverStatusRequest{})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("en", "validation.driver_id_required"))
	})

	t.Run("found", func(t *testing.T) {
		h := newHarness()
		h.driverRepo.SetDriver(newDriver("d1", "u1", domain.VehicleTypeCar))
		client := startBufConnServer(t, h)
		resp, err := client.GetDriverStatus(rpcCtx(t), &pb.GetDriverStatusRequest{DriverId: "d1"})
		if err != nil {
			t.Fatalf("GetDriverStatus() error = %v", err)
		}
		if resp.DriverId != "d1" {
			t.Errorf("DriverId = %q, want %q", resp.DriverId, "d1")
		}
		if resp.Status != string(domain.DriverStatusAvailable) {
			t.Errorf("Status = %q, want %q", resp.Status, domain.DriverStatusAvailable)
		}
	})

	t.Run("not found", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		_, err := client.GetDriverStatus(rpcCtx(t), &pb.GetDriverStatusRequest{DriverId: "ghost"})
		requireStatus(t, err, codes.NotFound, i18n.T("en", "driver.not_found"))
	})

	t.Run("not found is localized", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		ctx := metadata.AppendToOutgoingContext(rpcCtx(t), "x-lang", "ar")
		_, err := client.GetDriverStatus(ctx, &pb.GetDriverStatusRequest{DriverId: "ghost"})
		requireStatus(t, err, codes.NotFound, i18n.T("ar", "driver.not_found"))
	})

	t.Run("repository error reports not found", func(t *testing.T) {
		h := newHarness()
		h.driverRepo.SetError(errors.New("mongo down"))
		client := startBufConnServer(t, h)
		_, err := client.GetDriverStatus(rpcCtx(t), &pb.GetDriverStatusRequest{DriverId: "d1"})
		requireStatus(t, err, codes.NotFound, i18n.T("en", "driver.not_found"))
	})
}

// ---------------------------------------------------------------------------
// ValidateID
// ---------------------------------------------------------------------------

func TestValidateID(t *testing.T) {
	t.Run("missing id", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		_, err := client.ValidateID(rpcCtx(t), &pb.ValidateIDRequest{IdType: "driver"})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("en", "validation.id_required"))
	})

	t.Run("unknown idType", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		_, err := client.ValidateID(rpcCtx(t), &pb.ValidateIDRequest{Id: "x", IdType: "delivery"})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("en", "validation.invalid_id_type"))
	})

	t.Run("unknown idType is localized", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		ctx := metadata.AppendToOutgoingContext(rpcCtx(t), "x-lang", "ar")
		_, err := client.ValidateID(ctx, &pb.ValidateIDRequest{Id: "x", IdType: "delivery"})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("ar", "validation.invalid_id_type"))
	})

	t.Run("driver found (case-insensitive idType)", func(t *testing.T) {
		h := newHarness()
		d := newDriver("d1", "u1", domain.VehicleTypeCar)
		d.Status = domain.DriverStatusBusy
		h.driverRepo.SetDriver(d)
		client := startBufConnServer(t, h)

		resp, err := client.ValidateID(rpcCtx(t), &pb.ValidateIDRequest{Id: "d1", IdType: "DRIVER"})
		if err != nil {
			t.Fatalf("ValidateID() error = %v", err)
		}
		if !resp.Valid {
			t.Error("Valid = false, want true")
		}
		if resp.DriverId != "d1" {
			t.Errorf("DriverId = %q, want %q", resp.DriverId, "d1")
		}
		if resp.Status != string(domain.DriverStatusBusy) {
			t.Errorf("Status = %q, want %q", resp.Status, domain.DriverStatusBusy)
		}
		if resp.Message != i18n.T("en", "driver.found") {
			t.Errorf("Message = %q, want %q", resp.Message, i18n.T("en", "driver.found"))
		}
	})

	t.Run("driver not found", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		resp, err := client.ValidateID(rpcCtx(t), &pb.ValidateIDRequest{Id: "ghost", IdType: "driver"})
		if err != nil {
			t.Fatalf("ValidateID() error = %v", err)
		}
		if resp.Valid {
			t.Error("Valid = true, want false")
		}
		if resp.Message != i18n.T("en", "driver.not_found") {
			t.Errorf("Message = %q, want %q", resp.Message, i18n.T("en", "driver.not_found"))
		}
	})

	t.Run("driver repository error", func(t *testing.T) {
		h := newHarness()
		h.driverRepo.SetError(errors.New("mongo down"))
		client := startBufConnServer(t, h)
		resp, err := client.ValidateID(rpcCtx(t), &pb.ValidateIDRequest{Id: "d1", IdType: "driver"})
		if err != nil {
			t.Fatalf("ValidateID() error = %v", err)
		}
		if resp.Valid {
			t.Error("Valid = true, want false on repository error")
		}
		if resp.Message != i18n.T("en", "driver.not_found") {
			t.Errorf("Message = %q, want %q", resp.Message, i18n.T("en", "driver.not_found"))
		}
	})

	t.Run("user with driver profile", func(t *testing.T) {
		h := newHarness()
		h.driverRepo.SetDriver(newDriver("d1", "u1", domain.VehicleTypeCar))
		client := startBufConnServer(t, h)

		resp, err := client.ValidateID(rpcCtx(t), &pb.ValidateIDRequest{Id: "u1", IdType: "USER"})
		if err != nil {
			t.Fatalf("ValidateID() error = %v", err)
		}
		if !resp.Valid {
			t.Error("Valid = false, want true")
		}
		if resp.DriverId != "d1" {
			t.Errorf("DriverId = %q, want %q", resp.DriverId, "d1")
		}
		if resp.Message != i18n.T("en", "driver.profile_found") {
			t.Errorf("Message = %q, want %q", resp.Message, i18n.T("en", "driver.profile_found"))
		}
	})

	t.Run("user without driver profile", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		resp, err := client.ValidateID(rpcCtx(t), &pb.ValidateIDRequest{Id: "u-ghost", IdType: "user"})
		if err != nil {
			t.Fatalf("ValidateID() error = %v", err)
		}
		if resp.Valid {
			t.Error("Valid = true, want false")
		}
		if resp.Message != i18n.T("en", "driver.no_profile_for_user") {
			t.Errorf("Message = %q, want %q", resp.Message, i18n.T("en", "driver.no_profile_for_user"))
		}
	})

	t.Run("user repository error", func(t *testing.T) {
		h := newHarness()
		h.driverRepo.SetError(errors.New("mongo down"))
		client := startBufConnServer(t, h)
		resp, err := client.ValidateID(rpcCtx(t), &pb.ValidateIDRequest{Id: "u1", IdType: "user"})
		if err != nil {
			t.Fatalf("ValidateID() error = %v", err)
		}
		if resp.Valid {
			t.Error("Valid = true, want false on repository error")
		}
		if resp.Message != i18n.T("en", "driver.no_profile_for_user") {
			t.Errorf("Message = %q, want %q", resp.Message, i18n.T("en", "driver.no_profile_for_user"))
		}
	})
}

// ---------------------------------------------------------------------------
// error localization end-to-end (messages must actually differ ar vs en)
// ---------------------------------------------------------------------------

func TestErrorMessagesAreLocalized(t *testing.T) {
	t.Run("ReserveDriver domain error in arabic", func(t *testing.T) {
		h := newHarness()
		h.driverRepo.SetError(domain.ErrDriverAlreadyReserved)
		client := startBufConnServer(t, h)

		ctx := metadata.AppendToOutgoingContext(rpcCtx(t), "x-lang", "ar")
		_, err := client.ReserveDriver(ctx, &pb.ReserveDriverRequest{DriverId: "d1", DeliveryId: "del-1"})

		wantAr := i18n.T("ar", "driver.already_reserved")
		if wantAr == i18n.T("en", "driver.already_reserved") {
			t.Fatal("arabic and english messages are identical; localization is not applied")
		}
		requireStatus(t, err, codes.FailedPrecondition, wantAr)
	})

	t.Run("AssignDriver validation in arabic", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)

		ctx := metadata.AppendToOutgoingContext(rpcCtx(t), "x-lang", "ar")
		_, err := client.AssignDriver(ctx, &pb.AssignDriverRequest{DriverId: "d1"})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("ar", "validation.delivery_id_required"))
	})

	t.Run("default english without any metadata", func(t *testing.T) {
		h := newHarness()
		client := startBufConnServer(t, h)
		_, err := client.ReserveDriver(rpcCtx(t), &pb.ReserveDriverRequest{})
		requireStatus(t, err, codes.InvalidArgument, i18n.T("en", "validation.driver_id_required"))
	})
}

// ---------------------------------------------------------------------------
// NewGRPCServer / Start / Stop over a real TCP listener
// ---------------------------------------------------------------------------

func TestGRPCServer_StartAndStop(t *testing.T) {
	h := newHarness()
	srv := drivergrpc.NewGRPCServer(h.dispatch, h.driverRepo)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}

	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start(lis) }()

	conn, err := grpc.NewClient(
		"passthrough:///"+lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient failed: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// One real call against the TCP listener: the driver does not exist, so a
	// NotFound status proves the server is serving.
	client := pb.NewDriverServiceClient(conn)
	_, err = client.GetDriverStatus(ctx, &pb.GetDriverStatusRequest{DriverId: "ghost"})
	requireStatus(t, err, codes.NotFound, i18n.T("en", "driver.not_found"))

	srv.Stop()

	select {
	case err := <-startErr:
		if err != nil {
			t.Errorf("Start returned %v after Stop, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Start did not return after Stop")
	}
}
