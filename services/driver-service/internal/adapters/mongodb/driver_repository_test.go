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

// newDriverTestRepo returns a DriverRepository backed by a fresh, uniquely
// named collection that is dropped when the test finishes.
func newDriverTestRepo(t *testing.T) (*mongodb.DriverRepository, *mongo.Collection) {
	t.Helper()
	col := testCollection(t, "drivers")
	return mongodb.NewDriverRepository(testMongoClient, testDBName, col.Name()), col
}

func testDriver(id, userID string) *domain.Driver {
	now := time.Now()
	return &domain.Driver{
		ID:           id,
		UserID:       userID,
		Status:       domain.DriverStatusAvailable,
		IsActive:     true,
		IsBlocked:    false,
		Vehicle:      domain.VehicleInfo{Type: domain.VehicleTypeCar, PlateNumber: "PLT-001", CapacityKg: 500},
		Capabilities: []string{"gps"},
		ServiceArea:  "cairo",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func TestDriverRepositoryFindByID(t *testing.T) {
	repo, col := newDriverTestRepo(t)
	ctx := context.Background()

	t.Run("returns the stored driver", func(t *testing.T) {
		if err := repo.Save(ctx, testDriver("drv-1", "user-1")); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		got, err := repo.FindByID(ctx, "drv-1")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if got == nil {
			t.Fatal("FindByID() returned nil driver")
		}
		if got.ID != "drv-1" || got.UserID != "user-1" {
			t.Errorf("FindByID() = %+v, want id drv-1 / user-1", got)
		}
		if got.Status != domain.DriverStatusAvailable {
			t.Errorf("Status = %v, want %v", got.Status, domain.DriverStatusAvailable)
		}
		if got.Vehicle.Type != domain.VehicleTypeCar {
			t.Errorf("Vehicle.Type = %v, want %v", got.Vehicle.Type, domain.VehicleTypeCar)
		}
	})

	t.Run("returns nil when the driver does not exist", func(t *testing.T) {
		got, err := repo.FindByID(ctx, "drv-missing")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if got != nil {
			t.Errorf("FindByID() = %+v, want nil", got)
		}
	})

	t.Run("propagates decode errors", func(t *testing.T) {
		// A document whose "version" field holds a sub-document cannot be
		// decoded into the int field of domain.Driver.
		if _, err := col.InsertOne(ctx, bson.M{"_id": "drv-bad", "version": bson.M{"x": 1}}); err != nil {
			t.Fatalf("seed poison document: %v", err)
		}
		got, err := repo.FindByID(ctx, "drv-bad")
		if err == nil {
			t.Fatal("FindByID() expected a decode error, got nil")
		}
		if got != nil {
			t.Errorf("FindByID() = %+v, want nil on error", got)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.FindByID(cctx, "drv-1")
		if err == nil {
			t.Fatal("FindByID() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindByID() = %+v, want nil on error", got)
		}
	})
}

func TestDriverRepositoryFindByIDs(t *testing.T) {
	repo, col := newDriverTestRepo(t)
	ctx := context.Background()

	for _, id := range []string{"drv-1", "drv-2", "drv-3"} {
		if err := repo.Save(ctx, testDriver(id, "user-"+id)); err != nil {
			t.Fatalf("Save(%s) error: %v", id, err)
		}
	}
	// Poisoned document used to exercise the cursor.All error path.
	if _, err := col.InsertOne(ctx, bson.M{"_id": "drv-bad", "version": bson.M{"x": 1}}); err != nil {
		t.Fatalf("seed poison document: %v", err)
	}

	tests := []struct {
		name    string
		ids     []string
		wantIDs []string
		wantErr bool
	}{
		{"nil input returns an empty result", nil, nil, false},
		{"empty input returns an empty result", []string{}, nil, false},
		{"returns the requested drivers", []string{"drv-1", "drv-3"}, []string{"drv-1", "drv-3"}, false},
		{"ignores unknown ids", []string{"drv-2", "drv-missing"}, []string{"drv-2"}, false},
		{"propagates decode errors", []string{"drv-bad"}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FindByIDs(ctx, tt.ids)
			if tt.wantErr {
				if err == nil {
					t.Fatal("FindByIDs() expected an error, got nil")
				}
				if got != nil {
					t.Errorf("FindByIDs() = %+v, want nil on error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("FindByIDs() error: %v", err)
			}
			want := make(map[string]bool, len(tt.wantIDs))
			for _, id := range tt.wantIDs {
				want[id] = true
			}
			gotIDs := driverIDs(got)
			if len(gotIDs) != len(want) {
				t.Errorf("FindByIDs() ids = %v, want %v", gotIDs, want)
			}
			for id := range want {
				if !gotIDs[id] {
					t.Errorf("FindByIDs() missing %q (got %v)", id, gotIDs)
				}
			}
		})
	}

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.FindByIDs(cctx, []string{"drv-1"})
		if err == nil {
			t.Fatal("FindByIDs() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindByIDs() = %+v, want nil on error", got)
		}
	})
}

func TestDriverRepositoryFindByUserID(t *testing.T) {
	repo, col := newDriverTestRepo(t)
	ctx := context.Background()

	if err := repo.Save(ctx, testDriver("drv-1", "user-1")); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	// Legacy documents that stored the field as "userid" are still found via
	// the $or filter in FindByUserID.
	if _, err := col.InsertOne(ctx, bson.M{"_id": "drv-legacy", "userid": "user-legacy"}); err != nil {
		t.Fatalf("seed legacy document: %v", err)
	}

	tests := []struct {
		name    string
		userID  string
		wantID  string
		wantNil bool
	}{
		{"finds by the userId field", "user-1", "drv-1", false},
		{"finds by the legacy userid field", "user-legacy", "drv-legacy", false},
		{"returns nil for an unknown user", "user-missing", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FindByUserID(ctx, tt.userID)
			if err != nil {
				t.Fatalf("FindByUserID() error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("FindByUserID() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("FindByUserID() returned nil driver")
			}
			if got.ID != tt.wantID {
				t.Errorf("FindByUserID() id = %q, want %q", got.ID, tt.wantID)
			}
		})
	}

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.FindByUserID(cctx, "user-1")
		if err == nil {
			t.Fatal("FindByUserID() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindByUserID() = %+v, want nil on error", got)
		}
	})
}

