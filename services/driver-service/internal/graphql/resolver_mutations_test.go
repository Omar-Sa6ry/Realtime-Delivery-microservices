package graphql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// ---------------------------------------------------------------------------
// Mutation: goOnline
// ---------------------------------------------------------------------------

const goOnlineMutation = `mutation {
		goOnline(idempotencyKey: "k") { success statusCode message data { id status isActive isBlocked } }
	}`

func TestMutation_GoOnline(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		setup       func(*env)
		wantCode    float64
		wantMessage string
		wantDataNil bool
	}{
		{
			name:        "missing user context",
			headers:     nil,
			wantCode:    403,
			wantMessage: "Unauthorized",
			wantDataNil: true,
		},
		{
			name:        "unknown user",
			headers:     userHeaders("nobody", ""),
			wantCode:    403,
			wantMessage: "Driver profile not found",
			wantDataNil: true,
		},
		{
			name:    "inactive driver",
			headers: userHeaders("user-1", ""),
			setup: func(e *env) {
				d := newRichDriver("drv-1", "user-1")
				d.IsActive = false
				e.mockDrivers.SetDriver(d)
			},
			wantCode:    403,
			wantMessage: "Driver is not active, blocked, or suspended and cannot perform this action",
			wantDataNil: true,
		},
		{
			name:    "blocked driver",
			headers: userHeaders("user-1", ""),
			setup: func(e *env) {
				d := newRichDriver("drv-1", "user-1")
				d.IsBlocked = true
				e.mockDrivers.SetDriver(d)
			},
			wantCode:    403,
			wantMessage: "Driver is not active, blocked, or suspended and cannot perform this action",
			wantDataNil: true,
		},
		{
			name:    "suspended driver",
			headers: userHeaders("user-1", ""),
			setup: func(e *env) {
				d := newRichDriver("drv-1", "user-1")
				d.Status = domain.DriverStatusSuspended
				e.mockDrivers.SetDriver(d)
			},
			wantCode:    403,
			wantMessage: "Driver is not active, blocked, or suspended and cannot perform this action",
			wantDataNil: true,
		},
		{
			name:    "already online",
			headers: userHeaders("user-1", ""),
			setup: func(e *env) {
				e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))
			},
			wantCode:    400,
			wantMessage: "[driver_already_online] driver is already online",
			wantDataNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			if tt.setup != nil {
				tt.setup(e)
			}
			res := e.gql(goOnlineMutation, nil, tt.headers)

			wantNoErrors(t, res)
			base := "data.goOnline."
			wantNum(t, res.json, tt.wantCode, base+"statusCode")
			wantStr(t, res.json, tt.wantMessage, base+"message")
			if tt.wantDataNil {
				wantNull(t, res.json, base+"data")
			}
		})
	}

	t.Run("success", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(goOnlineMutation, nil, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.goOnline."
		wantBool(t, res.json, true, base+"success")
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Driver is now online and available", base+"message")
		wantStr(t, res.json, "AVAILABLE", base+"data", "status")
		wantBool(t, res.json, true, base+"data", "isActive")

		events := e.publisher.GetEvents()
		found := false
		for _, ev := range events {
			if ev == "DriverAvailable:drv-1" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected DriverAvailable:drv-1 event, got %v", events)
		}
	})
}

// ---------------------------------------------------------------------------
// Mutation: goOffline
// ---------------------------------------------------------------------------

const goOfflineMutation = `mutation {
		goOffline(idempotencyKey: "k") { success statusCode message data { id status } }
	}`

func TestMutation_GoOffline(t *testing.T) {
	t.Run("missing user context", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(goOfflineMutation, nil, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "goOffline", "statusCode")
		wantStr(t, res.json, "Unauthorized", "data", "goOffline", "message")
		wantNull(t, res.json, "data", "goOffline", "data")
	})

	t.Run("driver not available for transition", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1")) // OFFLINE
		res := e.gql(goOfflineMutation, nil, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 400, "data", "goOffline", "statusCode")
		wantStr(t, res.json, "[driver_not_offline] driver is not offline", "data", "goOffline", "message")
		wantNull(t, res.json, "data", "goOffline", "data")
	})

	t.Run("success", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))
		res := e.gql(goOfflineMutation, nil, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.goOffline."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Driver is now offline", base+"message")
		wantStr(t, res.json, "OFFLINE", base+"data", "status")
	})
}

// ---------------------------------------------------------------------------
// Mutation: acceptAssignment
// ---------------------------------------------------------------------------

const acceptMutation = `mutation($id: ID!) {
		acceptAssignment(assignmentId: $id, idempotencyKey: "k") {
			success statusCode message data { id status acceptedAt }
		}
	}`

