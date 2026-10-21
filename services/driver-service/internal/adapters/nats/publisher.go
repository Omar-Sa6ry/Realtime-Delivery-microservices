package nats

import (
	"fmt"
	"github.com/nats-io/nats.go"
)

type NATSPublisher struct {
	client *nats.Conn
}

func NewNATSPublisher(url string) (*NATSPublisher, error) {
	client, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &NATSPublisher{client: client}, nil
}

func (p *NATSPublisher) Publish(subject, payload string) error {
	return p.client.Publish(subject, []byte(payload))
}

func (p *NATSPublisher) PublishDriverLocationUpdated(driverID, latitude, longitude string) error {
	return p.Publish("driver.location.updated", fmt.Sprintf(`{"driverId":"%s","lat":"%s","lng":"%s"}`, driverID, latitude, longitude))
}