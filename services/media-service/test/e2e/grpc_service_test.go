package e2e

import (
	"testing"
	"time"

	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/grpc/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func pbResolveReq(mediaID, requesterID, versionType string, expiry int32) *pb.ResolveMediaUrlRequest {
	return &pb.ResolveMediaUrlRequest{
		MediaId:       mediaID,
		RequesterId:   requesterID,
		VersionType:   versionType,
		ExpirySeconds: expiry,
	}
}

func pbGetMediaReq(mediaID, requesterID string) *pb.GetMediaRequest {
	return &pb.GetMediaRequest{MediaId: mediaID, RequesterId: requesterID}
}

// TestGRPCResolveMediaUrlOwner resolves a signed URL as the media owner.
func TestGRPCResolveMediaUrlOwner(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")
	payload := PNGBytes(t, 48, 48)
	mediaID := UploadBytes(t, token, "grpc.png", "image/png", payload)
	WaitForMediaStatus(t, token, mediaID, "READY", 120*time.Second)

	client := DialGRPC(t)
	resp, err := client.ResolveMediaUrl(t.Context(), pbResolveReq(mediaID, userID, "", 300))
	if err != nil {
		t.Fatalf("ResolveMediaUrl as owner: %v", err)
	}
	if resp.Url == "" {
		t.Fatal("empty url from ResolveMediaUrl")
	}
	if resp.MediaId != mediaID {
		t.Fatalf("mediaId = %q, want %q", resp.MediaId, mediaID)
	}
	if resp.ContentType != "image/png" {
		t.Fatalf("contentType = %q", resp.ContentType)
	}
	if resp.ExpiresAtSeconds <= time.Now().Unix() {
		t.Fatalf("expiresAtSeconds %d not in the future", resp.ExpiresAtSeconds)
	}
}

// TestGRPCResolveMediaUrlDenied blocks a non-owner requester.
func TestGRPCResolveMediaUrlDenied(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")
	mediaID := UploadBytes(t, token, "grpc-denied.png", "image/png", PNGBytes(t, 16, 16))
	WaitForMediaStatus(t, token, mediaID, "READY", 120*time.Second)

	client := DialGRPC(t)
	_, err := client.ResolveMediaUrl(t.Context(), pbResolveReq(mediaID, "someone-else", "", 300))
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v (err=%v)", status.Code(err), err)
	}
}

// TestGRPCResolveMediaUrlNotFound returns NotFound for unknown media.
func TestGRPCResolveMediaUrlNotFound(t *testing.T) {
	client := DialGRPC(t)
	_, err := client.ResolveMediaUrl(t.Context(), pbResolveReq("no-such-media-id", uniqueUserID(t), "", 300))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
}

// TestGRPCGetMedia covers metadata retrieval and ownership enforcement.
func TestGRPCGetMedia(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")
	payload := PNGBytes(t, 24, 24)
	mediaID := UploadBytes(t, token, "grpc-meta.png", "image/png", payload)
	WaitForMediaStatus(t, token, mediaID, "READY", 120*time.Second)

	client := DialGRPC(t)
	resp, err := client.GetMedia(t.Context(), pbGetMediaReq(mediaID, userID))
	if err != nil {
		t.Fatalf("GetMedia as owner: %v", err)
	}
	if resp.MediaId != mediaID || resp.OwnerId != userID {
		t.Fatalf("unexpected identity: %+v", resp)
	}
	if resp.Size != int64(len(payload)) {
		t.Fatalf("size = %d, want %d", resp.Size, len(payload))
	}
	if resp.Status != "READY" {
		t.Fatalf("status = %q, want READY", resp.Status)
	}
	if resp.FileName != "grpc-meta.png" {
		t.Fatalf("fileName = %q", resp.FileName)
	}

	_, err = client.GetMedia(t.Context(), pbGetMediaReq(mediaID, "intruder"))
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for non-owner GetMedia, got %v (err=%v)", status.Code(err), err)
	}

	_, err = client.GetMedia(t.Context(), pbGetMediaReq("missing-media", userID))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
}

// TestGRPCResolveMediaUrlThumbnail resolves a rendition URL.
func TestGRPCResolveMediaUrlThumbnail(t *testing.T) {
	userID := uniqueUserID(t)
	token := mintToken(t, userID, "user")
	mediaID := UploadBytes(t, token, "grpc-thumb.png", "image/png", PNGBytes(t, 64, 64))
	WaitForVersion(t, token, mediaID, "thumbnail", 120*time.Second)

	client := DialGRPC(t)
	resp, err := client.ResolveMediaUrl(t.Context(), pbResolveReq(mediaID, userID, "thumbnail", 120))
	if err != nil {
		t.Fatalf("ResolveMediaUrl thumbnail: %v", err)
	}
	if resp.Url == "" {
		t.Fatal("empty thumbnail url")
	}
	downloaded := FetchBytes(t, resp.Url)
	if len(downloaded) == 0 {
		t.Fatal("empty thumbnail download via grpc-resolved url")
	}
}
