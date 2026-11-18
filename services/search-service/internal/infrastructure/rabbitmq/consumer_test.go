package rabbitmq

import (
	"context"
	"testing"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
	amqp "github.com/rabbitmq/amqp091-go"
)

func TestNewSearchRabbitMQConsumerWithHandler_NilHandler(t *testing.T) {
	c := NewSearchRabbitMQConsumerWithHandler(nil, nil)
	if c == nil {
		t.Fatal("nil consumer")
	}
	if c.orders == nil || c.media == nil || c.users == nil || c.drivers == nil {
		t.Fatal("consumers not initialized")
	}
	// nil handler is replaced by a no-op — invoking the adapted handler must succeed.
	err := c.ordersHandler(context.Background(), &shared.EventEnvelope{EventType: "x"}, amqp.Delivery{})
	if err != nil {
		t.Fatalf("nil-handler no-op: %v", err)
	}
	if c.mediaHandler == nil || c.usersHandler == nil || c.driversHandler == nil {
		t.Fatal("handlers not set")
	}
}

func TestNewSearchRabbitMQConsumer_UsesHandleIndexEvent(t *testing.T) {
	repo := testutil.NewFakeRepository()
	svc := indexing.NewService(repo)
	c := NewSearchRabbitMQConsumer(nil, svc)
	if c == nil {
		t.Fatal("nil consumer")
	}

	// Unknown type → log-only path via handler closure.
	if err := c.ordersHandler(context.Background(), &shared.EventEnvelope{
		EventType: "unknown.type",
		Payload:   []byte(`{}`),
	}, amqp.Delivery{}); err != nil {
		t.Fatalf("unknown type: %v", err)
	}

	// Nil envelope → permanent error through the closure.
	if err := c.ordersHandler(context.Background(), nil, amqp.Delivery{}); err == nil {
		t.Fatal("expected error for nil envelope")
	}

	// DeliveryCreated indexes through the closure.
	e := envelope(t, "delivery.created", map[string]interface{}{
		"deliveryId": "d1", "customerId": "c1", "status": "PENDING",
		"pickup":  map[string]interface{}{"city": "C", "country": "EG", "location": map[string]interface{}{"lat": 1.0, "lon": 2.0}},
		"dropoff": map[string]interface{}{"city": "D", "country": "EG", "location": map[string]interface{}{"lat": 3.0, "lon": 4.0}},
	})
	if err := c.driversHandler(context.Background(), e, amqp.Delivery{}); err != nil {
		t.Fatalf("delivery.created via consumer: %v", err)
	}
	if len(repo.UpsertDeliveries) != 1 || repo.UpsertDeliveries[0].DeliveryID != "d1" {
		t.Errorf("upserts = %+v", repo.UpsertDeliveries)
	}
}

func TestSearchRabbitMQConsumer_Start_AlreadyCanceledContext(t *testing.T) {
	c := NewSearchRabbitMQConsumerWithHandler(nil, func(context.Context, *shared.EventEnvelope) error {
		return nil
	})

	// Pre-canceled context: each shared.Consumer.Run returns immediately at the
	// ctx.Done() check without touching the (nil) connection.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start with canceled ctx: %v", err)
	}
}
