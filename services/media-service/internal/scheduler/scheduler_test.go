package scheduler

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerRunsJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s := NewScheduler(ctx)

	var runs atomic.Int32
	s.Add("tick", "@every 1s", func(ctx context.Context) {
		runs.Add(1)
	})

	s.Start()
	t.Cleanup(s.Stop)

	deadline := time.Now().Add(3 * time.Second)
	for runs.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if runs.Load() == 0 {
		t.Fatal("scheduled job did not run within 3s")
	}
}

func TestSchedulerAddInvalidExpressionPanics(t *testing.T) {
	s := NewScheduler(context.Background())

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Add with invalid expression must panic")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "invalid cron expression") {
			t.Fatalf("panic = %v, want invalid cron expression message", r)
		}
	}()

	s.Add("bad", "not-a-cron", func(ctx context.Context) {})
}

func TestSchedulerStartStopWithoutJobs(t *testing.T) {
	s := NewScheduler(context.Background())
	s.Start()
	s.Stop()
}

func TestCronLoggerForwardsToSlog(t *testing.T) {
	prev := slog.Default()
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	(&cronLogger{}).Printf("tick %d", 1)

	out := buf.String()
	if !strings.Contains(out, "tick 1") {
		t.Fatalf("slog output = %q, want formatted message", out)
	}
	if !strings.Contains(out, "component=cron") {
		t.Fatalf("slog output = %q, want component attribute", out)
	}
}
