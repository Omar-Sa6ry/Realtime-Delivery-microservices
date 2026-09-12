package services

import (
	"context"
	"fmt"
	"log"

	"github.com/realtime-delivery/payment-service/internal/domain"
	"github.com/realtime-delivery/payment-service/internal/ports"
)

// PaymentService orchestrates payment processing operations.
// It coordinates between the domain layer, repositories, and external providers.
type PaymentService struct {
	paymentRepo    domain.PaymentRepo
	attemptRepo    domain.AttemptRepo
	refundRepo     domain.RefundRepo
	provider       ports.PaymentProvider
	idempotency    domain.IdempotencyRepository
	outbox         domain.OutboxRepository
	state          domain.State
	logger         *log.Logger
}

// NewPaymentService creates a new PaymentService with all dependencies.
func NewPaymentService(
	paymentRepo domain.PaymentRepo,
	attemptRepo domain.AttemptRepo,
	refundRepo domain.RefundRepo,
	provider ports.PaymentProvider,
	idempotency domain.IdempotencyRepository,
	outbox domain.OutboxRepository,
	state domain.State,
	logger *log.Logger,
) *PaymentService {
	return &PaymentService{
		paymentRepo:    paymentRepo,
		attemptRepo:    attemptRepo,
		refundRepo:     refundRepo,
		provider:       provider,
		idempotency:    idempotency,
		outbox:         outbox,
		state:          state,
		logger:         logger,
	}
}

// CreatePayment orchestrates the creation of a new payment.
// It checks idempotency, creates the payment, and publishes events.
func (s *PaymentService) CreatePayment(ctx context.Context, deliveryID, userID string, amount int64, currency string) (*domain.Payment, error) {
	// Check idempotency key
	idempotencyKey := generateIdempotencyKey(deliveryID, userID, amount, currency)
	idempotencyValue, exists := s.idempotency.Retrieve(idempotencyKey)
	if exists && idempotencyValue != "" {
		s.logger.Printf("Idempotency key already exists: %s", idempotencyKey)
		return nil, nil // Placeholder: fetch existing payment
	}

	// Create the payment
	payment := domain.NewPayment(deliveryID, userID, amount, currency, "", "")

	// Save idempotency key
	if err := s.idempotency.Store(idempotencyKey, payment.ID); err != nil {
		return nil, fmt.Errorf("failed to store idempotency key: %w", err)
	}

	// Create in repository
	if err := s.paymentRepo.Create(payment); err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	// Update state
	s.state.CreatePayment(payment)

	// TODO: Publish PaymentCreated event to NATS/Kafka
	// s.outbox.PublishPaymentCreated(...)

	s.logger.Printf("Payment created: %s, ID: %s", payment.ID, payment.Status)

	return payment, nil
}

// AuthorizePayment orchestrates the authorization of a payment.
// It validates the payment state and calls the provider.
func (s *PaymentService) AuthorizePayment(ctx context.Context, paymentID string) (*domain.Payment, error) {
	// Retrieve the payment
	payment, err := s.paymentRepo.Get(paymentID)
	if err != nil {
		return nil, err
	}

	// Check if payment can be authorized
	if !payment.CanCapture() && payment.Status != domain.PaymentStatusPending {
		return nil, domain.ErrPaymentInvalidStatus
	}

	// Authorize via provider
	_ = s.provider.AuthorizePayment(ctx, payment)

	// Update payment status
	payment.Status = domain.PaymentStatusAuthorized
	if err := s.paymentRepo.Update(payment); err != nil {
		return nil, fmt.Errorf("failed to update payment: %w", err)
	}

	// Update state
	s.state.MarkAuthorized(payment)

	// TODO: Publish PaymentAuthorized event
	// s.outbox.PublishPaymentAuthorized(...)

	s.logger.Printf("Payment authorized: %s", payment.ID)

	return payment, nil
}

