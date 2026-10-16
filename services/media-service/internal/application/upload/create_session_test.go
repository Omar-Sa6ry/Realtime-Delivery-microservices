package upload

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
)

func TestCalculateParts(t *testing.T) {
	const min5 = int64(5 << 20)
	tests := []struct {
		name      string
		fileSize  int64
		minPart   int64
		wantParts int
		wantSize  int64
	}{
		{"zero size", 0, min5, 1, 0},
		{"below min part size", 1 << 20, min5, 1, 1 << 20},
		{"exact min part size", min5, min5, 1, min5},
		{"exact multiple of min part", 10 << 20, min5, 2, min5},
		{"non multiple rounds up part count", 12<<20 + 1, min5, 3, min5},
		{"exactly 10000 parts", 10000 * min5, min5, 10000, min5},
		{"part count capped at 10000", 10001 * min5, min5, 10000, 5243405},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, size := calculateParts(tt.fileSize, tt.minPart)
			if parts != tt.wantParts || size != tt.wantSize {
				t.Fatalf("calculateParts(%d, %d) = (%d, %d), want (%d, %d)",
					tt.fileSize, tt.minPart, parts, size, tt.wantParts, tt.wantSize)
			}
		})
	}
}

func TestBuildObjectKey(t *testing.T) {
	if got := buildObjectKey("u1", "d1", "m1", "f.jpg"); got != "users/u1/deliveries/d1/media/m1/original/f.jpg" {
		t.Fatalf("with deliveryID = %q", got)
	}
	if got := buildObjectKey("u1", "", "m1", "f.jpg"); got != "users/u1/media/m1/original/f.jpg" {
		t.Fatalf("without deliveryID = %q", got)
	}
}

func TestCreateSession_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		size         int64
		deliveryID   string
		wantParts    int
		wantPartSize int64
	}{
		{"small file single part", 1 << 20, "", 1, 1 << 20},
		{"exact multiple of min part", 10 << 20, "", 2, testMinPartSize},
		{"non multiple rounds up parts", 12 << 20, "", 3, testMinPartSize},
		{"delivery scoped key", 1 << 20, "deliv-9", 1, 1 << 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newUploadDeps(t)
			in := validCreateInput(tt.size)
			in.DeliveryID = tt.deliveryID

			out, err := d.newCreateSessionUC().Execute(d.ctx, in)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out.TotalParts != tt.wantParts || out.PartSize != tt.wantPartSize {
				t.Fatalf("parts = (%d, %d), want (%d, %d)", out.TotalParts, out.PartSize, tt.wantParts, tt.wantPartSize)
			}
			if len(out.PresignedParts) != tt.wantParts {
				t.Fatalf("presigned parts = %d, want %d", len(out.PresignedParts), tt.wantParts)
			}
			for i, p := range out.PresignedParts {
				if p.PartNumber != i+1 || p.PresignedURL == "" {
					t.Fatalf("presigned part %d = %+v", i, p)
				}
			}
			if out.MediaID == "" || out.UploadID == "" || out.S3UploadID == "" {
				t.Fatalf("ids must be generated: %+v", out)
			}
			wantExpiry := time.Now().Add(testSessionTTL)
			if out.ExpiresAt.Before(wantExpiry.Add(-time.Minute)) || out.ExpiresAt.After(wantExpiry.Add(time.Minute)) {
				t.Fatalf("ExpiresAt %v outside session TTL window", out.ExpiresAt)
			}

			if len(d.mediaRepo.Created) != 1 {
				t.Fatalf("media created = %d, want 1", len(d.mediaRepo.Created))
			}
			m := d.mediaRepo.Created[0]
			if m.OwnerID != "user-1" || m.Size != tt.size || m.ContentType != "image/jpeg" {
				t.Fatalf("unexpected media record: %+v", m)
			}
			if m.MediaType != domain.MediaTypeImage {
				t.Fatalf("MediaType = %s, want IMAGE", m.MediaType)
			}
			if tt.deliveryID == "" {
				if !strings.HasPrefix(m.ObjectKey, "users/user-1/media/"+out.MediaID+"/original/") {
					t.Fatalf("object key = %q", m.ObjectKey)
				}
			} else if !strings.Contains(m.ObjectKey, "/deliveries/deliv-9/") {
				t.Fatalf("object key = %q", m.ObjectKey)
			}

			if len(d.uploadRepo.Created) != 1 {
				t.Fatalf("sessions created = %d, want 1", len(d.uploadRepo.Created))
			}
			s := d.uploadRepo.Created[0]
			if s.Status != domain.UploadStatusUploading || s.S3UploadID != out.S3UploadID || s.TotalParts != tt.wantParts {
				t.Fatalf("unexpected session: %+v", s)
			}
			if len(d.mediaRepo.UpdateStatuses) != 1 {
				t.Fatalf("media status updates = %d, want 1", len(d.mediaRepo.UpdateStatuses))
			}
			call := d.mediaRepo.UpdateStatuses[0]
			if call.MediaID != out.MediaID || call.Current != domain.MediaStatusPending || call.Next != domain.MediaStatusUploading {
				t.Fatalf("unexpected status call: %+v", call)
			}
			if len(d.quotaRepo.Increments) != 1 || d.quotaRepo.Increments[0] != tt.size {
				t.Fatalf("quota increments = %v, want [%d]", d.quotaRepo.Increments, tt.size)
			}
			if !hasCallPrefix(d.storage.Calls(), "CreateMultipartUpload:") {
				t.Fatalf("storage calls = %v", d.storage.Calls())
			}
			if !hasCallPrefix(d.storage.Calls(), "GeneratePresignedParts:") {
				t.Fatalf("storage calls = %v", d.storage.Calls())
			}
		})
	}
}