func TestDriverRepositoryFindAvailableByLocation(t *testing.T) {
	repo, _ := newDriverTestRepo(t)
	ctx := context.Background()

	seed := func(id string, mutate func(*domain.Driver)) {
		t.Helper()
		d := testDriver(id, "user-"+id)
		if mutate != nil {
			mutate(d)
		}
		if err := repo.Save(ctx, d); err != nil {
			t.Fatalf("Save(%s) error: %v", id, err)
		}
	}

	seed("drv-car", nil)
	seed("drv-moto", func(d *domain.Driver) { d.Vehicle.Type = domain.VehicleTypeMotorcycle })
	seed("drv-offline", func(d *domain.Driver) { d.Status = domain.DriverStatusOffline })
	seed("drv-blocked", func(d *domain.Driver) { d.IsBlocked = true })
	seed("drv-inactive", func(d *domain.Driver) { d.IsActive = false })

	tests := []struct {
		name        string
		vehicleType domain.VehicleType
		wantIDs     []string
	}{
		{"filters by vehicle type", domain.VehicleTypeCar, []string{"drv-car"}},
		{"returns every available driver when no vehicle filter is given", "", []string{"drv-car", "drv-moto"}},
		{"returns an empty result for an unmatched vehicle type", domain.VehicleTypeTruck, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FindAvailableByLocation(ctx, 30.0, 31.0, 5.0, tt.vehicleType)
			if err != nil {
				t.Fatalf("FindAvailableByLocation() error: %v", err)
			}
			want := make(map[string]bool, len(tt.wantIDs))
			for _, id := range tt.wantIDs {
				want[id] = true
			}
			gotIDs := driverIDs(got)
			if len(gotIDs) != len(want) {
				t.Errorf("FindAvailableByLocation() ids = %v, want %v", gotIDs, want)
			}
			for id := range want {
				if !gotIDs[id] {
					t.Errorf("FindAvailableByLocation() missing %q (got %v)", id, gotIDs)
				}
			}
		})
	}

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.FindAvailableByLocation(cctx, 30.0, 31.0, 5.0, domain.VehicleTypeCar)
		if err == nil {
			t.Fatal("FindAvailableByLocation() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindAvailableByLocation() = %+v, want nil on error", got)
		}
	})
}

