package e2e

import (
	"testing"
	"time"
)

// TestSmokeImageUploadHappyPath is the harness smoke test: it proves the whole
// live pipeline (GraphQL -> S3 multipart -> DDB -> workers -> READY) works
// before the richer scenario files run.
func TestSmokeImageUploadHappyPath(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	payload := PNGBytes(t, 64, 64)
	mediaID := UploadBytes(t, token, "smoke.png", "image/png", payload)

	media := WaitForMediaStatus(t, token, mediaID, "READY", 60*time.Second)

	if got := str(media["ownerId"]); got != userID {
		t.Fatalf("ownerId = %q, want %q", got, userID)
	}
	if got := str(media["fileName"]); got != "smoke.png" {
		t.Fatalf("fileName = %q", got)
	}
	if got := num(media["size"]); got != float64(len(payload)) {
		t.Fatalf("size = %v, want %d", got, len(payload))
	}

	WaitForVersion(t, token, mediaID, "thumbnail", 90*time.Second)
}
