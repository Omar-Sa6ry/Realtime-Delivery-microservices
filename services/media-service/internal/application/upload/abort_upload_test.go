package upload

import (
	"errors"
	"strings"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

func TestAbortUpload_Success(t *testing.T) {
	d := newUploadDeps(t)
	_, err := d.storage.CreateMultipartUpload(d.ctx, "users/user-1/media/media-1/original/file.bin", "application/octet-stream")
	if err != nil {
		t.Fatalf("seed multipart upload: %v", err)
	}
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	if err := d.newAbortUC().Execute(d.ctx, "user-1", "upload-1"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !hasCallPrefix(d.storage.Calls(), "AbortMultipartUpload:") {
		t.Fatalf("storage abort must be invoked, calls = %v", d.storage.Calls())
	}
	if len(d.uploadRepo.StatusUpdates) != 1 || d.uploadRepo.StatusUpdates[0].Status != domain.UploadStatusAborted {
		t.Fatalf("upload status updates = %+v", d.uploadRepo.StatusUpdates)
	}
	if len(d.mediaRepo.UpdateStatuses) != 1 {
		t.Fatalf("media status updates = %d, want 1", len(d.mediaRepo.UpdateStatuses))
	}
	call := d.mediaRepo.UpdateStatuses[0]
	if call.MediaID != "media-1" || call.Current != domain.MediaStatusUploading || call.Next != domain.MediaStatusAborted {
		t.Fatalf("unexpected media status call: %+v", call)
	}
	if d.quotaRepo.Decrements != 1 {
		t.Fatalf("capacity must be released, decrements = %d", d.quotaRepo.Decrements)
	}
	if len(d.cache.LockReleases) != 1 || d.cache.LockReleases[0] != "media-1" {
		t.Fatalf("lock releases = %v", d.cache.LockReleases)
	}
}

func TestAbortUpload_SessionNotFound(t *testing.T) {
	d := newUploadDeps(t)
	err := d.newAbortUC().Execute(d.ctx, "user-1", "missing")
	if !errors.Is(err, domain.ErrUploadSessionNotFound) {
		t.Fatalf("err = %v, want ErrUploadSessionNotFound", err)
	}
}

func TestAbortUpload_Unauthorized(t *testing.T) {
	d := newUploadDeps(t)
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	err := d.newAbortUC().Execute(d.ctx, "user-2", "upload-1")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestAbortUpload_LockError(t *testing.T) {
	d := newUploadDeps(t)
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	d.cache.LockErr = errors.New("lock down")
	err := d.newAbortUC().Execute(d.ctx, "user-1", "upload-1")
	if err == nil || !strings.Contains(err.Error(), "acquire abort lock") {
		t.Fatalf("err = %v, want acquire abort lock error", err)
	}
}

func TestAbortUpload_LockDenied(t *testing.T) {
	d := newUploadDeps(t)
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	d.cache.LockDenied = true
	err := d.newAbortUC().Execute(d.ctx, "user-1", "upload-1")
	if err == nil || !strings.Contains(err.Error(), "currently being processed") {
		t.Fatalf("err = %v, want currently being processed error", err)
	}
}

func TestAbortUpload_StorageErrorTolerated(t *testing.T) {
	d := newUploadDeps(t)
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	d.storage.SetError("AbortMultipartUpload", errors.New("s3 down"))
	if err := d.newAbortUC().Execute(d.ctx, "user-1", "upload-1"); err != nil {
		t.Fatalf("storage failure must be tolerated: %v", err)
	}
	if len(d.uploadRepo.StatusUpdates) != 1 || d.uploadRepo.StatusUpdates[0].Status != domain.UploadStatusAborted {
		t.Fatalf("upload status updates = %+v", d.uploadRepo.StatusUpdates)
	}
}

func TestAbortUpload_UploadStatusErrorTolerated(t *testing.T) {
	d := newUploadDeps(t)
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	d.uploadRepo.UpdateStatusErr = errors.New("ddb down")
	if err := d.newAbortUC().Execute(d.ctx, "user-1", "upload-1"); err != nil {
		t.Fatalf("upload status failure must be tolerated: %v", err)
	}
}

func TestAbortUpload_MediaStatusErrorTolerated(t *testing.T) {
	d := newUploadDeps(t)
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	d.mediaRepo.UpdateStatusErr = errors.New("ddb down")
	if err := d.newAbortUC().Execute(d.ctx, "user-1", "upload-1"); err != nil {
		t.Fatalf("media status failure must be tolerated: %v", err)
	}
	if d.quotaRepo.Decrements != 1 {
		t.Fatalf("capacity must still be released, decrements = %d", d.quotaRepo.Decrements)
	}
}

func TestGetUploadStatus_Success(t *testing.T) {
	d := newUploadDeps(t)
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	session.TotalParts = 4
	session.CompletedParts = []domain.UploadPart{{PartNumber: 1, ETag: "e1"}, {PartNumber: 3, ETag: "e3"}}
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	uc := NewGetUploadStatusUseCase(d.uploadRepo, d.storage)
	out, err := uc.Execute(d.ctx, "user-1", "upload-1")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Status != domain.UploadStatusUploading || out.TotalParts != 4 || out.CompletedParts != 2 {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(out.MissingParts) != 2 || out.MissingParts[0] != 2 || out.MissingParts[1] != 4 {
		t.Fatalf("missing parts = %v, want [2 4]", out.MissingParts)
	}
	if out.ExpiresAt.IsZero() {
		t.Fatal("ExpiresAt must be set")
	}
}

func TestGetUploadStatus_SessionNotFound(t *testing.T) {
	d := newUploadDeps(t)
	uc := NewGetUploadStatusUseCase(d.uploadRepo, d.storage)
	_, err := uc.Execute(d.ctx, "user-1", "missing")
	if !errors.Is(err, domain.ErrUploadSessionNotFound) {
		t.Fatalf("err = %v, want ErrUploadSessionNotFound", err)
	}
}

func TestGetUploadStatus_Unauthorized(t *testing.T) {
	d := newUploadDeps(t)
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	uc := NewGetUploadStatusUseCase(d.uploadRepo, d.storage)
	_, err := uc.Execute(d.ctx, "user-2", "upload-1")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}
