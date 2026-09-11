package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	pkgKafka "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/services"
	kafkago "github.com/segmentio/kafka-go"
)

// DeliveryCreatedConsumer listens for delivery.created events and dispatches an available driver.
type DeliveryCreatedConsumer struct {
	consumer    *pkgKafka.Consumer
	dispatchSvc *services.DispatchService
}

// DeliveryCreatedPayloadExtended captures the payload fields sent by delivery-service outbox.
type DeliveryCreatedPayloadExtended struct {
	DeliveryID string `json:"deliveryId"`
	CustomerID string `json:"customerId"`
	DriverID   string `json:"driverId,omitempty"`
	Status     string `json:"status"`
	Amount     string `json:"amount,omitempty"`
	Currency   string `json:"currency,omitempty"`
	Pickup     struct {
		City      string      `json:"city"`
		Country   string      `json:"countryCode"`
		Latitude  interface{} `json:"latitude"`
		Longitude interface{} `json:"longitude"`
		Location  struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"location"`
	} `json:"pickup"`
	Dropoff struct {
		City      string      `json:"city"`
		Country   string      `json:"countryCode"`
		Latitude  interface{} `json:"latitude"`
		Longitude interface{} `json:"longitude"`
	} `json:"dropoff"`
}

func parseCoordinate(val interface{}, fallback float64) float64 {
	if val == nil {
		return fallback
	}
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

// NewDeliveryCreatedConsumer initializes the consumer.
func NewDeliveryCreatedConsumer(brokers []string, groupID string, dispatchSvc *services.DispatchService) *DeliveryCreatedConsumer {
	consumer := pkgKafka.NewConsumer(pkgKafka.ConsumerConfig{
		Brokers:    brokers,
		Topic:      string(events.DeliveryCreated),
		GroupID:    groupID,
		MaxRetries: 3,
	})

	return &DeliveryCreatedConsumer{
		consumer:    consumer,
		dispatchSvc: dispatchSvc,
	}
}

// Start runs the consumer loop until ctx is cancelled.
func (c *DeliveryCreatedConsumer) Start(ctx context.Context) error {
	slog.Info("DeliveryCreatedConsumer started listening for delivery.created events")
	return c.consumer.Run(ctx, c.handleMessage)
}

// Close closes the underlying consumer.
func (c *DeliveryCreatedConsumer) Close() error {
	return c.consumer.Close()
}

func (c *DeliveryCreatedConsumer) handleMessage(ctx context.Context, msg kafkago.Message) error {
	env, err := events.UnmarshalEnvelope(msg.Value)
	if err != nil {
		slog.Error("DeliveryCreatedConsumer: failed to unmarshal event envelope", "error", err)
		return fmt.Errorf("%w: invalid envelope json: %v", pkgKafka.ErrPermanent, err)
	}

	if env.EventType != string(events.DeliveryCreated) {
		slog.Debug("DeliveryCreatedConsumer: skipping event", "eventType", env.EventType)
		return nil
	}

	var payload DeliveryCreatedPayloadExtended
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		slog.Error("DeliveryCreatedConsumer: failed to unmarshal delivery.created payload", "error", err)
		return fmt.Errorf("%w: invalid payload json: %v", pkgKafka.ErrPermanent, err)
	}

	if payload.DeliveryID == "" {
		slog.Warn("DeliveryCreatedConsumer: missing deliveryId in payload, skipping")
		return nil
	}

	lat := parseCoordinate(payload.Pickup.Latitude, payload.Pickup.Location.Lat)
	lon := parseCoordinate(payload.Pickup.Longitude, payload.Pickup.Location.Lon)

	slog.Info("DeliveryCreatedConsumer: processing delivery",
		"deliveryId", payload.DeliveryID,
		"lat", lat,
		"lon", lon,
	)

	// 1. Search for available drivers near pickup location
	candidates, err := c.dispatchSvc.FindAvailableDrivers(ctx, lat, lon, 50.0, "", payload.DeliveryID)
	if err != nil {
		slog.Error("DeliveryCreatedConsumer: error searching for available drivers", "deliveryId", payload.DeliveryID, "error", err)
		return err
	}

	if len(candidates) == 0 {
		slog.Warn("DeliveryCreatedConsumer: no available drivers found for delivery", "deliveryId", payload.DeliveryID)
		return nil
	}

	// 2. Iterate candidates to reserve the first available driver (creates OFFERED assignment)
	var reserved bool
	for _, candidate := range candidates {
		slog.Info("DeliveryCreatedConsumer: attempting to reserve driver for delivery",
			"deliveryId", payload.DeliveryID,
			"driverId", candidate.DriverID,
		)

		var err error
		reserved, err = c.dispatchSvc.ReserveDriver(ctx, candidate.DriverID, payload.DeliveryID)
		if err != nil {
			slog.Warn("DeliveryCreatedConsumer: candidate could not be reserved, trying next",
				"driverId", candidate.DriverID,
				"error", err,
			)
			continue
		}
		if reserved {
			slog.Info("DeliveryCreatedConsumer: successfully assigned and offered delivery to driver",
				"deliveryId", payload.DeliveryID,
				"driverId", candidate.DriverID,
			)
			break
		}
	}

	if !reserved {
		slog.Warn("DeliveryCreatedConsumer: no candidates could be reserved at this moment for delivery", "deliveryId", payload.DeliveryID)
	}

	return nil
}
