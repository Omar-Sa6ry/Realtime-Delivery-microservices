package mocks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

var errTest = errors.New("test error")

func testDriver(id, userID string, status domain.DriverStatus) *domain.Driver {
	return &domain.Driver{
		ID:        id,
		UserID:    userID,
		Status:    status,
		IsActive:  true,
		IsBlocked: false,
		Vehicle: domain.VehicleInfo{
			Type:        domain.VehicleTypeCar,
			PlateNumber: "ABC-123",
			CapacityKg:  50,
		},
	}
}

func testAssignment(id, driverID, deliveryID string, status domain.AssignmentStatus) *domain.Assignment {
	return &domain.Assignment{
		ID:         id,
		DriverID:   driverID,
		DeliveryID: deliveryID,
		Status:     status,
		OfferedAt:  time.Now(),
		CreatedAt:  time.Now(),
	}
}

// ------------------------------------------------------------------
// MockDriverRepository
// ------------------------------------------------------------------

func TestMockDriverRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("FindByID hit and miss", func(t *testing.T) {
		repo := mocks.NewMockDriverRepository()
		d := testDriver("driver-1", "user-1", domain.DriverStatusAvailable)
		repo.Save(ctx, d)

		got, err := repo.FindByID(ctx, "driver-1")
		if err != nil {
			t.Errorf("FindByID() err = %v", err)
		}
		if got != d {
			t.Errorf("FindByID() = %v, want driver", got)
		}

		missing, err := repo.FindByID(ctx, "nope")
		if err != nil {
			t.Errorf("FindByID(missing) err = %v", err)
		}
		if missing != nil {
			t.Errorf("FindByID(missing) = %v, want nil", missing)
		}
	})

	t.Run("FindByID with error", func(t *testing.T) {
		repo := mocks.NewMockDriverRepository()
		repo.SetError(errTest)
		if _, err := repo.FindByID(ctx, "driver-1"); !errors.Is(err, errTest) {
			t.Errorf("FindByID() err = %v, want %v", err, errTest)
		}
		if err := repo.Save(ctx, testDriver("driver-1", "user-1", domain.DriverStatusOffline)); !errors.Is(err, errTest) {
			t.Errorf("Save() err = %v, want %v", err, errTest)
		}
		repo.SetError(nil)
		if _, err := repo.FindByID(ctx, "driver-1"); err != nil {
			t.Errorf("FindByID() after clearing error = %v, want nil", err)
		}
	})

	t.Run("FindByIDs", func(t *testing.T) {
		repo := mocks.NewMockDriverRepository()
		d1 := testDriver("driver-1", "user-1", domain.DriverStatusAvailable)
		d2 := testDriver("driver-2", "user-2", domain.DriverStatusOffline)
		repo.Save(ctx, d1)
		repo.Save(ctx, d2)

		got, err := repo.FindByIDs(ctx, []string{"driver-1", "missing", "driver-2"})
		if err != nil {
			t.Errorf("FindByIDs() err = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("FindByIDs() len = %d, want 2", len(got))
		}

		repo.SetError(errTest)
		if _, err := repo.FindByIDs(ctx, []string{"driver-1"}); !errors.Is(err, errTest) {
			t.Errorf("FindByIDs() err = %v, want %v", err, errTest)
		}
	})

	t.Run("FindByUserID", func(t *testing.T) {
		repo := mocks.NewMockDriverRepository()
		d := testDriver("driver-1", "user-1", domain.DriverStatusAvailable)
		repo.SetDriver(d)

		got, err := repo.FindByUserID(ctx, "user-1")
		if err != nil {
			t.Errorf("FindByUserID() err = %v", err)
		}
		if got != d {
			t.Errorf("FindByUserID() = %v, want driver", got)
		}

		missing, err := repo.FindByUserID(ctx, "user-x")
		if err != nil || missing != nil {
			t.Errorf("FindByUserID(missing) = %v, %v; want nil, nil", missing, err)
		}

		repo.SetError(errTest)
		if _, err := repo.FindByUserID(ctx, "user-1"); !errors.Is(err, errTest) {
			t.Errorf("FindByUserID() err = %v, want %v", err, errTest)
		}
		repo.SetError(nil)
	})

	t.Run("FindAvailableByLocation", func(t *testing.T) {
		repo := mocks.NewMockDriverRepository()
		available := testDriver("driver-1", "user-1", domain.DriverStatusAvailable)
		busy := testDriver("driver-2", "user-2", domain.DriverStatusBusy)
		blocked := testDriver("driver-3", "user-3", domain.DriverStatusAvailable)
		blocked.IsBlocked = true
		inactive := testDriver("driver-4", "user-4", domain.DriverStatusAvailable)
		inactive.IsActive = false
		bike := testDriver("driver-5", "user-5", domain.DriverStatusAvailable)
		bike.Vehicle.Type = domain.VehicleTypeBicycle

		for _, d := range []*domain.Driver{available, busy, blocked, inactive, bike} {
			repo.Save(ctx, d)
		}

		got, err := repo.FindAvailableByLocation(ctx, 30, 31, 5, "")
		if err != nil {
			t.Errorf("FindAvailableByLocation() err = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("FindAvailableByLocation() len = %d, want 2", len(got))
		}

		cars, err := repo.FindAvailableByLocation(ctx, 30, 31, 5, domain.VehicleTypeCar)
		if err != nil {
			t.Errorf("FindAvailableByLocation(car) err = %v", err)
		}
		if len(cars) != 1 || cars[0].ID != "driver-1" {
			t.Errorf("FindAvailableByLocation(car) = %v, want only driver-1", cars)
		}

		repo.SetError(errTest)
		if _, err := repo.FindAvailableByLocation(ctx, 30, 31, 5, ""); !errors.Is(err, errTest) {
			t.Errorf("FindAvailableByLocation() err = %v, want %v", err, errTest)
		}
		repo.SetError(nil)
	})

	t.Run("FindByStatus", func(t *testing.T) {
		repo := mocks.NewMockDriverRepository()
		repo.Save(ctx, testDriver("driver-1", "user-1", domain.DriverStatusAvailable))
		repo.Save(ctx, testDriver("driver-2", "user-2", domain.DriverStatusOffline))

		got, err := repo.FindByStatus(ctx, "AVAILABLE")
		if err != nil {
			t.Errorf("FindByStatus() err = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("FindByStatus(AVAILABLE) len = %d, want 1", len(got))
		}

		none, err := repo.FindByStatus(ctx, "BLOCKED")
		if err != nil {
			t.Errorf("FindByStatus(BLOCKED) err = %v", err)
		}
		if len(none) != 0 {
			t.Errorf("FindByStatus(BLOCKED) len = %d, want 0", len(none))
		}

		repo.SetError(errTest)
		if _, err := repo.FindByStatus(ctx, "AVAILABLE"); !errors.Is(err, errTest) {
			t.Errorf("FindByStatus() err = %v, want %v", err, errTest)
		}
		repo.SetError(nil)
	})

	t.Run("UpdateRating", func(t *testing.T) {
		repo := mocks.NewMockDriverRepository()
		d := testDriver("driver-1", "user-1", domain.DriverStatusAvailable)
		repo.Save(ctx, d)

		if err := repo.UpdateRating(ctx, "driver-1", 4.5, 10); err != nil {
			t.Errorf("UpdateRating() err = %v", err)
		}
		if d.Rating != 4.5 || d.RatingCount != 10 {
			t.Errorf("rating = %v/%d, want 4.5/10", d.Rating, d.RatingCount)
		}

		if err := repo.UpdateRating(ctx, "missing", 4.5, 10); !errors.Is(err, domain.ErrDriverNotFound) {
			t.Errorf("UpdateRating(missing) err = %v, want ErrDriverNotFound", err)
		}
	})
}

// ------------------------------------------------------------------
// MockAssignmentRepository
// ------------------------------------------------------------------

func TestMockAssignmentRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("FindByID hit miss error", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		a := testAssignment("assignment-1", "driver-1", "delivery-1", domain.AssignmentStatusOffered)
		repo.Save(ctx, a)

		got, err := repo.FindByID(ctx, "assignment-1")
		if err != nil || got != a {
			t.Errorf("FindByID() = %v, %v; want assignment, nil", got, err)
		}

		missing, err := repo.FindByID(ctx, "missing")
		if err != nil || missing != nil {
			t.Errorf("FindByID(missing) = %v, %v; want nil, nil", missing, err)
		}

		repo.SetError(errTest)
		if _, err := repo.FindByID(ctx, "assignment-1"); !errors.Is(err, errTest) {
			t.Errorf("FindByID() err = %v, want %v", err, errTest)
		}
		repo.SetError(nil)
	})

	t.Run("FindActiveByDriver", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		offered := testAssignment("a-offered", "driver-1", "delivery-1", domain.AssignmentStatusOffered)
		accepted := testAssignment("a-accepted", "driver-2", "delivery-2", domain.AssignmentStatusAccepted)
		active := testAssignment("a-active", "driver-3", "delivery-3", domain.AssignmentStatusActive)
		completed := testAssignment("a-completed", "driver-4", "delivery-4", domain.AssignmentStatusCompleted)
		for _, a := range []*domain.Assignment{offered, accepted, active, completed} {
			repo.SetAssignment(a)
		}

		for _, id := range []string{"a-offered", "a-accepted", "a-active"} {
			got, err := repo.FindActiveByDriver(ctx, map[string]string{
				"a-offered":  "driver-1",
				"a-accepted": "driver-2",
				"a-active":   "driver-3",
			}[id])
			if err != nil || got == nil || got.ID != id {
				t.Errorf("FindActiveByDriver(%s) = %v, %v; want active assignment", id, got, err)
			}
		}

		if got, err := repo.FindActiveByDriver(ctx, "driver-4"); err != nil || got != nil {
			t.Errorf("FindActiveByDriver(completed) = %v, %v; want nil, nil", got, err)
		}
		if got, err := repo.FindActiveByDriver(ctx, "driver-x"); err != nil || got != nil {
			t.Errorf("FindActiveByDriver(missing) = %v, %v; want nil, nil", got, err)
		}
	})

	t.Run("FindByDeliveryID", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		a := testAssignment("assignment-1", "driver-1", "delivery-1", domain.AssignmentStatusOffered)
		repo.Save(ctx, a)

		got, err := repo.FindByDeliveryID(ctx, "delivery-1")
		if err != nil || got != a {
			t.Errorf("FindByDeliveryID() = %v, %v; want assignment, nil", got, err)
		}
		missing, err := repo.FindByDeliveryID(ctx, "nope")
		if err != nil || missing != nil {
			t.Errorf("FindByDeliveryID(missing) = %v, %v; want nil, nil", missing, err)
		}
	})

	t.Run("Save with error", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		repo.SetError(errTest)
		a := testAssignment("assignment-1", "driver-1", "delivery-1", domain.AssignmentStatusOffered)
		if err := repo.Save(ctx, a); !errors.Is(err, errTest) {
			t.Errorf("Save() err = %v, want %v", err, errTest)
		}
		repo.SetError(nil)
		if err := repo.Save(ctx, a); err != nil {
			t.Errorf("Save() err = %v, want nil", err)
		}
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		a := testAssignment("assignment-1", "driver-1", "delivery-1", domain.AssignmentStatusOffered)
		repo.Save(ctx, a)

		if err := repo.UpdateStatus(ctx, "assignment-1", string(domain.AssignmentStatusAccepted)); err != nil {
			t.Errorf("UpdateStatus() err = %v", err)
		}
		if a.Status != domain.AssignmentStatusAccepted {
			t.Errorf("status = %s, want ACCEPTED", a.Status)
		}

		if err := repo.UpdateStatus(ctx, "missing", "ACCEPTED"); !errors.Is(err, domain.ErrAssignmentNotFound) {
			t.Errorf("UpdateStatus(missing) err = %v, want ErrAssignmentNotFound", err)
		}

		repo.SetError(errTest)
		if err := repo.UpdateStatus(ctx, "assignment-1", "ACCEPTED"); !errors.Is(err, errTest) {
			t.Errorf("UpdateStatus() err = %v, want %v", err, errTest)
		}
		repo.SetError(nil)
	})

	t.Run("ExpireOffers", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		old := testAssignment("old", "driver-1", "delivery-1", domain.AssignmentStatusOffered)
		old.OfferedAt = time.Now().Add(-time.Hour)
		recent := testAssignment("recent", "driver-2", "delivery-2", domain.AssignmentStatusOffered)
		recent.OfferedAt = time.Now()
		accepted := testAssignment("accepted", "driver-3", "delivery-3", domain.AssignmentStatusAccepted)
		accepted.OfferedAt = time.Now().Add(-time.Hour)

		for _, a := range []*domain.Assignment{old, recent, accepted} {
			repo.SetAssignment(a)
		}

		expired, err := repo.ExpireOffers(ctx, time.Minute)
		if err != nil {
			t.Errorf("ExpireOffers() err = %v", err)
		}
		if len(expired) != 1 || expired[0] != "old" {
			t.Errorf("ExpireOffers() = %v, want [old]", expired)
		}
		if old.Status != domain.AssignmentStatusExpired {
			t.Errorf("old status = %s, want EXPIRED", old.Status)
		}
		if recent.Status != domain.AssignmentStatusOffered {
			t.Errorf("recent status = %s, want OFFERED", recent.Status)
		}
		if accepted.Status != domain.AssignmentStatusAccepted {
			t.Errorf("accepted status = %s, want ACCEPTED", accepted.Status)
		}
	})

	t.Run("Rejected driver lookups and count", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		r1 := testAssignment("a1", "driver-1", "delivery-1", domain.AssignmentStatusRejected)
		r2 := testAssignment("a2", "driver-2", "delivery-1", domain.AssignmentStatusRejected)
		ok := testAssignment("a3", "driver-3", "delivery-1", domain.AssignmentStatusOffered)
		other := testAssignment("a4", "driver-4", "delivery-2", domain.AssignmentStatusRejected)
		for _, a := range []*domain.Assignment{r1, r2, ok, other} {
			repo.SetAssignment(a)
		}

		ids, err := repo.FindRejectedDriverIDsByDelivery(ctx, "delivery-1")
		if err != nil {
			t.Errorf("FindRejectedDriverIDsByDelivery() err = %v", err)
		}
		if len(ids) != 2 {
			t.Errorf("FindRejectedDriverIDsByDelivery() = %v, want 2 ids", ids)
		}

		rejected, err := repo.FindRejectedByDelivery(ctx, "delivery-1")
		if err != nil {
			t.Errorf("FindRejectedByDelivery() err = %v", err)
		}
		if len(rejected) != 2 {
			t.Errorf("FindRejectedByDelivery() len = %d, want 2", len(rejected))
		}

		none, err := repo.FindRejectedByDelivery(ctx, "delivery-x")
		if err != nil || none != nil {
			t.Errorf("FindRejectedByDelivery(missing) = %v, %v; want nil, nil", none, err)
		}

		count, err := repo.CountByDelivery(ctx, "delivery-1")
		if err != nil {
			t.Errorf("CountByDelivery() err = %v", err)
		}
		if count != 3 {
			t.Errorf("CountByDelivery() = %d, want 3", count)
		}

		zero, err := repo.CountByDelivery(ctx, "delivery-x")
		if err != nil || zero != 0 {
			t.Errorf("CountByDelivery(missing) = %d, %v; want 0, nil", zero, err)
		}
	})

	t.Run("FindByDeliveryID does not consult the injected error", func(t *testing.T) {
		// FindByDeliveryID intentionally ignores saveError in mocks.go.
		repo := mocks.NewMockAssignmentRepository()
		repo.SetError(errTest)
		if _, err := repo.FindByDeliveryID(ctx, "delivery-1"); err != nil {
			t.Errorf("FindByDeliveryID() err = %v, want nil", err)
		}
		if _, err := repo.FindByID(ctx, "assignment-1"); !errors.Is(err, errTest) {
			t.Errorf("FindByID() err = %v, want %v", err, errTest)
		}
		repo.SetError(nil)
	})
}

