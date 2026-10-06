package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Exchanges.
const (
	ExchangeOrders        = "delivery.orders.topic"
	ExchangeNotifications = "delivery.notifications.fanout"
	ExchangePayments      = "delivery.payments.direct"
	ExchangeDispatch      = "delivery.dispatch.direct"
	ExchangeMedia         = "delivery.media.topic"
	ExchangeRealtime      = "delivery.realtime.fanout"
	ExchangeUsers         = "delivery.users.topic"
	ExchangeAnalytics     = "delivery.analytics.fanout"
	ExchangeDrivers       = "delivery.drivers.topic"
	ExchangeDLX           = "delivery.dlx.fanout"
	ExchangeDLXDirect     = "delivery.dlx.direct"
	ExchangeUnroutable    = "delivery.unroutable.fanout"
)

var ExchangeTypes = map[string]string{
	ExchangeOrders:        "topic",
	ExchangeNotifications: "fanout",
	ExchangePayments:      "direct",
	ExchangeDispatch:      "direct",
	ExchangeMedia:         "topic",
	ExchangeRealtime:      "fanout",
	ExchangeUsers:         "topic",
	ExchangeAnalytics:     "fanout",
	ExchangeDrivers:       "topic",
	ExchangeDLX:           "fanout",
	ExchangeDLXDirect:     "direct",
	ExchangeUnroutable:    "fanout",
}

const (
	OrdersRoutingKeyCreated     = "order.created"
	OrdersRoutingKeyStatusAll   = "order.#"
	OrdersRoutingKeyCancelled   = "order.cancelled"
	OrdersRoutingKeyCompleted   = "order.completed"
	OrdersRoutingKeyPickedUp    = "order.picked_up"
	OrdersRoutingKeyInTransit   = "order.in_transit"
	OrdersRoutingKeyDriverAssgn = "order.driver.assigned"
)

const (
	PaymentsRoutingKeyAuthorized = "payment.authorized"
	PaymentsRoutingKeyFailed     = "payment.failed"
	PaymentsRoutingKeyRefunded   = "payment.refunded"
	PaymentsRoutingKeyCompleted  = "payment.completed"
)

const (
	DispatchRoutingKeyRequest  = "dispatch.request"
	DispatchRoutingKeyResponse = "dispatch.response"
)

const (
	MediaRoutingKeyUploaded  = "media.uploaded"
	MediaRoutingKeyProcessed = "media.processed"
	MediaRoutingKeyAll       = "media.#"
)

const (
	UsersRoutingKeyCreated = "user.created"
	UsersRoutingKeyUpdated = "user.updated"
	UsersRoutingKeyAll     = "user.#"
)

const (
	DriversRoutingKeyAll                = "driver.#"
	DriversRoutingKeyCreated            = "driver.created"
	DriversRoutingKeyAssignmentOffered  = "driver.assignment.offered"
	DriversRoutingKeyAssignmentAccepted = "driver.assignment.accepted"
)

const (
	DLQRoutingKeyOrders        = "orders.dlq"
	DLQRoutingKeyPayments      = "payments.dlq"
	DLQRoutingKeyNotifications = "notifications.dlq"
	DLQRoutingKeyDrivers       = "drivers.dlq"
)

const (
	QueueOrdersCreated     = "orders.created.queue"
	QueueOrdersStatus      = "orders.status.queue"
	QueueNotificationsMail = "notifications.email.queue"
	QueueNotificationsSMS  = "notifications.sms.queue"
	QueueNotificationsPush = "notifications.push.queue"
	QueuePaymentsAuth      = "payments.authorized.queue"
	QueuePaymentsFailed    = "payments.failed.queue"
	QueuePaymentsRefunded  = "payments.refunded.queue"
	QueueDispatchRequests  = "dispatch.requests.queue"
	QueueDispatchResponses = "dispatch.responses.queue"
	QueueMediaUploaded     = "media.uploaded.queue"
	QueueMediaProcessed    = "media.processed.queue"
	QueueRealtimeBroadcast = "realtime.broadcast.queue"
	QueueSearchIndex       = "search.index.queue"
	QueueDeliveryDLQ       = "delivery.dlq.queue"
	QueueUsersCreated      = "users.created.queue"
	QueueUsersUpdated      = "users.updated.queue"
	QueueAnalyticsEvents   = "analytics.events.queue"
	QueueDriversEvents     = "drivers.events.queue"
	QueueDLQOrders         = "orders.dlq.queue"
	QueueDLQPayments       = "payments.dlq.queue"
	QueueDLQNotifications  = "notifications.dlq.queue"
	QueueDLQDrivers        = "drivers.dlq.queue"
	QueueUnroutable        = "unroutable.messages.queue"
)

