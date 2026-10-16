package upload

import (
	"context"
	"strings"
	"testing"
	"time"

	sharedratelimiter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/ratelimiter"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/validation"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	testMaxFileSize = 100 << 20 // 100 MiB
	testMinPartSize = 5 << 20   // 5 MiB
	testSessionTTL  = time.Hour
	testPresignTTL  = 15 * time.Minute
)

func allowedTestTypes() map[string]struct{} {
	return map[string]struct{}{
		"image/jpeg":               {},
		"image/png":                {},
		"application/pdf":          {},
		"text/plain":               {},
		"video/mp4":                {},
		"application/octet-stream": {},
	}
}

func newTestLimiter(t *testing.T, limit int, window time.Duration) (*sharedratelimiter.RateLimiter, *redis.Client) {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		s.Close()
	})
	return sharedratelimiter.NewRateLimiter(client, limit, window), client
}

type uploadDeps struct {
	mediaRepo  *testutil.MockMediaRepository
	uploadRepo *testutil.MockUploadRepository
	quotaRepo  *testutil.MockQuotaRepository
	storage    *testutil.MemStorage
	cache      *testutil.MockCache
	outbox     *testutil.MockOutboxRepository
	publisher  *testutil.MockPublisher
	validator  *validation.Validator
	limiter    *sharedratelimiter.RateLimiter
	ctx        context.Context
}

func newUploadDeps(t *testing.T) *uploadDeps {
	t.Helper()
	observability.RegisterMediaMetrics()
	storage := testutil.NewMemStorage()
	limiter, _ := newTestLimiter(t, 100, time.Minute)
	return &uploadDeps{
		mediaRepo:  testutil.NewMockMediaRepository(),
		uploadRepo: testutil.NewMockUploadRepository(),
		quotaRepo:  testutil.NewMockQuotaRepository(),
		storage:    storage,
		cache:      testutil.NewMockCache(),
		outbox:     testutil.NewMockOutboxRepository(),
		publisher:  testutil.NewMockPublisher(),
		validator:  validation.NewValidator(allowedTestTypes(), storage, testMaxFileSize),
		limiter:    limiter,
		ctx:        testutil.Context("user-1", "trace-1"),
	}
}

func (d *uploadDeps) newCreateSessionUC() *CreateSessionUseCase {
	return NewCreateSessionUseCase(
		d.mediaRepo, d.uploadRepo, d.quotaRepo, d.storage, d.cache,
		d.validator, d.limiter, testSessionTTL, testPresignTTL, testMinPartSize, testMaxFileSize,
	)
}

func (d *uploadDeps) newCompleteUC() *CompleteUploadUseCase {
	return NewCompleteUploadUseCase(
		d.uploadRepo, d.mediaRepo, d.outbox, d.quotaRepo,
		d.storage, d.cache, d.publisher, d.validator,
	)
}

func (d *uploadDeps) newAbortUC() *AbortUploadUseCase {
	return NewAbortUploadUseCase(d.uploadRepo, d.mediaRepo, d.quotaRepo, d.storage, d.cache)
}

func (d *uploadDeps) newRenewUC(defaultExpiry time.Duration) *RenewPresignedUseCase {
	return NewRenewPresignedUseCase(d.uploadRepo, d.storage, d.cache, defaultExpiry)
}

func validCreateInput(size int64) CreateSessionInput {
	return CreateSessionInput{
		UserID:      "user-1",
		FileName:    "photo.jpg",
		ContentType: "image/jpeg",
		Size:        size,
	}
}

func hasCallPrefix(calls []string, prefix string) bool {
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}
