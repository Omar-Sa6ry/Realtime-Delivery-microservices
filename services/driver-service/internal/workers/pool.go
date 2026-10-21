package workers

import (
	"context"
	"log"
	"sync"
)

type WorkerPool struct {
	workers   []*worker
	stopChan  chan struct{}
	wg        sync.WaitGroup
	maxWorkers int
}

type worker struct {
	id        int
	stopChan  chan struct{}
	taskChan  chan func() error
}

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

func (p *WorkerPool) Start(ctx context.Context) {
	for _, w := range p.workers {
		p.wg.Add(1)
		go w.run()
	}
}

func (p *WorkerPool) Stop() {
	close(p.stopChan)
	p.wg.Wait()
}

func (p *WorkerPool) Submit(task func()) {
	go task()
}


var ErrWorkerPoolFull = func() error { return errWorkerPoolFull{} }()

type errWorkerPoolFull struct{}

func (e errWorkerPoolFull) Error() string {
	return "worker pool task queue full"
}

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