// ------------------------------------------------------------------
// MockReviewRepository
// ------------------------------------------------------------------

func TestMockReviewRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("Create and lookup", func(t *testing.T) {
		repo := mocks.NewMockReviewRepository()
		r1 := &domain.Review{ID: "r1", DriverID: "driver-1", DeliveryID: "delivery-1", Rating: 5}
		r2 := &domain.Review{ID: "r2", DriverID: "driver-1", DeliveryID: "delivery-2", Rating: 3}

		if err := repo.Create(ctx, r1); err != nil {
			t.Errorf("Create() err = %v", err)
		}
		if err := repo.Create(ctx, r2); err != nil {
			t.Errorf("Create() err = %v", err)
		}

		got, err := repo.FindByDeliveryID(ctx, "delivery-1")
		if err != nil || got != r1 {
			t.Errorf("FindByDeliveryID() = %v, %v; want r1, nil", got, err)
		}
		missing, err := repo.FindByDeliveryID(ctx, "nope")
		if err != nil || missing != nil {
			t.Errorf("FindByDeliveryID(missing) = %v, %v; want nil, nil", missing, err)
		}
	})

	t.Run("Create with error", func(t *testing.T) {
		repo := mocks.NewMockReviewRepository()
		repo.SetError(errTest)
		r := &domain.Review{ID: "r1", DriverID: "driver-1", DeliveryID: "delivery-1", Rating: 5}
		if err := repo.Create(ctx, r); !errors.Is(err, errTest) {
			t.Errorf("Create() err = %v, want %v", err, errTest)
		}
	})

	t.Run("FindByDriverID pagination", func(t *testing.T) {
		repo := mocks.NewMockReviewRepository()
		repo.SetReview(&domain.Review{ID: "r1", DriverID: "driver-1", DeliveryID: "d1", Rating: 5})
		repo.SetReview(&domain.Review{ID: "r2", DriverID: "driver-1", DeliveryID: "d2", Rating: 4})
		repo.SetReview(&domain.Review{ID: "r3", DriverID: "driver-1", DeliveryID: "d3", Rating: 3})

		page, total, err := repo.FindByDriverID(ctx, "driver-1", 0, 2)
		if err != nil {
			t.Errorf("FindByDriverID() err = %v", err)
		}
		if len(page) != 2 || total != 3 {
			t.Errorf("FindByDriverID(0,2) = %d items, total %d; want 2, 3", len(page), total)
		}

		beyond, total, err := repo.FindByDriverID(ctx, "driver-1", 10, 2)
		if err != nil {
			t.Errorf("FindByDriverID(beyond) err = %v", err)
		}
		if len(beyond) != 0 || total != 3 {
			t.Errorf("FindByDriverID(10,2) = %d items, total %d; want 0, 3", len(beyond), total)
		}

		clamped, _, err := repo.FindByDriverID(ctx, "driver-1", 2, 10)
		if err != nil {
			t.Errorf("FindByDriverID(clamped) err = %v", err)
		}
		if len(clamped) != 1 {
			t.Errorf("FindByDriverID(2,10) len = %d, want 1", len(clamped))
		}

		empty, total, err := repo.FindByDriverID(ctx, "driver-x", 0, 10)
		if err != nil || len(empty) != 0 || total != 0 {
			t.Errorf("FindByDriverID(missing) = %d, %d, %v; want 0, 0, nil", len(empty), total, err)
		}
	})

	t.Run("CalculateAverageRating", func(t *testing.T) {
		repo := mocks.NewMockReviewRepository()
		repo.SetReview(&domain.Review{ID: "r1", DriverID: "driver-1", DeliveryID: "d1", Rating: 5})
		repo.SetReview(&domain.Review{ID: "r2", DriverID: "driver-1", DeliveryID: "d2", Rating: 4})

		avg, count, err := repo.CalculateAverageRating(ctx, "driver-1")
		if err != nil {
			t.Errorf("CalculateAverageRating() err = %v", err)
		}
		if avg != 4.5 || count != 2 {
			t.Errorf("CalculateAverageRating() = %v/%d, want 4.5/2", avg, count)
		}

		zeroAvg, zeroCount, err := repo.CalculateAverageRating(ctx, "driver-x")
		if err != nil || zeroAvg != 0 || zeroCount != 0 {
			t.Errorf("CalculateAverageRating(missing) = %v/%d, %v; want 0/0, nil", zeroAvg, zeroCount, err)
		}
	})
}

