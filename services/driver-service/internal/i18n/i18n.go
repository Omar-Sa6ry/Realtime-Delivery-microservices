package i18n

import (
	"context"
	"strings"
)

type contextKey string

const (
	LangKey contextKey = "lang"
	DefaultLang       = "en"
)

// Supported languages: "en" and "ar"
var messages = map[string]map[string]string{
	"en": {
		// Server / System
		"server.healthy":        "healthy",
		"server.running":        "Driver service is running",
		"server.stopped":        "Driver service stopped",
		"server.shutdown":       "Shutting down Driver service",
		"server.method_not_allowed": "Method not allowed",

		// Common / Validation
		"validation.id_required":          "id is required",
		"validation.driver_id_required":   "driverId is required",
		"validation.delivery_id_required": "deliveryId is required",
		"validation.invalid_id_type":      "idType must be 'driver' or 'user'",
		"validation.idempotency_required": "idempotency key is required",

		// Driver Operations
		"driver.found":                    "Driver found successfully",
		"driver.not_found":                "Driver not found",
		"driver.profile_found":            "Driver profile found successfully",
		"driver.no_profile_for_user":      "No driver profile for this user",
		"driver.registered":               "Driver registered successfully",
		"driver.profile_updated":          "Driver profile updated successfully",
		"driver.online":                   "Driver went online successfully",
		"driver.offline":                  "Driver went offline successfully",
		"driver.suspended":                "Driver suspended successfully",
		"driver.activated":                "Driver activated successfully",
		"driver.not_available":            "Driver is not available for assignment",
		"driver.already_reserved":         "Driver is already reserved or busy",
		"driver.already_online":           "Driver is already online",
		"driver.already_offline":          "Driver is already offline",

		// Assignment Operations
		"assignment.offered":              "Assignment offered to driver",
		"assignment.accepted":             "Assignment accepted successfully",
		"assignment.rejected":             "Assignment rejected successfully",
		"assignment.reserved":             "Driver reserved successfully for delivery",
		"assignment.released":             "Driver released successfully",
		"assignment.not_found":            "Assignment not found",
		"assignment.expired":              "Assignment has expired",
		"assignment.already_handled":      "Assignment was already handled",
		"assignment.lock_conflict":        "Could not acquire lock for driver assignment",

		// Generic Errors
		"error.internal":                  "An internal server error occurred",
		"error.unauthorized":              "Unauthorized access",
		"error.forbidden":                 "Forbidden action",
	},
	"ar": {
		// Server / System
		"server.healthy":        "تعمل بصحة جيدة",
		"server.running":        "خدمة السائقين تعمل بنجاح",
		"server.stopped":        "تم إيقاف خدمة السائقين",
		"server.shutdown":       "جاري إيقاف خدمة السائقين",
		"server.method_not_allowed": "طريقة الطلب غير مسموح بها",

		// Common / Validation
		"validation.id_required":          "المعرف مطلوب",
		"validation.driver_id_required":   "معرف السائق مطلوب",
		"validation.delivery_id_required": "معرف الطلب مطلوب",
		"validation.invalid_id_type":      "يجب أن يكون نوع المعرف إما 'driver' أو 'user'",
		"validation.idempotency_required": "مفتاح منع التكرار مطلوب",

		// Driver Operations
		"driver.found":                    "تم العثور على السائق بنجاح",
		"driver.not_found":                "السائق غير موجود",
		"driver.profile_found":            "تم العثور على الملف الشخصي للسائق بنجاح",
		"driver.no_profile_for_user":      "لا يوجد ملف سائق لهذا المستخدم",
		"driver.registered":               "تم تسجيل السائق بنجاح",
		"driver.profile_updated":          "تم تحديث الملف الشخصي للسائق بنجاح",
		"driver.online":                   "تم تفعيل اتصال السائق بنجاح",
		"driver.offline":                  "تم قطع اتصال السائق بنجاح",
		"driver.suspended":                "تم تعليق حساب السائق بنجاح",
		"driver.activated":                "تم إعادة تنشيط حساب السائق بنجاح",
		"driver.not_available":            "السائق غير متاح لتلقي طلبات جديدة",
		"driver.already_reserved":         "السائق محجوز لطلب آخر بالفعل",
		"driver.already_online":           "السائق متصل بالفعل",
		"driver.already_offline":          "السائق غير متصل بالفعل",

		// Assignment Operations
		"assignment.offered":              "تم عرض التوصيل على السائق",
		"assignment.accepted":             "تم قبول التوصيل بنجاح",
		"assignment.rejected":             "تم رفض التوصيل بنجاح",
		"assignment.reserved":             "تم حجز السائق للتوصيل بنجاح",
		"assignment.released":             "تم تحرير السائق بنجاح",
		"assignment.not_found":            "لم يتم العثور على التوصيل أو الإسناد",
		"assignment.expired":              "انتهت صلاحية عرض التوصيل",
		"assignment.already_handled":      "تمت معالجة الإسناد مسبقاً",
		"assignment.lock_conflict":        "تعذر حجز القفل لإسناد السائق (تزامن عمليات)",

		// Generic Errors
		"error.internal":                  "حدث خطأ داخلي في الخادم",
		"error.unauthorized":              "غير مصرح بالدخول",
		"error.forbidden":                 "الإجراء غير مسموح به",
	},
}

// NormalizeLang normalizes the language code, defaulting to "en".
func NormalizeLang(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if strings.HasPrefix(lang, "ar") {
		return "ar"
	}
	return DefaultLang
}

// Translate returns the translated string for a given key in the specified language.
// Defaults to English ("en") if key or language is not found.
func Translate(lang, key string) string {
	normalized := NormalizeLang(lang)
	if langMap, ok := messages[normalized]; ok {
		if msg, ok := langMap[key]; ok {
			return msg
		}
	}
	// Fallback to English
	if langMap, ok := messages[DefaultLang]; ok {
		if msg, ok := langMap[key]; ok {
			return msg
		}
	}
	return key
}

// T is an alias for Translate
func T(lang, key string) string {
	return Translate(lang, key)
}

// FromContext extracts language from context or returns default "en"
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return DefaultLang
	}
	if v, ok := ctx.Value(LangKey).(string); ok && v != "" {
		return NormalizeLang(v)
	}
	return DefaultLang
}

// WithLanguage returns a context containing the specified language
func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, LangKey, NormalizeLang(lang))
}
