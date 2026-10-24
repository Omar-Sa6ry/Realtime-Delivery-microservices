package domain_test

import (
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

func TestDriverStatus_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		status   domain.DriverStatus
		expected bool
	}{
		{"Offline is valid", domain.DriverStatusOffline, true},
		{"Available is valid", domain.DriverStatusAvailable, true},
		{"Busy is valid", domain.DriverStatusBusy, true},
		{"Suspended is invalid", domain.DriverStatusSuspended, false},
		{"Blocked is invalid", domain.DriverStatusBlocked, false},
		{"Unknown is invalid", domain.DriverStatus("UNKNOWN"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsDriverStatusValid(t *testing.T) {
	tests := []struct {
		name     string
		status   domain.DriverStatus
		expected bool
	}{
		{"Offline is valid", domain.DriverStatusOffline, true},
		{"Available is valid", domain.DriverStatusAvailable, true},
		{"Busy is valid", domain.DriverStatusBusy, true},
		{"Suspended is invalid", domain.DriverStatusSuspended, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domain.IsDriverStatusValid(tt.status)
			if result != tt.expected {
				t.Errorf("IsDriverStatusValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsValidDriverTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     domain.DriverStatus
		to       domain.DriverStatus
		action   string
		expected bool
	}{
		{"Offline to Available via GoOnline", domain.DriverStatusOffline, domain.DriverStatusAvailable, "GoOnline", true},
		{"Available to Offline via GoOffline", domain.DriverStatusAvailable, domain.DriverStatusOffline, "GoOffline", true},
		{"Available to Busy via Reserve", domain.DriverStatusAvailable, domain.DriverStatusBusy, "Reserve", true},
		{"Busy to Available via CompleteDelivery", domain.DriverStatusBusy, domain.DriverStatusAvailable, "CompleteDelivery", true},
		{"Offline to Busy invalid", domain.DriverStatusOffline, domain.DriverStatusBusy, "Reserve", false},
		{"Available to Available invalid", domain.DriverStatusAvailable, domain.DriverStatusAvailable, "GoOnline", false},
		{"Busy to Offline invalid", domain.DriverStatusBusy, domain.DriverStatusOffline, "GoOffline", false},
		{"Unknown from invalid", domain.DriverStatus("UNKNOWN"), domain.DriverStatusAvailable, "GoOnline", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domain.IsValidDriverTransition(tt.from, tt.to, tt.action)
			if result != tt.expected {
				t.Errorf("IsValidDriverTransition(%v, %v, %q) = %v, want %v", tt.from, tt.to, tt.action, result, tt.expected)
			}
		})
	}
}

func TestDriver_GoOnline(t *testing.T) {
	tests := []struct {
		name          string
		driver        *domain.Driver
		expectedError error
		expectedStatus domain.DriverStatus
	}{
		{
			name: "success from offline",
			driver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusOffline,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError: nil,
			expectedStatus: domain.DriverStatusAvailable,
		},
		{
			name: "fails when driver is blocked",
			driver: &domain.Driver{
				ID:        "driver-2",
				Status:    domain.DriverStatusOffline,
				IsActive:  true,
				IsBlocked: true,
			},
			expectedError:   domain.ErrDriverBlocked,
			expectedStatus: domain.DriverStatusOffline,
		},
		{
			name: "fails when driver is not active",
			driver: &domain.Driver{
				ID:        "driver-3",
				Status:    domain.DriverStatusOffline,
				IsActive:  false,
				IsBlocked: false,
			},
			expectedError:   domain.ErrDriverNotActive,
			expectedStatus: domain.DriverStatusOffline,
		},
		{
			name: "fails when already available",
			driver: &domain.Driver{
				ID:        "driver-4",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError:   domain.ErrInvalidTransition,
			expectedStatus: domain.DriverStatusAvailable,
		},
		{
			name: "fails when busy",
			driver: &domain.Driver{
				ID:        "driver-5",
				Status:    domain.DriverStatusBusy,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError:   domain.ErrInvalidTransition,
			expectedStatus: domain.DriverStatusBusy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.driver.GoOnline()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("GoOnline() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("GoOnline() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("GoOnline() unexpected error: %v", err)
				}
			}
			if tt.driver.Status != tt.expectedStatus {
				t.Errorf("Driver status = %v, want %v", tt.driver.Status, tt.expectedStatus)
			}
		})
	}
}

func TestDriver_GoOffline(t *testing.T) {
	tests := []struct {
		name          string
		driver        *domain.Driver
		expectedError error
		expectedStatus domain.DriverStatus
	}{
		{
			name: "success from available",
			driver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError: nil,
			expectedStatus: domain.DriverStatusOffline,
		},
		{
			name: "fails when already offline",
			driver: &domain.Driver{
				ID:        "driver-2",
				Status:    domain.DriverStatusOffline,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError:   domain.ErrInvalidTransition,
			expectedStatus: domain.DriverStatusOffline,
		},
		{
			name: "fails when busy",
			driver: &domain.Driver{
				ID:        "driver-3",
				Status:    domain.DriverStatusBusy,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError:   domain.ErrInvalidTransition,
			expectedStatus: domain.DriverStatusBusy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.driver.GoOffline()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("GoOffline() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("GoOffline() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("GoOffline() unexpected error: %v", err)
				}
			}
			if tt.driver.Status != tt.expectedStatus {
				t.Errorf("Driver status = %v, want %v", tt.driver.Status, tt.expectedStatus)
			}
		})
	}
}

func TestDriver_Reserve(t *testing.T) {
	tests := []struct {
		name          string
		driver        *domain.Driver
		expectedError error
		expectedStatus domain.DriverStatus
	}{
		{
			name: "success from available",
			driver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError: nil,
			expectedStatus: domain.DriverStatusBusy,
		},
		{
			name: "fails when blocked",
			driver: &domain.Driver{
				ID:        "driver-2",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: true,
			},
			expectedError:   domain.ErrDriverBlocked,
			expectedStatus: domain.DriverStatusAvailable,
		},
		{
			name: "fails when not active",
			driver: &domain.Driver{
				ID:        "driver-3",
				Status:    domain.DriverStatusAvailable,
				IsActive:  false,
				IsBlocked: false,
			},
			expectedError:   domain.ErrDriverNotActive,
			expectedStatus: domain.DriverStatusAvailable,
		},
		{
			name: "fails when not available",
			driver: &domain.Driver{
				ID:        "driver-4",
				Status:    domain.DriverStatusOffline,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError:   domain.ErrInvalidTransition,
			expectedStatus: domain.DriverStatusOffline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.driver.Reserve()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Reserve() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("Reserve() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("Reserve() unexpected error: %v", err)
				}
			}
			if tt.driver.Status != tt.expectedStatus {
				t.Errorf("Driver status = %v, want %v", tt.driver.Status, tt.expectedStatus)
			}
		})
	}
}

func TestDriver_CompleteDelivery(t *testing.T) {
	tests := []struct {
		name          string
		driver        *domain.Driver
		expectedError error
		expectedStatus domain.DriverStatus
	}{
		{
			name: "success from busy",
			driver: &domain.Driver{
				ID:        "driver-1",
				Status:    domain.DriverStatusBusy,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError: nil,
			expectedStatus: domain.DriverStatusAvailable,
		},
		{
			name: "fails when not busy",
			driver: &domain.Driver{
				ID:        "driver-2",
				Status:    domain.DriverStatusAvailable,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError:   domain.ErrInvalidTransition,
			expectedStatus: domain.DriverStatusAvailable,
		},
		{
			name: "fails when offline",
			driver: &domain.Driver{
				ID:        "driver-3",
				Status:    domain.DriverStatusOffline,
				IsActive:  true,
				IsBlocked: false,
			},
			expectedError:   domain.ErrInvalidTransition,
			expectedStatus: domain.DriverStatusOffline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.driver.CompleteDelivery()
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("CompleteDelivery() expected error %v, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError.Error() {
					t.Errorf("CompleteDelivery() error = %v, want %v", err, tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("CompleteDelivery() unexpected error: %v", err)
				}
			}
			if tt.driver.Status != tt.expectedStatus {
				t.Errorf("Driver status = %v, want %v", tt.driver.Status, tt.expectedStatus)
			}
		})
	}
}

func TestDriver_Block(t *testing.T) {
	driver := &domain.Driver{
		ID:        "driver-1",
		IsBlocked: false,
	}
	err := driver.Block()
	if err != nil {
		t.Errorf("Block() unexpected error: %v", err)
	}
	if !driver.IsBlocked {
		t.Errorf("Driver should be blocked")
	}
}

func TestDriver_Unblock(t *testing.T) {
	driver := &domain.Driver{
		ID:        "driver-1",
		IsBlocked: true,
	}
	err := driver.Unblock()
	if err != nil {
		t.Errorf("Unblock() unexpected error: %v", err)
	}
	if driver.IsBlocked {
		t.Errorf("Driver should be unblocked")
	}
}

func TestDriver_Activate(t *testing.T) {
	driver := &domain.Driver{
		ID:       "driver-1",
		IsActive: false,
	}
	err := driver.Activate()
	if err != nil {
		t.Errorf("Activate() unexpected error: %v", err)
	}
	if !driver.IsActive {
		t.Errorf("Driver should be active")
	}
}

func TestDriver_Deactivate(t *testing.T) {
	tests := []struct {
		name          string
		driver        *domain.Driver
		expectedStatus domain.DriverStatus
	}{
		{
			name: "deactivate available driver sets offline",
			driver: &domain.Driver{
				ID:       "driver-1",
				Status:   domain.DriverStatusAvailable,
				IsActive: true,
			},
			expectedStatus: domain.DriverStatusOffline,
		},
		{
			name: "deactivate offline driver stays offline",
			driver: &domain.Driver{
				ID:       "driver-2",
				Status:   domain.DriverStatusOffline,
				IsActive: true,
			},
			expectedStatus: domain.DriverStatusOffline,
		},
		{
			name: "deactivate busy driver stays busy",
			driver: &domain.Driver{
				ID:       "driver-3",
				Status:   domain.DriverStatusBusy,
				IsActive: true,
			},
			expectedStatus: domain.DriverStatusBusy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.driver.Deactivate()
			if err != nil {
				t.Errorf("Deactivate() unexpected error: %v", err)
			}
			if tt.driver.IsActive {
				t.Errorf("Driver should not be active")
			}
			if tt.driver.Status != tt.expectedStatus {
				t.Errorf("Driver status = %v, want %v", tt.driver.Status, tt.expectedStatus)
			}
		})
	}
}

func TestDriver_UpdateRating(t *testing.T) {
	driver := &domain.Driver{
		ID:          "driver-1",
		Rating:      4.0,
		RatingCount: 10,
	}

	driver.UpdateRating(4.5, 15)

	if driver.Rating != 4.5 {
		t.Errorf("Rating = %v, want 4.5", driver.Rating)
	}
	if driver.RatingCount != 15 {
		t.Errorf("RatingCount = %v, want 15", driver.RatingCount)
	}
}

func TestDriver_String(t *testing.T) {
	driver := &domain.Driver{
		Status: domain.DriverStatusAvailable,
	}
	if driver.String() != "AVAILABLE" {
		t.Errorf("String() = %v, want AVAILABLE", driver.String())
	}
}

func TestVehicleType_Constants(t *testing.T) {
	if domain.VehicleTypeCar != "CAR" {
		t.Errorf("VehicleTypeCar = %v, want CAR", domain.VehicleTypeCar)
	}
	if domain.VehicleTypeMotorcycle != "MOTORCYCLE" {
		t.Errorf("VehicleTypeMotorcycle = %v, want MOTORCYCLE", domain.VehicleTypeMotorcycle)
	}
	if domain.VehicleTypeTruck != "TRUCK" {
		t.Errorf("VehicleTypeTruck = %v, want TRUCK", domain.VehicleTypeTruck)
	}
	if domain.VehicleTypeBicycle != "BICYCLE" {
		t.Errorf("VehicleTypeBicycle = %v, want BICYCLE", domain.VehicleTypeBicycle)
	}
	if domain.VehicleTypeVan != "VAN" {
		t.Errorf("VehicleTypeVan = %v, want VAN", domain.VehicleTypeVan)
	}
}