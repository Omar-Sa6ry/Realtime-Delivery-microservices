package domain

import (
	"strings"
)

// MediaTypeClassifier defines the strategy interface for classifying media types from MIME content types.
type MediaTypeClassifier interface {
	// MediaType returns the media type this classifier handles.
	MediaType() MediaType

	// Matches returns true if this classifier matches the given content type.
	Matches(contentType string) bool
}

// MediaTypeClassifierRegistry holds all registered classifiers.
type MediaTypeClassifierRegistry struct {
	classifiers []MediaTypeClassifier
}

func NewMediaTypeClassifierRegistry() *MediaTypeClassifierRegistry {
	return &MediaTypeClassifierRegistry{}
}

func (r *MediaTypeClassifierRegistry) Register(c MediaTypeClassifier) {
	r.classifiers = append(r.classifiers, c)
}

func (r *MediaTypeClassifierRegistry) Classify(contentType string) MediaType {
	for _, c := range r.classifiers {
		if c.Matches(contentType) {
			return c.MediaType()
		}
	}
	return MediaTypeOther
}

// ImageClassifier classifies image/* content types.
type ImageClassifier struct{}

func (c *ImageClassifier) MediaType() MediaType {
	return MediaTypeImage
}

func (c *ImageClassifier) Matches(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

// VideoClassifier classifies video/* content types.
type VideoClassifier struct{}

func (c *VideoClassifier) MediaType() MediaType {
	return MediaTypeVideo
}

func (c *VideoClassifier) Matches(contentType string) bool {
	return strings.HasPrefix(contentType, "video/")
}

// DocumentClassifier classifies document content types.
type DocumentClassifier struct {
	// MIME types that are considered documents
	documentTypes map[string]bool
}

func NewDocumentClassifier() *DocumentClassifier {
	return &DocumentClassifier{
		documentTypes: map[string]bool{
			"application/pdf":                                        true,
			"text/plain":                                             true,
			"application/msword":                                     true,
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		},
	}
}

func (c *DocumentClassifier) MediaType() MediaType {
	return MediaTypeDocument
}

func (c *DocumentClassifier) Matches(contentType string) bool {
	return c.documentTypes[contentType]
}

// BuildMediaTypeClassifierRegistry creates and populates the classifier registry.
func BuildMediaTypeClassifierRegistry() *MediaTypeClassifierRegistry {
	registry := NewMediaTypeClassifierRegistry()
	registry.Register(&ImageClassifier{})
	registry.Register(&VideoClassifier{})
	registry.Register(NewDocumentClassifier())
	return registry
}

// DefaultMediaTypeRegistry is the default registry for media type classification.
var DefaultMediaTypeRegistry = BuildMediaTypeClassifierRegistry()

// DeriveMediaType infers the MediaType from the MIME content type using the default registry.
func DeriveMediaType(contentType string) MediaType {
	return DefaultMediaTypeRegistry.Classify(contentType)
}