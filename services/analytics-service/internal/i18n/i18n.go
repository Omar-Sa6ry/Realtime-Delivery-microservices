package i18n

import (
	"context"
	"fmt"
	"strings"
)

type contextKey string

const (
	LangKey     contextKey = "lang"
	DefaultLang            = "en"
)

var messages = map[string]map[string]string{
	"en": enMessages,
	"ar": arMessages,
}

func NormalizeLang(tag string) string {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if strings.HasPrefix(tag, "ar") {
		return "ar"
	}
	return DefaultLang
}

func T(lang, key string, args ...interface{}) string {
	lang = NormalizeLang(lang)
	dict, exists := messages[lang]
	if !exists {
		dict = messages[DefaultLang]
	}

	msg, found := dict[key]
	if !found {
		// Fallback to English
		if fallbackDict, ok := messages[DefaultLang]; ok {
			msg = fallbackDict[key]
		}
		if msg == "" {
			msg = key
		}
	}

	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

func FromContext(ctx context.Context) string {
	if ctx == nil {
		return DefaultLang
	}
	if val := ctx.Value(LangKey); val != nil {
		if s, ok := val.(string); ok && s != "" {
			return NormalizeLang(s)
		}
	}
	return DefaultLang
}

func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, LangKey, NormalizeLang(lang))
}
