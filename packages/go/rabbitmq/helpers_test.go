package rabbitmq

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestMain(m *testing.M) {
	registerMetrics()
	// Safety net: no test may ever open a real AMQP socket.
	dialAMQP = func(url string) (brokerConn, error) {
		return nil, errors.New("amqp dial is disabled in tests")
	}
	os.Exit(m.Run())
}


type fakeAcker struct {
	mu      sync.Mutex
	acks    int
	nacks   int
	rejects int
	requeue bool
	lastTag uint64
	ackErr  error
	nackErr error
}

func (a *fakeAcker) Ack(tag uint64, multiple bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.acks++
	a.lastTag = tag
	return a.ackErr
}

func (a *fakeAcker) Nack(tag uint64, multiple bool, requeue bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nacks++
	a.requeue = requeue
	a.lastTag = tag
	return a.nackErr
}

func (a *fakeAcker) Reject(tag uint64, requeue bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rejects++
	a.requeue = requeue
	return nil
}

type declaredExchange struct {
	name, kind string
	args       amqp.Table
}

type declaredQueue struct {
	name string
	args amqp.Table
}

type boundQueue struct {
	queue, key, exchange string
}

type publishedMessage struct {
	exchange string
	key      string
	pub      amqp.Publishing
}

type fakeChannel struct {
	mu sync.Mutex

	exchangeErr error
	queueErr    error
	bindErr     error
	qosErr      error
	confirmErr  error
	consumeErr  error
	publishErr  error
	closeErr    error

	exchanges []declaredExchange
	queues    []declaredQueue
	binds     []boundQueue
	published []publishedMessage

	qosCall  []int
	seqNo    uint64
	closed   bool
	isClosed bool

	deliveries  chan amqp.Delivery
	confirmSink chan amqp.Confirmation
	autoConfirm bool
	confirmAck  bool
	confirmTag  uint64 // overrides the delivery tag echoed back in confirms
}

func newFakeChannel() *fakeChannel {
	return &fakeChannel{deliveries: make(chan amqp.Delivery, 8)}
}

func (f *fakeChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.exchangeErr != nil {
		return f.exchangeErr
	}
	f.exchanges = append(f.exchanges, declaredExchange{name: name, kind: kind, args: args})
	return nil
}

func (f *fakeChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.queueErr != nil {
		return amqp.Queue{}, f.queueErr
	}
	f.queues = append(f.queues, declaredQueue{name: name, args: args})
	return amqp.Queue{Name: name, Messages: 0, Consumers: 0}, nil
}

func (f *fakeChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.bindErr != nil {
		return f.bindErr
	}
	f.binds = append(f.binds, boundQueue{queue: name, key: key, exchange: exchange})
	return nil
}

func (f *fakeChannel) Qos(prefetchCount, prefetchSize int, global bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.qosCall = append(f.qosCall, prefetchCount)
	return f.qosErr
}

func (f *fakeChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	if f.consumeErr != nil {
		return nil, f.consumeErr
	}
	return f.deliveries, nil
}

func (f *fakeChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	f.mu.Lock()
	if f.publishErr != nil {
		err := f.publishErr
		f.mu.Unlock()
		return err
	}
	f.published = append(f.published, publishedMessage{exchange: exchange, key: key, pub: msg})
	sink := f.confirmSink
	auto := f.autoConfirm
	ack := f.confirmAck
	tag := f.seqNo
	if f.confirmTag != 0 {
		tag = f.confirmTag
	}
	f.mu.Unlock()

	if auto && sink != nil {
		select {
		case sink <- amqp.Confirmation{DeliveryTag: tag, Ack: ack}:
		default:
		}
	}
	return nil
}

func (f *fakeChannel) Confirm(noWait bool) error { return f.confirmErr }

func (f *fakeChannel) NotifyPublish(confirm chan amqp.Confirmation) chan amqp.Confirmation {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.confirmSink = confirm
	return confirm
}

func (f *fakeChannel) GetNextPublishSeqNo() uint64 { return f.seqNo }

func (f *fakeChannel) IsClosed() bool { return f.isClosed }

func (f *fakeChannel) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return f.closeErr
}

// fake broker connection 

type chanResult struct {
	ch  Channel
	err error
}

type fakeBrokerConn struct {
	results  []chanResult
	idx      int
	closed   bool
	closeErr error
	receiver chan *amqp.Error
}

func newFakeBrokerConn(results ...chanResult) *fakeBrokerConn {
	return &fakeBrokerConn{results: results}
}

func (f *fakeBrokerConn) Channel() (Channel, error) {
	if f.idx < len(f.results) {
		r := f.results[f.idx]
		f.idx++
		return r.ch, r.err
	}
	if len(f.results) > 0 {
		last := f.results[len(f.results)-1]
		return last.ch, last.err
	}
	return nil, errors.New("no channel configured")
}

func (f *fakeBrokerConn) IsClosed() bool { return f.closed }

func (f *fakeBrokerConn) Close() error {
	f.closed = true
	return f.closeErr
}

func (f *fakeBrokerConn) NotifyClose(receiver chan *amqp.Error) chan *amqp.Error {
	f.receiver = receiver
	return receiver
}

func (f *fakeBrokerConn) channelCalls() int { return f.idx }

// dial stub

func withDialAMQP(t *testing.T, dial func(string) (brokerConn, error)) {
	t.Helper()
	orig := dialAMQP
	dialAMQP = dial
	t.Cleanup(func() { dialAMQP = orig })
}

func testConnection(results ...chanResult) *Connection {
	return newConnection(Config{URL: "amqp://guest:guest@localhost:5672/", ServiceName: "test-svc"}, newFakeBrokerConn(results...))
}

func newTestConsumer(t *testing.T, queue string, maxRetries int, retryDelay time.Duration) *Consumer {
	t.Helper()
	return NewConsumer(ConsumerConfig{
		Conn:        testConnection(),
		Queue:       queue,
		Exchange:    ExchangeOrders,
		RoutingKeys: []string{OrdersRoutingKeyCreated},
		MaxRetries:  maxRetries,
		RetryDelay:  retryDelay,
		ConsumerTag: "test-consumer",
	})
}
