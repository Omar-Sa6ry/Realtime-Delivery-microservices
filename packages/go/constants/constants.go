package constants

// Delivery Status Constants (State Machine)
const (
	DeliveryStatusPending          = "PENDING"
	DeliveryStatusSearchingDriver  = "SEARCHING_DRIVER"
	DeliveryStatusDriverAssigned   = "DRIVER_ASSIGNED"
	DeliveryStatusDriverAccepted   = "DRIVER_ACCEPTED"
	DeliveryStatusPickupStarted    = "PICKUP_STARTED"
	DeliveryStatusPickedUp         = "PICKED_UP"
	DeliveryStatusInTransit        = "IN_TRANSIT"
	DeliveryStatusDelivered        = "DELIVERED"
	DeliveryStatusCancelled        = "CANCELLED"
	DeliveryStatusFailed           = "FAILED"
)

// Header Keys
const (
	HeaderXUserId        = "x-user-id"
	HeaderXUserRole      = "x-user-role"
	HeaderXUserSession   = "x-user-session"
	HeaderXCorrelationId = "x-correlation-id"
)

// Payment Status Constants
const (
	PaymentStatusPending   = "PENDING"
	PaymentStatusCompleted = "COMPLETED"
	PaymentStatusFailed    = "FAILED"
	PaymentStatusRefunded  = "REFUNDED"
)
