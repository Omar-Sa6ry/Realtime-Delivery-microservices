package graphql_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// ---------------------------------------------------------------------------
// Query: _service
// ---------------------------------------------------------------------------

func TestQuery_Service(t *testing.T) {
	e := newEnv(t)
	res := e.gql(`{ _service { sdl } }`, nil, nil)

	if res.status != 200 {
		t.Errorf("http status = %d, want 200", res.status)
	}
	wantNoErrors(t, res)
	wantStr(t, res.json, graphql.DriverSubgraphSDL, "data", "_service", "sdl")
}

// ---------------------------------------------------------------------------
// Query: driverServiceInfo
// ---------------------------------------------------------------------------

func TestQuery_DriverServiceInfo(t *testing.T) {
	e := newEnv(t)
	res := e.gql(`{ driverServiceInfo { success statusCode message timeStamp data { name version status } } }`,
		nil, nil)

	wantNoErrors(t, res)
	wantBool(t, res.json, true, "data", "driverServiceInfo", "success")
	wantNum(t, res.json, 200, "data", "driverServiceInfo", "statusCode")
	wantStr(t, res.json, "Driver service is operational", "data", "driverServiceInfo", "message")
	wantNotNull(t, res.json, "data", "driverServiceInfo", "timeStamp")
	wantStr(t, res.json, "driver-service", "data", "driverServiceInfo", "data", "name")
	wantStr(t, res.json, "1.0.0", "data", "driverServiceInfo", "data", "version")
	wantStr(t, res.json, "healthy", "data", "driverServiceInfo", "data", "status")
}

// ---------------------------------------------------------------------------
// Query: driver
// ---------------------------------------------------------------------------

func TestQuery_Driver(t *testing.T) {
	t.Run("success with fully populated driver", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))

		res := e.gql(`{ driver(id: "drv-1") { success statusCode message data { `+driverFields+` } } }`,
			nil, nil)
		wantNoErrors(t, res)

		data := "data.driver."
		wantBool(t, res.json, true, data+"success")
		wantNum(t, res.json, 200, data+"statusCode")
		wantStr(t, res.json, "Driver fetched successfully", data+"message")
		wantStr(t, res.json, "drv-1", data+"data", "id")
		wantStr(t, res.json, "user-1", data+"data", "userId")
		wantStr(t, res.json, "OFFLINE", data+"data", "status")
		wantStr(t, res.json, "CAR", data+"data", "vehicleType")
		wantStr(t, res.json, "PLATE-drv-1", data+"data", "plateNumber")
		wantNum(t, res.json, 75, data+"data", "capacityKg")
		wantListLen(t, res.json, 2, data+"data", "capabilities")
		wantStr(t, res.json, "STANDARD", data+"data", "capabilities", "0")
		wantStr(t, res.json, "FRAGILE", data+"data", "capabilities", "1")
		wantStr(t, res.json, "Cairo", data+"data", "serviceArea")
		wantNum(t, res.json, 4.5, data+"data", "rating")
		wantBool(t, res.json, true, data+"data", "isActive")
		wantBool(t, res.json, false, data+"data", "isBlocked")
		wantStr(t, res.json, createdAtRFC, data+"data", "createdAt")
		wantStr(t, res.json, updatedAtRFC, data+"data", "updatedAt")
		wantStr(t, res.json, "user-1", data+"data", "user", "id")
	})

	t.Run("success with driver missing every optional", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newBareDriver("drv-bare"))

		res := e.gql(`{ driver(id: "drv-bare") { success statusCode data { `+driverFields+` } } }`,
			nil, nil)
		wantNoErrors(t, res)

		data := "data.driver."
		wantNum(t, res.json, 200, data+"statusCode")
		wantNull(t, res.json, data+"data", "vehicleType")
		wantNull(t, res.json, data+"data", "plateNumber")
		wantNum(t, res.json, 0, data+"data", "capacityKg")
		wantListLen(t, res.json, 0, data+"data", "capabilities")
		wantNull(t, res.json, data+"data", "serviceArea")
		wantNum(t, res.json, 0, data+"data", "rating")
		wantBool(t, res.json, false, data+"data", "isActive")
		wantBool(t, res.json, false, data+"data", "isBlocked")
		wantNull(t, res.json, data+"data", "createdAt")
		wantNull(t, res.json, data+"data", "updatedAt")
		wantNull(t, res.json, data+"data", "user")
	})

	t.Run("not found", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(`{ driver(id: "missing") { success statusCode message data { id } } }`, nil, nil)

		wantNoErrors(t, res)
		wantBool(t, res.json, false, "data", "driver", "success")
		wantNum(t, res.json, 404, "data", "driver", "statusCode")
		wantStr(t, res.json, "Driver not found", "data", "driver", "message")
		wantNull(t, res.json, "data", "driver", "data")
	})

	t.Run("repository error", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetError(errors.New("db down"))

		res := e.gql(`{ driver(id: "drv-1") { success statusCode message data { id } } }`, nil, nil)

		wantNoErrors(t, res)
		wantBool(t, res.json, false, "data", "driver", "success")
		wantNum(t, res.json, 500, "data", "driver", "statusCode")
		wantStr(t, res.json, "db down", "data", "driver", "message")
		wantNull(t, res.json, "data", "driver", "data")
	})
}

// ---------------------------------------------------------------------------
// Query: myDriverProfile
// ---------------------------------------------------------------------------

