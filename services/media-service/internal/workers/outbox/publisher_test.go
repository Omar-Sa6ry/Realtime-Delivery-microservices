package outbox

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

func pendingEvent(id string, attempts int) *domain.OutboxEvent {
	return eventOfType(id, "media.ready", attempts)
}

func eventOfType(id, eventType string, attempts int) *domain.OutboxEvent {
	return &domain.OutboxEvent{
		EventID:     id,
		AggregateID: "media-1",
		EventType:   eventType,
		Payload:     []byte(`{"mediaId":"media-1"}`),
		Status:      domain.OutboxStatusPending,
		Attempts:    attempts,
		TraceID:     "trace-1",
	}
}

func TestProcessBatchPublishesAndMarksPublished(t *testing.T) {
	repo := testutil.NewMockOutboxRepository()
	repo.Pending = []*domain.OutboxEvent{pendingEvent("evt-a", 0), pendingEvent("evt-b", 1)}
	pub := testutil.NewMockPublisher()

	p := NewPublisher(repo, pub, time.Millisecond)
	p.processBatch(context.Background())

	if len(pub.Messages) != 2 {
		t.Fatalf("published %d messages, want 2", len(pub.Messages))
	}
	if want := []string{"evt-a", "evt-b"}; !equalStrings(repo.Published, want) {
		t.Fatalf("Published = %v, want %v", repo.Published, want)
	}
	if len(repo.Failed) != 0 {
		t.Fatalf("Failed = %v, want empty", repo.Failed)
	}
	m := pub.Messages[0]
	if m.Topic != "media.ready" || m.Key != "media-1" || m.TraceID != "trace-1" {
		t.Fatalf("unexpected message envelope: %+v", m)
	}
	if !strings.Contains(string(m.Payload), `"mediaId":"media-1"`) {
		t.Fatalf("payload = %s, want mediaId", m.Payload)
	}
}

func TestProcessBatchPublishFailureKeepsEventPending(t *testing.T) {
	repo := testutil.NewMockOutboxRepository()
	repo.Pending = []*domain.OutboxEvent{pendingEvent("evt-retry", 4)}
	pub := testutil.NewMockPublisher()
	pub.PublishErr = errors.New("broker down")

	p := NewPublisher(repo, pub, time.Millisecond)
	p.processBatch(context.Background())

	if len(repo.Published) != 0 {
		t.Fatalf("Published = %v, want none while broker is down", repo.Published)
	}
	if len(repo.Failed) != 0 {
		t.Fatalf("Failed = %v, want none because attempts < 5", repo.Failed)
	}
}

func TestProcessBatchMarksFailedAfterMaxAttempts(t *testing.T) {
	repo := testutil.NewMockOutboxRepository()
	repo.Pending = []*domain.OutboxEvent{pendingEvent("evt-doomed", 5)}
	pub := testutil.NewMockPublisher()
	pub.PublishErr = errors.New("broker down")

	p := NewPublisher(repo, pub, time.Millisecond)
	p.processBatch(context.Background())

	if repo.Failed["evt-doomed"] == "" {
		t.Fatalf("Failed = %v, want an entry for evt-doomed", repo.Failed)
	}
	if !strings.Contains(repo.Failed["evt-doomed"], "broker down") {
		t.Fatalf("failure reason = %q, want to contain the publish error", repo.Failed["evt-doomed"])
	}
	if len(repo.Published) != 0 {
		t.Fatalf("Published = %v, want none", repo.Published)
	}
}

func TestProcessBatchMixedResults(t *testing.T) {
	repo := testutil.NewMockOutboxRepository()
	repo.Pending = []*domain.OutboxEvent{
		eventOfType("evt-ok", "media.scan.completed", 0),
		pendingEvent("evt-retry", 0),
		pendingEvent("evt-dead", 5),
	}
	pub := testutil.NewMockPublisher()
	pub.PublishErrFor = map[string]error{"media.ready": errors.New("nope")}

	p := NewPublisher(repo, pub, time.Millisecond)
	p.processBatch(context.Background())

	if len(repo.Published) != 1 || repo.Published[0] != "evt-ok" {
		t.Fatalf("Published = %v, want only the successful event", repo.Published)
	}
	if _, ok := repo.Failed["evt-retry"]; ok {
		t.Fatalf("evt-retry should stay pending, got Failed=%v", repo.Failed)
	}
	if _, ok := repo.Failed["evt-dead"]; !ok {
		t.Fatalf("evt-dead should be marked failed, got Failed=%v", repo.Failed)
	}
}

func TestProcessBatchListPendingError(t *testing.T) {
	repo := testutil.NewMockOutboxRepository()
	repo.ListPendingErr = errors.New("dynamo unavailable")
	pub := testutil.NewMockPublisher()

	p := NewPublisher(repo, pub, time.Millisecond)
	p.processBatch(context.Background())

	if len(pub.Messages) != 0 {
		t.Fatalf("published %d messages, want 0", len(pub.Messages))
	}
}

func TestProcessBatchEmptyBatch(t *testing.T) {
	repo := testutil.NewMockOutboxRepository()
	pub := testutil.NewMockPublisher()

	p := NewPublisher(repo, pub, time.Millisecond)
	p.processBatch(context.Background())

	if len(pub.Messages) != 0 {
		t.Fatalf("published %d messages, want 0", len(pub.Messages))
	}
	if len(repo.Published) != 0 {
		t.Fatalf("Published = %v, want none", repo.Published)
	}
}

func TestProcessBatchToleratesRepoErrors(t *testing.T) {
	t.Run("mark published error", func(t *testing.T) {
		repo := testutil.NewMockOutboxRepository()
		repo.Pending = []*domain.OutboxEvent{pendingEvent("evt-1", 0)}
		repo.MarkPublishedErr = errors.New("dynamo write failed")
		pub := testutil.NewMockPublisher()

		p := NewPublisher(repo, pub, time.Millisecond)
		p.processBatch(context.Background())

		if len(pub.Messages) != 1 {
			t.Fatalf("published %d messages, want 1", len(pub.Messages))
		}
		if len(repo.Published) != 0 {
			t.Fatalf("Published = %v, want none when MarkPublished fails", repo.Published)
		}
	})

	t.Run("mark failed error", func(t *testing.T) {
		repo := testutil.NewMockOutboxRepository()
		repo.Pending = []*domain.OutboxEvent{pendingEvent("evt-2", 5)}
		repo.MarkFailedErr = errors.New("dynamo write failed")
		pub := testutil.NewMockPublisher()
		pub.PublishErr = errors.New("broker down")

		p := NewPublisher(repo, pub, time.Millisecond)
		p.processBatch(context.Background())

		if len(repo.Failed) != 0 {
			t.Fatalf("Failed = %v, want none when MarkFailed errors", repo.Failed)
		}
	})
}

func TestRunExitsOnContextCancel(t *testing.T) {
	repo := testutil.NewMockOutboxRepository()
	pub := testutil.NewMockPublisher()
	p := NewPublisher(repo, pub, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestRunProcessesBatchesOnTicks(t *testing.T) {
	repo := testutil.NewMockOutboxRepository()
	repo.Pending = []*domain.OutboxEvent{pendingEvent("evt-tick", 0)}
	pub := testutil.NewMockPublisher()
	p := NewPublisher(repo, pub, 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(repo.Published) == 0 {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done

	if len(repo.Published) == 0 {
		t.Fatalf("Published = %v, want the pending event to be published on a tick", repo.Published)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
