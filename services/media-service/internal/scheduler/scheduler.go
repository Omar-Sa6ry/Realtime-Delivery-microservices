package scheduler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/robfig/cron/v3"
)

// Job is a function that the scheduler calls on a cron schedule.
type Job func(ctx context.Context)

// Scheduler wraps the cron library with graceful shutdown and logging.
type Scheduler struct {
	cron *cron.Cron
	ctx  context.Context
}

// NewScheduler creates a new Scheduler backed by robfig/cron.
func NewScheduler(ctx context.Context) *Scheduler {
	c := cron.New(cron.WithSeconds(), cron.WithLogger(cron.PrintfLogger(
		&cronLogger{},
	)))
	return &Scheduler{cron: c, ctx: ctx}
}

// Add registers a job with a cron expression.
// Panics if the expression is invalid — this is a programming error, not a runtime condition.
func (s *Scheduler) Add(name, expression string, job Job) {
	_, err := s.cron.AddFunc(expression, func() {
		slog.Info("Scheduler: running job", "name", name)
		job(s.ctx)
	})
	if err != nil {
		panic(fmt.Sprintf("invalid cron expression for job %q: %v", name, err))
	}
	slog.Info("Scheduler: job registered", "name", name, "expression", expression)
}

// Start begins the scheduler background loop.
func (s *Scheduler) Start() {
	s.cron.Start()
	slog.Info("Scheduler: started")
}

// Stop gracefully halts the scheduler, waiting for running jobs to finish.
func (s *Scheduler) Stop() {
	s.cron.Stop()
	slog.Info("Scheduler: stopped")
}

// cronLogger bridges the cron logger interface to slog.
type cronLogger struct{}

func (l *cronLogger) Printf(format string, v ...interface{}) {
	slog.Debug(fmt.Sprintf(format, v...), "component", "cron")
}
