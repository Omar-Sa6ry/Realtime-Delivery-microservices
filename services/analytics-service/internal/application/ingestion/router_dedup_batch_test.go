package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion/handlers"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routerTestDeliveryHandler is a simple test handler for delivery events
type routerTestDeliveryHandler struct {
	handlers.BaseHandler
}

func (h *routerTestDeliveryHandler) Handles(eventType string) bool {
	return domain.KnownDeliveryEventTypes[eventType]
}

func (h *routerTestDeliveryHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload handlers.Payload) (*handlers.FactBatch, error) {
	batch := &handlers.FactBatch{}
	raw := h.NewRawLanding(env)
	raw.PayloadJSON = string(payload.Raw)
	batch.Raw = append(batch.Raw, raw)
	batch.Seen = append(batch.Seen, h.NewSeen(env))

	if env.EventType == domain.DeliveryDeleted {
		return batch, nil
	}

	event := &domain.FactDeliveryEvent{
		EventID:       env.EventID,
		DeliveryID:    handlers.GetString(payload.Map, "deliveryId"),
		UserID:        handlers.GetString(payload.Map, "userId"),
		DriverID:      handlers.GetString(payload.Map, "driverId"),
		EventType:     env.EventType,
		EventVersion:  env.EventVersion,
		CityID:        handlers.GetString(payload.Map, "cityId"),
		ZoneID:        handlers.GetString(payload.Map, "zoneId"),
		OccurredAt:    env.OccurredAt,
		IngestedAt:    env.IngestedAt,
		CorrelationID: env.CorrelationID,
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	batch.DeliveryEvents = append(batch.DeliveryEvents, event)

	return batch, nil
}

// routerTestDriverHandler is a simple test handler for driver events
type routerTestDriverHandler struct {
	handlers.BaseHandler
}

func (h *routerTestDriverHandler) Handles(eventType string) bool {
	if _, ok := driverAssignmentEvents[eventType]; ok {
		return true
	}
	return eventType == "driver.available" || eventType == "driver.unavailable"
}

func (h *routerTestDriverHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload handlers.Payload) (*handlers.FactBatch, error) {
	batch := &handlers.FactBatch{}
	raw := h.NewRawLanding(env)
	raw.PayloadJSON = string(payload.Raw)
	batch.Raw = append(batch.Raw, raw)
	batch.Seen = append(batch.Seen, h.NewSeen(env))
	return batch, nil
}

// routerTestPaymentHandler is a simple test handler for payment events
type routerTestPaymentHandler struct {
	handlers.BaseHandler
}

func (h *routerTestPaymentHandler) Handles(eventType string) bool {
	_, ok := paymentTypeMapping[eventType]
	return ok
}

func (h *routerTestPaymentHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload handlers.Payload) (*handlers.FactBatch, error) {
	batch := &handlers.FactBatch{}
	raw := h.NewRawLanding(env)
	raw.PayloadJSON = string(payload.Raw)
	batch.Raw = append(batch.Raw, raw)
	batch.Seen = append(batch.Seen, h.NewSeen(env))
	return batch, nil
}

// routerTestNotificationHandler is a simple test handler for notification events
type routerTestNotificationHandler struct {
	handlers.BaseHandler
}

func (h *routerTestNotificationHandler) Handles(eventType string) bool {
	_, ok := notificationStatusMapping[eventType]
	return ok
}

func (h *routerTestNotificationHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload handlers.Payload) (*handlers.FactBatch, error) {
	batch := &handlers.FactBatch{}
	raw := h.NewRawLanding(env)
	raw.PayloadJSON = string(payload.Raw)
	batch.Raw = append(batch.Raw, raw)
	batch.Seen = append(batch.Seen, h.NewSeen(env))
	return batch, nil
}

var driverAssignmentEvents = map[string]domain.AssignmentResult{
	"driver.assignment.offered":  domain.AssignmentOffered,
	"driver.assignment.accepted": domain.AssignmentAccepted,
	"driver.assignment.rejected": domain.AssignmentRejected,
	"driver.assignment.expired":  domain.AssignmentExpired,
	"driver.assignment.released": domain.AssignmentReleased,
}

var paymentTypeMapping = map[string]domain.PaymentTransactionType{
	"payment.created":               domain.TransactionAuthorization,
	"payment.authorization.started": domain.TransactionAuthorization,
	"payment.authorized":            domain.TransactionAuthorization,
	"payment.authorization.failed":  domain.TransactionAuthorization,
	"payment.capture.started":       domain.TransactionCapture,
	"payment.captured":              domain.TransactionCapture,
	"payment.completed":             domain.TransactionCapture,
	"payment.capture.failed":        domain.TransactionCapture,
	"payment.cancelled":             domain.TransactionCancel,
	"payment.refund.started":        domain.TransactionRefund,
	"payment.refunded":              domain.TransactionRefund,
	"payment.refund.failed":         domain.TransactionRefund,
	"payment.failed":                domain.TransactionAuthorization,
}

var notificationStatusMapping = map[string]domain.NotificationStatus{
	"notification.created":   domain.NotificationStatusCreated,
	"notification.sent":      domain.NotificationStatusSent,
	"notification.delivered": domain.NotificationStatusDelivered,
	"notification.failed":    domain.NotificationStatusFailed,
	"notification.retrying":  domain.NotificationStatusRetrying,
}

func TestNewRouter(t *testing.T) {
	router := NewRouter()
	assert.NotNil(t, router)
	assert.NotNil(t, router.routes)
}

func TestRouter_Register(t *testing.T) {
	router := NewRouter()
	handler := &routerTestDeliveryHandler{}

	router.Register("delivery", handler)

	assert.Len(t, router.routes, 1)
	assert.Equal(t, handler, router.routes["delivery"])
}

func TestRouter_Route_ExactMatch(t *testing.T) {
	router := NewRouter()
	router.Register("delivery", &routerTestDeliveryHandler{})

	handler, err := router.Route("delivery.created")

	require.NoError(t, err)
	require.NotNil(t, handler)
	assert.True(t, handler.Handles("delivery.created"))
}

func TestRouter_Route_PrefixMatch(t *testing.T) {
	router := NewRouter()
	router.Register("delivery", &routerTestDeliveryHandler{})
	router.Register("driver", &routerTestDriverHandler{})
	router.Register("payment", &routerTestPaymentHandler{})
	router.Register("notification", &routerTestNotificationHandler{})

	tests := []struct {
		name        string
		eventType   string
		expectError bool
	}{
		{"delivery.created", "delivery.created", false},
		{"delivery.driver.assigned", "delivery.driver.assigned", false},
		{"driver.available", "driver.available", false},
		{"driver.assignment.offered", "driver.assignment.offered", false},
		{"payment.created", "payment.created", false},
		{"notification.sent", "notification.sent", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := router.Route(tc.eventType)
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, handler)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, handler)
			}
		})
	}
}

