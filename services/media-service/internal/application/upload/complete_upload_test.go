package upload

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/validation"
)

type completeFixture struct {
	objectKey  string
	s3UploadID string
	data       []byte
	media      *domain.Media
	session    *domain.UploadSession
}

func seedCompleteFixture(t *testing.T, d *uploadDeps) *completeFixture {
	t.Helper()
	ctx := testutil.Context("user-1", "trace-1")
	objectKey := "users/user-1/media/media-1/original/photo.jpg"
	data := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 96)...)

	s3ID, err := d.storage.CreateMultipartUpload(ctx, objectKey, "image/jpeg")
	if err != nil {
		t.Fatalf("seed multipart upload: %v", err)
	}
	d.storage.SetObject(objectKey, data)

	media := testutil.NewMedia("media-1", "user-1")
	media.Status = domain.MediaStatusUploading
	media.ContentType = "image/jpeg"
	media.ObjectKey = objectKey
	media.Size = int64(len(data))
	media.Checksum = ""
	if err := d.mediaRepo.Create(ctx, media); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	session.S3UploadID = s3ID
	session.ObjectKey = objectKey
	session.TotalParts = 1
	session.PartSize = int64(len(data))
	session.CompletedParts = []domain.UploadPart{{PartNumber: 1, ETag: "etag-1"}}
	if err := d.uploadRepo.Create(ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	return &completeFixture{objectKey: objectKey, s3UploadID: s3ID, data: data, media: media, session: session}
}

func (f *completeFixture) input() CompleteUploadInput {
	return CompleteUploadInput{UserID: "user-1", UploadID: "upload-1", Parts: f.session.CompletedParts}
}

func TestCompleteUpload_Success(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	sum, err := validation.ComputeChecksum(bytes.NewReader(f.data))
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	d.mediaRepo.Store["media-1"].Checksum = sum

	out, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.MediaID != "media-1" || out.Status != domain.MediaStatusUploaded || out.Size != int64(len(f.data)) {
		t.Fatalf("unexpected output: %+v", out)
	}
	if out.ContentType != "image/jpeg" || out.MediaType != string(domain.MediaTypeImage) {
		t.Fatalf("unexpected content fields: %+v", out)
	}

	if len(d.mediaRepo.UpdateStatuses) != 1 {
		t.Fatalf("media status updates = %d, want 1", len(d.mediaRepo.UpdateStatuses))
	}
	call := d.mediaRepo.UpdateStatuses[0]
	if call.MediaID != "media-1" || call.Current != domain.MediaStatusUploading || call.Next != domain.MediaStatusUploaded {
		t.Fatalf("unexpected media status call: %+v", call)
	}
	if len(d.uploadRepo.StatusUpdates) == 0 {
		t.Fatal("upload session status must be updated")
	}
	last := d.uploadRepo.StatusUpdates[len(d.uploadRepo.StatusUpdates)-1]
	if last.UploadID != "upload-1" || last.Status != domain.UploadStatusCompleted {
		t.Fatalf("unexpected upload status call: %+v", last)
	}
	if d.quotaRepo.Decrements != 1 || len(d.quotaRepo.AddedBytes) != 1 || d.quotaRepo.AddedBytes[0] != int64(len(f.data)) {
		t.Fatalf("quota updates: decrements=%d addedBytes=%v", d.quotaRepo.Decrements, d.quotaRepo.AddedBytes)
	}
	if len(d.outbox.Created) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(d.outbox.Created))
	}
	ev := d.outbox.Created[0]
	if ev.EventType != kafka.TopicUploadCompleted || ev.AggregateID != "media-1" || ev.Status != domain.OutboxStatusPending {
		t.Fatalf("unexpected outbox event: %+v", ev)
	}
	var payload struct {
		MediaID string `json:"mediaId"`
		UserID  string `json:"userId"`
	}
	testutil.DecodeEnvelope(t, ev.Payload, &payload)
	if payload.MediaID != "media-1" || payload.UserID != "user-1" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if len(d.cache.LockReleases) != 1 || d.cache.LockReleases[0] != "media-1" {
		t.Fatalf("lock releases = %v", d.cache.LockReleases)
	}
}

