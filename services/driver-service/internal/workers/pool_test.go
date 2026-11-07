package workers_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/workers"
)

func TestErrWorkerPoolFull(t *testing.T) {
	var e error = workers.ErrWorkerPoolFull
	if e == nil {
		t.Fatal("ErrWorkerPoolFull is nil")
	}
	if got, want := workers.ErrWorkerPoolFull.Error(), "worker pool task queue full"; got != want {
		t.Errorf("ErrWorkerPoolFull.Error() = %q, want %q", got, want)
	}
}

func TestWorkerPool_StartSubmitStop(t *testing.T) {
	pool := workers.NewWorkerPool(3)
	pool.Start(context.Background())

	var executed int32
	for i := 0; i < 10; i++ {
		pool.Submit(func() {
			atomic.AddInt32(&executed, 1)
		})
	}

	deadline := time.Now().Add(3 * time.Second)
	for atomic.LoadInt32(&executed) != 10 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&executed); got != 10 {
		t.Errorf("executed tasks = %d, want 10", got)
	}

	pool.Stop()
}

func TestWorkerPool_StopReturns(t *testing.T) {
	pool := workers.NewWorkerPool(2)
	pool.Start(context.Background())

	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() did not return")
	}
}

func TestWorkerPool_ZeroWorkers(t *testing.T) {
	pool := workers.NewWorkerPool(0)
	pool.Start(context.Background())
	pool.Stop()
}
