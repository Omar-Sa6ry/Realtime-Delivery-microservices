package constants

// Delivery Status Constants (State Machine)
const (
	DeliveryStatusPending         = "PENDING"
	DeliveryStatusSearchingDriver = "SEARCHING_DRIVER"
	DeliveryStatusDriverAssigned  = "DRIVER_ASSIGNED"
	DeliveryStatusDriverAccepted  = "DRIVER_ACCEPTED"
	DeliveryStatusPickupStarted   = "PICKUP_STARTED"
	DeliveryStatusPickedUp        = "PICKED_UP"
	DeliveryStatusInTransit       = "IN_TRANSIT"
	DeliveryStatusDelivered       = "DELIVERED"
	DeliveryStatusCancelled       = "CANCELLED"
	DeliveryStatusFailed          = "FAILED"
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

// Roles
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleUser   Role = "user"
	RoleDriver Role = "driver"
)

// AllRoles returns every defined role value
func AllRoles() []Role {
	return []Role{RoleAdmin, RoleUser, RoleDriver}
}

// Permissions
type Permission string

const (
	// User permissions
	PermissionUpdateUser   Permission = "update_user"
	PermissionDeleteUser   Permission = "delete_user"
	PermissionEditUserRole Permission = "edit_user_role"
	PermissionViewUser     Permission = "view_user"

	// Auth permissions
	PermissionResetPassword  Permission = "RESET_PASSWORD"
	PermissionChangePassword Permission = "CHANGE_PASSWORD"
	PermissionForgotPassword Permission = "FORGOT_PASSWORD"
	PermissionRechargeWallet Permission = "RECHARGE_WALLET"
	PermissionLogout         Permission = "LOGOUT"

	// Notification permissions
	PermissionReadNotification              Permission = "READ_NOTIFICATION"
	PermissionUpdateNotification            Permission = "UPDATE_NOTIFICATION"
	PermissionDeleteNotification            Permission = "DELETE_NOTIFICATION"
	PermissionManageNotificationPreferences Permission = "MANAGE_NOTIFICATION_PREFERENCES"
)

// RolePermissionsMap maps roles to their list of permissions
var RolePermissionsMap = map[Role][]Permission{
	RoleAdmin: {
		PermissionUpdateUser,
		PermissionDeleteUser,
		PermissionEditUserRole,
		PermissionResetPassword,
		PermissionChangePassword,
		PermissionForgotPassword,
		PermissionLogout,
		PermissionViewUser,
		PermissionRechargeWallet,
		PermissionReadNotification,
		PermissionUpdateNotification,
		PermissionDeleteNotification,
		PermissionManageNotificationPreferences,
		PermissionCreateDelivery,
		PermissionViewDelivery,
		PermissionUpdateDeliveryStatus,
		PermissionCancelDelivery,
		PermissionAssignDeliveryDriver,
	},
	RoleUser: {
		PermissionUpdateUser,
		PermissionResetPassword,
		PermissionChangePassword,
		PermissionForgotPassword,
		PermissionLogout,
		PermissionRechargeWallet,
		PermissionReadNotification,
		PermissionUpdateNotification,
		PermissionManageNotificationPreferences,
		PermissionCreateDelivery,
		PermissionViewDelivery,
		PermissionCancelDelivery,
	},
	RoleDriver: {
		PermissionUpdateUser,
		PermissionResetPassword,
		PermissionChangePassword,
		PermissionForgotPassword,
		PermissionLogout,
		PermissionReadNotification,
		PermissionUpdateNotification,
		PermissionManageNotificationPreferences,
		PermissionViewDelivery,
		PermissionUpdateDeliveryStatus,
		PermissionAssignDeliveryDriver,
	},
}

// Payment Methods
type PaymentMethod string

const (
	PaymentMethodStripe PaymentMethod = "STRIPE"
	PaymentMethodPaypal PaymentMethod = "PAYPAL"
	PaymentMethodCash   PaymentMethod = "CASH"
)

// NotificationType defines the domain event that triggered a notification.
type NotificationType string

