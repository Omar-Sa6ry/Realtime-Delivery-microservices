package validation

// ContainerSignatureValidator defines the strategy interface for validating container format signatures.
type ContainerSignatureValidator interface {
	// ContentTypes returns the content types this validator handles.
	ContentTypes() []string

	// ValidateSignature validates the container signature in the header.
	ValidateSignature(header []byte) bool
}

// ContainerSignatureRegistry holds all registered container signature validators.
type ContainerSignatureRegistry struct {
	validators map[string]ContainerSignatureValidator
}

func NewContainerSignatureRegistry() *ContainerSignatureRegistry {
	return &ContainerSignatureRegistry{
		validators: make(map[string]ContainerSignatureValidator),
	}
}

func (r *ContainerSignatureRegistry) Register(v ContainerSignatureValidator) {
	for _, ct := range v.ContentTypes() {
		r.validators[ct] = v
	}
}

func (r *ContainerSignatureRegistry) Get(contentType string) (ContainerSignatureValidator, bool) {
	v, ok := r.validators[contentType]
	return v, ok
}

func (r *ContainerSignatureRegistry) IsContainerType(contentType string) bool {
	_, ok := r.validators[contentType]
	return ok
}

// WebPValidator validates WebP container signatures (RIFF + WEBP at offset 8).
type WebPValidator struct{}

func (v *WebPValidator) ContentTypes() []string {
	return []string{"image/webp"}
}

func (v *WebPValidator) ValidateSignature(header []byte) bool {
	return hasPrefixAt(header, []byte("RIFF"), 0) && hasPrefixAt(header, []byte("WEBP"), 8)
}

// MP4Validator validates MP4/QuickTime container signatures (ftyp at offset 4).
type MP4Validator struct{}

func (v *MP4Validator) ContentTypes() []string {
	return []string{"video/mp4", "video/quicktime"}
}

func (v *MP4Validator) ValidateSignature(header []byte) bool {
	return hasPrefixAt(header, []byte("ftyp"), 4)
}

// AVIValidator validates AVI container signatures (RIFF + AVI at offset 8).
type AVIValidator struct{}

func (v *AVIValidator) ContentTypes() []string {
	return []string{"video/x-msvideo"}
}

func (v *AVIValidator) ValidateSignature(header []byte) bool {
	return hasPrefixAt(header, []byte("RIFF"), 0) && hasPrefixAt(header, []byte("AVI "), 8)
}

// WebMValidator validates WebM container signatures (EBML at offset 0).
type WebMValidator struct{}

func (v *WebMValidator) ContentTypes() []string {
	return []string{"video/webm"}
}

func (v *WebMValidator) ValidateSignature(header []byte) bool {
	return hasPrefixAt(header, []byte{0x1A, 0x45, 0xDF, 0xA3}, 0)
}

// BuildContainerSignatureRegistry creates and populates the container signature registry.
func BuildContainerSignatureRegistry() *ContainerSignatureRegistry {
	registry := NewContainerSignatureRegistry()
	registry.Register(&WebPValidator{})
	registry.Register(&MP4Validator{})
	registry.Register(&AVIValidator{})
	registry.Register(&WebMValidator{})
	return registry
}

// DefaultContainerSignatureRegistry is the default registry for container signature validation.
var DefaultContainerSignatureRegistry = BuildContainerSignatureRegistry()

// isContainerType checks if a content type is a container format that requires signature validation.
func isContainerType(contentType string) bool {
	return DefaultContainerSignatureRegistry.IsContainerType(contentType)
}

// matchesContainerSignature validates the container signature for the given content type.
func matchesContainerSignature(contentType string, header []byte) bool {
	validator, ok := DefaultContainerSignatureRegistry.Get(contentType)
	if !ok {
		return false
	}
	return validator.ValidateSignature(header)
}