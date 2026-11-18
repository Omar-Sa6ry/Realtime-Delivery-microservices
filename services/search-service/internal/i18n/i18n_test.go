package i18n

import (
	"context"
	"testing"
)

func TestNormalizeLang(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "en"},
		{"en", "en"},
		{"EN", "en"},
		{"  Ar  ", "ar"},
		{"ar-EG", "ar"},
		{"AR", "ar"},
		{"fr", "en"},
		{"english", "en"},
		{"\tAR\n", "ar"},
	}
	for _, c := range cases {
		if got := NormalizeLang(c.in); got != c.want {
			t.Errorf("NormalizeLang(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTranslate(t *testing.T) {
	if got := Translate("en", "error.unauthorized"); got != "authentication required: missing x-user-id header" {
		t.Errorf("en unauthorized = %q", got)
	}
	if got := Translate("ar", "error.unauthorized"); got != "تسجيل الدخول مطلوب: لم يتم العثور على المعرف" {
		t.Errorf("ar unauthorized = %q", got)
	}
	// unknown lang falls back to default catalog
	if got := Translate("fr", "error.unauthorized"); got != "authentication required: missing x-user-id header" {
		t.Errorf("fr fallback = %q", got)
	}
	// missing key in non-default lang falls back to default catalog first; key missing everywhere → key itself
	if got := Translate("en", "nope.missing"); got != "nope.missing" {
		t.Errorf("missing key = %q", got)
	}
	if got := Translate("ar", "nope.missing"); got != "nope.missing" {
		t.Errorf("missing key ar = %q", got)
	}
}

func TestT_AliasesTranslate(t *testing.T) {
	for _, key := range []string{
		"server.healthy",
		"search.deliveries_fetched",
		"search.drivers_fetched",
		"search.media_fetched",
		"search.users_fetched",
		"search.autocomplete_fetched",
		"search.nearby_deliveries_fetched",
		"search.nearby_drivers_fetched",
		"search.reindex_started",
		"error.unauthorized",
		"error.forbidden_admin",
		"error.invalid_input",
		"error.internal",
	} {
		en := T("en", key)
		ar := T("ar", key)
		if en == "" || ar == "" {
			t.Errorf("empty translation for %q: en=%q ar=%q", key, en, ar)
		}
		if en == key {
			t.Errorf("en translation for %q returned the key itself", key)
		}
		if ar == key {
			t.Errorf("ar translation for %q returned the key itself", key)
		}
		if en == ar {
			t.Errorf("en and ar translations for %q are identical: %q", key, en)
		}
	}
}

func TestFromContext(t *testing.T) {
	if got := FromContext(nil); got != "en" {
		t.Errorf("FromContext(nil) = %q, want en", got)
	}
	if got := FromContext(context.Background()); got != "en" {
		t.Errorf("FromContext(bg) = %q, want en", got)
	}
	ctx := WithLanguage(context.Background(), "ar")
	if got := FromContext(ctx); got != "ar" {
		t.Errorf("FromContext(ar) = %q, want ar", got)
	}
	// empty string in context → default
	ctx = context.WithValue(context.Background(), LangKey, "")
	if got := FromContext(ctx); got != "en" {
		t.Errorf("FromContext(empty) = %q, want en", got)
	}
	// non-string value in context → default
	ctx = context.WithValue(context.Background(), LangKey, 42)
	if got := FromContext(ctx); got != "en" {
		t.Errorf("FromContext(non-string) = %q, want en", got)
	}
}

func TestWithLanguage_Normalizes(t *testing.T) {
	ctx := WithLanguage(context.Background(), " AR-EG ")
	if got := FromContext(ctx); got != "ar" {
		t.Errorf("WithLanguage should normalize, got %q", got)
	}
	if v, ok := ctx.Value(LangKey).(string); !ok || v != "ar" {
		t.Errorf("stored lang = %v", v)
	}
}