func TestCreateSession_ValidationTooSmall(t *testing.T) {
	d := newUploadDeps(t)
	_, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(0))
	if !errors.Is(err, domain.ErrFileTooSmall) {
		t.Fatalf("err = %v, want ErrFileTooSmall", err)
	}
	if len(d.mediaRepo.Created) != 0 || len(d.uploadRepo.Created) != 0 {
		t.Fatal("nothing must be persisted on validation failure")
	}
}

func TestCreateSession_ValidationTooLarge(t *testing.T) {
	d := newUploadDeps(t)
	_, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(testMaxFileSize+1))
	if !errors.Is(err, domain.ErrFileTooLarge) {
		t.Fatalf("err = %v, want ErrFileTooLarge", err)
	}
}

func TestCreateSession_ValidationContentTypeNotAllowed(t *testing.T) {
	d := newUploadDeps(t)
	in := validCreateInput(1 << 20)
	in.ContentType = "application/x-msdownload"
	_, err := d.newCreateSessionUC().Execute(d.ctx, in)
	if !errors.Is(err, domain.ErrInvalidFileType) {
		t.Fatalf("err = %v, want ErrInvalidFileType", err)
	}
}

func TestCreateSession_ValidationExtensionMismatch(t *testing.T) {
	d := newUploadDeps(t)
	in := validCreateInput(1 << 20)
	in.FileName = "payload.exe"
	_, err := d.newCreateSessionUC().Execute(d.ctx, in)
	if !errors.Is(err, domain.ErrInvalidFileType) {
		t.Fatalf("err = %v, want ErrInvalidFileType", err)
	}
}

func TestCreateSession_ValidationMissingExtension(t *testing.T) {
	d := newUploadDeps(t)
	in := validCreateInput(1 << 20)
	in.FileName = "README"
	in.ContentType = "text/plain"
	_, err := d.newCreateSessionUC().Execute(d.ctx, in)
	if !errors.Is(err, domain.ErrInvalidFileType) {
		t.Fatalf("err = %v, want ErrInvalidFileType", err)
	}
}

func TestCreateSession_RateLimitExceeded(t *testing.T) {
	d := newUploadDeps(t)
	limiter, _ := newTestLimiter(t, 1, time.Minute)
	d.limiter = limiter
	uc := d.newCreateSessionUC()
	if _, err := uc.Execute(d.ctx, validCreateInput(1<<20)); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	_, err := uc.Execute(d.ctx, validCreateInput(1<<20))
	if !errors.Is(err, domain.ErrRateLimitExceeded) {
		t.Fatalf("err = %v, want ErrRateLimitExceeded", err)
	}
}

func TestCreateSession_RateLimiterError(t *testing.T) {
	d := newUploadDeps(t)
	limiter, client := newTestLimiter(t, 10, time.Minute)
	if err := client.Close(); err != nil {
		t.Fatalf("close redis client: %v", err)
	}
	d.limiter = limiter
	_, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(1<<20))
	if err == nil || !strings.Contains(err.Error(), "rate limiter check") {
		t.Fatalf("err = %v, want rate limiter check error", err)
	}
}

func TestCreateSession_IdempotencyConflictOnRepeat(t *testing.T) {
	d := newUploadDeps(t)
	in := validCreateInput(1 << 20)
	in.IdempotencyKey = "idem-1"
	uc := d.newCreateSessionUC()
	if _, err := uc.Execute(d.ctx, in); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	_, err := uc.Execute(d.ctx, in)
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("err = %v, want ErrIdempotencyConflict", err)
	}
	if len(d.mediaRepo.Created) != 1 {
		t.Fatalf("second call must not create another media, created = %d", len(d.mediaRepo.Created))
	}
}

