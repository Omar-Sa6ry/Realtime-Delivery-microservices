package domain

import (
	"sync"
)

// PaymentRepo defines the interface for payment repository operations.
type PaymentRepo interface {
	Create(*Payment) error
	Get(string) (*Payment, error)
	Update(*Payment) error
	Delete(string) error
	List() ([]*Payment, error)
}

// AttemptRepo defines the interface for payment attempt repository operations.
type AttemptRepo interface {
	Create(*Attempt) error
	Get(string) (*Attempt, error)
	Update(*Attempt) error
	Delete(string) error
	List() ([]*Attempt, error)
}

// RefundRepo defines the interface for payment refund repository operations.
type RefundRepo interface {
	Create(*Refund) error
	Get(string) (*Refund, error)
	Update(*Refund) error
	Delete(string) error
	List() ([]*Refund, error)
}

// IdempotencyRepository stores and retrieves idempotency keys.
type IdempotencyRepository interface {
	Store(key string, result string) error
	Retrieve(key string) (string, bool)
	Delete(key string) error
	Exists(key string) (bool, error)
}

// OutboxMessage represents a message in the outbox for event publishing.
type OutboxMessage struct {
	ID        string
	EventType string
	Payload   string
	CreatedAt int64
	ProcessedAt int64
	Error     string
}

// OutboxRepository handles the outbox pattern for event publishing.
type OutboxRepository interface {
	GetPendingOutboxMessages(limit int) ([]*OutboxMessage, error)
	MarkAsProcessing(ids []string) error
	MarkAsProcessed(ids []string) error
	MarkAsFailed(ids []string, err string) error
}

// State represents the domain state for payment processing.
type State struct {
	mu        sync.RWMutex
	payments  map[string]*Payment
	attempts  map[string]*Attempt
	refunds   map[string]*Refund
}

// NewState creates a new domain state manager.
func NewState() *State {
	return &State{
		payments: make(map[string]*Payment),
		attempts: make(map[string]*Attempt),
		refunds:  make(map[string]*Refund),
	}
}

// MarkAuthorized marks a payment as authorized in state.
func (s *State) MarkAuthorized(p *Payment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[p.ID].Status = PaymentStatusAuthorized
}

// MarkCaptured marks a payment as captured in state.
func (s *State) MarkCaptured(p *Payment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[p.ID].Status = PaymentStatusCaptured
}

// MarkCancelled marks a payment as cancelled in state.
func (s *State) MarkCancelled(p *Payment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[p.ID].Status = PaymentStatusCancelled
}

// MarkRefunded marks a payment as refunded in state.
func (s *State) MarkRefunded(p *Payment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[p.ID].Status = PaymentStatusRefunded
}

// GetPayment retrieves a payment by ID.
func (s *State) GetPayment(id string) (*Payment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.payments[id]
	return p, ok
}

// CreatePayment creates a new payment and stores it.
func (s *State) CreatePayment(p *Payment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[p.ID] = p
}

// UpdatePayment updates an existing payment.
func (s *State) UpdatePayment(p *Payment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payments[p.ID] = p
}

// DeletePayment removes a payment from state.
func (s *State) DeletePayment(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.payments, id)
}

// GetAttempt retrieves an attempt by ID.
func (s *State) GetAttempt(id string) (*Attempt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.attempts[id]
	return a, ok
}

// CreateAttempt creates a new attempt and stores it.
func (s *State) CreateAttempt(a *Attempt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts[a.ID] = a
}

// GetRefund retrieves a refund by ID.
func (s *State) GetRefund(id string) (*Refund, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.refunds[id]
	return r, ok
}

// CreateRefund creates a new refund and stores it.
func (s *State) CreateRefund(r *Refund) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refunds[r.ID] = r
}

// ListPayments returns all payments.
func (s *State) ListPayments() []*Payment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Payment, 0, len(s.payments))
	for _, p := range s.payments {
		result = append(result, p)
	}
	return result
}

// ListAttempts returns all attempts.
func (s *State) ListAttempts() []*Attempt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Attempt, 0, len(s.attempts))
	for _, a := range s.attempts {
		result = append(result, a)
	}
	return result
}

// ListRefunds returns all refunds.
func (s *State) ListRefunds() []*Refund {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Refund, 0, len(s.refunds))
	for _, r := range s.refunds {
		result = append(result, r)
	}
	return result
}