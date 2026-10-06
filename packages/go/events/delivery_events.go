package events

import "time"

type DeliveryEventType string

const (
	DeliveryCreated        DeliveryEventType = "delivery.created"
	DeliveryDriverAssigned DeliveryEventType = "delivery.driver.assigned"
	DeliveryDriverAccepted DeliveryEventType = "delivery.driver.accepted"
	DeliveryPickedUp       DeliveryEventType = "delivery.picked_up"
	DeliveryInTransit      DeliveryEventType = "delivery.in_transit"
	DeliveryCompleted      DeliveryEventType = "delivery.completed"
	DeliveryCancelled      DeliveryEventType = "delivery.cancelled"
	DeliveryDeleted        DeliveryEventType = "delivery.deleted"
)

type DeliveryLocation struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type DeliveryAddress struct {
	City     string           `json:"city"`
	Country  string           `json:"country"`
	Location DeliveryLocation `json:"location"`
}

type DeliveryCreatedPayload struct {
	DeliveryID      string          `json:"deliveryId"`
	CustomerID      string          `json:"customerId"`
	DriverID        string          `json:"driverId,omitempty"`
	Status          string          `json:"status"`
	Pickup          DeliveryAddress `json:"pickup"`
	Dropoff         DeliveryAddress `json:"dropoff"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	SourceVersion   int64           `json:"sourceVersion"`
}

type DeliveryUpdatedPayload struct {
	DeliveryID    string    `json:"deliveryId"`
	CustomerID    string    `json:"customerId"`
	DriverID      string    `json:"driverId,omitempty"`
	Status        string    `json:"status"`
	UpdatedAt     time.Time `json:"updatedAt"`
	SourceVersion int64     `json:"sourceVersion"`
}

type DeliveryDriverAssignedPayload struct {
	DeliveryID    string    `json:"deliveryId"`
	DriverID      string    `json:"driverId"`
	AssignedAt    time.Time `json:"assignedAt"`
	SourceVersion int64     `json:"sourceVersion"`
}

type DeliveryDeletedPayload struct {
	DeliveryID string    `json:"deliveryId"`
	DeletedAt  time.Time `json:"deletedAt"`
}

type DeliveryEventEnvelope = EventEnvelope