// CapturePayment orchestrates the capture of an authorized payment.
func (s *PaymentService) CapturePayment(ctx context.Context, paymentID string, amount int64) (*domain.Payment, error) {
	// Retrieve the payment
	payment, err := s.paymentRepo.Get(paymentID)
	if err != nil {
		return nil, err
	}

	// Check if payment can be captured
	if !payment.CanCapture() {
		return nil, domain.ErrPaymentNotCaptureable
	}

	// Capture via provider
	_ = s.provider.CapturePayment(ctx, payment, amount)

	// Update payment status
	payment.Status = domain.PaymentStatusCaptured
	if err := s.paymentRepo.Update(payment); err != nil {
		return nil, fmt.Errorf("failed to update payment: %w", err)
	}

	// Update state
	s.state.MarkCaptured(payment)

	// TODO: Publish PaymentCaptured event
	// s.outbox.PublishPaymentCaptured(...)

	s.logger.Printf("Payment captured: %s", payment.ID)

	return payment, nil
}

// CancelAuthorization orchestrates the cancellation of an authorization.
func (s *PaymentService) CancelAuthorization(ctx context.Context, paymentID string) (*domain.Payment, error) {
	// Retrieve the payment
	payment, err := s.paymentRepo.Get(paymentID)
	if err != nil {
		return nil, err
	}

	// Cancel via provider
	_ = s.provider.CancelAuthorization(paymentID)

	// Update payment status
	payment.Status = domain.PaymentStatusCancelled
	if err := s.paymentRepo.Update(payment); err != nil {
		return nil, fmt.Errorf("failed to update payment: %w", err)
	}

	// Update state
	s.state.MarkCancelled(payment)

	// TODO: Publish PaymentCancelled event
	// s.outbox.PublishPaymentCancelled(...)

	s.logger.Printf("Authorization cancelled: %s", payment.ID)

	return payment, nil
}

// CreateRefund orchestrates the creation of a refund.
func (s *PaymentService) CreateRefund(ctx context.Context, paymentID, reason string, amount int64) (*domain.Refund, error) {
	// Retrieve the payment
	payment, err := s.paymentRepo.Get(paymentID)
	if err != nil {
		return nil, err
	}

	// Check if payment can be refunded
	if !payment.CanRefund() {
		return nil, domain.ErrPaymentNotRefundable
	}

	// Create the refund
	refund := domain.NewRefund(paymentID, amount, payment.Currency, reason)

	// Save refund in repository
	if err := s.refundRepo.Create(refund); err != nil {
		return nil, fmt.Errorf("failed to create refund: %w", err)
	}

	// Update payment status
	if err := payment.MarkRefunded(); err != nil {
		return nil, err
	}
	if err := s.paymentRepo.Update(payment); err != nil {
		return nil, fmt.Errorf("failed to update payment: %w", err)
	}

	// Update state
	s.state.MarkRefunded(payment)

	// TODO: Publish PaymentRefunded event
	// s.outbox.PublishPaymentRefunded(...)

	s.logger.Printf("Refund created: %s", refund.ID)

	return refund, nil
}

// GetPayment retrieves a payment by ID.
func (s *PaymentService) GetPayment(ctx context.Context, paymentID string) (*domain.Payment, error) {
	return s.paymentRepo.Get(paymentID)
}

// GetPaymentStatus retrieves the status of a payment.
func (s *PaymentService) GetPaymentStatus(ctx context.Context, paymentID string) (string, error) {
	payment, err := s.paymentRepo.Get(paymentID)
	if err != nil {
		return "", err
	}
	return string(payment.Status), nil
}

// generateIdempotencyKey generates a unique idempotency key.
func generateIdempotencyKey(deliveryID, userID string, amount int64, currency string) string {
	// Simple key generation - in production use snowflake or hash
	return fmt.Sprintf("idem_%s_%s_%d_%s", deliveryID, userID, amount, currency)
}