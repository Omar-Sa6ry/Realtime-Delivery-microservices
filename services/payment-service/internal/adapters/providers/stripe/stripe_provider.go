package stripe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/providers"
	stripego "github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/paymentintent"
	"github.com/stripe/stripe-go/v78/refund"
)

type StripeProvider struct {
	secretKey string
}

func NewStripeProvider(secretKey string) *StripeProvider {
	if secretKey == "" {
		slog.Warn("stripe_provider: STRIPE_SECRET_KEY is empty — Stripe calls will fail")
	}
	return &StripeProvider{secretKey: secretKey}
}

func (p *StripeProvider) Authorize(ctx context.Context, req providers.AuthorizeRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	stripego.Key = p.secretKey

	params := &stripego.PaymentIntentParams{
		Amount:   stripego.Int64(req.AmountMinor),
		Currency: stripego.String(strings.ToLower(req.Currency)),
		CaptureMethod:    stripego.String(string(stripego.PaymentIntentCaptureMethodManual)),
		Description:      stripego.String(req.Description),
		PaymentMethodTypes: stripego.StringSlice([]string{"card"}),
		Metadata: map[string]string{
			"payment_id":      req.PaymentID,
			"idempotency_key": req.IdempotencyKey,
		},
	}

	if req.IdempotencyKey != "" {
		params.IdempotencyKey = stripego.String("auth-" + req.IdempotencyKey)
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return nil, normalizeStripeError(err, "Authorize")
	}

	result := &providers.ProviderResult{
		ProviderTransactionID: pi.ID,
		ProviderPaymentID:     pi.ID,
		Status:                normalizePaymentIntentStatus(pi.Status),
		ClientSecret:          pi.ClientSecret,
	}

	if pi.Status == stripego.PaymentIntentStatusRequiresAction &&
		pi.NextAction != nil && pi.NextAction.RedirectToURL != nil {
		result.CheckoutURL = pi.NextAction.RedirectToURL.URL
	}

	slog.Debug("stripe: Authorize completed",
		"paymentID", req.PaymentID,
		"piID", pi.ID,
		"status", pi.Status,
	)
	return result, nil
}

func (p *StripeProvider) Capture(ctx context.Context, req providers.CaptureRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	stripego.Key = p.secretKey

	params := &stripego.PaymentIntentCaptureParams{
		AmountToCapture: stripego.Int64(req.AmountMinor),
	}
	if req.IdempotencyKey != "" {
		params.IdempotencyKey = stripego.String("cap-" + req.IdempotencyKey)
	}

	pi, err := paymentintent.Capture(req.ProviderPaymentID, params)
	if err != nil {
		return nil, normalizeStripeError(err, "Capture")
	}

	slog.Debug("stripe: Capture completed", "piID", req.ProviderPaymentID, "status", pi.Status)
	return &providers.ProviderResult{
		ProviderTransactionID: pi.ID,
		ProviderPaymentID:     pi.ID,
		Status:                normalizePaymentIntentStatus(pi.Status),
	}, nil
}

func (p *StripeProvider) Void(ctx context.Context, req providers.VoidRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	stripego.Key = p.secretKey

	params := &stripego.PaymentIntentCancelParams{}
	if req.IdempotencyKey != "" {
		params.IdempotencyKey = stripego.String("void-" + req.IdempotencyKey)
	}

	pi, err := paymentintent.Cancel(req.ProviderPaymentID, params)
	if err != nil {
		return nil, normalizeStripeError(err, "Void")
	}

	slog.Debug("stripe: Void completed", "piID", req.ProviderPaymentID, "status", pi.Status)
	return &providers.ProviderResult{
		ProviderTransactionID: pi.ID,
		ProviderPaymentID:     pi.ID,
		Status:                normalizePaymentIntentStatus(pi.Status),
	}, nil
}

func (p *StripeProvider) Refund(ctx context.Context, req providers.RefundRequest) (*providers.ProviderResult, *providers.NormalizedError) {
	stripego.Key = p.secretKey

	params := &stripego.RefundParams{
		PaymentIntent: stripego.String(req.ProviderPaymentID),
		Amount:        stripego.Int64(req.AmountMinor),
		Reason:        stripego.String(refundReasonToStripe(req.Reason)),
		Metadata: map[string]string{
			"refund_reason": req.Reason,
		},
	}
	if req.IdempotencyKey != "" {
		params.IdempotencyKey = stripego.String("ref-" + req.IdempotencyKey)
	}

	rf, err := refund.New(params)
	if err != nil {
		return nil, normalizeStripeError(err, "Refund")
	}

	piID := ""
	if rf.PaymentIntent != nil {
		piID = rf.PaymentIntent.ID
	}

	slog.Debug("stripe: Refund completed", "refundID", rf.ID, "piID", piID)
	return &providers.ProviderResult{
		ProviderTransactionID: rf.ID,
		ProviderPaymentID:     piID,
		Status:                normalizeRefundStatus(rf.Status),
	}, nil
}

