package dlq

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
	"github.com/google/uuid"
)

func newTestManager(t *testing.T, pub ports.EventPublisher) *DLQManager {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewDLQManager(pub, logger)
}

func TestDLQManagerImplementsPort(t *testing.T) {
	var manager ports.DLQManager = newTestManager(t, testutil.NewMockPublisher())
	if manager == nil {
		t.Fatal("manager is nil")
	}
}

func TestNewDLQManager(t *testing.T) {
	m := newTestManager(t, testutil.NewMockPublisher())
	if m == nil {
		t.Fatal("NewDLQManager returned nil")
	}
	if m.logger == nil {
		t.Error("logger not initialised")
	}
}

func TestListDLQTopics(t *testing.T) {
	m := newTestManager(t, testutil.NewMockPublisher())
	topics := m.ListDLQTopics()
	if len(topics) != 11 {
		t.Fatalf("expected 11 topics, got %d", len(topics))
	}
	want := []string{
		kafkaadapter.TopicUploadCompleted,
		kafkaadapter.TopicScanCompleted,
		kafkaadapter.TopicScanStarted,
		kafkaadapter.TopicScanFailed,
		kafkaadapter.TopicProcessingStarted,
		kafkaadapter.TopicProcessingCompleted,
		kafkaadapter.TopicProcessingFailed,
		kafkaadapter.TopicMediaReady,
		kafkaadapter.TopicDeleteRequested,
		kafkaadapter.TopicMediaDeleted,
		kafkaadapter.TopicDeleteFailed,
	}
	for i, topic := range want {
		if topics[i] != topic+".dlq" {
			t.Errorf("topic[%d] = %q, want %q", i, topics[i], topic+".dlq")
		}
	}
}

func TestReplayDLQMessage(t *testing.T) {
	t.Run("strips .dlq suffix and keeps trace header", func(t *testing.T) {
		pub := testutil.NewMockPublisher()
		m := newTestManager(t, pub)

		msg := DLQMessage{
			ID:      "id-1",
			Topic:   kafkaadapter.TopicUploadCompleted + ".dlq",
			Key:     "media-1",
			Value:   []byte(`{"mediaId":"m1"}`),
			Headers: map[string]string{"x-trace-id": "trace-123"},
		}
		if err := m.ReplayDLQMessage(context.Background(), kafkaadapter.TopicUploadCompleted+".dlq", msg); err != nil {
			t.Fatalf("ReplayDLQMessage: %v", err)
		}

		msgs := pub.ForTopic(kafkaadapter.TopicUploadCompleted)
		if len(msgs) != 1 {
			t.Fatalf("expected 1 published message, got %d", len(msgs))
		}
		if msgs[0].Key != "media-1" {
			t.Errorf("key = %q, want media-1", msgs[0].Key)
		}
		if string(msgs[0].Payload) != `{"mediaId":"m1"}` {
			t.Errorf("payload = %q", msgs[0].Payload)
		}
		if msgs[0].TraceID != "trace-123" {
			t.Errorf("traceID = %q, want trace-123", msgs[0].TraceID)
		}
	})

	t.Run("topic without .dlq suffix is used as-is", func(t *testing.T) {
		pub := testutil.NewMockPublisher()
		m := newTestManager(t, pub)

		msg := DLQMessage{Key: "k", Value: []byte(`{}`), Headers: map[string]string{"x-trace-id": "t"}}
		if err := m.ReplayDLQMessage(context.Background(), kafkaadapter.TopicMediaReady, msg); err != nil {
			t.Fatalf("ReplayDLQMessage: %v", err)
		}
		if pub.Count(kafkaadapter.TopicMediaReady) != 1 {
			t.Fatalf("expected publish to %s", kafkaadapter.TopicMediaReady)
		}
	})

	t.Run("generates uuid trace when header missing", func(t *testing.T) {
		pub := testutil.NewMockPublisher()
		m := newTestManager(t, pub)

		msg := DLQMessage{Key: "k", Value: []byte(`{}`)}
		if err := m.ReplayDLQMessage(context.Background(), kafkaadapter.TopicMediaDeleted+".dlq", msg); err != nil {
			t.Fatalf("ReplayDLQMessage: %v", err)
		}
		last, ok := pub.Last()
		if !ok {
			t.Fatal("no message published")
		}
		if _, err := uuid.Parse(last.TraceID); err != nil {
			t.Errorf("traceID %q is not a uuid: %v", last.TraceID, err)
		}
	})

	t.Run("publish error is wrapped", func(t *testing.T) {
		pub := testutil.NewMockPublisher()
		pub.PublishErr = errors.New("kafka unavailable")
		m := newTestManager(t, pub)

		msg := DLQMessage{Key: "k", Value: []byte(`{}`), Headers: map[string]string{"x-trace-id": "t"}}
		err := m.ReplayDLQMessage(context.Background(), kafkaadapter.TopicMediaReady+".dlq", msg)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "replay to "+kafkaadapter.TopicMediaReady) {
			t.Errorf("error %q does not mention original topic", err)
		}
		if !strings.Contains(err.Error(), "kafka unavailable") {
			t.Errorf("error %q does not wrap cause", err)
		}
	})

	t.Run("short topic without suffix", func(t *testing.T) {
		pub := testutil.NewMockPublisher()
		m := newTestManager(t, pub)

		msg := DLQMessage{Key: "k", Value: []byte(`{}`), Headers: map[string]string{"x-trace-id": "t"}}
		if err := m.ReplayDLQMessage(context.Background(), "abc", msg); err != nil {
			t.Fatalf("ReplayDLQMessage: %v", err)
		}
		if pub.Count("abc") != 1 {
			t.Fatalf("expected publish to abc")
		}
	})
}

