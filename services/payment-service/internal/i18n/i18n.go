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
	"en": {
		// Server / System
		"server.healthy":        "Payment service is healthy",
		"server.running":        "Payment service is running",
		"server.stopped":        "Payment service stopped",
		"server.shutdown":       "Shutting down Payment service",
		"server.method_not_allowed": "Method not allowed",

		// Validation & Commons
		"validation.id_required":          "Payment ID is required",
		"validation.delivery_id_required": "Delivery ID is required",
		"validation.user_id_required":     "User ID is required",
		"validation.amount_invalid":       "Amount must be greater than zero",
		"validation.currency_invalid":     "Invalid currency code",
		"validation.idempotency_required": "Idempotency key is required",

		// Payment Lifecycle
		"payment.created":     "Payment created successfully",
		"payment.authorized":  "Payment authorized successfully",
		"payment.captured":    "Payment captured successfully",
		"payment.cancelled":   "Payment authorization cancelled successfully",
		"payment.refunded":    "Payment refunded successfully",
		"payment.found":       "Payment retrieved successfully",
		"payment.not_found":   "Payment not found",
		"payment.list_found":  "Payments retrieved successfully",

		// Statuses
		"status.pending":    "Pending",
		"status.authorized": "Authorized",
		"status.captured":   "Captured",
		"status.cancelled":  "Cancelled",
		"status.failed":     "Failed",
		"status.refunded":   "Refunded",

		// Errors
		"error.internal":          "An internal server error occurred",
		"error.unauthorized":      "Unauthorized access",
		"error.forbidden":         "Forbidden action",
		"error.card_declined":     "Payment card declined by issuer",
		"error.timeout":           "Payment gateway timeout — reconciliation pending",
		"error.concurrency":       "Concurrent modification detected, please retry",
		"error.invalid_state":     "Invalid payment status transition",
		"error.cannot_refund":     "Payment amount cannot be refunded",
	},
	"ar": {
		// Server / System
		"server.healthy":        "خدمة المدفوعات تعمل بصحة جيدة",
		"server.running":        "خدمة المدفوعات قيد التشغيل بنجاح",
		"server.stopped":        "تم إيقاف خدمة المدفوعات",
		"server.shutdown":       "جاري إيقاف خدمة المدفوعات",
		"server.method_not_allowed": "طريقة الطلب غير مسموح بها",

		// Validation & Commons
		"validation.id_required":          "معرف عملية الدفع مطلوب",
		"validation.delivery_id_required": "معرف طلب التوصيل مطلوب",
		"validation.user_id_required":     "معرف المستخدم مطلوب",
		"validation.amount_invalid":       "يجب أن تكون قيمة المبلغ أكبر من الصفر",
		"validation.currency_invalid":     "رمز العملة غير صالح",
		"validation.idempotency_required": "مفتاح منع التكرار مطلوب",

		// Payment Lifecycle
		"payment.created":     "تم إنشاء عملية الدفع بنجاح",
		"payment.authorized":  "تم تفويض عملية الدفع وحجز المبلغ بنجاح",
		"payment.captured":    "تم تحصيل وتأكيد مبلغ الدفع بنجاح",
		"payment.cancelled":   "تم إلغاء حجز الدفع بنجاح",
		"payment.refunded":    "تم استرداد المبلغ بنجاح",
		"payment.found":       "تم العثور على تفاصيل عملية الدفع بنجاح",
		"payment.not_found":   "عملية الدفع غير موجودة",
		"payment.list_found":  "تم جلب قائمة المدفوعات بنجاح",

		// Statuses
		"status.pending":    "قيد الانتظار",
		"status.authorized": "مفوض / محجوز",
		"status.captured":   "مقبوض / مكتمل",
		"status.cancelled":  "ملغى",
		"status.failed":     "فشلت العملية",
		"status.refunded":   "مسترد",

		// Errors
		"error.internal":          "حدث خطأ داخلي في الخادم",
		"error.unauthorized":      "غير مصرح بالوصول",
		"error.forbidden":         "غير مسموح بهذا الإجراء",
		"error.card_declined":     "تم رفض البطاقة الائتمانية من جهة الإصدار",
		"error.timeout":           "انتهت مهلة بوابة الدفع — جاري تسوية العملية تلقائياً",
		"error.concurrency":       "حدث تعارض في التعديل المتزامن، يرجى إعادة المحاولة",
		"error.invalid_state":     "انتقال حالة الدفع غير صالح",
		"error.cannot_refund":     "لا يمكن استرداد هذا المبلغ",
	},
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