func TestQuery_MyDriverProfile(t *testing.T) {
	const q = `{ myDriverProfile { success statusCode message data { id userId status } } }`

	t.Run("missing user context", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(q, nil, nil)

		wantNoErrors(t, res)
		wantBool(t, res.json, false, "data", "myDriverProfile", "success")
		wantNum(t, res.json, 401, "data", "myDriverProfile", "statusCode")
		wantStr(t, res.json, "Unauthorized: missing user context", "data", "myDriverProfile", "message")
		wantNull(t, res.json, "data", "myDriverProfile", "data")
	})

	t.Run("unknown user", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(q, nil, userHeaders("nobody", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "myDriverProfile", "statusCode")
		wantStr(t, res.json, "No driver profile found for this user", "data", "myDriverProfile", "message")
		wantNull(t, res.json, "data", "myDriverProfile", "data")
	})

	t.Run("repository error", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetError(errors.New("db down"))
		res := e.gql(q, nil, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "myDriverProfile", "statusCode")
		wantStr(t, res.json, "db down", "data", "myDriverProfile", "message")
		wantNull(t, res.json, "data", "myDriverProfile", "data")
	})

	t.Run("success", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		res := e.gql(q, nil, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantBool(t, res.json, true, "data", "myDriverProfile", "success")
		wantNum(t, res.json, 200, "data", "myDriverProfile", "statusCode")
		wantStr(t, res.json, "Driver profile fetched successfully", "data", "myDriverProfile", "message")
		wantStr(t, res.json, "drv-1", "data", "myDriverProfile", "data", "id")
		wantStr(t, res.json, "user-1", "data", "myDriverProfile", "data", "userId")
		wantStr(t, res.json, "OFFLINE", "data", "myDriverProfile", "data", "status")
	})
}

// ---------------------------------------------------------------------------
// Query: driverActiveAssignment
// ---------------------------------------------------------------------------