func TestMutation_AcceptAssignment(t *testing.T) {
	t.Run("missing user context", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(acceptMutation, map[string]interface{}{"id": "a-1"}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "acceptAssignment", "statusCode")
		wantStr(t, res.json, "Unauthorized", "data", "acceptAssignment", "message")
		wantNull(t, res.json, "data", "acceptAssignment", "data")
	})

	t.Run("repository error", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockAssignments.SetError(errors.New("db down"))
		res := e.gql(acceptMutation, map[string]interface{}{"id": "a-1"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "acceptAssignment", "statusCode")
		wantStr(t, res.json, "Assignment not found", "data", "acceptAssignment", "message")
	})

	t.Run("not found", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(acceptMutation, map[string]interface{}{"id": "missing"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "acceptAssignment", "statusCode")
		wantNull(t, res.json, "data", "acceptAssignment", "data")
	})

	t.Run("assignment belongs to another driver", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-2", "del-1"))
		res := e.gql(acceptMutation, map[string]interface{}{"id": "a-1"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "acceptAssignment", "statusCode")
		wantNull(t, res.json, "data", "acceptAssignment", "data")
	})

	t.Run("assignment not offered", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		a := newOfferedAssignment("a-1", "drv-1", "del-1")
		a.Status = domain.AssignmentStatusAccepted
		e.mockAssignments.SetAssignment(a)
		res := e.gql(acceptMutation, map[string]interface{}{"id": "a-1"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 400, "data", "acceptAssignment", "statusCode")
		wantContains(t, res.json, "assignment is not in a valid state", "data", "acceptAssignment", "message")
		wantNull(t, res.json, "data", "acceptAssignment", "data")
	})

	t.Run("success", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
		res := e.gql(acceptMutation, map[string]interface{}{"id": "a-1"}, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.acceptAssignment."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Assignment accepted successfully", base+"message")
		wantStr(t, res.json, "ACCEPTED", base+"data", "status")
		wantNotNull(t, res.json, base+"data", "acceptedAt")

		found := false
		for _, ev := range e.publisher.GetEvents() {
			if ev == "AssignmentAccepted:a-1" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected AssignmentAccepted:a-1 event, got %v", e.publisher.GetEvents())
		}
	})
}

// ---------------------------------------------------------------------------
// Mutation: rejectAssignment
// ---------------------------------------------------------------------------

const rejectMutation = `mutation($id: ID!, $reason: String) {
		rejectAssignment(assignmentId: $id, reason: $reason, idempotencyKey: "k") {
			success statusCode message data { id status rejectedAt }
		}
	}`

func TestMutation_RejectAssignment(t *testing.T) {
	t.Run("missing user context", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(rejectMutation, map[string]interface{}{"id": "a-1"}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "rejectAssignment", "statusCode")
		wantNull(t, res.json, "data", "rejectAssignment", "data")
	})

	t.Run("not found", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(rejectMutation, map[string]interface{}{"id": "missing"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "rejectAssignment", "statusCode")
		wantNull(t, res.json, "data", "rejectAssignment", "data")
	})

	t.Run("assignment not offered", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		a := newOfferedAssignment("a-1", "drv-1", "del-1")
		a.Status = domain.AssignmentStatusExpired
		e.mockAssignments.SetAssignment(a)
		res := e.gql(rejectMutation, map[string]interface{}{"id": "a-1"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 400, "data", "rejectAssignment", "statusCode")
		wantNull(t, res.json, "data", "rejectAssignment", "data")
	})

	t.Run("success without reason", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
		res := e.gql(rejectMutation, map[string]interface{}{"id": "a-1"}, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.rejectAssignment."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Assignment rejected successfully", base+"message")
		wantStr(t, res.json, "REJECTED", base+"data", "status")
		wantNotNull(t, res.json, base+"data", "rejectedAt")

		found := false
		for _, ev := range e.publisher.GetEvents() {
			if ev == "AssignmentRejected:a-1" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected AssignmentRejected:a-1 event, got %v", e.publisher.GetEvents())
		}
	})

	t.Run("success with reason", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
		res := e.gql(rejectMutation,
			map[string]interface{}{"id": "a-1", "reason": "too far"}, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		wantNum(t, res.json, 200, "data", "rejectAssignment", "statusCode")
		wantStr(t, res.json, "REJECTED", "data", "rejectAssignment", "data", "status")
	})
}

// ---------------------------------------------------------------------------
// Mutation: selfAcceptDelivery
// ---------------------------------------------------------------------------

const selfAcceptMutation = `mutation($id: ID!) {
		selfAcceptDelivery(deliveryId: $id, idempotencyKey: "k") {
			success statusCode message data { id driverId deliveryId status }
		}
	}`