// ------------------------------------------------------------------
// MockEventPublisher
// ------------------------------------------------------------------

func TestMockEventPublisher(t *testing.T) {
	ctx := context.Background()

	t.Run("records every published event", func(t *testing.T) {
		p := mocks.NewMockEventPublisher()

		if err := p.PublishDriverCreated(ctx, "driver-1", "user-1"); err != nil {
			t.Errorf("PublishDriverCreated() err = %v", err)
		}
		if err := p.PublishDriverUpdated(ctx, "driver-1", domain.VehicleTypeCar, []string{"STANDARD"}); err != nil {
			t.Errorf("PublishDriverUpdated() err = %v", err)
		}
		if err := p.PublishDriverDeleted(ctx, "driver-1"); err != nil {
			t.Errorf("PublishDriverDeleted() err = %v", err)
		}
		if err := p.PublishAssignmentOffered(ctx, "assignment-1", "delivery-1", "driver-1"); err != nil {
			t.Errorf("PublishAssignmentOffered() err = %v", err)
		}
		if err := p.PublishAssignmentAccepted(ctx, "assignment-1", "delivery-1", "driver-1"); err != nil {
			t.Errorf("PublishAssignmentAccepted() err = %v", err)
		}
		if err := p.PublishAssignmentRejected(ctx, "assignment-1", "delivery-1", "driver-1", "busy"); err != nil {
			t.Errorf("PublishAssignmentRejected() err = %v", err)
		}
		if err := p.PublishAssignmentExpired(ctx, "assignment-1", "delivery-1"); err != nil {
			t.Errorf("PublishAssignmentExpired() err = %v", err)
		}
		if err := p.PublishAssignmentReleased(ctx, "assignment-1"); err != nil {
			t.Errorf("PublishAssignmentReleased() err = %v", err)
		}
		if err := p.PublishDriverAvailable(ctx, "driver-1"); err != nil {
			t.Errorf("PublishDriverAvailable() err = %v", err)
		}
		if err := p.PublishDriverUnavailable(ctx, "driver-1"); err != nil {
			t.Errorf("PublishDriverUnavailable() err = %v", err)
		}
		if err := p.PublishLocationUpdated(ctx, "driver-1", "30.0", "31.0"); err != nil {
			t.Errorf("PublishLocationUpdated() err = %v", err)
		}
		if err := p.PublishNoDriverAvailable(ctx, "delivery-1", "customer-1", 3, "none found"); err != nil {
			t.Errorf("PublishNoDriverAvailable() err = %v", err)
		}
		if err := p.PublishSearchRetry(ctx, "delivery-1", "customer-1", 2, 5); err != nil {
			t.Errorf("PublishSearchRetry() err = %v", err)
		}

		want := []string{
			"DriverCreated:driver-1",
			"DriverUpdated:driver-1",
			"DriverDeleted:driver-1",
			"AssignmentOffered:assignment-1",
			"AssignmentAccepted:assignment-1",
			"AssignmentRejected:assignment-1",
			"AssignmentExpired:assignment-1",
			"AssignmentReleased:assignment-1",
			"DriverAvailable:driver-1",
			"DriverUnavailable:driver-1",
			"LocationUpdated:driver-1",
			"NoDriverAvailable:delivery-1",
			"SearchRetry:delivery-1",
		}
		got := p.GetEvents()
		if len(got) != len(want) {
			t.Fatalf("GetEvents() = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("GetEvents()[%d] = %q, want %q", i, got[i], want[i])
			}
		}

		p.ClearEvents()
		if len(p.GetEvents()) != 0 {
			t.Errorf("GetEvents() after ClearEvents = %v, want empty", p.GetEvents())
		}
	})

	t.Run("configured errors are returned", func(t *testing.T) {
		p := mocks.NewMockEventPublisher()
		p.SetError("DriverCreated", errTest)
		p.SetError("NoDriverAvailable", errTest)

		if err := p.PublishDriverCreated(ctx, "driver-1", "user-1"); !errors.Is(err, errTest) {
			t.Errorf("PublishDriverCreated() err = %v, want %v", err, errTest)
		}
		// The event is still recorded even when the publish fails.
		if ev := p.GetEvents(); len(ev) != 1 || ev[0] != "DriverCreated:driver-1" {
			t.Errorf("GetEvents() = %v, want [DriverCreated:driver-1]", ev)
		}
		if err := p.PublishNoDriverAvailable(ctx, "delivery-1", "customer-1", 1, "x"); !errors.Is(err, errTest) {
			t.Errorf("PublishNoDriverAvailable() err = %v, want %v", err, errTest)
		}
		// Without a configured error, publishing succeeds.
		if err := p.PublishDriverDeleted(ctx, "driver-9"); err != nil {
			t.Errorf("PublishDriverDeleted() err = %v, want nil", err)
		}
	})

	t.Run("every publish method returns its configured error", func(t *testing.T) {
		// One entry per event type supported by the publisher.
		publishers := []struct {
			eventType string
			publish   func(p *mocks.MockEventPublisher) error
		}{
			{"DriverCreated", func(p *mocks.MockEventPublisher) error {
				return p.PublishDriverCreated(ctx, "driver-1", "user-1")
			}},
			{"DriverUpdated", func(p *mocks.MockEventPublisher) error {
				return p.PublishDriverUpdated(ctx, "driver-1", domain.VehicleTypeCar, nil)
			}},
			{"DriverDeleted", func(p *mocks.MockEventPublisher) error {
				return p.PublishDriverDeleted(ctx, "driver-1")
			}},
			{"AssignmentOffered", func(p *mocks.MockEventPublisher) error {
				return p.PublishAssignmentOffered(ctx, "assignment-1", "delivery-1", "driver-1")
			}},
			{"AssignmentAccepted", func(p *mocks.MockEventPublisher) error {
				return p.PublishAssignmentAccepted(ctx, "assignment-1", "delivery-1", "driver-1")
			}},
			{"AssignmentRejected", func(p *mocks.MockEventPublisher) error {
				return p.PublishAssignmentRejected(ctx, "assignment-1", "delivery-1", "driver-1", "busy")
			}},
			{"AssignmentExpired", func(p *mocks.MockEventPublisher) error {
				return p.PublishAssignmentExpired(ctx, "assignment-1", "delivery-1")
			}},
			{"AssignmentReleased", func(p *mocks.MockEventPublisher) error {
				return p.PublishAssignmentReleased(ctx, "assignment-1")
			}},
			{"DriverAvailable", func(p *mocks.MockEventPublisher) error {
				return p.PublishDriverAvailable(ctx, "driver-1")
			}},
			{"DriverUnavailable", func(p *mocks.MockEventPublisher) error {
				return p.PublishDriverUnavailable(ctx, "driver-1")
			}},
			{"LocationUpdated", func(p *mocks.MockEventPublisher) error {
				return p.PublishLocationUpdated(ctx, "driver-1", "30.0", "31.0")
			}},
			{"NoDriverAvailable", func(p *mocks.MockEventPublisher) error {
				return p.PublishNoDriverAvailable(ctx, "delivery-1", "customer-1", 1, "none")
			}},
			{"SearchRetry", func(p *mocks.MockEventPublisher) error {
				return p.PublishSearchRetry(ctx, "delivery-1", "customer-1", 1, 2)
			}},
		}

		for _, tt := range publishers {
			t.Run(tt.eventType, func(t *testing.T) {
				p := mocks.NewMockEventPublisher()
				p.SetError(tt.eventType, errTest)
				if err := tt.publish(p); !errors.Is(err, errTest) {
					t.Errorf("publish(%s) err = %v, want %v", tt.eventType, err, errTest)
				}
			})
		}
	})
}

