package ports

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
	ProviderPaymentID     string
	Status                string
	CheckoutURL           string
	ClientSecret          string
}

type NormalizedError struct {
	Category  ErrorCategory
	Message   string
	Retryable bool
}

func (e *NormalizedError) Error() string {
	return string(e.Category) + ": " + e.Message
}

type AuthorizeRequest struct {
	PaymentID      string
	AmountMinor    int64
	Currency       string
	IdempotencyKey string
	Description    string
	SuccessURL     string
	CancelURL      string
}

type CaptureRequest struct {
	ProviderPaymentID string
	AmountMinor       int64
	IdempotencyKey    string
}

type VoidRequest struct {
	ProviderPaymentID string
	IdempotencyKey    string
}

type RefundRequest struct {
	ProviderPaymentID string
	AmountMinor       int64
	Reason            string
	IdempotencyKey    string
}

type PaymentProvider interface {
	Authorize(ctx context.Context, req AuthorizeRequest) (*ProviderResult, *NormalizedError)
	Capture(ctx context.Context, req CaptureRequest) (*ProviderResult, *NormalizedError)
	Void(ctx context.Context, req VoidRequest) (*ProviderResult, *NormalizedError)
	Refund(ctx context.Context, req RefundRequest) (*ProviderResult, *NormalizedError)
	GetStatus(ctx context.Context, providerPaymentID string) (*ProviderResult, *NormalizedError)
}
