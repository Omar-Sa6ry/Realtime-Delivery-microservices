package domain

type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "PENDING"
	PaymentStatusAuthorized PaymentStatus = "AUTHORIZED"
	PaymentStatusCaptured   PaymentStatus = "CAPTURED"
	PaymentStatusCancelled  PaymentStatus = "CANCELLED"
	PaymentStatusFailed     PaymentStatus = "FAILED"
	PaymentStatusRefunded   PaymentStatus = "REFUNDED"
)

type RefundStatus string

const (
	RefundStatusPending   RefundStatus = "REFUND_PENDING"
	RefundStatusCompleted RefundStatus = "REFUNDED"
	RefundStatusFailed    RefundStatus = "REFUND_FAILED"
)

type OperationStatus string

const (
	OperationStatusNotStarted OperationStatus = "NOT_STARTED"
	OperationStatusProcessing OperationStatus = "PROCESSING"
	OperationStatusSucceeded  OperationStatus = "SUCCEEDED"
	OperationStatusFailed     OperationStatus = "FAILED"
	OperationStatusUnknown OperationStatus = "UNKNOWN"
)

var AllowedTransitions = map[PaymentStatus][]PaymentStatus{
	PaymentStatusPending:    {PaymentStatusAuthorized, PaymentStatusCancelled, PaymentStatusFailed},
	PaymentStatusAuthorized: {PaymentStatusCaptured, PaymentStatusCancelled, PaymentStatusFailed},
	PaymentStatusCaptured:   {PaymentStatusRefunded},
	PaymentStatusCancelled:  {},
	PaymentStatusFailed:     {},
	PaymentStatusRefunded:   {},
}