func TestMutation_SelfAcceptDelivery(t *testing.T) {
	t.Run("missing user context", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(selfAcceptMutation, map[string]interface{}{"id": "del-9"}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "selfAcceptDelivery", "statusCode")
		wantStr(t, res.json, "Unauthorized", "data", "selfAcceptDelivery", "message")
		wantNull(t, res.json, "data", "selfAcceptDelivery", "data")
	})

	t.Run("empty delivery id", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))
		res := e.gql(selfAcceptMutation, map[string]interface{}{"id": ""}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 400, "data", "selfAcceptDelivery", "statusCode")
		wantStr(t, res.json, "deliveryId is required", "data", "selfAcceptDelivery", "message")
		wantNull(t, res.json, "data", "selfAcceptDelivery", "data")
	})

	t.Run("driver not available", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1")) // OFFLINE
		res := e.gql(selfAcceptMutation, map[string]interface{}{"id": "del-9"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 400, "data", "selfAcceptDelivery", "statusCode")
		wantStr(t, res.json, "Driver must be online and available to self-accept a delivery",
			"data", "selfAcceptDelivery", "message")
		wantNull(t, res.json, "data", "selfAcceptDelivery", "data")
	})

	t.Run("reservation fails when the driver lock is held", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))
		acquired, err := e.lockManager.Acquire(context.Background(), "drv-1", time.Minute)
		if err != nil || !acquired {
			t.Fatalf("failed to pre-acquire lock: acquired=%v err=%v", acquired, err)
		}
		res := e.gql(selfAcceptMutation, map[string]interface{}{"id": "del-9"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 400, "data", "selfAcceptDelivery", "statusCode")
		wantContains(t, res.json, "driver is already reserved", "data", "selfAcceptDelivery", "message")
		wantNull(t, res.json, "data", "selfAcceptDelivery", "data")
	})

	t.Run("accepting the freshly reserved assignment fails", func(t *testing.T) {
		e := newEnv(t, withFlakyAssignmentSave(2))
		e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))
		res := e.gql(selfAcceptMutation, map[string]interface{}{"id": "del-9"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 400, "data", "selfAcceptDelivery", "statusCode")
		wantContains(t, res.json, "Failed to accept assignment", "data", "selfAcceptDelivery", "message")
		wantNull(t, res.json, "data", "selfAcceptDelivery", "data")
	})

	t.Run("success", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))
		res := e.gql(selfAcceptMutation, map[string]interface{}{"id": "del-9"}, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.selfAcceptDelivery."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Delivery self-accepted successfully", base+"message")
		wantStr(t, res.json, "del-9-drv-1", base+"data", "id")
		wantStr(t, res.json, "drv-1", base+"data", "driverId")
		wantStr(t, res.json, "del-9", base+"data", "deliveryId")
		wantStr(t, res.json, "ACCEPTED", base+"data", "status")

		found := false
		for _, ev := range e.publisher.GetEvents() {
			if ev == "AssignmentAccepted:del-9-drv-1" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected AssignmentAccepted:del-9-drv-1 event, got %v", e.publisher.GetEvents())
		}
	})

	t.Run("success without a root event publisher", func(t *testing.T) {
		e := newEnv(t, withoutRootPublisher())
		e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))
		res := e.gql(selfAcceptMutation, map[string]interface{}{"id": "del-9"}, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		wantNum(t, res.json, 200, "data", "selfAcceptDelivery", "statusCode")
		wantStr(t, res.json, "ACCEPTED", "data", "selfAcceptDelivery", "data", "status")
	})
}

// ---------------------------------------------------------------------------
// Mutation: registerDriver
// ---------------------------------------------------------------------------

const registerMutation = `mutation($input: RegisterDriverInput!) {
		registerDriver(input: $input) {
			success statusCode message data { id userId status isActive vehicleType plateNumber capacityKg capabilities serviceArea }
		}
	}`

func registerInput(userID interface{}) map[string]interface{} {
	in := map[string]interface{}{
		"vehicleType": "CAR",
		"plateNumber": "ABC-123",
		"capacityKg":  60,
	}
	if userID != nil {
		in["userId"] = userID
	}
	return in
}

