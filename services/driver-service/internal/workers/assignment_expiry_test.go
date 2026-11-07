package workers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

// fakeDispatch implements DispatchServiceForWorkers.
type fakeDispatch struct {
	mu       sync.Mutex
	released []string
	err      error
}

func (f *fakeDispatch) ReleaseDriver(ctx context.Context, driverID, deliveryID string) error {
	return f.err
}

func (f *fakeDispatch) ReleaseDriverByAssignment(ctx context.Context, assignmentID string) error {
	f.mu.Lock()
	f.released = append(f.released, assignmentID)
	f.mu.Unlock()
	return f.err
}

func (f *fakeDispatch) releaseCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.released)
}

// assignmentRepoStub injects ExpireOffers / UpdateStatus failures that the
// shared mock cannot produce.
type assignmentRepoStub struct {
	*mocks.MockAssignmentRepository
	expireErr  error
	expiredIDs []string
	updateErr  error
}

func (r *assignmentRepoStub) ExpireOffers(ctx context.Context, olderThan time.Duration) ([]string, error) {
	if r.expireErr != nil {
		return nil, r.expireErr
	}
	if r.expiredIDs != nil {
		return r.expiredIDs, nil
	}
	return r.MockAssignmentRepository.ExpireOffers(ctx, olderThan)
}

func (r *assignmentRepoStub) UpdateStatus(ctx context.Context, assignmentID string, status string) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.MockAssignmentRepository.UpdateStatus(ctx, assignmentID, status)
}

func staleOffer(driverID string) *domain.Assignment {
	now := time.Now().Add(-time.Hour)
	return &domain.Assignment{
		ID:         "a1",
		DriverID:   driverID,
		DeliveryID: "delivery-1",
		Status:     domain.AssignmentStatusOffered,
		OfferedAt:  now,
		ExpiresAt:  now.Add(30 * time.Second),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func TestAssignmentExpiryWorker_checkExpiredAssignments(t *testing.T) {
	t.Run("nothing to expire", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		dispatch := &fakeDispatch{}
		w := NewAssignmentExpiryWorker(repo, dispatch)

		w.checkExpiredAssignments(context.Background())

		if dispatch.releaseCount() != 0 {
			t.Errorf("dispatch called %d times, want 0", dispatch.releaseCount())
		}
	})

	t.Run("expires stale offer without dispatch service", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		repo.SetAssignment(staleOffer("driver-1"))
		w := NewAssignmentExpiryWorker(repo, nil)

		w.checkExpiredAssignments(context.Background())

		assignment, err := repo.FindByID(context.Background(), "a1")
		if err != nil {
			t.Fatalf("FindByID() unexpected error: %v", err)
		}
		if assignment == nil || assignment.Status != domain.AssignmentStatusExpired {
			t.Errorf("assignment = %+v, want EXPIRED", assignment)
		}
	})

	t.Run("expire failure is tolerated", func(t *testing.T) {
		repo := &assignmentRepoStub{
			MockAssignmentRepository: mocks.NewMockAssignmentRepository(),
			expireErr:                errors.New("mongo down"),
		}
		w := NewAssignmentExpiryWorker(repo, nil)

		w.checkExpiredAssignments(context.Background()) // must not panic
	})

	t.Run("dispatch service releases expired assignment", func(t *testing.T) {
		repo := &assignmentRepoStub{
			MockAssignmentRepository: mocks.NewMockAssignmentRepository(),
			expiredIDs:               []string{"a1", "a2"},
		}
		dispatch := &fakeDispatch{}
		w := NewAssignmentExpiryWorker(repo, dispatch)

		w.checkExpiredAssignments(context.Background())

		if got := dispatch.releaseCount(); got != 2 {
			t.Errorf("dispatch called %d times, want 2", got)
		}
	})

	t.Run("dispatch release failure is tolerated", func(t *testing.T) {
		repo := &assignmentRepoStub{
			MockAssignmentRepository: mocks.NewMockAssignmentRepository(),
			expiredIDs:               []string{"a1"},
		}
		dispatch := &fakeDispatch{err: errors.New("release failed")}
		w := NewAssignmentExpiryWorker(repo, dispatch)

		w.checkExpiredAssignments(context.Background())

		if got := dispatch.releaseCount(); got != 1 {
			t.Errorf("dispatch called %d times, want 1", got)
		}
	})

	t.Run("update status failure is tolerated", func(t *testing.T) {
		repo := &assignmentRepoStub{
			MockAssignmentRepository: mocks.NewMockAssignmentRepository(),
			expiredIDs:               []string{"a1"},
			updateErr:                errors.New("mongo down"),
		}
		w := NewAssignmentExpiryWorker(repo, nil)

		w.checkExpiredAssignments(context.Background()) // must not panic
	})
}

func TestAssignmentExpiryWorker_Run(t *testing.T) {
	t.Run("ticker drives dispatch releases", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		repo.SetAssignment(staleOffer("driver-1"))
		dispatch := &fakeDispatch{}
		w := NewAssignmentExpiryWorker(repo, dispatch)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			w.Run(ctx, 2*time.Millisecond)
			close(done)
		}()

		waitUntil(t, func() bool { return dispatch.releaseCount() >= 1 }, 3*time.Second,
			"assignment expiry worker never released the stale offer")

		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("Run() did not return after context cancellation")
		}
	})

	t.Run("ticker expires offers without dispatch service", func(t *testing.T) {
		repo := mocks.NewMockAssignmentRepository()
		repo.SetAssignment(staleOffer("driver-1"))
		w := NewAssignmentExpiryWorker(repo, nil)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			w.Run(ctx, 2*time.Millisecond)
			close(done)
		}()

		waitUntil(t, func() bool {
			assignment, err := repo.FindByID(context.Background(), "a1")
			return err == nil && assignment != nil && assignment.Status == domain.AssignmentStatusExpired
		}, 3*time.Second, "assignment expiry worker never expired the stale offer")

		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("Run() did not return after context cancellation")
		}
	})
}