const (
	NotificationTypeDeliveryCreated   NotificationType = "DELIVERY_CREATED"
	NotificationTypeDriverAssigned    NotificationType = "DRIVER_ASSIGNED"
	NotificationTypeDriverAccepted    NotificationType = "DRIVER_ACCEPTED"
	NotificationTypeDeliveryPickedUp  NotificationType = "DELIVERY_PICKED_UP"
	NotificationTypeDeliveryInTransit NotificationType = "DELIVERY_IN_TRANSIT"
	NotificationTypeDeliveryCompleted NotificationType = "DELIVERY_COMPLETED"
	NotificationTypeDeliveryCancelled NotificationType = "DELIVERY_CANCELLED"
	NotificationTypePaymentCompleted  NotificationType = "PAYMENT_COMPLETED"
	NotificationTypePaymentFailed     NotificationType = "PAYMENT_FAILED"
	NotificationTypePaymentRefunded   NotificationType = "PAYMENT_REFUNDED"
)

// NotificationStatus defines the lifecycle state of a notification.
type NotificationStatus string

const (
	NotificationStatusCreated    NotificationStatus = "CREATED"
	NotificationStatusQueued     NotificationStatus = "QUEUED"
	NotificationStatusProcessing NotificationStatus = "PROCESSING"
	NotificationStatusSent       NotificationStatus = "SENT"
	NotificationStatusDelivered  NotificationStatus = "DELIVERED"
	NotificationStatusFailed     NotificationStatus = "FAILED"
	NotificationStatusCancelled  NotificationStatus = "CANCELLED"
	NotificationStatusExpired    NotificationStatus = "EXPIRED"
)

// DeliveryChannelStatus defines the per-channel delivery lifecycle.
type DeliveryChannelStatus string

const (
	DeliveryChannelStatusPending    DeliveryChannelStatus = "PENDING"
	DeliveryChannelStatusQueued     DeliveryChannelStatus = "QUEUED"
	DeliveryChannelStatusProcessing DeliveryChannelStatus = "PROCESSING"
	DeliveryChannelStatusRetrying   DeliveryChannelStatus = "RETRYING"
	DeliveryChannelStatusSent       DeliveryChannelStatus = "SENT"
	DeliveryChannelStatusDelivered  DeliveryChannelStatus = "DELIVERED"
	DeliveryChannelStatusFailed     DeliveryChannelStatus = "FAILED"
)

// NotificationChannel defines the target delivery channel.
type NotificationChannel string

const (
	NotificationChannelEmail    NotificationChannel = "EMAIL"
	NotificationChannelSMS      NotificationChannel = "SMS"
	NotificationChannelPush     NotificationChannel = "PUSH"
	NotificationChannelInApp    NotificationChannel = "IN_APP"
	NotificationChannelRealtime NotificationChannel = "REALTIME"
)

// NotificationPriority defines the urgency level of a notification.
type NotificationPriority string

const (
	NotificationPriorityLow      NotificationPriority = "LOW"
	NotificationPriorityNormal   NotificationPriority = "NORMAL"
	NotificationPriorityHigh     NotificationPriority = "HIGH"
	NotificationPriorityCritical NotificationPriority = "CRITICAL"
)

// Pagination defaults
const (
	DefaultLimit = 10
	DefaultPage  = 1
)

// Message and Error Constants
const (
	CurrentUserMsg     = "User not found in request"
	PasswordValidator  = "Password should be from 6 to 16 digits"
	ExceptionFilterMsg = "An error occurred"
)

type SearchIndex string

const (
	SearchIndexDeliveries SearchIndex = "deliveries"
	SearchIndexDrivers    SearchIndex = "drivers"
	SearchIndexMedia      SearchIndex = "media"
)

