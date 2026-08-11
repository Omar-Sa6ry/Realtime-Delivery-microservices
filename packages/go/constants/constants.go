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

// Roles
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

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
	},
	RoleUser: {
		PermissionUpdateUser,
		PermissionResetPassword,
		PermissionChangePassword,
		PermissionForgotPassword,
		PermissionLogout,
		PermissionRechargeWallet,
	},
}

// Payment Methods
type PaymentMethod string

const (
	PaymentMethodStripe PaymentMethod = "STRIPE"
	PaymentMethodPaypal PaymentMethod = "PAYPAL"
	PaymentMethodCash   PaymentMethod = "CASH"
)

// Pagination defaults
const (
	DefaultLimit = 10
	DefaultPage  = 1
)

// Message and Error Constants
const (
	CurrentUserMsg        = "User not found in request"
	PasswordValidator     = "Password should be from 6 to 16 digits"
	ExceptionFilterMsg    = "An error occurred"
)
