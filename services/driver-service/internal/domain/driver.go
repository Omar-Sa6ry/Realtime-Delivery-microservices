package domain

import (
	"sync"
	"time"
)

// Driver represents a driver aggregate root with its state machine.
type Driver struct {
	mu         sync.Mutex
	ID         string
	UserID     string
	Status     DriverStatus
	Vehicle    VehicleInfo
	Capabilities []string
	ServiceArea string
	Version     int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// DriverStatus represents the operational state of a driver.
type DriverStatus string

const (
	DriverStatusOffline  DriverStatus = "OFFLINE"
	DriverStatusAvailable DriverStatus = "AVAILABLE"
	DriverStatusBusy     DriverStatus = "BUSY"
	DriverStatusSuspended DriverStatus = "SUSPENDED"
	DriverStatusBlocked  DriverStatus = "BLOCKED"
)

// Valid statuses for the public state machine.
var validDriverStatuses = map[DriverStatus]bool{
	DriverStatusOffline:  true,
	DriverStatusAvailable: true,
	DriverStatusBusy:     true,
}

// DriverStatusTransition represents a transition action.
type DriverStatusTransition struct {
	From   DriverStatus
	Action string
	To     DriverStatus
}

// DriverTransitionMap maps actions to allowed transitions for each state.
var DriverTransitionMap = map[DriverStatus][]DriverStatusTransition{
	DriverStatusOffline: {
		{From: DriverStatusOffline, Action: "GoOnline", To: DriverStatusAvailable},
	},
	DriverStatusAvailable: {
		{From: DriverStatusAvailable, Action: "GoOffline", To: DriverStatusOffline},
		{From: DriverStatusAvailable, Action: "Reserve", To: DriverStatusBusy},
	},
	DriverStatusBusy: {
		{From: DriverStatusBusy, Action: "CompleteDelivery", To: DriverStatusAvailable},
	},
}

// IsValidTransition checks if a transition from one status to another via an action is valid.
func IsValidDriverTransition(from, to DriverStatus, action string) bool {
	transitions, ok := DriverTransitionMap[from]
	if !ok {
		return false
	}
	for _, t := range transitions {
		if t.From == from && t.Action == action && t.To == to {
			return true
		}
	}
	return false
}

// GoOnline transitions a driver from OFFLINE to AVAILABLE.
func (d *Driver) GoOnline() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !IsValidDriverTransition(d.Status, DriverStatusAvailable, "GoOnline") {
		return ErrInvalidTransition
	}
	d.Status = DriverStatusAvailable
	d.UpdatedAt = time.Now()
	return nil
}

// GoOffline transitions a driver from AVAILABLE to OFFLINE.
func (d *Driver) GoOffline() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !IsValidDriverTransition(d.Status, DriverStatusOffline, "GoOffline") {
		return ErrInvalidTransition
	}
	d.Status = DriverStatusOffline
	d.UpdatedAt = time.Now()
	return nil
}

// Reserve transitions a driver from AVAILABLE to BUSY (reserved for a delivery).
func (d *Driver) Reserve() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !IsValidDriverTransition(d.Status, DriverStatusBusy, "Reserve") {
		return ErrInvalidTransition
	}
	d.Status = DriverStatusBusy
	d.UpdatedAt = time.Now()
	return nil
}

// CompleteDelivery transitions a driver from BUSY to AVAILABLE after delivery completion.
func (d *Driver) CompleteDelivery() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !IsValidDriverTransition(d.Status, DriverStatusAvailable, "CompleteDelivery") {
		return ErrInvalidTransition
	}
	d.Status = DriverStatusAvailable
	d.UpdatedAt = time.Now()
	return nil
}

// String returns the human-readable status.
func (d *Driver) String() string {
	return string(d.Status)
}

// VehicleInfo represents vehicle details for a driver.
type VehicleInfo struct {
	Type       string `json:"type"`        // CAR, MOTORCYCLE, TRUCK
	PlateNumber string `json:"plateNumber"`
	CapacityKg  int64  `json:"capacityKg"`
}

// IsValid checks if the driver status is a valid public state.
func (s DriverStatus) IsValid() bool {
	return validDriverStatuses[s]
}

// Ensure DriverStatusIsValidError checks if status is valid.
func IsDriverStatusValid(s DriverStatus) bool {
	return s.IsValid()
}