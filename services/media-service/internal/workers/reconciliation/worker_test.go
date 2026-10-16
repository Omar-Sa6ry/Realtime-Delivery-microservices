package reconciliation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

type ctxAwareCache struct {
	*testutil.MockCache
}

func (c *ctxAwareCache) AcquireLock(ctx context.Context, resource string, ttl time.Duration) (ports.LockToken, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	return c.MockCache.AcquireLock(ctx, resource, ttl)
}

type fixture struct {
	worker     *Worker
	uploadRepo *testutil.MockUploadRepository
	mediaRepo  *testutil.MockMediaRepository
	outboxRepo *testutil.MockOutboxRepository
	storage    *testutil.MemStorage
	cache      *testutil.MockCache
	pub        *testutil.MockPublisher
}

func newFixture() *fixture {
	f := &fixture{
		uploadRepo: testutil.NewMockUploadRepository(),
		mediaRepo:  testutil.NewMockMediaRepository(),
		outboxRepo: testutil.NewMockOutboxRepository(),
		storage:    testutil.NewMemStorage(),
		cache:      testutil.NewMockCache(),
		pub:        testutil.NewMockPublisher(),
	}
	f.worker = NewWorker(f.uploadRepo, f.mediaRepo, f.outboxRepo, f.storage, f.cache, f.pub,
		time.Hour, time.Hour)
	return f
}

func outboxEvent(id, topic string) *domain.OutboxEvent {
	return &domain.OutboxEvent{
		EventID:     id,
		AggregateID: "media-1",
		EventType:   topic,
		Payload:     []byte(`{"mediaId":"media-1"}`),
		Status:      domain.OutboxStatusPending,
		TraceID:     "trace-1",
	}
}

func TestRunSkipsWhenLockNotAcquired(t *testing.T) {
	f := newFixture()
	f.cache.LockDenied = true
	f.uploadRepo.ExpiredSessions = []*domain.UploadSession{testutil.NewUploadSession("up-1", "m-1", "u-1")}
	f.outboxRepo.Pending = []*domain.OutboxEvent{outboxEvent("evt-1", "media.ready")}

	f.worker.Run(context.Background())

	if len(f.uploadRepo.ListExpiredBefore) != 0 {
		t.Fatalf("ListExpired called %d times, want 0 when the lock is denied", len(f.uploadRepo.ListExpiredBefore))
	}
	if len(f.pub.Messages) != 0 {
		t.Fatalf("published %d messages, want 0 when the lock is denied", len(f.pub.Messages))
	}
	if len(f.cache.LockReleases) != 0 {
		t.Fatalf("LockReleases = %v, want none when the lock was never acquired", f.cache.LockReleases)
	}
}

func TestRunReturnsOnLockError(t *testing.T) {
	f := newFixture()
	f.cache.LockErr = errors.New("redis down")
	f.uploadRepo.ExpiredSessions = []*domain.UploadSession{testutil.NewUploadSession("up-1", "m-1", "u-1")}

	f.worker.Run(context.Background())

	if len(f.uploadRepo.ListExpiredBefore) != 0 {
		t.Fatalf("ListExpired called %d times, want 0 on lock error", len(f.uploadRepo.ListExpiredBefore))
	}
	if len(f.cache.LockReleases) != 0 {
		t.Fatalf("LockReleases = %v, want none on lock error", f.cache.LockReleases)
	}
}

func TestRunReturnsWhenContextCancelled(t *testing.T) {
	f := newFixture()
	f.worker = NewWorker(f.uploadRepo, f.mediaRepo, f.outboxRepo, f.storage,
		&ctxAwareCache{MockCache: f.cache}, f.pub, time.Hour, time.Hour)
	f.uploadRepo.ExpiredSessions = []*domain.UploadSession{testutil.NewUploadSession("up-1", "m-1", "u-1")}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f.worker.Run(ctx)

	if len(f.uploadRepo.ListExpiredBefore) != 0 {
		t.Fatalf("ListExpired called %d times, want 0 on cancelled context", len(f.uploadRepo.ListExpiredBefore))
	}
	if len(f.cache.LockReleases) != 0 {
		t.Fatalf("LockReleases = %v, want none on cancelled context", f.cache.LockReleases)
	}
}

