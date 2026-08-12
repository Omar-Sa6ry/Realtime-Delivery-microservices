package workers

import (
	"context"
	"log/slog"
	"sync"
)

// Job is the function signature for a single unit of work submitted to the pool.
type Job func(ctx context.Context) error

// WorkerPool is a bounded pool of goroutines for background processing.
// It prevents unlimited goroutine creation for CPU-intensive work (video transcoding, scanning).
// Submit blocks when the pool is saturated — providing back-pressure to callers.
type WorkerPool struct {
	name      string
	semaphore chan struct{}
	wg        sync.WaitGroup
}

// NewWorkerPool creates a pool with the given concurrency limit.
func NewWorkerPool(name string, concurrency int) *WorkerPool {
	return &WorkerPool{
		name:      name,
		semaphore: make(chan struct{}, concurrency),
	}
}

// Submit acquires a worker slot and runs the job in a goroutine.
// If all slots are occupied, Submit blocks until one is available or ctx is cancelled.
func (p *WorkerPool) Submit(ctx context.Context, job Job) error {
	select {
	case p.semaphore <- struct{}{}: // acquire slot
	case <-ctx.Done():
		return ctx.Err()
	}

	p.wg.Add(1)
	go func() {
		defer func() {
			<-p.semaphore // release slot
			p.wg.Done()
			if r := recover(); r != nil {
				slog.Error("Worker panic recovered", "pool", p.name, "panic", r)
			}
		}()
		if err := job(ctx); err != nil {
			slog.Error("Worker job failed", "pool", p.name, "error", err)
		}
	}()
	return nil
}

// Shutdown waits for all in-flight jobs to complete before returning.
// Call this during graceful shutdown.
func (p *WorkerPool) Shutdown() {
	p.wg.Wait()
	slog.Info("Worker pool shut down", "pool", p.name)
}

// ActiveWorkers returns the number of currently executing jobs.
func (p *WorkerPool) ActiveWorkers() int {
	return len(p.semaphore)
}
