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

// testDeliveryHandler is a simple test handler for delivery events
type testDeliveryHandler struct {
	handlers.BaseHandler
}

func (h *testDeliveryHandler) Handles(eventType string) bool {
	return domain.KnownDeliveryEventTypes[eventType]
}

func (h *testDeliveryHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload handlers.Payload) (*handlers.FactBatch, error) {
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

// failingDeliveryHandler always returns an error
type failingDeliveryHandler struct {
	handlers.BaseHandler
}

func (h *failingDeliveryHandler) Handles(eventType string) bool {
	return domain.KnownDeliveryEventTypes[eventType]
}

func (h *failingDeliveryHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload handlers.Payload) (*handlers.FactBatch, error) {
	return nil, errors.New("handler error")
}

func TestNewIngestionPipeline(t *testing.T) {
	validator := NewValidator()
	dedup := NewDeduplicator(testutil.NewMockIdempotencyStore())
	router := NewRouter()
	batches := NewBatchWriter(testutil.NewMockClickHouseWriter(), testutil.NewMockIdempotencyStore(), 100, 100*time.Millisecond)

	pipeline := NewIngestionPipeline(validator, dedup, router, batches)
	require.NotNil(t, pipeline)
}

func TestPipeline_Process_Success(t *testing.T) {
	validator := NewValidator()
	mockStore := testutil.NewMockIdempotencyStore()
	dedup := NewDeduplicator(mockStore)
	router := NewRouter()
	router.Register("delivery", &testDeliveryHandler{})
	mockWriter := testutil.NewMockClickHouseWriter()
	batches := NewBatchWriter(mockWriter, mockStore, 100, 100*time.Millisecond)

	pipeline := NewIngestionPipeline(validator, dedup, router, batches)

	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	payload, _ := testutil.NewTestDeliveryCreatedPayload("delivery-456")
	data, _ := testutil.NewTestEventEnvelopeJSON(env, payload)

	ctx := context.Background()
	outcome, err := pipeline.Process(ctx, "delivery.events", 0, 100, data)

	assert.NoError(t, err)
	assert.Equal(t, OutcomeProcessed, outcome)

	// Verify data was written
	rawEvents := mockWriter.GetRawEvents()
	require.Len(t, rawEvents, 1)
	assert.Equal(t, "evt-123", rawEvents[0].EventID)
	assert.Equal(t, "delivery.created", rawEvents[0].EventType)
}

func TestPipeline_Process_Duplicate(t *testing.T) {
	validator := NewValidator()
	mockStore := testutil.NewMockIdempotencyStore()
	// Pre-populate with existing event
	mockStore.Save(context.Background(), testutil.NewTestIdempotencyRecord("evt-123", "delivery", "delivery-456"))
	dedup := NewDeduplicator(mockStore)
	router := NewRouter()
	router.Register("delivery", &testDeliveryHandler{})
	mockWriter := testutil.NewMockClickHouseWriter()
	batches := NewBatchWriter(mockWriter, mockStore, 100, 100*time.Millisecond)

	pipeline := NewIngestionPipeline(validator, dedup, router, batches)

	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	payload, _ := testutil.NewTestDeliveryCreatedPayload("delivery-456")
	data, _ := testutil.NewTestEventEnvelopeJSON(env, payload)

	ctx := context.Background()
	outcome, err := pipeline.Process(ctx, "delivery.events", 0, 100, data)

	assert.NoError(t, err)
	assert.Equal(t, OutcomeDuplicate, outcome)
}

func TestPipeline_Process_InvalidJSON(t *testing.T) {
	validator := NewValidator()
	dedup := NewDeduplicator(testutil.NewMockIdempotencyStore())
	router := NewRouter()
	mockWriter := testutil.NewMockClickHouseWriter()
	batches := NewBatchWriter(mockWriter, testutil.NewMockIdempotencyStore(), 100, 100*time.Millisecond)

	pipeline := NewIngestionPipeline(validator, dedup, router, batches)

	ctx := context.Background()
	outcome, err := pipeline.Process(ctx, "delivery.events", 0, 100, []byte("invalid json"))

	assert.Error(t, err)
	assert.Equal(t, OutcomeProcessed, outcome)
	assert.True(t, IsPermanent(err))
	assert.Contains(t, err.Error(), "decode")
}

func TestPipeline_Process_InvalidEnvelope(t *testing.T) {
	validator := NewValidator()
	dedup := NewDeduplicator(testutil.NewMockIdempotencyStore())
	router := NewRouter()
	mockWriter := testutil.NewMockClickHouseWriter()
	batches := NewBatchWriter(mockWriter, testutil.NewMockIdempotencyStore(), 100, 100*time.Millisecond)

	pipeline := NewIngestionPipeline(validator, dedup, router, batches)

	// Missing eventId
	data := []byte(`{"eventType":"delivery.created","eventVersion":1,"occurredAt":"2024-01-01T00:00:00Z","producer":"test","aggregateType":"delivery","aggregateId":"delivery-123","payload":{}}`)

	ctx := context.Background()
	outcome, err := pipeline.Process(ctx, "delivery.events", 0, 100, data)

	assert.Error(t, err)
	assert.Equal(t, OutcomeProcessed, outcome)
	assert.True(t, IsPermanent(err))
	assert.Contains(t, err.Error(), "envelope")
}

func TestPipeline_Process_InvalidSchema(t *testing.T) {
	validator := NewValidator()
	mockStore := testutil.NewMockIdempotencyStore()
	dedup := NewDeduplicator(mockStore)
	router := NewRouter()
	router.Register("delivery", &testDeliveryHandler{})
	mockWriter := testutil.NewMockClickHouseWriter()
	batches := NewBatchWriter(mockWriter, mockStore, 100, 100*time.Millisecond)

	pipeline := NewIngestionPipeline(validator, dedup, router, batches)

	// Missing required deliveryId in payload
	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	payload, _ := testutil.NewTestPayload(map[string]any{"userId": "user-123"})
	data, _ := testutil.NewTestEventEnvelopeJSON(env, payload)

	ctx := context.Background()
	outcome, err := pipeline.Process(ctx, "delivery.events", 0, 100, data)

	assert.Error(t, err)
	assert.Equal(t, OutcomeProcessed, outcome)
	assert.True(t, IsPermanent(err))
	assert.Contains(t, err.Error(), "schema")
	assert.Contains(t, err.Error(), "deliveryId")
}

func TestPipeline_Process_UnknownEventType(t *testing.T) {
	validator := NewValidator()
	mockStore := testutil.NewMockIdempotencyStore()
	dedup := NewDeduplicator(mockStore)
	router := NewRouter() // No handlers registered
	mockWriter := testutil.NewMockClickHouseWriter()
	batches := NewBatchWriter(mockWriter, mockStore, 100, 100*time.Millisecond)

	pipeline := NewIngestionPipeline(validator, dedup, router, batches)

	env := testutil.NewTestEventEnvelope("unknown.event", "evt-123", "delivery-456")
	payload, _ := testutil.NewTestPayload(map[string]any{})
	data, _ := testutil.NewTestEventEnvelopeJSON(env, payload)

	ctx := context.Background()
	outcome, err := pipeline.Process(ctx, "delivery.events", 0, 100, data)

	assert.Error(t, err)
	assert.Equal(t, OutcomeProcessed, outcome)
	assert.True(t, IsPermanent(err))
	assert.Contains(t, err.Error(), "route")
	assert.Contains(t, err.Error(), "unknown.event")
}

func TestPipeline_Process_DedupStoreError(t *testing.T) {
	validator := NewValidator()
	mockStore := testutil.NewMockIdempotencyStore()
	mockStore.SetExistsError(errors.New("store error"))
	dedup := NewDeduplicator(mockStore)
	router := NewRouter()
	router.Register("delivery", &testDeliveryHandler{})
	mockWriter := testutil.NewMockClickHouseWriter()
	batches := NewBatchWriter(mockWriter, mockStore, 100, 100*time.Millisecond)

	pipeline := NewIngestionPipeline(validator, dedup, router, batches)

	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	payload, _ := testutil.NewTestDeliveryCreatedPayload("delivery-456")
	data, _ := testutil.NewTestEventEnvelopeJSON(env, payload)

	ctx := context.Background()
	outcome, err := pipeline.Process(ctx, "delivery.events", 0, 100, data)

	assert.Error(t, err)
	assert.Equal(t, OutcomeProcessed, outcome)
	assert.Contains(t, err.Error(), "deduplicate")
}

func TestPipeline_Process_HandlerError(t *testing.T) {
	validator := NewValidator()
	mockStore := testutil.NewMockIdempotencyStore()
	dedup := NewDeduplicator(mockStore)
	router := NewRouter()
	router.Register("delivery", &failingDeliveryHandler{})
	mockWriter := testutil.NewMockClickHouseWriter()
	batches := NewBatchWriter(mockWriter, mockStore, 100, 100*time.Millisecond)

	pipeline := NewIngestionPipeline(validator, dedup, router, batches)

	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	payload, _ := testutil.NewTestDeliveryCreatedPayload("delivery-456")
	data, _ := testutil.NewTestEventEnvelopeJSON(env, payload)

	ctx := context.Background()
	outcome, err := pipeline.Process(ctx, "delivery.events", 0, 100, data)

	assert.Error(t, err)
	assert.Equal(t, OutcomeProcessed, outcome)
	assert.True(t, IsPermanent(err))
	assert.Contains(t, err.Error(), "transform")
}

func TestPipeline_Process_BatchWriterError(t *testing.T) {
	validator := NewValidator()
	mockStore := testutil.NewMockIdempotencyStore()
	dedup := NewDeduplicator(mockStore)
	router := NewRouter()
	router.Register("delivery", &testDeliveryHandler{})
	mockWriter := testutil.NewMockClickHouseWriter()
	mockWriter.SetWriteError(errors.New("write error"))
	batches := NewBatchWriter(mockWriter, mockStore, 100, 100*time.Millisecond)

	pipeline := NewIngestionPipeline(validator, dedup, router, batches)

	env := testutil.NewTestEventEnvelope("delivery.created", "evt-123", "delivery-456")
	payload, _ := testutil.NewTestDeliveryCreatedPayload("delivery-456")
	data, _ := testutil.NewTestEventEnvelopeJSON(env, payload)

	ctx := context.Background()
	outcome, err := pipeline.Process(ctx, "delivery.events", 0, 100, data)

	assert.Error(t, err)
	assert.Equal(t, OutcomeProcessed, outcome)
	assert.Contains(t, err.Error(), "buffer")
}

func TestPermanentError(t *testing.T) {
	baseErr := errors.New("base error")
	perm := permanent(baseErr)
	assert.True(t, IsPermanent(perm))
	assert.Equal(t, "permanent: "+baseErr.Error(), perm.Error())

	var perr *PermanentError
	require.True(t, errors.As(perm, &perr))
	assert.Equal(t, baseErr, perr.Unwrap())

	// Already permanent
	perm2 := permanent(perm)
	assert.Same(t, perm, perm2)
}

func TestOutcome_Constants(t *testing.T) {
	assert.Equal(t, Outcome(0), OutcomeProcessed)
	assert.Equal(t, Outcome(1), OutcomeDuplicate)
}