package workers

import (
	"context"
	"sync"
	"time"
)

// WorkerPool manages a pool of background workers.
type WorkerPool struct {
	workers     []Worker
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.Mutex
	running     bool
	interval    time.Duration
}

// Worker defines the interface for a background worker.
type Worker interface {
	Run(ctx context.Context) error
	Name() string
	Interval() time.Duration
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(interval time.Duration) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		ctx:      ctx,
		cancel:   cancel,
		interval: interval,
	}
}

// AddWorker adds a worker to the pool.
func (p *WorkerPool) AddWorker(worker Worker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.workers = append(p.workers, worker)
}

// Start starts all workers in the pool.
func (p *WorkerPool) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	for _, worker := range p.workers {
		p.wg.Add(1)
		go func(w Worker) {
			defer p.wg.Done()
			p.runWorker(p.ctx, w)
		}(worker)
	}
}

// Stop stops all workers gracefully.
func (p *WorkerPool) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()

	p.cancel()
	p.wg.Wait()
}

// runWorker runs a single worker with its interval.
func (p *WorkerPool) runWorker(ctx context.Context, worker Worker) {
	ticker := time.NewTicker(worker.Interval())
	defer ticker.Stop()

	// Run immediately on start
	if err := worker.Run(ctx); err != nil {
		// Log error but continue
		// logger.Error("worker error", "worker", worker.Name(), "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := worker.Run(ctx); err != nil {
				// logger.Error("worker error", "worker", worker.Name(), "error", err)
			}
		}
	}
}