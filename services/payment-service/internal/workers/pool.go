package workers

import (
	"context"
	"log/slog"
	"time"
)

type WorkerPool struct {
	ctx     context.Context
	cancel  context.CancelFunc
	workers []namedWorker
	done    chan struct{}
}

type namedWorker struct {
	name     string
	interval time.Duration
	fn       func(ctx context.Context) error
}

func NewWorkerPool() *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

func (p *WorkerPool) Register(name string, interval time.Duration, fn func(ctx context.Context) error) {
	p.workers = append(p.workers, namedWorker{name: name, interval: interval, fn: fn})
}

func (p *WorkerPool) Start() {
	for _, w := range p.workers {
		w := w // capture loop variable
		go func() {
			slog.Info("worker started", "worker", w.name, "interval", w.interval)
			// Run once immediately.
			if err := w.fn(p.ctx); err != nil {
				slog.Error("worker error", "worker", w.name, "error", err)
			}

			ticker := time.NewTicker(w.interval)
			defer ticker.Stop()

			for {
				select {
				case <-p.ctx.Done():
					slog.Info("worker stopping", "worker", w.name)
					return
				case <-ticker.C:
					if err := w.fn(p.ctx); err != nil {
						slog.Error("worker error", "worker", w.name, "error", err)
					}
				}
			}
		}()
	}
}

func (p *WorkerPool) Stop() {
	slog.Info("worker_pool: stopping all workers")
	p.cancel()
}
