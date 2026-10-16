package domain

import (
	"strings"
)

type MediaTypeClassifier interface {
	MediaType() MediaType
	Matches(contentType string) bool
}

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

type ImageClassifier struct{}

func (c *ImageClassifier) MediaType() MediaType {
	return MediaTypeImage
}

func (c *ImageClassifier) Matches(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

type VideoClassifier struct{}

func (c *VideoClassifier) MediaType() MediaType {
	return MediaTypeVideo
}

func (c *VideoClassifier) Matches(contentType string) bool {
	return strings.HasPrefix(contentType, "video/")
}

type DocumentClassifier struct {
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

func BuildMediaTypeClassifierRegistry() *MediaTypeClassifierRegistry {
	registry := NewMediaTypeClassifierRegistry()
	registry.Register(&ImageClassifier{})
	registry.Register(&VideoClassifier{})
	registry.Register(NewDocumentClassifier())
	return registry
}

var DefaultMediaTypeRegistry = BuildMediaTypeClassifierRegistry()

func DeriveMediaType(contentType string) MediaType {
	return DefaultMediaTypeRegistry.Classify(contentType)
}