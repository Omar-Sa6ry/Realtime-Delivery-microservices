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

// newReviewTestRepo returns a ReviewRepository backed by a fresh, uniquely
// named collection that is dropped when the test finishes.
func newReviewTestRepo(t *testing.T) (*mongodb.ReviewRepository, *mongo.Collection) {
	t.Helper()
	col := testCollection(t, "reviews")
	return mongodb.NewReviewRepository(testMongoClient, testDBName, col.Name()), col
}

func testReview(driverID, deliveryID string, rating float64, createdAt time.Time) *domain.Review {
	return &domain.Review{
		ID:         "rev-" + driverID + "-" + deliveryID,
		DriverID:   driverID,
		UserID:     "user-1",
		DeliveryID: deliveryID,
		Rating:     rating,
		Comment:    "comment",
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}
}

func TestReviewRepositoryCreate(t *testing.T) {
	repo, col := newReviewTestRepo(t)
	ctx := context.Background()

	t.Run("inserts a review", func(t *testing.T) {
		review := testReview("drv-1", "del-1", 5, time.Now())
		if err := repo.Create(ctx, review); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
		count, err := col.CountDocuments(ctx, bson.M{"deliveryid": "del-1"})
		if err != nil {
			t.Fatalf("CountDocuments() error: %v", err)
		}
		if count != 1 {
			t.Errorf("document count = %d, want 1", count)
		}
	})

	t.Run("propagates a duplicate delivery id error", func(t *testing.T) {
		if err := repo.EnsureIndexes(ctx); err != nil {
			t.Fatalf("EnsureIndexes() error: %v", err)
		}
		duplicate := testReview("drv-2", "del-1", 4, time.Now())
		if err := repo.Create(ctx, duplicate); err == nil {
			t.Error("Create() expected a duplicate key error for a reused delivery id, got nil")
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := repo.Create(cctx, testReview("drv-3", "del-3", 3, time.Now())); err == nil {
			t.Error("Create() expected an error for a cancelled context, got nil")
		}
	})
}

func TestReviewRepositoryFindByDriverID(t *testing.T) {
	repo, col := newReviewTestRepo(t)
	ctx := context.Background()

	now := time.Now()
	// Newest first when sorted by createdat descending.
	seed := []*domain.Review{
		testReview("drv-1", "del-1", 5, now),
		testReview("drv-1", "del-2", 4, now.Add(-time.Hour)),
		testReview("drv-1", "del-3", 3, now.Add(-2*time.Hour)),
	}
	for _, r := range seed {
		if err := repo.Create(ctx, r); err != nil {
			t.Fatalf("Create() error: %v", err)
		}
	}
	// Poisoned document used to exercise the cursor.All error path.
	if _, err := col.InsertOne(ctx, bson.M{"driverid": "drv-bad", "rating": bson.M{"x": 1}}); err != nil {
		t.Fatalf("seed poison document: %v", err)
	}

	tests := []struct {
		name      string
		driverID  string
		skip      int
		limit     int
		wantIDs   []string
		wantTotal int64
	}{
		{"first page sorted newest first", "drv-1", 0, 2, []string{"rev-drv-1-del-1", "rev-drv-1-del-2"}, 3},
		{"second page continues the ordering", "drv-1", 1, 1, []string{"rev-drv-1-del-2"}, 3},
		{"page beyond the end returns nothing", "drv-1", 10, 5, nil, 3},
		{"unknown driver returns nothing", "drv-missing", 0, 10, nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, total, err := repo.FindByDriverID(ctx, tt.driverID, tt.skip, tt.limit)
			if err != nil {
				t.Fatalf("FindByDriverID() error: %v", err)
			}
			if total != tt.wantTotal {
				t.Errorf("FindByDriverID() total = %d, want %d", total, tt.wantTotal)
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("FindByDriverID() returned %d reviews, want %d", len(got), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("FindByDriverID()[%d].ID = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}

	t.Run("propagates an invalid skip value from the find operation", func(t *testing.T) {
		// CountDocuments succeeds, the subsequent find fails server-side.
		got, _, err := repo.FindByDriverID(ctx, "drv-1", -1, 10)
		if err == nil {
			t.Fatal("FindByDriverID() expected an error for a negative skip, got nil")
		}
		if got != nil {
			t.Errorf("FindByDriverID() = %+v, want nil on error", got)
		}
	})

	t.Run("propagates decode errors", func(t *testing.T) {
		got, _, err := repo.FindByDriverID(ctx, "drv-bad", 0, 10)
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
		got, total, err := repo.FindByDriverID(cctx, "drv-1", 0, 10)
		if err == nil {
			t.Fatal("FindByDriverID() expected an error for a cancelled context, got nil")
		}
		if got != nil || total != 0 {
			t.Errorf("FindByDriverID() = (%v, %d), want (nil, 0) on error", got, total)
		}
	})
}

func TestReviewRepositoryFindByDeliveryID(t *testing.T) {
	repo, col := newReviewTestRepo(t)
	ctx := context.Background()

	if err := repo.Create(ctx, testReview("drv-1", "del-1", 5, time.Now())); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if _, err := col.InsertOne(ctx, bson.M{"deliveryid": "del-bad", "rating": bson.M{"x": 1}}); err != nil {
		t.Fatalf("seed poison document: %v", err)
	}

	t.Run("returns the review of a delivery", func(t *testing.T) {
		got, err := repo.FindByDeliveryID(ctx, "del-1")
		if err != nil {
			t.Fatalf("FindByDeliveryID() error: %v", err)
		}
		if got == nil {
			t.Fatal("FindByDeliveryID() returned nil review")
		}
		if got.DriverID != "drv-1" || got.Rating != 5 {
			t.Errorf("FindByDeliveryID() = %+v, want driver drv-1 with rating 5", got)
		}
	})

	t.Run("returns nil when the delivery has no review", func(t *testing.T) {
		got, err := repo.FindByDeliveryID(ctx, "del-missing")
		if err != nil {
			t.Fatalf("FindByDeliveryID() error: %v", err)
		}
		if got != nil {
			t.Errorf("FindByDeliveryID() = %+v, want nil", got)
		}
	})

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

func TestReviewRepositoryCalculateAverageRating(t *testing.T) {
	repo, _ := newReviewTestRepo(t)
	ctx := context.Background()

	t.Run("averages the ratings of a driver", func(t *testing.T) {
		for i, rating := range []float64{5, 4} {
			r := testReview("drv-1", "del-"+string(rune('a'+i)), rating, time.Now().Add(-time.Duration(i)*time.Hour))
			if err := repo.Create(ctx, r); err != nil {
				t.Fatalf("Create() error: %v", err)
			}
		}
		avg, total, err := repo.CalculateAverageRating(ctx, "drv-1")
		if err != nil {
			t.Fatalf("CalculateAverageRating() error: %v", err)
		}
		if avg != 4.5 {
			t.Errorf("CalculateAverageRating() average = %v, want 4.5", avg)
		}
		if total != 2 {
			t.Errorf("CalculateAverageRating() total = %d, want 2", total)
		}
	})

	t.Run("returns zero for a driver without reviews", func(t *testing.T) {
		avg, total, err := repo.CalculateAverageRating(ctx, "drv-missing")
		if err != nil {
			t.Fatalf("CalculateAverageRating() error: %v", err)
		}
		if avg != 0 || total != 0 {
			t.Errorf("CalculateAverageRating() = (%v, %d), want (0, 0)", avg, total)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		_, _, err := repo.CalculateAverageRating(cctx, "drv-1")
		if err == nil {
			t.Fatal("CalculateAverageRating() expected an error for a cancelled context, got nil")
		}
	})
}

func TestReviewRepositoryEnsureIndexes(t *testing.T) {
	repo, col := newReviewTestRepo(t)
	ctx := context.Background()

	t.Run("creates the expected indexes", func(t *testing.T) {
		if err := repo.EnsureIndexes(ctx); err != nil {
			t.Fatalf("EnsureIndexes() error: %v", err)
		}
		names := indexNames(t, col)
		for _, want := range []string{"driverid_1_createdat_-1", "deliveryid_1"} {
			if !names[want] {
				t.Errorf("missing index %q, have %v", want, names)
			}
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := repo.EnsureIndexes(cctx); err == nil {
			t.Error("EnsureIndexes() expected an error for a cancelled context, got nil")
		}
	})
}
