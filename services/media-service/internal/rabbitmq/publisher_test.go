package rabbitmq

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type fakeChannel struct {
	seq              uint64
	notify           chan amqp.Confirmation
	published        []amqp.Publishing
	publishedKeys    []string
	declaredExchange []string
	declaredQueues   []string
	boundQueues      []string
	closed           bool
}

func (f *fakeChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	f.declaredExchange = append(f.declaredExchange, name)
	return nil
}

func (f *fakeChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	f.declaredQueues = append(f.declaredQueues, name)
	return amqp.Queue{Name: name}, nil
}

func (f *fakeChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	f.boundQueues = append(f.boundQueues, name)
	return nil
}

func (f *fakeChannel) Qos(prefetchCount, prefetchSize int, global bool) error { return nil }

func (f *fakeChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	return make(chan amqp.Delivery), nil
}

func (f *fakeChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	f.published = append(f.published, msg)
	f.publishedKeys = append(f.publishedKeys, key)
	if f.notify != nil {
		f.notify <- amqp.Confirmation{DeliveryTag: f.seq, Ack: true}
	}
	return nil
}

func (f *fakeChannel) Confirm(noWait bool) error { return nil }

func (f *fakeChannel) NotifyPublish(confirm chan amqp.Confirmation) chan amqp.Confirmation {
	f.notify = confirm
	return confirm
}

func (f *fakeChannel) GetNextPublishSeqNo() uint64 {
	f.seq++
	return f.seq
}

func (f *fakeChannel) IsClosed() bool { return f.closed }

func (f *fakeChannel) Close() error {
	f.closed = true
	return nil
}

type fakeBrokerConn struct{ ch shared.Channel }

func (f *fakeBrokerConn) Channel() (shared.Channel, error) { return f.ch, nil }
func (f *fakeBrokerConn) IsClosed() bool                   { return false }
func (f *fakeBrokerConn) Close() error                     { return nil }
func (f *fakeBrokerConn) NotifyClose(receiver chan *amqp.Error) chan *amqp.Error {
	return make(chan *amqp.Error, 1)
}

func newTestConnection(t *testing.T, ch shared.Channel) *shared.Connection {
	t.Helper()
	conn := &shared.Connection{}
	v := reflect.ValueOf(conn).Elem()
	f := v.FieldByName("conn")
	reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem().Set(reflect.ValueOf(&fakeBrokerConn{ch: ch}))
	return conn
}

func TestRoutingKeyFor(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		mediaType string
		want      string
	}{
		{"empty media type returns base", "media.uploaded", "", "media.uploaded"},
		{"lowercases media type", "media.uploaded", "Video", "media.uploaded.video"},
		{"trims whitespace", "media.uploaded", "  mp3  ", "media.uploaded.mp3"},
		{"keeps compound media types", "media.uploaded", "video/mp4", "media.uploaded.video/mp4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := routingKeyFor(tt.base, tt.mediaType); got != tt.want {
				t.Fatalf("routingKeyFor(%q, %q) = %q, want %q", tt.base, tt.mediaType, got, tt.want)
			}
		})
	}
}

func TestNewMediaRabbitMQPublisher(t *testing.T) {
	ch := &fakeChannel{}
	conn := newTestConnection(t, ch)
	p := NewMediaRabbitMQPublisher(conn)
	if p == nil {
		t.Fatal("NewMediaRabbitMQPublisher returned nil")
	}
	if p.pub == nil {
		t.Fatal("publisher must wrap a shared Publisher")
	}
	t.Cleanup(func() { _ = p.Close() })
}

func TestPublishUploadedSuccess(t *testing.T) {
	ch := &fakeChannel{}
	conn := newTestConnection(t, ch)
	p := NewMediaRabbitMQPublisher(conn)
	t.Cleanup(func() { _ = p.Close() })

	err := p.PublishUploaded(context.Background(), "Video", map[string]string{"mediaId": "m1"}, nil)
	if err != nil {
		t.Fatalf("PublishUploaded: %v", err)
	}
	if len(ch.published) != 1 {
		t.Fatalf("published messages = %d, want 1", len(ch.published))
	}
	if got := ch.publishedKeys[0]; got != "media.uploaded.video" {
		t.Fatalf("routing key = %q, want media.uploaded.video", got)
	}
	if !strings.Contains(string(ch.published[0].Body), `"mediaId":"m1"`) {
		t.Fatalf("body = %s, want payload inside envelope", ch.published[0].Body)
	}
}

