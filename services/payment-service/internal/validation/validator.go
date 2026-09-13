package validation

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Common validation errors.
var (
	ErrInvalidAmount       = errors.New("invalid amount: must be positive")
	ErrInvalidCurrency     = errors.New("invalid currency code")
	ErrInvalidPaymentID    = errors.New("invalid payment ID format")
	ErrInvalidDeliveryID   = errors.New("invalid delivery ID format")
	ErrInvalidUserID       = errors.New("invalid user ID format")
	ErrInvalidCorrelationID = errors.New("invalid correlation ID")
	ErrInvalidCausationID  = errors.New("invalid causation ID")
	ErrInvalidStatus       = errors.New("invalid status")
	ErrInvalidReason       = errors.New("invalid reason: must not be empty")
	ErrInvalidRefundAmount = errors.New("invalid refund amount")
	ErrInvalidPaymentType  = errors.New("invalid payment type")
	ErrEmptyInput          = errors.New("input cannot be empty")
	ErrInvalidWebhookType  = errors.New("invalid webhook event type")
	ErrInvalidProvider     = errors.New("invalid provider name")
)

// Valid currency codes (ISO 4217)
var validCurrencies = map[string]bool{
	"USD": true, "EUR": true, "GBP": true, "JPY": true,
	"CAD": true, "AUD": true, "CHF": true, "CNY": true,
	"INR": true, "BRL": true, "MXN": true, "SGD": true,
	"HKD": true, "NZD": true, "SEK": true, "NOK": true,
	"DKK": true, "PLN": true, "CZK": true, "HUF": true,
}

// Valid payment statuses
var validStatuses = map[string]bool{
	"pending":   true,
	"authorized": true,
	"captureable": true,
	"captured":  true,
	"cancelled": true,
	"failed":    true,
	"refunded":  true,
	"refund_pending": true,
}

// Valid payment methods
var validMethods = map[string]bool{
	"stripe": true,
	"cash":   true,
	"card":   true,
}

// Valid payment types
var validPaymentTypes = map[string]bool{
	"payment": true,
	"refund":  true,
	"webhook": true,
}

// Valid webhook event types
var validWebhookTypes = map[string]bool{
	"payment.created":                true,
	"payment.authorization.started":  true,
	"payment.authorized":             true,
	"payment.authorization.failed":   true,
	"payment.capture.started":        true,
	"payment.captured":               true,
	"payment.capture.failed":         true,
	"payment.cancelled":              true,
	"payment.refund.started":         true,
	"payment.refunded":               true,
	"payment.refund.failed":          true,
	"payment.failed":                 true,
}

// Valid providers
var validProviders = map[string]bool{
	"stripe": true,
	"mock":   true,
}

// ValidateAmount validates that the amount is positive.
func ValidateAmount(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	return nil
}

// ValidateCurrency validates the currency code.
func ValidateCurrency(currency string) error {
	if currency == "" {
		return ErrInvalidCurrency
	}
	currency = strings.ToUpper(currency)
	if !validCurrencies[currency] {
		return ErrInvalidCurrency
	}
	return nil
}

// ValidatePaymentID validates the payment ID format.
func ValidatePaymentID(paymentID string) error {
	if paymentID == "" {
		return ErrInvalidPaymentID
	}
	// Expected format: pay_<alphanumeric>
	if len(paymentID) < 4 || !strings.HasPrefix(paymentID, "pay_") {
		return ErrInvalidPaymentID
	}
	return nil
}

// ValidateDeliveryID validates the delivery ID format.
func ValidateDeliveryID(deliveryID string) error {
	if deliveryID == "" {
		return ErrInvalidDeliveryID
	}
	// Expected format: del_<alphanumeric>
	if len(deliveryID) < 4 || !strings.HasPrefix(deliveryID, "del_") {
		return ErrInvalidDeliveryID
	}
	return nil
}

