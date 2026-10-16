package domain

import "errors"

var (
	ErrMediaNotFound = errors.New("media not found")
	ErrUploadSessionNotFound = errors.New("upload session not found")
	ErrUnauthorized = errors.New("unauthorized: resource ownership mismatch")
	ErrInvalidFileType = errors.New("invalid file type: not in allowed list")
	ErrFileTooLarge = errors.New("file size exceeds maximum allowed limit")
	ErrFileTooSmall = errors.New("file size must be greater than zero")
	ErrQuotaExceeded = errors.New("storage quota exceeded")
	ErrConcurrentUploadsExceeded = errors.New("maximum concurrent uploads exceeded")
	ErrUploadExpired = errors.New("upload session has expired")
	ErrInvalidStateTransition = errors.New("invalid media state transition")
	ErrIdempotencyConflict = errors.New("idempotent request already processed or in progress")
	ErrChecksumMismatch = errors.New("checksum mismatch: file integrity check failed")
	ErrS3OperationFailed = errors.New("s3 operation failed")
	ErrMediaQuarantined = errors.New("media is quarantined due to security scan failure")
	ErrUploadPartsMissing = errors.New("one or more upload parts are missing")
	ErrInvalidMagicBytes = errors.New("file magic bytes do not match declared content type")
	ErrRateLimitExceeded = errors.New("rate limit exceeded: too many requests")
	ErrJobNotFound = errors.New("processing job not found")
)
