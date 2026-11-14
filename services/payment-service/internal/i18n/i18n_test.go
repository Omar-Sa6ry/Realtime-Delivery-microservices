package i18n_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/i18n"
)

func TestNormalizeLang(t *testing.T) {
	cases := map[string]string{
		"ar":       "ar",
		"AR":       "ar",
		"ar-EG":    "ar",
		"  ar  ":   "ar",
		"en":       "en",
		"fr":       "en",
		"":         "en",
		"english":  "en",
		"Arabic":   "ar",
		"\tAr-SA\n": "ar",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, i18n.NormalizeLang(in))
		})
	}
}

func TestTEnglishAndArabic(t *testing.T) {
	assert.Equal(t, "Payment service is healthy", i18n.T("en", "server.healthy"))
	assert.Equal(t, "خدمة المدفوعات تعمل بصحة جيدة", i18n.T("ar", "server.healthy"))
	assert.Equal(t, "خدمة المدفوعات تعمل بصحة جيدة", i18n.T("AR-eg", "server.healthy"))
	// unknown language falls back to english lookup through NormalizeLang
	assert.Equal(t, "Payment service is healthy", i18n.T("de", "server.healthy"))
}

func TestTUnknownKeyFallsBackToEnglishThenKey(t *testing.T) {
	// key missing in the requested dictionary -> english lookup, then key itself
	assert.Equal(t, "totally.unknown.key", i18n.T("en", "totally.unknown.key"))
	assert.Equal(t, "totally.unknown.key", i18n.T("ar", "totally.unknown.key"))
}

func TestTWithArgs(t *testing.T) {
	// messages contain no verbs, but the args branch must still execute
	assert.Equal(t, "Payment not found%!(EXTRA string=ignored)", i18n.T("en", "payment.not_found", "ignored"))
	assert.Equal(t, "x", i18n.T("en", "%s", "x"))
}

func TestFromContext(t *testing.T) {
	assert.Equal(t, "en", i18n.FromContext(nil)) // nil context
	assert.Equal(t, "en", i18n.FromContext(context.Background()))
	assert.Equal(t, "ar", i18n.FromContext(i18n.WithLanguage(context.Background(), "ar")))
	assert.Equal(t, "en", i18n.FromContext(i18n.WithLanguage(context.Background(), "fr")))

	// non-string value stored under the key
	ctx := context.WithValue(context.Background(), i18n.LangKey, 42)
	assert.Equal(t, "en", i18n.FromContext(ctx))

	// empty string value
	ctx = context.WithValue(context.Background(), i18n.LangKey, "")
	assert.Equal(t, "en", i18n.FromContext(ctx))
}

func TestWithLanguageNormalizes(t *testing.T) {
	ctx := i18n.WithLanguage(context.Background(), "AR-XA")
	assert.Equal(t, "ar", i18n.FromContext(ctx))
}

func TestDefaultLangConstant(t *testing.T) {
	assert.Equal(t, "en", i18n.DefaultLang)
}
