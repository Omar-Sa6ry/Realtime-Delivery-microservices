package events

import "time"

// DriverEventType is the canonical event type for driver domain Kafka messages.
type DriverEventType string

const (
	DriverCreated DriverEventType = "driver.created"
	DriverUpdated DriverEventType = "driver.updated"
	DriverDeleted DriverEventType = "driver.deleted"
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
