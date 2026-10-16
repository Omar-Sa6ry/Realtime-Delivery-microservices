package upload

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
)

func TestUploadFailureReason(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, "none"},
		{"quota exceeded", domain.ErrQuotaExceeded, "quota_exceeded"},
		{"concurrent uploads", domain.ErrConcurrentUploadsExceeded, "concurrent_uploads_exceeded"},
		{"rate limited", domain.ErrRateLimitExceeded, "rate_limited"},
		{"idempotency conflict", domain.ErrIdempotencyConflict, "idempotency_conflict"},
		{"s3 failure", domain.ErrS3OperationFailed, "s3_error"},
		{"checksum mismatch", domain.ErrChecksumMismatch, "checksum_mismatch"},
		{"expired", domain.ErrUploadExpired, "expired"},
		{"unauthorized", domain.ErrUnauthorized, "unauthorized"},
		{"invalid file type", domain.ErrInvalidFileType, "validation"},
		{"file too large", domain.ErrFileTooLarge, "validation"},
		{"file too small", domain.ErrFileTooSmall, "validation"},
		{"parts missing", domain.ErrUploadPartsMissing, "validation"},
		{"invalid magic bytes", domain.ErrInvalidMagicBytes, "validation"},
		{"quarantined", domain.ErrMediaQuarantined, "validation"},
		{"wrapped domain error", fmt.Errorf("context: %w", domain.ErrQuotaExceeded), "quota_exceeded"},
		{"validation prefix fallback", errors.New("validation: something odd"), "validation"},
		{"unknown", errors.New("boom"), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uploadFailureReason(tt.err); got != tt.want {
				t.Fatalf("uploadFailureReason(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
