package workers

import (
	"context"
	"log"
	"sync"
)

// WorkerPool manages a pool of background workers for operational tasks.
type WorkerPool struct {
	workers   []*worker
	stopChan  chan struct{}
	wg        sync.WaitGroup
	maxWorkers int
}

// worker represents a single background worker.
type worker struct {
	id        int
	stopChan  chan struct{}
	taskChan  chan func() error
}

// NewWorkerPool creates a new worker pool with the specified number of workers.
func NewWorkerPool(numWorkers int) *WorkerPool {
	pool := &WorkerPool{
		stopChan:   make(chan struct{}),
		maxWorkers: numWorkers,
	}

	for i := 0; i < numWorkers; i++ {
		w := &worker{
			id:     i,
			stopChan: pool.stopChan,
			taskChan: make(chan func() error, 100),
		}
		pool.workers = append(pool.workers, w)
	}

	return pool
}

// Start starts all workers in the pool.
func (p *WorkerPool) Start(ctx context.Context) {
	for _, w := range p.workers {
		p.wg.Add(1)
		go w.run()
	}
}

// Stop stops all workers in the pool.
func (p *WorkerPool) Stop() {
	close(p.stopChan)
	p.wg.Wait()
}

// Submit submits a task to the worker pool.
func (p *WorkerPool) Submit(task func() error) error {
	// Distribute task to first available worker
	select {
	case p.workers[0].taskChan <- task:
		return nil
	default:
		return ErrWorkerPoolFull
	}
}

// ErrWorkerPoolFull is returned when the worker pool task queue is full.
var ErrWorkerPoolFull = func() error { return errWorkerPoolFull{} }()

type errWorkerPoolFull struct{}

func (e errWorkerPoolFull) Error() string {
	return "worker pool task queue full"
}

// run is the main loop for each worker.
func (w *worker) run() {
	defer func() {

	}()

	for {
		select {
		case <-w.stopChan:
			return
		case task, ok := <-w.taskChan:
			if !ok {
				return
			}
			if err := task(); err != nil {
				log.Printf("worker %d: task failed: %v", w.id, err)
			}
		}
	}
}