const (
	HeaderRetryCount = "x-retry-count"
	HeaderTraceID    = "x-trace-id"
	HeaderDLQReason  = "x-dlq-reason"
)

var queueDLQ = map[string]struct {
	Exchange   string
	RoutingKey string
}{
	QueueOrdersCreated:     {ExchangeDLXDirect, DLQRoutingKeyOrders},
	QueueOrdersStatus:      {ExchangeDLXDirect, DLQRoutingKeyOrders},
	QueuePaymentsAuth:      {ExchangeDLXDirect, DLQRoutingKeyPayments},
	QueuePaymentsFailed:    {ExchangeDLXDirect, DLQRoutingKeyPayments},
	QueuePaymentsRefunded:  {ExchangeDLXDirect, DLQRoutingKeyPayments},
	QueueNotificationsMail: {ExchangeDLXDirect, DLQRoutingKeyNotifications},
	QueueNotificationsSMS:  {ExchangeDLXDirect, DLQRoutingKeyNotifications},
	QueueNotificationsPush: {ExchangeDLXDirect, DLQRoutingKeyNotifications},
	QueueDispatchRequests:  {ExchangeDLXDirect, DLQRoutingKeyDrivers},
	QueueDispatchResponses: {ExchangeDLXDirect, DLQRoutingKeyDrivers},
	QueueDriversEvents:     {ExchangeDLXDirect, DLQRoutingKeyDrivers},
	QueueMediaUploaded:     {ExchangeDLXDirect, DLQRoutingKeyNotifications},
	QueueMediaProcessed:    {ExchangeDLXDirect, DLQRoutingKeyNotifications},
	QueueRealtimeBroadcast: {ExchangeDLXDirect, DLQRoutingKeyOrders},
	QueueSearchIndex:       {ExchangeDLXDirect, DLQRoutingKeyOrders},
	QueueUsersCreated:      {ExchangeDLXDirect, DLQRoutingKeyNotifications},
	QueueUsersUpdated:      {ExchangeDLXDirect, DLQRoutingKeyNotifications},
	QueueAnalyticsEvents:   {ExchangeDLXDirect, DLQRoutingKeyNotifications},
}

func DefaultQueueArguments(queue string) amqp.Table {
	if queue == QueueAnalyticsEvents || queue == QueueRealtimeBroadcast {
		return amqp.Table{
			"x-queue-type":                    "stream",
			"x-max-age":                       "7D",
			"x-stream-max-segment-size-bytes": int64(50000000),
		}
	}
	if queue == QueueDeliveryDLQ || queue == QueueDLQOrders || queue == QueueDLQPayments ||
		queue == QueueDLQNotifications || queue == QueueDLQDrivers {
		return amqp.Table{
			"x-queue-type":  "quorum",
			"x-message-ttl": int64(604800000), // 7 days retention
		}
	}
	args := amqp.Table{
		"x-queue-type": "quorum",
	}
	if dlq, ok := queueDLQ[queue]; ok {
		args["x-dead-letter-exchange"] = dlq.Exchange
		args["x-dead-letter-routing-key"] = dlq.RoutingKey
	} else {
		args["x-dead-letter-exchange"] = ExchangeDLX
	}
	return args
}

var prefetchHints = map[string]int{
	QueueNotificationsMail: 10,
	QueueNotificationsSMS:  20,
	QueueNotificationsPush: 50,
}

func PrefetchFor(queue string) int {
	if n, ok := prefetchHints[queue]; ok {
		return n
	}
	return 10
}

// Metrics

