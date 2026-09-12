package events

import "time"

// PaymentEventType is the canonical event type for payment domain Kafka messages.
type PaymentEventType string

const (
	PaymentCreated                PaymentEventType = "payment.created"
	PaymentAuthorizationStarted   PaymentEventType = "payment.authorization.started"
	PaymentAuthorized             PaymentEventType = "payment.authorized"
	PaymentAuthorizationFailed    PaymentEventType = "payment.authorization.failed"
	PaymentCaptureStarted         PaymentEventType = "payment.capture.started"
	PaymentCaptured               PaymentEventType = "payment.captured"
	PaymentCaptureFailed          PaymentEventType = "payment.capture.failed"
	PaymentCancelled              PaymentEventType = "payment.cancelled"
	PaymentRefundStarted          PaymentEventType = "payment.refund.started"
	PaymentRefunded               PaymentEventType = "payment.refunded"
	PaymentRefundFailed           PaymentEventType = "payment.refund.failed"
	PaymentFailed                 PaymentEventType = "payment.failed"
)

// PaymentCreatedPayload is emitted when a new payment is created.
type PaymentCreatedPayload struct {
	PaymentID      string    `json:"paymentId"`
	DeliveryID     string    `json:"deliveryId"`
	UserID         string    `json:"userId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"`
	CorrelationID  string    `json:"correlationId"`
	CausationID    string    `json:"causationId"`
	CreatedAt      time.Time `json:"createdAt"`
}

// PaymentAuthorizationStartedPayload is emitted when payment authorization starts.
type PaymentAuthorizationStartedPayload struct {
	PaymentID      string    `json:"paymentId"`
	DeliveryID     string    `json:"deliveryId"`
	UserID         string    `json:"userId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	CorrelationID  string    `json:"correlationId"`
	CausationID    string    `json:"causationId"`
	StartedAt      time.Time `json:"startedAt"`
}

// PaymentAuthorizedPayload is emitted when payment authorization succeeds.
type PaymentAuthorizedPayload struct {
	PaymentID             string    `json:"paymentId"`
	DeliveryID            string    `json:"deliveryId"`
	UserID                string    `json:"userId"`
	AmountMinor           int64     `json:"amountMinor"`
	Currency              string    `json:"currency"`
	AuthorizedAmountMinor int64     `json:"authorizedAmountMinor"`
	ProviderPaymentID     string    `json:"providerPaymentId"`
	CorrelationID         string    `json:"correlationId"`
	CausationID           string    `json:"causationId"`
	AuthorizedAt          time.Time `json:"authorizedAt"`
}

// PaymentAuthorizationFailedPayload is emitted when payment authorization fails.
type PaymentAuthorizationFailedPayload struct {
	PaymentID      string    `json:"paymentId"`
	DeliveryID     string    `json:"deliveryId"`
	UserID         string    `json:"userId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	ErrorCode      string    `json:"errorCode"`
	ErrorCategory  string    `json:"errorCategory"`
	ErrorMessage   string    `json:"errorMessage"`
	CorrelationID  string    `json:"correlationId"`
	CausationID    string    `json:"causationId"`
	FailedAt       time.Time `json:"failedAt"`
}

// PaymentCaptureStartedPayload is emitted when payment capture starts.
type PaymentCaptureStartedPayload struct {
	PaymentID      string    `json:"paymentId"`
	DeliveryID     string    `json:"deliveryId"`
	UserID         string    `json:"userId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	CorrelationID  string    `json:"correlationId"`
	CausationID    string    `json:"causationId"`
	StartedAt      time.Time `json:"startedAt"`
}

// PaymentCapturedPayload is emitted when payment capture succeeds.
type PaymentCapturedPayload struct {
	PaymentID             string    `json:"paymentId"`
	DeliveryID            string    `json:"deliveryId"`
	UserID                string    `json:"userId"`
	AmountMinor           int64     `json:"amountMinor"`
	Currency              string    `json:"currency"`
	CapturedAmountMinor   int64     `json:"capturedAmountMinor"`
	ProviderTransactionID string    `json:"providerTransactionId"`
	CorrelationID         string    `json:"correlationId"`
	CausationID           string    `json:"causationId"`
	CapturedAt            time.Time `json:"capturedAt"`
}

// PaymentCaptureFailedPayload is emitted when payment capture fails.
type PaymentCaptureFailedPayload struct {
	PaymentID      string    `json:"paymentId"`
	DeliveryID     string    `json:"deliveryId"`
	UserID         string    `json:"userId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	ErrorCode      string    `json:"errorCode"`
	ErrorCategory  string    `json:"errorCategory"`
	ErrorMessage   string    `json:"errorMessage"`
	CorrelationID  string    `json:"correlationId"`
	CausationID    string    `json:"causationId"`
	FailedAt       time.Time `json:"failedAt"`
}