func TestCreateSession_IdempotencyStoreError(t *testing.T) {
	d := newUploadDeps(t)
	d.cache.IdemErr = errors.New("redis down")
	in := validCreateInput(1 << 20)
	in.IdempotencyKey = "idem-1"
	_, err := d.newCreateSessionUC().Execute(d.ctx, in)
	if err == nil || !strings.Contains(err.Error(), "idempotency check") {
		t.Fatalf("err = %v, want idempotency check error", err)
	}
}

func TestCreateSession_QuotaReadError(t *testing.T) {
	d := newUploadDeps(t)
	d.quotaRepo.GetErr = errors.New("ddb down")
	_, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(1<<20))
	if err == nil || !strings.Contains(err.Error(), "quota read") {
		t.Fatalf("err = %v, want quota read error", err)
	}
}

func TestCreateSession_QuotaExceeded(t *testing.T) {
	d := newUploadDeps(t)
	d.quotaRepo.Usage = &domain.QuotaUsage{QuotaBytes: 100, MaxConcurrent: 10}
	_, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(200))
	if !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("err = %v, want ErrQuotaExceeded", err)
	}
	if len(d.mediaRepo.Created) != 0 {
		t.Fatal("nothing must be persisted when quota is exceeded")
	}
}

func TestCreateSession_ConcurrentUploadsExceeded(t *testing.T) {
	d := newUploadDeps(t)
	d.quotaRepo.Usage = &domain.QuotaUsage{QuotaBytes: 1 << 40, MaxConcurrent: 2, ActiveUploads: 2}
	_, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(1<<20))
	if !errors.Is(err, domain.ErrConcurrentUploadsExceeded) {
		t.Fatalf("err = %v, want ErrConcurrentUploadsExceeded", err)
	}
}

func TestCreateSession_CreateMultipartUploadError(t *testing.T) {
	d := newUploadDeps(t)
	d.storage.SetError("CreateMultipartUpload", errors.New("s3 down"))
	_, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(1<<20))
	if !errors.Is(err, domain.ErrS3OperationFailed) {
		t.Fatalf("err = %v, want ErrS3OperationFailed", err)
	}
}

func TestCreateSession_PresignErrorAbortsMultipart(t *testing.T) {
	d := newUploadDeps(t)
	d.storage.SetError("GeneratePresignedParts", errors.New("presign down"))
	_, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(1<<20))
	if !errors.Is(err, domain.ErrS3OperationFailed) {
		t.Fatalf("err = %v, want ErrS3OperationFailed", err)
	}
	if !hasCallPrefix(d.storage.Calls(), "AbortMultipartUpload:") {
		t.Fatalf("multipart upload must be aborted, calls = %v", d.storage.Calls())
	}
}

func TestCreateSession_MediaCreateErrorAbortsMultipart(t *testing.T) {
	d := newUploadDeps(t)
	d.mediaRepo.CreateErr = errors.New("ddb down")
	_, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(1<<20))
	if err == nil || !strings.Contains(err.Error(), "persist media record") {
		t.Fatalf("err = %v, want persist media record error", err)
	}
	if !hasCallPrefix(d.storage.Calls(), "AbortMultipartUpload:") {
		t.Fatalf("multipart upload must be aborted, calls = %v", d.storage.Calls())
	}
}

func TestCreateSession_UploadCreateErrorAbortsMultipart(t *testing.T) {
	d := newUploadDeps(t)
	d.uploadRepo.CreateErr = errors.New("ddb down")
	_, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(1<<20))
	if err == nil || !strings.Contains(err.Error(), "persist upload session") {
		t.Fatalf("err = %v, want persist upload session error", err)
	}
	if !hasCallPrefix(d.storage.Calls(), "AbortMultipartUpload:") {
		t.Fatalf("multipart upload must be aborted, calls = %v", d.storage.Calls())
	}
}

func TestCreateSession_UpdateStatusFailureIsNonFatal(t *testing.T) {
	d := newUploadDeps(t)
	d.mediaRepo.UpdateStatusErr = errors.New("conditional check failed")
	out, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(1<<20))
	if err != nil {
		t.Fatalf("Execute must succeed despite status update failure: %v", err)
	}
	if len(d.uploadRepo.Created) != 1 || out.UploadID != d.uploadRepo.Created[0].UploadID {
		t.Fatal("upload session must still be persisted")
	}
}

func TestCreateSession_IncrementUsageFailureIsNonFatal(t *testing.T) {
	d := newUploadDeps(t)
	d.quotaRepo.IncrementErr = errors.New("ddb down")
	if _, err := d.newCreateSessionUC().Execute(d.ctx, validCreateInput(1<<20)); err != nil {
		t.Fatalf("Execute must succeed despite quota increment failure: %v", err)
	}
	if len(d.quotaRepo.Increments) != 1 {
		t.Fatalf("increment must still be attempted, increments = %v", d.quotaRepo.Increments)
	}
}
