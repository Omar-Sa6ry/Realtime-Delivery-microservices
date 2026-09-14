package providers

import "context"

type ErrorCategory string

const (
	ErrCategoryTemporary ErrorCategory = "TEMPORARY"
	ErrCategoryPermanent ErrorCategory = "PERMANENT"
	ErrCategoryDeclined ErrorCategory = "DECLINED"
	ErrCategoryAuthError ErrorCategory = "AUTHENTICATION_ERROR"
	ErrCategoryRateLimited ErrorCategory = "RATE_LIMITED"
	ErrCategoryTimeout ErrorCategory = "TIMEOUT"
	ErrCategoryUnknown ErrorCategory = "UNKNOWN"
)

type ProviderResult struct {
	ProviderTransactionID string
	ProviderPaymentID string
	Status string
	CheckoutURL string
	ClientSecret string
}

// NormalizedError represents a provider error with retry guidance.
type NormalizedError struct {
	Category  ErrorCategory
	// Message is a safe message (no PII, no raw provider error).
	Message   string
	Retryable bool
}

func (e *NormalizedError) Error() string {
	return string(e.Category) + ": " + e.Message
}

// AuthorizeRequest is the input for the Authorize operation.
type AuthorizeRequest struct {
	PaymentID      string
	AmountMinor    int64
	Currency       string
	IdempotencyKey string
	Description    string
	SuccessURL     string
	CancelURL      string
}

// CaptureRequest is the input for the Capture operation.
type CaptureRequest struct {
	ProviderPaymentID string
	AmountMinor       int64
	IdempotencyKey    string
}

// VoidRequest is the input for the Void/Cancel operation.
type VoidRequest struct {
	ProviderPaymentID string
	IdempotencyKey    string
}

// RefundRequest is the input for the Refund operation.
type RefundRequest struct {
	ProviderPaymentID string
	AmountMinor       int64
	Reason            string
	IdempotencyKey    string
}

// PaymentProvider is the interface all payment providers must implement.
// Implementations MUST:
//   - Use IdempotencyKey to prevent double-charges on retries.
//   - Return TIMEOUT category on context.DeadlineExceeded — callers must treat outcome as UNKNOWN.
//   - Never log raw error messages containing PII.
type PaymentProvider interface {
	Authorize(ctx context.Context, req AuthorizeRequest) (*ProviderResult, *NormalizedError)
	Capture(ctx context.Context, req CaptureRequest) (*ProviderResult, *NormalizedError)
	Void(ctx context.Context, req VoidRequest) (*ProviderResult, *NormalizedError)
	Refund(ctx context.Context, req RefundRequest) (*ProviderResult, *NormalizedError)
	// GetStatus queries the provider for the current state of a payment.
	// Used by the reconciliation worker to resolve UNKNOWN outcomes.
	GetStatus(ctx context.Context, providerPaymentID string) (*ProviderResult, *NormalizedError)
}
