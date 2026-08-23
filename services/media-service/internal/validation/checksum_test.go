package validation

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

type validationStorage struct {
	data []byte
}

func (s validationStorage) CreateMultipartUpload(context.Context, string, string) (string, error) {
	return "", nil
}
func (s validationStorage) GeneratePresignedParts(context.Context, string, string, int, int64, time.Duration) ([]ports.PresignedPart, error) {
	return nil, nil
}
func (s validationStorage) GeneratePresignedGET(context.Context, string, time.Duration) (string, error) {
	return "", nil
}
func (s validationStorage) GeneratePresignedGETWithRange(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
func (s validationStorage) CompleteMultipartUpload(context.Context, string, string, []domain.UploadPart) error {
	return nil
}
func (s validationStorage) AbortMultipartUpload(context.Context, string, string) error { return nil }
func (s validationStorage) HeadObject(context.Context, string) (*ports.ObjectInfo, error) {
	return nil, nil
}
func (s validationStorage) DeleteObject(context.Context, string) error    { return nil }
func (s validationStorage) DeleteObjects(context.Context, []string) error { return nil }
func (s validationStorage) GetObject(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}
func (s validationStorage) GetObjectWithRange(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}
func (s validationStorage) PutObject(context.Context, string, string, []byte) error { return nil }
func (s validationStorage) ListObjectsWithPrefix(context.Context, string) ([]string, error) {
	return nil, nil
}

func TestMagicBytesValidatorRejectsForgedWebP(t *testing.T) {
	validator := NewMagicBytesValidator(validationStorage{data: []byte("RIFFfake-data")})
	if err := validator.ValidateObject(context.Background(), "x", "image/webp"); err != domain.ErrInvalidMagicBytes {
		t.Fatalf("expected invalid WebP signature, got %v", err)
	}
}

func TestMagicBytesValidatorAcceptsWebPContainer(t *testing.T) {
	data := append([]byte("RIFF"), []byte{0, 0, 0, 0}...)
	data = append(data, []byte("WEBP")...)
	validator := NewMagicBytesValidator(validationStorage{data: data})
	if err := validator.ValidateObject(context.Background(), "x", "image/webp"); err != nil {
		t.Fatalf("expected valid WebP signature, got %v", err)
	}
}

func TestMagicBytesValidatorRejectsForgedMP4(t *testing.T) {
	validator := NewMagicBytesValidator(validationStorage{data: []byte{0, 0, 0, 0, 'n', 'o', 'p', 'e'}})
	if err := validator.ValidateObject(context.Background(), "x", "video/mp4"); err != domain.ErrInvalidMagicBytes {
		t.Fatalf("expected invalid MP4 signature, got %v", err)
	}
}

func TestMagicBytesValidatorAcceptsMP4Ftyp(t *testing.T) {
	validator := NewMagicBytesValidator(validationStorage{data: []byte{0, 0, 0, 0, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}})
	if err := validator.ValidateObject(context.Background(), "x", "video/mp4"); err != nil {
		t.Fatalf("expected valid MP4 signature, got %v", err)
	}
}
