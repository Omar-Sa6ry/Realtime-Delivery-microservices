package download

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	sharedratelimiter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/ratelimiter"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const defaultDownloadExpiry = 15 * time.Minute

type downloadDeps struct {
	mediaRepo   *testutil.MockMediaRepository
	versionRepo *testutil.MockVersionRepository
	storage     *testutil.MemStorage
	limiter     *sharedratelimiter.RateLimiter
	ctx         context.Context
}

func newDownloadLimiter(t *testing.T, limit int, window time.Duration) (*sharedratelimiter.RateLimiter, *redis.Client) {
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

func newDownloadDeps(t *testing.T) *downloadDeps {
	t.Helper()
	limiter, _ := newDownloadLimiter(t, 1000, time.Minute)
	return &downloadDeps{
		mediaRepo:   testutil.NewMockMediaRepository(),
		versionRepo: testutil.NewMockVersionRepository(),
		storage:     testutil.NewMemStorage(),
		limiter:     limiter,
		ctx:         testutil.Context("user-1", "trace-1"),
	}
}

func (d *downloadDeps) newUC() *GetDownloadUrlUseCase {
	return NewGetDownloadUrlUseCase(d.mediaRepo, d.versionRepo, d.storage, d.limiter, defaultDownloadExpiry)
}

func (d *downloadDeps) seedMedia(t *testing.T, id, owner string) *domain.Media {
	t.Helper()
	m := testutil.NewMedia(id, owner)
	if err := d.mediaRepo.Create(d.ctx, m); err != nil {
		t.Fatalf("seed media: %v", err)
	}
	return m
}

func TestGetDownloadUrl_SuccessOriginal(t *testing.T) {
	d := newDownloadDeps(t)
	seeded := d.seedMedia(t, "media-1", "user-1")

	out, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "media-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.URL == "" || out.ContentType != seeded.ContentType {
		t.Fatalf("unexpected output: %+v", out)
	}
	if out.ExpiresAt.Before(time.Now().Add(defaultDownloadExpiry-time.Minute)) ||
		out.ExpiresAt.After(time.Now().Add(defaultDownloadExpiry+time.Minute)) {
		t.Fatalf("ExpiresAt %v outside default expiry window", out.ExpiresAt)
	}
	calls := d.storage.PresignedCalls()
	if len(calls) != 1 {
		t.Fatalf("presigned calls = %d", len(calls))
	}
	if calls[0].Op != "GeneratePresignedGETWithRange" || calls[0].ObjectKey != seeded.ObjectKey ||
		calls[0].Expiry != defaultDownloadExpiry || calls[0].RangeHeader != "" {
		t.Fatalf("unexpected presigned call: %+v", calls[0])
	}
	if len(d.versionRepo.ListCalls) != 0 {
		t.Fatal("original download must not query versions")
	}
}

func TestGetDownloadUrl_RateLimitExceeded(t *testing.T) {
	d := newDownloadDeps(t)
	limiter, _ := newDownloadLimiter(t, 1, time.Minute)
	d.limiter = limiter
	d.seedMedia(t, "media-1", "user-1")
	uc := d.newUC()

	if _, err := uc.Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "media-1"}); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	_, err := uc.Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "media-1"})
	if !errors.Is(err, domain.ErrRateLimitExceeded) {
		t.Fatalf("err = %v, want ErrRateLimitExceeded", err)
	}
}

func TestGetDownloadUrl_RateLimiterError(t *testing.T) {
	d := newDownloadDeps(t)
	limiter, client := newDownloadLimiter(t, 10, time.Minute)
	if err := client.Close(); err != nil {
		t.Fatalf("close redis client: %v", err)
	}
	d.limiter = limiter
	d.seedMedia(t, "media-1", "user-1")
	_, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "media-1"})
	if err == nil || !strings.Contains(err.Error(), "rate limiter check") {
		t.Fatalf("err = %v, want rate limiter check error", err)
	}
}

func TestGetDownloadUrl_NotFound(t *testing.T) {
	d := newDownloadDeps(t)
	_, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "missing"})
	if !errors.Is(err, domain.ErrMediaNotFound) {
		t.Fatalf("err = %v, want ErrMediaNotFound", err)
	}
}

func TestGetDownloadUrl_NotOwnerUnauthorized(t *testing.T) {
	d := newDownloadDeps(t)
	d.seedMedia(t, "media-1", "user-1")
	_, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "user-2", MediaID: "media-1"})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestGetDownloadUrl_AdminBypassesOwnership(t *testing.T) {
	d := newDownloadDeps(t)
	d.seedMedia(t, "media-1", "user-1")
	out, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "admin-1", IsAdmin: true, MediaID: "media-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.URL == "" {
		t.Fatal("URL must be generated for admin")
	}
}

func TestGetDownloadUrl_Quarantined(t *testing.T) {
	d := newDownloadDeps(t)
	m := d.seedMedia(t, "media-1", "user-1")
	d.mediaRepo.Store[m.MediaID].Status = domain.MediaStatusQuarantined
	_, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "media-1"})
	if !errors.Is(err, domain.ErrMediaQuarantined) {
		t.Fatalf("err = %v, want ErrMediaQuarantined", err)
	}
}

