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
		"server.healthy":                      "Search service is healthy",
		"search.deliveries_fetched":           "Deliveries fetched successfully",
		"search.drivers_fetched":              "Drivers fetched successfully",
		"search.media_fetched":                "Media fetched successfully",
		"search.users_fetched":                "Users fetched successfully",
		"search.autocomplete_fetched":         "Autocomplete suggestions fetched successfully",
		"search.nearby_deliveries_fetched":    "Nearby deliveries fetched successfully",
		"search.nearby_drivers_fetched":       "Nearby drivers fetched successfully",
		"search.reindex_started":              "Reindex job started successfully",
		"error.unauthorized":                  "authentication required: missing x-user-id header",
		"error.forbidden_admin":               "forbidden: admin access required",
		"error.invalid_input":                 "invalid input",
		"error.internal":                      "internal server error",
	},
	"ar": {
		"server.healthy":                      "خدمة البحث تعمل بصحة جيدة",
		"search.deliveries_fetched":           "تم استرجاع الشحنات بنجاح",
		"search.drivers_fetched":              "تم استرجاع السائقين بنجاح",
		"search.media_fetched":                "تم استرجاع الوسائط بنجاح",
		"search.users_fetched":                "تم استرجاع المستخدمين بنجاح",
		"search.autocomplete_fetched":         "تم استرجاع اقتراحات الإكمال التلقائي بنجاح",
		"search.nearby_deliveries_fetched":    "تم استرجاع الشحنات القريبة بنجاح",
		"search.nearby_drivers_fetched":       "تم استرجاع السائقين القريبين بنجاح",
		"search.reindex_started":              "بدأت عملية إعادة الفهرسة بنجاح",
		"error.unauthorized":                  "تسجيل الدخول مطلوب: لم يتم العثور على المعرف",
		"error.forbidden_admin":               "غير مصرح: الوصول مقتصر على المشرفين فقط",
		"error.invalid_input":                 "بيانات غير صالحة",
		"error.internal":                      "حدث خطأ داخلي في الخادم",
	},
}

func NormalizeLang(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if strings.HasPrefix(lang, "ar") {
		return "ar"
	}
	return DefaultLang
}

func Translate(lang, key string) string {
	normalized := NormalizeLang(lang)
	if langMap, ok := messages[normalized]; ok {
		if msg, ok := langMap[key]; ok {
			return msg
		}
	}
	if langMap, ok := messages[DefaultLang]; ok {
		if msg, ok := langMap[key]; ok {
			return msg
		}
	}
	return key
}

func T(lang, key string) string {
	return Translate(lang, key)
}

func FromContext(ctx context.Context) string {
	if ctx == nil {
		return DefaultLang
	}
	if v, ok := ctx.Value(LangKey).(string); ok && v != "" {
		return NormalizeLang(v)
	}
	return DefaultLang
}

func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, LangKey, NormalizeLang(lang))
}