func TestRunAbortsExpiredUploadsAndRepublishesOutbox(t *testing.T) {
	f := newFixture()
	session := testutil.NewUploadSession("up-1", "m-1", "u-1")
	session.ExpiresAt = time.Now().Add(-time.Minute)
	f.uploadRepo.ExpiredSessions = []*domain.UploadSession{session}
	f.mediaRepo.Media = testutil.NewMedia("m-1", "u-1")
	f.outboxRepo.Pending = []*domain.OutboxEvent{outboxEvent("evt-1", "media.ready")}

	f.worker.Run(context.Background())

	if len(f.uploadRepo.StatusUpdates) != 1 ||
		f.uploadRepo.StatusUpdates[0] != (testutil.UploadStatusCall{UploadID: "up-1", Status: domain.UploadStatusExpired}) {
		t.Fatalf("upload StatusUpdates = %+v, want single EXPIRED update", f.uploadRepo.StatusUpdates)
	}
	if len(f.mediaRepo.UpdateStatuses) != 1 ||
		f.mediaRepo.UpdateStatuses[0] != (testutil.MediaStatusCall{
			MediaID: "m-1", Current: domain.MediaStatusUploading, Next: domain.MediaStatusAborted,
		}) {
		t.Fatalf("media UpdateStatuses = %+v, want single UPLOADING->ABORTED update", f.mediaRepo.UpdateStatuses)
	}
	if len(f.cache.LockReleases) != 1 || f.cache.LockReleases[0] != "reconciliation:run" {
		t.Fatalf("LockReleases = %v, want the reconciliation lock released", f.cache.LockReleases)
	}
	if len(f.outboxRepo.Published) != 1 || f.outboxRepo.Published[0] != "evt-1" {
		t.Fatalf("outbox Published = %v, want [evt-1]", f.outboxRepo.Published)
	}
	msgs := f.pub.ForTopic("media.ready")
	if len(msgs) != 1 {
		t.Fatalf("republished %d messages, want 1", len(msgs))
	}
	if msgs[0].Key != "media-1" || msgs[0].TraceID != "trace-1" {
		t.Fatalf("unexpected republished message: %+v", msgs[0])
	}
}

func TestReconcileExpiredUploadsSurvivesStorageAndRepoErrors(t *testing.T) {
	f := newFixture()
	session := testutil.NewUploadSession("up-1", "m-1", "u-1")
	f.uploadRepo.ExpiredSessions = []*domain.UploadSession{session}
	f.storage.SetError("AbortMultipartUpload", errors.New("s3 unavailable"))
	f.uploadRepo.UpdateStatusErr = errors.New("dynamo upload write failed")
	f.mediaRepo.UpdateStatusErr = errors.New("dynamo media write failed")

	f.worker.reconcileExpiredUploads(context.Background())

	if len(f.storage.Calls()) == 0 {
		t.Fatal("AbortMultipartUpload was never attempted")
	}
	aborted := false
	for _, c := range f.storage.Calls() {
		if strings.HasPrefix(c, "AbortMultipartUpload:") {
			aborted = true
		}
	}
	if !aborted {
		t.Fatalf("storage calls = %v, want an AbortMultipartUpload attempt", f.storage.Calls())
	}
	if len(f.uploadRepo.StatusUpdates) != 1 {
		t.Fatalf("upload StatusUpdates = %+v, want the EXPIRED attempt despite the error", f.uploadRepo.StatusUpdates)
	}
	if len(f.mediaRepo.UpdateStatuses) != 1 {
		t.Fatalf("media UpdateStatuses = %+v, want the ABORTED attempt despite the error", f.mediaRepo.UpdateStatuses)
	}
}

func TestReconcileExpiredUploadsListError(t *testing.T) {
	f := newFixture()
	f.uploadRepo.ListExpiredErr = errors.New("query failed")

	f.worker.reconcileExpiredUploads(context.Background())

	if len(f.uploadRepo.StatusUpdates) != 0 {
		t.Fatalf("upload StatusUpdates = %+v, want none on list error", f.uploadRepo.StatusUpdates)
	}
	if len(f.storage.Calls()) != 0 {
		t.Fatalf("storage calls = %v, want none on list error", f.storage.Calls())
	}
}

func TestReconcileExpiredUploadsNoSessions(t *testing.T) {
	f := newFixture()

	f.worker.reconcileExpiredUploads(context.Background())

	if len(f.uploadRepo.ListExpiredBefore) != 1 {
		t.Fatalf("ListExpired called %d times, want 1", len(f.uploadRepo.ListExpiredBefore))
	}
	if len(f.storage.Calls()) != 0 {
		t.Fatalf("storage calls = %v, want none without expired sessions", f.storage.Calls())
	}
}

func TestReconcileOutboxFailuresListError(t *testing.T) {
	f := newFixture()
	f.outboxRepo.ListPendingErr = errors.New("query failed")

	f.worker.reconcileOutboxFailures(context.Background())

	if len(f.pub.Messages) != 0 {
		t.Fatalf("published %d messages, want 0 on list error", len(f.pub.Messages))
	}
}

func TestReconcileOutboxPublishFailureKeepsEventPending(t *testing.T) {
	f := newFixture()
	f.outboxRepo.Pending = []*domain.OutboxEvent{outboxEvent("evt-1", "media.ready")}
	f.pub.PublishErr = errors.New("broker down")

	f.worker.reconcileOutboxFailures(context.Background())

	if len(f.outboxRepo.Published) != 0 {
		t.Fatalf("Published = %v, want none when publish fails", f.outboxRepo.Published)
	}
	if len(f.pub.Messages) != 1 {
		t.Fatalf("publish attempted %d times, want 1", len(f.pub.Messages))
	}
}

func TestRunWithNothingToDo(t *testing.T) {
	f := newFixture()

	f.worker.Run(context.Background())

	if len(f.cache.LockReleases) != 1 {
		t.Fatalf("LockReleases = %v, want the lock released even with nothing to do", f.cache.LockReleases)
	}
	if len(f.pub.Messages) != 0 {
		t.Fatalf("published %d messages, want 0", len(f.pub.Messages))
	}
}
