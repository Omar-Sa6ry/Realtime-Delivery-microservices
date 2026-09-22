package i18n

// Arabic messages (same pattern as payment-service).
var arMessages = map[string]string{
	// Server / System
	"server.healthy": "خدمة التحليلات تعمل بصحة جيدة",
	"server.running": "خدمة التحليلات قيد التشغيل بنجاح",

	// Queries
	"analytics.found":           "تم جلب التحليلات بنجاح",
	"platform.overview.found":   "تم جلب نظرة عامة على المنصة بنجاح",
	"delivery.analytics.found":  "تم جلب تحليلات التوصيل بنجاح",
	"driver.analytics.found":    "تم جلب تحليلات السائق بنجاح",
	"driver.top.found":          "تم جلب أفضل السائقين بنجاح",
	"payment.analytics.found":   "تم جلب تحليلات الدفع بنجاح",
	"raw.events.found":          "تم جلب الأحداث الخام بنجاح",
	"data.quality.issues.found": "تم جلب مشكلات جودة البيانات بنجاح",

	// Validation errors
	"error.range.from.missing": "حقل 'from' في نطاق الاستعلام مطلوب",
	"error.range.to.missing":   "حقل 'to' في نطاق الاستعلام مطلوب",
	"error.range.invalid":      "يجب أن يكون 'from' قبل 'to'",
	"error.severity.invalid":   "يجب أن تكون درجة الخطورة إحدى: ERROR أو WARNING أو INFO",
	"error.limit.invalid":      "يجب أن يكون الحد بين 1 و 100",

	// Generic errors
	"error.internal":     "حدث خطأ داخلي في الخادم",
	"error.unauthorized": "غير مصرح: يتطلب صلاحيات مسؤول النظام (Admin)",
	"error.not.found":    "المورد المطلوب غير موجود",
}

