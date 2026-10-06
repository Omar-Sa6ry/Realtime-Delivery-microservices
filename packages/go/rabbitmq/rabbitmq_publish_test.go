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

func publisherConn(service string, results ...chanResult) (*Connection, *Publisher) {
	conn := newConnection(Config{URL: "amqp://test/", ServiceName: service}, newFakeBrokerConn(results...))
	return conn, NewPublisher(conn)
}

func ackingChannel() *fakeChannel {
	ch := newFakeChannel()
	ch.autoConfirm = true
	ch.confirmAck = true
	return ch
}

func publishedHeaders(ch *fakeChannel) amqp.Table {
	return ch.published[len(ch.published)-1].pub.Headers
}

func mustEnvelope(t *testing.T, body []byte) *EventEnvelope {
	t.Helper()
	env, err := UnmarshalEnvelope(body)
	require.NoError(t, err)
	return env
}

func TestNewPublisherUsesConnectionServiceName(t *testing.T) {
	_, p := publisherConn("publisher-name-test")
	assert.Equal(t, "publisher-name-test", p.service)
}

func TestPublishBuildsEnvelopeAndHeaders(t *testing.T) {
	ch := ackingChannel()
	_, p := publisherConn("pub-basic", chanResult{ch: ch})

	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, OrdersRoutingKeyCreated,
		"order.created", map[string]string{"id": "o-1"}, nil))

	require.Len(t, ch.published, 1)
	sent := ch.published[0]
	assert.Equal(t, ExchangeOrders, sent.exchange)
	assert.Equal(t, OrdersRoutingKeyCreated, sent.key)
	assert.Equal(t, "application/json", sent.pub.ContentType)
	assert.Equal(t, amqp.Persistent, sent.pub.DeliveryMode)
	assert.NotEmpty(t, sent.pub.MessageId)
	assert.False(t, sent.pub.Timestamp.IsZero())

	headers := sent.pub.Headers
	assert.NotEmpty(t, headers[HeaderTraceID])
	assert.Equal(t, "order.created", mustEnvelope(t, sent.pub.Body).EventType)

	require.NoError(t, p.Close())
	assert.True(t, ch.closed)
}

func TestPublishMarshalFailure(t *testing.T) {
	_, p := publisherConn("pub-marshal")

	err := p.Publish(context.Background(), ExchangeOrders, "rk", "order.created", make(chan int), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rabbitmq envelope marshal")
}

func TestPublishEnrichesEnvelopeMetadata(t *testing.T) {
	ch := ackingChannel()
	_, p := publisherConn("pub-enrich", chanResult{ch: ch})

	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, "rk", "order.created",
		map[string]string{"id": "1"}, &PublishOptions{
			Producer:      "orders-service",
			AggregateID:   "agg-9",
			AggregateType: "Order",
		}))

	env := mustEnvelope(t, ch.published[0].pub.Body)
	assert.Equal(t, "orders-service", env.Producer)
	assert.Equal(t, "agg-9", env.AggregateID)
	assert.Equal(t, "Order", env.AggregateType)
}

func TestPublishKeepsEnvelopeProducerWhenNotOverridden(t *testing.T) {
	ch := ackingChannel()
	_, p := publisherConn("default-producer-svc", chanResult{ch: ch})

	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, "rk", "order.created",
		map[string]string{"id": "1"}, &PublishOptions{AggregateID: "agg-1"}))

	assert.Equal(t, "agg-1", mustEnvelope(t, ch.published[0].pub.Body).AggregateID)
	assert.NotEmpty(t, mustEnvelope(t, ch.published[0].pub.Body).Producer)
}

func TestPublishExplicitTraceIDWins(t *testing.T) {
	ch := ackingChannel()
	_, p := publisherConn("pub-trace", chanResult{ch: ch})

	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, "rk", "order.created",
		map[string]string{"id": "1"}, &PublishOptions{TraceID: "trace-explicit"}))

	assert.Equal(t, "trace-explicit", publishedHeaders(ch)[HeaderTraceID])
}

