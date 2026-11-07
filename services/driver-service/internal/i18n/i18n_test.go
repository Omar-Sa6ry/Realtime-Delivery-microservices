package i18n_test

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/i18n"
)

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase ar", "ar", "ar"},
		{"uppercase AR", "AR", "ar"},
		{"mixed case Ar", "Ar", "ar"},
		{"regional ar-EG", "ar-EG", "ar"},
		{"regional ar-eg lowercase", "ar-eg", "ar"},
		{"whitespace padded ar", "  ar  ", "ar"},
		{"english en", "en", "en"},
		{"uppercase EN", "EN", "en"},
		{"regional en-US", "en-US", "en"},
		{"regional en-GB", "en-GB", "en"},
		{"whitespace padded en", "\ten\n", "en"},
		{"empty string", "", "en"},
		{"garbage", "french", "en"},
		{"garbage symbols", "!!", "en"},
		{"unrelated language", "fr", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := i18n.NormalizeLang(tt.input); got != tt.expected {
				t.Errorf("NormalizeLang(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTKnownKeysPerLanguage(t *testing.T) {
	// Real message text for representative keys in both supported languages,
	// so the translation tables are genuinely exercised.
	tests := []struct {
		lang     string
		key      string
		expected string
	}{
		{"en", "server.healthy", "healthy"},
		{"en", "server.running", "Driver service is running"},
		{"en", "server.method_not_allowed", "Method not allowed"},
		{"en", "validation.id_required", "id is required"},
		{"en", "driver.not_found", "Driver not found"},
		{"en", "driver.registered", "Driver registered successfully"},
		{"en", "assignment.lock_conflict", "Could not acquire lock for driver assignment"},
		{"en", "error.internal", "An internal server error occurred"},
		{"ar", "server.healthy", "تعمل بصحة جيدة"},
		{"ar", "server.running", "خدمة السائقين تعمل بنجاح"},
		{"ar", "server.method_not_allowed", "طريقة الطلب غير مسموح بها"},
		{"ar", "validation.id_required", "المعرف مطلوب"},
		{"ar", "driver.not_found", "السائق غير موجود"},
		{"ar", "driver.registered", "تم تسجيل السائق بنجاح"},
		{"ar", "assignment.lock_conflict", "تعذر حجز القفل لإسناد السائق (تزامن عمليات)"},
		{"ar", "error.internal", "حدث خطأ داخلي في الخادم"},
	}

	for _, tt := range tests {
		t.Run(tt.lang+"/"+tt.key, func(t *testing.T) {
			if got := i18n.T(tt.lang, tt.key); got != tt.expected {
				t.Errorf("T(%q, %q) = %q, want %q", tt.lang, tt.key, got, tt.expected)
			}
		})
	}
}

func TestTranslateMatchesT(t *testing.T) {
	if got, want := i18n.Translate("ar", "driver.offline"), i18n.T("ar", "driver.offline"); got != want {
		t.Errorf("Translate = %q, T = %q; they must agree", got, want)
	}
	if got, want := i18n.Translate("ar", "driver.offline"), "تم قطع اتصال السائق بنجاح"; got != want {
		t.Errorf("Translate(ar, driver.offline) = %q, want %q", got, want)
	}
}

func TestTNormalizesLanguageBeforeLookup(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		expected string
	}{
		{"uppercase arabic resolves to ar", "AR", "السائق غير موجود"},
		{"regional arabic resolves to ar", "ar-SA", "السائق غير موجود"},
		{"uppercase english resolves to en", "EN", "Driver not found"},
		{"unknown language falls back to en", "de", "Driver not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := i18n.T(tt.lang, "driver.not_found"); got != tt.expected {
				t.Errorf("T(%q, driver.not_found) = %q, want %q", tt.lang, got, tt.expected)
			}
		})
	}
}

func TestTUnknownKeyReturnsKey(t *testing.T) {
	tests := []struct {
		name string
		lang string
		key  string
	}{
		{"unknown key english", "en", "no.such.key"},
		{"unknown key arabic", "ar", "no.such.key"},
		{"unknown key unknown language", "xx", "another/missing:key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := i18n.T(tt.lang, tt.key); got != tt.key {
				t.Errorf("T(%q, %q) = %q, want the key itself", tt.lang, tt.key, got)
			}
		})
	}
}

func TestFromContext(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		if got := i18n.FromContext(nil); got != i18n.DefaultLang {
			t.Errorf("FromContext(nil) = %q, want %q", got, i18n.DefaultLang)
		}
	})

	t.Run("context without value", func(t *testing.T) {
		if got := i18n.FromContext(context.Background()); got != i18n.DefaultLang {
			t.Errorf("FromContext(bg) = %q, want %q", got, i18n.DefaultLang)
		}
	})

	t.Run("context with empty string value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), i18n.LangKey, "")
		if got := i18n.FromContext(ctx); got != i18n.DefaultLang {
			t.Errorf("FromContext(empty lang) = %q, want %q", got, i18n.DefaultLang)
		}
	})

	t.Run("context with non-string value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), i18n.LangKey, 42)
		if got := i18n.FromContext(ctx); got != i18n.DefaultLang {
			t.Errorf("FromContext(int lang) = %q, want %q", got, i18n.DefaultLang)
		}
	})

	t.Run("context with arabic value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), i18n.LangKey, "ar")
		if got := i18n.FromContext(ctx); got != "ar" {
			t.Errorf("FromContext(ar) = %q, want %q", got, "ar")
		}
	})

	t.Run("context with unnormalized value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), i18n.LangKey, "AR-EG")
		if got := i18n.FromContext(ctx); got != "ar" {
			t.Errorf("FromContext(AR-EG) = %q, want %q", got, "ar")
		}
	})
}

func TestWithLanguage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"arabic", "ar", "ar"},
		{"uppercase arabic", "AR", "ar"},
		{"regional arabic", "ar-EG", "ar"},
		{"english", "en", "en"},
		{"invalid falls back to en", "bogus", "en"},
		{"empty falls back to en", "", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := i18n.WithLanguage(context.Background(), tt.input)
			if got := i18n.FromContext(ctx); got != tt.expected {
				t.Errorf("WithLanguage(%q) then FromContext = %q, want %q", tt.input, got, tt.expected)
			}
			// Round trip: the stored language must resolve real translations.
			want := i18n.T(tt.expected, "driver.online")
			if got := i18n.T(i18n.FromContext(ctx), "driver.online"); got != want {
				t.Errorf("T via context = %q, want %q", got, want)
			}
		})
	}
}

func TestDefaultLangConstant(t *testing.T) {
	if i18n.DefaultLang != "en" {
		t.Errorf("DefaultLang = %q, want %q", i18n.DefaultLang, "en")
	}
}
