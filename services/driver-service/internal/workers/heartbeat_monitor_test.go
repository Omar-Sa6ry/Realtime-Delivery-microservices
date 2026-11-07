package workers

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/mocks"
)

// countingDriverRepo counts FindByStatus calls so tests can observe the
// heartbeat ticker firing.
type countingDriverRepo struct {
	*mocks.MockDriverRepository
	calls int32
}

func (r *countingDriverRepo) FindByStatus(ctx context.Context, status string) ([]*domain.Driver, error) {
	atomic.AddInt32(&r.calls, 1)
	return r.MockDriverRepository.FindByStatus(ctx, status)
}

func TestHeartbeatMonitor_checkStaleDrivers(t *testing.T) {
	t.Run("inspects available drivers", func(t *testing.T) {
		repo := mocks.NewMockDriverRepository()
		repo.SetDriver(&domain.Driver{ID: "d1", Status: domain.DriverStatusAvailable, IsActive: true})
		repo.SetDriver(&domain.Driver{ID: "d2", Status: domain.DriverStatusBusy, IsActive: true})

		h := NewHeartbeatMonitor(repo)
		h.checkStaleDrivers(context.Background()) // must not panic
	})

	t.Run("repo error is tolerated", func(t *testing.T) {
		repo := mocks.NewMockDriverRepository()
		repo.SetError(errors.New("mongo down"))

		h := NewHeartbeatMonitor(repo)
		h.checkStaleDrivers(context.Background()) // must not panic
	})
}

func TestHeartbeatMonitor_Run(t *testing.T) {
	base := mocks.NewMockDriverRepository()
	base.SetDriver(&domain.Driver{ID: "d1", Status: domain.DriverStatusAvailable, IsActive: true})
	repo := &countingDriverRepo{MockDriverRepository: base}

	h := NewHeartbeatMonitor(repo)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.Run(ctx, 2*time.Millisecond)
		close(done)
	}()

	waitUntil(t, func() bool { return atomic.LoadInt32(&repo.calls) >= 1 }, 3*time.Second,
		"heartbeat monitor never ran a check")

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}