func TestMutation_RegisterDriver(t *testing.T) {
	t.Run("anonymous application is rejected", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(registerMutation, map[string]interface{}{"input": registerInput(nil)}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 401, "data", "registerDriver", "statusCode")
		wantStr(t, res.json, "Unauthorized: Login required to apply as driver",
			"data", "registerDriver", "message")
		wantNull(t, res.json, "data", "registerDriver", "data")
	})

	t.Run("user application starts inactive without optionals", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(registerMutation, map[string]interface{}{"input": registerInput(nil)},
			userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.registerDriver."
		wantNum(t, res.json, 201, base+"statusCode")
		wantStr(t, res.json, "Driver application submitted successfully, waiting for admin approval", base+"message")
		wantStr(t, res.json, "user-1", base+"data", "userId")
		wantBool(t, res.json, false, base+"data", "isActive")
		wantStr(t, res.json, "OFFLINE", base+"data", "status")
		wantStr(t, res.json, "CAR", base+"data", "vehicleType")
		wantStr(t, res.json, "ABC-123", base+"data", "plateNumber")
		wantNum(t, res.json, 60, base+"data", "capacityKg")
		wantListLen(t, res.json, 0, base+"data", "capabilities")
		wantNull(t, res.json, base+"data", "serviceArea")
	})

	t.Run("user application with capabilities and service area", func(t *testing.T) {
		e := newEnv(t)
		input := registerInput(nil)
		input["capabilities"] = []string{"STANDARD", "COLD_CHAIN"}
		input["serviceArea"] = "Giza"
		res := e.gql(registerMutation, map[string]interface{}{"input": input},
			userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.registerDriver."
		wantNum(t, res.json, 201, base+"statusCode")
		wantListLen(t, res.json, 2, base+"data", "capabilities")
		wantStr(t, res.json, "STANDARD", base+"data", "capabilities", "0")
		wantStr(t, res.json, "Giza", base+"data", "serviceArea")
	})

	t.Run("admin registers an active driver for another user", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(registerMutation,
			map[string]interface{}{"input": registerInput("target-user")},
			userHeaders("admin-1", "admin"))
		wantNoErrors(t, res)

		base := "data.registerDriver."
		wantNum(t, res.json, 201, base+"statusCode")
		wantStr(t, res.json, "Driver registered successfully", base+"message")
		wantStr(t, res.json, "target-user", base+"data", "userId")
		wantBool(t, res.json, true, base+"data", "isActive")
	})

	t.Run("admin without userId falls back to the admin identity", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(registerMutation, map[string]interface{}{"input": registerInput(nil)},
			userHeaders("admin-1", "admin"))
		wantNoErrors(t, res)

		wantStr(t, res.json, "admin-1", "data", "registerDriver", "data", "userId")
		wantBool(t, res.json, true, "data", "registerDriver", "data", "isActive")
	})

	t.Run("uppercase ADMIN role with empty userId falls back to caller", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(registerMutation, map[string]interface{}{"input": registerInput("")},
			userHeaders("admin-1", "ADMIN"))
		wantNoErrors(t, res)

		wantStr(t, res.json, "admin-1", "data", "registerDriver", "data", "userId")
		wantBool(t, res.json, true, "data", "registerDriver", "data", "isActive")
	})

	t.Run("duplicate registration maps to 400", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(registerMutation, map[string]interface{}{"input": registerInput("user-1")},
			userHeaders("admin-1", "admin"))
		wantNoErrors(t, res)

		base := "data.registerDriver."
		wantBool(t, res.json, false, base+"success")
		wantNum(t, res.json, 400, base+"statusCode")
		wantStr(t, res.json, "[driver_already_exists] driver is already registered", base+"message")
		wantNull(t, res.json, base+"data")
	})

	t.Run("repository failure maps to 500", func(t *testing.T) {
		e := newEnv(t, withDriverSaveError(errors.New("save failed")))
		res := e.gql(registerMutation, map[string]interface{}{"input": registerInput(nil)},
			userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.registerDriver."
		wantBool(t, res.json, false, base+"success")
		wantNum(t, res.json, 500, base+"statusCode")
		wantStr(t, res.json, "save failed", base+"message")
		wantNull(t, res.json, base+"data")
	})
}

// ---------------------------------------------------------------------------
// Mutation: updateDriverProfile
// ---------------------------------------------------------------------------

const updateMutation = `mutation($input: UpdateDriverProfileInput!) {
		updateDriverProfile(input: $input) {
			success statusCode message data { id vehicleType plateNumber capacityKg serviceArea }
		}
	}`

