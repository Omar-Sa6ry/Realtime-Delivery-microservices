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

// newAssignmentTestRepo returns an AssignmentRepository backed by a fresh,
// uniquely named collection that is dropped when the test finishes.
func newAssignmentTestRepo(t *testing.T) (*mongodb.AssignmentRepository, *mongo.Collection) {
	t.Helper()
	col := testCollection(t, "assignments")
	return mongodb.NewAssignmentRepository(testMongoClient, testDBName, col.Name()), col
}

func testAssignment(id, driverID, deliveryID string, status domain.AssignmentStatus) *domain.Assignment {
	now := time.Now()
	return &domain.Assignment{
		ID:         id,
		DriverID:   driverID,
		DeliveryID: deliveryID,
		Status:     status,
		OfferedAt:  now,
		ExpiresAt:  now.Add(30 * time.Second),
		CreatedAt:  now,
		UpdatedAt:  now.Add(-time.Hour),
	}
}

func assignmentIDs(assignments []*domain.Assignment) map[string]bool {
	out := make(map[string]bool, len(assignments))
	for _, a := range assignments {
		out[a.ID] = true
	}
	return out
}

func TestAssignmentRepositoryFindByID(t *testing.T) {
	repo, col := newAssignmentTestRepo(t)
	ctx := context.Background()

	t.Run("returns the stored assignment", func(t *testing.T) {
		if err := repo.Save(ctx, testAssignment("asg-1", "drv-1", "del-1", domain.AssignmentStatusOffered)); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		got, err := repo.FindByID(ctx, "asg-1")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if got == nil {
			t.Fatal("FindByID() returned nil assignment")
		}
		if got.DriverID != "drv-1" || got.DeliveryID != "del-1" {
			t.Errorf("FindByID() = %+v, want driver drv-1 / delivery del-1", got)
		}
		if got.Status != domain.AssignmentStatusOffered {
			t.Errorf("Status = %v, want %v", got.Status, domain.AssignmentStatusOffered)
		}
	})

	t.Run("returns nil when the assignment does not exist", func(t *testing.T) {
		got, err := repo.FindByID(ctx, "asg-missing")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if got != nil {
			t.Errorf("FindByID() = %+v, want nil", got)
		}
	})

	t.Run("propagates decode errors", func(t *testing.T) {
		// A sub-document cannot be decoded into the int AttemptNumber field.
		if _, err := col.InsertOne(ctx, bson.M{"_id": "asg-bad", "attemptNumber": bson.M{"x": 1}}); err != nil {
			t.Fatalf("seed poison document: %v", err)
		}
		got, err := repo.FindByID(ctx, "asg-bad")
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
		got, err := repo.FindByID(cctx, "asg-1")
		if err == nil {
			t.Fatal("FindByID() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindByID() = %+v, want nil on error", got)
		}
	})
}

func TestAssignmentRepositoryFindActiveByDriver(t *testing.T) {
	repo, col := newAssignmentTestRepo(t)
	ctx := context.Background()

	seed := []struct {
		id     string
		driver string
		status domain.AssignmentStatus
	}{
		{"asg-offered", "drv-1", domain.AssignmentStatusOffered},
		{"asg-accepted", "drv-2", domain.AssignmentStatusAccepted},
		{"asg-active", "drv-3", domain.AssignmentStatusActive},
		{"asg-completed", "drv-4", domain.AssignmentStatusCompleted},
	}
	for _, s := range seed {
		if err := repo.Save(ctx, testAssignment(s.id, s.driver, "del-active", s.status)); err != nil {
			t.Fatalf("Save(%s) error: %v", s.id, err)
		}
	}
	// Legacy documents stored the driver field as "driverid"; the $or filter
	// in FindActiveByDriver still matches them.
	if _, err := col.InsertOne(ctx, bson.M{"_id": "asg-legacy", "driverid": "drv-5", "status": "OFFERED"}); err != nil {
		t.Fatalf("seed legacy document: %v", err)
	}

	tests := []struct {
		name     string
		driverID string
		wantID   string
		wantNil  bool
	}{
		{"finds an offered assignment", "drv-1", "asg-offered", false},
		{"finds an accepted assignment", "drv-2", "asg-accepted", false},
		{"finds an active assignment", "drv-3", "asg-active", false},
		{"ignores a completed assignment", "drv-4", "", true},
		{"finds documents stored with the legacy driverid key", "drv-5", "asg-legacy", false},
		{"returns nil when the driver has no active assignment", "drv-missing", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FindActiveByDriver(ctx, tt.driverID)
			if err != nil {
				t.Fatalf("FindActiveByDriver() error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("FindActiveByDriver() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("FindActiveByDriver() returned nil assignment")
			}
			if got.ID != tt.wantID {
				t.Errorf("FindActiveByDriver() id = %q, want %q", got.ID, tt.wantID)
			}
		})
	}

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.FindActiveByDriver(cctx, "drv-1")
		if err == nil {
			t.Fatal("FindActiveByDriver() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindActiveByDriver() = %+v, want nil on error", got)
		}
	})
}

