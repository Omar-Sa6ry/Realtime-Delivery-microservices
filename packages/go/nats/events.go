package nats

// User NATS events subjects matching the TypeScript definitions
const (
	UserEventGetByID         = "user.get.by.id"
	UserEventGetByEmail      = "user.get.by.email"
	UserEventExists          = "user.exists"
	UserEventUpdated         = "user.updated"
	UserEventDataExisted     = "user.dataExists"
	UserEventCreateData      = "user.createData"
	UserEventRoleUpdated     = "user.role.updated"
	UserEventFindUsersWithIDs = "user.findUsersWithIds"
	UserEventCheckIfInstructor = "user.checkIfInstructor"
)
