package reindex

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
)

func TestStartReindex_ReturnsJobImmediately(t *testing.T) {
	repo := testutil.NewFakeRepository()
	repo.ReindexDone = make(chan struct{})
	svc := NewService(repo)

	job, err := svc.StartReindex(context.Background(), "deliveries")
	if err != nil {
		t.Fatalf("StartReindex: %v", err)
	}
	if job == nil {
		t.Fatal("nil job")
	}
	if !strings.HasPrefix(job.JobID, "reindex-deliveries-") {
		t.Errorf("jobID = %q", job.JobID)
	}
	if job.Index != "deliveries" || job.Status != "IN_PROGRESS" {
		t.Errorf("job = %+v", job)
	}
	if job.StartedAt.IsZero() {
		t.Error("StartedAt zero")
	}
	if job.CompletedAt != nil || job.Error != "" {
		t.Errorf("unexpected completed fields: %+v", job)
	}

	// Wait for background goroutine to hit the repo.
	select {
	case <-repo.ReindexDone:
	case <-time.After(2 * time.Second):
		t.Fatal("background reindex was not invoked")
	}

	calls := append([][2]string(nil), repo.ReindexCalls...)
	if len(calls) != 1 || calls[0][0] != "deliveries-v1" || calls[0][1] != "deliveries-v1" {
		t.Errorf("reindex calls = %v", calls)
	}
}

func TestStartReindex_EmptyIndex(t *testing.T) {
	repo := testutil.NewFakeRepository()
	svc := NewService(repo)
	for _, name := range []string{"", "   "} {
		if _, err := svc.StartReindex(context.Background(), name); err == nil || err.Error() != "index name is required" {
			t.Errorf("StartReindex(%q) err = %v", name, err)
		}
	}
}

func TestStartReindex_BackgroundErrorIgnored(t *testing.T) {
	repo := testutil.NewFakeRepository()
	repo.ReindexErr = errors.New("os down")
	repo.ReindexDone = make(chan struct{})
	svc := NewService(repo)

	job, err := svc.StartReindex(context.Background(), "drivers")
	if err != nil {
		t.Fatalf("StartReindex should not fail on background error: %v", err)
	}
	if job.Status != "IN_PROGRESS" {
		t.Errorf("status = %q", job.Status)
	}

	select {
	case <-repo.ReindexDone:
	case <-time.After(2 * time.Second):
		t.Fatal("background reindex was not invoked")
	}
}