func TestMutation_UpdateDriverProfile(t *testing.T) {
	t.Run("driver not found", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(updateMutation,
			map[string]interface{}{"input": map[string]interface{}{"driverId": "missing"}}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "updateDriverProfile", "statusCode")
		wantStr(t, res.json, "Driver profile not found", "data", "updateDriverProfile", "message")
		wantNull(t, res.json, "data", "updateDriverProfile", "data")
	})

	t.Run("repository error", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetError(errors.New("db down"))
		res := e.gql(updateMutation,
			map[string]interface{}{"input": map[string]interface{}{"driverId": "drv-1"}}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "updateDriverProfile", "statusCode")
		wantNull(t, res.json, "data", "updateDriverProfile", "data")
	})

	t.Run("forbidden for other users", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-2"))
		res := e.gql(updateMutation,
			map[string]interface{}{"input": map[string]interface{}{"driverId": "drv-1"}},
			userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "updateDriverProfile", "statusCode")
		wantStr(t, res.json, "Forbidden: Only the driver or admin can update driver profile",
			"data", "updateDriverProfile", "message")
		wantNull(t, res.json, "data", "updateDriverProfile", "data")
	})

	t.Run("owner keeps optionals untouched when all are null", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(updateMutation,
			map[string]interface{}{"input": map[string]interface{}{"driverId": "drv-1"}},
			userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.updateDriverProfile."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Driver profile updated successfully", base+"message")
		wantStr(t, res.json, "CAR", base+"data", "vehicleType")
		wantStr(t, res.json, "PLATE-drv-1", base+"data", "plateNumber")
		wantNum(t, res.json, 75, base+"data", "capacityKg")
		wantStr(t, res.json, "Cairo", base+"data", "serviceArea")
	})

	t.Run("admin applies every optional", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-2"))
		res := e.gql(updateMutation,
			map[string]interface{}{"input": map[string]interface{}{
				"driverId":    "drv-1",
				"vehicleType": "MOTORCYCLE",
				"plateNumber": "NEW-PLATE",
				"capacityKg":  100,
				"serviceArea": "Giza",
			}},
			userHeaders("admin-1", "admin"))
		wantNoErrors(t, res)

		base := "data.updateDriverProfile."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "MOTORCYCLE", base+"data", "vehicleType")
		wantStr(t, res.json, "NEW-PLATE", base+"data", "plateNumber")
		wantNum(t, res.json, 100, base+"data", "capacityKg")
		wantStr(t, res.json, "Giza", base+"data", "serviceArea")
	})

	t.Run("save failure maps to 500", func(t *testing.T) {
		e := newEnv(t, withDriverSaveError(errors.New("save failed")))
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(updateMutation,
			map[string]interface{}{"input": map[string]interface{}{"driverId": "drv-1"}},
			userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "updateDriverProfile", "statusCode")
		wantStr(t, res.json, "save failed", "data", "updateDriverProfile", "message")
		wantNull(t, res.json, "data", "updateDriverProfile", "data")
	})
}

// ---------------------------------------------------------------------------
// Mutation: suspendDriver
// ---------------------------------------------------------------------------

const suspendMutation = `mutation($id: ID!, $reason: String!) {
		suspendDriver(driverId: $id, reason: $reason) { success statusCode message data { id status } }
	}`

func TestMutation_SuspendDriver(t *testing.T) {
	t.Run("driver not found", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(suspendMutation,
			map[string]interface{}{"id": "missing", "reason": "violation"}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "suspendDriver", "statusCode")
		wantStr(t, res.json, "Driver not found", "data", "suspendDriver", "message")
		wantNull(t, res.json, "data", "suspendDriver", "data")
	})

	t.Run("repository error", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetError(errors.New("db down"))
		res := e.gql(suspendMutation,
			map[string]interface{}{"id": "drv-1", "reason": "violation"}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "suspendDriver", "statusCode")
		wantNull(t, res.json, "data", "suspendDriver", "data")
	})

	t.Run("save failure maps to 500", func(t *testing.T) {
		e := newEnv(t, withDriverSaveError(errors.New("save failed")))
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(suspendMutation,
			map[string]interface{}{"id": "drv-1", "reason": "violation"}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "suspendDriver", "statusCode")
		wantStr(t, res.json, "save failed", "data", "suspendDriver", "message")
		wantNull(t, res.json, "data", "suspendDriver", "data")
	})

	t.Run("success", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))
		res := e.gql(suspendMutation,
			map[string]interface{}{"id": "drv-1", "reason": "late deliveries"}, nil)
		wantNoErrors(t, res)

		base := "data.suspendDriver."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Driver suspended successfully: late deliveries", base+"message")
		wantStr(t, res.json, "SUSPENDED", base+"data", "status")
	})
}

// ---------------------------------------------------------------------------
// Mutation: activateDriver
// ---------------------------------------------------------------------------

const activateMutation = `mutation($id: ID!) {
		activateDriver(driverId: $id) { success statusCode message data { id isActive } }
	}`