func TestGetDownloadUrl_UnavailableStatuses(t *testing.T) {
	statuses := []domain.MediaStatus{domain.MediaStatusFailed, domain.MediaStatusDeleted, domain.MediaStatusAborted}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			d := newDownloadDeps(t)
			m := d.seedMedia(t, "media-1", "user-1")
			d.mediaRepo.Store[m.MediaID].Status = status
			_, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "media-1"})
			if err == nil || !strings.Contains(err.Error(), "not available for download") {
				t.Fatalf("err = %v, want not available for download error", err)
			}
		})
	}
}

func TestGetDownloadUrl_VersionTypeResolvesObjectKey(t *testing.T) {
	d := newDownloadDeps(t)
	d.seedMedia(t, "media-1", "user-1")
	d.versionRepo.Versions["media-1"] = []*domain.MediaVersion{
		{VersionID: "v1", MediaID: "media-1", VersionType: domain.VersionTypeThumbnail, ObjectKey: "users/user-1/media/media-1/thumbnail/photo.webp", ContentType: "image/webp"},
		{VersionID: "v2", MediaID: "media-1", VersionType: domain.VersionType720p, ObjectKey: "users/user-1/media/media-1/720p/video.mp4", ContentType: "video/mp4"},
	}

	out, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "media-1", VersionType: "thumbnail"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.ContentType != "image/webp" {
		t.Fatalf("ContentType = %q, want image/webp", out.ContentType)
	}
	calls := d.storage.PresignedCalls()
	if len(calls) != 1 || calls[0].ObjectKey != "users/user-1/media/media-1/thumbnail/photo.webp" {
		t.Fatalf("unexpected presigned calls: %+v", calls)
	}
}

func TestGetDownloadUrl_UnknownVersionFallsBackToOriginal(t *testing.T) {
	d := newDownloadDeps(t)
	seeded := d.seedMedia(t, "media-1", "user-1")
	d.versionRepo.Versions["media-1"] = []*domain.MediaVersion{
		{VersionID: "v1", MediaID: "media-1", VersionType: domain.VersionTypeThumbnail, ObjectKey: "thumb.webp", ContentType: "image/webp"},
	}

	out, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "media-1", VersionType: "1080p"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.ContentType != seeded.ContentType {
		t.Fatalf("ContentType = %q, want original %q", out.ContentType, seeded.ContentType)
	}
	calls := d.storage.PresignedCalls()
	if len(calls) != 1 || calls[0].ObjectKey != seeded.ObjectKey {
		t.Fatalf("unexpected presigned calls: %+v", calls)
	}
}

func TestGetDownloadUrl_VersionRepoErrorFallsBackToOriginal(t *testing.T) {
	d := newDownloadDeps(t)
	seeded := d.seedMedia(t, "media-1", "user-1")
	d.versionRepo.ListErr = errors.New("ddb down")

	out, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "media-1", VersionType: "thumbnail"})
	if err != nil {
		t.Fatalf("Execute must tolerate version repo errors: %v", err)
	}
	if out.ContentType != seeded.ContentType {
		t.Fatalf("ContentType = %q, want original %q", out.ContentType, seeded.ContentType)
	}
	calls := d.storage.PresignedCalls()
	if len(calls) != 1 || calls[0].ObjectKey != seeded.ObjectKey {
		t.Fatalf("unexpected presigned calls: %+v", calls)
	}
}

func TestGetDownloadUrl_ExpiryClamping(t *testing.T) {
	tests := []struct {
		name         string
		expirySecond int
		wantExpiry   time.Duration
	}{
		{"zero uses default", 0, defaultDownloadExpiry},
		{"negative uses default", -1, defaultDownloadExpiry},
		{"above cap uses default", 90000, defaultDownloadExpiry},
		{"within bounds honored", 60, 60 * time.Second},
		{"at cap accepted", 86400, 86400 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDownloadDeps(t)
			d.seedMedia(t, "media-1", "user-1")
			out, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{
				UserID: "user-1", MediaID: "media-1", ExpirySeconds: tt.expirySecond,
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			calls := d.storage.PresignedCalls()
			if len(calls) != 1 || calls[0].Expiry != tt.wantExpiry {
				t.Fatalf("expiry = %+v, want %v", calls, tt.wantExpiry)
			}
			if out.ExpiresAt.After(time.Now().Add(tt.wantExpiry + time.Minute)) {
				t.Fatalf("ExpiresAt %v too far in the future", out.ExpiresAt)
			}
		})
	}
}

func TestGetDownloadUrl_PresignError(t *testing.T) {
	d := newDownloadDeps(t)
	d.seedMedia(t, "media-1", "user-1")
	d.storage.SetError("GeneratePresignedGETWithRange", errors.New("s3 down"))
	_, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{UserID: "user-1", MediaID: "media-1"})
	if !errors.Is(err, domain.ErrS3OperationFailed) {
		t.Fatalf("err = %v, want ErrS3OperationFailed", err)
	}
}

func TestGetDownloadUrl_RangeForwarded(t *testing.T) {
	d := newDownloadDeps(t)
	d.seedMedia(t, "media-1", "user-1")
	out, err := d.newUC().Execute(d.ctx, GetDownloadUrlInput{
		UserID: "user-1", MediaID: "media-1", Range: "bytes=0-1023",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.URL, "range=bytes=0-1023") {
		t.Fatalf("URL must carry the range parameter: %q", out.URL)
	}
	calls := d.storage.PresignedCalls()
	if len(calls) != 1 || calls[0].RangeHeader != "bytes=0-1023" {
		t.Fatalf("unexpected presigned calls: %+v", calls)
	}
}
