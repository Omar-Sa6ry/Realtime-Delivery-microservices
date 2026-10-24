package queries_test

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/queries"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

func TestFindAvailableDriversQuery_Execute(t *testing.T) {
	tests := []struct {
		name          string
		setupDrivers  []*domain.Driver
		setupError    error
		vehicleType   domain.VehicleType
		expectedCount int
	}{
		{
			name: "success - available drivers found",
			setupDrivers: []*domain.Driver{
				{ID: "driver-1", Status: domain.DriverStatusAvailable, IsActive: true, IsBlocked: false, Vehicle: domain.VehicleInfo{Type: domain.VehicleTypeCar}},
				{ID: "driver-2", Status: domain.DriverStatusAvailable, IsActive: true, IsBlocked: false, Vehicle: domain.VehicleInfo{Type: domain.VehicleTypeMotorcycle}},
			},
			vehicleType:   domain.VehicleTypeCar,
			expectedCount: 1, // Only Car driver matches vehicle type filter
		},
		{
			name: "success - filter by vehicle type",
			setupDrivers: []*domain.Driver{
				{ID: "driver-1", Status: domain.DriverStatusAvailable, IsActive: true, IsBlocked: false, Vehicle: domain.VehicleInfo{Type: domain.VehicleTypeCar}},
				{ID: "driver-2", Status: domain.DriverStatusAvailable, IsActive: true, IsBlocked: false, Vehicle: domain.VehicleInfo{Type: domain.VehicleTypeMotorcycle}},
			},
			vehicleType:   domain.VehicleTypeCar,
			expectedCount: 1,
		},
		{
			name: "success - no available drivers",
			setupDrivers: []*domain.Driver{
				{ID: "driver-1", Status: domain.DriverStatusOffline, IsActive: true, IsBlocked: false},
				{ID: "driver-2", Status: domain.DriverStatusBusy, IsActive: true, IsBlocked: false},
			},
			vehicleType:   domain.VehicleTypeCar,
			expectedCount: 0,
		},
		{
			name: "fails - repo error",
			setupDrivers: []*domain.Driver{
				{ID: "driver-1", Status: domain.DriverStatusAvailable, IsActive: true, IsBlocked: false},
			},
			setupError: domain.ErrInternal,
			vehicleType:   domain.VehicleTypeCar,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driverRepo := mocks.NewMockDriverRepository()
			locationStore := mocks.NewMockLocationStore()

			for _, d := range tt.setupDrivers {
				driverRepo.SetDriver(d)
			}
			if tt.setupError != nil {
				driverRepo.SetError(tt.setupError)
			}

			query := queries.NewFindAvailableDriversQuery(37.7749, -122.4194, 5.0, tt.vehicleType, "delivery-1", driverRepo, locationStore)

			candidates, err := query.Execute(context.Background())

			if tt.setupError != nil {
				if err == nil {
					t.Errorf("Execute() expected error %v, got nil", tt.setupError)
				} else if err.Error() != tt.setupError.Error() {
					t.Errorf("Execute() error = %v, want %v", err, tt.setupError)
				}
				if candidates != nil {
					t.Errorf("Execute() should return nil candidates on error")
				}
			} else {
				if err != nil {
					t.Errorf("Execute() unexpected error: %v", err)
				}
				if len(candidates) != tt.expectedCount {
					t.Errorf("Candidates count = %v, want %v", len(candidates), tt.expectedCount)
				}
			}
		})
	}
}