func TestPublishGeneratesTraceIDWhenMissing(t *testing.T) {
	ch := ackingChannel()
	_, p := publisherConn("pub-trace-fallback", chanResult{ch: ch})

	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, "rk", "order.created",
		map[string]string{"id": "1"}, nil))

	trace, ok := publishedHeaders(ch)[HeaderTraceID].(string)
	require.True(t, ok)
	assert.NotEmpty(t, trace)
	assert.Len(t, trace, 36) // uuid.NewString()
}

func TestPublishAddsCustomAndCorrelationHeaders(t *testing.T) {
	ch := ackingChannel()
	_, p := publisherConn("pub-headers", chanResult{ch: ch})

	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, "rk", "order.created",
		map[string]string{"id": "1"}, &PublishOptions{
			TraceID:       "tr",
			CorrelationID: "corr-1",
			Headers:       map[string]interface{}{"x-custom": "yes"},
		}))

	headers := publishedHeaders(ch)
	assert.Equal(t, "yes", headers["x-custom"])
	assert.Equal(t, "corr-1", headers["x-correlation-id"])
	assert.Equal(t, "corr-1", ch.published[0].pub.CorrelationId)
}

func TestPublishTransientDeliveryMode(t *testing.T) {
	ch := ackingChannel()
	_, p := publisherConn("pub-transient", chanResult{ch: ch})

	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, "rk", "order.created",
		nil, &PublishOptions{Transient: true, Priority: 7}))

	pub := ch.published[0].pub
	assert.Equal(t, amqp.Transient, pub.DeliveryMode)
	assert.Equal(t, uint8(7), pub.Priority)
}

func TestPublishConfirmTimeoutRecoversOnRetry(t *testing.T) {
	silent := newFakeChannel() // never confirms
	healthy := ackingChannel()
	_, p := publisherConn("pub-timeout-retry", chanResult{ch: silent}, chanResult{ch: healthy})

	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, "rk", "order.created",
		nil, &PublishOptions{ConfirmTimout: 20 * time.Millisecond}))

	// The silent channel published but never confirmed; the retry landed on the healthy one.
	require.Len(t, silent.published, 1)
	require.Len(t, healthy.published, 1)
}

func TestPublishConfirmTimeoutGivesUp(t *testing.T) {
	silent := newFakeChannel()
	_, p := publisherConn("pub-timeout-fail", chanResult{ch: silent}, chanResult{ch: silent})

	err := p.Publish(context.Background(), ExchangeOrders, "rk", "order.created",
		nil, &PublishOptions{ConfirmTimout: 10 * time.Millisecond})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out waiting for publisher confirm")

	assert.Equal(t, float64(1),
		testutil.ToFloat64(failedTotal.WithLabelValues("pub-timeout-fail", ExchangeOrders, "publish_error")))
}

func TestPublishBrokerNackRecoversOnRetry(t *testing.T) {
	nacking := newFakeChannel()
	nacking.autoConfirm = true
	nacking.confirmAck = false
	healthy := ackingChannel()
	conn, p := publisherConn("pub-nack-retry", chanResult{ch: nacking}, chanResult{ch: healthy})

	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, "rk", "order.created", nil, nil))

	require.Len(t, healthy.published, 1)
	require.NoError(t, conn.Close())
}

func TestPublishBrokerNackGivesUp(t *testing.T) {
	nacking := newFakeChannel()
	nacking.autoConfirm = true
	nacking.confirmAck = false
	_, p := publisherConn("pub-nack-fail", chanResult{ch: nacking}, chanResult{ch: nacking})

	err := p.Publish(context.Background(), ExchangeOrders, "rk", "order.created", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broker nack for delivery tag")
}

func TestPublishConfirmTagMismatchStillSucceeds(t *testing.T) {
	ch := ackingChannel()
	ch.seqNo = 5
	ch.confirmTag = 99
	_, p := publisherConn("pub-tag-mismatch", chanResult{ch: ch})

	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, "rk", "order.created", nil, nil))
	require.Len(t, ch.published, 1)
}