var (
	publishedTotal  *prometheus.CounterVec
	consumedTotal   *prometheus.CounterVec
	failedTotal     *prometheus.CounterVec
	dlqTotal        *prometheus.CounterVec
	retryTotal      *prometheus.CounterVec
	connErrorsTotal *prometheus.CounterVec
	metricsOnce     sync.Once
)

func registerMetrics() {
	metricsOnce.Do(func() {
		publishedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rabbitmq_messages_published_total",
				Help: "Total RabbitMQ messages published with confirm (Go services)",
			},
			[]string{"service", "exchange", "routing_key"},
		)
		consumedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rabbitmq_messages_consumed_total",
				Help: "Total RabbitMQ messages consumed and acked (Go services)",
			},
			[]string{"service", "queue"},
		)
		failedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rabbitmq_messages_failed_total",
				Help: "Total RabbitMQ publish/consume failures (Go services)",
			},
			[]string{"service", "queue", "reason"},
		)
		dlqTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rabbitmq_messages_dlq_total",
				Help: "Total RabbitMQ messages routed to a DLQ (Go services)",
			},
			[]string{"service", "queue"},
		)
		retryTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rabbitmq_messages_retry_total",
				Help: "Total RabbitMQ consumer retries (Go services)",
			},
			[]string{"service", "queue"},
		)
		connErrorsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rabbitmq_connection_errors_total",
				Help: "Total RabbitMQ connection/channel errors (Go services)",
			},
			[]string{"service"},
		)
		prometheus.MustRegister(
			publishedTotal, consumedTotal, failedTotal,
			dlqTotal, retryTotal, connErrorsTotal,
		)
	})
}

// Envelope helpers (same contract as the events package / TS envelope)

type EventEnvelope = events.EventEnvelope

func MarshalEnvelope(eventID, eventType, traceID string, payload interface{}) ([]byte, error) {
	return events.MarshalEnvelope(eventID, eventType, traceID, payload)
}

func UnmarshalEnvelope(data []byte) (*EventEnvelope, error) {
	return events.UnmarshalEnvelope(data)
}

// Connection — one per service, multiplexed channels

// Channel is the AMQP channel surface used across this package. Declaring it
// as an interface lets tests exercise topology/publish/consume logic without a
// live broker.
type Channel interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	Qos(prefetchCount, prefetchSize int, global bool) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Confirm(noWait bool) error
	NotifyPublish(confirm chan amqp.Confirmation) chan amqp.Confirmation
	GetNextPublishSeqNo() uint64
	IsClosed() bool
	Close() error
}

// brokerConn is the AMQP connection surface used by Connection.
type brokerConn interface {
	Channel() (Channel, error)
	IsClosed() bool
	Close() error
	NotifyClose(receiver chan *amqp.Error) chan *amqp.Error
}

// realConn adapts *amqp.Connection onto brokerConn.
type realConn struct{ c *amqp.Connection }

func (r *realConn) Channel() (Channel, error) { return r.c.Channel() }
func (r *realConn) IsClosed() bool            { return r.c.IsClosed() }
func (r *realConn) Close() error              { return r.c.Close() }
func (r *realConn) NotifyClose(receiver chan *amqp.Error) chan *amqp.Error {
	return r.c.NotifyClose(receiver)
}

// dialAMQP is swappable so tests never open a real socket.
var dialAMQP = func(url string) (brokerConn, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	return &realConn{c: conn}, nil
}

type Config struct {
	URL         string
	ServiceName string
	// MaxAttempts caps the initial dial retry loop (0 = 15).
	MaxAttempts int
	// BaseDelay is the base backoff delay (0 = 1s, capped at 30s).
	BaseDelay time.Duration
}

func (c Config) withDefaults() Config {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 15
	}
	if c.BaseDelay <= 0 {
		c.BaseDelay = time.Second
	}
	if c.ServiceName == "" {
		c.ServiceName = "delivery-service"
	}
	return c
}

type Connection struct {
	mu     sync.Mutex
	cfg    Config
	conn   brokerConn
	closed bool
}

// newConnection builds a Connection around an already-established broker
// connection (used by tests and by Connect).
func newConnection(cfg Config, conn brokerConn) *Connection {
	return &Connection{cfg: cfg.withDefaults(), conn: conn}
}

