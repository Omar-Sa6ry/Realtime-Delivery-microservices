package scheduler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/robfig/cron/v3"
)

type Job func(ctx context.Context)

type Scheduler struct {
	cron *cron.Cron
	ctx  context.Context
}

func NewScheduler(ctx context.Context) *Scheduler {
	c := cron.New(cron.WithSeconds(), cron.WithLogger(cron.PrintfLogger(
		&cronLogger{},
	)))
	return &Scheduler{cron: c, ctx: ctx}
}

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

func (s *Scheduler) Start() {
	s.cron.Start()
	slog.Info("Scheduler: started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	slog.Info("Scheduler: stopped")
}

type cronLogger struct{}

func (l *cronLogger) Printf(format string, v ...interface{}) {
	slog.Debug(fmt.Sprintf(format, v...), "component", "cron")
}
