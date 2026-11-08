package workers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type Job func(ctx context.Context) error

type WorkerPool struct {
	name      string
	semaphore chan struct{}
	wg        sync.WaitGroup
}

func NewWorkerPool(name string, concurrency int) *WorkerPool {
	return &WorkerPool{
		name:      name,
		semaphore: make(chan struct{}, concurrency),
	}
}

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

// SubmitSync runs job under the pool's concurrency limit and returns its
// result. Unlike Submit, job errors and panics surface to the caller so
// Kafka consumers can retry and route poison messages to the DLQ.
func (p *WorkerPool) SubmitSync(ctx context.Context, job Job) (err error) {
	select {
	case p.semaphore <- struct{}{}: // acquire slot
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-p.semaphore }() // release slot
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Worker panic recovered", "pool", p.name, "panic", r)
			err = fmt.Errorf("worker panic: %v", r)
		}
	}()
	return job(ctx)
}

func (p *WorkerPool) Shutdown() {
	p.wg.Wait()
	slog.Info("Worker pool shut down", "pool", p.name)
}

func (p *WorkerPool) ActiveWorkers() int {
	return len(p.semaphore)
}