func TestQuery_DriverActiveAssignment(t *testing.T) {
	const q = `query($driverId: ID) {
		driverActiveAssignment(driverId: $driverId) {
			success statusCode message data { id driverId deliveryId status }
		}
	}`

	tests := []struct {
		name        string
		headers     map[string]string
		vars        map[string]interface{}
		setup       func(*env)
		wantCode    float64
		wantMessage string
		wantDataNil bool
		wantDataID  string
	}{
		{
			name:        "no driverId and no user context",
			headers:     nil,
			vars:        map[string]interface{}{"driverId": nil},
			wantCode:    401,
			wantMessage: "Unauthorized: driver profile not found for authenticated user",
			wantDataNil: true,
		},
		{
			name:    "no driverId, driver without assignment",
			headers: userHeaders("user-1", ""),
			vars:    map[string]interface{}{"driverId": nil},
			setup: func(e *env) {
				e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
			},
			wantCode:    200,
			wantMessage: "No active assignment found for driver",
			wantDataNil: true,
		},
		{
			name:    "no driverId, active assignment",
			headers: userHeaders("user-1", ""),
			vars:    map[string]interface{}{"driverId": nil},
			setup: func(e *env) {
				e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
				e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
			},
			wantCode:   200,
			wantDataID: "a-1",
		},
		{
			name:    "driverId of missing target driver",
			headers: userHeaders("user-1", ""),
			vars:    map[string]interface{}{"driverId": "drv-missing"},
			wantCode:    403,
			wantMessage: "Forbidden: Only the driver or admin can view this assignment",
			wantDataNil: true,
		},
		{
			name:    "driverId owned by another user",
			headers: userHeaders("user-1", ""),
			vars:    map[string]interface{}{"driverId": "drv-2"},
			setup: func(e *env) {
				e.mockDrivers.SetDriver(newRichDriver("drv-2", "user-2"))
			},
			wantCode:    403,
			wantMessage: "Forbidden: Only the driver or admin can view this assignment",
			wantDataNil: true,
		},
		{
			name:    "driverId owned by caller",
			headers: userHeaders("user-1", ""),
			vars:    map[string]interface{}{"driverId": "drv-1"},
			setup: func(e *env) {
				e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
				e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
			},
			wantCode:   200,
			wantDataID: "a-1",
		},
		{
			name:    "admin bypasses ownership check",
			headers: userHeaders("someone", "admin"),
			vars:    map[string]interface{}{"driverId": "drv-ghost"},
			wantCode:    200,
			wantMessage: "No active assignment found for driver",
			wantDataNil: true,
		},
		{
			name:    "empty driverId falls back to caller identity",
			headers: userHeaders("user-1", ""),
			vars:    map[string]interface{}{"driverId": ""},
			setup: func(e *env) {
				e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
			},
			wantCode:    200,
			wantMessage: "No active assignment found for driver",
			wantDataNil: true,
		},
		{
			name:    "assignment lookup error",
			headers: userHeaders("user-1", ""),
			vars:    map[string]interface{}{"driverId": nil},
			setup: func(e *env) {
				e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
			},
			wantCode:    500,
			wantMessage: "assignment lookup failed",
			wantDataNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []envOption
			if tt.name == "assignment lookup error" {
				opts = append(opts, withAssignmentLookupErrors(errors.New("assignment lookup failed"), nil))
			}
			e := newEnv(t, opts...)
			if tt.setup != nil {
				tt.setup(e)
			}
			res := e.gql(q, tt.vars, tt.headers)

			wantNoErrors(t, res)
			base := "data.driverActiveAssignment."
			wantNum(t, res.json, tt.wantCode, base+"statusCode")
			if tt.wantMessage != "" {
				wantStr(t, res.json, tt.wantMessage, base+"message")
			}
			if tt.wantDataNil {
				wantNull(t, res.json, base+"data")
			} else {
				wantStr(t, res.json, tt.wantDataID, base+"data", "id")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Query: driverStatus
// ---------------------------------------------------------------------------

func TestQuery_DriverStatus(t *testing.T) {
	const q = `query($id: ID!) {
		driverStatus(driverId: $id) {
			success statusCode message
			data { status hasActiveAssignment activeDelivery { id } lastSeenAt driver { id status } }
		}
	}`

	t.Run("repository error", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetError(errors.New("db down"))
		res := e.gql(q, map[string]interface{}{"id": "drv-1"}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "driverStatus", "statusCode")
		wantStr(t, res.json, "db down", "data", "driverStatus", "message")
		wantNull(t, res.json, "data", "driverStatus", "data")
	})

	t.Run("not found", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(q, map[string]interface{}{"id": "missing"}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "driverStatus", "statusCode")
		wantStr(t, res.json, "Driver not found", "data", "driverStatus", "message")
		wantNull(t, res.json, "data", "driverStatus", "data")
	})

	t.Run("forbidden for other users", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-2"))
		res := e.gql(q, map[string]interface{}{"id": "drv-1"}, userHeaders("user-1", "driver"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "driverStatus", "statusCode")
		wantStr(t, res.json, "Forbidden: Only the driver or admin can view driver operational status",
			"data", "driverStatus", "message")
		wantNull(t, res.json, "data", "driverStatus", "data")
	})

	t.Run("admin without active assignment and without last seen", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newBareDriver("drv-bare"))
		res := e.gql(q, map[string]interface{}{"id": "drv-bare"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		base := "data.driverStatus."
		wantNum(t, res.json, 200, base+"statusCode")
		wantBool(t, res.json, false, base+"data", "hasActiveAssignment")
		wantNull(t, res.json, base+"data", "activeDelivery")
		wantNull(t, res.json, base+"data", "lastSeenAt")
		wantStr(t, res.json, "OFFLINE", base+"data", "status")
		wantStr(t, res.json, "drv-bare", base+"data", "driver", "id")
	})

	t.Run("owner with active assignment and last seen", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
		res := e.gql(q, map[string]interface{}{"id": "drv-1"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		base := "data.driverStatus."
		wantNum(t, res.json, 200, base+"statusCode")
		wantBool(t, res.json, true, base+"data", "hasActiveAssignment")
		wantStr(t, res.json, "del-1", base+"data", "activeDelivery", "id")
		wantStr(t, res.json, updatedAtRFC, base+"data", "lastSeenAt")
		wantStr(t, res.json, "OFFLINE", base+"data", "status")
	})
}

// ---------------------------------------------------------------------------
// Query: nearbyDrivers
// ---------------------------------------------------------------------------

const nearbyQuery = `query($input: NearbyDriversInput!) {
		nearbyDrivers(input: $input) {
			success statusCode message
			data {
				paginationInfo { totalItems currentPage nextPage }
				items {
					driverId distanceMeters status vehicleType latitude longitude rating isBlocked
					driver { id rating isBlocked }
				}
			}
		}
	}`

func seedNearbyDrivers(e *env) {
	truck := newAvailableDriver("drv-b", "user-b")
	truck.Vehicle.Type = domain.VehicleTypeTruck
	moto := newAvailableDriver("drv-c", "user-c")
	moto.Vehicle.Type = domain.VehicleTypeMotorcycle
	e.mockDrivers.SetDriver(newAvailableDriver("drv-a", "user-a"))
	e.mockDrivers.SetDriver(truck)
	e.mockDrivers.SetDriver(moto)
	e.locationStore.SetLocation("drv-a", 31.0, 30.0, 100)
	e.locationStore.SetLocation("drv-b", 31.0, 30.0, 250)
	e.locationStore.SetLocation("drv-c", 31.0, 30.0, 50)
}

func nearbyInput(vehicleType interface{}, limit interface{}) map[string]interface{} {
	input := map[string]interface{}{
		"latitude":  30.0,
		"longitude": 31.0,
		"radiusKm":  5.0,
	}
	if vehicleType != nil {
		input["vehicleType"] = vehicleType
	}
	if limit != nil {
		input["limit"] = limit
	}
	return input
}

func TestQuery_NearbyDrivers(t *testing.T) {
	t.Run("dispatch service with loaders and limit truncation", func(t *testing.T) {
		e := newEnv(t)
		seedNearbyDrivers(e)
		res := e.gql(nearbyQuery,
			map[string]interface{}{"input": nearbyInput(nil, 2)}, nil)
		wantNoErrors(t, res)

		base := "data.nearbyDrivers."
		wantNum(t, res.json, 200, base+"statusCode")
		wantNum(t, res.json, 3, base+"data", "paginationInfo", "totalItems")
		wantNum(t, res.json, 1, base+"data", "paginationInfo", "currentPage")
		wantNull(t, res.json, base+"data", "paginationInfo", "nextPage")
		// Ranking sorts ascending by distance: 50m (drv-c), 100m (drv-a); drv-b is cut.
		wantListLen(t, res.json, 2, base+"data", "items")
		wantStr(t, res.json, "drv-c", base+"data", "items", "0", "driverId")
		wantNum(t, res.json, 50, base+"data", "items", "0", "distanceMeters")
		wantStr(t, res.json, "MOTORCYCLE", base+"data", "items", "0", "vehicleType")
		wantStr(t, res.json, "AVAILABLE", base+"data", "items", "0", "status")
		wantNum(t, res.json, 30.0, base+"data", "items", "0", "latitude")
		wantNum(t, res.json, 31.0, base+"data", "items", "0", "longitude")
		wantNum(t, res.json, 4.5, base+"data", "items", "0", "rating")
		wantBool(t, res.json, false, base+"data", "items", "0", "isBlocked")
		wantStr(t, res.json, "drv-c", base+"data", "items", "0", "driver", "id")
		wantStr(t, res.json, "drv-a", base+"data", "items", "1", "driverId")
	})

	t.Run("vehicle type filter", func(t *testing.T) {
		e := newEnv(t)
		seedNearbyDrivers(e)
		res := e.gql(nearbyQuery,
			map[string]interface{}{"input": nearbyInput("TRUCK", nil)}, nil)
		wantNoErrors(t, res)

		base := "data.nearbyDrivers."
		wantListLen(t, res.json, 1, base+"data", "items")
		wantStr(t, res.json, "drv-b", base+"data", "items", "0", "driverId")
		wantNum(t, res.json, 1, base+"data", "paginationInfo", "totalItems")
	})

	t.Run("zero and negative limits fall back to the default of 50", func(t *testing.T) {
		for _, limit := range []interface{}{0, -1} {
			e := newEnv(t)
			seedNearbyDrivers(e)
			res := e.gql(nearbyQuery,
				map[string]interface{}{"input": nearbyInput(nil, limit)}, nil)
			wantNoErrors(t, res)
			wantListLen(t, res.json, 3, "data", "nearbyDrivers", "data", "items")
		}
	})

	t.Run("without loaders falls back to per-item repo lookups", func(t *testing.T) {
		e := newEnv(t, withoutLoaders())
		seedNearbyDrivers(e)
		res := e.gql(nearbyQuery,
			map[string]interface{}{"input": nearbyInput(nil, nil)}, nil)
		wantNoErrors(t, res)

		base := "data.nearbyDrivers."
		wantListLen(t, res.json, 3, base+"data", "items")
		wantStr(t, res.json, "drv-c", base+"data", "items", "0", "driver", "id")
	})

	t.Run("dispatch service repository error", func(t *testing.T) {
		e := newEnv(t)
		seedNearbyDrivers(e)
		e.mockDrivers.SetError(errors.New("db down"))
		res := e.gql(nearbyQuery,
			map[string]interface{}{"input": nearbyInput(nil, nil)}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "nearbyDrivers", "statusCode")
		wantStr(t, res.json, "db down", "data", "nearbyDrivers", "message")
		wantNull(t, res.json, "data", "nearbyDrivers", "data")
	})

	t.Run("without dispatch service uses the repository query", func(t *testing.T) {
		e := newEnv(t, withoutDispatch())
		seedNearbyDrivers(e)
		res := e.gql(nearbyQuery,
			map[string]interface{}{"input": nearbyInput(nil, nil)}, nil)
		wantNoErrors(t, res)

		base := "data.nearbyDrivers."
		wantNum(t, res.json, 200, base+"statusCode")
		wantListLen(t, res.json, 3, base+"data", "items")
		// The repository path does not rank by distance, so only the set of
		// drivers is deterministic.
		seen := map[string]bool{}
		for i := 0; i < 3; i++ {
			v, ok := walk(res.json, []string{base + "data", "items", strconv.Itoa(i), "driver", "id"})
			if !ok {
				t.Fatalf("path %sdata.items.%d.driver.id not found in %v", base, i, res.json)
			}
			id, _ := v.(string)
			seen[id] = true
		}
		for _, id := range []string{"drv-a", "drv-b", "drv-c"} {
			if !seen[id] {
				t.Errorf("driver %s missing from nearbyDrivers items: %v", id, seen)
			}
		}
	})

	t.Run("without dispatch service repository error", func(t *testing.T) {
		e := newEnv(t, withoutDispatch())
		seedNearbyDrivers(e)
		e.mockDrivers.SetError(errors.New("geo down"))
		res := e.gql(nearbyQuery,
			map[string]interface{}{"input": nearbyInput(nil, nil)}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "nearbyDrivers", "statusCode")
		wantStr(t, res.json, "geo down", "data", "nearbyDrivers", "message")
		wantNull(t, res.json, "data", "nearbyDrivers", "data")
	})

	t.Run("candidate that cannot be loaded is tolerated", func(t *testing.T) {
		ghost := &domain.Driver{
			ID:       "drv-ghost",
			Status:   domain.DriverStatusAvailable,
			IsActive: true,
			Vehicle:  domain.VehicleInfo{Type: domain.VehicleTypeVan},
		}
		e := newEnv(t, withoutDispatch(), withGhostDriver(ghost))
		res := e.gql(nearbyQuery,
			map[string]interface{}{"input": nearbyInput(nil, nil)}, nil)
		wantNoErrors(t, res)

		base := "data.nearbyDrivers."
		wantNum(t, res.json, 200, base+"statusCode")
		wantListLen(t, res.json, 1, base+"data", "items")
		wantStr(t, res.json, "", base+"data", "items", "0", "driverId")
		wantNull(t, res.json, base+"data", "items", "0", "driver")
		wantNum(t, res.json, 0, base+"data", "items", "0", "rating")
		wantBool(t, res.json, false, base+"data", "items", "0", "isBlocked")
		wantStr(t, res.json, "VAN", base+"data", "items", "0", "vehicleType")
	})
}

// ---------------------------------------------------------------------------
// Query: assignment
// ---------------------------------------------------------------------------

const assignmentQuery = `query($id: ID!) {
		assignment(id: $id) {
			success statusCode message
			data {
				id deliveryId driverId status attemptNumber
				offeredAt expiresAt acceptedAt rejectedAt completedAt createdAt updatedAt
				delivery { id }
				driver { id userId }
			}
		}
	}`

func TestQuery_Assignment(t *testing.T) {
	t.Run("repository error", func(t *testing.T) {
		e := newEnv(t)
		e.mockAssignments.SetError(errors.New("db down"))
		res := e.gql(assignmentQuery, map[string]interface{}{"id": "a-1"}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "assignment", "statusCode")
		wantStr(t, res.json, "db down", "data", "assignment", "message")
		wantNull(t, res.json, "data", "assignment", "data")
	})

	t.Run("not found", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(assignmentQuery, map[string]interface{}{"id": "missing"}, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 404, "data", "assignment", "statusCode")
		wantStr(t, res.json, "Assignment not found", "data", "assignment", "message")
		wantNull(t, res.json, "data", "assignment", "data")
	})

	t.Run("forbidden when assignment driver is unknown", func(t *testing.T) {
		e := newEnv(t)
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-ghost", "del-1"))
		res := e.gql(assignmentQuery, map[string]interface{}{"id": "a-1"},
			userHeaders("user-1", "driver"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "assignment", "statusCode")
		wantStr(t, res.json, "Forbidden: Only the assigned driver or admin can view this assignment",
			"data", "assignment", "message")
		wantNull(t, res.json, "data", "assignment", "data")
	})

	t.Run("forbidden when assignment belongs to another user", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-2", "user-2"))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-2", "del-1"))
		res := e.gql(assignmentQuery, map[string]interface{}{"id": "a-1"},
			userHeaders("user-1", "driver"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "assignment", "statusCode")
		wantNull(t, res.json, "data", "assignment", "data")
	})

	t.Run("owner resolves driver through the loader", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
		res := e.gql(assignmentQuery, map[string]interface{}{"id": "a-1"},
			userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.assignment."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Assignment fetched successfully", base+"message")
		wantStr(t, res.json, "a-1", base+"data", "id")
		wantStr(t, res.json, "drv-1", base+"data", "driverId")
		wantStr(t, res.json, "del-1", base+"data", "deliveryId")
		wantStr(t, res.json, "OFFERED", base+"data", "status")
		wantNum(t, res.json, 2, base+"data", "attemptNumber")
		wantStr(t, res.json, offeredAtRFC, base+"data", "offeredAt")
		wantStr(t, res.json, expiresAtRFC, base+"data", "expiresAt")
		wantStr(t, res.json, createdAtRFC, base+"data", "createdAt")
		wantStr(t, res.json, updatedAtRFC, base+"data", "updatedAt")
		wantNull(t, res.json, base+"data", "acceptedAt")
		wantNull(t, res.json, base+"data", "rejectedAt")
		wantNull(t, res.json, base+"data", "completedAt")
		wantStr(t, res.json, "del-1", base+"data", "delivery", "id")
		wantStr(t, res.json, "drv-1", base+"data", "driver", "id")
		wantStr(t, res.json, "user-1", base+"data", "driver", "userId")
	})

	t.Run("admin sees optional timestamps", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockAssignments.SetAssignment(newStampedAssignment("a-2", "drv-1", "del-2"))
		res := e.gql(assignmentQuery, map[string]interface{}{"id": "a-2"},
			userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.assignment.data."
		wantNum(t, res.json, 200, "data", "assignment", "statusCode")
		wantStr(t, res.json, acceptedAtRFC, base, "acceptedAt")
		wantStr(t, res.json, rejectedAtRFC, base, "rejectedAt")
		wantStr(t, res.json, completedAtRFC, base, "completedAt")
	})

	t.Run("without loaders returns an ID-only driver stub", func(t *testing.T) {
		e := newEnv(t, withoutLoaders())
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
		res := e.gql(assignmentQuery, map[string]interface{}{"id": "a-1"},
			userHeaders("user-1", ""))
		wantNoErrors(t, res)

		wantStr(t, res.json, "a-1", "data", "assignment", "data", "id")
		wantStr(t, res.json, "drv-1", "data", "assignment", "data", "driver", "id")
		wantStr(t, res.json, "", "data", "assignment", "data", "driver", "userId")
	})

	t.Run("loader error surfaces as a GraphQL error", func(t *testing.T) {
		e := newEnv(t, withBatchLoadError(errors.New("batch boom")))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
		res := e.gql(assignmentQuery, map[string]interface{}{"id": "a-1"},
			userHeaders("", "admin"))
		wantSomeErrors(t, res)
	})
}

