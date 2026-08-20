package validation

import (
	"fmt"
	"path/filepath"
	"strings"
)

// allowedExtensions maps content types to their permitted file extensions.
var allowedExtensions = map[string][]string{
	"image/jpeg":                    {".jpg", ".jpeg"},
	"image/png":                     {".png"},
	"image/gif":                     {".gif"},
	"image/webp":                    {".webp"},
	"video/mp4":                     {".mp4"},
	"video/quicktime":               {".mov"},
	"video/x-msvideo":               {".avi"},
	"video/x-matroska":              {".mkv"},
	"video/webm":                    {".webm"},
	"application/pdf":               {".pdf"},
	"application/zip":               {".zip"},
	"application/x-zip-compressed":  {".zip"},
	"text/plain":                    {".txt"},
	"application/octet-stream":      {}, // allowed but no extension constraint
}

type FileTypeValidator struct {
	allowedTypes map[string]struct{}
}

func NewFileTypeValidator(allowedTypes map[string]struct{}) *FileTypeValidator {
	return &FileTypeValidator{allowedTypes: allowedTypes}
}

func (v *FileTypeValidator) ValidateContentType(contentType string) error {
	ct := strings.ToLower(strings.Split(contentType, ";")[0])
	ct = strings.TrimSpace(ct)

	if _, ok := v.allowedTypes[ct]; !ok {
		return fmt.Errorf("content type %q is not permitted", ct)
	}
	return nil
}

func (v *FileTypeValidator) ValidateExtension(fileName, contentType string) error {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		return fmt.Errorf("file %q has no extension", fileName)
	}

	ct := strings.ToLower(strings.Split(contentType, ";")[0])
	allowed, ok := allowedExtensions[ct]
	if !ok {
		// Content type not in our extension map — content type validator caught this first.
		return nil
	}
	if len(allowed) == 0 {
		// No extension constraint for this type.
		return nil
	}

	for _, a := range allowed {
		if ext == a {
			return nil
		}
	}
	return fmt.Errorf("extension %q is not valid for content type %q", ext, ct)
}

func NormalizeFileName(name string) string {
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if len(name) > 255 {
		ext := filepath.Ext(name)
		name = name[:255-len(ext)] + ext
	}
	return name
}
