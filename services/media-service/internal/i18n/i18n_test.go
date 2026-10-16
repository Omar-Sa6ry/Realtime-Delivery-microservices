package i18n

import (
	"context"
	"testing"
)

func TestNormalizeLang(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"english", "en", "en"},
		{"arabic", "ar", "ar"},
		{"regional_arabic", "ar-EG", "ar"},
		{"uppercase_arabic", "AR", "ar"},
		{"uppercase_regional", "AR-eg", "ar"},
		{"padded_english", " en ", "en"},
		{"padded_arabic", "  AR  ", "ar"},
		{"unsupported_language", "fr", "en"},
		{"empty", "", "en"},
		{"arabic_word_prefix", "arabic", "ar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeLang(tc.in); got != tc.want {
				t.Errorf("NormalizeLang(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTranslate(t *testing.T) {
	cases := []struct {
		name string
		lang string
		key  string
		want string
	}{
		{"english_known_key", "en", "media.fetched", "Media fetched successfully"},
		{"arabic_known_key", "ar", "media.fetched", "تم استرجاع الوسائط بنجاح"},
		{"arabic_error_key", "ar", "error.internal", "حدث خطأ داخلي في الخادم"},
		{"english_error_key", "en", "error.internal", "internal server error"},
		{"regional_tag_falls_back_to_arabic", "ar-SA", "media.not_found", "الملف غير موجود"},
		{"unknown_language_uses_english", "fr", "media.not_found", "Media not found"},
		{"unknown_language_uppercase", "DE", "server.healthy", "healthy"},
		{"unknown_key_returns_key", "en", "does.not.exist", "does.not.exist"},
		{"unknown_key_arabic_returns_key", "ar", "does.not.exist", "does.not.exist"},
		{"unknown_key_unknown_language", "xx", "does.not.exist", "does.not.exist"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Translate(tc.lang, tc.key); got != tc.want {
				t.Errorf("Translate(%q, %q) = %q, want %q", tc.lang, tc.key, got, tc.want)
			}
		})
	}
}

func TestT(t *testing.T) {
	if got := T("ar", "media.deleted"); got != "تم حذف الوسائط بنجاح" {
		t.Errorf("T(ar, media.deleted) = %q", got)
	}
	if got := T("en", "media.deleted"); got != Translate("en", "media.deleted") {
		t.Errorf("T(en, ...) = %q, want same as Translate", got)
	}
}

func TestFromContext(t *testing.T) {
	t.Run("nil_context", func(t *testing.T) {
		if got := FromContext(nil); got != DefaultLang {
			t.Errorf("FromContext(nil) = %q, want %q", got, DefaultLang)
		}
	})

	t.Run("empty_context", func(t *testing.T) {
		if got := FromContext(context.Background()); got != DefaultLang {
			t.Errorf("FromContext(background) = %q, want %q", got, DefaultLang)
		}
	})

	t.Run("empty_language_value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), LangKey, "")
		if got := FromContext(ctx); got != DefaultLang {
			t.Errorf("FromContext(empty value) = %q, want %q", got, DefaultLang)
		}
	})

	t.Run("non_string_value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), LangKey, 42)
		if got := FromContext(ctx); got != DefaultLang {
			t.Errorf("FromContext(non-string) = %q, want %q", got, DefaultLang)
		}
	})

	t.Run("arabic_value", func(t *testing.T) {
		ctx := WithLanguage(context.Background(), "ar")
		if got := FromContext(ctx); got != "ar" {
			t.Errorf("FromContext(arabic) = %q, want ar", got)
		}
	})

	t.Run("regional_value_normalized", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), LangKey, "AR-EG")
		if got := FromContext(ctx); got != "ar" {
			t.Errorf("FromContext(AR-EG) = %q, want ar", got)
		}
	})
}

func TestWithLanguage(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"arabic", "ar", "ar"},
		{"regional", "ar-EG", "ar"},
		{"unsupported_normalized_to_default", "fr", "en"},
		{"uppercase", "EN", "en"},
		{"padded", " ar ", "ar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithLanguage(context.Background(), tc.in)
			if got := FromContext(ctx); got != tc.want {
				t.Errorf("FromContext(WithLanguage(%q)) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTranslate_UsesContextLanguageRoundTrip(t *testing.T) {
	ctx := WithLanguage(context.Background(), "ar")
	if got := Translate(FromContext(ctx), "media.upload_completed"); got != "اكتمل الرفع بنجاح" {
		t.Errorf("round-trip translation = %q", got)
	}
}

func TestTranslate_FallsBackToEnglishWhenTargetLanguageKeyMissing(t *testing.T) {
	orig := messages["ar"]
	messages["ar"] = map[string]string{}
	defer func() { messages["ar"] = orig }()

	if got := Translate("ar", "server.healthy"); got != "healthy" {
		t.Errorf("Translate(ar, server.healthy) = %q, want English fallback", got)
	}
	if got := Translate("ar", "missing.everywhere"); got != "missing.everywhere" {
		t.Errorf("Translate(ar, missing key) = %q, want the key itself", got)
	}
}
