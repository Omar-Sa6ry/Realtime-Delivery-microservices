package validation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

var _ ports.ObjectStorage = stubStorage{}

type errReader struct{ err error }

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }
func (r *errReader) Close() error             { return nil }

// stubStorage serves a fixed body from GetObject, with optional error
// injection for the open and read paths.
type stubStorage struct {
	data    []byte
	getErr  error
	readErr error
}

func (s stubStorage) GetObject(context.Context, string) (io.ReadCloser, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.readErr != nil {
		return &errReader{err: s.readErr}, nil
	}
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

func (s stubStorage) CreateMultipartUpload(context.Context, string, string) (string, error) {
	return "", nil
}
func (s stubStorage) GeneratePresignedParts(context.Context, string, string, int, int64, time.Duration) ([]ports.PresignedPart, error) {
	return nil, nil
}
func (s stubStorage) GeneratePresignedGET(context.Context, string, time.Duration) (string, error) {
	return "", nil
}
func (s stubStorage) GeneratePresignedGETWithRange(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
func (s stubStorage) CompleteMultipartUpload(context.Context, string, string, []domain.UploadPart) error {
	return nil
}
func (s stubStorage) AbortMultipartUpload(context.Context, string, string) error { return nil }
func (s stubStorage) HeadObject(context.Context, string) (*ports.ObjectInfo, error) {
	return nil, nil
}
func (s stubStorage) DeleteObject(context.Context, string) error    { return nil }
func (s stubStorage) DeleteObjects(context.Context, []string) error { return nil }
func (s stubStorage) GetObjectWithRange(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}
func (s stubStorage) PutObject(context.Context, string, string, []byte) error { return nil }
func (s stubStorage) ListObjectsWithPrefix(context.Context, string) ([]string, error) {
	return nil, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func newTestValidator(maxFileSize int64) *Validator {
	allowed := map[string]struct{}{
		"image/png":       {},
		"image/jpeg":      {},
		"application/pdf": {},
		"text/plain":      {},
	}
	return NewValidator(allowed, stubStorage{}, maxFileSize)
}

func TestValidator_ValidateUploadRequest(t *testing.T) {
	v := newTestValidator(1024)
	cases := []struct {
		name    string
		req     UploadRequest
		wantErr error
	}{
		{
			name:    "zero_size",
			req:     UploadRequest{FileName: "photo.png", ContentType: "image/png", Size: 0},
			wantErr: domain.ErrFileTooSmall,
		},
		{
			name:    "negative_size",
			req:     UploadRequest{FileName: "photo.png", ContentType: "image/png", Size: -5},
			wantErr: domain.ErrFileTooSmall,
		},
		{
			name:    "oversize",
			req:     UploadRequest{FileName: "photo.png", ContentType: "image/png", Size: 1025},
			wantErr: domain.ErrFileTooLarge,
		},
		{
			name:    "disallowed_content_type",
			req:     UploadRequest{FileName: "page.html", ContentType: "text/html", Size: 10},
			wantErr: domain.ErrInvalidFileType,
		},
		{
			name:    "extension_mismatch",
			req:     UploadRequest{FileName: "photo.txt", ContentType: "image/png", Size: 10},
			wantErr: domain.ErrInvalidFileType,
		},
		{
			name:    "missing_extension",
			req:     UploadRequest{FileName: "README", ContentType: "image/png", Size: 10},
			wantErr: domain.ErrInvalidFileType,
		},
		{
			name:    "happy_png",
			req:     UploadRequest{FileName: "photo.png", ContentType: "image/png", Size: 100},
			wantErr: nil,
		},
		{
			name:    "happy_jpeg_uppercase_extension",
			req:     UploadRequest{FileName: "photo.JPG", ContentType: "image/jpeg", Size: 100},
			wantErr: nil,
		},
		{
			name:    "happy_pdf_with_content_type_params",
			req:     UploadRequest{FileName: "doc.pdf", ContentType: "application/pdf; charset=binary", Size: 100},
			wantErr: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateUploadRequest(tc.req)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateUploadRequest() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidateUploadRequest() = %v, want wrapped %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidator_ValidateUploadRequest_OversizeMessage(t *testing.T) {
	v := newTestValidator(1024)
	err := v.ValidateUploadRequest(UploadRequest{FileName: "big.png", ContentType: "image/png", Size: 2048})
	if err == nil || !strings.Contains(err.Error(), "declared size") {
		t.Fatalf("expected oversize error to mention declared size, got %v", err)
	}
}

func pngData() []byte {
	b := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	return append(b, []byte("body")...)
}

func TestValidator_ValidatePostUpload(t *testing.T) {
	data := pngData()
	goodChecksum := sha256Hex(data)

	cases := []struct {
		name     string
		storage  stubStorage
		checksum string
		wantErr  error
	}{
		{
			name:     "magic_ok_no_checksum",
			storage:  stubStorage{data: data},
			checksum: "",
			wantErr:  nil,
		},
		{
			name:     "magic_ok_checksum_match",
			storage:  stubStorage{data: data},
			checksum: goodChecksum,
			wantErr:  nil,
		},
		{
			name:     "magic_ok_checksum_mismatch",
			storage:  stubStorage{data: data},
			checksum: strings.Repeat("0", 64),
			wantErr:  domain.ErrChecksumMismatch,
		},
		{
			name:     "magic_mismatch_short_circuits_checksum",
			storage:  stubStorage{data: []byte("not-a-png")},
			checksum: strings.Repeat("0", 64),
			wantErr:  domain.ErrInvalidMagicBytes,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := NewValidator(
				map[string]struct{}{"image/png": {}},
				tc.storage,
				1024,
			)
			err := v.ValidatePostUpload(context.Background(), "obj-key", "image/png", tc.checksum)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidatePostUpload() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidatePostUpload() = %v, want wrapped %v", err, tc.wantErr)
			}
		})
	}
}
