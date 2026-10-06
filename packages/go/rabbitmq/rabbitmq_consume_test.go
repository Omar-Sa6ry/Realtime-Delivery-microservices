package rabbitmq

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envelopeBody(t *testing.T) []byte {
	t.Helper()
	data, err := MarshalEnvelope("evt-1", "order.created", "trace-env", map[string]string{"id": "1"})
	require.NoError(t, err)
	return data
}

func delivery(body []byte, acker *fakeAcker) amqp.Delivery {
	return amqp.Delivery{
		DeliveryTag:  11,
		Body:         body,
		Acknowledger: acker,
	}
}

// NewConsumer

func TestNewConsumerDefaults(t *testing.T) {
	conn := testConnection()
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueNotificationsMail, Exchange: ExchangeOrders})

	assert.Equal(t, 10, c.cfg.Prefetch)
	assert.Equal(t, 3, c.maxRetries)
	assert.Equal(t, 2*time.Second, c.retryDelay)
	assert.Equal(t, "test-svc-"+QueueNotificationsMail, c.cfg.ConsumerTag)
	assert.Equal(t, "test-svc", c.service)
}

func TestNewConsumerExplicitSettings(t *testing.T) {
	conn := testConnection()
	c := NewConsumer(ConsumerConfig{
		Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders,
		Prefetch: 42, MaxRetries: 5, RetryDelay: time.Millisecond, ConsumerTag: "custom",
	})

	assert.Equal(t, 42, c.cfg.Prefetch)
	assert.Equal(t, 5, c.maxRetries)
	assert.Equal(t, time.Millisecond, c.retryDelay)
	assert.Equal(t, "custom", c.cfg.ConsumerTag)
}

func TestNewConsumerWithoutConnection(t *testing.T) {
	c := NewConsumer(ConsumerConfig{Queue: QueueOrdersCreated})
	assert.Empty(t, c.cfg.ConsumerTag)
	assert.Empty(t, c.service)
}

// handleDelivery

func TestHandleDeliveryAcksSuccessfulMessages(t *testing.T) {
	ch := newFakeChannel()
	acker := &fakeAcker{}
	c := newTestConsumer(t, QueueOrdersCreated, 3, time.Nanosecond)

	before := testutil.ToFloat64(consumedTotal.WithLabelValues("test-svc", QueueOrdersCreated))
	c.handleDelivery(context.Background(), ch, delivery(envelopeBody(t), acker),
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil })

	assert.Equal(t, 1, acker.acks)
	assert.Equal(t, 0, acker.nacks)
	assert.Equal(t, before+1, testutil.ToFloat64(consumedTotal.WithLabelValues("test-svc", QueueOrdersCreated)))
}

func TestHandleDeliveryDeadLettersPoisonMessages(t *testing.T) {
	ch := newFakeChannel()
	acker := &fakeAcker{}
	c := newTestConsumer(t, QueueOrdersCreated, 3, time.Nanosecond)

	failedBefore := testutil.ToFloat64(failedTotal.WithLabelValues("test-svc", QueueOrdersCreated, "invalid_json"))
	dlqBefore := testutil.ToFloat64(dlqTotal.WithLabelValues("test-svc", QueueOrdersCreated))

	called := false
	c.handleDelivery(context.Background(), ch, delivery([]byte("{not json"), acker),
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error {
			called = true
			return nil
		})

	assert.False(t, called)
	assert.Equal(t, 1, acker.nacks)
	assert.False(t, acker.requeue)
	assert.Equal(t, failedBefore+1, testutil.ToFloat64(failedTotal.WithLabelValues("test-svc", QueueOrdersCreated, "invalid_json")))
	assert.Equal(t, dlqBefore+1, testutil.ToFloat64(dlqTotal.WithLabelValues("test-svc", QueueOrdersCreated)))
}

func TestHandleDeliveryDeadLettersPermanentFailures(t *testing.T) {
	ch := newFakeChannel()
	acker := &fakeAcker{}
	c := newTestConsumer(t, QueueOrdersCreated, 3, time.Nanosecond)

	c.handleDelivery(context.Background(), ch, delivery(envelopeBody(t), acker),
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return ErrPermanent })

	assert.Equal(t, 1, acker.nacks)
	assert.False(t, acker.requeue)
	assert.Equal(t, float64(1),
		testutil.ToFloat64(failedTotal.WithLabelValues("test-svc", QueueOrdersCreated, "permanent")))
}

func TestHandleDeliveryRequeuesForRetry(t *testing.T) {
	ch := newFakeChannel()
	acker := &fakeAcker{}
	c := newTestConsumer(t, QueueOrdersCreated, 3, time.Nanosecond)
	body := envelopeBody(t)

	c.handleDelivery(context.Background(), ch, delivery(body, acker),
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error {
			return errors.New("transient")
		})

	assert.Equal(t, 1, acker.acks)
	assert.Equal(t, 0, acker.nacks)

	require.Len(t, ch.published, 1)
	republished := ch.published[0]
	assert.Equal(t, "", republished.exchange)
	assert.Equal(t, QueueOrdersCreated, republished.key)
	assert.Equal(t, "trace-env", republished.pub.Headers[HeaderTraceID])
	assert.Equal(t, int32(1), republished.pub.Headers[HeaderRetryCount])
	assert.Equal(t, body, republished.pub.Body)
}