// SearchKafkaTopics are the Kafka topics consumed by the Search Service consumer group.
const (
	// Delivery topics
	SearchTopicDeliveryCreated        = "delivery.created"
	SearchTopicDeliveryDriverAssigned = "delivery.driver.assigned"
	SearchTopicDeliveryDriverAccepted = "delivery.driver.accepted"
	SearchTopicDeliveryPickedUp       = "delivery.picked_up"
	SearchTopicDeliveryInTransit      = "delivery.in_transit"
	SearchTopicDeliveryCompleted      = "delivery.completed"
	SearchTopicDeliveryCancelled      = "delivery.cancelled"
	SearchTopicDeliveryDeleted        = "delivery.deleted"

	// Driver topics
	SearchTopicDriverCreated = "driver.created"
	SearchTopicDriverUpdated = "driver.updated"
	SearchTopicDriverDeleted = "driver.deleted"

	// Media topics (only index once media is READY)
	SearchTopicMediaReady   = "media.ready"
	SearchTopicMediaDeleted = "media.deleted"

	// Search DLQ
	SearchTopicDLQ = "search.dlq"

	// Analytics / observability
	SearchTopicQueryCompleted   = "search.query.completed"
	SearchTopicReindexCompleted = "search.reindex.completed"
)

const SearchConsumerGroupID = "search-service"

// DriverStatus defines the driver availability states.
type DriverStatus string

const (
	DriverStatusAvailable DriverStatus = "AVAILABLE"
	DriverStatusBusy      DriverStatus = "BUSY"
	DriverStatusOffline   DriverStatus = "OFFLINE"
)

type VehicleType string

const (
	VehicleTypeCar        VehicleType = "CAR"
	VehicleTypeMotorcycle VehicleType = "MOTORCYCLE"
	VehicleTypeTruck      VehicleType = "TRUCK"
	VehicleTypeBicycle    VehicleType = "BICYCLE"
)

// Search capability limits — tune by load testing.
const (
	SearchMaxPageSize      = 100
	SearchDefaultPageSize  = 10
	SearchMaxQueryLength   = 500
	SearchMaxFuzziness     = 2
	SearchCacheTTLSeconds  = 120
	SearchSuggestCacheTTL  = 300
)


// Delivery permissions shared with the TypeScript package.
const (
	PermissionCreateDelivery        Permission = "create_delivery"
	PermissionViewDelivery          Permission = "view_delivery"
	PermissionUpdateDeliveryStatus  Permission = "update_delivery_status"
	PermissionCancelDelivery        Permission = "cancel_delivery"
	PermissionAssignDeliveryDriver  Permission = "assign_delivery_driver"
)

// Additional notification types shared with the TypeScript package.
const (
	NotificationTypeMediaUploadCompleted   NotificationType = "MEDIA_UPLOAD_COMPLETED"
	NotificationTypeMediaUploadFailed      NotificationType = "MEDIA_UPLOAD_FAILED"
	NotificationTypeMediaScanFailed        NotificationType = "MEDIA_SCAN_FAILED"
	NotificationTypeMediaProcessingFailed  NotificationType = "MEDIA_PROCESSING_FAILED"
	NotificationTypeMediaReady             NotificationType = "MEDIA_READY"
	NotificationTypeMediaDeleted            NotificationType = "MEDIA_DELETED"
	NotificationTypeMediaDeleteFailed       NotificationType = "MEDIA_DELETE_FAILED"
	NotificationTypeUserRegistered          NotificationType = "USER_REGISTERED"
	NotificationTypePasswordResetRequested  NotificationType = "PASSWORD_RESET_REQUESTED"
)

// Payment, user, and realtime Kafka topics shared with the TypeScript package.
const (
	PaymentTopicCompleted = "payment.completed"
	PaymentTopicFailed    = "payment.failed"
	PaymentTopicRefunded  = "payment.refunded"

	UserTopicCreated              = "user.created"
	UserTopicUpdated              = "user.updated"
	UserTopicDeleted              = "user.deleted"
	UserTopicPasswordResetRequested = "user.password_reset_requested"

	RealtimeTopicDLQDelivery = "realtime.delivery.dlq"
	RealtimeTopicDLQPayment  = "realtime.payment.dlq"
)
