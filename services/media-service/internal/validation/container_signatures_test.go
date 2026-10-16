package validation

import (
	"bytes"
	"testing"
)

func TestContainerSignatureRegistry_RegisterAndGet(t *testing.T) {
	r := NewContainerSignatureRegistry()
	if _, ok := r.Get("video/mp4"); ok {
		t.Error("empty registry must not resolve any content type")
	}
	if r.IsContainerType("video/mp4") {
		t.Error("empty registry must not report container types")
	}

	mp4 := &MP4Validator{}
	r.Register(mp4)

	for _, ct := range mp4.ContentTypes() {
		got, ok := r.Get(ct)
		if !ok {
			t.Fatalf("Get(%q) not found after Register", ct)
		}
		if got != mp4 {
			t.Errorf("Get(%q) = %v, want the registered validator", ct, got)
		}
		if !r.IsContainerType(ct) {
			t.Errorf("IsContainerType(%q) = false, want true", ct)
		}
	}
	if _, ok := r.Get("video/webm"); ok {
		t.Error("unregistered content type must not resolve")
	}
	if r.IsContainerType("image/png") {
		t.Error("image/png must not be a container type")
	}
}

func TestBuildContainerSignatureRegistry(t *testing.T) {
	r := BuildContainerSignatureRegistry()
	containerTypes := []string{
		"image/webp", "video/mp4", "video/quicktime", "video/x-msvideo", "video/webm",
	}
	for _, ct := range containerTypes {
		if !r.IsContainerType(ct) {
			t.Errorf("IsContainerType(%q) = false, want true", ct)
		}
		if _, ok := r.Get(ct); !ok {
			t.Errorf("Get(%q) not found in built registry", ct)
		}
	}
	for _, ct := range []string{"image/png", "application/pdf", ""} {
		if r.IsContainerType(ct) {
			t.Errorf("IsContainerType(%q) = true, want false", ct)
		}
	}
}

func TestIsContainerType_DefaultRegistry(t *testing.T) {
	if !isContainerType("image/webp") {
		t.Error("isContainerType(image/webp) = false, want true")
	}
	if isContainerType("text/plain") {
		t.Error("isContainerType(text/plain) = true, want false")
	}
}

func TestMatchesContainerSignature(t *testing.T) {
	webpData := make([]byte, 0, 12)
	webpData = append(webpData, []byte("RIFF")...)
	webpData = append(webpData, 0, 0, 0, 0)
	webpData = append(webpData, []byte("WEBP")...)

	if !matchesContainerSignature("image/webp", webpData) {
		t.Error("expected valid WebP header to match")
	}
	if matchesContainerSignature("image/webp", []byte("RIFF----NOTW")) {
		t.Error("expected mismatched WebP header to fail")
	}
	if matchesContainerSignature("image/png", webpData) {
		t.Error("non-container content type must not match")
	}
}

func riffHeader(suffix string) []byte {
	h := make([]byte, 0, 12)
	h = append(h, []byte("RIFF")...)
	h = append(h, 0, 0, 0, 0)
	return append(h, []byte(suffix)...)
}

func TestWebPValidator_ValidateSignature(t *testing.T) {
	v := &WebPValidator{}
	if got := v.ContentTypes(); len(got) != 1 || got[0] != "image/webp" {
		t.Errorf("ContentTypes() = %v, want [image/webp]", got)
	}
	cases := []struct {
		name   string
		header []byte
		want   bool
	}{
		{"valid", riffHeader("WEBP"), true},
		{"wrong_fourcc", riffHeader("WEBx"), false},
		{"avi_header", riffHeader("AVI "), false},
		{"short_riff_only", []byte("RIFF"), false},
		{"too_short_for_webp_tag", riffHeader("WEB"), false},
		{"empty", nil, false},
		{"garbage", []byte("not a riff header!"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := v.ValidateSignature(tc.header); got != tc.want {
				t.Errorf("ValidateSignature() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMP4Validator_ValidateSignature(t *testing.T) {
	v := &MP4Validator{}
	if got := v.ContentTypes(); len(got) != 2 || got[0] != "video/mp4" || got[1] != "video/quicktime" {
		t.Errorf("ContentTypes() = %v, want [video/mp4 video/quicktime]", got)
	}
	cases := []struct {
		name   string
		header []byte
		want   bool
	}{
		{"valid_isom", []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, true},
		{"valid_minimal", []byte{0, 0, 0, 0, 'f', 't', 'y', 'p'}, true},
		{"ftyp_wrong_offset", []byte{'f', 't', 'y', 'p', 0, 0, 0, 0}, false},
		{"short_header", []byte{0, 0, 0, 0}, false},
		{"empty", nil, false},
		{"garbage", bytes.Repeat([]byte{0xAB}, 16), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := v.ValidateSignature(tc.header); got != tc.want {
				t.Errorf("ValidateSignature() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAVIValidator_ValidateSignature(t *testing.T) {
	v := &AVIValidator{}
	if got := v.ContentTypes(); len(got) != 1 || got[0] != "video/x-msvideo" {
		t.Errorf("ContentTypes() = %v, want [video/x-msvideo]", got)
	}
	cases := []struct {
		name   string
		header []byte
		want   bool
	}{
		{"valid", riffHeader("AVI "), true},
		{"webp_header", riffHeader("WEBP"), false},
		{"short", riffHeader("AV"), false},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := v.ValidateSignature(tc.header); got != tc.want {
				t.Errorf("ValidateSignature() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWebMValidator_ValidateSignature(t *testing.T) {
	v := &WebMValidator{}
	if got := v.ContentTypes(); len(got) != 1 || got[0] != "video/webm" {
		t.Errorf("ContentTypes() = %v, want [video/webm]", got)
	}
	cases := []struct {
		name   string
		header []byte
		want   bool
	}{
		{"valid", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x01, 0x00}, true},
		{"minimal", []byte{0x1A, 0x45, 0xDF, 0xA3}, true},
		{"last_byte_mismatch", []byte{0x1A, 0x45, 0xDF, 0x00}, false},
		{"too_short", []byte{0x1A, 0x45}, false},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := v.ValidateSignature(tc.header); got != tc.want {
				t.Errorf("ValidateSignature() = %v, want %v", got, tc.want)
			}
		})
	}
}
