package mongodb_test

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/mongodb"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
)

// newDispatchAttemptTestRepo returns a DispatchAttemptRepository backed by a
// fresh, uniquely named collection that is dropped when the test finishes.
func newDispatchAttemptTestRepo(t *testing.T) (*mongodb.DispatchAttemptRepository, *mongo.Collection) {
	t.Helper()
	col := testCollection(t, "dispatch_attempts")
	return mongodb.NewDispatchAttemptRepository(testMongoClient, testDBName, col.Name()), col
}

func testAttempt(id, deliveryID, driverID string) *domain.DispatchAttempt {
	return &domain.DispatchAttempt{
		ID:             id,
		DeliveryID:     deliveryID,
		DriverID:       driverID,
		DistanceMeters: 1234.5,
		AttemptNumber:  1,
		Result:         domain.DispatchResultAccepted,
		Reason:         "ok",
		CreatedAt:      time.Now(),
	}
}

// NOTE on field names: domain.DispatchAttempt has no bson struct tags, so the
// driver stores "deliveryid"/"driverid" (lowercase) while the Find* methods
// query the camelCase keys "deliveryId"/"driverId". The read tests therefore
// seed documents with the keys the queries use; the mismatch between Save and
// the Find* methods is called out in the test report.
func TestDispatchAttemptRepositorySave(t *testing.T) {
	repo, col := newDispatchAttemptTestRepo(t)
	ctx := context.Background()

	t.Run("inserts an attempt", func(t *testing.T) {
		if err := repo.Save(ctx, testAttempt("att-1", "del-1", "drv-1")); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		if err := repo.Save(ctx, testAttempt("att-2", "del-1", "drv-2")); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		count, err := col.CountDocuments(ctx, bson.M{})
		if err != nil {
			t.Fatalf("CountDocuments() error: %v", err)
		}
		if count != 2 {
			t.Errorf("document count = %d, want 2 (attempts are appended, not upserted)", count)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := repo.Save(cctx, testAttempt("att-3", "del-3", "drv-3")); err == nil {
			t.Error("Save() expected an error for a cancelled context, got nil")
		}
	})
}

func TestDispatchAttemptRepositoryFindByDeliveryID(t *testing.T) {
	repo, col := newDispatchAttemptTestRepo(t)
	ctx := context.Background()

	seed := []bson.M{
		{"deliveryId": "del-1", "driverId": "drv-1", "attemptnumber": 1},
		{"deliveryId": "del-1", "driverId": "drv-2", "attemptnumber": 2},
		{"deliveryId": "del-2", "driverId": "drv-3", "attemptnumber": 1},
		// Legacy lowercase key, not matched by the query.
		{"deliveryid": "del-1", "driverid": "drv-4", "attemptnumber": 1},
		// Poisoned document used to exercise the cursor.All error path.
		{"deliveryId": "del-bad", "attemptnumber": bson.M{"x": 1}},
	}
	if _, err := col.InsertMany(ctx, seed); err != nil {
		t.Fatalf("seed documents: %v", err)
	}

	tests := []struct {
		name       string
		deliveryID string
		wantCount  int
	}{
		{"returns the attempts of a delivery", "del-1", 2},
		{"returns the attempts of another delivery", "del-2", 1},
		{"returns an empty result for an unknown delivery", "del-none", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FindByDeliveryID(ctx, tt.deliveryID)
			if err != nil {
				t.Fatalf("FindByDeliveryID() error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("FindByDeliveryID() returned %d attempts, want %d", len(got), tt.wantCount)
			}
		})
	}

	t.Run("propagates decode errors", func(t *testing.T) {
		got, err := repo.FindByDeliveryID(ctx, "del-bad")
		if err == nil {
			t.Fatal("FindByDeliveryID() expected a decode error, got nil")
		}
		if got != nil {
			t.Errorf("FindByDeliveryID() = %+v, want nil on error", got)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.FindByDeliveryID(cctx, "del-1")
		if err == nil {
			t.Fatal("FindByDeliveryID() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindByDeliveryID() = %+v, want nil on error", got)
		}
	})
}

func TestDispatchAttemptRepositoryFindByDriverID(t *testing.T) {
	repo, col := newDispatchAttemptTestRepo(t)
	ctx := context.Background()

	seed := []bson.M{
		{"deliveryId": "del-1", "driverId": "drv-1", "attemptnumber": 1},
		{"deliveryId": "del-2", "driverId": "drv-1", "attemptnumber": 2},
		{"deliveryId": "del-3", "driverId": "drv-2", "attemptnumber": 1},
		// Poisoned document used to exercise the cursor.All error path.
		{"driverId": "drv-bad", "attemptnumber": bson.M{"x": 1}},
	}
	if _, err := col.InsertMany(ctx, seed); err != nil {
		t.Fatalf("seed documents: %v", err)
	}

	tests := []struct {
		name      string
		driverID  string
		wantCount int
	}{
		{"returns the attempts of a driver", "drv-1", 2},
		{"returns the attempts of another driver", "drv-2", 1},
		{"returns an empty result for an unknown driver", "drv-none", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FindByDriverID(ctx, tt.driverID)
			if err != nil {
				t.Fatalf("FindByDriverID() error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("FindByDriverID() returned %d attempts, want %d", len(got), tt.wantCount)
			}
		})
	}

	t.Run("propagates decode errors", func(t *testing.T) {
		got, err := repo.FindByDriverID(ctx, "drv-bad")
		if err == nil {
			t.Fatal("FindByDriverID() expected a decode error, got nil")
		}
		if got != nil {
			t.Errorf("FindByDriverID() = %+v, want nil on error", got)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.FindByDriverID(cctx, "drv-1")
		if err == nil {
			t.Fatal("FindByDriverID() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindByDriverID() = %+v, want nil on error", got)
		}
	})
}

func TestDispatchAttemptRepositoryCountByDeliveryID(t *testing.T) {
	repo, col := newDispatchAttemptTestRepo(t)
	ctx := context.Background()

	seed := []bson.M{
		{"deliveryId": "del-1", "driverId": "drv-1"},
		{"deliveryId": "del-1", "driverId": "drv-2"},
		{"deliveryId": "del-2", "driverId": "drv-3"},
	}
	if _, err := col.InsertMany(ctx, seed); err != nil {
		t.Fatalf("seed documents: %v", err)
	}

	tests := []struct {
		name       string
		deliveryID string
		want       int64
	}{
		{"counts the attempts of a delivery", "del-1", 2},
		{"counts zero for an unknown delivery", "del-none", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.CountByDeliveryID(ctx, tt.deliveryID)
			if err != nil {
				t.Fatalf("CountByDeliveryID() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("CountByDeliveryID() = %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.CountByDeliveryID(cctx, "del-1")
		if err == nil {
			t.Fatal("CountByDeliveryID() expected an error for a cancelled context, got nil")
		}
		if got != 0 {
			t.Errorf("CountByDeliveryID() = %d, want 0 on error", got)
		}
	})
}

func TestDispatchAttemptRepositoryEnsureIndex(t *testing.T) {
	repo, col := newDispatchAttemptTestRepo(t)
	ctx := context.Background()

	t.Run("creates the expected indexes", func(t *testing.T) {
		if err := repo.EnsureIndex(ctx); err != nil {
			t.Fatalf("EnsureIndex() error: %v", err)
		}
		names := indexNames(t, col)
		for _, want := range []string{"deliveryId_1", "driverId_1", "createdAt_1"} {
			if !names[want] {
				t.Errorf("missing index %q, have %v", want, names)
			}
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := repo.EnsureIndex(cctx); err == nil {
			t.Error("EnsureIndex() expected an error for a cancelled context, got nil")
		}
	})
}
