package validation

import (
	"context"
	"fmt"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// UploadRequest carries all client-provided parameters for CreateUploadSession.
type UploadRequest struct {
	FileName    string
	ContentType string
	Size        int64
	Checksum    string // optional SHA-256 hex
}

// Validator orchestrates all pre-upload validation checks.
// It is the single entry point for upload validation — callers do not invoke individual validators.
type Validator struct {
	fileTypeValidator *FileTypeValidator
	magicBytes        *MagicBytesValidator
	checksumValidator *ChecksumValidator
	maxFileSize       int64
	minFileSize       int64
}

// NewValidator creates a Validator with all sub-validators wired in.
func NewValidator(
	allowedTypes map[string]struct{},
	storage ports.ObjectStorage,
	maxFileSize int64,
) *Validator {
	return &Validator{
		fileTypeValidator: NewFileTypeValidator(allowedTypes),
		magicBytes:        NewMagicBytesValidator(storage),
		checksumValidator: NewChecksumValidator(storage),
		maxFileSize:       maxFileSize,
		minFileSize:       1, // zero-byte files are rejected
	}
}

// ValidateUploadRequest validates the client's upload request before creating a session.
// This is server-side validation — client-side validation is just a UX optimisation.
func (v *Validator) ValidateUploadRequest(req UploadRequest) error {
	// 1. File size bounds
	if req.Size <= 0 {
		return domain.ErrFileTooSmall
	}
	if req.Size > v.maxFileSize {
		return fmt.Errorf("%w: declared size %d bytes exceeds maximum %d bytes", domain.ErrFileTooLarge, req.Size, v.maxFileSize)
	}

	// 2. Content type whitelist
	if err := v.fileTypeValidator.ValidateContentType(req.ContentType); err != nil {
		return fmt.Errorf("%w: %s", domain.ErrInvalidFileType, err.Error())
	}

	// 3. Extension matches content type
	if err := v.fileTypeValidator.ValidateExtension(req.FileName, req.ContentType); err != nil {
		return fmt.Errorf("%w: %s", domain.ErrInvalidFileType, err.Error())
	}

	return nil
}

// ValidatePostUpload validates an uploaded S3 object after it has been assembled.
// Runs magic bytes check and optional checksum verification.
func (v *Validator) ValidatePostUpload(ctx context.Context, objectKey, contentType, expectedChecksum string) error {
	// 1. Magic bytes — verify the file signature matches the declared type
	if err := v.magicBytes.ValidateObject(ctx, objectKey, contentType); err != nil {
		return err
	}

	// 2. Checksum — verify integrity if the client provided one
	if err := v.checksumValidator.Validate(ctx, objectKey, expectedChecksum); err != nil {
		return err
	}

	return nil
}
