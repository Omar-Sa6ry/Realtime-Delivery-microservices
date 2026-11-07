package rabbitmq_test

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	adapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/rabbitmq"
	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	rabbitAddr = "127.0.0.1:5673"
	rabbitURL  = "amqp://guest:guest@127.0.0.1:5673/"
)

const composeHint = "run `docker compose -f docker-compose.e2e.yml -p driver-e2e up -d --wait` from the service directory"

// probeRabbit skips the test when the live broker is unreachable so a missing
// Docker daemon never fails the build.
func probeRabbit(t *testing.T) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", rabbitAddr, time.Second)
	if err != nil {
		t.Skipf("RabbitMQ at %s is not reachable (%v); %s", rabbitAddr, err, composeHint)
	}
	_ = conn.Close()
}

// connectRabbit opens the shared connection the adapter wraps.
func connectRabbit(t *testing.T) *shared.Connection {
	t.Helper()
	probeRabbit(t)
	conn, err := shared.ConnectURL(rabbitURL, "driver-service-tests")
	if err != nil {
		t.Fatalf("ConnectURL: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// tapExchange declares an exclusive queue bound to exchange/routingKey so a
// test can observe what the adapter publishes without touching shared queues.
func tapExchange(t *testing.T, conn *shared.Connection, exchange, routingKey string) (<-chan amqp.Delivery, func()) {
	t.Helper()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	queue, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		t.Fatalf("declare tap queue: %v", err)
	}
	if err := ch.QueueBind(queue.Name, routingKey, exchange, false, nil); err != nil {
		t.Fatalf("bind tap queue: %v", err)
	}
	deliveries, err := ch.Consume(queue.Name, "", true, false, false, false, nil)
	if err != nil {
		t.Fatalf("consume tap queue: %v", err)
	}
	return deliveries, func() { _ = ch.Close() }
}

// awaitDelivery waits for the next message on a tap queue.
func awaitDelivery(t *testing.T, deliveries <-chan amqp.Delivery) amqp.Delivery {
	t.Helper()
	select {
	case msg, ok := <-deliveries:
		if !ok {
			t.Fatal("tap queue closed before a message arrived")
		}
		return msg
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a published message")
	}
	return amqp.Delivery{}
}

// envelope is the shared event wrapper the publisher marshals into.
type envelope struct {
	EventType string          `json:"eventType"`
	EventID   string          `json:"eventId"`
	Payload   json.RawMessage `json:"payload"`
}

func TestEnsureDriverTopology(t *testing.T) {
	conn := connectRabbit(t)
	if err := adapter.EnsureDriverTopology(conn); err != nil {
		t.Fatalf("EnsureDriverTopology: %v", err)
	}
	// Declaring twice must be idempotent.
	if err := adapter.EnsureDriverTopology(conn); err != nil {
		t.Fatalf("EnsureDriverTopology (second call): %v", err)
	}
}

func TestPublisher_DriverEventRoundTrip(t *testing.T) {
	conn := connectRabbit(t)
	if err := adapter.EnsureDriverTopology(conn); err != nil {
		t.Fatalf("EnsureDriverTopology: %v", err)
	}

	deliveries, closeTap := tapExchange(t, conn, shared.ExchangeDrivers, "#")
	defer closeTap()

	pub := adapter.NewRabbitMQPublisher(conn)
	defer func() { _ = pub.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pub.PublishDriverEvent(ctx, shared.DriversRoutingKeyCreated, "driver.created",
		map[string]string{"driverId": "drv-1"}, nil); err != nil {
		t.Fatalf("PublishDriverEvent: %v", err)
	}

	msg := awaitDelivery(t, deliveries)
	if msg.RoutingKey != shared.DriversRoutingKeyCreated {
		t.Errorf("routing key = %q, want %q", msg.RoutingKey, shared.DriversRoutingKeyCreated)
	}
	var env envelope
	if err := json.Unmarshal(msg.Body, &env); err != nil {
		t.Fatalf("envelope unmarshal: %v", err)
	}
	if env.EventType != "driver.created" {
		t.Errorf("eventType = %q, want driver.created", env.EventType)
	}
	// Message identity travels in the AMQP MessageId; the envelope's eventId is
	// left empty by the shared publisher today.
	if msg.MessageId == "" {
		t.Error("AMQP MessageId is empty")
	}
	var payload map[string]string
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if payload["driverId"] != "drv-1" {
		t.Errorf("payload.driverId = %q, want drv-1", payload["driverId"])
	}
}

func TestPublisher_DispatchResponseRoundTrip(t *testing.T) {
	conn := connectRabbit(t)
	if err := adapter.EnsureDriverTopology(conn); err != nil {
		t.Fatalf("EnsureDriverTopology: %v", err)
	}

	deliveries, closeTap := tapExchange(t, conn, shared.ExchangeDispatch, shared.DispatchRoutingKeyResponse)
	defer closeTap()

	pub := adapter.NewRabbitMQPublisher(conn)
	defer func() { _ = pub.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pub.PublishDispatchResponse(ctx, map[string]any{"status": "accepted"}, nil); err != nil {
		t.Fatalf("PublishDispatchResponse: %v", err)
	}

	msg := awaitDelivery(t, deliveries)
	if msg.RoutingKey != shared.DispatchRoutingKeyResponse {
		t.Errorf("routing key = %q, want %q", msg.RoutingKey, shared.DispatchRoutingKeyResponse)
	}
	if !json.Valid(msg.Body) {
		t.Errorf("body is not valid JSON: %s", msg.Body)
	}
}

// TestPublisher_DefaultRoutingKey covers the empty routing key / event type
// fallbacks of PublishDriverEvent.
func TestPublisher_DefaultRoutingKey(t *testing.T) {
	conn := connectRabbit(t)
	if err := adapter.EnsureDriverTopology(conn); err != nil {
		t.Fatalf("EnsureDriverTopology: %v", err)
	}

	deliveries, closeTap := tapExchange(t, conn, shared.ExchangeDrivers, "#")
	defer closeTap()

	pub := adapter.NewRabbitMQPublisher(conn)
	defer func() { _ = pub.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pub.PublishDriverEvent(ctx, "", "", map[string]string{"driverId": "drv-2"}, nil); err != nil {
		t.Fatalf("PublishDriverEvent with empty routing key: %v", err)
	}

	msg := awaitDelivery(t, deliveries)
	if msg.RoutingKey != shared.DriversRoutingKeyAll {
		t.Errorf("routing key = %q, want the default %q", msg.RoutingKey, shared.DriversRoutingKeyAll)
	}
	var env envelope
	if err := json.Unmarshal(msg.Body, &env); err != nil {
		t.Fatalf("envelope unmarshal: %v", err)
	}
	if env.EventType != shared.DriversRoutingKeyAll {
		t.Errorf("eventType = %q, want the routing key default %q", env.EventType, shared.DriversRoutingKeyAll)
	}
}

// TestPublisher_NilReceiver covers the defensive nil checks on the adapter.
func TestPublisher_NilReceiver(t *testing.T) {
	var pub *adapter.RabbitMQPublisher
	ctx := context.Background()

	if err := pub.PublishDispatchResponse(ctx, nil, nil); err == nil {
		t.Error("PublishDispatchResponse on a nil publisher: got nil error, want error")
	}
	if err := pub.PublishDriverEvent(ctx, "driver.created", "driver.created", nil, nil); err == nil {
		t.Error("PublishDriverEvent on a nil publisher: got nil error, want error")
	}
	if err := pub.Close(); err != nil {
		t.Errorf("Close on a nil publisher: %v, want nil", err)
	}
}

// TestDispatchConsumer_RoundTrip drives the consumer with a real
// dispatch.request published on the live broker.
func TestDispatchConsumer_RoundTrip(t *testing.T) {
	conn := connectRabbit(t)
	if err := adapter.EnsureDriverTopology(conn); err != nil {
		t.Fatalf("EnsureDriverTopology: %v", err)
	}

	handled := make(chan string, 1)
	consumer := adapter.NewDispatchConsumer(conn, func(_ context.Context, env *shared.EventEnvelope) error {
		handled <- env.EventType
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- consumer.Start(ctx) }()

	// Give the consumer a moment to register its subscription.
	time.Sleep(500 * time.Millisecond)

	rawConn, err := amqp.Dial(rabbitURL)
	if err != nil {
		t.Fatalf("amqp dial: %v", err)
	}
	defer rawConn.Close()
	raw, err := rawConn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	defer raw.Close()
	if err := raw.Publish(shared.ExchangeDispatch, shared.DispatchRoutingKeyRequest, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        []byte(`{"eventType":"dispatch.request","eventId":"evt-1"}`),
	}); err != nil {
		t.Fatalf("publish dispatch request: %v", err)
	}

	select {
	case got := <-handled:
		if got != "dispatch.request" {
			t.Errorf("handled eventType = %q, want dispatch.request", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("consumer never handled the dispatch request")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned %v after cancellation, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("consumer did not stop after its context was cancelled")
	}
}

// TestDispatchConsumer_NilHandler covers the default no-op handler branch:
// a dispatch request that arrives while the handler is nil must be taken and
// acknowledged instead of crashing the consumer.
func TestDispatchConsumer_NilHandler(t *testing.T) {
	conn := connectRabbit(t)
	if err := adapter.EnsureDriverTopology(conn); err != nil {
		t.Fatalf("EnsureDriverTopology: %v", err)
	}

	rawConn, err := amqp.Dial(rabbitURL)
	if err != nil {
		t.Fatalf("amqp dial: %v", err)
	}
	defer rawConn.Close()
	raw, err := rawConn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	defer raw.Close()

	// Start from a known state: requests left behind by an earlier run would
	// make the depth assertions below meaningless.
	if _, err := raw.QueuePurge(shared.QueueDispatchRequests, false); err != nil {
		t.Fatalf("purge dispatch queue: %v", err)
	}

	// Publish with confirms so the broker has definitely queued the requests
	// before this test looks at the queue: an unconfirmed Publish returns as
	// soon as the frame is written, and a passive declare could observe the
	// queue while the broker is still processing it.
	if err := raw.Confirm(false); err != nil {
		t.Fatalf("confirm mode: %v", err)
	}
	confirms := raw.NotifyPublish(make(chan amqp.Confirmation, 1))

	const messages = 3
	for i := 0; i < messages; i++ {
		if err := raw.Publish(shared.ExchangeDispatch, shared.DispatchRoutingKeyRequest, true, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(`{"eventType":"dispatch.request","eventId":"evt-nil"}`),
		}); err != nil {
			t.Fatalf("publish dispatch request: %v", err)
		}
		select {
		case c := <-confirms:
			if !c.Ack {
				t.Fatalf("broker rejected dispatch request %d", i+1)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("no confirm for dispatch request %d", i+1)
		}
	}

	// With no consumer attached yet the requests must be visible in the queue.
	waitForDepth(t, raw, messages, 10*time.Second)

	consumer := adapter.NewDispatchConsumer(conn, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.StartWithHandler(ctx, nil) }()

	// The requests must leave the queue, which only happens when a consumer
	// took them (the nil handler answers with a silent ack).
	waitForDepth(t, raw, 0, 10*time.Second)

	// Let the handler finish before the consumer is torn down.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("consumer with a nil handler did not stop after cancellation")
	}
}

// waitForDepth polls the dispatch queue until it reports exactly want queued
// messages, failing the test when the deadline passes first.
func waitForDepth(t *testing.T, ch *amqp.Channel, want int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		q, err := ch.QueueDeclarePassive(shared.QueueDispatchRequests, true, false, false, false, nil)
		if err != nil {
			t.Fatalf("passive queue declare: %v", err)
		}
		if q.Messages == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("dispatch queue depth = %d after %s, want %d", q.Messages, within, want)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
