package events

import "time"

// DriverEventType is the canonical event type for driver domain Kafka messages.
type DriverEventType string

const (
	DriverCreated             DriverEventType = "driver.created"
	DriverUpdated             DriverEventType = "driver.updated"
	DriverDeleted             DriverEventType = "driver.deleted"
	DriverActivated           DriverEventType = "driver.activated"
	DriverDeactivated         DriverEventType = "driver.deactivated"
	DriverAvailable           DriverEventType = "driver.available"
	DriverUnavailable         DriverEventType = "driver.unavailable"
	DriverAssignmentOffered   DriverEventType = "driver.assignment.offered"
	DriverAssignmentAccepted  DriverEventType = "driver.assignment.accepted"
	DriverAssignmentRejected  DriverEventType = "driver.assignment.rejected"
	DriverAssignmentExpired   DriverEventType = "driver.assignment.expired"
	DriverAssignmentReleased  DriverEventType = "driver.assignment.released"
	DriverAssignmentCompleted DriverEventType = "driver.assignment.completed"
)

// DriverGeoPoint holds a geographic coordinate for a driver's location.
type DriverGeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// DriverCreatedPayload is emitted when a driver profile is first created.
type DriverCreatedPayload struct {
	DriverID      string         `json:"driverId"`
	Name          string         `json:"name"`
	Status        string         `json:"status"`      // AVAILABLE | BUSY | OFFLINE
	VehicleType   string         `json:"vehicleType"` // CAR | MOTORCYCLE | TRUCK
	Rating        float64        `json:"rating"`
	Location      *DriverGeoPoint `json:"location,omitempty"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	SourceVersion int64          `json:"sourceVersion"`
}

type DriverUpdatedPayload struct {
	DriverID      string         `json:"driverId"`
	Name          string         `json:"name,omitempty"`
	Status        string         `json:"status,omitempty"`
	VehicleType   string         `json:"vehicleType,omitempty"`
	Rating        float64        `json:"rating,omitempty"`
	Location      *DriverGeoPoint `json:"location,omitempty"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	SourceVersion int64          `json:"sourceVersion"`
}

// DriverDeletedPayload is emitted when a driver profile is permanently removed.
type DriverDeletedPayload struct {
	DriverID  string    `json:"driverId"`
	DeletedAt time.Time `json:"deletedAt"`
}

// DriverActivatedPayload is emitted when a driver is activated.
type DriverActivatedPayload struct {
	DriverID string `json:"driverId"`
}

// DriverDeactivatedPayload is emitted when a driver is deactivated.
type DriverDeactivatedPayload struct {
	DriverID string `json:"driverId"`
}

// DriverAvailablePayload is emitted when a driver becomes available.
type DriverAvailablePayload struct {
	DriverID string `json:"driverId"`
}

// DriverUnavailablePayload is emitted when a driver becomes unavailable.
type DriverUnavailablePayload struct {
	DriverID string `json:"driverId"`
}

// DriverAssignmentOfferedPayload is emitted when a driver is offered an assignment.
type DriverAssignmentOfferedPayload struct {
	AssignmentID   string  `json:"assignmentId"`
	DriverID       string  `json:"driverId"`
	DeliveryID     string  `json:"deliveryId"`
	ExpiresAt      string  `json:"expiresAt"`
	RadiusKm       float64 `json:"radiusKm"`
}

// DriverAssignmentAcceptedPayload is emitted when a driver accepts an assignment.
type DriverAssignmentAcceptedPayload struct {
	AssignmentID string `json:"assignmentId"`
	DriverID     string `json:"driverId"`
	AcceptedAt   string `json:"acceptedAt"`
}

// DriverAssignmentRejectedPayload is emitted when a driver rejects an assignment.
type DriverAssignmentRejectedPayload struct {
	AssignmentID string `json:"assignmentId"`
	DriverID     string `json:"driverId"`
	Reason       string `json:"reason"`
}

// DriverAssignmentExpiredPayload is emitted when a driver assignment offer expires.
type DriverAssignmentExpiredPayload struct {
	AssignmentID string `json:"assignmentId"`
	DriverID     string `json:"driverId"`
	ExpiredAt    string `json:"expiredAt"`
}

// DriverAssignmentReleasedPayload is emitted when a driver assignment is released.
type DriverAssignmentReleasedPayload struct {
	AssignmentID string `json:"assignmentId"`
	DriverID     string `json:"driverId"`
	ReleasedAt   string `json:"releasedAt"`
}

// DriverAssignmentCompletedPayload is emitted when a driver assignment is completed.
type DriverAssignmentCompletedPayload struct {
	AssignmentID string `json:"assignmentId"`
	DriverID     string `json:"driverId"`
	CompletedAt  string `json:"completedAt"`
}
