package mongodb_test

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// The repositories decode documents straight into the domain structs, so a
// document whose types do not match makes cursor.All / Decode fail. That is
// the only way to reach those error branches: a context that is already
// cancelled makes the preceding Find fail instead, and a context that expires
// in between the two calls would be a race.
func TestRepositories_CursorDecodeFailures(t *testing.T) {
	requireMongo(t)
	ctx := context.Background()

	t.Run("ExpireOffers", func(t *testing.T) {
		repo, col := newAssignmentTestRepo(t)
		if _, err := col.InsertOne(ctx, bson.M{
			"_id": "asg-expire-bad", "status": "OFFERED",
			"expiresAt":     time.Now().Add(-2 * time.Hour),
			"attemptNumber": "two",
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}

		got, err := repo.ExpireOffers(ctx, time.Hour)
		if err == nil {
			t.Fatal("ExpireOffers() error = nil, want a decode error")
		}
		if got != nil {
			t.Errorf("ExpireOffers() ids = %v, want nil on error", got)
		}
	})

	t.Run("FindRejectedDriverIDsByDelivery", func(t *testing.T) {
		repo, col := newAssignmentTestRepo(t)
		if _, err := col.InsertOne(ctx, bson.M{
			"_id": "asg-rej-bad", "deliveryId": "del-rej-bad", "status": "REJECTED",
			"driverId": 7,
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}

		got, err := repo.FindRejectedDriverIDsByDelivery(ctx, "del-rej-bad")
		if err == nil {
			t.Fatal("FindRejectedDriverIDsByDelivery() error = nil, want a decode error")
		}
		if got != nil {
			t.Errorf("FindRejectedDriverIDsByDelivery() = %v, want nil on error", got)
		}
	})

	t.Run("FindRejectedByDelivery", func(t *testing.T) {
		repo, col := newAssignmentTestRepo(t)
		if _, err := col.InsertOne(ctx, bson.M{
			"_id": "asg-rej-bad2", "deliveryId": "del-rej-bad2", "status": "CANCELLED",
			"createdAt": "not-a-time",
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}

		got, err := repo.FindRejectedByDelivery(ctx, "del-rej-bad2")
		if err == nil {
			t.Fatal("FindRejectedByDelivery() error = nil, want a decode error")
		}
		if got != nil {
			t.Errorf("FindRejectedByDelivery() = %v, want nil on error", got)
		}
	})

	t.Run("FindAvailableByLocation", func(t *testing.T) {
		repo, col := newDriverTestRepo(t)
		if _, err := col.InsertOne(ctx, bson.M{
			"_id": "drv-loc-bad", "status": "AVAILABLE",
			"isBlocked": false, "isActive": true,
			"rating": "high",
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}

		got, err := repo.FindAvailableByLocation(ctx, 30, 31, 100, "")
		if err == nil {
			t.Fatal("FindAvailableByLocation() error = nil, want a decode error")
		}
		if got != nil {
			t.Errorf("FindAvailableByLocation() = %v, want nil on error", got)
		}
	})

	t.Run("FindByStatus", func(t *testing.T) {
		repo, col := newDriverTestRepo(t)
		if _, err := col.InsertOne(ctx, bson.M{
			"_id": "drv-status-bad", "status": "OFFLINE",
			"userId": 7,
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}

		got, err := repo.FindByStatus(ctx, "OFFLINE")
		if err == nil {
			t.Fatal("FindByStatus() error = nil, want a decode error")
		}
		if got != nil {
			t.Errorf("FindByStatus() = %v, want nil on error", got)
		}
	})
}