func TestRouter_Route_UnknownEventType(t *testing.T) {
	router := NewRouter()

	handler, err := router.Route("unknown.event")

	assert.Error(t, err)
	assert.Nil(t, handler)
	assert.Contains(t, err.Error(), "unknown event type")
}

func TestRouter_Route_HandlerDoesNotHandle(t *testing.T) {
	router := NewRouter()
	router.Register("delivery", &routerTestDeliveryHandler{})

	// This handler doesn't handle "driver.available"
	handler, err := router.Route("driver.available")

	assert.Error(t, err)
	assert.Nil(t, handler)
	assert.Contains(t, err.Error(), "unknown event type")
}

func TestNewDeduplicator(t *testing.T) {
	store := testutil.NewMockIdempotencyStore()
	dedup := NewDeduplicator(store)
	assert.NotNil(t, dedup)
	assert.Equal(t, store, dedup.store)
}

func TestDeduplicator_IsDuplicate(t *testing.T) {
	store := testutil.NewMockIdempotencyStore()
	dedup := NewDeduplicator(store)

	ctx := context.Background()

	// First check - not duplicate
	dup, err := dedup.IsDuplicate(ctx, "evt-123")
	assert.NoError(t, err)
	assert.False(t, dup)

	// Save the event
	store.Save(ctx, testutil.NewTestIdempotencyRecord("evt-123", "delivery", "delivery-456"))

	// Second check - duplicate
	dup, err = dedup.IsDuplicate(ctx, "evt-123")
	assert.NoError(t, err)
	assert.True(t, dup)
}

func TestDeduplicator_IsDuplicate_StoreError(t *testing.T) {
	store := testutil.NewMockIdempotencyStore()
	store.SetExistsError(errors.New("store error"))
	dedup := NewDeduplicator(store)

	ctx := context.Background()
	dup, err := dedup.IsDuplicate(ctx, "evt-123")

	assert.Error(t, err)
	assert.False(t, dup)
	assert.Contains(t, err.Error(), "store error")
}

func TestNewBatchWriter(t *testing.T) {
	writer := testutil.NewMockClickHouseWriter()
	store := testutil.NewMockIdempotencyStore()

	batchWriter := NewBatchWriter(writer, store, 100, 100*time.Millisecond)
	assert.NotNil(t, batchWriter)
	assert.Equal(t, 100, batchWriter.maxRows)
	assert.Equal(t, 100*time.Millisecond, batchWriter.flushInterval)

	// Test defaults
	batchWriter2 := NewBatchWriter(writer, store, -1, -1)
	assert.Equal(t, 500, batchWriter2.maxRows)
	assert.Equal(t, 500*time.Millisecond, batchWriter2.flushInterval)
}

