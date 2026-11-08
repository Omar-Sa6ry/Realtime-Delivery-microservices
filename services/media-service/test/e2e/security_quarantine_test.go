package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestMagicBytesMismatchMarksFailed uploads an EXE payload declared as
// image/png: completeUpload must fail the magic-bytes check and the media
// must end FAILED (quarantine pipeline entry point).
func TestMagicBytesMismatchMarksFailed(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	payload := EXEBytes()
	session := CreateUploadSession(t, token, UploadInput{
		FileName:    "photo.png",
		ContentType: "image/png",
		Size:        int64(len(payload)),
	})
	parts := PutParts(t, session, payload)

	res := gqlDo(t, session.Token, completeUploadMutation, map[string]any{"input": map[string]any{
		"uploadId":       session.UploadID,
		"parts":          parts,
		"idempotencyKey": "mismatch-" + session.UploadID,
	}})
	assertGraphQLError(t, res, "magic bytes")

	// The media record exists and was marked FAILED by the validation path.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if m := GetMedia(t, token, session.MediaID); m != nil && str(m["status"]) == "FAILED" {
			return
		}
		time.Sleep(1500 * time.Millisecond)
	}
	t.Fatalf("media %s not FAILED after magic-bytes rejection", session.MediaID)
}

// TestEICARDetectionQuarantines uploads the pure EICAR test signature as a
// text/plain file (no magic-signature constraint), so the bytes reach ClamAV
// intact and the scan worker must quarantine the media. Skipped automatically
// when clamd is unreachable.
func TestEICARDetectionQuarantines(t *testing.T) {
	if !ProbeTCP("127.0.0.1:3310") {
		t.Skip("clamd not reachable on 127.0.0.1:3310; skipping live malware scenario")
	}
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	mediaID := UploadBytes(t, token, "eicar.txt", "text/plain", []byte(EICARString))
	media := WaitForMediaStatus(t, token, mediaID, "QUARANTINED", 180*time.Second)
	if got := str(media["status"]); got != "QUARANTINED" {
		t.Fatalf("status = %q, want QUARANTINED", got)
	}

	// Quarantined media must not be downloadable.
	res := DownloadURL(t, token, mediaID, "original", 60)
	if len(res.Errors) == 0 {
		t.Fatalf("expected downloadUrl to be denied for quarantined media")
	}
}

// TestExtensionMismatchRejected spoofs the extension against the content type.
func TestExtensionMismatchRejected(t *testing.T) {
	token := mintToken(t, uniqueUserID(t), "user")
	res := gqlDo(t, token, createUploadSessionMutation, map[string]any{"input": map[string]any{
		"fileName":    "malware.exe",
		"contentType": "image/png",
		"size":        1024,
	}})
	assertGraphQLError(t, res, "extension")
}

// TestUnsupportedContentTypeRejected rejects types outside the whitelist.
func TestUnsupportedContentTypeRejected(t *testing.T) {
	token := mintToken(t, uniqueUserID(t), "user")
	res := gqlDo(t, token, createUploadSessionMutation, map[string]any{"input": map[string]any{
		"fileName":    "payload.bin",
		"contentType": "application/x-msdownload",
		"size":        1024,
	}})
	assertGraphQLError(t, res, "content type")
}

// TestZeroAndOversizeRejected covers the size bounds at session creation.
func TestZeroAndOversizeRejected(t *testing.T) {
	token := mintToken(t, uniqueUserID(t), "user")

	zero := gqlDo(t, token, createUploadSessionMutation, map[string]any{"input": map[string]any{
		"fileName": "empty.png", "contentType": "image/png", "size": 0,
	}})
	assertGraphQLError(t, zero, "greater than zero")

	over := gqlDo(t, token, createUploadSessionMutation, map[string]any{"input": map[string]any{
		"fileName": "huge.png", "contentType": "image/png", "size": 53687091201,
	}})
	assertGraphQLError(t, over, "exceeds maximum")
}