func Connect(cfg Config) (*Connection, error) {
	cfg = cfg.withDefaults()
	registerMetrics()
	c := &Connection{cfg: cfg}
	if err := c.ensureConn(); err != nil {
		return nil, err
	}
	return c, nil
}

func ConnectURL(url, serviceName string) (*Connection, error) {
	return Connect(Config{URL: url, ServiceName: serviceName})
}

func (c *Connection) ensureConn() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ensureConnLocked()
}

func (c *Connection) ensureConnLocked() error {
	if c.closed {
		return errors.New("rabbitmq connection is closed")
	}
	if c.conn != nil && !c.conn.IsClosed() {
		return nil
	}
	delay := c.cfg.BaseDelay
	var lastErr error
	for i := 1; i <= c.cfg.MaxAttempts; i++ {
		conn, err := dialAMQP(c.cfg.URL)
		if err == nil {
			c.conn = conn
			closeErr := make(chan *amqp.Error, 1)
			conn.NotifyClose(closeErr)
			go func() {
				if amqpErr := <-closeErr; amqpErr != nil {
					connErrorsTotal.WithLabelValues(c.cfg.ServiceName).Inc()
					slog.Warn("RabbitMQ connection closed",
						"service", c.cfg.ServiceName, "error", amqpErr.Error())
				}
			}()
			if lastErr != nil {
				slog.Info("RabbitMQ connected successfully",
					"service", c.cfg.ServiceName, "attempt", i)
			} else {
				slog.Info("RabbitMQ connected", "service", c.cfg.ServiceName)
			}
			return nil
		}
		lastErr = err
		slog.Warn("RabbitMQ dial failed, retrying...",
			"service", c.cfg.ServiceName,
			"attempt", i, "maxAttempts", c.cfg.MaxAttempts, "error", err)
		c.mu.Unlock()
		time.Sleep(delay)
		c.mu.Lock()
		if c.closed {
			return errors.New("rabbitmq connection is closed")
		}
		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}
	connErrorsTotal.WithLabelValues(c.cfg.ServiceName).Inc()
	return fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w",
		c.cfg.MaxAttempts, lastErr)
}

