package workers

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type job struct {
	name     string
	interval time.Duration
	fn       func(ctx context.Context)
}

type Pool struct {
	mu     sync.Mutex
	jobs   []job
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewPool() *Pool {
	return &Pool{stopCh: make(chan struct{})}
}

func (p *Pool) Register(name string, interval time.Duration, fn func(ctx context.Context)) {
	if interval <= 0 || fn == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.jobs = append(p.jobs, job{name: name, interval: interval, fn: fn})
}

func (p *Pool) Start(ctx context.Context) {
	p.mu.Lock()
	jobs := append([]job(nil), p.jobs...)
	p.mu.Unlock()
	for _, j := range jobs {
		p.wg.Add(1)
		go p.run(ctx, j)
	}
}

func (p *Pool) run(ctx context.Context, j job) {
	defer p.wg.Done()
	slog.Info("worker started", "job", j.name, "interval", j.interval.String())
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker stopped", "job", j.name, "reason", "context cancelled")
			return
		case <-p.stopCh:
			slog.Info("worker stopped", "job", j.name, "reason", "pool stopped")
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("worker panic recovered", "job", j.name, "panic", r)
					}
				}()
				j.fn(ctx)
			}()
		}
	}
}

func (p *Pool) Stop() {
	select {
	case <-p.stopCh:
		return
	default:
		close(p.stopCh)
	}
	p.wg.Wait()
}
