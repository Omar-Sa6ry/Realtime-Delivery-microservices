package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWriter struct {
	msgs   []kafkago.Message
	err    error
	closed bool
}

func (w *fakeWriter) WriteMessages(ctx context.Context, msgs ...kafkago.Message) error {
	if w.err != nil {
		return w.err
	}
	w.msgs = append(w.msgs, msgs...)
	return nil
}

func (w *fakeWriter) Close() error {
	w.closed = true
	return nil
}

type fetchResult struct {
	msg kafkago.Message
	err error
}

type fakeReader struct {
	fetches   []fetchResult
	idx       int
	cfg       kafkago.ReaderConfig
	commits   []kafkago.Message
	commitErr error
	closed    bool
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	if r.idx < len(r.fetches) {
		fr := r.fetches[r.idx]
		r.idx++
		return fr.msg, fr.err
	}
	return kafkago.Message{}, context.Canceled
}

func (r *fakeReader) CommitMessages(ctx context.Context, msgs ...kafkago.Message) error {
	if r.commitErr != nil {
		return r.commitErr
	}
	r.commits = append(r.commits, msgs...)
	return nil
}

func (r *fakeReader) Config() kafkago.ReaderConfig { return r.cfg }
func (r *fakeReader) Close() error                 { r.closed = true; return nil }

func newTestConsumer(r *fakeReader, retries int) *Consumer {
	return &Consumer{reader: r, maxRetries: retries, retryDelay: 0}
}

func TestPublishBuildsKafkaMessage(t *testing.T) {
	w := &fakeWriter{}
	p := &Producer{writer: w}

	require.NoError(t, p.Publish(context.Background(), "orders", "order-1", []byte(`{"a":1}`), "trace-9"))

	require.Len(t, w.msgs, 1)
	got := w.msgs[0]
	assert.Equal(t, "orders", got.Topic)
	assert.Equal(t, []byte("order-1"), got.Key)
	assert.Equal(t, []byte(`{"a":1}`), got.Value)
	assert.False(t, got.Time.IsZero())

	headers := map[string]string{}
	for _, h := range got.Headers {
		headers[h.Key] = string(h.Value)
	}
	assert.Equal(t, "trace-9", headers["x-trace-id"])
	assert.NotEmpty(t, headers["x-timestamp"])
}

func TestPublishWrapsWriterError(t *testing.T) {
	p := &Producer{writer: &fakeWriter{err: errors.New("broker down")}}

	err := p.Publish(context.Background(), "orders", "k", nil, "t")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `kafka publish to topic "orders"`)
	assert.Contains(t, err.Error(), "broker down")
}

func TestPublishEnvelopeRoundTrip(t *testing.T) {
	w := &fakeWriter{}
	p := &Producer{writer: w}

	require.NoError(t, p.PublishEnvelope(context.Background(), "drivers", "d-1", "driver.location", "trace-1", map[string]float64{"lat": 1.5}))

	require.Len(t, w.msgs, 1)
	env, err := UnmarshalEnvelope(w.msgs[0].Value)
	require.NoError(t, err)
	assert.Equal(t, "driver.location", env.EventType)
	assert.Equal(t, "trace-1", env.TraceID)
	assert.Equal(t, "trace-1", string(w.msgs[0].Headers[0].Value))
}

