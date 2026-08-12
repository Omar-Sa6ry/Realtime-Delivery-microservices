package domain

import "errors"

// Sentinel domain errors — wrap these with fmt.Errorf for context.
var (
	// ErrMediaNotFound is returned when a media record does not exist.
	ErrMediaNotFound = errors.New("media not found")

	// ErrUploadSessionNotFound is returned when an upload session does not exist.
	ErrUploadSessionNotFound = errors.New("upload session not found")

	// ErrUnauthorized is returned when the caller does not own the resource.
	ErrUnauthorized = errors.New("unauthorized: resource ownership mismatch")

	// ErrInvalidFileType is returned when the MIME type or extension is not allowed.
	ErrInvalidFileType = errors.New("invalid file type: not in allowed list")

	// ErrFileTooLarge is returned when the declared file size exceeds the maximum.
	ErrFileTooLarge = errors.New("file size exceeds maximum allowed limit")

	// ErrFileTooSmall is returned when the declared file size is zero or negative.
	ErrFileTooSmall = errors.New("file size must be greater than zero")

	// ErrQuotaExceeded is returned when the user has exceeded their storage quota.
	ErrQuotaExceeded = errors.New("storage quota exceeded")

	// ErrConcurrentUploadsExceeded is returned when the user has too many active uploads.
	ErrConcurrentUploadsExceeded = errors.New("maximum concurrent uploads exceeded")

	// ErrUploadExpired is returned when the upload session TTL has elapsed.
	ErrUploadExpired = errors.New("upload session has expired")

	// ErrInvalidStateTransition is returned when a state machine transition is not allowed.
	ErrInvalidStateTransition = errors.New("invalid media state transition")

	// ErrIdempotencyConflict is returned when an identical request is already being processed.
	ErrIdempotencyConflict = errors.New("idempotent request already processed or in progress")

	// ErrChecksumMismatch is returned when the provided checksum does not match the actual object.
	ErrChecksumMismatch = errors.New("checksum mismatch: file integrity check failed")

	// ErrS3OperationFailed is returned when an S3 operation fails after all retries.
	ErrS3OperationFailed = errors.New("s3 operation failed")

	// ErrMediaQuarantined is returned when attempting to access a quarantined media item.
	ErrMediaQuarantined = errors.New("media is quarantined due to security scan failure")

	// ErrUploadPartsMissing is returned when CompleteUpload is called with missing parts.
	ErrUploadPartsMissing = errors.New("one or more upload parts are missing")

	// ErrInvalidMagicBytes is returned when the file signature does not match the declared type.
	ErrInvalidMagicBytes = errors.New("file magic bytes do not match declared content type")

	// ErrRateLimitExceeded is returned when the caller has hit a rate limit.
	ErrRateLimitExceeded = errors.New("rate limit exceeded: too many requests")

	// ErrJobNotFound is returned when a processing job does not exist.
	ErrJobNotFound = errors.New("processing job not found")
)