func TestReplayAllDLQMessagesUnreachableBroker(t *testing.T) {
	pub := testutil.NewMockPublisher()
	m := newTestManager(t, pub)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	replayed, err := m.ReplayAllDLQMessages(ctx, []string{"127.0.0.1:1"}, kafkaadapter.TopicMediaReady, 5)
	if err != nil {
		t.Fatalf("ReplayAllDLQMessages: %v", err)
	}
	if replayed != 0 {
		t.Fatalf("replayed = %d, want 0", replayed)
	}
	if len(pub.Messages) != 0 {
		t.Fatalf("expected no publishes, got %d", len(pub.Messages))
	}
}

func TestReplayAllDLQMessagesAppendsSuffix(t *testing.T) {
	pub := testutil.NewMockPublisher()
	m := newTestManager(t, pub)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Topic already carrying the .dlq suffix must not get a second one; both
	// variants reach the same unreachable broker and replay nothing.
	replayed, err := m.ReplayAllDLQMessages(ctx, []string{"127.0.0.1:1"}, kafkaadapter.TopicMediaReady+".dlq", 1)
	if err != nil {
		t.Fatalf("ReplayAllDLQMessages: %v", err)
	}
	if replayed != 0 {
		t.Fatalf("replayed = %d, want 0", replayed)
	}
}

func TestGetDLQStatsUnreachableBroker(t *testing.T) {
	pub := testutil.NewMockPublisher()
	m := newTestManager(t, pub)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stats, err := m.GetDLQStats(ctx, []string{"127.0.0.1:1"}, []string{kafkaadapter.TopicMediaReady})
	if err != nil {
		t.Fatalf("GetDLQStats: %v", err)
	}
	if count, ok := stats[kafkaadapter.TopicMediaReady+".dlq"]; !ok || count != 0 {
		t.Fatalf("stats = %v, want %q -> 0", stats, kafkaadapter.TopicMediaReady+".dlq")
	}
}

func TestGetDLQStatsExistingSuffix(t *testing.T) {
	pub := testutil.NewMockPublisher()
	m := newTestManager(t, pub)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stats, err := m.GetDLQStats(ctx, []string{"127.0.0.1:1"}, []string{"media.ready.dlq"})
	if err != nil {
		t.Fatalf("GetDLQStats: %v", err)
	}
	if count, ok := stats["media.ready.dlq"]; !ok || count != 0 {
		t.Fatalf("stats = %v, want media.ready.dlq -> 0", stats)
	}
}