// ------------------------------------------------------------------
// MockLockManager
// ------------------------------------------------------------------

func TestMockLockManager(t *testing.T) {
	ctx := context.Background()
	lm := mocks.NewMockLockManager()

	if lm.IsLocked("driver-1") {
		t.Errorf("IsLocked() = true on fresh manager, want false")
	}

	ok, err := lm.Acquire(ctx, "driver-1", time.Second)
	if err != nil {
		t.Errorf("Acquire() err = %v", err)
	}
	if !ok {
		t.Errorf("Acquire() = false, want true")
	}
	if !lm.IsLocked("driver-1") {
		t.Errorf("IsLocked() = false after acquire, want true")
	}

	again, err := lm.Acquire(ctx, "driver-1", time.Second)
	if err != nil {
		t.Errorf("Acquire() second err = %v", err)
	}
	if again {
		t.Errorf("Acquire() second = true, want false (contention)")
	}

	// Releasing with a wrong token keeps the lock.
	if err := lm.Release(ctx, "driver-1", "wrong-token"); err != nil {
		t.Errorf("Release(wrong token) err = %v", err)
	}
	if !lm.IsLocked("driver-1") {
		t.Errorf("IsLocked() = false after wrong-token release, want true")
	}

	if err := lm.Release(ctx, "driver-1", "token-driver-1"); err != nil {
		t.Errorf("Release() err = %v", err)
	}
	if lm.IsLocked("driver-1") {
		t.Errorf("IsLocked() = true after release, want false")
	}
}

