package domain

import "testing"

type fixedClassifier struct {
	mediaType MediaType
	match     string
}

func (c *fixedClassifier) MediaType() MediaType { return c.mediaType }
func (c *fixedClassifier) Matches(contentType string) bool {
	return contentType == c.match
}

func TestImageClassifier(t *testing.T) {
	c := &ImageClassifier{}
	if got := c.MediaType(); got != MediaTypeImage {
		t.Errorf("MediaType() = %q, want %q", got, MediaTypeImage)
	}
	cases := map[string]bool{
		"image/png":  true,
		"image/":     true,
		"video/mp4":  false,
		"text/plain": false,
		"":           false,
	}
	for ct, want := range cases {
		if got := c.Matches(ct); got != want {
			t.Errorf("ImageClassifier.Matches(%q) = %v, want %v", ct, got, want)
		}
	}
}

func TestVideoClassifier(t *testing.T) {
	c := &VideoClassifier{}
	if got := c.MediaType(); got != MediaTypeVideo {
		t.Errorf("MediaType() = %q, want %q", got, MediaTypeVideo)
	}
	cases := map[string]bool{
		"video/webm": true,
		"image/png":  false,
		"":           false,
	}
	for ct, want := range cases {
		if got := c.Matches(ct); got != want {
			t.Errorf("VideoClassifier.Matches(%q) = %v, want %v", ct, got, want)
		}
	}
}

func TestDocumentClassifier(t *testing.T) {
	c := NewDocumentClassifier()
	if got := c.MediaType(); got != MediaTypeDocument {
		t.Errorf("MediaType() = %q, want %q", got, MediaTypeDocument)
	}
	cases := map[string]bool{
		"application/pdf":    true,
		"text/plain":         true,
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"application/octet-stream": false,
		"image/png":                false,
		"":                         false,
	}
	for ct, want := range cases {
		if got := c.Matches(ct); got != want {
			t.Errorf("DocumentClassifier.Matches(%q) = %v, want %v", ct, got, want)
		}
	}
}

func TestMediaTypeClassifierRegistry_Classify(t *testing.T) {
	r := NewMediaTypeClassifierRegistry()
	if got := r.Classify("image/png"); got != MediaTypeOther {
		t.Errorf("empty registry Classify() = %q, want %q", got, MediaTypeOther)
	}

	r.Register(&ImageClassifier{})
	r.Register(&VideoClassifier{})
	r.Register(NewDocumentClassifier())

	cases := map[string]MediaType{
		"image/jpeg":      MediaTypeImage,
		"video/mp4":       MediaTypeVideo,
		"application/pdf": MediaTypeDocument,
		"text/plain":      MediaTypeDocument,
		"image/webp":      MediaTypeImage,
		"audio/mpeg":      MediaTypeOther,
		"":                MediaTypeOther,
	}
	for ct, want := range cases {
		if got := r.Classify(ct); got != want {
			t.Errorf("Classify(%q) = %q, want %q", ct, got, want)
		}
	}
}

func TestMediaTypeClassifierRegistry_FirstMatchWins(t *testing.T) {
	r := NewMediaTypeClassifierRegistry()
	r.Register(&fixedClassifier{mediaType: MediaTypeVideo, match: "application/special"})
	r.Register(&fixedClassifier{mediaType: MediaTypeImage, match: "application/special"})

	if got := r.Classify("application/special"); got != MediaTypeVideo {
		t.Errorf("Classify() = %q, want first registered classifier (%q)", got, MediaTypeVideo)
	}
}

func TestBuildMediaTypeClassifierRegistry(t *testing.T) {
	r := BuildMediaTypeClassifierRegistry()
	cases := map[string]MediaType{
		"image/png":        MediaTypeImage,
		"video/x-matroska": MediaTypeVideo,
		"application/pdf":  MediaTypeDocument,
		"application/zip":  MediaTypeOther,
	}
	for ct, want := range cases {
		if got := r.Classify(ct); got != want {
			t.Errorf("Classify(%q) = %q, want %q", ct, got, want)
		}
	}
}

func TestDeriveMediaType(t *testing.T) {
	cases := map[string]MediaType{
		"image/gif":         MediaTypeImage,
		"video/quicktime":   MediaTypeVideo,
		"text/plain":        MediaTypeDocument,
		"application/x-foo": MediaTypeOther,
	}
	for ct, want := range cases {
		if got := DeriveMediaType(ct); got != want {
			t.Errorf("DeriveMediaType(%q) = %q, want %q", ct, got, want)
		}
	}
}
