package graphql_test

import (
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// The tests below close the remaining gaps in internal/graphql: response
// timeStamp accessors that no existing document selects, the zero-value
// timestamp branches of AssignmentResolver, and the defensive nil branches of
// the status / service-info resolvers.

// ---------------------------------------------------------------------------
// timeStamp on every response type
// ---------------------------------------------------------------------------

func TestQuery_ResponseTimeStamps(t *testing.T) {
	t.Run("driver", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))

		res := e.gql(`{ driver(id: "drv-1") { success statusCode timeStamp data { id } } }`, nil, nil)
		wantNoErrors(t, res)
		wantBool(t, res.json, true, "data", "driver", "success")
		wantNotNull(t, res.json, "data", "driver", "timeStamp")
	})

	t.Run("driverStatus", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))

		res := e.gql(`query($id: ID!) {
			driverStatus(driverId: $id) { success statusCode timeStamp data { status } }
		}`, map[string]interface{}{"id": "drv-1"}, userHeaders("", "admin"))
		wantNoErrors(t, res)
		wantBool(t, res.json, true, "data", "driverStatus", "success")
		wantNotNull(t, res.json, "data", "driverStatus", "timeStamp")
	})

	t.Run("assignment", func(t *testing.T) {
		e := newEnv(t)
		e.mockAssignments.SetAssignment(newOfferedAssignment("a-1", "drv-1", "del-1"))

		res := e.gql(`query($id: ID!) {
			assignment(id: $id) { success statusCode timeStamp data { id } }
		}`, map[string]interface{}{"id": "a-1"}, userHeaders("", "admin"))
		wantNoErrors(t, res)
		wantBool(t, res.json, true, "data", "assignment", "success")
		wantNotNull(t, res.json, "data", "assignment", "timeStamp")
	})

	t.Run("nearbyDrivers", func(t *testing.T) {
		e := newEnv(t)
		seedNearbyDrivers(e)

		res := e.gql(`query($input: NearbyDriversInput!) {
			nearbyDrivers(input: $input) {
				success statusCode timeStamp
				data { paginationInfo { totalItems } items { driverId } }
			}
		}`, map[string]interface{}{"input": nearbyInput(nil, 2)}, nil)
		wantNoErrors(t, res)
		wantBool(t, res.json, true, "data", "nearbyDrivers", "success")
		wantNotNull(t, res.json, "data", "nearbyDrivers", "timeStamp")
	})

	t.Run("dispatchAttempts", func(t *testing.T) {
		e := newEnv(t)

		res := e.gql(`query($deliveryId: ID!) {
			dispatchAttempts(deliveryId: $deliveryId) {
				success statusCode timeStamp
				data { paginationInfo { totalItems } items { id } }
			}
		}`, map[string]interface{}{"deliveryId": "del-1"}, userHeaders("", "admin"))
		wantNoErrors(t, res)
		wantBool(t, res.json, true, "data", "dispatchAttempts", "success")
		wantNotNull(t, res.json, "data", "dispatchAttempts", "timeStamp")
	})

	t.Run("driverReviews", func(t *testing.T) {
		e := newEnv(t)
		seedReviewDriver(e)

		res := e.gql(`query($driverId: ID!) {
			driverReviews(driverId: $driverId) {
				success statusCode timeStamp
				data { averageRating totalReviews items { id } }
			}
		}`, map[string]interface{}{"driverId": "drv-1"}, userHeaders("", "admin"))
		wantNoErrors(t, res)
		wantBool(t, res.json, true, "data", "driverReviews", "success")
		wantNotNull(t, res.json, "data", "driverReviews", "timeStamp")
	})

	t.Run("availableDeliveries", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newAvailableDriver("drv-1", "user-1"))

		res := e.gql(`{
			availableDeliveries { success statusCode timeStamp data { totalItems items { id } } }
		}`, nil, userHeaders("user-1", ""))
		wantNoErrors(t, res)
		wantBool(t, res.json, true, "data", "availableDeliveries", "success")
		wantNotNull(t, res.json, "data", "availableDeliveries", "timeStamp")
	})

	t.Run("rateDriver", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.mockClient.SetDelivery(&ports.DeliveryInfo{
			DeliveryID: "del-1",
			CustomerID: "user-c",
			DriverID:   "drv-1",
		})

		res := e.gql(`mutation($input: RateDriverInput!) {
			rateDriver(input: $input) { success statusCode timeStamp data { id rating } }
		}`, map[string]interface{}{"input": rateInput("drv-1", "del-1", 5, "nice")},
			userHeaders("user-c", ""))
		wantNoErrors(t, res)
		wantBool(t, res.json, true, "data", "rateDriver", "success")
		wantNotNull(t, res.json, "data", "rateDriver", "timeStamp")
	})
}

// ---------------------------------------------------------------------------
// AssignmentResolver zero-value timestamps
// ---------------------------------------------------------------------------

// TestQuery_AssignmentZeroTimestamps feeds an assignment whose optional
// timestamps were never written; each accessor must answer null instead of a
// zero-value RFC3339 string.
func TestQuery_AssignmentZeroTimestamps(t *testing.T) {
	e := newEnv(t)

	a := newOfferedAssignment("a-zero", "drv-1", "del-1")
	a.OfferedAt, a.ExpiresAt = time.Time{}, time.Time{}
	a.CreatedAt, a.UpdatedAt = time.Time{}, time.Time{}
	if err := e.mockAssignments.Save(t.Context(), a); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}

	res := e.gql(`query($id: ID!) {
		assignment(id: $id) {
			success statusCode
			data { id offeredAt expiresAt acceptedAt rejectedAt completedAt createdAt updatedAt }
		}
	}`, map[string]interface{}{"id": "a-zero"}, userHeaders("", "admin"))

	wantNoErrors(t, res)
	base := "data.assignment."
	wantBool(t, res.json, true, base+"success")
	wantStr(t, res.json, "a-zero", base+"data", "id")
	wantNull(t, res.json, base+"data", "offeredAt")
	wantNull(t, res.json, base+"data", "expiresAt")
	wantNull(t, res.json, base+"data", "createdAt")
	wantNull(t, res.json, base+"data", "updatedAt")
	wantNull(t, res.json, base+"data", "acceptedAt")
	wantNull(t, res.json, base+"data", "rejectedAt")
	wantNull(t, res.json, base+"data", "completedAt")
}

// ---------------------------------------------------------------------------
// Defensive nil branches unreachable through the schema
// ---------------------------------------------------------------------------

// TestResolver_NilPayloadBranches calls the resolvers directly for the states
// no root resolver ever produces, so their guards are still exercised.
func TestResolver_NilPayloadBranches(t *testing.T) {
	t.Run("driverStatus without a driver", func(t *testing.T) {
		r := graphql.NewDriverStatusResolverForTest(graphql.DriverStatusData{Status: "OFFLINE"})
		if got := r.Driver(); got != nil {
			t.Errorf("Driver() = %v, want nil when the payload carries no driver", got)
		}
		if got := r.Status(); got != "OFFLINE" {
			t.Errorf("Status() = %q, want OFFLINE", got)
		}
	})

	t.Run("driverServiceInfo without a payload", func(t *testing.T) {
		r := graphql.NewEmptyDriverServiceInfoResponseForTest()
		if got := r.Data(); got != nil {
			t.Errorf("Data() = %v, want nil when the response payload is nil", got)
		}
		if r.Success() {
			t.Error("Success() = true on an empty response, want false")
		}
	})
}
