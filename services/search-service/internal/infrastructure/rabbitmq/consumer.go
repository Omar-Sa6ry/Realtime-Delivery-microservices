package rabbitmq

import (
	"context"
	"log/slog"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	amqp "github.com/rabbitmq/amqp091-go"
)

// IndexEventHandler processes one search-index envelope.
// It receives only the envelope (no amqp.Delivery) so domain logic stays
// decoupled from transport details. Return shared.ErrPermanent to dead-letter
// immediately without retries.
type IndexEventHandler func(ctx context.Context, env *shared.EventEnvelope) error

// SearchRabbitMQConsumer consumes search.index.queue bound to both the orders
// topic exchange (order.#) and the media topic exchange (media.#).
//
// The shared Consumer supports a single exchange, so this type owns two
// underlying consumers sharing one queue name: RabbitMQ allows one queue to be
// bound to multiple exchanges, and both declarations are idempotent.
// It complements (does not replace) the Kafka ConsumerManager.
type SearchRabbitMQConsumer struct {
	orders        *shared.Consumer
	media         *shared.Consumer
	ordersHandler shared.MessageHandler
	mediaHandler  shared.MessageHandler
}

// NewSearchRabbitMQConsumer builds a dual-source consumer that fans every
// envelope into HandleIndexEvent (which reuses indexing.Service).
func NewSearchRabbitMQConsumer(conn *shared.Connection, svc *indexing.Service) *SearchRabbitMQConsumer {
	return NewSearchRabbitMQConsumerWithHandler(conn, func(ctx context.Context, env *shared.EventEnvelope) error {
		return HandleIndexEvent(ctx, svc, env)
	})
}

// NewSearchRabbitMQConsumerWithHandler builds a dual-source consumer with an
// explicit callback (useful for tests or custom pipelines).
func NewSearchRabbitMQConsumerWithHandler(conn *shared.Connection, handler IndexEventHandler) *SearchRabbitMQConsumer {
	if handler == nil {
		handler = func(_ context.Context, _ *shared.EventEnvelope) error { return nil }
	}
	adapt := func(ctx context.Context, env *shared.EventEnvelope, _ amqp.Delivery) error {
		return handler(ctx, env)
	}

	orders := shared.NewConsumer(shared.ConsumerConfig{
		Conn:        conn,
		Queue:       shared.QueueSearchIndex,
		Exchange:    shared.ExchangeOrders,
		RoutingKeys: []string{shared.OrdersRoutingKeyStatusAll, shared.OrdersRoutingKeyCreated},
		Prefetch:    shared.PrefetchFor(shared.QueueSearchIndex),
	})
	media := shared.NewConsumer(shared.ConsumerConfig{
		Conn:        conn,
		Queue:       shared.QueueSearchIndex,
		Exchange:    shared.ExchangeMedia,
		RoutingKeys: []string{shared.MediaRoutingKeyAll},
		Prefetch:    shared.PrefetchFor(shared.QueueSearchIndex),
	})
	return &SearchRabbitMQConsumer{
		orders:        orders,
		media:         media,
		ordersHandler: adapt,
		mediaHandler:  adapt,
	}
}

// Start runs both the orders and media consume loops until ctx is cancelled.
// It blocks; callers should run it in a goroutine. It returns when ctx is done.
func (c *SearchRabbitMQConsumer) Start(ctx context.Context) error {
	errCh := make(chan error, 2)
	go func() {
		if err := c.orders.Run(ctx, c.ordersHandler); err != nil {
			slog.Error("search RabbitMQ orders consumer stopped", "error", err)
			errCh <- err
			return
		}
		errCh <- nil
	}()
	go func() {
		if err := c.media.Run(ctx, c.mediaHandler); err != nil {
			slog.Error("search RabbitMQ media consumer stopped", "error", err)
			errCh <- err
			return
		}
		errCh <- nil
	}()

	// Wait for context cancellation, then drain both goroutines.
	<-ctx.Done()
	firstErr := <-errCh
	secondErr := <-errCh
	if firstErr != nil {
		return firstErr
	}
	return secondErr
}