func (c *Connection) Channel() (Channel, error) {
	if err := c.ensureConn(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	ch, err := conn.Channel()
	if err != nil {
		connErrorsTotal.WithLabelValues(c.cfg.ServiceName).Inc()
		// The connection may be half-open — force a redial and retry once.
		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
		if err := c.ensureConn(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		conn = c.conn
		c.mu.Unlock()
		ch, err = conn.Channel()
		if err != nil {
			connErrorsTotal.WithLabelValues(c.cfg.ServiceName).Inc()
			return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
		}
	}
	return ch, nil
}

func (c *Connection) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil && !c.conn.IsClosed() && !c.closed
}

func (c *Connection) ServiceName() string { return c.cfg.ServiceName }

func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

type TopologyBinding struct {
	Exchange     string
	ExchangeType string // defaults from ExchangeTypes, falls back to "topic"
	Queue        string
	RoutingKey   string
}

func exchangeDeclareArgs(exchange string) amqp.Table {
	args := amqp.Table{}
	if exchange != ExchangeDLX && exchange != ExchangeDLXDirect && exchange != ExchangeUnroutable && !strings.Contains(exchange, "dlq") {
		args["alternate-exchange"] = ExchangeUnroutable
	}
	return args
}

func (c *Connection) EnsureTopology(bindings []TopologyBinding) error {
	ch, err := c.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	seen := map[string]string{}
	for _, b := range bindings {
		if _, ok := seen[b.Exchange]; !ok {
			kind := b.ExchangeType
			if kind == "" {
				kind = ExchangeTypes[b.Exchange]
			}
			if kind == "" {
				kind = "topic"
			}
			seen[b.Exchange] = kind
		}
	}
	for exchange, kind := range seen {
		if err := ch.ExchangeDeclare(exchange, kind, true, false, false, false, exchangeDeclareArgs(exchange)); err != nil {
			return fmt.Errorf("declare exchange %q: %w", exchange, err)
		}
	}
	for _, b := range bindings {
		if _, err := ch.QueueDeclare(b.Queue, true, false, false, false,
			DefaultQueueArguments(b.Queue)); err != nil {
			return fmt.Errorf("declare queue %q: %w", b.Queue, err)
		}
		if err := ch.QueueBind(b.Queue, b.RoutingKey, b.Exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue %q to %q: %w", b.Queue, b.Exchange, err)
		}
	}
	return nil
}

// Publisher — confirms + reconnect + metrics

type PublishOptions struct {
	Transient     bool
	Priority      uint8
	TraceID       string
	Headers       map[string]interface{}
	AggregateID   string
	AggregateType string
	Producer      string
	CorrelationID string
	CausationID   string
	ConfirmTimout time.Duration // default 5s
}

type Publisher struct {
	conn     *Connection
	service  string
	mu       sync.Mutex
	ch       Channel
	confirms chan amqp.Confirmation
}

func NewPublisher(conn *Connection) *Publisher {
	registerMetrics()
	return &Publisher{conn: conn, service: conn.ServiceName()}
}

func (p *Publisher) ensureChannel() error {
	if p.ch != nil && !p.ch.IsClosed() {
		return nil
	}
	ch, err := p.conn.Channel()
	if err != nil {
		return err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return fmt.Errorf("enable publisher confirms: %w", err)
	}
	p.ch = ch
	p.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	return nil
}

func (p *Publisher) Publish(ctx context.Context, exchange, routingKey, eventType string,
	payload interface{}, opts *PublishOptions) error {
	if opts == nil {
		opts = &PublishOptions{}
	}
	body, err := MarshalEnvelope("", eventType, opts.TraceID, payload)
	if err != nil {
		return fmt.Errorf("rabbitmq envelope marshal: %w", err)
	}

	// Enrich the envelope's producer/aggregate metadata when provided.
	if opts.Producer != "" || opts.AggregateID != "" || opts.AggregateType != "" {
		var env map[string]interface{}
		if jerr := json.Unmarshal(body, &env); jerr == nil {
			if opts.Producer != "" {
				env["producer"] = opts.Producer
			} else if _, ok := env["producer"]; !ok {
				env["producer"] = p.service
			}
			if opts.AggregateID != "" {
				env["aggregateId"] = opts.AggregateID
			}
			if opts.AggregateType != "" {
				env["aggregateType"] = opts.AggregateType
			}
			if re, jerr := json.Marshal(env); jerr == nil {
				body = re
			}
		}
	}

	headers := amqp.Table{}
	for k, v := range opts.Headers {
		headers[k] = v
	}
	traceID := opts.TraceID
	if traceID == "" {
		traceID = bothIDs(body)
	}
	if traceID == "" {
		traceID = uuid.NewString()
	}
	headers[HeaderTraceID] = traceID
	if opts.CorrelationID != "" {
		headers["x-correlation-id"] = opts.CorrelationID
	}

	pub := amqp.Publishing{
		Headers:       headers,
		ContentType:   "application/json",
		Body:          body,
		DeliveryMode:  amqp.Persistent,
		Priority:      opts.Priority,
		MessageId:     uuid.NewString(),
		Timestamp:     time.Now(),
		CorrelationId: opts.CorrelationID,
	}
	if opts.Transient {
		pub.DeliveryMode = amqp.Transient
	}

	confirmTimeout := opts.ConfirmTimout
	if confirmTimeout <= 0 {
		confirmTimeout = 5 * time.Second
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.publishOnce(ctx, exchange, routingKey, pub, confirmTimeout); err != nil {
		// Reopen the channel once and retry (covers broker restarts).
		_ = p.closeChannel()
		if rerr := p.ensureChannel(); rerr != nil {
			failedTotal.WithLabelValues(p.service, exchange, "reconnect").Inc()
			return fmt.Errorf("rabbitmq publish %q rk=%q: %w", exchange, routingKey, err)
		}
		if rerr := p.publishOnce(ctx, exchange, routingKey, pub, confirmTimeout); rerr != nil {
			failedTotal.WithLabelValues(p.service, exchange, "publish_error").Inc()
			return fmt.Errorf("rabbitmq publish %q rk=%q: %w", exchange, routingKey, rerr)
		}
	}

	publishedTotal.WithLabelValues(p.service, exchange, routingKey).Inc()
	slog.Debug("RabbitMQ event published",
		"service", p.service, "exchange", exchange, "routing_key", routingKey, "type", eventType)
	return nil
}

func (p *Publisher) publishOnce(ctx context.Context, exchange, routingKey string,
	pub amqp.Publishing, confirmTimeout time.Duration) error {
	if err := p.ensureChannel(); err != nil {
		return err
	}
	seqNo := p.ch.GetNextPublishSeqNo()
	if err := p.ch.PublishWithContext(ctx, exchange, routingKey, false, false, pub); err != nil {
		return err
	}
	select {
	case confirm := <-p.confirms:
		if !confirm.Ack {
			return fmt.Errorf("broker nack for delivery tag %d", confirm.DeliveryTag)
		}
		if confirm.DeliveryTag != seqNo {
			slog.Warn("RabbitMQ confirm tag mismatch",
				"service", p.service, "want", seqNo, "got", confirm.DeliveryTag)
		}
		return nil
	case <-time.After(confirmTimeout):
		return errors.New("timed out waiting for publisher confirm")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func bothIDs(body []byte) string {
	var env struct {
		EventID string `json:"eventId"`
		TraceID string `json:"traceId"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return ""
	}
	if env.TraceID != "" {
		return env.TraceID
	}
	return env.EventID
}

func (p *Publisher) closeChannel() error {
	if p.ch != nil {
		err := p.ch.Close()
		p.ch = nil
		return err
	}
	return nil
}

func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeChannel()
}

type MessageHandler func(ctx context.Context, env *EventEnvelope, msg amqp.Delivery) error

var ErrPermanent = errors.New("permanent processing failure")

type ConsumerConfig struct {
	Conn        *Connection
	Queue       string
	Exchange    string
	RoutingKeys []string
	ConsumerTag string // defaults to "<service>-<queue>"
	Prefetch    int
	MaxRetries  int
	RetryDelay  time.Duration
}

type Consumer struct {
	cfg        ConsumerConfig
	service    string
	maxRetries int
	retryDelay time.Duration
}

func NewConsumer(cfg ConsumerConfig) *Consumer {
	registerMetrics()
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = PrefetchFor(cfg.Queue)
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryDelay := cfg.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 2 * time.Second
	}
	if cfg.ConsumerTag == "" && cfg.Conn != nil {
		cfg.ConsumerTag = cfg.Conn.ServiceName() + "-" + cfg.Queue
	}
	c := &Consumer{cfg: cfg, maxRetries: maxRetries, retryDelay: retryDelay}
	if cfg.Conn != nil {
		c.service = cfg.Conn.ServiceName()
	}
	return c
}

// consumeBackoff is the base delay before reopening a failed consume channel.
// It is a package variable so tests can shrink it.
var consumeBackoff = time.Second

func (c *Consumer) Run(ctx context.Context, handler MessageHandler) error {
	slog.Info("RabbitMQ consumer started",
		"service", c.service, "queue", c.cfg.Queue, "exchange", c.cfg.Exchange)
	backoff := consumeBackoff

	for {
		select {
		case <-ctx.Done():
			slog.Info("RabbitMQ consumer stopped",
				"service", c.service, "queue", c.cfg.Queue)
			return nil
		default:
		}

		if err := c.consumeLoop(ctx, handler); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			slog.Error("RabbitMQ consume loop failed, reopening channel...",
				"service", c.service, "queue", c.cfg.Queue, "error", err,
				"backoff", backoff.String())
			connErrorsTotal.WithLabelValues(c.service).Inc()
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}
		backoff = consumeBackoff
	}
}

func (c *Consumer) consumeLoop(ctx context.Context, handler MessageHandler) error {
	ch, err := c.cfg.Conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	kind := ExchangeTypes[c.cfg.Exchange]
	if kind == "" {
		kind = "topic"
	}
	if err := ch.ExchangeDeclare(c.cfg.Exchange, kind, true, false, false, false, exchangeDeclareArgs(c.cfg.Exchange)); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	if _, err := ch.QueueDeclare(c.cfg.Queue, true, false, false, false,
		DefaultQueueArguments(c.cfg.Queue)); err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	for _, rk := range c.cfg.RoutingKeys {
		if err := ch.QueueBind(c.cfg.Queue, rk, c.cfg.Exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue: %w", err)
		}
	}
	if err := ch.Qos(c.cfg.Prefetch, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	deliveries, err := ch.Consume(c.cfg.Queue, c.cfg.ConsumerTag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("basic consume: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-deliveries:
			if !ok {
				return errors.New("delivery channel closed by broker")
			}
			c.handleDelivery(ctx, ch, msg, handler)
		}
	}
}

func (c *Consumer) handleDelivery(ctx context.Context, ch Channel,
	msg amqp.Delivery, handler MessageHandler) {

	env, err := UnmarshalEnvelope(msg.Body)
	if err != nil {
		failedTotal.WithLabelValues(c.service, c.cfg.Queue, "invalid_json").Inc()
		slog.Error("RabbitMQ poison message, dead-lettering",
			"service", c.service, "queue", c.cfg.Queue, "error", err)
		_ = msg.Nack(false, false)
		dlqTotal.WithLabelValues(c.service, c.cfg.Queue).Inc()
		return
	}

	traceID := headerString(msg.Headers, HeaderTraceID)
	if traceID == "" {
		traceID = env.TraceID
	}

	if err := handler(ctx, env, msg); err == nil {
		_ = msg.Ack(false)
		consumedTotal.WithLabelValues(c.service, c.cfg.Queue).Inc()
		return
	} else if errors.Is(err, ErrPermanent) {
		failedTotal.WithLabelValues(c.service, c.cfg.Queue, "permanent").Inc()
		slog.Error("RabbitMQ permanent failure, dead-lettering",
			"service", c.service, "queue", c.cfg.Queue,
			"event", env.EventType, "error", err)
		_ = msg.Nack(false, false)
		dlqTotal.WithLabelValues(c.service, c.cfg.Queue).Inc()
		return
	} else {
		attempt := headerRetryCount(msg.Headers)
		if attempt < c.maxRetries {
			retryTotal.WithLabelValues(c.service, c.cfg.Queue).Inc()
			delay := c.retryDelay * time.Duration(1<<attempt)
			slog.Warn("RabbitMQ processing failed, retrying...",
				"service", c.service, "queue", c.cfg.Queue,
				"event", env.EventType, "attempt", attempt+1,
				"maxRetries", c.maxRetries, "delay", delay.String(), "error", err)
			select {
			case <-ctx.Done():
				_ = msg.Nack(false, true)
				return
			case <-time.After(delay):
			}
			headers := amqp.Table{}
			for k, v := range msg.Headers {
				headers[k] = v
			}
			headers[HeaderRetryCount] = int32(attempt + 1)
			if traceID != "" {
				headers[HeaderTraceID] = traceID
			}
			rerr := ch.PublishWithContext(ctx, "", c.cfg.Queue, false, false, amqp.Publishing{
				Headers:       headers,
				ContentType:   "application/json",
				Body:          msg.Body,
				DeliveryMode:  amqp.Persistent,
				Priority:      msg.Priority,
				MessageId:     msg.MessageId,
				Timestamp:     time.Now(),
				CorrelationId: msg.CorrelationId,
			})
			if rerr != nil {
				slog.Error("RabbitMQ retry republish failed, requeueing",
					"service", c.service, "queue", c.cfg.Queue, "error", rerr)
				_ = msg.Nack(false, true)
				return
			}
			_ = msg.Ack(false)
			return
		}
		failedTotal.WithLabelValues(c.service, c.cfg.Queue, "max_retries_exceeded").Inc()
		slog.Error("RabbitMQ retries exhausted, dead-lettering",
			"service", c.service, "queue", c.cfg.Queue,
			"event", env.EventType, "error", err)
		_ = msg.Nack(false, false)
		dlqTotal.WithLabelValues(c.service, c.cfg.Queue).Inc()
	}
}

func headerString(headers amqp.Table, key string) string {
	if headers == nil {
		return ""
	}
	if v, ok := headers[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func headerRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	switch v := headers[HeaderRetryCount].(type) {
	case int32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}