// ValidateUserID validates the user ID format.
func ValidateUserID(userID string) error {
	if userID == "" {
		return ErrInvalidUserID
	}
	// Expected format: usr_<alphanumeric>
	if len(userID) < 4 || !strings.HasPrefix(userID, "usr_") {
		return ErrInvalidUserID
	}
	return nil
}

// ValidateCorrelationID validates the correlation ID.
func ValidateCorrelationID(correlationID string) error {
	if correlationID == "" {
		return ErrInvalidCorrelationID
	}
	// UUID format or custom format
	if len(correlationID) < 8 {
		return ErrInvalidCorrelationID
	}
	return nil
}

// ValidateCausationID validates the causation ID.
func ValidateCausationID(causationID string) error {
	if causationID == "" {
		return ErrInvalidCausationID
	}
	if len(causationID) < 8 {
		return ErrInvalidCausationID
	}
	return nil
}

// ValidateStatus validates the payment status.
func ValidateStatus(status string) error {
	if status == "" {
		return ErrInvalidStatus
	}
	if !validStatuses[status] {
		return ErrInvalidStatus
	}
	return nil
}

// ValidateMethod validates the payment method.
func ValidateMethod(method string) error {
	if method == "" {
		return ErrInvalidPaymentType
	}
	if !validMethods[method] {
		return ErrInvalidPaymentType
	}
	return nil
}

// ValidateReason validates the refund reason.
func ValidateReason(reason string) error {
	if strings.TrimSpace(reason) == "" {
		return ErrInvalidReason
	}
	if len(reason) > 500 {
		return errors.New("reason too long: maximum 500 characters")
	}
	return nil
}

// ValidateRefundAmount validates the refund amount against the original amount.
func ValidateRefundAmount(refundAmount, originalAmount int64) error {
	if refundAmount <= 0 {
		return ErrInvalidRefundAmount
	}
	if refundAmount > originalAmount {
		return errors.New("refund amount cannot exceed original amount")
	}
	return nil
}

// ValidateWebhookEventType validates the webhook event type.
func ValidateWebhookEventType(eventType string) error {
	if eventType == "" {
		return ErrInvalidWebhookType
	}
	if !validWebhookTypes[eventType] {
		return ErrInvalidWebhookType
	}
	return nil
}

// ValidateProvider validates the provider name.
func ValidateProvider(provider string) error {
	if provider == "" {
		return ErrInvalidProvider
	}
	if !validProviders[provider] {
		return ErrInvalidProvider
	}
	return nil
}

// ValidatePaymentInput validates the payment creation input.
type PaymentInput struct {
	DeliveryID    string
	UserID        string
	AmountMinor   int64
	Currency      string
	CorrelationID string
	CausationID   string
}

// ValidatePaymentInput validates all payment input fields.
func ValidatePaymentInput(input PaymentInput) error {
	if err := ValidateDeliveryID(input.DeliveryID); err != nil {
		return err
	}
	if err := ValidateUserID(input.UserID); err != nil {
		return err
	}
	if err := ValidateAmount(input.AmountMinor); err != nil {
		return err
	}
	if err := ValidateCurrency(input.Currency); err != nil {
		return err
	}
	if err := ValidateCorrelationID(input.CorrelationID); err != nil {
		return err
	}
	if err := ValidateCausationID(input.CausationID); err != nil {
		return err
	}
	return nil
}

// ValidateAuthorizationInput validates the authorization input.
type AuthorizationInput struct {
	PaymentID     string
	CorrelationID string
	CausationID   string
}

// ValidateAuthorizationInput validates the authorization input.
func ValidateAuthorizationInput(input AuthorizationInput) error {
	if err := ValidatePaymentID(input.PaymentID); err != nil {
		return err
	}
	if err := ValidateCorrelationID(input.CorrelationID); err != nil {
		return err
	}
	if err := ValidateCausationID(input.CausationID); err != nil {
		return err
	}
	return nil
}

