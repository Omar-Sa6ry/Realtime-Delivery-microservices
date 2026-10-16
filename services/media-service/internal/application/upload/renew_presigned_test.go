package upload

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

const renewDefaultExpiry = 15 * time.Minute

func seedRenewSession(t *testing.T, d *uploadDeps) {
	t.Helper()
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
}

func lastPresignedOp(t *testing.T, d *uploadDeps) testutil.PresignedCall {
	t.Helper()
	calls := d.storage.PresignedCalls()
	if len(calls) == 0 {
		t.Fatal("no presigned calls recorded")
	}
	return calls[len(calls)-1]
}

func TestRenewPresigned_SuccessUsesDefaultExpiry(t *testing.T) {
	d := newUploadDeps(t)
	seedRenewSession(t, d)

	out, err := d.newRenewUC(renewDefaultExpiry).Execute(d.ctx, RenewPresignedInput{
		UserID: "user-1", UploadID: "upload-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.UploadID != "upload-1" || out.S3UploadID != "s3-upload-upload-1" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if out.TotalParts != 2 || out.PartSize != 5<<20 || len(out.PresignedParts) != 2 {
		t.Fatalf("unexpected parts: %+v", out)
	}
	call := lastPresignedOp(t, d)
	if call.Op != "GeneratePresignedParts" || call.ObjectKey != "users/user-1/media/media-1/original/file.bin" {
		t.Fatalf("unexpected presigned call: %+v", call)
	}
	if call.Expiry != renewDefaultExpiry || call.TotalParts != 2 {
		t.Fatalf("unexpected presigned expiry/parts: %+v", call)
	}
	newExpiry, ok := d.uploadRepo.ExpiryUpdates["upload-1"]
	if !ok {
		t.Fatal("session expiry must be updated")
	}
	if newExpiry.Before(time.Now().Add(renewDefaultExpiry-time.Minute)) ||
		newExpiry.After(time.Now().Add(renewDefaultExpiry+time.Minute)) {
		t.Fatalf("new expiry %v outside expected window", newExpiry)
	}
	if d.cache.UploadProgress["upload-1"] != 0 {
		t.Fatalf("upload progress cache = %v", d.cache.UploadProgress)
	}
}

func TestRenewPresigned_CustomExpiry(t *testing.T) {
	d := newUploadDeps(t)
	seedRenewSession(t, d)

	out, err := d.newRenewUC(renewDefaultExpiry).Execute(d.ctx, RenewPresignedInput{
		UserID: "user-1", UploadID: "upload-1", ExpirySeconds: 60,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if call := lastPresignedOp(t, d); call.Expiry != 60*time.Second {
		t.Fatalf("expiry = %v, want 60s", call.Expiry)
	}
	if out.ExpiresAt.After(time.Now().Add(61 * time.Second)) {
		t.Fatalf("ExpiresAt %v too far in the future", out.ExpiresAt)
	}
}

func TestRenewPresigned_ExpiryClamping(t *testing.T) {
	tests := []struct {
		name         string
		expirySecond int
		wantExpiry   time.Duration
	}{
		{"zero uses default", 0, renewDefaultExpiry},
		{"negative uses default", -10, renewDefaultExpiry},
		{"above cap uses default", 90000, renewDefaultExpiry},
		{"at cap accepted", 86400, 86400 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newUploadDeps(t)
			seedRenewSession(t, d)
			_, err := d.newRenewUC(renewDefaultExpiry).Execute(d.ctx, RenewPresignedInput{
				UserID: "user-1", UploadID: "upload-1", ExpirySeconds: tt.expirySecond,
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if call := lastPresignedOp(t, d); call.Expiry != tt.wantExpiry {
				t.Fatalf("expiry = %v, want %v", call.Expiry, tt.wantExpiry)
			}
		})
	}
}

func TestRenewPresigned_SessionNotFound(t *testing.T) {
	d := newUploadDeps(t)
	_, err := d.newRenewUC(renewDefaultExpiry).Execute(d.ctx, RenewPresignedInput{
		UserID: "user-1", UploadID: "missing",
	})
	if !errors.Is(err, domain.ErrUploadSessionNotFound) {
		t.Fatalf("err = %v, want ErrUploadSessionNotFound", err)
	}
}

func TestRenewPresigned_Unauthorized(t *testing.T) {
	d := newUploadDeps(t)
	seedRenewSession(t, d)
	_, err := d.newRenewUC(renewDefaultExpiry).Execute(d.ctx, RenewPresignedInput{
		UserID: "user-2", UploadID: "upload-1",
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestRenewPresigned_WrongStatus(t *testing.T) {
	d := newUploadDeps(t)
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	session.Status = domain.UploadStatusCompleted
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	_, err := d.newRenewUC(renewDefaultExpiry).Execute(d.ctx, RenewPresignedInput{
		UserID: "user-1", UploadID: "upload-1",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot renew presigned URLs") {
		t.Fatalf("err = %v, want cannot renew presigned URLs error", err)
	}
}

func TestRenewPresigned_Expired(t *testing.T) {
	d := newUploadDeps(t)
	session := testutil.NewUploadSession("upload-1", "media-1", "user-1")
	session.ExpiresAt = time.Now().Add(-time.Minute)
	if err := d.uploadRepo.Create(d.ctx, session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	_, err := d.newRenewUC(renewDefaultExpiry).Execute(d.ctx, RenewPresignedInput{
		UserID: "user-1", UploadID: "upload-1",
	})
	if !errors.Is(err, domain.ErrUploadExpired) {
		t.Fatalf("err = %v, want ErrUploadExpired", err)
	}
}

func TestRenewPresigned_PresignError(t *testing.T) {
	d := newUploadDeps(t)
	seedRenewSession(t, d)
	d.storage.SetError("GeneratePresignedParts", errors.New("s3 down"))
	_, err := d.newRenewUC(renewDefaultExpiry).Execute(d.ctx, RenewPresignedInput{
		UserID: "user-1", UploadID: "upload-1",
	})
	if !errors.Is(err, domain.ErrS3OperationFailed) {
		t.Fatalf("err = %v, want ErrS3OperationFailed", err)
	}
	if len(d.uploadRepo.ExpiryUpdates) != 0 {
		t.Fatal("expiry must not be updated on presign failure")
	}
}

func TestRenewPresigned_UpdateExpiryError(t *testing.T) {
	d := newUploadDeps(t)
	seedRenewSession(t, d)
	d.uploadRepo.UpdateExpiryErr = errors.New("ddb down")
	_, err := d.newRenewUC(renewDefaultExpiry).Execute(d.ctx, RenewPresignedInput{
		UserID: "user-1", UploadID: "upload-1",
	})
	if err == nil || !strings.Contains(err.Error(), "update session expiry") {
		t.Fatalf("err = %v, want update session expiry error", err)
	}
}