func TestMutation_ActivateDriver(t *testing.T) {
	t.Run("forbidden for non-admins", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(activateMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("user-1", "driver"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "activateDriver", "statusCode")
		wantStr(t, res.json, "Forbidden: Only admin can activate driver", "data", "activateDriver", "message")
		wantNull(t, res.json, "data", "activateDriver", "data")
	})

	t.Run("driver not found", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(activateMutation, map[string]interface{}{"id": "missing"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "activateDriver", "statusCode")
		wantStr(t, res.json, "Driver not found", "data", "activateDriver", "message")
		wantNull(t, res.json, "data", "activateDriver", "data")
	})

	t.Run("repository error", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetError(errors.New("db down"))
		res := e.gql(activateMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "activateDriver", "statusCode")
		wantNull(t, res.json, "data", "activateDriver", "data")
	})

	t.Run("save failure maps to 500", func(t *testing.T) {
		e := newEnv(t, withDriverSaveError(errors.New("save failed")))
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(activateMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "activateDriver", "statusCode")
		wantStr(t, res.json, "save failed", "data", "activateDriver", "message")
		wantNull(t, res.json, "data", "activateDriver", "data")
	})

	t.Run("success as admin", func(t *testing.T) {
		e := newEnv(t)
		d := newRichDriver("drv-1", "user-1")
		d.IsActive = false
		e.mockDrivers.SetDriver(d)
		res := e.gql(activateMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.activateDriver."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Driver activated successfully", base+"message")
		wantBool(t, res.json, true, base+"data", "isActive")
	})

	t.Run("success with uppercase ADMIN role", func(t *testing.T) {
		e := newEnv(t)
		d := newRichDriver("drv-1", "user-1")
		d.IsActive = false
		e.mockDrivers.SetDriver(d)
		res := e.gql(activateMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "ADMIN"))
		wantNoErrors(t, res)

		wantNum(t, res.json, 200, "data", "activateDriver", "statusCode")
		wantBool(t, res.json, true, "data", "activateDriver", "data", "isActive")
	})
}

// ---------------------------------------------------------------------------
// Mutation: deactivateDriver
// ---------------------------------------------------------------------------

const deactivateMutation = `mutation($id: ID!) {
		deactivateDriver(driverId: $id) { success statusCode message data { id isActive status } }
	}`

func TestMutation_DeactivateDriver(t *testing.T) {
	t.Run("forbidden for non-admins", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(deactivateMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("user-1", "driver"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "deactivateDriver", "statusCode")
		wantStr(t, res.json, "Forbidden: Only admin can deactivate driver", "data", "deactivateDriver", "message")
		wantNull(t, res.json, "data", "deactivateDriver", "data")
	})

	t.Run("driver not found", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(deactivateMutation, map[string]interface{}{"id": "missing"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "deactivateDriver", "statusCode")
		wantStr(t, res.json, "Driver not found", "data", "deactivateDriver", "message")
		wantNull(t, res.json, "data", "deactivateDriver", "data")
	})

	t.Run("save failure maps to 500", func(t *testing.T) {
		e := newEnv(t, withDriverSaveError(errors.New("save failed")))
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(deactivateMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "deactivateDriver", "statusCode")
		wantStr(t, res.json, "save failed", "data", "deactivateDriver", "message")
		wantNull(t, res.json, "data", "deactivateDriver", "data")
	})

	t.Run("available driver is demoted to offline", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))
		res := e.gql(deactivateMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.deactivateDriver."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Driver deactivated successfully", base+"message")
		wantBool(t, res.json, false, base+"data", "isActive")
		wantStr(t, res.json, "OFFLINE", base+"data", "status")
	})

	t.Run("busy driver keeps its status", func(t *testing.T) {
		e := newEnv(t)
		d := newAvailableDriver("drv-1", "user-1")
		d.Status = domain.DriverStatusBusy
		e.mockDrivers.SetDriver(d)
		res := e.gql(deactivateMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.deactivateDriver."
		wantBool(t, res.json, false, base+"data", "isActive")
		wantStr(t, res.json, "BUSY", base+"data", "status")
	})
}

// ---------------------------------------------------------------------------
// Mutation: blockDriver / unblockDriver
// ---------------------------------------------------------------------------

const blockMutation = `mutation($id: ID!, $reason: String) {
		blockDriver(driverId: $id, reason: $reason) { success statusCode message data { id isBlocked } }
	}`

