package rabbitmq

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- pure helpers -----------------------------------------------------------

func TestDefaultQueueArgumentsStreamQueues(t *testing.T) {
	for _, q := range []string{QueueAnalyticsEvents, QueueRealtimeBroadcast} {
		args := DefaultQueueArguments(q)
		assert.Equal(t, "stream", args["x-queue-type"], q)
		assert.Equal(t, "7D", args["x-max-age"], q)
		assert.Equal(t, int64(50000000), args["x-stream-max-segment-size-bytes"], q)
		assert.NotContains(t, args, "x-dead-letter-exchange", q)
	}
}

func TestDefaultQueueArgumentsDLQQueues(t *testing.T) {
	for _, q := range []string{QueueDeliveryDLQ, QueueDLQOrders, QueueDLQPayments, QueueDLQNotifications, QueueDLQDrivers} {
		args := DefaultQueueArguments(q)
		assert.Equal(t, "quorum", args["x-queue-type"], q)
		assert.Equal(t, int64(604800000), args["x-message-ttl"], q)
		assert.NotContains(t, args, "x-dead-letter-exchange", q)
	}
}

func TestDefaultQueueArgumentsRouting(t *testing.T) {
	tests := []struct {
		queue, dlx, rk string
	}{
		{QueueOrdersCreated, ExchangeDLXDirect, DLQRoutingKeyOrders},
		{QueuePaymentsAuth, ExchangeDLXDirect, DLQRoutingKeyPayments},
		{QueueNotificationsSMS, ExchangeDLXDirect, DLQRoutingKeyNotifications},
		{QueueDispatchRequests, ExchangeDLXDirect, DLQRoutingKeyDrivers},
		{QueueSearchIndex, ExchangeDLXDirect, DLQRoutingKeyOrders},
		{QueueUsersCreated, ExchangeDLXDirect, DLQRoutingKeyNotifications},
		{QueueUnroutable, ExchangeDLX, ""},
		{"totally.unknown.queue", ExchangeDLX, ""},
	}

	for _, tt := range tests {
		t.Run(tt.queue, func(t *testing.T) {
			args := DefaultQueueArguments(tt.queue)
			assert.Equal(t, "quorum", args["x-queue-type"])
			assert.Equal(t, tt.dlx, args["x-dead-letter-exchange"])
			if tt.rk == "" {
				assert.NotContains(t, args, "x-dead-letter-routing-key")
			} else {
				assert.Equal(t, tt.rk, args["x-dead-letter-routing-key"])
			}
		})
	}
}

func TestPrefetchFor(t *testing.T) {
	assert.Equal(t, 10, PrefetchFor(QueueNotificationsMail))
	assert.Equal(t, 20, PrefetchFor(QueueNotificationsSMS))
	assert.Equal(t, 50, PrefetchFor(QueueNotificationsPush))
	assert.Equal(t, 10, PrefetchFor(QueueOrdersCreated))
}

func TestExchangeDeclareArgs(t *testing.T) {
	assert.NotContains(t, exchangeDeclareArgs(ExchangeDLX), "alternate-exchange")
	assert.NotContains(t, exchangeDeclareArgs(ExchangeDLXDirect), "alternate-exchange")
	assert.NotContains(t, exchangeDeclareArgs(ExchangeUnroutable), "alternate-exchange")
	assert.NotContains(t, exchangeDeclareArgs("delivery.something.dlq.topic"), "alternate-exchange")
	assert.Equal(t, ExchangeUnroutable, exchangeDeclareArgs(ExchangeOrders)["alternate-exchange"])
}

func TestExchangeTypesCoversEveryDeclaredExchange(t *testing.T) {
	exchanges := []string{
		ExchangeOrders, ExchangeNotifications, ExchangePayments, ExchangeDispatch,
		ExchangeMedia, ExchangeRealtime, ExchangeUsers, ExchangeAnalytics,
		ExchangeDrivers, ExchangeDLX, ExchangeDLXDirect, ExchangeUnroutable,
	}
	for _, ex := range exchanges {
		assert.Contains(t, ExchangeTypes, ex, ex)
	}
}

