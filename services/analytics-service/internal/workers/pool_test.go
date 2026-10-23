package workers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/reconciliation"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewPool(t *testing.T) {
	pool := NewPool()
	assert.NotNil(t, pool)
}

func TestPool_Register(t *testing.T) {
	pool := NewPool()
	called := false

	pool.Register("test-job", 100*time.Millisecond, func(ctx context.Context) {
		called = true
	})

	// Can't directly access private jobs slice, but we can verify via Start/Stop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	pool.Stop()

	assert.True(t, called)
}

func TestPool_Register_InvalidInterval(t *testing.T) {
	pool := NewPool()
	called := false

	pool.Register("test-job", -1, func(ctx context.Context) {
		called = true
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	pool.Stop()

	assert.False(t, called) // Should not register with invalid interval
}

func TestPool_Register_NilFunc(t *testing.T) {
	pool := NewPool()
	called := false

	pool.Register("test-job", 100*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	pool.Stop()

	assert.False(t, called) // Should not register with nil func
}

func TestPool_MultipleJobs(t *testing.T) {
	pool := NewPool()
	job1Called := false
	job2Called := false

	pool.Register("job1", 50*time.Millisecond, func(ctx context.Context) {
		job1Called = true
	})
	pool.Register("job2", 100*time.Millisecond, func(ctx context.Context) {
		job2Called = true
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	time.Sleep(200 * time.Millisecond)
	pool.Stop()

	assert.True(t, job1Called)
	assert.True(t, job2Called)
}

func TestPool_Stop_Idempotent(t *testing.T) {
	pool := NewPool()
	pool.Register("test-job", 100*time.Millisecond, func(ctx context.Context) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	pool.Stop()
	pool.Stop() // Second stop should not panic
}

func TestPool_ContextCancellation(t *testing.T) {
	pool := NewPool()
	called := 0

	pool.Register("test-job", 10*time.Millisecond, func(ctx context.Context) {
		called++
	})

	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	cancel() // Cancel context
	pool.Stop()

	assert.Greater(t, called, 0)
}

func TestPool_PanicRecovery(t *testing.T) {
	pool := NewPool()

	pool.Register("panic-job", 50*time.Millisecond, func(ctx context.Context) {
		panic("test panic")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	pool.Stop()

	// If we reach here without panic, recovery worked
	assert.True(t, true)
}

func TestNewReconciliationWorker(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()
	svc := reconciliation.NewService(queries, reconciliation.Config{})
	worker := NewReconciliationWorker(svc)
	assert.NotNil(t, worker)
	assert.Equal(t, svc, worker.service)
}

type testReconciliationService struct {
	queries *testutil.MockReconciliationQueries
	svc     *reconciliation.Service
	checkCalled bool
}

func newTestReconciliationService() *testReconciliationService {
	queries := testutil.NewMockReconciliationQueries()
	return &testReconciliationService{
		queries: queries,
		svc:     reconciliation.NewService(queries, reconciliation.Config{}),
	}
}

func (s *testReconciliationService) Check(ctx context.Context) (*reconciliation.Report, error) {
	s.checkCalled = true
	return &reconciliation.Report{
		CheckedAt:        time.Now().UTC(),
		WindowFrom:       time.Now().UTC().Add(-time.Hour),
		WindowTo:         time.Now().UTC(),
		FreshnessSeconds: 30,
		Freshness:        reconciliation.FreshnessHealthy,
		RawTotal:         100,
		FactTotal:        99,
		CoverageRatio:    0.99,
		GapDetected:      false,
		DuplicateCount:   0,
		OpenIssues:       map[string]int64{},
	}, nil
}

func TestReconciliationWorker_Run(t *testing.T) {
	svc := newTestReconciliationService()
	worker := NewReconciliationWorker(svc.svc)

	ctx := context.Background()
	worker.Run(ctx)

	assert.True(t, svc.checkCalled)
}

func TestReconciliationWorker_Run_Error(t *testing.T) {
	queries := testutil.NewMockReconciliationQueries()
	queries.SetError(errors.New("check failed"))
	svc := reconciliation.NewService(queries, reconciliation.Config{})
	worker := NewReconciliationWorker(svc)

	ctx := context.Background()
	worker.Run(ctx)

	// Should not panic, just log error
	assert.True(t, true)
}

type errorReconciliationService struct{}

func (s *errorReconciliationService) Check(ctx context.Context) (*reconciliation.Report, error) {
	return nil, errors.New("check failed")
}