func TestMutation_BlockDriver(t *testing.T) {
	t.Run("forbidden for non-admins", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(blockMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("user-1", "driver"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "blockDriver", "statusCode")
		wantStr(t, res.json, "Forbidden: Only admin can block driver", "data", "blockDriver", "message")
		wantNull(t, res.json, "data", "blockDriver", "data")
	})

	t.Run("uppercase ADMIN role is rejected (exact match only)", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(blockMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "ADMIN"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "blockDriver", "statusCode")
		wantStr(t, res.json, "Forbidden: Only admin can block driver", "data", "blockDriver", "message")
	})

	t.Run("driver not found maps to 500", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(blockMutation, map[string]interface{}{"id": "missing"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "blockDriver", "statusCode")
		wantContains(t, res.json, "driver not found", "data", "blockDriver", "message")
		wantNull(t, res.json, "data", "blockDriver", "data")
	})

	t.Run("repository error maps to 500", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetError(errors.New("db down"))
		res := e.gql(blockMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "blockDriver", "statusCode")
		wantStr(t, res.json, "db down", "data", "blockDriver", "message")
		wantNull(t, res.json, "data", "blockDriver", "data")
	})

	t.Run("success without reason", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(blockMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.blockDriver."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Driver blocked successfully", base+"message")
		wantBool(t, res.json, true, base+"data", "isBlocked")
	})

	t.Run("success with reason", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(blockMutation,
			map[string]interface{}{"id": "drv-1", "reason": "fraud"}, userHeaders("", "admin"))
		wantNoErrors(t, res)

		wantNum(t, res.json, 200, "data", "blockDriver", "statusCode")
		wantBool(t, res.json, true, "data", "blockDriver", "data", "isBlocked")
	})
}

const unblockMutation = `mutation($id: ID!, $reason: String) {
		unblockDriver(driverId: $id, reason: $reason) { success statusCode message data { id isBlocked } }
	}`

func TestMutation_UnblockDriver(t *testing.T) {
	t.Run("forbidden for non-admins", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(unblockMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("user-1", "driver"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "unblockDriver", "statusCode")
		wantStr(t, res.json, "Forbidden: Only admin can unblock driver", "data", "unblockDriver", "message")
		wantNull(t, res.json, "data", "unblockDriver", "data")
	})

	t.Run("driver not found maps to 500", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(unblockMutation, map[string]interface{}{"id": "missing"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "unblockDriver", "statusCode")
		wantContains(t, res.json, "driver not found", "data", "unblockDriver", "message")
		wantNull(t, res.json, "data", "unblockDriver", "data")
	})

	t.Run("success without reason", func(t *testing.T) {
		e := newEnv(t)
		d := newRichDriver("drv-1", "user-1")
		d.IsBlocked = true
		e.mockDrivers.SetDriver(d)
		res := e.gql(unblockMutation, map[string]interface{}{"id": "drv-1"}, userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.unblockDriver."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Driver unblocked successfully", base+"message")
		wantBool(t, res.json, false, base+"data", "isBlocked")
	})

	t.Run("success with reason", func(t *testing.T) {
		e := newEnv(t)
		d := newRichDriver("drv-1", "user-1")
		d.IsBlocked = true
		e.mockDrivers.SetDriver(d)
		res := e.gql(unblockMutation,
			map[string]interface{}{"id": "drv-1", "reason": "reviewed"}, userHeaders("", "admin"))
		wantNoErrors(t, res)

		wantNum(t, res.json, 200, "data", "unblockDriver", "statusCode")
		wantBool(t, res.json, false, "data", "unblockDriver", "data", "isBlocked")
	})
}

// ---------------------------------------------------------------------------
// Mutation: rateDriver
// ---------------------------------------------------------------------------

const rateMutation = `mutation($input: RateDriverInput!) {
		rateDriver(input: $input) {
			success statusCode message data { id driverId userId deliveryId rating comment createdAt }
		}
	}`

func rateInput(driverID, deliveryID string, rating float64, comment interface{}) map[string]interface{} {
	in := map[string]interface{}{
		"driverId":   driverID,
		"deliveryId": deliveryID,
		"rating":     rating,
	}
	if comment != nil {
		in["comment"] = comment
	}
	return in
}