func (p *StripeProvider) GetStatus(ctx context.Context, providerPaymentID string) (*providers.ProviderResult, *providers.NormalizedError) {
	stripego.Key = p.secretKey

	pi, err := paymentintent.Get(providerPaymentID, nil)
	if err != nil {
		return nil, normalizeStripeError(err, "GetStatus")
	}

	return &providers.ProviderResult{
		ProviderTransactionID: pi.ID,
		ProviderPaymentID:     pi.ID,
		Status:                normalizePaymentIntentStatus(pi.Status),
		ClientSecret:          pi.ClientSecret,
	}, nil
}

func normalizePaymentIntentStatus(status stripego.PaymentIntentStatus) string {
	switch status {
	case stripego.PaymentIntentStatusRequiresPaymentMethod,
		stripego.PaymentIntentStatusRequiresConfirmation,
		stripego.PaymentIntentStatusRequiresAction,
		stripego.PaymentIntentStatusProcessing:
		return "PROCESSING"
	case stripego.PaymentIntentStatusRequiresCapture:
		return "AUTHORIZED"
	case stripego.PaymentIntentStatusSucceeded:
		return "SUCCEEDED"
	case stripego.PaymentIntentStatusCanceled:
		return "CANCELLED"
	default:
		return "UNKNOWN"
	}
}

func normalizeRefundStatus(status stripego.RefundStatus) string {
	switch status {
	case stripego.RefundStatusSucceeded:
		return "SUCCEEDED"
	case stripego.RefundStatusPending:
		return "PROCESSING"
	case stripego.RefundStatusFailed, stripego.RefundStatusCanceled:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

func refundReasonToStripe(reason string) string {
	switch strings.ToLower(reason) {
	case "duplicate":
		return "duplicate"
	case "fraudulent", "fraud":
		return "fraudulent"
	case "requested_by_customer", "customer_request":
		return "requested_by_customer"
	default:
		return "requested_by_customer"
	}
}

func normalizeStripeError(err error, operation string) *providers.NormalizedError {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return &providers.NormalizedError{
			Category:  providers.ErrCategoryTimeout,
			Message:   fmt.Sprintf("stripe %s timed out — outcome unknown", operation),
			Retryable: false, // don't retry; let reconciliation handle it
		}
	}

	stripeErr, ok := err.(*stripego.Error)
	if !ok {
		return &providers.NormalizedError{
			Category:  providers.ErrCategoryUnknown,
			Message:   fmt.Sprintf("stripe %s: non-stripe error", operation),
			Retryable: false,
		}
	}

	var category providers.ErrorCategory
	var retryable bool

	switch string(stripeErr.Type) {
	case string(stripego.ErrorTypeCard):
		category = providers.ErrCategoryDeclined
		retryable = false
	case "rate_limit_error":
		category = providers.ErrCategoryRateLimited
		retryable = true
	case string(stripego.ErrorTypeInvalidRequest):
		category = providers.ErrCategoryPermanent
		retryable = false
	case "authentication_error":
		category = providers.ErrCategoryAuthError
		retryable = false
	case string(stripego.ErrorTypeAPI), "api_connection_error":
		category = providers.ErrCategoryTemporary
		retryable = true
	default:
		// Specific codes for card errors
		switch stripeErr.Code {
		case stripego.ErrorCodeCardDeclined,
			stripego.ErrorCodeExpiredCard,
			stripego.ErrorCodeIncorrectCVC,
			stripego.ErrorCodeIncorrectNumber,
			stripego.ErrorCodeIncorrectZip,
			stripego.ErrorCodeInsufficientFunds:
			category = providers.ErrCategoryDeclined
			retryable = false
		default:
			category = providers.ErrCategoryUnknown
			retryable = false
		}
	}

	safeMsg := fmt.Sprintf("stripe %s error: type=%s code=%s", operation, stripeErr.Type, stripeErr.Code)

	return &providers.NormalizedError{
		Category:  category,
		Message:   safeMsg,
		Retryable: retryable,
	}
}