func TestConfigWithDefaults(t *testing.T) {
	got := Config{}.withDefaults()
	assert.Equal(t, 15, got.MaxAttempts)
	assert.Equal(t, time.Second, got.BaseDelay)
	assert.Equal(t, "delivery-service", got.ServiceName)

	custom := Config{MaxAttempts: 4, BaseDelay: 10 * time.Millisecond, ServiceName: "orders"}.withDefaults()
	assert.Equal(t, 4, custom.MaxAttempts)
	assert.Equal(t, 10*time.Millisecond, custom.BaseDelay)
	assert.Equal(t, "orders", custom.ServiceName)
}

func TestEnvelopeWrappers(t *testing.T) {
	data, err := MarshalEnvelope("e-1", "order.created", "tr-1", map[string]string{"id": "9"})
	require.NoError(t, err)

	env, err := UnmarshalEnvelope(data)
	require.NoError(t, err)
	assert.Equal(t, "e-1", env.EventID)
	assert.Equal(t, "order.created", env.EventType)
	assert.Equal(t, "tr-1", env.TraceID)

	_, err = UnmarshalEnvelope([]byte("not json"))
	require.Error(t, err)
}

func TestBothIDs(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"prefers trace id", `{"traceId":"trace-1","eventId":"evt-1"}`, "trace-1"},
		{"falls back to event id", `{"eventId":"evt-1"}`, "evt-1"},
		{"invalid json", `{{{`, ""},
		{"empty object", `{}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, bothIDs([]byte(tt.body)))
		})
	}
}

func TestHeaderString(t *testing.T) {
	assert.Equal(t, "", headerString(nil, HeaderTraceID))
	assert.Equal(t, "", headerString(amqp.Table{}, HeaderTraceID))
	assert.Equal(t, "tr-1", headerString(amqp.Table{HeaderTraceID: "tr-1"}, HeaderTraceID))
	assert.Equal(t, "", headerString(amqp.Table{HeaderTraceID: 42}, HeaderTraceID))
}

func TestHeaderRetryCount(t *testing.T) {
	tests := []struct {
		name string
		h    amqp.Table
		want int
	}{
		{"nil headers", nil, 0},
		{"missing", amqp.Table{}, 0},
		{"int32", amqp.Table{HeaderRetryCount: int32(3)}, 3},
		{"int64", amqp.Table{HeaderRetryCount: int64(4)}, 4},
		{"int", amqp.Table{HeaderRetryCount: 5}, 5},
		{"float64", amqp.Table{HeaderRetryCount: float64(6)}, 6},
		{"unsupported type", amqp.Table{HeaderRetryCount: "7"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headerRetryCount(tt.h))
		})
	}
}

// --- Connection --------------------------------------------------------------

func TestConnectionBasics(t *testing.T) {
	conn := testConnection()
	assert.Equal(t, "test-svc", conn.ServiceName())
	assert.True(t, conn.IsConnected())

	require.NoError(t, conn.Close())
	assert.False(t, conn.IsConnected())

	require.NoError(t, conn.Close()) // idempotent
}

func TestConnectionEnsureConnRejectsWhenClosed(t *testing.T) {
	conn := testConnection()
	require.NoError(t, conn.Close())

	_, err := conn.Channel()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rabbitmq connection is closed")
}

func TestConnectSuccess(t *testing.T) {
	fc := newFakeBrokerConn(chanResult{ch: newFakeChannel()})
	withDialAMQP(t, func(url string) (brokerConn, error) {
		assert.Equal(t, "amqp://other:5672/", url)
		return fc, nil
	})

	conn, err := Connect(Config{URL: "amqp://other:5672/", ServiceName: "connect-test"})
	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.True(t, conn.IsConnected())
	assert.Equal(t, "connect-test", conn.ServiceName())
	require.NoError(t, conn.Close())
}

func TestConnectUsesDefaultServiceName(t *testing.T) {
	withDialAMQP(t, func(url string) (brokerConn, error) {
		return newFakeBrokerConn(), nil
	})

	conn, err := ConnectURL("amqp://x/", "")
	require.NoError(t, err)
	assert.Equal(t, "delivery-service", conn.ServiceName())
	require.NoError(t, conn.Close())
}

func TestConnectFailsAfterAllAttempts(t *testing.T) {
	calls := 0
	withDialAMQP(t, func(url string) (brokerConn, error) {
		calls++
		return nil, errors.New("dial refused")
	})

	conn, err := Connect(Config{URL: "amqp://x/", MaxAttempts: 3, BaseDelay: time.Nanosecond})
	require.Error(t, err)
	assert.Nil(t, conn)
	assert.Equal(t, 3, calls)
	assert.Contains(t, err.Error(), "failed to connect to RabbitMQ after 3 attempts")
}

func TestConnectionChannelOpensChannel(t *testing.T) {
	ch := newFakeChannel()
	conn := testConnection(chanResult{ch: ch})

	got, err := conn.Channel()
	require.NoError(t, err)
	assert.Same(t, ch, got)
}

func TestConnectionChannelRedialsAfterFailure(t *testing.T) {
	first := testConnection(chanResult{err: errors.New("channel failed")})
	redial := newFakeBrokerConn(chanResult{ch: newFakeChannel()})
	withDialAMQP(t, func(url string) (brokerConn, error) { return redial, nil })

	got, err := first.Channel()
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, redial.channelCalls())
}

func TestConnectionChannelStillFailingAfterRedial(t *testing.T) {
	withDialAMQP(t, func(url string) (brokerConn, error) {
		return newFakeBrokerConn(chanResult{err: errors.New("no channels")}), nil
	})
	conn := testConnection(chanResult{err: errors.New("first channel failed")})

	_, err := conn.Channel()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open RabbitMQ channel")
}

func TestConnectionChannelFailsWhenRedialFails(t *testing.T) {
	withDialAMQP(t, func(url string) (brokerConn, error) {
		return nil, errors.New("broker gone")
	})
	conn := testConnection(chanResult{err: errors.New("channel failed")})
	conn.cfg.MaxAttempts = 1
	conn.cfg.BaseDelay = time.Nanosecond

	_, err := conn.Channel()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to RabbitMQ")
}

func TestConnectNotifierCountsConnectionErrors(t *testing.T) {
	fc := newFakeBrokerConn()
	withDialAMQP(t, func(url string) (brokerConn, error) { return fc, nil })

	conn, err := Connect(Config{URL: "amqp://x/", ServiceName: "notify-svc"})
	require.NoError(t, err)

	before := testutil.ToFloat64(connErrorsTotal.WithLabelValues("notify-svc"))
	fc.receiver <- &amqp.Error{Code: 320, Reason: "connection closed by peer"}

	require.Eventually(t, func() bool {
		return testutil.ToFloat64(connErrorsTotal.WithLabelValues("notify-svc")) > before
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, conn.Close())
}

func TestConnectNotifierIgnoresNilError(t *testing.T) {
	fc := newFakeBrokerConn()
	withDialAMQP(t, func(url string) (brokerConn, error) { return fc, nil })

	conn, err := Connect(Config{URL: "amqp://x/", ServiceName: "notify-nil-svc"})
	require.NoError(t, err)

	before := testutil.ToFloat64(connErrorsTotal.WithLabelValues("notify-nil-svc"))
	close(fc.receiver)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, before, testutil.ToFloat64(connErrorsTotal.WithLabelValues("notify-nil-svc")))
	require.NoError(t, conn.Close())
}

// --- EnsureTopology ----------------------------------------------------------

func TestEnsureTopologyDeclaresAndBinds(t *testing.T) {
	ch := newFakeChannel()
	conn := testConnection(chanResult{ch: ch})

	bindings := []TopologyBinding{
		{Exchange: ExchangeOrders, Queue: QueueOrdersCreated, RoutingKey: OrdersRoutingKeyCreated},
		{Exchange: ExchangeOrders, Queue: QueueOrdersStatus, RoutingKey: OrdersRoutingKeyStatusAll},
	}

	require.NoError(t, conn.EnsureTopology(bindings))

	// Exchange declared once despite two bindings.
	require.Len(t, ch.exchanges, 1)
	assert.Equal(t, ExchangeOrders, ch.exchanges[0].name)
	assert.Equal(t, "topic", ch.exchanges[0].kind)
	assert.Equal(t, ExchangeUnroutable, ch.exchanges[0].args["alternate-exchange"])

	require.Len(t, ch.queues, 2)
	assert.Equal(t, QueueOrdersCreated, ch.queues[0].name)
	assert.Equal(t, DLQRoutingKeyOrders, ch.queues[0].args["x-dead-letter-routing-key"])

	require.Len(t, ch.binds, 2)
	assert.Equal(t, boundQueue{QueueOrdersCreated, OrdersRoutingKeyCreated, ExchangeOrders}, ch.binds[0])
	assert.True(t, ch.closed, "channel must be released")
}

func TestEnsureTopologyExplicitAndFallbackExchangeType(t *testing.T) {
	ch := newFakeChannel()
	conn := testConnection(chanResult{ch: ch})

	require.NoError(t, conn.EnsureTopology([]TopologyBinding{
		{Exchange: "custom.direct", ExchangeType: "direct", Queue: QueueOrdersCreated, RoutingKey: "k"},
		{Exchange: "unknown.exchange", Queue: QueueOrdersStatus, RoutingKey: "k2"},
	}))

	kinds := map[string]string{}
	for _, e := range ch.exchanges {
		kinds[e.name] = e.kind
	}
	assert.Equal(t, "direct", kinds["custom.direct"])
	assert.Equal(t, "topic", kinds["unknown.exchange"])
}

func TestEnsureTopologyChannelFailure(t *testing.T) {
	withDialAMQP(t, func(url string) (brokerConn, error) {
		return nil, errors.New("no broker")
	})
	conn := testConnection(chanResult{err: errors.New("channel failed")})
	conn.cfg.MaxAttempts = 1
	conn.cfg.BaseDelay = time.Nanosecond

	err := conn.EnsureTopology([]TopologyBinding{{Exchange: ExchangeOrders, Queue: QueueOrdersCreated}})
	require.Error(t, err)
}

func TestEnsureTopologyExchangeDeclareFailure(t *testing.T) {
	ch := newFakeChannel()
	ch.exchangeErr = errors.New("access refused")
	conn := testConnection(chanResult{ch: ch})

	err := conn.EnsureTopology([]TopologyBinding{{Exchange: ExchangeOrders, Queue: QueueOrdersCreated}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `declare exchange "delivery.orders.topic"`)
}

func TestEnsureTopologyQueueDeclareFailure(t *testing.T) {
	ch := newFakeChannel()
	ch.queueErr = errors.New("queue refused")
	conn := testConnection(chanResult{ch: ch})

	err := conn.EnsureTopology([]TopologyBinding{{Exchange: ExchangeOrders, Queue: QueueOrdersCreated}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `declare queue "orders.created.queue"`)
}

func TestEnsureTopologyQueueBindFailure(t *testing.T) {
	ch := newFakeChannel()
	ch.bindErr = errors.New("bind refused")
	conn := testConnection(chanResult{ch: ch})

	err := conn.EnsureTopology([]TopologyBinding{{Exchange: ExchangeOrders, Queue: QueueOrdersCreated, RoutingKey: "k"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `bind queue "orders.created.queue"`)
}