// ---------------------------------------------------------------------------
// Query: dispatchAttempts
// ---------------------------------------------------------------------------

const dispatchAttemptsQuery = `query($deliveryId: ID!) {
		dispatchAttempts(deliveryId: $deliveryId) {
			success statusCode message
			data {
				paginationInfo { totalItems currentPage nextPage }
				items {
					id deliveryId driverId distanceMeters attemptNumber result reason createdAt
					delivery { id }
					driver { id userId }
				}
			}
		}
	}`

func TestQuery_DispatchAttempts(t *testing.T) {
	t.Run("forbidden for non-admins", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(dispatchAttemptsQuery, map[string]interface{}{"deliveryId": "del-1"},
			userHeaders("user-1", "driver"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "dispatchAttempts", "statusCode")
		wantStr(t, res.json, "Forbidden: Only admin can view dispatch attempts",
			"data", "dispatchAttempts", "message")
		wantNull(t, res.json, "data", "dispatchAttempts", "data")
	})

	t.Run("assignment lookup error", func(t *testing.T) {
		e := newEnv(t, withAssignmentLookupErrors(nil, errors.New("delivery lookup failed")))
		res := e.gql(dispatchAttemptsQuery, map[string]interface{}{"deliveryId": "del-1"},
			userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "dispatchAttempts", "statusCode")
		wantStr(t, res.json, "delivery lookup failed", "data", "dispatchAttempts", "message")
		wantNull(t, res.json, "data", "dispatchAttempts", "data")
	})

	t.Run("no assignment for delivery", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(dispatchAttemptsQuery, map[string]interface{}{"deliveryId": "missing"},
			userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.dispatchAttempts."
		wantNum(t, res.json, 200, base+"statusCode")
		wantListLen(t, res.json, 0, base+"data", "items")
		wantNum(t, res.json, 0, base+"data", "paginationInfo", "totalItems")
		wantNum(t, res.json, 1, base+"data", "paginationInfo", "currentPage")
		wantNull(t, res.json, base+"data", "paginationInfo", "nextPage")
	})

	t.Run("admin resolves attempt with driver through loader", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
		res := e.gql(dispatchAttemptsQuery, map[string]interface{}{"deliveryId": "del-1"},
			userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.dispatchAttempts."
		wantNum(t, res.json, 200, base+"statusCode")
		wantNum(t, res.json, 1, base+"data", "paginationInfo", "totalItems")
		wantListLen(t, res.json, 1, base+"data", "items")
		item := base + "data.items.0."
		wantStr(t, res.json, "a-1", item+"id")
		wantStr(t, res.json, "del-1", item+"deliveryId")
		wantStr(t, res.json, "drv-1", item+"driverId")
		wantNum(t, res.json, 0, item+"distanceMeters")
		wantNum(t, res.json, 2, item+"attemptNumber")
		wantStr(t, res.json, "OFFERED", item+"result")
		wantStr(t, res.json, "", item+"reason")
		wantStr(t, res.json, createdAtRFC, item+"createdAt")
		wantStr(t, res.json, "del-1", item+"delivery", "id")
		wantStr(t, res.json, "drv-1", item+"driver", "id")
		wantStr(t, res.json, "user-1", item+"driver", "userId")
	})

	t.Run("without loaders returns an ID-only driver stub", func(t *testing.T) {
		e := newEnv(t, withoutLoaders())
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
		res := e.gql(dispatchAttemptsQuery, map[string]interface{}{"deliveryId": "del-1"},
			userHeaders("", "admin"))
		wantNoErrors(t, res)

		wantStr(t, res.json, "drv-1", "data", "dispatchAttempts", "data", "items", "0", "driver", "id")
		wantStr(t, res.json, "", "data", "dispatchAttempts", "data", "items", "0", "driver", "userId")
	})

	t.Run("loader error surfaces as a GraphQL error", func(t *testing.T) {
		e := newEnv(t, withBatchLoadError(errors.New("batch boom")))
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))
		res := e.gql(dispatchAttemptsQuery, map[string]interface{}{"deliveryId": "del-1"},
			userHeaders("", "admin"))
		wantSomeErrors(t, res)
	})
}

