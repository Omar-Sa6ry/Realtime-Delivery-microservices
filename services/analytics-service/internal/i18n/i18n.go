package i18n

import (
	"context"
	"strings"
)

type contextKey string

const (
	LangKey     contextKey = "lang"
	DefaultLang            = "en"
)

var messages = map[string]map[string]string{
	"en": {
		"server.healthy":  "Analytics service is healthy",
		"server.running":  "Analytics service is running",
		"analytics.found": "Analytics retrieved successfully",
		"error.internal":  "An internal server error occurred",
	},
	"ar": {
		"server.healthy":  "خدمة التحليلات تعمل بصحة جيدة",
		"server.running":  "خدمة التحليلات قيد التشغيل بنجاح",
		"analytics.found": "تم جلب التحليلات بنجاح",
		"error.internal":  "حدث خطأ داخلي في الخادم",
	},
}

func NormalizeLang(tag string) string {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if strings.HasPrefix(tag, "ar") {
		return "ar"
	}
	return DefaultLang
}

func T(lang, key string) string {
	lang = NormalizeLang(lang)
	if dict, ok := messages[lang]; ok {
		if msg, found := dict[key]; found {
			return msg
		}
	}
	if msg, found := messages[DefaultLang][key]; found {
		return msg
	}
	return key
}

func FromContext(ctx context.Context) string {
	if ctx == nil {
		return DefaultLang
	}
	if v := ctx.Value(LangKey); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return NormalizeLang(s)
		}
	}
	return DefaultLang
}

func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, LangKey, NormalizeLang(lang))
}