func TestHandleDeliveryPrefersHeaderTraceID(t *testing.T) {
	ch := newFakeChannel()
	acker := &fakeAcker{}
	c := newTestConsumer(t, QueueOrdersCreated, 3, time.Nanosecond)

	msg := delivery(envelopeBody(t), acker)
	msg.Headers = amqp.Table{HeaderTraceID: "trace-header"}

	c.handleDelivery(context.Background(), ch, msg,
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error {
			return errors.New("transient")
		})

	require.Len(t, ch.published, 1)
	assert.Equal(t, "trace-header", ch.published[0].pub.Headers[HeaderTraceID])
}

func TestHandleDeliveryLeavesOffTraceHeaderWhenAbsent(t *testing.T) {
	ch := newFakeChannel()
	acker := &fakeAcker{}
	c := newTestConsumer(t, QueueOrdersCreated, 3, time.Nanosecond)

	body, err := MarshalEnvelope("evt-1", "order.created", "", map[string]string{"id": "1"})
	require.NoError(t, err)

	c.handleDelivery(context.Background(), ch, delivery(body, acker),
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error {
			return errors.New("transient")
		})

	require.Len(t, ch.published, 1)
	assert.NotContains(t, ch.published[0].pub.Headers, HeaderTraceID)
}

func TestHandleDeliveryRequeuesWhenContextCancelledDuringRetry(t *testing.T) {
	ch := newFakeChannel()
	acker := &fakeAcker{}
	c := newTestConsumer(t, QueueOrdersCreated, 3, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c.handleDelivery(ctx, ch, delivery(envelopeBody(t), acker),
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error {
			return errors.New("transient")
		})

	assert.Equal(t, 1, acker.nacks)
	assert.True(t, acker.requeue)
	assert.Empty(t, ch.published)
}

func TestHandleDeliveryRequeuesWhenRepublishFails(t *testing.T) {
	ch := newFakeChannel()
	ch.publishErr = errors.New("channel closed")
	acker := &fakeAcker{}
	c := newTestConsumer(t, QueueOrdersCreated, 3, time.Nanosecond)

	c.handleDelivery(context.Background(), ch, delivery(envelopeBody(t), acker),
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error {
			return errors.New("transient")
		})

	assert.Equal(t, 1, acker.nacks)
	assert.True(t, acker.requeue)
}

func TestHandleDeliveryDeadLettersAfterRetriesExhausted(t *testing.T) {
	ch := newFakeChannel()
	acker := &fakeAcker{}
	c := newTestConsumer(t, QueueOrdersCreated, 2, time.Nanosecond)

	msg := delivery(envelopeBody(t), acker)
	msg.Headers = amqp.Table{HeaderRetryCount: int32(2)}

	c.handleDelivery(context.Background(), ch, msg,
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error {
			return errors.New("transient")
		})

	assert.Equal(t, 1, acker.nacks)
	assert.False(t, acker.requeue)
	assert.Equal(t, float64(1),
		testutil.ToFloat64(failedTotal.WithLabelValues("test-svc", QueueOrdersCreated, "max_retries_exceeded")))
}

// --- consumeLoop -------------------------------------------------------------

func TestConsumeLoopChannelFailure(t *testing.T) {
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc", MaxAttempts: 1, BaseDelay: time.Nanosecond},
		newFakeBrokerConn(chanResult{err: errors.New("no channel")}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders})

	err := c.consumeLoop(context.Background(), func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to RabbitMQ")
}

func TestConsumeLoopExchangeDeclareFailure(t *testing.T) {
	ch := newFakeChannel()
	ch.exchangeErr = errors.New("exchange refused")
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"}, newFakeBrokerConn(chanResult{ch: ch}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders})

	err := c.consumeLoop(context.Background(), func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "declare exchange")
}

func TestConsumeLoopQueueDeclareFailure(t *testing.T) {
	ch := newFakeChannel()
	ch.queueErr = errors.New("queue refused")
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"}, newFakeBrokerConn(chanResult{ch: ch}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders})

	err := c.consumeLoop(context.Background(), func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "declare queue")
}

func TestConsumeLoopQueueBindFailure(t *testing.T) {
	ch := newFakeChannel()
	ch.bindErr = errors.New("bind refused")
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"}, newFakeBrokerConn(chanResult{ch: ch}))
	c := NewConsumer(ConsumerConfig{
		Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders,
		RoutingKeys: []string{OrdersRoutingKeyCreated},
	})

	err := c.consumeLoop(context.Background(), func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind queue")
}

func TestConsumeLoopQosFailure(t *testing.T) {
	ch := newFakeChannel()
	ch.qosErr = errors.New("qos refused")
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"}, newFakeBrokerConn(chanResult{ch: ch}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders, Prefetch: 7})

	err := c.consumeLoop(context.Background(), func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set qos")
	assert.Equal(t, []int{7}, ch.qosCall)
}

func TestConsumeLoopConsumeFailure(t *testing.T) {
	ch := newFakeChannel()
	ch.consumeErr = errors.New("consume refused")
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"}, newFakeBrokerConn(chanResult{ch: ch}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders})

	err := c.consumeLoop(context.Background(), func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "basic consume")
}

func TestConsumeLoopHandlesDeliveriesThenStops(t *testing.T) {
	ch := newFakeChannel()
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"}, newFakeBrokerConn(chanResult{ch: ch}))
	c := NewConsumer(ConsumerConfig{
		Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders,
		RoutingKeys: []string{OrdersRoutingKeyCreated},
	})

	ctx, cancel := context.WithCancel(context.Background())
	var handled []string
	acker := &fakeAcker{}
	ch.deliveries <- amqp.Delivery{DeliveryTag: 3, Body: envelopeBody(t), Acknowledger: acker}

	require.NoError(t, c.consumeLoop(ctx, func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error {
		handled = append(handled, env.EventType)
		cancel()
		return nil
	}))

	assert.Equal(t, []string{"order.created"}, handled)
	require.Len(t, ch.queues, 1)
	require.Len(t, ch.binds, 1)
	assert.Equal(t, 1, acker.acks)
}

func TestConsumeLoopDetectsClosedDeliveryChannel(t *testing.T) {
	ch := newFakeChannel()
	close(ch.deliveries)
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"}, newFakeBrokerConn(chanResult{ch: ch}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders})

	err := c.consumeLoop(context.Background(), func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delivery channel closed by broker")
}

func TestConsumeLoopUsesFallbackExchangeKind(t *testing.T) {
	ch := newFakeChannel()
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"}, newFakeBrokerConn(chanResult{ch: ch}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: "brand.new.exchange"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, c.consumeLoop(ctx, func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil }))
	require.Len(t, ch.exchanges, 1)
	assert.Equal(t, "topic", ch.exchanges[0].kind)
}

// --- Run ---------------------------------------------------------------------

func TestRunStopsImmediatelyOnCancelledContext(t *testing.T) {
	c := newTestConsumer(t, QueueOrdersCreated, 1, time.Nanosecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, c.Run(ctx, func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error {
		t.Fatal("handler must not run")
		return nil
	}))
}

func TestRunConsumesUntilContextCancelled(t *testing.T) {
	ch := newFakeChannel()
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"}, newFakeBrokerConn(chanResult{ch: ch}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders})

	ctx, cancel := context.WithCancel(context.Background())
	var handled int
	acker := &fakeAcker{}

	ch.deliveries <- amqp.Delivery{
		DeliveryTag:  1,
		Body:         envelopeBody(t),
		Acknowledger: acker,
	}

	require.NoError(t, c.Run(ctx, func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error {
		handled++
		cancel()
		return nil
	}))

	assert.Equal(t, 1, handled)
	assert.Equal(t, 1, acker.acks)
}

func TestRunReturnsCleanlyWhenChannelCannotBeOpened(t *testing.T) {
	withDialAMQP(t, func(url string) (brokerConn, error) {
		t.Error("no dial should be attempted when the context is already cancelled")
		return nil, errors.New("unreachable")
	})

	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc", MaxAttempts: 1, BaseDelay: time.Nanosecond},
		newFakeBrokerConn(chanResult{err: errors.New("no channel")}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, c.Run(ctx, func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil }))
}

func TestRunStopsWithoutBackoffWhenCancelledDuringFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	withDialAMQP(t, func(url string) (brokerConn, error) {
		cancel()
		return nil, errors.New("broker gone")
	})

	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc", MaxAttempts: 1, BaseDelay: time.Nanosecond},
		newFakeBrokerConn(chanResult{err: errors.New("no channel")}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders})

	require.NoError(t, c.Run(ctx, func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil }))
}

func TestRunTreatsWrappedContextCancellationAsCleanStop(t *testing.T) {
	ch := newFakeChannel()
	ch.consumeErr = context.Canceled
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"}, newFakeBrokerConn(chanResult{ch: ch}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders})

	require.NoError(t, c.Run(context.Background(),
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil }))
}

func TestRunBacksOffAndRecoversAfterChannelFailure(t *testing.T) {
	shrinkConsumeBackoff(t, time.Millisecond)

	failing := newFakeChannel()
	failing.exchangeErr = errors.New("transient topology failure")
	healthy := newFakeChannel()
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: "test-svc"},
		newFakeBrokerConn(chanResult{ch: failing}, chanResult{ch: healthy}))
	c := NewConsumer(ConsumerConfig{Conn: conn, Queue: QueueOrdersCreated, Exchange: ExchangeOrders})

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel once the loop has had a chance to fail, log, measure and sleep.
	time.AfterFunc(5*time.Millisecond, cancel)

	require.NoError(t, c.Run(ctx,
		func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error { return nil }))
}

func shrinkConsumeBackoff(t *testing.T, d time.Duration) {
	t.Helper()
	orig := consumeBackoff
	consumeBackoff = d
	t.Cleanup(func() { consumeBackoff = orig })
}