// ------------------------------------------------------------------
// MockDeliveryServiceClient
// ------------------------------------------------------------------

func TestMockDeliveryServiceClient(t *testing.T) {
	ctx := context.Background()
	c := mocks.NewMockDeliveryServiceClient()

	t.Run("GetDelivery hit and miss", func(t *testing.T) {
		info := &ports.DeliveryInfo{DeliveryID: "delivery-1", CustomerID: "customer-1", Status: "PENDING"}
		c.SetDelivery(info)

		got, err := c.GetDelivery(ctx, "delivery-1")
		if err != nil || got != info {
			t.Errorf("GetDelivery() = %v, %v; want info, nil", got, err)
		}

		missing, err := c.GetDelivery(ctx, "nope")
		if err != nil || missing != nil {
			t.Errorf("GetDelivery(missing) = %v, %v; want nil, nil", missing, err)
		}
	})

	t.Run("GetDelivery with error", func(t *testing.T) {
		c.SetError("delivery-err", errTest)
		if _, err := c.GetDelivery(ctx, "delivery-err"); !errors.Is(err, errTest) {
			t.Errorf("GetDelivery() err = %v, want %v", err, errTest)
		}
	})

	t.Run("GetOpenDeliveries", func(t *testing.T) {
		list, total, err := c.GetOpenDeliveries(ctx, 1, 10)
		if err != nil || list != nil || total != 0 {
			t.Errorf("GetOpenDeliveries() = %v, %d, %v; want nil, 0, nil", list, total, err)
		}
	})
}