// PaymentCancelledPayload is emitted when a payment authorization is cancelled.
type PaymentCancelledPayload struct {
	PaymentID      string    `json:"paymentId"`
	DeliveryID     string    `json:"deliveryId"`
	UserID         string    `json:"userId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	CorrelationID  string    `json:"correlationId"`
	CausationID    string    `json:"causationId"`
	CancelledAt    time.Time `json:"cancelledAt"`
}

// PaymentRefundStartedPayload is emitted when a refund starts.
type PaymentRefundStartedPayload struct {
	PaymentID      string    `json:"paymentId"`
	DeliveryID     string    `json:"deliveryId"`
	UserID         string    `json:"userId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	Reason         string    `json:"reason"`
	CorrelationID  string    `json:"correlationId"`
	CausationID    string    `json:"causationId"`
	StartedAt      time.Time `json:"startedAt"`
}

// PaymentRefundedPayload is emitted when a refund succeeds.
type PaymentRefundedPayload struct {
	PaymentID            string    `json:"paymentId"`
	DeliveryID           string    `json:"deliveryId"`
	UserID               string    `json:"userId"`
	AmountMinor          int64     `json:"amountMinor"`
	Currency             string    `json:"currency"`
	RefundedAmountMinor  int64     `json:"refundedAmountMinor"`
	ProviderRefundID     string    `json:"providerRefundId"`
	CorrelationID        string    `json:"correlationId"`
	CausationID          string    `json:"causationId"`
	RefundedAt           time.Time `json:"refundedAt"`
}

// PaymentRefundFailedPayload is emitted when a refund fails.
type PaymentRefundFailedPayload struct {
	PaymentID      string    `json:"paymentId"`
	DeliveryID     string    `json:"deliveryId"`
	UserID         string    `json:"userId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	ErrorCode      string    `json:"errorCode"`
	ErrorCategory  string    `json:"errorCategory"`
	ErrorMessage   string    `json:"errorMessage"`
	CorrelationID  string    `json:"correlationId"`
	CausationID    string    `json:"causationId"`
	FailedAt       time.Time `json:"failedAt"`
}

// PaymentFailedPayload is emitted when a payment fails permanently.
type PaymentFailedPayload struct {
	PaymentID      string    `json:"paymentId"`
	DeliveryID     string    `json:"deliveryId"`
	UserID         string    `json:"userId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	ErrorCode      string    `json:"errorCode"`
	ErrorCategory  string    `json:"errorCategory"`
	ErrorMessage   string    `json:"errorMessage"`
	CorrelationID  string    `json:"correlationId"`
	CausationID    string    `json:"causationId"`
	FailedAt       time.Time `json:"failedAt"`
}

// NewPaymentEventEnvelope creates a new EventEnvelope for payment events.
func NewPaymentEventEnvelope(eventID string, eventType PaymentEventType, traceID string, payload interface{}) (*EventEnvelope, error) {
	return NewEventEnvelope(eventID, string(eventType), traceID, payload)
}

// MarshalPaymentEnvelope marshals a payment event into an EventEnvelope.
func MarshalPaymentEnvelope(eventID string, eventType PaymentEventType, traceID string, payload interface{}) ([]byte, error) {
	return MarshalEnvelope(eventID, string(eventType), traceID, payload)
}