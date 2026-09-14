package domain_test

import (
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

func TestPaymentStateMachine(t *testing.T) {
	// Create Payment
	p, err := domain.NewPayment("pay-1", "del-1", "usr-1", 5000, "USD", "corr-1", "caus-1")
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}

	if p.Status != domain.PaymentStatusPending {
		t.Fatalf("expected status PENDING, got %s", p.Status)
	}

	// Authorize
	if err := p.Authorize("pi_stripe_123", "secret_123", 5000); err != nil {
		t.Fatalf("unexpected error authorizing: %v", err)
	}

	if p.Status != domain.PaymentStatusAuthorized {
		t.Fatalf("expected status AUTHORIZED, got %s", p.Status)
	}

	// Capture
	if err := p.Capture(5000); err != nil {
		t.Fatalf("unexpected error capturing: %v", err)
	}

	if p.Status != domain.PaymentStatusCaptured {
		t.Fatalf("expected status CAPTURED, got %s", p.Status)
	}

	// Refund reservation & commit
	if !p.CanRefund(2000) {
		t.Fatalf("expected CanRefund(2000) to be true")
	}

	if err := p.ReserveRefund(2000); err != nil {
		t.Fatalf("unexpected error reserving refund: %v", err)
	}

	if err := p.CommitRefund(2000); err != nil {
		t.Fatalf("unexpected error committing refund: %v", err)
	}

	if p.RefundedAmountMinor != 2000 {
		t.Fatalf("expected RefundedAmountMinor=2000, got %d", p.RefundedAmountMinor)
	}
}
