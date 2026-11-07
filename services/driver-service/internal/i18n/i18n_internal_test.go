package i18n

import "testing"

// TestTranslateFallsBackToEnglish removes a key from the Arabic table for
// the duration of the test so that the English fallback branch of Translate
// is genuinely exercised, then restores it.
func TestTranslateFallsBackToEnglish(t *testing.T) {
	const key = "driver.not_found"

	origAr, ok := messages["ar"][key]
	if !ok {
		t.Fatalf("expected %q to exist in the ar table", key)
	}
	delete(messages["ar"], key)
	defer func() {
		messages["ar"][key] = origAr
	}()

	got := Translate("ar", key)
	want := messages["en"][key]
	if got != want {
		t.Errorf("Translate(ar, %q) = %q, want English fallback %q", key, got, want)
	}

	// Restore the Arabic table explicitly, then verify it is intact.
	messages["ar"][key] = origAr
	if restored := messages["ar"][key]; restored != origAr {
		t.Errorf("ar table not restored: got %q, want %q", restored, origAr)
	}
}
