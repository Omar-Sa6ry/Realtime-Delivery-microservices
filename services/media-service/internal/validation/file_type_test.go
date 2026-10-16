package validation

import (
	"strings"
	"testing"
)

func newFileTypeValidator() *FileTypeValidator {
	return NewFileTypeValidator(map[string]struct{}{
		"image/png":                {},
		"image/jpeg":               {},
		"image/avif":               {},
		"application/pdf":          {},
		"application/zip":          {},
		"application/octet-stream": {},
		"text/plain":               {},
	})
}

func TestFileTypeValidator_ValidateContentType(t *testing.T) {
	v := newFileTypeValidator()
	cases := []struct {
		name        string
		contentType string
		wantErr     bool
	}{
		{"plain_allowed", "image/png", false},
		{"uppercase", "IMAGE/PNG", false},
		{"with_params", "image/png; charset=binary", false},
		{"with_whitespace", "  image/jpeg  ", false},
		{"disallowed", "text/html", true},
		{"empty", "", true},
		{"octet_stream", "application/octet-stream", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateContentType(tc.contentType)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateContentType(%q) = nil, want error", tc.contentType)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateContentType(%q) = %v, want nil", tc.contentType, err)
			}
		})
	}
}

func TestFileTypeValidator_ValidateExtension(t *testing.T) {
	v := newFileTypeValidator()
	cases := []struct {
		name        string
		fileName    string
		contentType string
		wantErr     bool
	}{
		{"png_match", "photo.png", "image/png", false},
		{"uppercase_extension", "PHOTO.PNG", "image/png", false},
		{"jpeg_jpg_alias", "photo.jpg", "image/jpeg", false},
		{"jpeg_jpeg_alias", "photo.jpeg", "image/jpeg", false},
		{"zip_match", "archive.zip", "application/zip", false},
		{"mismatch", "photo.txt", "image/png", true},
		{"no_extension", "README", "image/png", true},
		{"content_type_without_extension_map_entry", "file.bin", "image/avif", false},
		{"octet_stream_no_constraint", "file.bin", "application/octet-stream", false},
		{"params_stripped", "doc.pdf", "application/pdf; charset=binary", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateExtension(tc.fileName, tc.contentType)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateExtension(%q, %q) = nil, want error", tc.fileName, tc.contentType)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateExtension(%q, %q) = %v, want nil", tc.fileName, tc.contentType, err)
			}
		})
	}
}

func TestNormalizeFileName(t *testing.T) {
	longName := strings.Repeat("a", 300) + ".png"
	longNoExt := strings.Repeat("b", 300)

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "photo.jpg", "photo.jpg"},
		{"path_traversal_dotdot", "../secret.png", "secret.png"},
		{"nested_relative", "dir/sub/file.txt", "file.txt"},
		{"absolute_posix", "/etc/passwd", "passwd"},
		{"absolute_windows", `C:\Users\me\photo.png`, "photo.png"},
		{"windows_traversal", `..\..\evil.exe`, "evil.exe"},
		{"control_chars_stripped", "photo\x00\x1f\x7f.jpg", "photo.jpg"},
		{"empty", "", "."},
		{"dot", ".", "."},
		{"long_name_truncated_with_ext", longName, longName[:251] + ".png"},
		{"long_name_truncated_no_ext", longNoExt, longNoExt[:255]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeFileName(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeFileName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeFileName_TruncatedLength(t *testing.T) {
	in := strings.Repeat("x", 400) + ".jpeg"
	got := NormalizeFileName(in)
	if len(got) != 255 {
		t.Fatalf("len(NormalizeFileName()) = %d, want 255", len(got))
	}
	if !strings.HasSuffix(got, ".jpeg") {
		t.Errorf("truncated name lost extension: %q", got)
	}
}
