package events

import "time"

// PaymentCompletedPayload is emitted when a payment is completed.
type PaymentCompletedPayload struct {
	PaymentID  string    `json:"paymentId"`
	DeliveryID string    `json:"deliveryId"`
	CustomerID string    `json:"customerId"`
	Amount     float64   `json:"amount"`
	Currency   string    `json:"currency"`
	CompletedAt time.Time `json:"completedAt"`
}

// PaymentFailedPayload is emitted when a payment fails.
type PaymentFailedPayload struct {
	PaymentID  string    `json:"paymentId,omitempty"`
	DeliveryID string    `json:"deliveryId"`
	CustomerID string    `json:"customerId"`
	Amount     float64   `json:"amount"`
	Currency   string    `json:"currency"`
	Reason     string    `json:"reason"`
	FailedAt   time.Time `json:"failedAt"`
}

// PaymentRefundedPayload is emitted when a payment is refunded.
type PaymentRefundedPayload struct {
	PaymentID  string    `json:"paymentId"`
	DeliveryID string    `json:"deliveryId"`
	Amount     float64   `json:"amount"`
	Currency   string    `json:"currency"`
	Reason     string    `json:"reason"`
	RefundedAt time.Time `json:"refundedAt"`
}
