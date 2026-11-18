package rabbitmq

import (
	"context"
	"sync"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	amqp "github.com/rabbitmq/amqp091-go"
)

type IndexEventHandler func(ctx context.Context, env *shared.EventEnvelope) error

type SearchRabbitMQConsumer struct {
	orders         *shared.Consumer
	media          *shared.Consumer
	users          *shared.Consumer
	drivers        *shared.Consumer
	ordersHandler  shared.MessageHandler
	mediaHandler   shared.MessageHandler
	usersHandler   shared.MessageHandler
	driversHandler shared.MessageHandler
}

func NewSearchRabbitMQConsumer(conn *shared.Connection, svc *indexing.Service) *SearchRabbitMQConsumer {
	return NewSearchRabbitMQConsumerWithHandler(conn, func(ctx context.Context, env *shared.EventEnvelope) error {
		return HandleIndexEvent(ctx, svc, env)
	})
}

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

func (c *SearchRabbitMQConsumer) Start(ctx context.Context) error {
	// shared.Consumer.Run returns nil on context cancellation (never errors), so
	// there is no error to propagate — just wait for shutdown and drain.
	var wg sync.WaitGroup
	run := func(cons *shared.Consumer, h shared.MessageHandler) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cons.Run(ctx, h)
		}()
	}
	run(c.orders, c.ordersHandler)
	run(c.media, c.mediaHandler)
	run(c.users, c.usersHandler)
	run(c.drivers, c.driversHandler)

	<-ctx.Done()
	wg.Wait()
	return nil
}
