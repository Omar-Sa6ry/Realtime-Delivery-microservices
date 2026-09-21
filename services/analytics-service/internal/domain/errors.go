package domain

import "errors"

var (
	// Envelope / identity errors.
	ErrInvalidEnvelope         = errors.New("invalid event envelope")
	ErrMissingEventID          = errors.New("eventId is required")
	ErrMissingEventType        = errors.New("eventType is required")
	ErrMissingAggregateID      = errors.New("aggregateId is required")
	ErrUnknownEventType        = errors.New("unknown event type")
	ErrUnsupportedEventVersion = errors.New("unsupported event version")

	// Time errors.
	ErrInvalidOccurredAt = errors.New("occurredAt is invalid")
	ErrEventFromFuture   = errors.New("occurredAt is beyond the future tolerance")

	// Idempotency errors.
	ErrDuplicateEvent = errors.New("duplicate event identity")

	// Fact validation errors.
	ErrMissingDeliveryID     = errors.New("deliveryId is required")
	ErrMissingDriverID       = errors.New("driverId is required")
	ErrMissingAssignmentID   = errors.New("assignmentId is required")
	ErrMissingPaymentID      = errors.New("paymentId is required")
	ErrMissingNotificationID = errors.New("notificationId is required")
	ErrNegativeDuration      = errors.New("negative duration")
	ErrOutOfOrderTimestamps  = errors.New("timestamps out of order")
	ErrNegativeAmount        = errors.New("negative payment amount")
	ErrInvalidAmountFormat   = errors.New("invalid decimal amount format")
	ErrRefundExceedsCapture  = errors.New("refund amount exceeds captured amount")
	ErrInvalidRetryCount     = errors.New("retryCount must not be negative")

	// Data quality errors.
	ErrInvalidIssueSeverity = errors.New("invalid data quality severity")
	ErrMissingIssueType     = errors.New("issueType is required")
	ErrMissingIssueID       = errors.New("issueId is required")

	// State errors.
	ErrInvalidCheckpoint = errors.New("invalid ingestion checkpoint")
)