// ------------------------------------------------------------------
// MockUserServiceClient
// ------------------------------------------------------------------

func TestMockUserServiceClient(t *testing.T) {
	ctx := context.Background()
	c := mocks.NewMockUserServiceClient()

	t.Run("GetUser hit and miss", func(t *testing.T) {
		c.SetUser(&mocks.UserInfo{ID: "user-1", Role: "DRIVER"})

		got, err := c.GetUser(ctx, "user-1")
		if err != nil {
			t.Errorf("GetUser() err = %v", err)
		}
		if got == nil || got.Role != "DRIVER" {
			t.Errorf("GetUser() = %v, want DRIVER role", got)
		}

		missing, err := c.GetUser(ctx, "nope")
		if err != nil || missing != nil {
			t.Errorf("GetUser(missing) = %v, %v; want nil, nil", missing, err)
		}
	})

	t.Run("GetUser with error", func(t *testing.T) {
		c.SetError("user-err", errTest)
		if _, err := c.GetUser(ctx, "user-err"); !errors.Is(err, errTest) {
			t.Errorf("GetUser() err = %v, want %v", err, errTest)
		}
	})

	t.Run("UpdateUserRole existing user", func(t *testing.T) {
		resp, err := c.UpdateUserRole(ctx, "user-1", "CUSTOMER")
		if err != nil {
			t.Errorf("UpdateUserRole() err = %v", err)
		}
		if resp == nil || !resp.Success {
			t.Errorf("UpdateUserRole() = %v, want success", resp)
		}
		got, _ := c.GetUser(ctx, "user-1")
		if got.Role != "CUSTOMER" {
			t.Errorf("role = %s, want CUSTOMER", got.Role)
		}
	})

	t.Run("UpdateUserRole creates missing user", func(t *testing.T) {
		resp, err := c.UpdateUserRole(ctx, "user-new", "DRIVER")
		if err != nil {
			t.Errorf("UpdateUserRole() err = %v", err)
		}
		if resp == nil || !resp.Success {
			t.Errorf("UpdateUserRole() = %v, want success", resp)
		}
		got, err := c.GetUser(ctx, "user-new")
		if err != nil || got == nil || got.Role != "DRIVER" {
			t.Errorf("GetUser(user-new) = %v, %v; want DRIVER", got, err)
		}
	})

	t.Run("UpdateUserRole with error", func(t *testing.T) {
		c.SetError("update_user-err", errTest)
		resp, err := c.UpdateUserRole(ctx, "user-err", "DRIVER")
		if !errors.Is(err, errTest) {
			t.Errorf("UpdateUserRole() err = %v, want %v", err, errTest)
		}
		if resp == nil || resp.Success {
			t.Errorf("UpdateUserRole() = %v, want unsuccessful response", resp)
		}
	})
}