func TestBatchWriter_Add_FlushOnThreshold(t *testing.T) {
	writer := testutil.NewMockClickHouseWriter()
	store := testutil.NewMockIdempotencyStore()
	batchWriter := NewBatchWriter(writer, store, 2, 10*time.Second) // flush after 2 rows

	ctx := context.Background()

	// Add first batch (1 row)
	batch1 := &handlers.FactBatch{
		Raw: []*domain.RawEventLanding{testutil.NewTestRawEventLanding("delivery.created", "evt-1", "delivery-1")},
	}
	ids, err := batchWriter.Add(ctx, batch1)
	assert.NoError(t, err)
	assert.Nil(t, ids) // Not flushed yet

	// Add second batch (should trigger flush)
	batch2 := &handlers.FactBatch{
		Raw: []*domain.RawEventLanding{testutil.NewTestRawEventLanding("delivery.created", "evt-2", "delivery-2")},
	}
	ids, err = batchWriter.Add(ctx, batch2)
	assert.NoError(t, err)
	assert.Len(t, ids, 2) // Both event IDs returned

	// Verify data written
	rawEvents := writer.GetRawEvents()
	assert.Len(t, rawEvents, 2)
}

func TestBatchWriter_Flush(t *testing.T) {
	writer := testutil.NewMockClickHouseWriter()
	store := testutil.NewMockIdempotencyStore()
	batchWriter := NewBatchWriter(writer, store, 100, 10*time.Second)

	ctx := context.Background()

	batch := &handlers.FactBatch{
		Raw: []*domain.RawEventLanding{testutil.NewTestRawEventLanding("delivery.created", "evt-1", "delivery-1")},
	}
	batchWriter.Add(ctx, batch)

	ids, err := batchWriter.Flush(ctx)
	assert.NoError(t, err)
	assert.Len(t, ids, 1)

	// Verify data written
	rawEvents := writer.GetRawEvents()
	assert.Len(t, rawEvents, 1)
}

func TestBatchWriter_FlushEmpty(t *testing.T) {
	writer := testutil.NewMockClickHouseWriter()
	store := testutil.NewMockIdempotencyStore()
	batchWriter := NewBatchWriter(writer, store, 100, 10*time.Second)

	ctx := context.Background()
	ids, err := batchWriter.Flush(ctx)

	assert.NoError(t, err)
	assert.Nil(t, ids)
}

func TestBatchWriter_Flush_WriteError(t *testing.T) {
	writer := testutil.NewMockClickHouseWriter()
	writer.SetWriteError(errors.New("write error"))
	store := testutil.NewMockIdempotencyStore()
	batchWriter := NewBatchWriter(writer, store, 100, 10*time.Second)

	ctx := context.Background()

	batch := &handlers.FactBatch{
		Raw: []*domain.RawEventLanding{testutil.NewTestRawEventLanding("delivery.created", "evt-1", "delivery-1")},
	}
	batchWriter.Add(ctx, batch)

	ids, err := batchWriter.Flush(ctx)
	assert.Error(t, err)
	assert.Nil(t, ids)
	assert.Contains(t, err.Error(), "write error")

	// Verify batch was requeued
	rawEvents := writer.GetRawEvents()
	assert.Len(t, rawEvents, 0) // Nothing written due to error
}

func TestBatchWriter_StartStop(t *testing.T) {
	writer := testutil.NewMockClickHouseWriter()
	store := testutil.NewMockIdempotencyStore()
	batchWriter := NewBatchWriter(writer, store, 100, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batchWriter.Start(ctx)
	
	// Add some data
	batch := &handlers.FactBatch{
		Raw: []*domain.RawEventLanding{testutil.NewTestRawEventLanding("delivery.created", "evt-1", "delivery-1")},
	}
	batchWriter.Add(ctx, batch)

	// Stop and flush
	err := batchWriter.Stop(ctx)
	assert.NoError(t, err)

	// Verify data was flushed
	rawEvents := writer.GetRawEvents()
	assert.Len(t, rawEvents, 1)
}

func TestBatchWriter_Stop_WithFlushError(t *testing.T) {
	writer := testutil.NewMockClickHouseWriter()
	writer.SetWriteError(errors.New("write error"))
	store := testutil.NewMockIdempotencyStore()
	batchWriter := NewBatchWriter(writer, store, 100, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batchWriter.Start(ctx)
	
	batch := &handlers.FactBatch{
		Raw: []*domain.RawEventLanding{testutil.NewTestRawEventLanding("delivery.created", "evt-1", "delivery-1")},
	}
	batchWriter.Add(ctx, batch)

	err := batchWriter.Stop(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write error")
}