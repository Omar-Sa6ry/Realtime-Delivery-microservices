package observability

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	logger := Init()
	assert.NotNil(t, logger)
}

func TestEventAttrs(t *testing.T) {
	attrs := EventAttrs("analytics-service", "delivery.created", "evt-123", "delivery-456", "delivery.events", 0, 100, "corr-123", "trace-456")

	assert.Len(t, attrs, 18) // 9 key-value pairs
	assert.Equal(t, "analytics-service", attrs[1])
	assert.Equal(t, "delivery.created", attrs[3])
	assert.Equal(t, "evt-123", attrs[5])
	assert.Equal(t, "delivery-456", attrs[7])
	assert.Equal(t, "delivery.events", attrs[9])
	assert.Equal(t, int32(0), attrs[11])
	assert.Equal(t, int64(100), attrs[13])
	assert.Equal(t, "corr-123", attrs[15])
	assert.Equal(t, "trace-456", attrs[17])

	// Test with empty values
	attrs2 := EventAttrs("", "", "", "", "", 0, 0, "", "")
	assert.Len(t, attrs2, 18)
	assert.Equal(t, "", attrs2[1])
}