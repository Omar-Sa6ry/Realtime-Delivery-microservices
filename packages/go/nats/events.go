package nats

// User NATS events subjects matching the TypeScript definitions
const (
	UserEventGetByID           = "user.get.by.id"
	UserEventGetByEmail        = "user.get.by.email"
	UserEventExists            = "user.exists"
	UserEventUpdated           = "user.updated"
	UserEventDataExisted       = "user.dataExists"
	UserEventCreateData        = "user.createData"
	UserEventRoleUpdated       = "user.role.updated"
	UserEventFindUsersWithIDs  = "user.findUsersWithIds"
	UserEventCheckIfInstructor = "user.checkIfInstructor"
)

// DeliveryKafkaTopics defines the Kafka topics emitted by the delivery domain.
const (
	DeliveryTopicCreated        = "delivery.created"
	DeliveryTopicDriverAssigned = "delivery.driver.assigned"
	DeliveryTopicDriverAccepted = "delivery.driver.accepted"
	DeliveryTopicPickedUp       = "delivery.picked_up"
	DeliveryTopicInTransit      = "delivery.in_transit"
	DeliveryTopicCompleted      = "delivery.completed"
	DeliveryTopicCancelled      = "delivery.cancelled"
)

// PaymentKafkaTopics defines the Kafka topics emitted by the payment domain.
const (
	PaymentTopicCompleted = "payment.completed"
	PaymentTopicFailed    = "payment.failed"
	PaymentTopicRefunded  = "payment.refunded"
)

// MediaKafkaTopics defines the Kafka topics emitted by the media domain.
const (
	MediaTopicUploadCreated       = "media.upload.created"
	MediaTopicUploadCompleted     = "media.upload.completed"
	MediaTopicUploadAborted       = "media.upload.aborted"
	MediaTopicScanStarted         = "media.scan.started"
	MediaTopicScanCompleted       = "media.scan.completed"
	MediaTopicScanFailed          = "media.scan.failed"
	MediaTopicProcessingStarted   = "media.processing.started"
	MediaTopicProcessingCompleted = "media.processing.completed"
	MediaTopicProcessingFailed    = "media.processing.failed"
	MediaTopicReady               = "media.ready"
	MediaTopicDeleteRequested     = "media.delete.requested"
	MediaTopicDeleted             = "media.deleted"
	MediaTopicDeleteFailed        = "media.delete.failed"
)

// UserKafkaTopics defines the Kafka topics emitted by the user domain.
const (
	UserTopicCreated                = "user.created"
	UserTopicPasswordResetRequested = "user.password_reset_requested"
)

// NotificationNatsSubjects defines the NATS subjects used by the notification domain.
// The user-specific subject is suffixed with .{userId}.
const (
	NotificationSubjectUser = "notification.user"
)

// RealtimeNatsSubjects defines the Core NATS subjects used by the realtime domain.
const (
	RealtimeLocationDriverUpdated  = "realtime.location.driver.updated"
	RealtimeDeliveryLocationUpd    = "realtime.delivery.location.updated"
	RealtimeDeliveryStatusUpdated  = "realtime.delivery.status.updated"
	RealtimeDriverAssignmentUpd    = "realtime.driver.assignment.updated"
	RealtimeDriverPresenceUpdated  = "realtime.driver.presence.updated"
	RealtimeCommandDriver          = "realtime.command.driver"
	RealtimeCommandDelivery        = "realtime.command.delivery"
)

// RealtimeKafkaTopics defines the Kafka DLQ topics used by the realtime domain.
const (
	RealtimeTopicDLQDelivery = "realtime.delivery.dlq"
	RealtimeTopicDLQPayment  = "realtime.payment.dlq"
)
