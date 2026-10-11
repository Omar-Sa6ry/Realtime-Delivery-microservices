package upload

import (
	"errors"
	"strings"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
)

func uploadFailureReason(err error) string {
	if err == nil {
		return "none"
	}

	switch {
	case errors.Is(err, domain.ErrQuotaExceeded):
		return "quota_exceeded"
	case errors.Is(err, domain.ErrConcurrentUploadsExceeded):
		return "concurrent_uploads_exceeded"
	case errors.Is(err, domain.ErrRateLimitExceeded):
		return "rate_limited"
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, domain.ErrS3OperationFailed):
		return "s3_error"
	case errors.Is(err, domain.ErrChecksumMismatch):
		return "checksum_mismatch"
	case errors.Is(err, domain.ErrUploadExpired):
		return "expired"
	case errors.Is(err, domain.ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, domain.ErrInvalidFileType),
		errors.Is(err, domain.ErrFileTooLarge),
		errors.Is(err, domain.ErrFileTooSmall),
		errors.Is(err, domain.ErrUploadPartsMissing),
		errors.Is(err, domain.ErrInvalidMagicBytes),
		errors.Is(err, domain.ErrMediaQuarantined):
		return "validation"
	}

	if strings.Contains(err.Error(), "validation:") {
		return "validation"
	}
	return "unknown"
}
