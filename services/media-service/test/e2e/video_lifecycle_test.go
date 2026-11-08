package e2e

import (
	"testing"
	"time"
)

// TestVideoUploadLifecycle uploads a real H.264 clip and waits for the full
// in-cluster pipeline: scan -> ffmpeg transcode -> READY with renditions.
func TestVideoUploadLifecycle(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	payload := TinyMP4(t)
	mediaID := UploadBytes(t, token, "clip.mp4", "video/mp4", payload)
	media := WaitForMediaStatus(t, token, mediaID, "READY", 240*time.Second)

	if got := str(media["mediaType"]); got != "VIDEO" {
		t.Fatalf("mediaType = %q, want VIDEO", got)
	}
	if got := str(media["contentType"]); got != "video/mp4" {
		t.Fatalf("contentType = %q", got)
	}
	if got := num(media["size"]); got != float64(len(payload)) {
		t.Fatalf("size = %v, want %d", got, len(payload))
	}

	thumbnail := WaitForVersion(t, token, mediaID, "thumbnail", 120*time.Second)
	if num(thumbnail["size"]) <= 0 {
		t.Fatalf("video thumbnail has no size: %v", thumbnail)
	}

	// Original download is the untouched MP4 (valid ftyp container).
	origURL := DownloadURL(t, token, mediaID, "original", 600).opData(t, "downloadUrl")
	downloaded := FetchBytes(t, str(origURL["url"]))
	if len(downloaded) < 8 || string(downloaded[4:8]) != "ftyp" {
		t.Fatalf("downloaded original is not an mp4 (len=%d head=%q)", len(downloaded), truncate(downloaded, 16))
	}

	// Thumbnail download is a real image rendition produced by ffmpeg.
	thumbURL := DownloadURL(t, token, mediaID, "thumbnail", 300).opData(t, "downloadUrl")
	thumbBytes := FetchBytes(t, str(thumbURL["url"]))
	if len(thumbBytes) == 0 {
		t.Fatalf("empty video thumbnail download")
	}
}

// TestVideoRenewPresigned mid-upload covers renewPresigned for a live session.
func TestVideoRenewPresigned(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	payload := TinyMP4(t)
	session := CreateUploadSession(t, token, UploadInput{
		FileName:    "renew.mp4",
		ContentType: "video/mp4",
		Size:        int64(len(payload)),
	})

	renewed := gqlDo(t, token, `
mutation($input: RenewPresignedInput!) {
  renewPresigned(input: $input) {
    success statusCode message
    data { uploadId partSize totalParts presignedParts { partNumber presignedUrl } }
  }
}`, map[string]any{"input": map[string]any{
		"uploadId":      session.UploadID,
		"expirySeconds": 3600,
	}}).opData(t, "renewPresigned")

	rawParts, _ := renewed["presignedParts"].([]any)
	if len(rawParts) == 0 {
		t.Fatalf("renewPresigned returned no parts: %v", renewed)
	}
	first, _ := rawParts[0].(map[string]any)
	newURL := str(first["presignedUrl"])
	if newURL == "" {
		t.Fatalf("renewPresigned returned empty url: %v", renewed)
	}

	// The renewed URL is usable: upload part 1 through it.
	etag := putPart(t, httpClient, newURL, payload[:min64(int64(len(payload)), session.PartSize)])
	if etag == "" {
		t.Fatal("renewed part upload returned empty etag")
	}

	// Cleanup: abort so the session does not linger.
	AbortUpload(t, token, session.UploadID).assertNoErrors(t)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
