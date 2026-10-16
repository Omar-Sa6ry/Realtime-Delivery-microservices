package workers

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(msg)
}

func TestNewWorkerPoolHasNoActiveWorkers(t *testing.T) {
	p := NewWorkerPool("idle", 4)
	if got := p.ActiveWorkers(); got != 0 {
		t.Fatalf("ActiveWorkers() = %d, want 0", got)
	}
}

func TestSubmitRespectsConcurrencyCap(t *testing.T) {
	p := NewWorkerPool("cap", 2)
	ctx := context.Background()

	const jobs = 6
	started := make(chan struct{}, jobs)
	release := make(chan struct{})
	errCh := make(chan error, jobs)

	var active int32
	var peak int32
	for i := 0; i < jobs; i++ {
		go func() {
			errCh <- p.Submit(ctx, func(ctx context.Context) error {
				n := atomic.AddInt32(&active, 1)
				for {
					cur := atomic.LoadInt32(&peak)
					if n <= cur || atomic.CompareAndSwapInt32(&peak, cur, n) {
						break
					}
				}
				started <- struct{}{}
				select {
				case <-release:
				case <-ctx.Done():
				}
				atomic.AddInt32(&active, -1)
				return nil
			})
		}()
	}

	<-started
	<-started

	if got := p.ActiveWorkers(); got != 2 {
		t.Fatalf("ActiveWorkers() = %d while 2 jobs running, want 2", got)
	}
	select {
	case <-started:
		t.Fatal("a third job started although the semaphore allows only 2")
	case <-time.After(75 * time.Millisecond):
	}

	close(release)
	for i := 0; i < jobs; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("Submit returned %v, want nil", err)
		}
	}
	p.Shutdown()

	if got := atomic.LoadInt32(&peak); got != 2 {
		t.Fatalf("peak concurrency = %d, want exactly 2", got)
	}
	if got := p.ActiveWorkers(); got != 0 {
		t.Fatalf("ActiveWorkers() after shutdown = %d, want 0", got)
	}
}

func TestSubmitReturnsCtxErrWhenSemaphoreBusy(t *testing.T) {
	p := NewWorkerPool("cancel", 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	release := make(chan struct{})
	if err := p.Submit(ctx, func(ctx context.Context) error {
		close(started)
		<-release
		return nil
	}); err != nil {
		t.Fatalf("first Submit: %v", err)
	}
	<-started

	cancel()
	err := p.Submit(ctx, func(ctx context.Context) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Submit on cancelled ctx = %v, want context.Canceled", err)
	}

	close(release)
	p.Shutdown()
}

func TestPanicInJobDoesNotKillPool(t *testing.T) {
	p := NewWorkerPool("panic", 1)
	ctx := context.Background()

	panicking := make(chan struct{})
	if err := p.Submit(ctx, func(ctx context.Context) error {
		close(panicking)
		panic("boom")
	}); err != nil {
		t.Fatalf("Submit panicking job: %v", err)
	}
	<-panicking

	waitUntil(t, 2*time.Second, func() bool { return p.ActiveWorkers() == 0 },
		"semaphore slot was not released after panic")

	ran := make(chan struct{})
	if err := p.Submit(ctx, func(ctx context.Context) error {
		close(ran)
		return nil
	}); err != nil {
		t.Fatalf("Submit after panic: %v", err)
	}
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("subsequent job did not run after a previous job panicked")
	}
	p.Shutdown()
}

func TestJobErrorDoesNotFailSubmit(t *testing.T) {
	p := NewWorkerPool("err", 2)
	ctx := context.Background()

	done := make(chan struct{})
	if err := p.Submit(ctx, func(ctx context.Context) error {
		defer close(done)
		return errors.New("job failed")
	}); err != nil {
		t.Fatalf("Submit = %v, want nil even when the job errors", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("job did not run")
	}
	p.Shutdown()
}

func TestShutdownWaitsForInflightJobs(t *testing.T) {
	p := NewWorkerPool("graceful", 1)
	ctx := context.Background()

	started := make(chan struct{})
	release := make(chan struct{})
	if err := p.Submit(ctx, func(ctx context.Context) error {
		close(started)
		<-release
		return nil
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	<-started

	done := make(chan struct{})
	go func() {
		p.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Shutdown returned before the in-flight job finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not return after the job finished")
	}
	if got := p.ActiveWorkers(); got != 0 {
		t.Fatalf("ActiveWorkers() after shutdown = %d, want 0", got)
	}
}

func TestSubmitAfterShutdownStillRunsJob(t *testing.T) {
	p := NewWorkerPool("post-shutdown", 1)
	p.Shutdown()

	ran := make(chan struct{})
	if err := p.Submit(context.Background(), func(ctx context.Context) error {
		close(ran)
		return nil
	}); err != nil {
		t.Fatalf("Submit after Shutdown = %v, want nil", err)
	}
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("job submitted after Shutdown did not run")
	}
	p.Shutdown()
}
