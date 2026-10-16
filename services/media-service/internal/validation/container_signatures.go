package validation

type ContainerSignatureValidator interface {
	ContentTypes() []string
	ValidateSignature(header []byte) bool
}

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

type WebPValidator struct{}

func (v *WebPValidator) ContentTypes() []string {
	return []string{"image/webp"}
}

func (v *WebPValidator) ValidateSignature(header []byte) bool {
	return hasPrefixAt(header, []byte("RIFF"), 0) && hasPrefixAt(header, []byte("WEBP"), 8)
}

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

type WebMValidator struct{}

func (v *WebMValidator) ContentTypes() []string {
	return []string{"video/webm"}
}

func (v *WebMValidator) ValidateSignature(header []byte) bool {
	return hasPrefixAt(header, []byte{0x1A, 0x45, 0xDF, 0xA3}, 0)
}

func BuildContainerSignatureRegistry() *ContainerSignatureRegistry {
	registry := NewContainerSignatureRegistry()
	registry.Register(&WebPValidator{})
	registry.Register(&MP4Validator{})
	registry.Register(&AVIValidator{})
	registry.Register(&WebMValidator{})
	return registry
}

var DefaultContainerSignatureRegistry = BuildContainerSignatureRegistry()

func isContainerType(contentType string) bool {
	return DefaultContainerSignatureRegistry.IsContainerType(contentType)
}

func matchesContainerSignature(contentType string, header []byte) bool {
	validator, ok := DefaultContainerSignatureRegistry.Get(contentType)
	if !ok {
		return false
	}
	return validator.ValidateSignature(header)
}