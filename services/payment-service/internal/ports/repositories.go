package ports

import (
	"github.com/realtime-delivery/payment-service/internal/domain"
)

// PaymentRepo defines the interface for payment repository operations.
type PaymentRepo interface {
	Create(*domain.Payment) error
	Get(string) (*domain.Payment, error)
	Update(*domain.Payment) error
	Delete(string) error
	List() ([]*domain.Payment, error)
}

// AttemptRepo defines the interface for payment attempt repository operations.
type AttemptRepo interface {
	Create(*domain.Attempt) error
	Get(string) (*domain.Attempt, error)
	Update(*domain.Attempt) error
	Delete(string) error
	List() ([]*domain.Attempt, error)
}

// RefundRepo defines the interface for payment refund repository operations.
type RefundRepo interface {
	Create(*domain.Refund) error
	Get(string) (*domain.Refund, error)
	Update(*domain.Refund) error
	Delete(string) error
	List() ([]*domain.Refund, error)
}

// PaymentMethodRepo defines the interface for payment method operations.
type PaymentMethodRepo interface {
	List() ([]domain.PaymentMethod, error)
	Get(string) (domain.PaymentMethod, error)
}