func TestPublishProcessedSuccess(t *testing.T) {
	ch := &fakeChannel{}
	conn := newTestConnection(t, ch)
	p := NewMediaRabbitMQPublisher(conn)
	t.Cleanup(func() { _ = p.Close() })

	err := p.PublishProcessed(context.Background(), "audio", map[string]string{"mediaId": "m2"}, nil)
	if err != nil {
		t.Fatalf("PublishProcessed: %v", err)
	}
	if len(ch.publishedKeys) != 1 || ch.publishedKeys[0] != "media.processed.audio" {
		t.Fatalf("published keys = %v, want [media.processed.audio]", ch.publishedKeys)
	}
}

func TestPublishWithoutMediaTypeUsesBaseKey(t *testing.T) {
	ch := &fakeChannel{}
	conn := newTestConnection(t, ch)
	p := NewMediaRabbitMQPublisher(conn)
	t.Cleanup(func() { _ = p.Close() })

	if err := p.PublishUploaded(context.Background(), "", map[string]string{}, nil); err != nil {
		t.Fatalf("PublishUploaded: %v", err)
	}
	if got := ch.publishedKeys[0]; got != shared.MediaRoutingKeyUploaded {
		t.Fatalf("routing key = %q, want %q", got, shared.MediaRoutingKeyUploaded)
	}
}

func TestPublishUploadedNotInitialized(t *testing.T) {
	var p *MediaRabbitMQPublisher
	err := p.PublishUploaded(context.Background(), "video", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("error = %v, want not initialized", err)
	}

	p2 := &MediaRabbitMQPublisher{}
	err = p2.PublishUploaded(context.Background(), "video", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("error = %v, want not initialized", err)
	}
}

func TestPublishProcessedNotInitialized(t *testing.T) {
	var p *MediaRabbitMQPublisher
	err := p.PublishProcessed(context.Background(), "video", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("error = %v, want not initialized", err)
	}

	p2 := &MediaRabbitMQPublisher{}
	err = p2.PublishProcessed(context.Background(), "video", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("error = %v, want not initialized", err)
	}
}

func TestCloseNilPublisher(t *testing.T) {
	var p *MediaRabbitMQPublisher
	if err := p.Close(); err != nil {
		t.Fatalf("Close on nil publisher: %v", err)
	}
	p2 := &MediaRabbitMQPublisher{}
	if err := p2.Close(); err != nil {
		t.Fatalf("Close on publisher without shared publisher: %v", err)
	}
}

func TestEnsureMediaTopology(t *testing.T) {
	ch := &fakeChannel{}
	conn := newTestConnection(t, ch)

	if err := EnsureMediaTopology(conn); err != nil {
		t.Fatalf("EnsureMediaTopology: %v", err)
	}

	if len(ch.declaredExchange) != 1 || ch.declaredExchange[0] != shared.ExchangeMedia {
		t.Fatalf("declared exchanges = %v, want [%s]", ch.declaredExchange, shared.ExchangeMedia)
	}

	wantQueues := map[string]bool{
		shared.QueueMediaUploaded:  true,
		shared.QueueMediaProcessed: true,
		shared.QueueSearchIndex:    true,
	}
	if len(ch.declaredQueues) != len(wantQueues) {
		t.Fatalf("declared queues = %v, want %d media queues", ch.declaredQueues, len(wantQueues))
	}
	for _, q := range ch.declaredQueues {
		if !wantQueues[q] {
			t.Fatalf("unexpected declared queue %q", q)
		}
	}
	if len(ch.boundQueues) != 3 {
		t.Fatalf("bound queues = %v, want 3", ch.boundQueues)
	}
}

func TestEnsureMediaTopologyClosedConnection(t *testing.T) {
	conn := &shared.Connection{}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := EnsureMediaTopology(conn)
	if err == nil {
		t.Fatal("EnsureMediaTopology must fail on a closed connection")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Fatalf("error = %v, want closed connection context", err)
	}
}
