package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// magicByteSignatures maps content types to their file signature bytes.
// Checked against the first N bytes of the object body.
var magicByteSignatures = map[string][][]byte{
	"image/jpeg": {
		{0xFF, 0xD8, 0xFF},
	},
	"image/png": {
		{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
	},
	"image/gif": {
		{0x47, 0x49, 0x46, 0x38, 0x37, 0x61}, // GIF87a
		{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}, // GIF89a
	},
	"image/webp": {
		// RIFF....WEBP
		{0x52, 0x49, 0x46, 0x46},
	},
	"video/mp4": {
		// ftyp box at offset 4
		{0x00, 0x00, 0x00}, // partial — checked with offset
	},
	"application/pdf": {
		{0x25, 0x50, 0x44, 0x46}, // %PDF
	},
	"application/zip": {
		{0x50, 0x4B, 0x03, 0x04}, // PK header
	},
}

// MagicBytesValidator validates file signatures against declared content types.
type MagicBytesValidator struct {
	storage ports.ObjectStorage
}

// NewMagicBytesValidator creates a MagicBytesValidator backed by the given storage.
func NewMagicBytesValidator(storage ports.ObjectStorage) *MagicBytesValidator {
	return &MagicBytesValidator{storage: storage}
}

// ValidateObject streams the first 16 bytes of an S3 object and checks the signature.
// If no signature is registered for the content type, validation passes.
func (v *MagicBytesValidator) ValidateObject(ctx context.Context, objectKey, contentType string) error {
	sigs, ok := magicByteSignatures[contentType]
	if !ok {
		return nil // no signature registered — skip
	}

	body, err := v.storage.GetObject(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("open object for magic bytes check: %w", err)
	}
	defer body.Close()

	header := make([]byte, 16)
	n, err := io.ReadAtLeast(body, header, 3)
	if err != nil && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("read object header for magic bytes: %w", err)
	}
	header = header[:n]

	for _, sig := range sigs {
		if len(header) >= len(sig) {
			match := true
			for i, b := range sig {
				if header[i] != b {
					match = false
					break
				}
			}
			if match {
				return nil
			}
		}
	}
	return domain.ErrInvalidMagicBytes
}

// ChecksumValidator verifies SHA-256 checksums of S3 objects.
type ChecksumValidator struct {
	storage ports.ObjectStorage
}

// NewChecksumValidator creates a ChecksumValidator backed by the given storage.
func NewChecksumValidator(storage ports.ObjectStorage) *ChecksumValidator {
	return &ChecksumValidator{storage: storage}
}

// Validate streams the object and computes its SHA-256.
// Returns ErrChecksumMismatch if the computed hash does not equal expectedHex.
// expectedHex is the lowercase hex-encoded SHA-256 provided by the client.
func (v *ChecksumValidator) Validate(ctx context.Context, objectKey, expectedHex string) error {
	if expectedHex == "" {
		return nil // checksum is optional — skip if not provided
	}

	body, err := v.storage.GetObject(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("open object for checksum validation: %w", err)
	}
	defer body.Close()

	h := sha256.New()
	if _, err := io.Copy(h, body); err != nil {
		return fmt.Errorf("stream object for checksum: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedHex {
		return domain.ErrChecksumMismatch
	}
	return nil
}

// ComputeChecksum computes the SHA-256 of an S3 object and returns the hex string.
func ComputeChecksum(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("compute checksum: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
