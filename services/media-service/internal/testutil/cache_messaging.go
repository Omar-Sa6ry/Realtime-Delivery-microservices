package testutil

import (
	"context"
	"sync"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

var (
	_ ports.Cache             = (*MockCache)(nil)
	_ ports.EventPublisher    = (*MockPublisher)(nil)
	_ ports.RealtimePublisher = (*MockRealtimePublisher)(nil)
)

type IdempotencyCall struct {
	UserID   string
	Key      string
	Response []byte
	TTL      time.Duration
}

type LockCall struct {
	Resource string
	TTL      time.Duration
}

type MockCache struct {
	mu sync.Mutex

	IdemErr error
	IdemExisting map[string]bool
	IdemResponse []byte

	LockErr error
	LockDenied bool
	ReleaseErr error

	ProgressErr error
	DeleteErr error

	UploadProgress     map[string]int
	ProcessingProgress map[string]int

	IdemCalls    []IdempotencyCall
	LockCalls    []LockCall
	LockReleases []string
	DeletedKeys  []string
}

func NewMockCache() *MockCache {
	return &MockCache{
		IdemExisting:       make(map[string]bool),
		UploadProgress:     make(map[string]int),
		ProcessingProgress: make(map[string]int),
	}
}

func (c *MockCache) CheckAndStoreIdempotency(ctx context.Context, userID, key string, response []byte, ttl time.Duration) (*ports.IdempotencyResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.IdemCalls = append(c.IdemCalls, IdempotencyCall{UserID: userID, Key: key, Response: response, TTL: ttl})
	if c.IdemErr != nil {
		return nil, c.IdemErr
	}
	if c.IdemExisting == nil {
		c.IdemExisting = make(map[string]bool)
	}
	k := userID + ":" + key
	if c.IdemExisting[k] {
		return &ports.IdempotencyResult{Exists: true, CachedResponse: c.IdemResponse}, nil
	}
	c.IdemExisting[k] = true
	return &ports.IdempotencyResult{Exists: false}, nil
}

func (c *MockCache) AcquireLock(ctx context.Context, resource string, ttl time.Duration) (ports.LockToken, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LockCalls = append(c.LockCalls, LockCall{Resource: resource, TTL: ttl})
	if c.LockErr != nil {
		return "", false, c.LockErr
	}
	if c.LockDenied {
		return "", false, nil
	}
	return ports.LockToken("token-" + resource), true, nil
}

func (c *MockCache) ReleaseLock(ctx context.Context, resource string, token ports.LockToken) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LockReleases = append(c.LockReleases, resource)
	return c.ReleaseErr
}

func (c *MockCache) SetUploadProgress(ctx context.Context, uploadID string, percent int, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ProgressErr != nil {
		return c.ProgressErr
	}
	if c.UploadProgress == nil {
		c.UploadProgress = make(map[string]int)
	}
	c.UploadProgress[uploadID] = percent
	return nil
}

func (c *MockCache) GetUploadProgress(ctx context.Context, uploadID string) (int, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ProgressErr != nil {
		return 0, false, c.ProgressErr
	}
	v, ok := c.UploadProgress[uploadID]
	return v, ok, nil
}

func (c *MockCache) SetProcessingProgress(ctx context.Context, mediaID string, percent int, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ProgressErr != nil {
		return c.ProgressErr
	}
	if c.ProcessingProgress == nil {
		c.ProcessingProgress = make(map[string]int)
	}
	c.ProcessingProgress[mediaID] = percent
	return nil
}

func (c *MockCache) GetProcessingProgress(ctx context.Context, mediaID string) (int, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ProgressErr != nil {
		return 0, false, c.ProgressErr
	}
	v, ok := c.ProcessingProgress[mediaID]
	return v, ok, nil
}

func (c *MockCache) DeleteKey(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.DeletedKeys = append(c.DeletedKeys, key)
	return c.DeleteErr
}

type PublishedMessage struct {
	Topic   string
	Key     string
	Payload []byte
	TraceID string
}

type MockPublisher struct {
	mu sync.Mutex

	PublishErr error
	PublishErrFor map[string]error
	CloseErr error

	Messages []PublishedMessage
	Closed   bool
}

func NewMockPublisher() *MockPublisher {
	return &MockPublisher{PublishErrFor: make(map[string]error)}
}

func (p *MockPublisher) Publish(ctx context.Context, topic string, key string, payload []byte, traceID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Messages = append(p.Messages, PublishedMessage{
		Topic:   topic,
		Key:     key,
		Payload: append([]byte(nil), payload...),
		TraceID: traceID,
	})
	if p.PublishErrFor != nil {
		if err, ok := p.PublishErrFor[topic]; ok {
			return err
		}
	}
	return p.PublishErr
}

func (p *MockPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Closed = true
	return p.CloseErr
}

func (p *MockPublisher) Count(topic string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, m := range p.Messages {
		if m.Topic == topic {
			n++
		}
	}
	return n
}

func (p *MockPublisher) ForTopic(topic string) []PublishedMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []PublishedMessage
	for _, m := range p.Messages {
		if m.Topic == topic {
			out = append(out, m)
		}
	}
	return out
}

func (p *MockPublisher) Last() (PublishedMessage, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.Messages) == 0 {
		return PublishedMessage{}, false
	}
	return p.Messages[len(p.Messages)-1], true
}

type RealtimeCall struct {
	Subject string
	Payload interface{}
}

type MockRealtimePublisher struct {
	mu sync.Mutex

	Err   error
	Calls []RealtimeCall
}

func (p *MockRealtimePublisher) PublishProgress(ctx context.Context, subject string, payload interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Calls = append(p.Calls, RealtimeCall{Subject: subject, Payload: payload})
	return p.Err
}

func (p *MockRealtimePublisher) Count(subject string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, c := range p.Calls {
		if c.Subject == subject {
			n++
		}
	}
	return n
}