func TestPublishEnvelopeMarshalFailure(t *testing.T) {
	p := &Producer{writer: &fakeWriter{}}

	err := p.PublishEnvelope(context.Background(), "t", "k", "evt", "tr", make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kafka envelope marshal")
}

func TestProducerClose(t *testing.T) {
	w := &fakeWriter{}
	require.NoError(t, (&Producer{writer: w}).Close())
	assert.True(t, w.closed)
}

// -- EnsureTopics --

type fakeTopicConn struct {
	created []kafkago.TopicConfig
	err     error
	closed  bool
}

func (c *fakeTopicConn) CreateTopics(topics ...kafkago.TopicConfig) error {
	if c.err != nil {
		return c.err
	}
	c.created = append(c.created, topics...)
	return nil
}

func (c *fakeTopicConn) Close() error {
	c.closed = true
	return nil
}

func withDialStub(t *testing.T, conn TopicConn, err error, calls *int) {
	t.Helper()
	orig := dialKafka
	dialKafka = func(network, address string) (TopicConn, error) {
		if calls != nil {
			*calls++
		}
		return conn, err
	}
	t.Cleanup(func() { dialKafka = orig })
}

func TestEnsureTopicsRejectsEmptyBrokers(t *testing.T) {
	err := EnsureTopics(nil, []string{"a"}, 3, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no brokers provided")
}

func TestEnsureTopicsDialFailure(t *testing.T) {
	withDialStub(t, nil, errors.New("dial tcp: refused"), nil)

	err := EnsureTopics([]string{"127.0.0.1:9092"}, []string{"a"}, 3, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dial kafka for topic creation")
}

func TestEnsureTopicsCreatesConfiguredTopics(t *testing.T) {
	fc := &fakeTopicConn{}
	withDialStub(t, fc, nil, nil)

	require.NoError(t, EnsureTopics([]string{"k:9092"}, []string{"orders", "drivers"}, 6, 2))

	assert.True(t, fc.closed)
	require.Len(t, fc.created, 2)
	assert.Equal(t, kafkago.TopicConfig{Topic: "orders", NumPartitions: 6, ReplicationFactor: 2}, fc.created[0])
	assert.Equal(t, "drivers", fc.created[1].Topic)
}

func TestEnsureTopicsToleratesAlreadyExists(t *testing.T) {
	fc := &fakeTopicConn{err: errors.New("kafka: topic already exists TopicAlreadyExists")}
	withDialStub(t, fc, nil, nil)

	require.NoError(t, EnsureTopics([]string{"k:9092"}, []string{"orders"}, 1, 1))
}

func TestEnsureTopicsPropagatesCreateFailure(t *testing.T) {
	fc := &fakeTopicConn{err: errors.New("not a broker")}
	withDialStub(t, fc, nil, nil)

	err := EnsureTopics([]string{"k:9092"}, []string{"orders"}, 1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create topics: not a broker")
}

// -- processWithRetry --

func TestProcessWithRetrySucceedsFirstAttempt(t *testing.T) {
	c := newTestConsumer(&fakeReader{}, 3)
	calls := 0

	err := c.processWithRetry(context.Background(), kafkago.Message{Offset: 7},
		func(ctx context.Context, msg kafkago.Message) error {
			calls++
			return nil
		})

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestProcessWithRetryRetriesTransientFailures(t *testing.T) {
	c := newTestConsumer(&fakeReader{}, 3)
	calls := 0

	err := c.processWithRetry(context.Background(), kafkago.Message{Offset: 7},
		func(ctx context.Context, msg kafkago.Message) error {
			calls++
			if calls < 3 {
				return errors.New("transient")
			}
			return nil
		})

	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestProcessWithRetryStopsOnPermanentFailure(t *testing.T) {
	c := newTestConsumer(&fakeReader{}, 5)
	calls := 0

	err := c.processWithRetry(context.Background(), kafkago.Message{},
		func(ctx context.Context, msg kafkago.Message) error {
			calls++
			return ErrPermanent
		})

	require.ErrorIs(t, err, ErrPermanent)
	assert.Equal(t, 1, calls)
}

func TestProcessWithRetryExhaustsAttempts(t *testing.T) {
	c := newTestConsumer(&fakeReader{}, 2)
	calls := 0

	err := c.processWithRetry(context.Background(), kafkago.Message{},
		func(ctx context.Context, msg kafkago.Message) error {
			calls++
			return errors.New("always")
		})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exhausted 2 retries")
	assert.Equal(t, 3, calls) // 1 initial + 2 retries
}

func TestProcessWithRetryHonoursContextCancellation(t *testing.T) {
	c := newTestConsumer(&fakeReader{}, 5)
	c.retryDelay = time.Hour
	ctx, cancel := context.WithCancel(context.Background())

	err := c.processWithRetry(ctx, kafkago.Message{},
		func(ctx context.Context, msg kafkago.Message) error {
			cancel()
			return errors.New("transient")
		})

	require.ErrorIs(t, err, context.Canceled)
}

// -- routeToDLQ --

func TestRouteToDLQWithoutProducerIsNoop(t *testing.T) {
	r := &fakeReader{cfg: kafkago.ReaderConfig{Topic: "orders"}}
	c := &Consumer{reader: r, dlqProducer: nil}

	assert.NotPanics(t, func() {
		c.routeToDLQ(context.Background(), kafkago.Message{}, errors.New("bad"))
	})
}

func TestRouteToDLQWritesToDeadLetterTopic(t *testing.T) {
	r := &fakeReader{cfg: kafkago.ReaderConfig{Topic: "orders"}}
	w := &fakeWriter{}
	c := &Consumer{
		reader:      r,
		dlqProducer: &Producer{writer: w},
	}

	msg := kafkago.Message{
		Key:     []byte("k1"),
		Value:   []byte("payload"),
		Headers: []kafkago.Header{{Key: "x-existing", Value: []byte("1")}},
	}
	c.routeToDLQ(context.Background(), msg, errors.New("poison"))

	require.Len(t, w.msgs, 1)
	dlq := w.msgs[0]
	assert.Equal(t, "orders.dlq", dlq.Topic)
	assert.Equal(t, []byte("k1"), dlq.Key)
	assert.Equal(t, []byte("payload"), dlq.Value)

	headers := map[string]string{}
	for _, h := range dlq.Headers {
		headers[h.Key] = string(h.Value)
	}
	assert.Equal(t, "1", headers["x-existing"])
	assert.Equal(t, "poison", headers["x-dlq-reason"])
}

func TestRouteToDLQLogsWriteFailure(t *testing.T) {
	r := &fakeReader{cfg: kafkago.ReaderConfig{Topic: "orders"}}
	c := &Consumer{
		reader:      r,
		dlqProducer: &Producer{writer: &fakeWriter{err: errors.New("dlq broker down")}},
	}

	assert.NotPanics(t, func() {
		c.routeToDLQ(context.Background(), kafkago.Message{}, errors.New("bad"))
	})
}

// -- Run --

func TestRunStopsOnContextCancellation(t *testing.T) {
	r := &fakeReader{
		fetches: []fetchResult{{err: context.Canceled}},
		cfg:     kafkago.ReaderConfig{Topic: "t", GroupID: "g"},
	}
	c := newTestConsumer(r, 1)

	require.NoError(t, c.Run(context.Background(), func(ctx context.Context, msg kafkago.Message) error {
		return nil
	}))
	assert.Empty(t, r.commits)
}

func TestRunProcessesAndCommits(t *testing.T) {
	msg := kafkago.Message{Topic: "t", Offset: 42, Value: []byte("v")}
	r := &fakeReader{
		fetches: []fetchResult{{msg: msg}},
		cfg:     kafkago.ReaderConfig{Topic: "t", GroupID: "g"},
	}
	c := newTestConsumer(r, 1)

	var handled []kafkago.Message
	require.NoError(t, c.Run(context.Background(), func(ctx context.Context, m kafkago.Message) error {
		handled = append(handled, m)
		return nil
	}))

	require.Len(t, handled, 1)
	require.Len(t, r.commits, 1)
	assert.Equal(t, int64(42), r.commits[0].Offset)
}

func TestRunSendsPoisonMessageToDLQThenCommits(t *testing.T) {
	msg := kafkago.Message{Topic: "t", Offset: 9, Value: []byte("v")}
	r := &fakeReader{
		fetches: []fetchResult{{msg: msg}},
		cfg:     kafkago.ReaderConfig{Topic: "t", GroupID: "g"},
	}
	w := &fakeWriter{}
	c := newTestConsumer(r, 1)
	c.dlqProducer = &Producer{writer: w}

	require.NoError(t, c.Run(context.Background(), func(ctx context.Context, m kafkago.Message) error {
		return ErrPermanent
	}))

	require.Len(t, w.msgs, 1)
	assert.Equal(t, "t.dlq", w.msgs[0].Topic)
	require.Len(t, r.commits, 1)
}

func TestRunRetriesTransientErrorsWithoutDLQ(t *testing.T) {
	msg := kafkago.Message{Topic: "t", Offset: 1}
	r := &fakeReader{
		fetches: []fetchResult{{msg: msg}},
		cfg:     kafkago.ReaderConfig{Topic: "t", GroupID: "g"},
	}
	c := newTestConsumer(r, 1)

	calls := 0
	require.NoError(t, c.Run(context.Background(), func(ctx context.Context, m kafkago.Message) error {
		calls++
		return errors.New("nope")
	}))

	// Both attempts failed, so the DLQ (nil) path is taken and the offset still commits.
	assert.Equal(t, 2, calls)
	require.Len(t, r.commits, 1)
}

func TestRunWaitsOutRebalanceThenStopsOnCancel(t *testing.T) {
	r := &fakeReader{
		fetches: []fetchResult{
			{err: errors.New("Rebalance In Progress")},
			{err: context.Canceled},
		},
		cfg: kafkago.ReaderConfig{Topic: "t", GroupID: "g"},
	}
	c := newTestConsumer(r, 1)

	require.NoError(t, c.Run(context.Background(), func(ctx context.Context, msg kafkago.Message) error {
		return nil
	}))
}

func TestRunRecoversFromGenericFetchError(t *testing.T) {
	msg := kafkago.Message{Topic: "t", Offset: 5}
	r := &fakeReader{
		fetches: []fetchResult{
			{err: errors.New("some transient fetch failure")},
			{msg: msg},
		},
		cfg: kafkago.ReaderConfig{Topic: "t", GroupID: "g"},
	}
	c := newTestConsumer(r, 1)

	require.NoError(t, c.Run(context.Background(), func(ctx context.Context, m kafkago.Message) error {
		return nil
	}))
	require.Len(t, r.commits, 1)
}

func TestRunLogsCommitFailure(t *testing.T) {
	msg := kafkago.Message{Topic: "t", Offset: 5}
	r := &fakeReader{
		fetches:   []fetchResult{{msg: msg}},
		cfg:       kafkago.ReaderConfig{Topic: "t", GroupID: "g"},
		commitErr: errors.New("commit refused"),
	}
	c := newTestConsumer(r, 1)

	assert.NotPanics(t, func() {
		require.NoError(t, c.Run(context.Background(), func(ctx context.Context, m kafkago.Message) error {
			return nil
		}))
	})
	assert.Empty(t, r.commits)
}

func TestConsumerClose(t *testing.T) {
	r := &fakeReader{}
	require.NoError(t, newTestConsumer(r, 1).Close())
	assert.True(t, r.closed)
}

func TestNewConsumerDefaultsMaxRetries(t *testing.T) {
	c := NewConsumer(ConsumerConfig{Brokers: []string{"127.0.0.1:1"}, Topic: "t", GroupID: "g"})
	require.NotNil(t, c)
	assert.Equal(t, 3, c.maxRetries)
	assert.Equal(t, 2*time.Second, c.retryDelay)
	require.NoError(t, c.Close())

	c2 := NewConsumer(ConsumerConfig{Brokers: []string{"127.0.0.1:1"}, Topic: "t", GroupID: "g", MaxRetries: 7})
	assert.Equal(t, 7, c2.maxRetries)
	require.NoError(t, c2.Close())
}

func TestNewProducerConfiguresWriter(t *testing.T) {
	p := NewProducer([]string{"127.0.0.1:1"})
	require.NotNil(t, p)
	require.NoError(t, p.Close())
}
