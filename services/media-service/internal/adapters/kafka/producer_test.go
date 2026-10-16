package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/observability"
	dto "github.com/prometheus/client_model/go"
	kafkago "github.com/segmentio/kafka-go"
	metadataAPI "github.com/segmentio/kafka-go/protocol/metadata"
	produceAPI "github.com/segmentio/kafka-go/protocol/produce"
)

// deadBroker refuses every connection, which bounds all network calls in tests.
const deadBroker = "127.0.0.1:1"

func boundedCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	return ctx
}

func counterDelta(t *testing.T, counter interface {
	Write(*dto.Metric) error
}, call func()) float64 {
	t.Helper()
	read := func() float64 {
		t.Helper()
		m := &dto.Metric{}
		if err := counter.Write(m); err != nil {
			t.Fatalf("write metric: %v", err)
		}
		return m.GetCounter().GetValue()
	}
	before := read()
	call()
	return read() - before
}

func TestMarshalEnvelopeRoundTrip(t *testing.T) {
	payload := map[string]string{"mediaId": "m1"}
	before := time.Now().Add(-time.Second).UnixMilli()

	raw, err := MarshalEnvelope("evt-9", TopicMediaReady, "trace-1", payload)
	if err != nil {
		t.Fatalf("MarshalEnvelope: %v", err)
	}
	after := time.Now().Add(time.Second).UnixMilli()

	var env EventEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.EventID != "evt-9" {
		t.Fatalf("EventID = %q, want evt-9", env.EventID)
	}
	if env.EventType != TopicMediaReady {
		t.Fatalf("EventType = %q, want %q", env.EventType, TopicMediaReady)
	}
	if env.TraceID != "trace-1" {
		t.Fatalf("TraceID = %q, want trace-1", env.TraceID)
	}
	if env.Timestamp < before || env.Timestamp > after {
		t.Fatalf("Timestamp = %d outside [%d, %d]", env.Timestamp, before, after)
	}

	var decoded map[string]string
	if err := json.Unmarshal(env.Payload, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decoded["mediaId"] != "m1" {
		t.Fatalf("payload mediaId = %q, want m1", decoded["mediaId"])
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal raw envelope: %v", err)
	}
	for _, name := range []string{"eventId", "eventType", "traceId", "timestamp", "payload"} {
		if _, ok := fields[name]; !ok {
			t.Fatalf("envelope is missing field %q", name)
		}
	}
}

func TestMarshalEnvelopePayloadMarshalError(t *testing.T) {
	raw, err := MarshalEnvelope("evt-1", TopicMediaReady, "trace-1", make(chan struct{}))
	if err == nil {
		t.Fatalf("expected payload marshal error, got raw=%s", raw)
	}
	if !strings.Contains(err.Error(), "marshal event payload") {
		t.Fatalf("error = %v, want it to mention marshal event payload", err)
	}
}

func TestProducerPublishReturnsErrorAgainstDeadBroker(t *testing.T) {
	observability.RegisterMediaMetrics()
	p := NewProducer([]string{deadBroker})
	t.Cleanup(func() { _ = p.Close() })

	failures := observability.KafkaPublishFailures.WithLabelValues(TopicMediaReady)
	delta := counterDelta(t, failures, func() {
		err := p.Publish(boundedCtx(t), TopicMediaReady, "key-1", []byte(`{}`), "trace-1")
		if err == nil {
			t.Fatal("Publish against a dead broker must fail")
		}
		if !strings.Contains(err.Error(), `kafka publish to topic "media.ready"`) {
			t.Fatalf("error = %v, want topic context", err)
		}
	})
	if delta != 1 {
		t.Fatalf("KafkaPublishFailures delta = %v, want 1", delta)
	}
}

func TestProducerCloseIsIdempotent(t *testing.T) {
	p := NewProducer([]string{deadBroker})
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

type fakeKafkaTransport struct{}

func (fakeKafkaTransport) RoundTrip(_ context.Context, _ net.Addr, req kafkago.Request) (kafkago.Response, error) {
	switch r := req.(type) {
	case *metadataAPI.Request:
		name := ""
		if len(r.TopicNames) > 0 {
			name = r.TopicNames[0]
		}
		return &metadataAPI.Response{
			Topics: []metadataAPI.ResponseTopic{{
				Name:       name,
				Partitions: []metadataAPI.ResponsePartition{{PartitionIndex: 0}},
			}},
		}, nil
	case *produceAPI.Request:
		topic := ""
		if len(r.Topics) > 0 {
			topic = r.Topics[0].Topic
		}
		return &produceAPI.Response{
			Topics: []produceAPI.ResponseTopic{{
				Topic:      topic,
				Partitions: []produceAPI.ResponsePartition{{Partition: 0, BaseOffset: 1}},
			}},
		}, nil
	default:
		return nil, fmt.Errorf("fake kafka transport: unexpected request type %T", req)
	}
}

func newFakeTransportProducer() *Producer {
	return &Producer{writer: &kafkago.Writer{
		Addr:         kafkago.TCP(deadBroker),
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireAll,
		Transport:    fakeKafkaTransport{},
	}}
}

func TestProducerPublishSucceedsViaFakeTransport(t *testing.T) {
	observability.RegisterMediaMetrics()
	p := newFakeTransportProducer()
	t.Cleanup(func() { _ = p.Close() })

	failures := observability.KafkaPublishFailures.WithLabelValues(TopicMediaReady)
	var err error
	delta := counterDelta(t, failures, func() {
		err = p.Publish(boundedCtx(t), TopicMediaReady, "key-1", []byte(`{"mediaId":"m1"}`), "trace-1")
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if delta != 0 {
		t.Fatalf("KafkaPublishFailures delta = %v, want 0 on success", delta)
	}
}

func TestProducerLoggersForwardToSlog(t *testing.T) {
	observability.RegisterMediaMetrics()
	p := NewProducer([]string{deadBroker})
	t.Cleanup(func() { _ = p.Close() })

	prev := slog.Default()
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	p.writer.Logger.Printf("writing %d messages", 3)
	p.writer.ErrorLogger.Printf("error writing messages")

	out := buf.String()
	if !strings.Contains(out, "writing 3 messages") {
		t.Fatalf("slog output = %q, want producer debug log", out)
	}
	if !strings.Contains(out, "kafka-producer") {
		t.Fatalf("slog output = %q, want component attribute", out)
	}
	if !strings.Contains(out, "error writing messages") {
		t.Fatalf("slog output = %q, want producer error log", out)
	}
	if !strings.Contains(out, "level=ERROR") {
		t.Fatalf("slog output = %q, want ERROR level for error logger", out)
	}
}
