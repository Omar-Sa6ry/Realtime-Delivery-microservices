package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc/proto"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/commands"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
	"google.golang.org/grpc"
)

// fakeProtoUserClient implements proto.UserServiceClient for tests.
type fakeProtoUserClient struct {
	role      string
	getErr    error
	updateErr error
	updateOK  bool
}

func (f *fakeProtoUserClient) GetUser(ctx context.Context, in *proto.GetUserRequest, opts ...grpc.CallOption) (*proto.GetUserResponse, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &proto.GetUserResponse{Id: in.Id, Role: f.role}, nil
}

func (f *fakeProtoUserClient) GetUserByEmail(ctx context.Context, in *proto.GetUserByEmailRequest, opts ...grpc.CallOption) (*proto.GetUserResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeProtoUserClient) ValidateToken(ctx context.Context, in *proto.ValidateTokenRequest, opts ...grpc.CallOption) (*proto.ValidateTokenResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeProtoUserClient) GetUserPermissions(ctx context.Context, in *proto.GetUserPermissionsRequest, opts ...grpc.CallOption) (*proto.GetUserPermissionsResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeProtoUserClient) UpdateUserRole(ctx context.Context, in *proto.UpdateUserRoleRequest, opts ...grpc.CallOption) (*proto.UpdateUserRoleResponse, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &proto.UpdateUserRoleResponse{Success: f.updateOK, Message: "ok"}, nil
}

func TestRegisterDriverHandler_Execute(t *testing.T) {
	tests := []struct {
		name          string
		existingUser  string // userID of an already registered driver ("" = none)
		userClient    *fakeProtoUserClient // nil = no user client at all
		driverSaveErr error
		publishErr    error
		wantErr       error
		wantDriver    bool
	}{
		{
			name:       "success - nil user client skips role update",
			userClient: nil,
			wantErr:    nil,
			wantDriver: true,
		},
		{
			name:       "success - user role updated to DRIVER",
			userClient: &fakeProtoUserClient{role: "CUSTOMER", updateOK: true},
			wantErr:    nil,
			wantDriver: true,
		},
		{
			name:         "fails - driver profile already exists for user",
			existingUser: "user-1",
			userClient:   &fakeProtoUserClient{updateOK: true},
			wantErr:      domain.ErrDriverAlreadyExists,
			wantDriver:   false,
		},
		{
			name:       "fails - user already has driver role",
			userClient: &fakeProtoUserClient{role: "driver", updateOK: true},
			wantErr:    domain.ErrDriverAlreadyExists,
			wantDriver: false,
		},
		{
			name:       "get user error continues to role update",
			userClient: &fakeProtoUserClient{getErr: errors.New("grpc down"), updateOK: true},
			wantErr:    nil,
			wantDriver: true,
		},
		{
			name:       "fails - role update rpc error",
			userClient: &fakeProtoUserClient{updateErr: errors.New("grpc down")},
			wantErr:    domain.ErrInternal,
			wantDriver: false,
		},
		{
			name:       "fails - role update reports success=false",
			userClient: &fakeProtoUserClient{updateOK: false},
			wantErr:    domain.ErrInternal,
			wantDriver: false,
		},
		{
			name:          "fails - driver save error",
			userClient:    &fakeProtoUserClient{updateOK: true},
			driverSaveErr: errors.New("save failed"),
			wantErr:       errors.New("save failed"),
			wantDriver:    false,
		},
		{
			name:       "publish failure is non-fatal",
			userClient: &fakeProtoUserClient{updateOK: true},
			publishErr: errors.New("kafka down"),
			wantErr:    nil,
			wantDriver: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseRepo := mocks.NewMockDriverRepository()
			var repo ports.DriverRepository = baseRepo
			if tt.existingUser != "" {
				baseRepo.SetDriver(&domain.Driver{ID: "existing", UserID: tt.existingUser, Status: domain.DriverStatusAvailable})
			}
			if tt.driverSaveErr != nil {
				repo = &driverRepoWithSaveError{MockDriverRepository: baseRepo, saveErr: tt.driverSaveErr}
			}
			pub := mocks.NewMockEventPublisher()
			if tt.publishErr != nil {
				pub.SetError("DriverCreated", tt.publishErr)
			}

			var userClient proto.UserServiceClient
			if tt.userClient != nil {
				userClient = tt.userClient
			}

			handler := commands.NewRegisterDriverHandler(repo, pub, userClient)

			driver, err := handler.Execute(context.Background(), commands.RegisterDriverCommand{
				UserID:       "user-1",
				VehicleType:  domain.VehicleTypeMotorcycle,
				PlateNumber:  "ABC-123",
				CapacityKg:   50,
				Capabilities: []string{"express"},
				ServiceArea:  "cairo",
				IsActive:     true,
			})

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Execute() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("Execute() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Execute() unexpected error: %v", err)
			}

			if tt.wantDriver {
				if driver == nil {
					t.Fatal("Execute() returned nil driver")
				}
				if driver.Status != domain.DriverStatusOffline {
					t.Errorf("driver status = %s, want OFFLINE", driver.Status)
				}
				if driver.Vehicle.Type != domain.VehicleTypeMotorcycle || driver.Vehicle.PlateNumber != "ABC-123" || driver.Vehicle.CapacityKg != 50 {
					t.Errorf("driver vehicle = %+v, want motorcycle/ABC-123/50", driver.Vehicle)
				}
				if driver.ServiceArea != "cairo" || !driver.IsActive || len(driver.Capabilities) != 1 {
					t.Errorf("driver metadata = %+v, want cairo/active/1 capability", driver)
				}
				saved, findErr := baseRepo.FindByUserID(context.Background(), "user-1")
				if findErr != nil {
					t.Fatalf("FindByUserID() unexpected error: %v", findErr)
				}
				if saved == nil || saved.ID != driver.ID {
					t.Errorf("saved driver = %+v, want the returned driver persisted", saved)
				}
			} else if driver != nil {
				t.Errorf("Execute() returned driver %+v, want nil", driver)
			}
		})
	}
}
