package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/analytics"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion/handlers"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type e2eDeliveryHandler struct {
	handlers.BaseHandler
}

func (h *e2eDeliveryHandler) Handles(eventType string) bool {
	return domain.KnownDeliveryEventTypes[eventType]
}

func (h *e2eDeliveryHandler) Handle(ctx context.Context, env *domain.EventEnvelope, payload handlers.Payload) (*handlers.FactBatch, error) {
	batch := &handlers.FactBatch{}
	raw := h.NewRawLanding(env)
	raw.PayloadJSON = string(payload.Raw)
	batch.Raw = append(batch.Raw, raw)
	batch.Seen = append(batch.Seen, h.NewSeen(env))

	event := &domain.FactDeliveryEvent{
		EventID:       env.EventID,
		DeliveryID:    handlers.GetString(payload.Map, "deliveryId"),
		UserID:        handlers.GetString(payload.Map, "userId"),
		DriverID:      handlers.GetString(payload.Map, "driverId"),
		EventType:     env.EventType,
		EventVersion:  env.EventVersion,
		CityID:        handlers.GetString(payload.Map, "cityId"),
		OccurredAt:    env.OccurredAt,
		IngestedAt:    env.IngestedAt,
		CorrelationID: env.CorrelationID,
	}
	batch.DeliveryEvents = append(batch.DeliveryEvents, event)
	return batch, nil
}

func TestE2E_AnalyticsIngestionAndQueryFlow(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize Ingestion Pipeline Components
	validator := ingestion.NewValidator()
	mockStore := testutil.NewMockIdempotencyStore()
	dedup := ingestion.NewDeduplicator(mockStore)

	router := ingestion.NewRouter()
	router.Register("delivery", &e2eDeliveryHandler{})

	mockWriter := testutil.NewMockClickHouseWriter()
	batches := ingestion.NewBatchWriter(mockWriter, mockStore, 1, 100*time.Millisecond)

	pipeline := ingestion.NewIngestionPipeline(validator, dedup, router, batches)
	require.NotNil(t, pipeline)

	// 2. Prepare incoming event envelope & payload
	env := testutil.NewTestEventEnvelope("delivery.created", "evt-e2e-001", "delivery-e2e-123")
	payload, err := testutil.NewTestDeliveryCreatedPayload("delivery-e2e-123")
	require.NoError(t, err)

	data, err := testutil.NewTestEventEnvelopeJSON(env, payload)
	require.NoError(t, err)

	// 3. Process event through pipeline
	outcome, err := pipeline.Process(ctx, "delivery.events", 0, 100, data)
	require.NoError(t, err)
	assert.Equal(t, ingestion.OutcomeProcessed, outcome)

	// 4. Flush batch to persistence
	_, err = batches.Flush(ctx)
	require.NoError(t, err)

	// Verify ClickHouse Writer captured the raw and fact records
	rawEvents := mockWriter.GetRawEvents()
	require.Len(t, rawEvents, 1)
	assert.Equal(t, "evt-e2e-001", rawEvents[0].EventID)
	assert.Equal(t, "delivery.created", rawEvents[0].EventType)

	deliveryFacts := mockWriter.GetDeliveryEvents()
	require.Len(t, deliveryFacts, 1)
	assert.Equal(t, "delivery-e2e-123", deliveryFacts[0].DeliveryID)

	// 5. Query via Delivery Analytics Service with Cache
	mockCache := testutil.NewMockCacheRepository()
	mockRepo := testutil.NewMockAnalyticsQueryRepository()
	expectedAnalytics := &ports.DeliveryAnalytics{
		Total:          1,
		Completed:      1,
		CompletionRate: 1.0,
		DataAsOf:       time.Now().UTC(),
	}
	mockRepo.SetDeliveryAnalytics(expectedAnalytics)

	querySvc := analytics.NewDeliveryAnalyticsService(mockRepo, mockCache, 5*time.Minute)
	require.NotNil(t, querySvc)

	filter := ports.DeliveryAnalyticsFilter{
		Range: ports.TimeRange{
			From:        time.Now().UTC().Add(-24 * time.Hour),
			To:          time.Now().UTC(),
			Granularity: ports.GranularityDay,
		},
		CityID: "cairo",
	}

	result, err := querySvc.GetDeliveryAnalytics(ctx, filter, "e2e-scope")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, int64(1), result.Completed)
	assert.Equal(t, 1.0, result.CompletionRate)
}
