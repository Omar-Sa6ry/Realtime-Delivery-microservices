package i18n

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"english", "en", "en"},
		{"ENGLISH", "ENGLISH", "en"},
		{"english US", "en-US", "en"},
		{"arabic", "ar", "ar"},
		{"ARABIC", "AR", "ar"},
		{"arabic SA", "ar-SA", "ar"},
		{"arabic EG", "ar-EG", "ar"},
		{"french", "fr", "en"},
		{"spanish", "es", "en"},
		{"empty", "", "en"},
		{"whitespace", "  en  ", "en"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, NormalizeLang(tc.input))
		})
	}
}

func TestT_English(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		args     []any
		expected string
	}{
		{"server.healthy", "server.healthy", nil, "Analytics service is healthy"},
		{"error.internal", "error.internal", nil, "An internal server error occurred"},
		{"error.unauthorized", "error.unauthorized", nil, "Unauthorized: admin role required"},
		{"platform.overview.found", "platform.overview.found", nil, "Platform overview retrieved successfully"},
		{"with args", "server.healthy", []any{"arg1"}, "Analytics service is healthy%!(EXTRA string=arg1)"}, // No format specifiers in message
		{"unknown key", "unknown.key", nil, "unknown.key"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := T("en", tc.key, tc.args...)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestT_Arabic(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"server.healthy", "server.healthy", "خدمة التحليلات تعمل بصحة جيدة"},
		{"error.internal", "error.internal", "حدث خطأ داخلي في الخادم"},
		{"error.unauthorized", "error.unauthorized", "غير مصرح: يتطلب صلاحيات مسؤول النظام (Admin)"},
		{"platform.overview.found", "platform.overview.found", "تم جلب نظرة عامة على المنصة بنجاح"},
		{"unknown key", "unknown.key", "unknown.key"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := T("ar", tc.key)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestT_FallbackToEnglish(t *testing.T) {
	// Key exists in English but not Arabic
	result := T("ar", "server.healthy")
	assert.Equal(t, "خدمة التحليلات تعمل بصحة جيدة", result)

	// Key doesn't exist in either
	result = T("ar", "completely.unknown.key")
	assert.Equal(t, "completely.unknown.key", result)
}

func TestFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{"with lang in context", context.WithValue(context.Background(), LangKey, "ar"), "ar"},
		{"with lang in context uppercase", context.WithValue(context.Background(), LangKey, "EN"), "en"},
		{"without lang in context", context.Background(), "en"},
		{"nil context", nil, "en"},
		{"empty string in context", context.WithValue(context.Background(), LangKey, ""), "en"},
		{"invalid type in context", context.WithValue(context.Background(), LangKey, 123), "en"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FromContext(tc.ctx))
		})
	}
}

func TestWithLanguage(t *testing.T) {
	ctx := WithLanguage(context.Background(), "ar")
	assert.Equal(t, "ar", FromContext(ctx))

	ctx = WithLanguage(context.Background(), "EN")
	assert.Equal(t, "en", FromContext(ctx))
}

func TestContextKey(t *testing.T) {
	key := LangKey
	assert.Equal(t, "lang", string(key))
}