func TestAssignmentRepositoryFindByDeliveryID(t *testing.T) {
	repo, col := newAssignmentTestRepo(t)
	ctx := context.Background()

	if err := repo.Save(ctx, testAssignment("asg-1", "drv-1", "del-1", domain.AssignmentStatusOffered)); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	// Poisoned document used to exercise the decode error branch.
	if _, err := col.InsertOne(ctx, bson.M{"_id": "asg-bad", "deliveryId": "del-bad", "attemptNumber": bson.M{"x": 1}}); err != nil {
		t.Fatalf("seed poison document: %v", err)
	}

	t.Run("returns the assignment of a delivery", func(t *testing.T) {
		got, err := repo.FindByDeliveryID(ctx, "del-1")
		if err != nil {
			t.Fatalf("FindByDeliveryID() error: %v", err)
		}
		if got == nil || got.ID != "asg-1" {
			t.Errorf("FindByDeliveryID() = %+v, want asg-1", got)
		}
	})

	t.Run("returns nil when the delivery has no assignment", func(t *testing.T) {
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

func TestAssignmentRepositorySave(t *testing.T) {
	repo, col := newAssignmentTestRepo(t)
	ctx := context.Background()

	t.Run("inserts a new assignment (upsert inserts)", func(t *testing.T) {
		if err := repo.Save(ctx, testAssignment("asg-new", "drv-new", "del-new", domain.AssignmentStatusOffered)); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		count, err := col.CountDocuments(ctx, bson.M{"_id": "asg-new"})
		if err != nil {
			t.Fatalf("CountDocuments() error: %v", err)
		}
		if count != 1 {
			t.Errorf("document count = %d, want 1", count)
		}
	})

	t.Run("replaces an existing assignment instead of duplicating it (upsert replaces)", func(t *testing.T) {
		a := testAssignment("asg-replace", "drv-r", "del-r", domain.AssignmentStatusOffered)
		if err := repo.Save(ctx, a); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		a.Status = domain.AssignmentStatusAccepted
		if err := repo.Save(ctx, a); err != nil {
			t.Fatalf("Save() second call error: %v", err)
		}
		count, err := col.CountDocuments(ctx, bson.M{"_id": "asg-replace"})
		if err != nil {
			t.Fatalf("CountDocuments() error: %v", err)
		}
		if count != 1 {
			t.Errorf("document count = %d, want 1 (replace, not insert)", count)
		}
		got, err := repo.FindByID(ctx, "asg-replace")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if got == nil || got.Status != domain.AssignmentStatusAccepted {
			t.Errorf("FindByID() = %+v, want status ACCEPTED", got)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := repo.Save(cctx, testAssignment("asg-cancelled", "drv-c", "del-c", domain.AssignmentStatusOffered)); err == nil {
			t.Error("Save() expected an error for a cancelled context, got nil")
		}
	})
}

func TestAssignmentRepositoryUpdateStatus(t *testing.T) {
	repo, _ := newAssignmentTestRepo(t)
	ctx := context.Background()

	t.Run("updates status and updatedAt", func(t *testing.T) {
		if err := repo.Save(ctx, testAssignment("asg-1", "drv-1", "del-1", domain.AssignmentStatusOffered)); err != nil {
			t.Fatalf("Save() error: %v", err)
		}
		if err := repo.UpdateStatus(ctx, "asg-1", string(domain.AssignmentStatusRejected)); err != nil {
			t.Fatalf("UpdateStatus() error: %v", err)
		}
		got, err := repo.FindByID(ctx, "asg-1")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if got == nil {
			t.Fatal("FindByID() returned nil assignment")
		}
		if got.Status != domain.AssignmentStatusRejected {
			t.Errorf("Status = %v, want %v", got.Status, domain.AssignmentStatusRejected)
		}
		if time.Since(got.UpdatedAt) > time.Minute {
			t.Errorf("UpdatedAt = %v, want a recent timestamp", got.UpdatedAt)
		}
	})

	t.Run("is a no-op for an unknown assignment", func(t *testing.T) {
		if err := repo.UpdateStatus(ctx, "asg-unknown", string(domain.AssignmentStatusAccepted)); err != nil {
			t.Errorf("UpdateStatus() error for unknown assignment: %v", err)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		if err := repo.UpdateStatus(cctx, "asg-1", string(domain.AssignmentStatusAccepted)); err == nil {
			t.Error("UpdateStatus() expected an error for a cancelled context, got nil")
		}
	})
}

func TestAssignmentRepositoryExpireOffers(t *testing.T) {
	repo, _ := newAssignmentTestRepo(t)
	ctx := context.Background()

	t.Run("expires stale offers only", func(t *testing.T) {
		stale := testAssignment("asg-stale", "drv-1", "del-1", domain.AssignmentStatusOffered)
		stale.ExpiresAt = time.Now().Add(-2 * time.Hour)
		if err := repo.Save(ctx, stale); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		future := testAssignment("asg-future", "drv-2", "del-2", domain.AssignmentStatusOffered)
		future.ExpiresAt = time.Now().Add(10 * time.Minute)
		if err := repo.Save(ctx, future); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		accepted := testAssignment("asg-accepted", "drv-3", "del-3", domain.AssignmentStatusAccepted)
		accepted.ExpiresAt = time.Now().Add(-2 * time.Hour)
		if err := repo.Save(ctx, accepted); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		expiredIDs, err := repo.ExpireOffers(ctx, time.Hour)
		if err != nil {
			t.Fatalf("ExpireOffers() error: %v", err)
		}
		got := make(map[string]bool, len(expiredIDs))
		for _, id := range expiredIDs {
			got[id] = true
		}
		if len(got) != 1 || !got["asg-stale"] {
			t.Errorf("ExpireOffers() = %v, want [asg-stale]", expiredIDs)
		}

		staleGot, err := repo.FindByID(ctx, "asg-stale")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if staleGot == nil || staleGot.Status != domain.AssignmentStatusExpired {
			t.Errorf("stale assignment status = %+v, want EXPIRED", staleGot)
		}

		futureGot, err := repo.FindByID(ctx, "asg-future")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if futureGot == nil || futureGot.Status != domain.AssignmentStatusOffered {
			t.Errorf("future assignment status = %+v, want OFFERED", futureGot)
		}

		acceptedGot, err := repo.FindByID(ctx, "asg-accepted")
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if acceptedGot == nil || acceptedGot.Status != domain.AssignmentStatusAccepted {
			t.Errorf("accepted assignment status = %+v, want ACCEPTED", acceptedGot)
		}
	})

	t.Run("returns an empty result when nothing expired", func(t *testing.T) {
		expiredIDs, err := repo.ExpireOffers(ctx, time.Hour)
		if err != nil {
			t.Fatalf("ExpireOffers() error: %v", err)
		}
		if len(expiredIDs) != 0 {
			t.Errorf("ExpireOffers() = %v, want an empty result", expiredIDs)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		expiredIDs, err := repo.ExpireOffers(cctx, time.Hour)
		if err == nil {
			t.Fatal("ExpireOffers() expected an error for a cancelled context, got nil")
		}
		if expiredIDs != nil {
			t.Errorf("ExpireOffers() = %v, want nil on error", expiredIDs)
		}
	})
}

func TestAssignmentRepositoryEnsureIndexes(t *testing.T) {
	repo, col := newAssignmentTestRepo(t)
	ctx := context.Background()

	t.Run("creates the expected indexes", func(t *testing.T) {
		if err := repo.EnsureIndexes(ctx); err != nil {
			t.Fatalf("EnsureIndexes() error: %v", err)
		}
		names := indexNames(t, col)
		for _, want := range []string{"driverId_1_status_1", "deliveryId_1_status_1", "expiresAt_1", "createdAt_-1"} {
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

func TestAssignmentRepositoryFindRejectedDriverIDsByDelivery(t *testing.T) {
	repo, col := newAssignmentTestRepo(t)
	ctx := context.Background()

	seed := []struct {
		id     string
		driver string
		del    string
		status domain.AssignmentStatus
	}{
		{"asg-r1", "drv-1", "del-x", domain.AssignmentStatusRejected},
		{"asg-r2", "drv-1", "del-x", domain.AssignmentStatusRejected}, // duplicate driver
		{"asg-r3", "drv-2", "del-x", domain.AssignmentStatusRejected},
		{"asg-r4", "", "del-x", domain.AssignmentStatusRejected}, // empty driver id is skipped
		{"asg-r5", "drv-3", "del-x", domain.AssignmentStatusAccepted},
		{"asg-r6", "drv-4", "del-y", domain.AssignmentStatusRejected},
	}
	for _, s := range seed {
		if err := repo.Save(ctx, testAssignment(s.id, s.driver, s.del, s.status)); err != nil {
			t.Fatalf("Save(%s) error: %v", s.id, err)
		}
	}
	// A document written with the legacy lowercase deliveryid key must be
	// matched by the $or filter as well (driverId keeps the struct tag so the
	// field survives decoding).
	if _, err := col.InsertOne(ctx, bson.M{"_id": "asg-legacy", "deliveryid": "del-x", "driverId": "drv-5", "status": "REJECTED"}); err != nil {
		t.Fatalf("seed legacy document: %v", err)
	}

	t.Run("returns the deduplicated rejected driver ids", func(t *testing.T) {
		got, err := repo.FindRejectedDriverIDsByDelivery(ctx, "del-x")
		if err != nil {
			t.Fatalf("FindRejectedDriverIDsByDelivery() error: %v", err)
		}
		seen := make(map[string]bool, len(got))
		for _, id := range got {
			if seen[id] {
				t.Errorf("driver id %q appears more than once in %v", id, got)
			}
			seen[id] = true
		}
		want := map[string]bool{"drv-1": true, "drv-2": true, "drv-5": true}
		if len(seen) != len(want) {
			t.Errorf("FindRejectedDriverIDsByDelivery() = %v, want %v", got, want)
		}
		for id := range want {
			if !seen[id] {
				t.Errorf("FindRejectedDriverIDsByDelivery() missing %q (got %v)", id, got)
			}
		}
	})

	t.Run("returns an empty result when nothing was rejected", func(t *testing.T) {
		got, err := repo.FindRejectedDriverIDsByDelivery(ctx, "del-none")
		if err != nil {
			t.Fatalf("FindRejectedDriverIDsByDelivery() error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("FindRejectedDriverIDsByDelivery() = %v, want an empty result", got)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.FindRejectedDriverIDsByDelivery(cctx, "del-x")
		if err == nil {
			t.Fatal("FindRejectedDriverIDsByDelivery() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindRejectedDriverIDsByDelivery() = %v, want nil on error", got)
		}
	})
}

func TestAssignmentRepositoryCountByDelivery(t *testing.T) {
	repo, col := newAssignmentTestRepo(t)
	ctx := context.Background()

	seed := []struct {
		id  string
		del string
	}{
		{"asg-c1", "del-c"},
		{"asg-c2", "del-c"},
		{"asg-c3", "del-other"},
	}
	for _, s := range seed {
		if err := repo.Save(ctx, testAssignment(s.id, "drv-"+s.id, s.del, domain.AssignmentStatusOffered)); err != nil {
			t.Fatalf("Save(%s) error: %v", s.id, err)
		}
	}
	if _, err := col.InsertOne(ctx, bson.M{"_id": "asg-legacy", "deliveryid": "del-c"}); err != nil {
		t.Fatalf("seed legacy document: %v", err)
	}

	tests := []struct {
		name  string
		delID string
		want  int
	}{
		{"counts every assignment of the delivery", "del-c", 3},
		{"counts zero for an unknown delivery", "del-none", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.CountByDelivery(ctx, tt.delID)
			if err != nil {
				t.Fatalf("CountByDelivery() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("CountByDelivery() = %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.CountByDelivery(cctx, "del-c")
		if err == nil {
			t.Fatal("CountByDelivery() expected an error for a cancelled context, got nil")
		}
		if got != 0 {
			t.Errorf("CountByDelivery() = %d, want 0 on error", got)
		}
	})
}

func TestAssignmentRepositoryFindRejectedByDelivery(t *testing.T) {
	repo, col := newAssignmentTestRepo(t)
	ctx := context.Background()

	seed := []struct {
		id     string
		status domain.AssignmentStatus
	}{
		{"asg-rejected", domain.AssignmentStatusRejected},
		{"asg-expired", domain.AssignmentStatusExpired},
		{"asg-cancelled", domain.AssignmentStatusCancelled},
		{"asg-accepted", domain.AssignmentStatusAccepted},
	}
	for _, s := range seed {
		if err := repo.Save(ctx, testAssignment(s.id, "drv-"+s.id, "del-r", s.status)); err != nil {
			t.Fatalf("Save(%s) error: %v", s.id, err)
		}
	}
	// Legacy lowercase deliveryid key must be matched too.
	if _, err := col.InsertOne(ctx, bson.M{"_id": "asg-legacy", "deliveryid": "del-r", "driverid": "drv-legacy", "status": "REJECTED"}); err != nil {
		t.Fatalf("seed legacy document: %v", err)
	}

	t.Run("returns rejected, expired and cancelled assignments", func(t *testing.T) {
		got, err := repo.FindRejectedByDelivery(ctx, "del-r")
		if err != nil {
			t.Fatalf("FindRejectedByDelivery() error: %v", err)
		}
		ids := assignmentIDs(got)
		want := map[string]bool{
			"asg-rejected":  true,
			"asg-expired":   true,
			"asg-cancelled": true,
			"asg-legacy":    true,
		}
		if len(ids) != len(want) {
			t.Errorf("FindRejectedByDelivery() = %v, want %v", ids, want)
		}
		for id := range want {
			if !ids[id] {
				t.Errorf("FindRejectedByDelivery() missing %q (got %v)", id, ids)
			}
		}
	})

	t.Run("returns an empty result for an unknown delivery", func(t *testing.T) {
		got, err := repo.FindRejectedByDelivery(ctx, "del-none")
		if err != nil {
			t.Fatalf("FindRejectedByDelivery() error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("FindRejectedByDelivery() = %v, want an empty result", got)
		}
	})

	t.Run("propagates a cancelled context", func(t *testing.T) {
		cctx, cancel := cancelledCtx()
		defer cancel()
		got, err := repo.FindRejectedByDelivery(cctx, "del-r")
		if err == nil {
			t.Fatal("FindRejectedByDelivery() expected an error for a cancelled context, got nil")
		}
		if got != nil {
			t.Errorf("FindRejectedByDelivery() = %v, want nil on error", got)
		}
	})
}