func TestPublishContextCancellation(t *testing.T) {
	ch := ackingChannel()
	ch.autoConfirm = false
	_, p := publisherConn("pub-ctx", chanResult{ch: ch}, chanResult{ch: ch})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.Publish(ctx, ExchangeOrders, "rk", "order.created", nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, float64(1),
		testutil.ToFloat64(failedTotal.WithLabelValues("pub-ctx", ExchangeOrders, "publish_error")))
}

func TestPublishReconnectFailureIsReported(t *testing.T) {
	broken := newFakeChannel()
	broken.publishErr = errors.New("channel died")
	withDialAMQP(t, func(url string) (brokerConn, error) {
		return nil, errors.New("broker gone")
	})
	conn := newConnection(Config{
		URL: "amqp://test/", ServiceName: "pub-reconnect-fail",
		MaxAttempts: 1, BaseDelay: time.Nanosecond,
	}, newFakeBrokerConn(chanResult{ch: broken}, chanResult{err: errors.New("cannot open channel")}))
	p := NewPublisher(conn)

	err := p.Publish(context.Background(), ExchangeOrders, "rk", "order.created", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rabbitmq publish")
	assert.Equal(t, float64(1),
		testutil.ToFloat64(failedTotal.WithLabelValues("pub-reconnect-fail", ExchangeOrders, "reconnect")))
}

func TestPublishReconnectStillFailingIsReported(t *testing.T) {
	broken := newFakeChannel()
	broken.publishErr = errors.New("channel died")
	_, p := publisherConn("pub-republish-fail",
		chanResult{ch: broken}, chanResult{ch: broken})

	err := p.Publish(context.Background(), ExchangeOrders, "rk", "order.created", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel died")
	assert.Equal(t, float64(1),
		testutil.ToFloat64(failedTotal.WithLabelValues("pub-republish-fail", ExchangeOrders, "publish_error")))
	assert.Equal(t, float64(0),
		testutil.ToFloat64(publishedTotal.WithLabelValues("pub-republish-fail", ExchangeOrders, "rk")))
}

func TestPublishConfirmModeFailure(t *testing.T) {
	ch := newFakeChannel()
	ch.confirmErr = errors.New("confirm refused")
	_, p := publisherConn("pub-confirm-err", chanResult{ch: ch}, chanResult{ch: ch})

	err := p.Publish(context.Background(), ExchangeOrders, "rk", "order.created", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enable publisher confirms")
}

func TestPublishIncrementsPublishedMetric(t *testing.T) {
	ch := ackingChannel()
	_, p := publisherConn("pub-metric", chanResult{ch: ch})

	before := testutil.ToFloat64(publishedTotal.WithLabelValues("pub-metric", ExchangeOrders, "rk"))
	require.NoError(t, p.Publish(context.Background(), ExchangeOrders, "rk", "order.created", nil, nil))
	assert.Equal(t, before+1, testutil.ToFloat64(publishedTotal.WithLabelValues("pub-metric", ExchangeOrders, "rk")))
}

func TestPublisherCloseWithoutChannel(t *testing.T) {
	_, p := publisherConn("pub-close-empty")
	require.NoError(t, p.Close())
}

func TestPublisherClosePropagatesChannelError(t *testing.T) {
	ch := ackingChannel()
	ch.closeErr = errors.New("close failed")
	_, p := publisherConn("pub-close-err", chanResult{ch: ch})

	require.NoError(t, p.ensureChannel())
	require.Error(t, p.Close())
}

func TestPublisherEnsureChannelReusesLiveChannel(t *testing.T) {
	ch := ackingChannel()
	_, p := publisherConn("pub-reuse", chanResult{ch: ch})

	require.NoError(t, p.ensureChannel())
	first := p.ch
	require.NoError(t, p.ensureChannel())
	assert.Same(t, first, p.ch)
}
