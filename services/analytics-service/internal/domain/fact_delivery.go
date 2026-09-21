package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	DeliveryCreated        = "delivery.created"
	DeliveryDriverAssigned = "delivery.driver.assigned"
	DeliveryDriverAccepted = "delivery.driver.accepted"
	DeliveryPickupStarted  = "delivery.pickup.started"
	DeliveryPickedUp       = "delivery.picked_up"
	DeliveryInTransit      = "delivery.in_transit"
	DeliveryCompleted      = "delivery.completed"
	DeliveryCancelled      = "delivery.cancelled"
	DeliveryFailed         = "delivery.failed"
	DeliveryDeleted        = "delivery.deleted"
)

var KnownDeliveryEventTypes = map[string]bool{
	DeliveryCreated:        true,
	DeliveryDriverAssigned: true,
	DeliveryDriverAccepted: true,
	DeliveryPickupStarted:  true,
	DeliveryPickedUp:       true,
	DeliveryInTransit:      true,
	DeliveryCompleted:      true,
	DeliveryCancelled:      true,
	DeliveryFailed:         true,
	DeliveryDeleted:        true,
}

type FactDeliveryEvent struct {
	EventID       string
	DeliveryID    string
	UserID        string
	DriverID      string
	EventType     string
	EventVersion  EventVersion
	CityID        string
	ZoneID        string
	OccurredAt    time.Time
	IngestedAt    time.Time
	CorrelationID string
}

func (f *FactDeliveryEvent) Validate() error {
	if f == nil {
		return ErrInvalidEnvelope
	}
	if strings.TrimSpace(f.EventID) == "" {
		return ErrMissingEventID
	}
	if strings.TrimSpace(f.DeliveryID) == "" {
		return ErrMissingDeliveryID
	}
	if !KnownDeliveryEventTypes[f.EventType] {
		return fmt.Errorf("%w: %s", ErrUnknownEventType, f.EventType)
	}
	if f.OccurredAt.IsZero() {
		return ErrInvalidOccurredAt
	}
	return nil
}

type FactDeliveryCompleted struct {
	DeliveryID          string
	UserID              string
	DriverID            string
	CreatedAt           time.Time
	AssignedAt          *time.Time
	AcceptedAt          *time.Time
	PickupStartedAt     *time.Time
	PickedUpAt          *time.Time
	InTransitAt         *time.Time
	DeliveredAt         *time.Time
	CompletedAt         *time.Time
	TotalDurationS      *uint32
	AssignmentDurationS *uint32
	PickupDurationS     *uint32
	TransitDurationS    *uint32
	CityID              string
	IngestedAt          time.Time
}

func durationSeconds(from time.Time, to time.Time, metric string) (uint32, error) {
	d := to.Sub(from)
	if d < 0 {
		return 0, fmt.Errorf("%w: %s", ErrNegativeDuration, metric)
	}
	return uint32(d.Seconds()), nil
}

func (f *FactDeliveryCompleted) ComputeDurations() error {
	if f == nil {
		return ErrInvalidEnvelope
	}
	set := func(dst **uint32, from time.Time, to *time.Time, metric string) error {
		if to == nil {
			*dst = nil
			return nil
		}
		v, err := durationSeconds(from, *to, metric)
		if err != nil {
			return err
		}
		*dst = &v
		return nil
	}
	if err := set(&f.TotalDurationS, f.CreatedAt, f.CompletedAt, "total_delivery_duration"); err != nil {
		return err
	}
	if f.AssignedAt != nil {
		if err := set(&f.AssignmentDurationS, f.CreatedAt, f.AssignedAt, "assignment_time"); err != nil {
			return err
		}
	}
	if f.PickupStartedAt != nil {
		if err := set(&f.PickupDurationS, *f.PickupStartedAt, f.PickedUpAt, "pickup_duration"); err != nil {
			return err
		}
	}
	if f.InTransitAt != nil {
		end := f.CompletedAt
		if end == nil {
			end = f.DeliveredAt
		}
		if err := set(&f.TransitDurationS, *f.InTransitAt, end, "transit_duration"); err != nil {
			return err
		}
	}
	return nil
}

func (f *FactDeliveryCompleted) Validate() error {
	if f == nil {
		return ErrInvalidEnvelope
	}
	if strings.TrimSpace(f.DeliveryID) == "" {
		return ErrMissingDeliveryID
	}
	if f.CreatedAt.IsZero() {
		return ErrInvalidOccurredAt
	}
	ordered := []struct {
		name string
		t    *time.Time
	}{
		{"assigned_at", f.AssignedAt},
		{"accepted_at", f.AcceptedAt},
		{"pickup_started_at", f.PickupStartedAt},
		{"picked_up_at", f.PickedUpAt},
		{"in_transit_at", f.InTransitAt},
		{"delivered_at", f.DeliveredAt},
		{"completed_at", f.CompletedAt},
	}
	for _, o := range ordered {
		if o.t != nil && o.t.Before(f.CreatedAt) {
			return fmt.Errorf("%w: %s before created_at", ErrOutOfOrderTimestamps, o.name)
		}
	}
	return nil
}
