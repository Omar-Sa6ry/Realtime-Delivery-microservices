package graphql_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

func TestLoaders_ContextRoundTrip(t *testing.T) {
	plain := context.Background()
	if got := graphql.GetLoaders(plain); got != nil {
		t.Errorf("GetLoaders on a plain context = %v, want nil", got)
	}

	loaders := graphql.NewLoaders(mocks.NewMockDriverRepository())
	ctx := graphql.WithLoaders(plain, loaders)
	got := graphql.GetLoaders(ctx)
	if got != loaders {
		t.Errorf("GetLoaders did not return the loaders stored by WithLoaders")
	}

	empty := graphql.WithLoaders(plain, &graphql.Loaders{})
	if graphql.GetLoaders(empty) == nil {
		t.Errorf("GetLoaders returned nil for a stored empty Loaders value")
	}
}

// TestNewLoaders_BatchBehavior drives the batch function created by NewLoaders
// through the real HTTP handler: keys present in the repository, keys missing
// from it, and a repository that errors on the batched lookup.
func TestNewLoaders_BatchBehavior(t *testing.T) {
	t.Run("present keys resolve to the stored driver", func(t *testing.T) {
		e := newEnv(t)
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.reviewRepo.SetReview(newReview("rev-1", "drv-1", "user-c", "del-1", 5, "great"))

		res := e.gql(`{ driverReviews(driverId: "drv-1") {
				success data { items { id driver { id userId } } }
			}}`, nil, userHeaders("", "admin"))
		wantNoErrors(t, res)
		wantStr(t, res.json, "drv-1", "data", "driverReviews", "data", "items", "0", "driver", "id")
		wantStr(t, res.json, "user-1", "data", "driverReviews", "data", "items", "0", "driver", "userId")
	})

	t.Run("missing keys resolve to a nil driver", func(t *testing.T) {
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
		wantListLen(t, res.json, 1, "data", "nearbyDrivers", "data", "items")
		wantNull(t, res.json, "data", "nearbyDrivers", "data", "items", "0", "driver")
	})

	t.Run("repository error is handed to every key", func(t *testing.T) {
		e := newEnv(t, withBatchLoadError(errors.New("batch boom")))
		e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))
		e.reviewRepo.SetReview(newReview("rev-1", "drv-1", "user-c", "del-1", 5, "great"))

		res := e.gql(`{ driverReviews(driverId: "drv-1") {
				success data { items { id driver { id } } }
			}}`, nil, userHeaders("", "admin"))
		wantSomeErrors(t, res)
	})
}
