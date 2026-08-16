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

// NotificationNatsSubjects defines the NATS subjects used by the notification domain.
// The user-specific subject is suffixed with .{userId}.
const (
	NotificationSubjectUser = "notification.user"
)
