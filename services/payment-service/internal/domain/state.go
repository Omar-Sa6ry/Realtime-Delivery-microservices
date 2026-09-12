package domain

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