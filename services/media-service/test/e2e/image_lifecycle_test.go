package e2e

import (
	"bytes"
	"crypto/sha256"
	"testing"
	"time"

	sharednats "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/nats"
	sharedws "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/websocket"
)

// TestImageUploadLifecycle covers the full happy path for images:
// quota accounting, processing, versions, download URLs and realtime events.
func TestImageUploadLifecycle(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	quotaBefore := Quota(t, token)
	if used := num(quotaBefore["usedBytes"]); used != 0 {
		t.Fatalf("expected fresh user quota 0, got %v", used)
	}

	natsCh := SubscribeNATS(t, sharednats.RealtimeMediaProcessingProgress)
	readyCh := SubscribeNATS(t, sharednats.RealtimeMediaReady)

	payload := PNGBytes(t, 128, 96)
	mediaID := UploadBytes(t, token, "photo.png", "image/png", payload)
	media := WaitForMediaStatus(t, token, mediaID, "READY", 120*time.Second)

	if got := str(media["ownerId"]); got != userID {
		t.Fatalf("ownerId = %q, want %q", got, userID)
	}
	if got := str(media["contentType"]); got != "image/png" {
		t.Fatalf("contentType = %q", got)
	}
	if got := str(media["mediaType"]); got != "IMAGE" {
		t.Fatalf("mediaType = %q, want IMAGE", got)
	}
	if got := num(media["size"]); got != float64(len(payload)) {
		t.Fatalf("size = %v, want %d", got, len(payload))
	}

	thumbnail := WaitForVersion(t, token, mediaID, "thumbnail", 90*time.Second)
	if num(thumbnail["size"]) <= 0 {
		t.Fatalf("thumbnail has no size: %v", thumbnail)
	}

	// Quota now accounts for the stored original.
	quotaAfter := Quota(t, token)
	if used := num(quotaAfter["usedBytes"]); used < float64(len(payload)) {
		t.Fatalf("usedBytes = %v, want >= %d", used, len(payload))
	}
	if remaining := num(quotaAfter["remainingBytes"]); remaining <= 0 {
		t.Fatalf("remainingBytes = %v, want > 0", remaining)
	}

	// Original download URL returns the exact bytes we uploaded.
	origURL := DownloadURL(t, token, mediaID, "original", 300).opData(t, "downloadUrl")
	if str(origURL["url"]) == "" {
		t.Fatalf("empty download url: %v", origURL)
	}
	downloaded := FetchBytes(t, str(origURL["url"]))
	wantSum := sha256.Sum256(payload)
	gotSum := sha256.Sum256(downloaded)
	if gotSum != wantSum {
		t.Fatalf("downloaded original bytes differ from uploaded payload")
	}

	// Thumbnail download is a real image rendition (PNG signature).
	thumbURL := DownloadURL(t, token, mediaID, "thumbnail", 300).opData(t, "downloadUrl")
	thumbBytes := FetchBytes(t, str(thumbURL["url"]))
	if len(thumbBytes) < 8 || !bytes.HasPrefix(thumbBytes, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}) {
		t.Fatalf("thumbnail download is not a png (len=%d head=%q)", len(thumbBytes), truncate(thumbBytes, 16))
	}

	// The media appears in the owner's listing.
	list := gqlDo(t, token, `
query($ownerId: String!, $limit: Int) {
  listMedia(ownerId: $ownerId, limit: $limit) {
    success statusCode message
    data { items { mediaId fileName status } }
  }
}`, map[string]any{"ownerId": userID, "limit": 10}).opData(t, "listMedia")
	found := false
	for _, raw := range list["items"].([]any) {
		item, _ := raw.(map[string]any)
		if str(item["mediaId"]) == mediaID {
			found = true
		}
	}
	if !found {
		t.Fatalf("media %s missing from listMedia", mediaID)
	}

	// Realtime pipeline events were fanned out over NATS.
	WaitForNATSMention(t, natsCh, mediaID, 90*time.Second)
	WaitForNATSMention(t, readyCh, mediaID, 90*time.Second)
}

// TestImageDeleteLifecycle verifies delete propagation: READY -> gone.
func TestImageDeleteLifecycle(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")

	payload := PNGBytes(t, 32, 32)
	mediaID := UploadBytes(t, token, "delete-me.png", "image/png", payload)
	WaitForMediaStatus(t, token, mediaID, "READY", 120*time.Second)

	DeleteMedia(t, token, mediaID).assertNoErrors(t)

	// Delete is asynchronous and soft: the worker flips the record to DELETED.
	WaitForMediaStatus(t, token, mediaID, "DELETED", 90*time.Second)
}

// TestWebSocketHandshake verifies the live socket contract: PONG for PING and
// SUBSCRIBED for SUBSCRIBE_DELIVERY (the hub is silent until the client speaks).
func TestWebSocketHandshake(t *testing.T) {
	token := mintToken(t, uniqueUserID(t), "user")
	conn := DialWS(t, token)

	writeWSJSON(t, conn, map[string]any{
		"type": string(sharedws.ClientMessagePing), "requestId": "ping-1",
	})
	pong := WaitForWSEvent(t, conn, string(sharedws.ServerMessagePong), 10*time.Second)
	if str(pong["requestId"]) != "ping-1" {
		t.Fatalf("pong requestId = %v", pong["requestId"])
	}

	writeWSJSON(t, conn, map[string]any{
		"type": string(sharedws.ClientMessageSubscribeDelivery), "requestId": "sub-1",
	})
	sub := WaitForWSEvent(t, conn, string(sharedws.ServerMessageSubscribed), 10*time.Second)
	if str(sub["requestId"]) != "sub-1" {
		t.Fatalf("subscribed requestId = %v", sub["requestId"])
	}
}

// TestDownloadRateLimit exhausts the per-user download limiter (60/min).
func TestDownloadRateLimit(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")
	mediaID := UploadBytes(t, token, "rl.png", "image/png", PNGBytes(t, 16, 16))
	WaitForMediaStatus(t, token, mediaID, "READY", 120*time.Second)

	var limited bool
	for i := 0; i < 75; i++ {
		res := DownloadURL(t, token, mediaID, "original", 60)
		if len(res.Errors) > 0 {
			assertGraphQLError(t, res, "rate limit")
			limited = true
			break
		}
	}
	if !limited {
		t.Fatalf("download rate limit never triggered within 75 requests")
	}
}

func writeWSJSON(t *testing.T, conn interface {
	WriteJSON(v any) error
}, msg map[string]any) {
	t.Helper()
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write ws json: %v", err)
	}
}
