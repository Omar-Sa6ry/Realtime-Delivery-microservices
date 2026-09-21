package ingestion

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion/handlers"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type BatchWriter struct {
	mu            sync.Mutex
	writer        ports.ClickHouseWriter
	store         ports.IdempotencyStore
	maxRows       int
	flushInterval time.Duration

	pending     []*handlers.FactBatch
	pendingRows int

	ticker *time.Ticker
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewBatchWriter(writer ports.ClickHouseWriter, store ports.IdempotencyStore, maxRows int, flushInterval time.Duration) *BatchWriter {
	if maxRows <= 0 {
		maxRows = 500
	}
	if flushInterval <= 0 {
		flushInterval = 500 * time.Millisecond
	}
	return &BatchWriter{
		writer:        writer,
		store:         store,
		maxRows:       maxRows,
		flushInterval: flushInterval,
		stopCh:        make(chan struct{}),
	}
}

// Start begins periodic flushing until Stop is called.
func (b *BatchWriter) Start(ctx context.Context) {
	b.ticker = time.NewTicker(b.flushInterval)
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.stopCh:
				return
			case <-b.ticker.C:
				if _, err := b.Flush(context.Background()); err != nil {
					// Logged by the caller on manual flush; periodic flush
					// failures surface as consumer lag, not lost data.
					_ = err
				}
			}
		}
	}()
}

// Stop halts periodic flushing and flushes what remains.
func (b *BatchWriter) Stop(ctx context.Context) error {
	close(b.stopCh)
	if b.ticker != nil {
		b.ticker.Stop()
	}
	b.wg.Wait()
	_, err := b.Flush(ctx)
	return err
}

// Add buffers a batch, flushing first when the row budget is exceeded.
// It returns the flushed event IDs (durable) for observability.
func (b *BatchWriter) Add(ctx context.Context, batch *handlers.FactBatch) ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = append(b.pending, batch)
	b.pendingRows += batch.RowCount()
	if b.pendingRows < b.maxRows {
		return nil, nil
	}
	return b.flushLocked(ctx)
}

// Flush writes all buffered rows. Empty buffers are a no-op.
func (b *BatchWriter) Flush(ctx context.Context) ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.flushLocked(ctx)
}

func (b *BatchWriter) flushLocked(ctx context.Context) ([]string, error) {
	if len(b.pending) == 0 {
		return nil, nil
	}
	pending := b.pending
	b.pending = nil
	b.pendingRows = 0

	if err := b.writeTables(ctx, pending); err != nil {
		// Transient: requeue so nothing is silently dropped.
		b.pending = append(pending, b.pending...)
		b.pendingRows += countRows(pending)
		return nil, err
	}
	ids := make([]string, 0, len(pending))
	for _, batch := range pending {
		for _, seen := range batch.Seen {
			if err := b.store.Save(ctx, seen); err != nil {
				return ids, fmt.Errorf("mark seen %s: %w", seen.EventID, err)
			}
			ids = append(ids, seen.EventID)
		}
	}
	return ids, nil
}

func (b *BatchWriter) writeTables(ctx context.Context, batches []*handlers.FactBatch) error {
	var raw []*domain.RawEventLanding
	var deliveryEvents []*domain.FactDeliveryEvent
	var deliveryCompleted []*domain.FactDeliveryCompleted
	var driverAssignments []*domain.FactDriverAssignment
	var paymentTransactions []*domain.FactPaymentTransaction
	var notificationEvents []*domain.FactNotificationEvent
	var dataQuality []*domain.DataQualityIssue
	for _, batch := range batches {
		raw = append(raw, batch.Raw...)
		deliveryEvents = append(deliveryEvents, batch.DeliveryEvents...)
		deliveryCompleted = append(deliveryCompleted, batch.DeliveryCompleted...)
		driverAssignments = append(driverAssignments, batch.DriverAssignments...)
		paymentTransactions = append(paymentTransactions, batch.PaymentTransactions...)
		notificationEvents = append(notificationEvents, batch.NotificationEvents...)
		dataQuality = append(dataQuality, batch.DataQuality...)
	}
	writes := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"raw_events", func(ctx context.Context) error { return b.writer.WriteRawEvents(ctx, raw) }},
		{"fact_delivery_events", func(ctx context.Context) error { return b.writer.WriteDeliveryEvents(ctx, deliveryEvents) }},
		{"fact_delivery_completed", func(ctx context.Context) error { return b.writer.WriteDeliveryCompleted(ctx, deliveryCompleted) }},
		{"fact_driver_assignments", func(ctx context.Context) error { return b.writer.WriteDriverAssignments(ctx, driverAssignments) }},
		{"fact_payment_transactions", func(ctx context.Context) error { return b.writer.WritePaymentTransactions(ctx, paymentTransactions) }},
		{"fact_notification_events", func(ctx context.Context) error { return b.writer.WriteNotificationEvents(ctx, notificationEvents) }},
		{"analytics_data_quality_issues", func(ctx context.Context) error { return b.writer.WriteDataQualityIssues(ctx, dataQuality) }},
	}
	for _, w := range writes {
		if err := w.fn(ctx); err != nil {
			return fmt.Errorf("flush %s: %w", w.name, err)
		}
	}
	return nil
}

func countRows(batches []*handlers.FactBatch) int {
	n := 0
	for _, b := range batches {
		n += b.RowCount()
	}
	return n
}
