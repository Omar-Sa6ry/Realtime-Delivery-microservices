package stripe

import (
	"context"
	"fmt"
	"strings"

	"github.com/realtime-delivery/payment-service/internal/adapters/providers"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/paymentintent"
	"github.com/stripe/stripe-go/v78/refund"
)

// StripeProvider implements the PaymentProvider interface using Stripe.
type StripeProvider struct {
	secretKey string
}

// NewStripeProvider creates a new StripeProvider.
func NewStripeProvider(secretKey string) *StripeProvider {
	return &StripeProvider{
		secretKey: secretKey,
	}
}

func (p *StripeProvider) getStripeKey() string {
	return p.secretKey
}

// Authorize authorizes a payment using Stripe PaymentIntent.
func (p *StripeProvider) Authorize(ctx context.Context, req providers.AuthorizeRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	stripe.Key = p.getStripeKey()

	params := &stripe.PaymentIntentParams{
		Amount:          stripe.Int64(req.AmountMinor),
		Currency:        stripe.String(req.Currency),
		Description:     stripe.String(req.Description),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		ReceiptEmail:    stripe.String(""), // optional
		Metadata: map[string]string{
			"payment_id":  req.PaymentID,
			"idempotency_key": req.IdempotencyKey,
		},
	}

	// Set idempotency key if provided
	if req.IdempotencyKey != "" {
		params.IdempotencyKey = stripe.String(req.IdempotencyKey)
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return nil, normalizeStripeError(err)
	}

	result := &providers.ProviderResult{
		ProviderTransactionID: pi.ID,
		ProviderPaymentID:     pi.ID,
		Status:                normalizeStripeStatus(pi.Status),
		ClientSecret:          pi.ClientSecret,
	}

	if pi.Status == stripe.PaymentIntentStatusRequiresAction {
		// For 3D Secure, return the next action URL
		if pi.NextAction != nil && pi.NextAction.RedirectToURL != nil {
			result.CheckoutURL = pi.NextAction.RedirectToURL.URL
		}
	}

	return result, nil
}

// Capture captures an authorized payment.
func (p *StripeProvider) Capture(ctx context.Context, req providers.CaptureRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	stripe.Key = p.getStripeKey()

	params := &stripe.PaymentIntentCaptureParams{
		AmountToCapture: stripe.Int64(req.AmountMinor),
	}

	if req.IdempotencyKey != "" {
		params.IdempotencyKey = stripe.String(req.IdempotencyKey)
	}

	pi, err := paymentintent.Capture(req.ProviderPaymentID, params)
	if err != nil {
		return nil, normalizeStripeError(err)
	}

	return &providers.ProviderResult{
		ProviderTransactionID: pi.ID,
		ProviderPaymentID:     pi.ID,
		Status:                normalizeStripeStatus(pi.Status),
	}, nil
}

// Void cancels an authorization.
func (p *StripeProvider) Void(ctx context.Context, req providers.VoidRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	stripe.Key = p.getStripeKey()

	params := &stripe.PaymentIntentCancelParams{}
	if req.IdempotencyKey != "" {
		params.IdempotencyKey = stripe.String(req.IdempotencyKey)
	}

	pi, err := paymentintent.Cancel(req.ProviderPaymentID, params)
	if err != nil {
		return nil, normalizeStripeError(err)
	}

	return &providers.ProviderResult{
		ProviderTransactionID: pi.ID,
		ProviderPaymentID:     pi.ID,
		Status:                normalizeStripeStatus(pi.Status),
	}, nil
}

// Refund refunds a captured payment.
func (p *StripeProvider) Refund(ctx context.Context, req providers.RefundRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	stripe.Key = p.getStripeKey()

	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(req.ProviderPaymentID),
		Amount:        stripe.Int64(req.AmountMinor),
		Reason:        stripe.String(refundReasonToStripe(req.Reason)),
		Metadata: map[string]string{
			"refund_reason": req.Reason,
		},
	}

	if req.IdempotencyKey != "" {
		params.IdempotencyKey = stripe.String(req.IdempotencyKey)
	}

	rf, err := refund.New(params)
	if err != nil {
		return nil, normalizeStripeError(err)
	}

	return &providers.ProviderResult{
		ProviderTransactionID: rf.ID,
		ProviderPaymentID:     rf.PaymentIntent.ID,
		Status:                normalizeStripeStatus(rf.Status),
	}, nil
}