func TestCompleteUpload_SessionNotFound(t *testing.T) {
	d := newUploadDeps(t)
	_, err := d.newCompleteUC().Execute(d.ctx, CompleteUploadInput{UserID: "user-1", UploadID: "missing"})
	if !errors.Is(err, domain.ErrUploadSessionNotFound) {
		t.Fatalf("err = %v, want ErrUploadSessionNotFound", err)
	}
}

func TestCompleteUpload_Unauthorized(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	in := f.input()
	in.UserID = "user-2"
	_, err := d.newCompleteUC().Execute(d.ctx, in)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestCompleteUpload_Expired(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.uploadRepo.Store["upload-1"].ExpiresAt = time.Now().Add(-time.Minute)
	_, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if !errors.Is(err, domain.ErrUploadExpired) {
		t.Fatalf("err = %v, want ErrUploadExpired", err)
	}
}

func TestCompleteUpload_IdempotentReplayReturnsMedia(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.mediaRepo.Store["media-1"].Status = domain.MediaStatusReady
	d.cache.IdemExisting["user-1:complete:upload-1"] = true

	in := f.input()
	in.IdempotencyKey = "idem-1"
	out, err := d.newCompleteUC().Execute(d.ctx, in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.MediaID != "media-1" || out.Status != domain.MediaStatusReady {
		t.Fatalf("unexpected replay output: %+v", out)
	}
	if hasCallPrefix(d.storage.Calls(), "CompleteMultipartUpload:") {
		t.Fatal("replay must not complete the S3 upload again")
	}
}

func TestCompleteUpload_IdempotentReplayMediaMissingFallsThrough(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	delete(d.mediaRepo.Store, "media-1")
	d.cache.IdemExisting["user-1:complete:upload-1"] = true

	in := f.input()
	in.IdempotencyKey = "idem-1"
	_, err := d.newCompleteUC().Execute(d.ctx, in)
	if err == nil || !strings.Contains(err.Error(), "fetch media after completion") || !errors.Is(err, domain.ErrMediaNotFound) {
		t.Fatalf("err = %v, want fetch media after completion wrapping ErrMediaNotFound", err)
	}
	if !hasCallPrefix(d.storage.Calls(), "CompleteMultipartUpload:") {
		t.Fatal("flow must fall through to S3 completion")
	}
}

func TestCompleteUpload_IdempotencyStoreErrorContinues(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.cache.IdemErr = errors.New("redis down")

	in := f.input()
	in.IdempotencyKey = "idem-1"
	out, err := d.newCompleteUC().Execute(d.ctx, in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Status != domain.MediaStatusUploaded {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestCompleteUpload_LockError(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.cache.LockErr = errors.New("lock down")
	_, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if err == nil || !strings.Contains(err.Error(), "acquire completion lock") {
		t.Fatalf("err = %v, want acquire completion lock error", err)
	}
}

func TestCompleteUpload_LockDenied(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.cache.LockDenied = true
	_, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("err = %v, want already in progress error", err)
	}
}

func TestCompleteUpload_LockReleaseErrorTolerated(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.cache.ReleaseErr = errors.New("release down")
	if _, err := d.newCompleteUC().Execute(d.ctx, f.input()); err != nil {
		t.Fatalf("Execute must tolerate lock release failure: %v", err)
	}
}

func TestCompleteUpload_S3CompleteError(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.storage.SetError("CompleteMultipartUpload", errors.New("s3 down"))
	_, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if !errors.Is(err, domain.ErrS3OperationFailed) {
		t.Fatalf("err = %v, want ErrS3OperationFailed", err)
	}
}

func TestCompleteUpload_S3UnknownUploadID(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.uploadRepo.Store["upload-1"].S3UploadID = "not-registered"
	_, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if !errors.Is(err, domain.ErrS3OperationFailed) {
		t.Fatalf("err = %v, want ErrS3OperationFailed", err)
	}
}

func TestCompleteUpload_HeadObjectError(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.storage.SetError("HeadObject", errors.New("s3 down"))
	_, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if !errors.Is(err, domain.ErrS3OperationFailed) {
		t.Fatalf("err = %v, want ErrS3OperationFailed", err)
	}
}

func TestCompleteUpload_MediaFetchError(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	delete(d.mediaRepo.Store, "media-1")
	_, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if err == nil || !strings.Contains(err.Error(), "fetch media after completion") {
		t.Fatalf("err = %v, want fetch media after completion error", err)
	}
}

func TestCompleteUpload_SizeMismatch(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.mediaRepo.Store["media-1"].Size = int64(len(f.data)) + 10
	_, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if !errors.Is(err, domain.ErrChecksumMismatch) {
		t.Fatalf("err = %v, want ErrChecksumMismatch", err)
	}
	if len(d.mediaRepo.UpdateStatuses) != 0 {
		t.Fatal("media must not be transitioned on size mismatch")
	}
}

func TestCompleteUpload_MagicBytesFailure(t *testing.T) {
	d := newUploadDeps(t)
	objectKey := "users/user-1/media/media-1/original/photo.jpg"
	data := []byte("definitely not a jpeg payload")
	s3ID, err := d.storage.CreateMultipartUpload(d.ctx, objectKey, "image/jpeg")
	if err != nil {
		t.Fatalf("seed multipart upload: %v", err)
	}
	d.storage.SetObject(objectKey, data)
	media := testutil.NewMedia("media-1", "user-1")
	media.Status = domain.MediaStatusUploading
	media.ObjectKey = objectKey
	media.Size = int64(len(data))
	if err := d.mediaRepo.Create(d.ctx, media); err != nil {
		t.Fatalf("seed media: %v", err)
	}
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	session.S3UploadID = s3ID
	session.ObjectKey = objectKey
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	_, err = d.newCompleteUC().Execute(d.ctx, CompleteUploadInput{
		UserID: "user-1", UploadID: "upload-1", Parts: session.CompletedParts,
	})
	if !errors.Is(err, domain.ErrInvalidMagicBytes) {
		t.Fatalf("err = %v, want ErrInvalidMagicBytes", err)
	}
	callCount := len(d.mediaRepo.UpdateStatuses)
	if callCount != 1 {
		t.Fatalf("media status updates = %d, want 1", callCount)
	}
	call := d.mediaRepo.UpdateStatuses[0]
	if call.Current != domain.MediaStatusUploading || call.Next != domain.MediaStatusFailed {
		t.Fatalf("media must be marked FAILED, call = %+v", call)
	}
	if len(d.outbox.Created) != 0 {
		t.Fatal("no outbox event must be created on validation failure")
	}
}

func TestCompleteUpload_ChecksumMismatch(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.mediaRepo.Store["media-1"].Checksum = strings.Repeat("ab", 32)
	_, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if !errors.Is(err, domain.ErrChecksumMismatch) {
		t.Fatalf("err = %v, want ErrChecksumMismatch", err)
	}
	if len(d.mediaRepo.UpdateStatuses) != 1 {
		t.Fatalf("media status updates = %d, want 1", len(d.mediaRepo.UpdateStatuses))
	}
	call := d.mediaRepo.UpdateStatuses[0]
	if call.Next != domain.MediaStatusFailed {
		t.Fatalf("media must be marked FAILED, call = %+v", call)
	}
}

func TestCompleteUpload_OutboxCreateErrorTolerated(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.outbox.CreateErr = errors.New("ddb down")
	out, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if err != nil {
		t.Fatalf("Execute must tolerate outbox failure: %v", err)
	}
	if out.Status != domain.MediaStatusUploaded {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(d.outbox.Created) != 0 {
		t.Fatal("outbox event must not be recorded on create failure")
	}
}

func TestCompleteUpload_StatusUpdateError(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.mediaRepo.UpdateStatusErr = errors.New("conditional check failed")
	_, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if err == nil || !strings.Contains(err.Error(), "media state transition UPLOADING->UPLOADED") {
		t.Fatalf("err = %v, want media state transition error", err)
	}
}

func TestCompleteUpload_QuotaUsageReadErrorTolerated(t *testing.T) {
	d := newUploadDeps(t)
	f := seedCompleteFixture(t, d)
	d.quotaRepo.GetErr = errors.New("ddb down")
	out, err := d.newCompleteUC().Execute(d.ctx, f.input())
	if err != nil {
		t.Fatalf("Execute must tolerate quota usage read failure: %v", err)
	}
	if out.Status != domain.MediaStatusUploaded {
		t.Fatalf("unexpected output: %+v", out)
	}
}
