package validation

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
)

func TestMagicBytesValidator_Signatures(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		data        []byte
		wantErr     bool
	}{
		{"jpeg_valid", "image/jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0, 0}, false},
		{"jpeg_invalid", "image/jpeg", []byte{0x89, 0x50, 0x4E, 0x47, 0, 0, 0, 0}, true},
		{"jpeg_prefix_only", "image/jpeg", []byte{0xFF, 0xD8, 0xFF}, false},
		{"png_valid", "image/png", pngData(), false},
		{"png_invalid", "image/png", []byte("not-a-png-file"), true},
		{"gif87a_valid", "image/gif", []byte("GIF87a...."), false},
		{"gif89a_valid", "image/gif", []byte("GIF89a...."), false},
		{"gif_invalid", "image/gif", []byte("GIF98a...."), true},
		{"pdf_valid", "application/pdf", []byte("%PDF-1.7 body"), false},
		{"pdf_invalid", "application/pdf", []byte("!PDF-1.7 body"), true},
		{"zip_valid", "application/zip", []byte{0x50, 0x4B, 0x03, 0x04, 0, 0, 0, 0}, false},
		{"zip_invalid_eocd_marker", "application/zip", []byte{0x50, 0x4B, 0x05, 0x06, 0, 0, 0, 0}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := NewMagicBytesValidator(stubStorage{data: tc.data})
			err := v.ValidateObject(context.Background(), "key", tc.contentType)
			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidMagicBytes) {
					t.Fatalf("ValidateObject() = %v, want ErrInvalidMagicBytes", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateObject() = %v, want nil", err)
			}
		})
	}
}

func TestMagicBytesValidator_UnknownContentTypeSkipped(t *testing.T) {
	cases := []string{"application/octet-stream", "text/plain", "application/x-zip-compressed", "audio/mpeg"}
	for _, ct := range cases {
		t.Run(ct, func(t *testing.T) {
			v := NewMagicBytesValidator(stubStorage{data: []byte("garbage data here")})
			if err := v.ValidateObject(context.Background(), "key", ct); err != nil {
				t.Fatalf("ValidateObject(%q) = %v, want nil for unregistered type", ct, err)
			}
		})
	}
}

func TestMagicBytesValidator_ContainerTypes(t *testing.T) {
	aviValid := riffHeader("AVI ")
	aviValid = append(aviValid, []byte("LIST....")...)
	webmValid := []byte{0x1A, 0x45, 0xDF, 0xA3, 0x01, 0x00}

	cases := []struct {
		name        string
		contentType string
		data        []byte
		wantErr     bool
	}{
		{"avi_valid", "video/x-msvideo", aviValid, false},
		{"avi_header_mismatch", "video/x-msvideo", riffHeader("WEBPextra!"), true},
		{"webm_valid", "video/webm", webmValid, false},
		{"webm_header_mismatch", "video/webm", []byte{0x1A, 0x45, 0xDF, 0x00}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := NewMagicBytesValidator(stubStorage{data: tc.data})
			err := v.ValidateObject(context.Background(), "key", tc.contentType)
			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidMagicBytes) {
					t.Fatalf("ValidateObject() = %v, want ErrInvalidMagicBytes", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateObject() = %v, want nil", err)
			}
		})
	}
}

func TestMagicBytesValidator_GetObjectError(t *testing.T) {
	v := NewMagicBytesValidator(stubStorage{getErr: errors.New("s3 down")})
	err := v.ValidateObject(context.Background(), "key", "image/png")
	if err == nil || !strings.Contains(err.Error(), "open object for magic bytes check") {
		t.Fatalf("expected wrapped GetObject error, got %v", err)
	}
}

func TestMagicBytesValidator_ReaderError(t *testing.T) {
	v := NewMagicBytesValidator(stubStorage{readErr: errors.New("connection reset")})
	err := v.ValidateObject(context.Background(), "key", "image/png")
	if err == nil || !strings.Contains(err.Error(), "read object header for magic bytes") {
		t.Fatalf("expected wrapped read error, got %v", err)
	}
	if errors.Is(err, domain.ErrInvalidMagicBytes) {
		t.Fatalf("read failure must not be reported as invalid magic bytes: %v", err)
	}
}

func TestMagicBytesValidator_BodyShorterThanThreeBytes(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		data        []byte
		wantErr     bool
	}{
		{"png_short_body", "image/png", []byte("ab"), true},
		{"text_short_body_skipped", "text/plain", []byte("ab"), false},
		{"gif_sig_longer_than_header", "image/gif", []byte("GIF"), true},
		{"zip_sig_longer_than_header", "application/zip", []byte{0x50, 0x4B}, true},
		{"one_byte_unknown_type", "application/octet-stream", []byte{0x00}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := NewMagicBytesValidator(stubStorage{data: tc.data})
			err := v.ValidateObject(context.Background(), "key", tc.contentType)
			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidMagicBytes) {
					t.Fatalf("ValidateObject() = %v, want ErrInvalidMagicBytes", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateObject() = %v, want nil", err)
			}
		})
	}
}

func TestChecksumValidator_EmptyChecksumSkipsStorage(t *testing.T) {
	v := NewChecksumValidator(stubStorage{getErr: errors.New("must not be consulted")})
	if err := v.Validate(context.Background(), "key", ""); err != nil {
		t.Fatalf("Validate() with empty checksum = %v, want nil", err)
	}
}

func TestChecksumValidator_MatchAndMismatch(t *testing.T) {
	body := []byte("hello world")
	v := NewChecksumValidator(stubStorage{data: body})

	if err := v.Validate(context.Background(), "key", sha256Hex(body)); err != nil {
		t.Fatalf("Validate() with matching checksum = %v, want nil", err)
	}
	if err := v.Validate(context.Background(), "key", strings.Repeat("ab", 32)); !errors.Is(err, domain.ErrChecksumMismatch) {
		t.Fatalf("Validate() with mismatching checksum = %v, want ErrChecksumMismatch", err)
	}
}

func TestChecksumValidator_GetObjectError(t *testing.T) {
	v := NewChecksumValidator(stubStorage{getErr: errors.New("s3 down")})
	err := v.Validate(context.Background(), "key", strings.Repeat("a", 64))
	if err == nil || !strings.Contains(err.Error(), "open object for checksum validation") {
		t.Fatalf("expected wrapped GetObject error, got %v", err)
	}
}

func TestChecksumValidator_ReaderError(t *testing.T) {
	v := NewChecksumValidator(stubStorage{readErr: errors.New("connection reset")})
	err := v.Validate(context.Background(), "key", strings.Repeat("a", 64))
	if err == nil || !strings.Contains(err.Error(), "stream object for checksum") {
		t.Fatalf("expected wrapped stream error, got %v", err)
	}
}

func TestComputeChecksum(t *testing.T) {
	body := []byte("hello world")
	got, err := ComputeChecksum(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("ComputeChecksum() error = %v", err)
	}
	if want := sha256Hex(body); got != want {
		t.Errorf("ComputeChecksum() = %q, want %q", got, want)
	}
}

func TestComputeChecksum_ReaderError(t *testing.T) {
	got, err := ComputeChecksum(&errReader{err: errors.New("boom")})
	if err == nil {
		t.Fatal("ComputeChecksum() error = nil, want reader error")
	}
	if got != "" {
		t.Errorf("ComputeChecksum() = %q, want empty string on error", got)
	}
	if !strings.Contains(err.Error(), "compute checksum") {
		t.Errorf("error %q does not mention compute checksum", err)
	}
}
