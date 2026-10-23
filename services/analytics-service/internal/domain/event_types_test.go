package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKnownDeliveryEventTypes(t *testing.T) {
	expected := []string{
		DeliveryCreated,
		DeliveryDriverAssigned,
		DeliveryDriverAccepted,
		DeliveryPickupStarted,
		DeliveryPickedUp,
		DeliveryInTransit,
		DeliveryCompleted,
		DeliveryCancelled,
		DeliveryFailed,
		DeliveryDeleted,
	}

	for _, e := range expected {
		assert.True(t, KnownDeliveryEventTypes[e], "event type %s should be known", e)
	}

	assert.False(t, KnownDeliveryEventTypes["unknown.event"])
}