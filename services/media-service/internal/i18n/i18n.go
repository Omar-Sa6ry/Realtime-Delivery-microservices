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
		"server.healthy":                   "healthy",
		"media.fetched":                    "Media fetched successfully",
		"media.list_fetched":               "Media list fetched successfully",
		"media.not_found":                  "Media not found",
		"media.deleted":                    "Media deleted successfully",
		"media.upload_status_fetched":      "Upload status fetched successfully",
		"media.download_url_generated":     "Download URL generated successfully",
		"media.quota_fetched":              "User quota fetched successfully",
		"media.upload_session_created":     "Upload session created successfully",
		"media.upload_completed":           "Upload completed successfully",
		"media.upload_aborted":             "Upload aborted successfully",
		"media.presigned_renewed":          "Presigned URLs renewed successfully",
		"dlq.stats_fetched":                "DLQ stats fetched successfully",
		"dlq.replayed":                     "DLQ messages replayed successfully",
		"error.unauthorized":               "authentication required: missing x-user-id header",
		"error.forbidden_admin":            "forbidden: admin privileges required",
		"error.topic_required":             "topic is required",
		"error.upload_id_required":         "uploadId is required",
		"error.invalid_input":              "invalid input",
		"error.internal":                   "internal server error",
	},
	"ar": {
		"server.healthy":                   "تعمل بصحة جيدة",
		"media.fetched":                    "تم استرجاع الوسائط بنجاح",
		"media.list_fetched":               "تم استرجاع قائمة الوسائط بنجاح",
		"media.not_found":                  "الملف غير موجود",
		"media.deleted":                    "تم حذف الوسائط بنجاح",
		"media.upload_status_fetched":      "تم استرجاع حالة الرفع بنجاح",
		"media.download_url_generated":     "تم إنشاء رابط التحميل بنجاح",
		"media.quota_fetched":              "تم استرجاع السعة التخزينية بنجاح",
		"media.upload_session_created":     "تم إنشاء جلسة الرفع بنجاح",
		"media.upload_completed":           "اكتمل الرفع بنجاح",
		"media.upload_aborted":             "تم إلغاء عملية الرفع بنجاح",
		"media.presigned_renewed":          "تم تجديد روابط الرفع بنجاح",
		"dlq.stats_fetched":                "تم استرجاع إحصائيات الرسائل بنجاح",
		"dlq.replayed":                     "تمت إعادة إرسال الرسائل بنجاح",
		"error.unauthorized":               "تسجيل الدخول مطلوب: لم يتم العثور على المعرف",
		"error.forbidden_admin":            "غير مصرح: هذه العملية خاصة بالمسؤولين فقط",
		"error.topic_required":             "اسم القناة مطلوب",
		"error.upload_id_required":         "معرف الرفع مطلوب",
		"error.invalid_input":              "بيانات غير صالحة",
		"error.internal":                   "حدث خطأ داخلي في الخادم",
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
