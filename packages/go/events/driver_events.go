package events

import "time"

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
	DriverNoDriverAvailable   DriverEventType = "driver.no_driver_available"
	DriverSearchRetry         DriverEventType = "driver.search.retry"
)

type DriverGeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

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

type DriverDeletedPayload struct {
	DriverID  string    `json:"driverId"`
	DeletedAt time.Time `json:"deletedAt"`
}

type DriverActivatedPayload struct {
	DriverID string `json:"driverId"`
}

type DriverDeactivatedPayload struct {
	DriverID string `json:"driverId"`
}

type DriverAvailablePayload struct {
	DriverID string `json:"driverId"`
}

type DriverUnavailablePayload struct {
	DriverID string `json:"driverId"`
}

type DriverAssignmentOfferedPayload struct {
	AssignmentID   string           `json:"assignmentId"`
	DriverID       string           `json:"driverId"`
	DeliveryID     string           `json:"deliveryId"`
	ExpiresAt      string           `json:"expiresAt"`
	RadiusKm       float64          `json:"radiusKm"`
	DistanceMeters float64          `json:"distanceMeters,omitempty"`
	PickupAddress  *DeliveryAddress `json:"pickupAddress,omitempty"`
	DropoffAddress *DeliveryAddress `json:"dropoffAddress,omitempty"`
	Amount         string           `json:"amount,omitempty"`
	Currency       string           `json:"currency,omitempty"`
}

type DriverAssignmentAcceptedPayload struct {
	AssignmentID string `json:"assignmentId"`
	DeliveryID   string `json:"deliveryId,omitempty"`
	DriverID     string `json:"driverId"`
	AcceptedAt   string `json:"acceptedAt"`
}

type DriverAssignmentRejectedPayload struct {
	AssignmentID string `json:"assignmentId"`
	DeliveryID   string `json:"deliveryId,omitempty"`
	DriverID     string `json:"driverId"`
	Reason       string `json:"reason"`
}

type DriverAssignmentExpiredPayload struct {
	AssignmentID string `json:"assignmentId"`
	DeliveryID   string `json:"deliveryId,omitempty"`
	DriverID     string `json:"driverId"`
	ExpiredAt    string `json:"expiredAt"`
}

type DriverAssignmentReleasedPayload struct {
	AssignmentID string `json:"assignmentId"`
	DriverID     string `json:"driverId"`
	ReleasedAt   string `json:"releasedAt"`
}

type DriverAssignmentCompletedPayload struct {
	AssignmentID string `json:"assignmentId"`
	DriverID     string `json:"driverId"`
	CompletedAt  string `json:"completedAt"`
}

type DriverNoDriverAvailablePayload struct {
	DeliveryID    string `json:"deliveryId"`
	CustomerID    string `json:"customerId"`
	AttemptNumber int    `json:"attemptNumber"`
	Reason        string `json:"reason"` // "NO_DRIVERS_NEARBY" | "ALL_DRIVERS_BUSY"
	TriedAt       string `json:"triedAt"`
}

type DriverSearchRetryPayload struct {
	DeliveryID    string `json:"deliveryId"`
	CustomerID    string `json:"customerId"`
	AttemptNumber int    `json:"attemptNumber"`
	NextRetryAt   string `json:"nextRetryAt,omitempty"`
	RetryInterval int    `json:"retryInterval"` // in seconds (e.g. 30)
}

