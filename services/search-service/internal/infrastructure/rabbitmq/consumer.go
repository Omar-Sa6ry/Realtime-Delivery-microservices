package rabbitmq

import (
	"context"
	"log/slog"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	amqp "github.com/rabbitmq/amqp091-go"
)

type IndexEventHandler func(ctx context.Context, env *shared.EventEnvelope) error

type SearchRabbitMQConsumer struct {
	orders        *shared.Consumer
	media         *shared.Consumer
	users         *shared.Consumer
	drivers       *shared.Consumer
	ordersHandler shared.MessageHandler
	mediaHandler  shared.MessageHandler
	usersHandler  shared.MessageHandler
	driversHandler shared.MessageHandler
}

// envelope into HandleIndexEvent (which reuses indexing.Service).
func NewSearchRabbitMQConsumer(conn *shared.Connection, svc *indexing.Service) *SearchRabbitMQConsumer {
	return NewSearchRabbitMQConsumerWithHandler(conn, func(ctx context.Context, env *shared.EventEnvelope) error {
		return HandleIndexEvent(ctx, svc, env)
	})
}

// NewSearchRabbitMQConsumerWithHandler builds a multi-source consumer with an
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
	users := shared.NewConsumer(shared.ConsumerConfig{
		Conn:        conn,
		Queue:       shared.QueueSearchIndex,
		Exchange:    shared.ExchangeUsers,
		RoutingKeys: []string{shared.UsersRoutingKeyAll},
		Prefetch:    shared.PrefetchFor(shared.QueueSearchIndex),
	})
	drivers := shared.NewConsumer(shared.ConsumerConfig{
		Conn:        conn,
		Queue:       shared.QueueSearchIndex,
		Exchange:    shared.ExchangeDrivers,
		RoutingKeys: []string{shared.DriversRoutingKeyAll},
		Prefetch:    shared.PrefetchFor(shared.QueueSearchIndex),
	})
	return &SearchRabbitMQConsumer{
		orders:         orders,
		media:          media,
		users:          users,
		drivers:        drivers,
		ordersHandler:  adapt,
		mediaHandler:   adapt,
		usersHandler:   adapt,
		driversHandler: adapt,
	}
}

// Start runs the consume loops for all bound exchanges until ctx is cancelled.
// It blocks; callers should run it in a goroutine. It returns when ctx is done.
func (c *SearchRabbitMQConsumer) Start(ctx context.Context) error {
	errCh := make(chan error, 4)
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
	go func() {
		if err := c.users.Run(ctx, c.usersHandler); err != nil {
			slog.Error("search RabbitMQ users consumer stopped", "error", err)
			errCh <- err
			return
		}
		errCh <- nil
	}()
	go func() {
		if err := c.drivers.Run(ctx, c.driversHandler); err != nil {
			slog.Error("search RabbitMQ drivers consumer stopped", "error", err)
			errCh <- err
			return
		}
		errCh <- nil
	}()

	// Wait for context cancellation, then drain all 4 goroutines.
	<-ctx.Done()
	var firstErr error
	for i := 0; i < 4; i++ {
		err := <-errCh
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
