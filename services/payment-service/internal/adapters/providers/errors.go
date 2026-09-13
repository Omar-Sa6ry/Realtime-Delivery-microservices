package providers

// ErrorCategory represents the category of a payment provider error.
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

// ProviderResult represents the result of a provider operation.
type ProviderResult struct {
	ProviderTransactionID string
	ProviderPaymentID     string
	Status                string // OperationStatus from domain
	CheckoutURL           string // Stripe Checkout URL if applicable
	ClientSecret          string // Stripe PaymentIntent client_secret
}

// NormalizedError represents a normalized provider error.
type NormalizedError struct {
	Category  ErrorCategory
	Message   string
	Retryable bool
}

// AuthorizeRequest represents a request to authorize a payment.
type AuthorizeRequest struct {
	PaymentID         string
	AmountMinor       int64
	Currency          string
	IdempotencyKey    string
	Description       string
	SuccessURL        string
	CancelURL         string
}

// CaptureRequest represents a request to capture a payment.
type CaptureRequest struct {
	ProviderPaymentID string
	AmountMinor       int64
	IdempotencyKey    string
}

// VoidRequest represents a request to void/cancel an authorization.
type VoidRequest struct {
	ProviderPaymentID string
	IdempotencyKey    string
}

// RefundRequest represents a request to refund a payment.
type RefundRequest struct {
	ProviderPaymentID string
	AmountMinor       int64
	Reason            string
	IdempotencyKey    string
}

// PaymentProvider defines the interface for payment providers.
type PaymentProvider interface {
	Authorize(ctx context.Context, req AuthorizeRequest) (*ProviderResult, *NormalizedError)
	Capture(ctx context.Context, req CaptureRequest) (*ProviderResult, *NormalizedError)
	Void(ctx context.Context, req VoidRequest) (*ProviderResult, *NormalizedError)
	Refund(ctx context.Context, req RefundRequest) (*ProviderResult, *NormalizedError)
	GetStatus(ctx context.Context, providerPaymentID string) (*ProviderResult, *NormalizedError)
}