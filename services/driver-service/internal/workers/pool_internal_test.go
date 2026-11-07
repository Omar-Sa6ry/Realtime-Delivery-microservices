package workers

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// waitUntil polls cond until it becomes true or the timeout expires.
// Shared by the in-package worker tests.
func waitUntil(t *testing.T, cond func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal(msg)
}

func TestWorkerRun(t *testing.T) {
	t.Run("runs successful and failing tasks then stops on stopChan", func(t *testing.T) {
		w := &worker{id: 1, stopChan: make(chan struct{}), taskChan: make(chan func() error, 10)}
		done := make(chan struct{})
		go func() {
			w.run()
			close(done)
		}()

		var calls int32
		w.taskChan <- func() error {
			atomic.AddInt32(&calls, 1)
			return nil
		}
		w.taskChan <- func() error {
			atomic.AddInt32(&calls, 1)
			return errors.New("task failed")
		}

		waitUntil(t, func() bool { return atomic.LoadInt32(&calls) == 2 }, 3*time.Second, "tasks were not executed")

		close(w.stopChan)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("worker.run did not return after stopChan was closed")
		}
	})

	t.Run("returns when taskChan is closed", func(t *testing.T) {
		w := &worker{id: 2, stopChan: make(chan struct{}), taskChan: make(chan func() error)}
		close(w.taskChan)

		done := make(chan struct{})
		go func() {
			w.run()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("worker.run did not return after taskChan was closed")
		}
	})
}