func TestDriverRepositoryFindByStatus(t *testing.T) {
	repo, _ := newDriverTestRepo(t)
	ctx := context.Background()

	available := testDriver("drv-a", "user-a")
	if err := repo.Save(ctx, available); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	offline := testDriver("drv-o", "user-o")
	offline.Status = domain.DriverStatusOffline
	if err := repo.Save(ctx, offline); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	tests := []struct {
		name    string
		status  string
		wantIDs []string
	}{
		{"returns drivers matching the status", "AVAILABLE", []string{"drv-a"}},
		{"returns drivers matching another status", "OFFLINE", []string{"drv-o"}},
		{"returns an empty result for an unknown status", "SUSPENDED", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FindByStatus(ctx, tt.status)
			if err != nil {
				t.Fatalf("FindByStatus() error: %v", err)
			}
			want := make(map[string]bool, len(tt.wantIDs))
			for _, id := range tt.wantIDs {
				want[id] = true
			}
			gotIDs := driverIDs(got)
			if len(gotIDs) != len(want) {
				t.Errorf("FindByStatus() ids = %v, want %v", gotIDs, want)
			}
		})
	}

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.FindByStatus(cctx, "AVAILABLE")
		if err == nil {
			t.Fatal("FindByStatus() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindByStatus() = %+v, want nil on error", got)
		}
	})
}