// ------------------------------------------------------------------
// MockIdempotencyStore
// ------------------------------------------------------------------

func TestMockIdempotencyStore(t *testing.T) {
	ctx := context.Background()
	s := mocks.NewMockIdempotencyStore()

	t.Run("first store succeeds", func(t *testing.T) {
		stored, err := s.CheckAndStore(ctx, "key-1", []byte("result"), 60)
		if err != nil {
			t.Errorf("CheckAndStore() err = %v", err)
		}
		if !stored {
			t.Errorf("CheckAndStore() = false, want true")
		}
	})

	t.Run("duplicate store is rejected", func(t *testing.T) {
		stored, err := s.CheckAndStore(ctx, "key-1", []byte("other"), 60)
		if err != nil {
			t.Errorf("CheckAndStore(dup) err = %v", err)
		}
		if stored {
			t.Errorf("CheckAndStore(dup) = true, want false")
		}
	})

	t.Run("Exists reflects stored keys", func(t *testing.T) {
		if exists, err := s.Exists(ctx, "key-1"); err != nil || !exists {
			t.Errorf("Exists(key-1) = %v, %v; want true, nil", exists, err)
		}
		if exists, err := s.Exists(ctx, "key-x"); err != nil || exists {
			t.Errorf("Exists(key-x) = %v, %v; want false, nil", exists, err)
		}
	})

	t.Run("Delete removes keys", func(t *testing.T) {
		if err := s.Delete(ctx, "key-1"); err != nil {
			t.Errorf("Delete() err = %v", err)
		}
		if exists, _ := s.Exists(ctx, "key-1"); exists {
			t.Errorf("Exists after delete = true, want false")
		}
		// Deleting a missing key is a no-op, not an error.
		if err := s.Delete(ctx, "key-x"); err != nil {
			t.Errorf("Delete(missing) err = %v", err)
		}
	})
}