// ---------------------------------------------------------------------------
// Query: driverReviews
// ---------------------------------------------------------------------------

const reviewsQuery = `query($driverId: ID!, $page: Int, $limit: Int) {
		driverReviews(driverId: $driverId, page: $page, limit: $limit) {
			success statusCode message
			data {
				averageRating totalReviews
				paginationInfo { totalItems currentPage nextPage }
				items {
					id driverId userId deliveryId rating comment createdAt
					user { id }
					delivery { id }
					driver { id userId }
				}
			}
		}
	}`

func seedReviewDriver(e *env) {
	d := newRichDriver("drv-1", "user-1")
	d.Rating = 4.5
	d.RatingCount = 3
	e.mockDrivers.SetDriver(d)
	e.reviewRepo.SetReview(newReview("rev-1", "drv-1", "user-c", "del-1", 5, "great"))
	e.reviewRepo.SetReview(newReview("rev-2", "drv-1", "user-c", "del-2", 4, ""))
	e.reviewRepo.SetReview(newReview("rev-3", "drv-1", "user-c", "del-3", 3, "ok"))
}

func TestQuery_DriverReviews(t *testing.T) {
	t.Run("forbidden when driver is unknown", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(reviewsQuery, map[string]interface{}{"driverId": "missing"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "driverReviews", "statusCode")
		wantStr(t, res.json, "Forbidden: Only the driver or admin can view these reviews",
			"data", "driverReviews", "message")
		wantNull(t, res.json, "data", "driverReviews", "data")
	})

	t.Run("forbidden when driver belongs to another user", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-2"))
		res := e.gql(reviewsQuery, map[string]interface{}{"driverId": "drv-1"}, userHeaders("user-1", ""))

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "driverReviews", "statusCode")
		wantNull(t, res.json, "data", "driverReviews", "data")
	})

	t.Run("owner may read reviews", func(t *testing.T) {
		e := newEnv(t)
		seedReviewDriver(e)
		res := e.gql(reviewsQuery, map[string]interface{}{"driverId": "drv-1"}, userHeaders("user-1", ""))
		wantNoErrors(t, res)
		wantNum(t, res.json, 200, "data", "driverReviews", "statusCode")
	})

	t.Run("driver lookup failure for admins", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(reviewsQuery, map[string]interface{}{"driverId": "missing"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "driverReviews", "statusCode")
		wantContains(t, res.json, "driver not found", "data", "driverReviews", "message")
		wantNull(t, res.json, "data", "driverReviews", "data")
	})

	t.Run("repository error", func(t *testing.T) {
		e := newEnv(t)
		seedReviewDriver(e)
		e.mockDrivers.SetError(errors.New("db down"))
		res := e.gql(reviewsQuery, map[string]interface{}{"driverId": "drv-1"}, userHeaders("", "admin"))

		wantNoErrors(t, res)
		wantNum(t, res.json, 500, "data", "driverReviews", "statusCode")
		wantStr(t, res.json, "db down", "data", "driverReviews", "message")
		wantNull(t, res.json, "data", "driverReviews", "data")
	})

	t.Run("default pagination returns every review", func(t *testing.T) {
		e := newEnv(t)
		seedReviewDriver(e)
		res := e.gql(reviewsQuery, map[string]interface{}{"driverId": "drv-1"}, userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.driverReviews."
		wantNum(t, res.json, 200, base+"statusCode")
		wantNum(t, res.json, 4.5, base+"data", "averageRating")
		wantNum(t, res.json, 3, base+"data", "totalReviews")
		wantNum(t, res.json, 3, base+"data", "paginationInfo", "totalItems")
		wantNum(t, res.json, 1, base+"data", "paginationInfo", "currentPage")
		wantNull(t, res.json, base+"data", "paginationInfo", "nextPage")
		wantListLen(t, res.json, 3, base+"data", "items")

		item := base + "data.items.0."
		wantStr(t, res.json, "rev-1", item+"id")
		wantStr(t, res.json, "drv-1", item+"driverId")
		wantStr(t, res.json, "user-c", item+"userId")
		wantStr(t, res.json, "del-1", item+"deliveryId")
		wantNum(t, res.json, 5, item+"rating")
		wantStr(t, res.json, "great", item+"comment")
		wantStr(t, res.json, reviewAtRFC, item+"createdAt")
		wantStr(t, res.json, "user-c", item+"user", "id")
		wantStr(t, res.json, "del-1", item+"delivery", "id")
		wantStr(t, res.json, "drv-1", item+"driver", "id")
		wantStr(t, res.json, "user-1", item+"driver", "userId")
		// The middle review carries an empty comment -> null branch.
		wantNull(t, res.json, base+"data", "items", "1", "comment")
	})

	t.Run("page 0 and negative limit fall back to defaults", func(t *testing.T) {
		e := newEnv(t)
		seedReviewDriver(e)
		res := e.gql(reviewsQuery,
			map[string]interface{}{"driverId": "drv-1", "page": 0, "limit": -1},
			userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.driverReviews."
		wantNum(t, res.json, 1, base+"data", "paginationInfo", "currentPage")
		wantNull(t, res.json, base+"data", "paginationInfo", "nextPage")
		wantListLen(t, res.json, 3, base+"data", "items")
	})

	t.Run("first page with limit 1 announces a next page", func(t *testing.T) {
		e := newEnv(t)
		seedReviewDriver(e)
		res := e.gql(reviewsQuery,
			map[string]interface{}{"driverId": "drv-1", "page": 1, "limit": 1},
			userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.driverReviews."
		wantNum(t, res.json, 1, base+"data", "paginationInfo", "currentPage")
		wantNum(t, res.json, 2, base+"data", "paginationInfo", "nextPage")
		wantListLen(t, res.json, 1, base+"data", "items")
		wantStr(t, res.json, "rev-1", base+"data", "items", "0", "id")
	})

	t.Run("second page with limit 1 continues pagination", func(t *testing.T) {
		e := newEnv(t)
		seedReviewDriver(e)
		res := e.gql(reviewsQuery,
			map[string]interface{}{"driverId": "drv-1", "page": 2, "limit": 1},
			userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.driverReviews."
		wantNum(t, res.json, 2, base+"data", "paginationInfo", "currentPage")
		wantNum(t, res.json, 3, base+"data", "paginationInfo", "nextPage")
		wantListLen(t, res.json, 1, base+"data", "items")
		wantStr(t, res.json, "rev-2", base+"data", "items", "0", "id")
	})

	t.Run("limit above the maximum is clamped by the query handler", func(t *testing.T) {
		e := newEnv(t)
		seedReviewDriver(e)
		res := e.gql(reviewsQuery,
			map[string]interface{}{"driverId": "drv-1", "limit": 100},
			userHeaders("", "admin"))
		wantNoErrors(t, res)

		base := "data.driverReviews."
		wantNum(t, res.json, 1, base+"data", "paginationInfo", "currentPage")
		wantNull(t, res.json, base+"data", "paginationInfo", "nextPage")
		wantListLen(t, res.json, 3, base+"data", "items")
	})

	t.Run("without loaders returns an ID-only driver stub", func(t *testing.T) {
		e := newEnv(t, withoutLoaders())
		seedReviewDriver(e)
		res := e.gql(reviewsQuery, map[string]interface{}{"driverId": "drv-1"}, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.driverReviews."
		wantStr(t, res.json, "drv-1", base+"data", "items", "0", "driver", "id")
		wantStr(t, res.json, "", base+"data", "items", "0", "driver", "userId")
	})

	t.Run("loader error surfaces as a GraphQL error", func(t *testing.T) {
		e := newEnv(t, withBatchLoadError(errors.New("batch boom")))
		seedReviewDriver(e)
		res := e.gql(reviewsQuery, map[string]interface{}{"driverId": "drv-1"}, userHeaders("", "admin"))
		wantSomeErrors(t, res)
	})
}

// ---------------------------------------------------------------------------
// Query: availableDeliveries
// ---------------------------------------------------------------------------

const openDeliveriesQuery = `{ availableDeliveries {
		success statusCode message
		data {
			totalItems
			items { id customerId status pickupCity pickupCountry dropoffCity dropoffCountry amount currency createdAt }
		}
	}
}`

func TestQuery_AvailableDeliveries(t *testing.T) {
	seedActive := func(e *env) {
		e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))
	}

	t.Run("forbidden without user context", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(openDeliveriesQuery, nil, nil)

		wantNoErrors(t, res)
		wantNum(t, res.json, 403, "data", "availableDeliveries", "statusCode")
		wantNull(t, res.json, "data", "availableDeliveries", "data")
	})

	t.Run("nil delivery client yields an empty page", func(t *testing.T) {
		e := newEnv(t, withoutDeliveryClient())
		seedActive(e)
		res := e.gql(openDeliveriesQuery, nil, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.availableDeliveries."
		wantNum(t, res.json, 200, base+"statusCode")
		wantNum(t, res.json, 0, base+"data", "totalItems")
		wantListLen(t, res.json, 0, base+"data", "items")
	})

	t.Run("mock client without deliveries yields an empty page", func(t *testing.T) {
		e := newEnv(t)
		seedActive(e)
		res := e.gql(openDeliveriesQuery, nil, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.availableDeliveries."
		wantNum(t, res.json, 200, base+"statusCode")
		wantStr(t, res.json, "Open deliveries retrieved successfully", base+"message")
		wantNum(t, res.json, 0, base+"data", "totalItems")
		wantListLen(t, res.json, 0, base+"data", "items")
	})

	t.Run("client error is swallowed", func(t *testing.T) {
		e := newEnv(t, withOpenDeliveries(nil, 0, errors.New("delivery service down")))
		seedActive(e)
		res := e.gql(openDeliveriesQuery, nil, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.availableDeliveries."
		wantNum(t, res.json, 200, base+"statusCode")
		wantNum(t, res.json, 0, base+"data", "totalItems")
		wantListLen(t, res.json, 0, base+"data", "items")
	})

	t.Run("populated and sparse deliveries", func(t *testing.T) {
		full := &ports.OpenDeliveryInfo{
			ID: "del-full", CustomerID: "cust-1", Status: "PENDING",
			PickupCity: "Cairo", PickupCountry: "EG",
			DropoffCity: "Alex", DropoffCountry: "EG",
			Amount: "120.50", Currency: "EGP", CreatedAt: createdAtRFC,
		}
		sparse := &ports.OpenDeliveryInfo{ID: "del-sparse", CustomerID: "cust-2", Status: "PENDING"}
		e := newEnv(t, withOpenDeliveries([]*ports.OpenDeliveryInfo{full, sparse}, 7, nil))
		seedActive(e)
		res := e.gql(openDeliveriesQuery, nil, userHeaders("user-1", ""))
		wantNoErrors(t, res)

		base := "data.availableDeliveries."
		wantNum(t, res.json, 7, base+"data", "totalItems")
		wantListLen(t, res.json, 2, base+"data", "items")

		first := base + "data.items.0."
		wantStr(t, res.json, "del-full", first+"id")
		wantStr(t, res.json, "cust-1", first+"customerId")
		wantStr(t, res.json, "PENDING", first+"status")
		wantStr(t, res.json, "Cairo", first+"pickupCity")
		wantStr(t, res.json, "EG", first+"pickupCountry")
		wantStr(t, res.json, "Alex", first+"dropoffCity")
		wantStr(t, res.json, "EG", first+"dropoffCountry")
		wantStr(t, res.json, "120.50", first+"amount")
		wantStr(t, res.json, "EGP", first+"currency")
		wantStr(t, res.json, createdAtRFC, first+"createdAt")

		second := base + "data.items.1."
		wantStr(t, res.json, "del-sparse", second+"id")
		wantNull(t, res.json, second, "pickupCity")
		wantNull(t, res.json, second, "pickupCountry")
		wantNull(t, res.json, second, "dropoffCity")
		wantNull(t, res.json, second, "dropoffCountry")
		wantNull(t, res.json, second, "amount")
		wantNull(t, res.json, second, "currency")
		wantNull(t, res.json, second, "createdAt")
	})
}