// ValidateCaptureInput validates the capture input.
type CaptureInput struct {
	PaymentID     string
	AmountMinor   int64
	CorrelationID string
	CausationID   string
}

// ValidateCaptureInput validates the capture input.
func ValidateCaptureInput(input CaptureInput) error {
	if err := ValidatePaymentID(input.PaymentID); err != nil {
		return err
	}
	if err := ValidateAmount(input.AmountMinor); err != nil {
		return err
	}
	if err := ValidateCorrelationID(input.CorrelationID); err != nil {
		return err
	}
	if err := ValidateCausationID(input.CausationID); err != nil {
		return err
	}
	return nil
}

// ValidateCancelInput validates the cancel input.
type CancelInput struct {
	PaymentID     string
	CorrelationID string
	CausationID   string
}

// ValidateCancelInput validates the cancel input.
func ValidateCancelInput(input CancelInput) error {
	if err := ValidatePaymentID(input.PaymentID); err != nil {
		return err
	}
	if err := ValidateCorrelationID(input.CorrelationID); err != nil {
		return err
	}
	if err := ValidateCausationID(input.CausationID); err != nil {
		return err
	}
	return nil
}

// ValidateRefundInput validates the refund input.
type RefundInput struct {
	PaymentID     string
	AmountMinor   int64
	Reason        string
	CorrelationID string
	CausationID   string
}

// ValidateRefundInput validates the refund input.
func ValidateRefundInput(input RefundInput) error {
	if err := ValidatePaymentID(input.PaymentID); err != nil {
		return err
	}
	if err := ValidateAmount(input.AmountMinor); err != nil {
		return err
	}
	if err := ValidateReason(input.Reason); err != nil {
		return err
	}
	if err := ValidateCorrelationID(input.CorrelationID); err != nil {
		return err
	}
	if err := ValidateCausationID(input.CausationID); err != nil {
		return err
	}
	return nil
}

// ValidateRefundAmountAgainstOriginal validates refund amount against original.
func ValidateRefundAmountAgainstOriginal(refundAmount, originalAmount int64) error {
	if refundAmount <= 0 {
		return ErrInvalidRefundAmount
	}
	if refundAmount > originalAmount {
		return errors.New("refund amount cannot exceed original amount")
	}
	return nil
}

// ValidateISO8601Timestamp validates an ISO 8601 timestamp.
func ValidateISO8601Timestamp(timestamp string) error {
	if timestamp == "" {
		return errors.New("timestamp cannot be empty")
	}
	_, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return errors.New("invalid timestamp format: must be RFC3339")
	}
	return nil
}

// SanitizeString sanitizes a string for safe use.
func SanitizeString(input string) string {
	// Remove control characters
	re := regexp.MustCompile(`[\x00-\x1f\x7f]`)
	return re.ReplaceAllString(input, "")
}

// ValidateAmountString validates an amount string.
func ValidateAmountString(amountStr string) (int64, error) {
	amount, err := strconv.ParseInt(strings.TrimSpace(amountStr), 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	if err := ValidateAmount(amount); err != nil {
		return 0, err
	}
	return amount, nil
}

// ValidateDateRange validates a date range.
func ValidateDateRange(start, end string) error {
	if start == "" || end == "" {
		return errors.New("date range cannot be empty")
	}
	startTime, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return errors.New("invalid start date format")
	}
	endTime, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return errors.New("invalid end date format")
	}
	if startTime.After(endTime) {
		return errors.New("start date must be before end date")
	}
	return nil
}

// IsValidUUID validates a UUID string.
func IsValidUUID(uuid string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(strings.ToLower(uuid))
}

// ValidateID validates a generic ID string.
func ValidateID(id, prefix string) error {
	if id == "" {
		return errors.New("ID cannot be empty")
	}
	expectedPrefix := prefix + "_"
	if !strings.HasPrefix(id, expectedPrefix) {
		return errors.New("invalid ID format: expected " + expectedPrefix + "prefix")
	}
	return nil
}