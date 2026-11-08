package e2e

import (
	"testing"
	"time"
)

// TestQuotaTracking verifies usedBytes grows with completed uploads.
func TestQuotaTracking(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	first := PNGBytes(t, 16, 16)
	second := PNGBytes(t, 32, 32)
	UploadBytes(t, token, "q1.png", "image/png", first)
	UploadBytes(t, token, "q2.png", "image/png", second)

	quota := Quota(t, token)
	used := num(quota["usedBytes"])
	if used < float64(len(first)+len(second)) {
		t.Fatalf("usedBytes = %v, want >= %d", used, len(first)+len(second))
	}
	if q := num(quota["quotaBytes"]); q <= 0 {
		t.Fatalf("quotaBytes = %v, want > 0", q)
	}
	if r := num(quota["remainingBytes"]); r <= 0 {
		t.Fatalf("remainingBytes = %v, want > 0", r)
	}
	if mc := num(quota["maxConcurrentUploads"]); mc != 5 {
		t.Fatalf("maxConcurrentUploads = %v, want 5", mc)
	}
}

// TestUploadSessionRateLimit exhausts the per-user upload limiter (10/min).
func TestUploadSessionRateLimit(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	payload := PNGBytes(t, 8, 8)
	var limited bool
	for i := 0; i < 15; i++ {
		res := gqlDo(t, token, createUploadSessionMutation, map[string]any{"input": map[string]any{
			"fileName":    "rl.png",
			"contentType": "image/png",
			"size":        int64(len(payload)),
		}})
		if len(res.Errors) > 0 {
			assertGraphQLError(t, res, "rate limit")
			limited = true
			break
		}
		// Abort immediately so the concurrent-upload cap never interferes;
		// this loop must isolate the per-user rate limiter.
		env := res.op(t, "createUploadSession")
		inner, _ := env["data"].(map[string]any)
		AbortUpload(t, token, str(inner["uploadId"]))
	}

	if !limited {
		t.Fatalf("upload rate limit never triggered within 15 session creations")
	}
}

// TestMaxConcurrentUploads enforces MEDIA_MAX_CONCURRENT_UPLOADS=5.
func TestMaxConcurrentUploads(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	payload := PNGBytes(t, 8, 8)
	var sessions []UploadSession
	for i := 0; i < 5; i++ {
		session := CreateUploadSession(t, token, UploadInput{
			FileName:    "conc.png",
			ContentType: "image/png",
			Size:        int64(len(payload)),
		})
		sessions = append(sessions, session)
	}

	res := gqlDo(t, token, createUploadSessionMutation, map[string]any{"input": map[string]any{
		"fileName":    "conc-6.png",
		"contentType": "image/png",
		"size":        int64(len(payload)),
	}})
	assertGraphQLError(t, res, "concurrent")

	// Cleanup: aborting sessions frees the active-upload counters.
	for _, s := range sessions {
		AbortUpload(t, token, s.UploadID)
	}

	// After cleanup a new session succeeds again.
	time.Sleep(2 * time.Second)
	CreateUploadSession(t, token, UploadInput{
		FileName:    "conc-again.png",
		ContentType: "image/png",
		Size:        int64(len(payload)),
	})
}
