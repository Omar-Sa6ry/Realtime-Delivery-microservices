package events

import "time"

type PaymentEventType string

const (
	PaymentCreated                PaymentEventType = "payment.created"
	PaymentAuthorizationStarted   PaymentEventType = "payment.authorization.started"
	PaymentAuthorized             PaymentEventType = "payment.authorized"
	PaymentAuthorizationFailed    PaymentEventType = "payment.authorization.failed"
	PaymentCaptureStarted         PaymentEventType = "payment.capture.started"
	PaymentCaptured               PaymentEventType = "payment.captured"
	PaymentCompleted              PaymentEventType = "payment.completed"
	PaymentCaptureFailed          PaymentEventType = "payment.capture.failed"
	PaymentCancelled              PaymentEventType = "payment.cancelled"
	PaymentRefundStarted          PaymentEventType = "payment.refund.started"
	PaymentRefunded               PaymentEventType = "payment.refunded"
	PaymentRefundFailed           PaymentEventType = "payment.refund.failed"
	PaymentFailed                 PaymentEventType = "payment.failed"
)

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

func NewPaymentEventEnvelope(eventID string, eventType PaymentEventType, traceID string, payload interface{}) (*EventEnvelope, error) {
	return NewEventEnvelope(eventID, string(eventType), traceID, payload)
}

func MarshalPaymentEnvelope(eventID string, eventType PaymentEventType, traceID string, payload interface{}) ([]byte, error) {
	return MarshalEnvelope(eventID, string(eventType), traceID, payload)
}