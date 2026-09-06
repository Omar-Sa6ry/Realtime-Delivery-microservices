package nats

import (
	"fmt"
	"github.com/nats-io/nats.go"
)

// NATSPublisher publishes events to NATS subjects.
type NATSPublisher struct {
	client *nats.Conn
}

// NewNATSPublisher creates a new NATSPublisher.
func NewNATSPublisher(url string) (*NATSPublisher, error) {
	client, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &NATSPublisher{client: client}, nil
}

// Publish publishes a message to a NATS subject.
func (p *NATSPublisher) Publish(subject, payload string) error {
	return p.client.Publish(subject, []byte(payload))
}

// PublishDriverLocationUpdated publishes a driver.location.updated event.
func (p *NATSPublisher) PublishDriverLocationUpdated(driverID, latitude, longitude string) error {
	return p.Publish("driver.location.updated", fmt.Sprintf(`{"driverId":"%s","lat":"%s","lng":"%s"}`, driverID, latitude, longitude))
}