func TestMutation_RateDriver(t *testing.T) {
	seedRatedDelivery := func(customer, driver string) func(*env) {
		return func(e *env) {
			e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
			e.mockClient.SetDelivery(&ports.DeliveryInfo{
				DeliveryID: "del-1",
				CustomerID: customer,
				DriverID:   driver,
			})
		}
	}

	tests := []struct {
		name        string
		headers     map[string]string
		input       map[string]interface{}
		setup       func(*env)
		wantCode    float64
		wantMessage string
		wantDataNil bool
	}{
		{
			name:        "rating below range",
			input:       rateInput("drv-1", "del-1", 0, nil),
			wantCode:    400,
			wantMessage: "Rating must be between 1.0 and 5.0",
			wantDataNil: true,
		},
		{
			name:        "rating above range",
			input:       rateInput("drv-1", "del-1", 6, nil),
			wantCode:    400,
			wantMessage: "Rating must be between 1.0 and 5.0",
			wantDataNil: true,
		},
		{
			name:    "repository failure is not a domain error",
			headers: userHeaders("user-c", ""),
			input:   rateInput("drv-1", "del-1", 4, nil),
			setup: func(e *env) {
				e.mockDrivers.SetError(errors.New("db down"))
			},
			wantCode:    500,
			wantMessage: "db down",
			wantDataNil: true,
		},
		{
			name:    "delivery client failure",
			headers: userHeaders("user-c", ""),
			input:   rateInput("drv-1", "del-1", 4, nil),
			setup: func(e *env) {
				seedRatedDelivery("user-c", "drv-1")(e)
				e.mockClient.SetError("del-1", errors.New("grpc down"))
			},
			wantCode:    500,
			wantMessage: "grpc down",
			wantDataNil: true,
		},
		{
			name:    "delivery not found maps to 404",
			headers: userHeaders("user-c", ""),
			input:   rateInput("drv-1", "del-1", 4, nil),
			setup: func(e *env) {
				e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
			},
			wantCode:    404,
			wantMessage: "delivery not found",
			wantDataNil: true,
		},
		{
			name:    "caller is not the delivery owner",
			headers: userHeaders("someone-else", ""),
			input:   rateInput("drv-1", "del-1", 4, nil),
			setup: func(e *env) {
				seedRatedDelivery("user-c", "drv-1")(e)
			},
			wantCode:    403,
			wantMessage: "only the customer who created this delivery can rate the driver",
			wantDataNil: true,
		},
		{
			name:    "driver was not assigned to the delivery",
			headers: userHeaders("user-c", ""),
			input:   rateInput("drv-1", "del-1", 4, nil),
			setup: func(e *env) {
				seedRatedDelivery("user-c", "drv-other")(e)
			},
			wantCode:    403,
			wantMessage: "this driver was not assigned to this delivery",
			wantDataNil: true,
		},
		{
			name:    "duplicate review maps to 400",
			headers: userHeaders("user-c", ""),
			input:   rateInput("drv-1", "del-1", 4, nil),
			setup: func(e *env) {
				seedRatedDelivery("user-c", "drv-1")(e)
				e.reviewRepo.SetReview(newReview("rev-old", "drv-1", "user-c", "del-1", 3, ""))
			},
			wantCode:    400,
			wantMessage: "driver has already been reviewed for this delivery",
			wantDataNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			if tt.setup != nil {
				tt.setup(e)
			}
			res := e.gql(rateMutation, map[string]interface{}{"input": tt.input}, tt.headers)

			wantNoErrors(t, res)
			base := "data.rateDriver."
			wantBool(t, res.json, false, base+"success")
			wantNum(t, res.json, tt.wantCode, base+"statusCode")
			wantStr(t, res.json, tt.wantMessage, base+"message")
			if tt.wantDataNil {
				wantNull(t, res.json, base+"data")
			}
		})
	}

	t.Run("success with the default anonymous user", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockClient.SetDelivery(&ports.DeliveryInfo{
			DeliveryID: "del-1",
			CustomerID: "user-client",
			DriverID:   "drv-1",
		})
		res := e.gql(rateMutation,
			map[string]interface{}{"input": rateInput("drv-1", "del-1", 4.5, "nice ride")},
			nil)
		wantNoErrors(t, res)

		base := "data.rateDriver."
		wantBool(t, res.json, true, base+"success")
		wantNum(t, res.json, 201, base+"statusCode")
		wantStr(t, res.json, "Driver rated successfully", base+"message")
		wantNotNull(t, res.json, base+"data", "id")
		wantStr(t, res.json, "drv-1", base+"data", "driverId")
		wantStr(t, res.json, "user-client", base+"data", "userId")
		wantStr(t, res.json, "del-1", base+"data", "deliveryId")
		wantNum(t, res.json, 4.5, base+"data", "rating")
		wantStr(t, res.json, "nice ride", base+"data", "comment")
		wantNotNull(t, res.json, base+"data", "createdAt")
	})

	t.Run("success for an authenticated customer without a comment", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockClient.SetDelivery(&ports.DeliveryInfo{
			DeliveryID: "del-1",
			CustomerID: "user-c",
			DriverID:   "drv-1",
		})
		res := e.gql(rateMutation,
			map[string]interface{}{"input": rateInput("drv-1", "del-1", 5, nil)},
			userHeaders("user-c", ""))
		wantNoErrors(t, res)

		base := "data.rateDriver."
		wantNum(t, res.json, 201, base+"statusCode")
		wantStr(t, res.json, "user-c", base+"data", "userId")
		wantNull(t, res.json, base+"data", "comment")
	})
}
