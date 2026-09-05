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

const (
	NotificationSubjectUser = "notification.user"
)

const (
	RealtimeLocationDriverUpdated  = "realtime.location.driver.updated"
	RealtimeDeliveryLocationUpd    = "realtime.delivery.location.updated"
	RealtimeDeliveryStatusUpdated  = "realtime.delivery.status.updated"
	RealtimeDriverAssignmentUpd    = "realtime.driver.assignment.updated"
	RealtimeDriverPresenceUpdated  = "realtime.driver.presence.updated"
	RealtimeCommandDriver          = "realtime.command.driver"
	RealtimeCommandDelivery        = "realtime.command.delivery"
	// Media realtime fan-out subjects (published by media-service workers)
	RealtimeMediaUploadProgress     = "realtime.media.upload.progress"
	RealtimeMediaProcessingProgress = "realtime.media.processing.progress"
	RealtimeMediaReady              = "realtime.media.ready"
	RealtimeMediaDeleted            = "realtime.media.deleted"
	RealtimeMediaFailed             = "realtime.media.failed"
)

// RealtimeKafkaTopics defines the Kafka DLQ topics used by the realtime domain.
const (
	RealtimeTopicDLQDelivery = "realtime.delivery.dlq"
	RealtimeTopicDLQPayment  = "realtime.payment.dlq"
)

// Driver topic constants (Kafka)
const (
	DriverTopicCreated = "driver.created"
	DriverTopicUpdated = "driver.updated"
	DriverTopicDeleted = "driver.deleted"
)

// Driver NATS subjects (transient, published via NATS for realtime coordination)
const (
	DriverSubjectLocationUpdated    = "driver.location.updated"
	DriverSubjectAssignmentOffered  = "driver.assignment.offered"
	DriverSubjectAssignmentCancelled = "driver.assignment.cancelled"
	DriverSubjectAssignmentUpdated  = "driver.assignment.updated"
	DriverSubjectStatusUpdated      = "driver.status.updated"
)

// Driver assignment events (published to Kafka for durable consumption).
const (
	DriverAssignmentOffered   = "driver.assignment.offered"
	DriverAssignmentAccepted  = "driver.assignment.accepted"
	DriverAssignmentRejected  = "driver.assignment.rejected"
	DriverAssignmentExpired   = "driver.assignment.expired"
	DriverAssignmentReleased  = "driver.assignment.released"
	DriverAssignmentCompleted = "driver.assignment.completed"
)

// Consumed by Search Service → OpenSearch for indexing.
const (
	SearchSubjectQueryStarted   = "search.query.started"
	SearchSubjectQueryCompleted = "search.query.completed"
	SearchSubjectReindexProgress = "search.reindex.progress"
)