// GetStatus retrieves the status of a payment from Stripe.
func (p *StripeProvider) GetStatus(ctx context.Context, providerPaymentID string) (*providers.ProviderResult, *providers.NormalizedError) {
	stripe.Key = p.getStripeKey()

	pi, err := paymentintent.Get(providerPaymentID, nil)
	if err != nil {
		return nil, normalizeStripeError(err)
	}

	return &providers.ProviderResult{
		ProviderTransactionID: pi.ID,
		ProviderPaymentID:     pi.ID,
		Status:                normalizeStripeStatus(pi.Status),
		ClientSecret:          pi.ClientSecret,
	}, nil
}

// normalizeStripeStatus converts Stripe status to domain OperationStatus.
func normalizeStripeStatus(status stripe.PaymentIntentStatus) string {
	switch status {
	case stripe.PaymentIntentStatusRequiresPaymentMethod,
		stripe.PaymentIntentStatusRequiresConfirmation,
		stripe.PaymentIntentStatusRequiresAction,
		stripe.PaymentIntentStatusProcessing:
		return "PROCESSING"
	case stripe.PaymentIntentStatusRequiresCapture:
		return "AUTHORIZED"
	case stripe.PaymentIntentStatusSucceeded:
		return "SUCCEEDED"
	case stripe.PaymentIntentStatusCanceled:
		return "CANCELLED"
	default:
		return "UNKNOWN"
	}
}

// refundReasonToStripe converts refund reason to Stripe refund reason.
func refundReasonToStripe(reason string) string {
	switch strings.ToLower(reason) {
	case "duplicate":
		return "duplicate"
	case "fraudulent":
		return "fraudulent"
	case "requested_by_customer":
		return "requested_by_customer"
	default:
		return "requested_by_customer"
	}
}

// normalizeStripeError converts a Stripe error to a normalized error.
func normalizeStripeError(err error) *providers.NormalizedError {
	if err == nil {
		return nil
	}

	stripeErr, ok := err.(*stripe.Error)
	if !ok {
		return &providers.NormalizedError{
			Category:  providers.ErrCategoryUnknown,
			Message:   err.Error(),
			Retryable: false,
		}
	}

	var category providers.ErrorCategory
	var retryable bool

	switch stripeErr.Code {
	case stripe.ErrorCodeCardDeclined:
		category = providers.ErrCategoryDeclined
		retryable = false
	case stripe.ErrorCodeRateLimitError:
		category = providers.ErrCategoryRateLimited
		retryable = true
	case stripe.ErrorCodeAPIConnectionError:
		category = providers.ErrCategoryTemporary
		retryable = true
	case stripe.ErrorCodeAPIError:
		category = providers.ErrCategoryTemporary
		retryable = true
	case stripe.ErrorCodeAuthenticationError:
		category = providers.ErrCategoryAuthError
		retryable = false
	case stripe.ErrorCodeInvalidRequestError:
		category = providers.ErrCategoryPermanent
		retryable = false
	case stripe.ErrorCodeCardExpired:
		category = providers.ErrCategoryDeclined
		retryable = false
	case stripe.ErrorCodeIncorrectNumber:
		category = providers.ErrCategoryDeclined
		retryable = false
	case stripe.ErrorCodeIncorrectCVC:
		category = providers.ErrCategoryDeclined
		retryable = false
	case stripe.ErrorCodeExpiredCard:
		category = providers.ErrCategoryDeclined
		retryable = false
	case stripe.ErrorCodeIncorrectZIP:
		category = providers.ErrCategoryDeclined
		retryable = false
	case stripe.ErrorCodeCardDeclined:
		category = providers.ErrCategoryDeclined
		retryable = false
	default:
		category = providers.ErrCategoryUnknown
		retryable = false
	}

	return &providers.NormalizedError{
		Category:  category,
		Message:   stripeErr.Msg,
		Retryable: retryable,
	}
}