func TestDriverRepositorySave(t *testing.T) {
	repo, col := newDriverTestRepo(t)
	ctx := context.Background()

	t.Run("inserts a new driver (upsert inserts)", func(t *testing.T) {
		if err := repo.Save(ctx, testDriver("drv-new", "user-new")); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		count, err := col.CountDocuments(ctx, bson.M{"_id": "drv-new"})
		if err != nil {
			t.Fatalf("CountDocuments() error: %v", err)
		}
		if count != 1 {
			t.Errorf("document count = %d, want 1", count)
		}
	})

	t.Run("replaces an existing driver instead of duplicating it (upsert replaces)", func(t *testing.T) {
		d := testDriver("drv-replace", "user-replace")
		if err := repo.Save(ctx, d); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		d.Status = domain.DriverStatusBusy
		d.Version++
		if err := repo.Save(ctx, d); err != nil {
			t.Fatalf("Save() second call error: %v", err)
		}
		count, err := col.CountDocuments(ctx, bson.M{"_id": "drv-replace"})
		if err != nil {
			t.Fatalf("CountDocuments() error: %v", err)
		}
		if count != 1 {
			t.Errorf("document count = %d, want 1 (replace, not insert)", count)
		}
		got, err := repo.FindByID(ctx, "drv-replace")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if got == nil || got.Status != domain.DriverStatusBusy {
			t.Errorf("FindByID() = %+v, want status BUSY", got)
		}
	})

	t.Run("propagates a duplicate userId error", func(t *testing.T) {
		if err := repo.EnsureIndexes(ctx); err != nil {
			t.Fatalf("EnsureIndexes() error: %v", err)
		}
		if err := repo.Save(ctx, testDriver("drv-first", "dup-user")); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		if err := repo.Save(ctx, testDriver("drv-second", "dup-user")); err == nil {
			t.Error("Save() expected a duplicate key error for a reused userId, got nil")
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := repo.Save(cctx, testDriver("drv-cancelled", "user-cancelled")); err == nil {
			t.Error("Save() expected an error for a cancelled context, got nil")
		}
	})
}

func TestDriverRepositoryEnsureIndexes(t *testing.T) {
	repo, col := newDriverTestRepo(t)
	ctx := context.Background()

	t.Run("creates the expected indexes", func(t *testing.T) {
		if err := repo.EnsureIndexes(ctx); err != nil {
			t.Fatalf("EnsureIndexes() error: %v", err)
		}
		names := indexNames(t, col)
		for _, want := range []string{"status_1", "userId_1", "vehicle.type_1", "updatedAt_-1"} {
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

func TestDriverRepositoryDeleteExpiredLocations(t *testing.T) {
	repo, _ := newDriverTestRepo(t)
	ctx := context.Background()

	t.Run("marks stale available drivers offline", func(t *testing.T) {
		stale := testDriver("drv-stale", "user-stale")
		stale.UpdatedAt = time.Now().Add(-48 * time.Hour)
		if err := repo.Save(ctx, stale); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		recent := testDriver("drv-recent", "user-recent")
		if err := repo.Save(ctx, recent); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		offline := testDriver("drv-offline", "user-offline")
		offline.Status = domain.DriverStatusOffline
		offline.UpdatedAt = time.Now().Add(-48 * time.Hour)
		if err := repo.Save(ctx, offline); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		modified, err := repo.DeleteExpiredLocations(ctx, 24*time.Hour)
		if err != nil {
			t.Fatalf("DeleteExpiredLocations() error: %v", err)
		}
		if modified != 1 {
			t.Errorf("DeleteExpiredLocations() modified = %d, want 1", modified)
		}

		staleGot, err := repo.FindByID(ctx, "drv-stale")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if staleGot == nil || staleGot.Status != domain.DriverStatusOffline {
			t.Errorf("stale driver status = %+v, want OFFLINE", staleGot)
		}

		recentGot, err := repo.FindByID(ctx, "drv-recent")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if recentGot == nil || recentGot.Status != domain.DriverStatusAvailable {
			t.Errorf("recent driver status = %+v, want AVAILABLE", recentGot)
		}

		// A second run finds nothing left to expire.
		modified, err = repo.DeleteExpiredLocations(ctx, 24*time.Hour)
		if err != nil {
			t.Fatalf("DeleteExpiredLocations() second run error: %v", err)
		}
		if modified != 0 {
			t.Errorf("DeleteExpiredLocations() second run modified = %d, want 0", modified)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		modified, err := repo.DeleteExpiredLocations(cctx, 24*time.Hour)
		if err == nil {
			t.Fatal("DeleteExpiredLocations() expected an error for a cancelled context, got nil")
		}
		if modified != 0 {
			t.Errorf("DeleteExpiredLocations() modified = %d, want 0 on error", modified)
		}
	})
}

func TestDriverRepositoryUpdateRating(t *testing.T) {
	repo, col := newDriverTestRepo(t)
	ctx := context.Background()

	t.Run("updates the rating fields", func(t *testing.T) {
		if err := repo.Save(ctx, testDriver("drv-rated", "user-rated")); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		if err := repo.UpdateRating(ctx, "drv-rated", 4.75, 12); err != nil {
			t.Fatalf("UpdateRating() error: %v", err)
		}

		got, err := repo.FindByID(ctx, "drv-rated")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if got == nil {
			t.Fatal("FindByID() returned nil driver")
		}
		if got.Rating != 4.75 {
			t.Errorf("Rating = %v, want 4.75", got.Rating)
		}
		if got.RatingCount != 12 {
			t.Errorf("RatingCount = %v, want 12", got.RatingCount)
		}

		// The raw document must carry the same keys the struct's bson tags
		// declare, otherwise the update writes unreadable shadow fields.
		var raw bson.M
		if err := col.FindOne(ctx, bson.M{"_id": "drv-rated"}).Decode(&raw); err != nil {
			t.Fatalf("raw decode: %v", err)
		}
		if n, ok := asInt64(raw["ratingCount"]); !ok || n != 12 {
			t.Errorf("raw ratingCount = %v, want 12", raw["ratingCount"])
		}
		if _, ok := raw["updatedAt"]; !ok {
			t.Errorf("raw document missing updatedAt key, have %v", raw)
		}
		if _, ok := raw["ratingcount"]; ok {
			t.Errorf("raw document has stray lowercase ratingcount key, have %v", raw)
		}
	})

	t.Run("does not fail for an unknown driver", func(t *testing.T) {
		if err := repo.UpdateRating(ctx, "drv-unknown", 3.5, 1); err != nil {
			t.Errorf("UpdateRating() error for unknown driver: %v", err)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := repo.UpdateRating(cctx, "drv-rated", 1.0, 1); err == nil {
			t.Error("UpdateRating() expected an error for a cancelled context, got nil")
		}
	})
}
