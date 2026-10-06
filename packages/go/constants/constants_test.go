package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllRoles(t *testing.T) {
	roles := AllRoles()

	expected := []Role{RoleAdmin, RoleUser, RoleDriver}
	require.Len(t, roles, len(expected))
	assert.Equal(t, expected, roles)
}

func TestAllRolesCoverEveryRolePermissionsKey(t *testing.T) {
	declared := map[Role]bool{}
	for _, role := range AllRoles() {
		declared[role] = true
	}

	for role := range RolePermissionsMap {
		assert.Truef(t, declared[role], "RolePermissionsMap has undeclared role %q", role)
	}

	require.Len(t, RolePermissionsMap, len(AllRoles()))
}

func TestRolePermissionsMapEntries(t *testing.T) {
	for role, perms := range RolePermissionsMap {
		assert.NotEmptyf(t, perms, "role %q must grant at least one permission", role)

		seen := map[Permission]bool{}
		for _, perm := range perms {
			assert.NotEmpty(t, string(perm), "role %q has an empty permission", role)
			assert.Falsef(t, seen[perm], "role %q repeats permission %q", role, perm)
			seen[perm] = true
		}
	}
}

func TestRolePermissionInheritance(t *testing.T) {
	// Every role must be able to manage notifications and reset its password.
	required := []Permission{
		PermissionResetPassword,
		PermissionChangePassword,
		PermissionForgotPassword,
		PermissionLogout,
		PermissionReadNotification,
		PermissionUpdateNotification,
	}

	for _, role := range AllRoles() {
		perms := RolePermissionsMap[role]
		for _, want := range required {
			assert.Containsf(t, perms, want, "role %q is missing %q", role, want)
		}
	}
}

func TestDeliveryStatusConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"pending", DeliveryStatusPending, "PENDING"},
		{"searching", DeliveryStatusSearchingDriver, "SEARCHING_DRIVER"},
		{"assigned", DeliveryStatusDriverAssigned, "DRIVER_ASSIGNED"},
		{"accepted", DeliveryStatusDriverAccepted, "DRIVER_ACCEPTED"},
		{"pickup started", DeliveryStatusPickupStarted, "PICKUP_STARTED"},
		{"picked up", DeliveryStatusPickedUp, "PICKED_UP"},
		{"in transit", DeliveryStatusInTransit, "IN_TRANSIT"},
		{"delivered", DeliveryStatusDelivered, "DELIVERED"},
		{"cancelled", DeliveryStatusCancelled, "CANCELLED"},
		{"failed", DeliveryStatusFailed, "FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

func TestHeaderKeyConstants(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{HeaderXUserId, "x-user-id"},
		{HeaderXUserRole, "x-user-role"},
		{HeaderXUserSession, "x-user-session"},
		{HeaderXCorrelationId, "x-correlation-id"},
		{HeaderXLang, "x-lang"},
		{HeaderAcceptLanguage, "accept-language"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.key)
	}
}

func TestPaginationDefaults(t *testing.T) {
	assert.Equal(t, 10, DefaultLimit)
	assert.Equal(t, 1, DefaultPage)
}

func TestSearchCapabilityLimits(t *testing.T) {
	assert.Equal(t, 100, SearchMaxPageSize)
	assert.Equal(t, 10, SearchDefaultPageSize)
	assert.Equal(t, 500, SearchMaxQueryLength)
	assert.Equal(t, 2, SearchMaxFuzziness)
	assert.Equal(t, 120, SearchCacheTTLSeconds)
	assert.Equal(t, 300, SearchSuggestCacheTTL)
}

func TestPaymentAndDriverEnums(t *testing.T) {
	assert.Equal(t, PaymentMethod("STRIPE"), PaymentMethodStripe)
	assert.Equal(t, PaymentMethod("PAYPAL"), PaymentMethodPaypal)
	assert.Equal(t, PaymentMethod("CASH"), PaymentMethodCash)

	assert.Equal(t, PaymentProvider("STRIPE"), PaymentProviderStripe)
	assert.Equal(t, PaymentProvider("PAYPAL"), PaymentProviderPaypal)
	assert.Equal(t, PaymentProvider("CASH"), PaymentProviderCash)

	assert.Equal(t, DriverStatus("AVAILABLE"), DriverStatusAvailable)
	assert.Equal(t, DriverStatus("BUSY"), DriverStatusBusy)
	assert.Equal(t, DriverStatus("OFFLINE"), DriverStatusOffline)

	assert.Equal(t, VehicleType("CAR"), VehicleTypeCar)
	assert.Equal(t, VehicleType("MOTORCYCLE"), VehicleTypeMotorcycle)
	assert.Equal(t, VehicleType("TRUCK"), VehicleTypeTruck)
	assert.Equal(t, VehicleType("BICYCLE"), VehicleTypeBicycle)
}

func TestNotificationEnumsAreDistinct(t *testing.T) {
	channels := []NotificationChannel{
		NotificationChannelEmail,
		NotificationChannelSMS,
		NotificationChannelPush,
		NotificationChannelInApp,
		NotificationChannelRealtime,
	}
	seen := map[NotificationChannel]bool{}
	for _, c := range channels {
		require.NotEmpty(t, string(c))
		assert.Falsef(t, seen[c], "duplicate notification channel %q", c)
		seen[c] = true
	}

	priorities := []NotificationPriority{
		NotificationPriorityLow,
		NotificationPriorityNormal,
		NotificationPriorityHigh,
		NotificationPriorityCritical,
	}
	seenP := map[NotificationPriority]bool{}
	for _, p := range priorities {
		assert.Falsef(t, seenP[p], "duplicate notification priority %q", p)
		seenP[p] = true
	}

	statuses := []NotificationStatus{
		NotificationStatusCreated,
		NotificationStatusQueued,
		NotificationStatusProcessing,
		NotificationStatusSent,
		NotificationStatusDelivered,
		NotificationStatusFailed,
		NotificationStatusCancelled,
		NotificationStatusExpired,
	}
	seenS := map[NotificationStatus]bool{}
	for _, s := range statuses {
		assert.Falsef(t, seenS[s], "duplicate notification status %q", s)
		seenS[s] = true
	}
}

func TestAnalyticsEventTypesFollowSnakeCaseConvention(t *testing.T) {
	for _, et := range []AnalyticsEventType{
		AnalyticsEventDeliveryCreated,
		AnalyticsEventDriverAvailable,
		AnalyticsEventPaymentCaptured,
		AnalyticsEventNotificationSent,
	} {
		assert.Regexp(t, `^[A-Z]+(_[A-Z]+)+$`, string(et),
			"event type %q should be upper snake case", et)
	}
}

func TestSearchTopicsAndConsumerGroup(t *testing.T) {
	assert.Equal(t, "search-service", SearchConsumerGroupID)
	assert.Equal(t, "search.dlq", SearchTopicDLQ)
	assert.Equal(t, SearchIndex("deliveries"), SearchIndexDeliveries)
	assert.Equal(t, SearchIndex("drivers"), SearchIndexDrivers)
	assert.Equal(t, SearchIndex("media"), SearchIndexMedia)
}

func TestMessagesConstants(t *testing.T) {
	assert.Equal(t, "User not found in request", CurrentUserMsg)
	assert.Equal(t, "Password should be from 6 to 16 digits", PasswordValidator)
	assert.Equal(t, "An error occurred", ExceptionFilterMsg)
}