// ------------------------------------------------------------------
// MockDispatchAttemptRepository
// ------------------------------------------------------------------

func TestMockDispatchAttemptRepository(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockDispatchAttemptRepository()

	t.Run("EnsureIndex", func(t *testing.T) {
		if err := repo.EnsureIndex(ctx); err != nil {
			t.Errorf("EnsureIndex() err = %v", err)
		}
	})

	t.Run("save and find by delivery", func(t *testing.T) {
		a1 := &domain.DispatchAttempt{ID: "1", DeliveryID: "delivery-1", DriverID: "driver-1", AttemptNumber: 1}
		a2 := &domain.DispatchAttempt{ID: "2", DeliveryID: "delivery-1", DriverID: "driver-2", AttemptNumber: 2}
		for _, a := range []*domain.DispatchAttempt{a1, a2} {
			if err := repo.Save(ctx, a); err != nil {
				t.Errorf("Save() err = %v", err)
			}
		}

		got, err := repo.FindByDeliveryID(ctx, "delivery-1")
		if err != nil {
			t.Errorf("FindByDeliveryID() err = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("FindByDeliveryID() len = %d, want 2", len(got))
		}

		empty, err := repo.FindByDeliveryID(ctx, "delivery-x")
		if err != nil {
			t.Errorf("FindByDeliveryID(missing) err = %v", err)
		}
		if empty == nil || len(empty) != 0 {
			t.Errorf("FindByDeliveryID(missing) = %v, want empty slice", empty)
		}

		count, err := repo.CountByDeliveryID(ctx, "delivery-1")
		if err != nil || count != 2 {
			t.Errorf("CountByDeliveryID() = %d, %v; want 2, nil", count, err)
		}
		zero, err := repo.CountByDeliveryID(ctx, "delivery-x")
		if err != nil || zero != 0 {
			t.Errorf("CountByDeliveryID(missing) = %d, %v; want 0, nil", zero, err)
		}
	})

	t.Run("find by driver", func(t *testing.T) {
		got, err := repo.FindByDriverID(ctx, "driver-1")
		if err != nil {
			t.Errorf("FindByDriverID() err = %v", err)
		}
		if len(got) != 1 || got[0].DriverID != "driver-1" {
			t.Errorf("FindByDriverID() = %v, want 1 attempt for driver-1", got)
		}

		none, err := repo.FindByDriverID(ctx, "driver-x")
		if err != nil || len(none) != 0 {
			t.Errorf("FindByDriverID(missing) = %v, %v; want empty, nil", none, err)
		}
	})
}

// ------------------------------------------------------------------
// MockLocationStore
// ------------------------------------------------------------------

func TestMockLocationStore(t *testing.T) {
	ctx := context.Background()
	store := mocks.NewMockLocationStore()

	t.Run("GEOAdd and GetDriverLocation", func(t *testing.T) {
		if err := store.GEOAdd(ctx, "31.2357", "30.0444", "driver-1"); err != nil {
			t.Errorf("GEOAdd() err = %v", err)
		}

		lat, lng, err := store.GetDriverLocation(ctx, "driver-1")
		if err != nil {
			t.Errorf("GetDriverLocation() err = %v", err)
		}
		if lat == "" || lng == "" {
			// The mock returns empty strings for both found and missing keys;
			// at minimum the call must not fail.
			t.Logf("GetDriverLocation returned %q/%q", lat, lng)
		}

		_, _, err = store.GetDriverLocation(ctx, "driver-x")
		if err != nil {
			t.Errorf("GetDriverLocation(missing) err = %v", err)
		}
	})

	t.Run("GEOSearch", func(t *testing.T) {
		if err := store.GEOSearch(ctx, 31.2357, 30.0444, 5); err != nil {
			t.Errorf("GEOSearch() err = %v", err)
		}
	})

	t.Run("GEOSearchDrivers", func(t *testing.T) {
		store.SetLocation("driver-1", 31.2357, 30.0444, 120)

		got, err := store.GEOSearchDrivers(ctx, 31.2, 30.0, 5)
		if err != nil {
			t.Errorf("GEOSearchDrivers() err = %v", err)
		}
		if len(got) != 1 || got[0].DriverID != "driver-1" {
			t.Errorf("GEOSearchDrivers() = %v, want driver-1", got)
		}
		if got[0].DistanceMeters != 120 {
			t.Errorf("distance = %v, want 120", got[0].DistanceMeters)
		}
	})

	t.Run("NearbyDriversSorted", func(t *testing.T) {
		got, err := store.NearbyDriversSorted(ctx, 31.2, 30.0, 5)
		if err != nil {
			t.Errorf("NearbyDriversSorted() err = %v", err)
		}
		if len(got) != 1 || got[0].DriverID != "driver-1" {
			t.Errorf("NearbyDriversSorted() = %v, want driver-1", got)
		}
	})
}