// TestChecksumMismatchRejected feeds a wrong SHA-256 at session creation.
func TestChecksumMismatchRejected(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	payload := PNGBytes(t, 20, 20)
	session := CreateUploadSession(t, token, UploadInput{
		FileName:    "badsum.png",
		ContentType: "image/png",
		Size:        int64(len(payload)),
		Checksum:    strings.Repeat("ab", 32),
	})
	parts := PutParts(t, session, payload)
	res := gqlDo(t, token, completeUploadMutation, map[string]any{"input": map[string]any{
		"uploadId":       session.UploadID,
		"parts":          parts,
		"idempotencyKey": "badsum-" + session.UploadID,
	}})
	assertGraphQLError(t, res, "checksum")
}

// TestDeclaredSizeMismatchRejected uploads more bytes than declared.
func TestDeclaredSizeMismatchRejected(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	real := PNGBytes(t, 40, 40)
	declared := int64(len(real) / 2)
	session := CreateUploadSession(t, token, UploadInput{
		FileName:    "sizelie.png",
		ContentType: "image/png",
		Size:        declared,
	})

	// Bypass PutParts and push the full payload into the single part slot.
	if len(session.Presigned) == 0 {
		t.Fatal("no presigned parts")
	}
	putPart(t, httpClient, session.Presigned[0].URL, real)

	res := gqlDo(t, token, completeUploadMutation, map[string]any{"input": map[string]any{
		"uploadId":       session.UploadID,
		"parts":          []CompletedPart{{PartNumber: session.Presigned[0].PartNumber, ETag: putPartETag(t, session.Presigned[0].URL, real)}},
		"idempotencyKey": "sizelie-" + session.UploadID,
	}})
	assertGraphQLError(t, res, "size")
}

func putPartETag(t *testing.T, url string, body []byte) string {
	t.Helper()
	return putPart(t, httpClient, url, body)
}

// TestUnauthorizedRejected verifies the auth gate on GraphQL.
func TestUnauthorizedRejected(t *testing.T) {
	res := gqlDo(t, "not-a-jwt", getMediaQuery, map[string]any{"mediaId": "whatever"})
	assertGraphQLError(t, res, "authentication required")
}

// TestAbortUploadSession verifies abort removes the session from tracking.
func TestAbortUploadSession(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	session := CreateUploadSession(t, token, UploadInput{
		FileName:    "abort.png",
		ContentType: "image/png",
		Size:        64,
	})
	AbortUpload(t, token, session.UploadID).assertNoErrors(t)

	// Completing an aborted session must fail.
	res := gqlDo(t, token, completeUploadMutation, map[string]any{"input": map[string]any{
		"uploadId":       session.UploadID,
		"parts":          []CompletedPart{{PartNumber: 1, ETag: "\"dead\""}},
		"idempotencyKey": "abort-complete-" + session.UploadID,
	}})
	if len(res.Errors) == 0 {
		t.Fatalf("expected completeUpload on aborted session to fail")
	}
}

// TestAdminCanDeleteOthersMediaAndUserCannot covers the ownership rule on delete.
func TestAdminCanDeleteOthersMediaAndUserCannot(t *testing.T) {
	ownerID := uniqueUserID(t)
	ownerToken := mintToken(t, ownerID, "user")
	mediaID := UploadBytes(t, ownerToken, "owned.png", "image/png", PNGBytes(t, 24, 24))
	WaitForMediaStatus(t, ownerToken, mediaID, "READY", 120*time.Second)

	otherToken := mintToken(t, uniqueUserID(t), "user")
	denied := DeleteMedia(t, otherToken, mediaID)
	assertGraphQLError(t, denied, "ownership mismatch")

	adminToken := mintToken(t, uniqueUserID(t), "admin")
	DeleteMedia(t, adminToken, mediaID).assertNoErrors(t)

	// Delete is asynchronous and soft: the worker flips the record to DELETED.
	WaitForMediaStatus(t, ownerToken, mediaID, "DELETED", 90*time.Second)
}
