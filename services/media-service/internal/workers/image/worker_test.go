package image

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kafkago "github.com/segmentio/kafka-go"

	sharednats "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/nats"
	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

const (
	fakeFFmpegPipeScript = "@echo off\r\n" +
		"setlocal\r\nset \"target=\"\r\n:loop\r\nif \"%~1\"==\"\" goto done\r\nset \"target=%~1\"\r\nshift\r\ngoto loop\r\n:done\r\n" +
		"if \"%target%\"==\"pipe:1\" goto tostdout\r\necho file-output>\"%target%\"\r\nexit /b 0\r\n:tostdout\r\necho fake-modern-output\r\nexit /b 0\r\n"
	fakeFFmpegFailScript  = "@echo off\r\nexit /b 1\r\n"
	fakeFFmpegEmptyScript = "@echo off\r\nexit /b 0\r\n"
)

type fixture struct {
	worker      *Worker
	mediaRepo   *testutil.MockMediaRepository
	versionRepo *testutil.MockVersionRepository
	storage     *testutil.MemStorage
	cache       *testutil.MockCache
	pub         *testutil.MockPublisher
	realtime    *testutil.MockRealtimePublisher
}

func newFixture() *fixture {
	f := &fixture{
		mediaRepo:   testutil.NewMockMediaRepository(),
		versionRepo: testutil.NewMockVersionRepository(),
		storage:     testutil.NewMemStorage(),
		cache:       testutil.NewMockCache(),
		pub:         testutil.NewMockPublisher(),
		realtime:    &testutil.MockRealtimePublisher{},
	}
	f.worker = NewWorker(f.mediaRepo, f.versionRepo, f.storage, f.cache, f.pub, f.realtime)
	return f
}

func hideFFmpeg(t *testing.T) {
	t.Helper()
	t.Setenv("FFMPEG_PATH", "")
	t.Setenv("PATH", t.TempDir())
}

func fakeExecutable(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
	return path
}

func imageBytes(t *testing.T, w, h int, encode func(*bytes.Buffer, image.Image) error) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := encode(&buf, img); err != nil {
		t.Fatalf("encode image: %v", err)
	}
	return buf.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	return imageBytes(t, w, h, func(buf *bytes.Buffer, img image.Image) error { return png.Encode(buf, img) })
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	return imageBytes(t, w, h, func(buf *bytes.Buffer, img image.Image) error {
		return jpeg.Encode(buf, img, &jpeg.Options{Quality: 90})
	})
}

func imageMessage(t *testing.T, mediaType, objectKey, fileName, contentType string) kafkago.Message {
	t.Helper()
	payload := map[string]interface{}{
		"mediaId":     "m-1",
		"userId":      "u-1",
		"fileName":    fileName,
		"objectKey":   objectKey,
		"size":        int64(2048),
		"contentType": contentType,
		"mediaType":   mediaType,
	}
	return testutil.EnvelopeMessage(t, kafkaadapter.TopicScanCompleted, "trace-1", payload)
}

func versionsByType(repo *testutil.MockVersionRepository) map[domain.VersionType]*domain.MediaVersion {
	out := make(map[domain.VersionType]*domain.MediaVersion, len(repo.Created))
	for _, v := range repo.Created {
		out[v.VersionType] = v
	}
	return out
}

func mediaReadyPayload(t *testing.T, pub *testutil.MockPublisher) map[string]interface{} {
	t.Helper()
	msgs := pub.ForTopic(kafkaadapter.TopicMediaReady)
	if len(msgs) == 0 {
		t.Fatal("no media.ready event published")
	}
	var payload map[string]interface{}
	testutil.DecodeEnvelope(t, msgs[len(msgs)-1].Payload, &payload)
	return payload
}

func TestHandleSkipsNonImage(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)

	if err := f.worker.Handle(context.Background(), imageMessage(t, "VIDEO", "clip.mp4", "clip.mp4", "video/mp4")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	if len(f.storage.Calls()) != 0 {
		t.Fatalf("storage calls = %v, want none for non-image media", f.storage.Calls())
	}
	if len(f.pub.Messages) != 0 {
		t.Fatalf("published %d events, want none for non-image media", len(f.pub.Messages))
	}
}

func TestHandleInvalidEnvelope(t *testing.T) {
	f := newFixture()
	err := f.worker.Handle(context.Background(), kafkago.Message{Value: []byte("not-json")})
	if !errors.Is(err, kafkaadapter.ErrPermanent) {
		t.Fatalf("Handle = %v, want ErrPermanent", err)
	}
}

func TestHandleInvalidPayload(t *testing.T) {
	f := newFixture()
	raw := []byte(`{"eventId":"e-1","eventType":"media.scan.completed","timestamp":1,"payload":"not-an-object"}`)
	err := f.worker.Handle(context.Background(), kafkago.Message{Value: raw})
	if !errors.Is(err, kafkaadapter.ErrPermanent) {
		t.Fatalf("Handle = %v, want ErrPermanent", err)
	}
}

func TestProcessImageDownloadFailure(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)

	err := f.worker.Handle(context.Background(), imageMessage(t, "IMAGE", "missing.png", "missing.png", "image/png"))
	if err == nil {
		t.Fatal("Handle = nil, want a download error")
	}
	if !strings.Contains(err.Error(), "download original image") {
		t.Fatalf("Handle = %v, want download original image error", err)
	}
}

func TestProcessImageGarbageBytesStillBecomesReady(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)
	f.storage.SetObject("bad.png", []byte("definitely not a png"))

	if err := f.worker.Handle(context.Background(), imageMessage(t, "IMAGE", "bad.png", "bad.png", "image/png")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	if len(f.versionRepo.Created) != 0 {
		t.Fatalf("created %d versions, want none for undecodable bytes", len(f.versionRepo.Created))
	}
	if len(f.mediaRepo.UpdateStatuses) == 0 ||
		f.mediaRepo.UpdateStatuses[len(f.mediaRepo.UpdateStatuses)-1].Next != domain.MediaStatusReady {
		t.Fatalf("UpdateStatuses = %+v, want the final READY transition", f.mediaRepo.UpdateStatuses)
	}
	if f.pub.Count(kafkaadapter.TopicMediaReady) != 1 {
		t.Fatalf("media.ready count = %d, want 1", f.pub.Count(kafkaadapter.TopicMediaReady))
	}
	if f.realtime.Count(sharednats.RealtimeMediaReady) != 1 {
		t.Fatalf("realtime ready events = %d, want 1", f.realtime.Count(sharednats.RealtimeMediaReady))
	}
}

func TestProcessImagePNGHappyPathWithoutFFmpeg(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)
	f.storage.SetObjectWithContentType("users/u-1/media/m-1/original/photo.png", "image/png", pngBytes(t, 400, 300))

	msg := imageMessage(t, "IMAGE", "users/u-1/media/m-1/original/photo.png", "photo.png", "image/png")
	if err := f.worker.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	versions := versionsByType(f.versionRepo)
	if len(versions) != 2 {
		t.Fatalf("created versions %v, want exactly thumbnail and medium", versions)
	}
	thumb, ok := versions[domain.VersionTypeThumbnail]
	if !ok {
		t.Fatalf("versions %v, want a thumbnail", versions)
	}
	if thumb.ObjectKey != "users/u-1/media/m-1/thumbnail/photo.png" {
		t.Fatalf("thumbnail key = %q", thumb.ObjectKey)
	}
	if thumb.ContentType != "image/png" || thumb.Width != 200 || thumb.Height != 150 {
		t.Fatalf("thumbnail = %+v, want 200x150 image/png", thumb)
	}
	if data, found := f.storage.Object(thumb.ObjectKey); !found || len(data) == 0 {
		t.Fatalf("thumbnail object missing or empty in storage (found=%v)", found)
	}

	medium, ok := versions[domain.VersionTypeMedium]
	if !ok {
		t.Fatalf("versions %v, want a medium rendition", versions)
	}
	if medium.ObjectKey != "users/u-1/media/m-1/medium/photo.png" {
		t.Fatalf("medium key = %q", medium.ObjectKey)
	}
	if medium.ContentType != "image/png" || medium.Width != 800 || medium.Height != 600 {
		t.Fatalf("medium = %+v, want 800x600 image/png", medium)
	}
	if data, found := f.storage.Object(medium.ObjectKey); !found || len(data) == 0 {
		t.Fatal("medium object missing in storage")
	}

	if len(f.mediaRepo.UpdateStatuses) != 1 ||
		f.mediaRepo.UpdateStatuses[0] != (testutil.MediaStatusCall{
			MediaID: "m-1", Current: domain.MediaStatusProcessing, Next: domain.MediaStatusReady,
		}) {
		t.Fatalf("UpdateStatuses = %+v, want PROCESSING->READY", f.mediaRepo.UpdateStatuses)
	}

	payload := mediaReadyPayload(t, f.pub)
	if payload["fileName"] != "photo.png" {
		t.Fatalf("media.ready payload = %v, want fileName photo.png", payload)
	}

	if got := f.cache.ProcessingProgress["m-1"]; got != 100 {
		t.Fatalf("processing progress = %d, want 100", got)
	}
	if f.realtime.Count(sharednats.RealtimeMediaProcessingProgress) < 4 {
		t.Fatalf("realtime progress events = %d, want at least 4 (10/30/60/100)", f.realtime.Count(sharednats.RealtimeMediaProcessingProgress))
	}
}

func TestProcessJPEGUsesJPEGRenditions(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)
	f.storage.SetObject("photo.jpg", jpegBytes(t, 300, 300))

	msg := imageMessage(t, "IMAGE", "photo.jpg", "photo.jpg", "image/jpeg")
	if err := f.worker.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	versions := versionsByType(f.versionRepo)
	thumb, ok := versions[domain.VersionTypeThumbnail]
	if !ok {
		t.Fatalf("versions %v, want a thumbnail", versions)
	}
	if thumb.ContentType != "image/jpeg" {
		t.Fatalf("thumbnail ContentType = %q, want image/jpeg", thumb.ContentType)
	}
	if !strings.HasSuffix(thumb.ObjectKey, ".jpg") {
		t.Fatalf("thumbnail key = %q, want a .jpg suffix", thumb.ObjectKey)
	}
	if data, found := f.storage.Object(thumb.ObjectKey); !found || len(data) == 0 {
		t.Fatal("jpeg thumbnail object missing in storage")
	}
}

func TestProcessImageModernFormatsWithFakeFFmpeg(t *testing.T) {
	f := newFixture()
	t.Setenv("FFMPEG_PATH", fakeExecutable(t, "ffmpeg.bat", fakeFFmpegPipeScript))
	f.storage.SetObject("photo.png", pngBytes(t, 250, 250))

	msg := imageMessage(t, "IMAGE", "photo.png", "photo.png", "image/png")
	if err := f.worker.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	versions := versionsByType(f.versionRepo)
	if len(versions) != 4 {
		t.Fatalf("created versions %v, want thumbnail, medium, webp and avif", versions)
	}
	for _, vt := range []domain.VersionType{domain.VersionTypeWebP, domain.VersionTypeAVIF} {
		v, ok := versions[vt]
		if !ok {
			t.Fatalf("versions %v, want a %s rendition", versions, vt)
		}
		if data, found := f.storage.Object(v.ObjectKey); !found || len(data) == 0 {
			t.Fatalf("%s object %q missing in storage", vt, v.ObjectKey)
		}
	}
	if versions[domain.VersionTypeWebP].ContentType != "image/webp" {
		t.Fatalf("webp ContentType = %q", versions[domain.VersionTypeWebP].ContentType)
	}
	if versions[domain.VersionTypeAVIF].ContentType != "image/avif" {
		t.Fatalf("avif ContentType = %q", versions[domain.VersionTypeAVIF].ContentType)
	}
}

func TestProcessImageModernFormatFailuresAreNonFatal(t *testing.T) {
	cases := []struct {
		name   string
		script string
	}{
		{name: "ffmpeg exits non-zero", script: fakeFFmpegFailScript},
		{name: "ffmpeg returns empty output", script: fakeFFmpegEmptyScript},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			t.Setenv("FFMPEG_PATH", fakeExecutable(t, "ffmpeg.bat", tc.script))
			f.storage.SetObject("photo.png", pngBytes(t, 250, 250))

			msg := imageMessage(t, "IMAGE", "photo.png", "photo.png", "image/png")
			if err := f.worker.Handle(context.Background(), msg); err != nil {
				t.Fatalf("Handle = %v, want nil despite the modern format failure", err)
			}
			versions := versionsByType(f.versionRepo)
			if len(versions) != 2 {
				t.Fatalf("created versions %v, want only thumbnail and medium", versions)
			}
			if f.pub.Count(kafkaadapter.TopicMediaReady) != 1 {
				t.Fatal("media.ready was not published")
			}
		})
	}
}

func TestProcessImageUploadAndPersistErrorsAreNonFatal(t *testing.T) {
	t.Run("put object failure", func(t *testing.T) {
		f := newFixture()
		hideFFmpeg(t)
		f.storage.SetObject("photo.png", pngBytes(t, 200, 200))
		f.storage.SetError("PutObject", errors.New("s3 unavailable"))

		msg := imageMessage(t, "IMAGE", "photo.png", "photo.png", "image/png")
		if err := f.worker.Handle(context.Background(), msg); err != nil {
			t.Fatalf("Handle = %v, want nil despite PutObject failures", err)
		}
		if len(f.versionRepo.Created) != 0 {
			t.Fatalf("created %d versions, want none when uploads fail", len(f.versionRepo.Created))
		}
		if f.pub.Count(kafkaadapter.TopicMediaReady) != 1 {
			t.Fatal("media.ready was not published")
		}
	})

	t.Run("version repo failure", func(t *testing.T) {
		f := newFixture()
		hideFFmpeg(t)
		f.storage.SetObject("photo.png", pngBytes(t, 200, 200))
		f.versionRepo.CreateErr = errors.New("dynamo down")

		msg := imageMessage(t, "IMAGE", "photo.png", "photo.png", "image/png")
		if err := f.worker.Handle(context.Background(), msg); err != nil {
			t.Fatalf("Handle = %v, want nil despite version repo failures", err)
		}
		if f.pub.Count(kafkaadapter.TopicMediaReady) != 1 {
			t.Fatal("media.ready was not published")
		}
	})
}

func TestProcessImageTransitionFailure(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)
	f.storage.SetObject("photo.png", pngBytes(t, 100, 100))
	f.mediaRepo.UpdateStatusErr = errors.New("stale status")

	err := f.worker.Handle(context.Background(), imageMessage(t, "IMAGE", "photo.png", "photo.png", "image/png"))
	if err == nil {
		t.Fatal("Handle = nil, want a transition error")
	}
	if !strings.Contains(err.Error(), "transition to READY") {
		t.Fatalf("Handle = %v, want transition to READY error", err)
	}
}

func TestProcessImageFallsBackToRepoMetadata(t *testing.T) {
	t.Run("payload has no file name", func(t *testing.T) {
		f := newFixture()
		hideFFmpeg(t)
		f.storage.SetObject("photo.png", pngBytes(t, 80, 80))
		f.mediaRepo.Media = testutil.NewMedia("m-1", "u-1")

		payload := map[string]interface{}{
			"mediaId":     "m-1",
			"userId":      "u-1",
			"objectKey":   "photo.png",
			"size":        int64(0),
			"contentType": "image/png",
			"mediaType":   "IMAGE",
		}
		msg := testutil.EnvelopeMessage(t, kafkaadapter.TopicScanCompleted, "trace-1", payload)
		if err := f.worker.Handle(context.Background(), msg); err != nil {
			t.Fatalf("Handle = %v, want nil", err)
		}

		ready := mediaReadyPayload(t, f.pub)
		if ready["fileName"] != "photo.jpg" {
			t.Fatalf("media.ready payload = %v, want the file name from the media record", ready)
		}
		if int64(ready["size"].(float64)) != 1024 {
			t.Fatalf("size = %v, want the size from the media record", ready["size"])
		}
	})

	t.Run("media lookup fails", func(t *testing.T) {
		f := newFixture()
		hideFFmpeg(t)
		f.storage.SetObject("photo.png", pngBytes(t, 80, 80))
		f.mediaRepo.GetErr = errors.New("dynamo down")

		payload := map[string]interface{}{
			"mediaId":     "m-1",
			"userId":      "u-1",
			"objectKey":   "photo.png",
			"size":        int64(0),
			"contentType": "image/png",
			"mediaType":   "IMAGE",
		}
		msg := testutil.EnvelopeMessage(t, kafkaadapter.TopicScanCompleted, "trace-1", payload)
		if err := f.worker.Handle(context.Background(), msg); err != nil {
			t.Fatalf("Handle = %v, want nil", err)
		}

		ready := mediaReadyPayload(t, f.pub)
		if ready["fileName"] != "" {
			t.Fatalf("fileName = %v, want empty when the media record cannot be fetched", ready["fileName"])
		}
	})
}

func TestProcessImagePublisherErrorsAreSwallowed(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)
	f.pub.PublishErr = errors.New("broker down")
	f.storage.SetObject("photo.png", pngBytes(t, 80, 80))

	msg := imageMessage(t, "IMAGE", "photo.png", "photo.png", "image/png")
	if err := f.worker.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil despite the publisher error", err)
	}
}

func TestProcessImageModernFormatUploadAndPersistErrorsAreNonFatal(t *testing.T) {
	t.Run("put object failure", func(t *testing.T) {
		f := newFixture()
		t.Setenv("FFMPEG_PATH", fakeExecutable(t, "ffmpeg.bat", fakeFFmpegPipeScript))
		f.storage.SetObject("photo.png", pngBytes(t, 220, 220))
		f.storage.SetError("PutObject", errors.New("s3 unavailable"))

		msg := imageMessage(t, "IMAGE", "photo.png", "photo.png", "image/png")
		if err := f.worker.Handle(context.Background(), msg); err != nil {
			t.Fatalf("Handle = %v, want nil despite the modern format upload failure", err)
		}
		for _, vt := range []domain.VersionType{domain.VersionTypeWebP, domain.VersionTypeAVIF} {
			if _, ok := versionsByType(f.versionRepo)[vt]; ok {
				t.Fatalf("versions = %+v, want no %s version after a failed upload", f.versionRepo.Created, vt)
			}
		}
		if f.pub.Count(kafkaadapter.TopicMediaReady) != 1 {
			t.Fatal("media.ready was not published")
		}
	})

	t.Run("version repo failure", func(t *testing.T) {
		f := newFixture()
		t.Setenv("FFMPEG_PATH", fakeExecutable(t, "ffmpeg.bat", fakeFFmpegPipeScript))
		f.storage.SetObject("photo.png", pngBytes(t, 220, 220))
		f.versionRepo.CreateErr = errors.New("dynamo down")

		msg := imageMessage(t, "IMAGE", "photo.png", "photo.png", "image/png")
		if err := f.worker.Handle(context.Background(), msg); err != nil {
			t.Fatalf("Handle = %v, want nil despite the modern format persist failure", err)
		}
		if f.pub.Count(kafkaadapter.TopicMediaReady) != 1 {
			t.Fatal("media.ready was not published")
		}
	})
}

func TestBuildVersionKey(t *testing.T) {
	cases := []struct {
		name        string
		originalKey string
		versionType string
		ext         string
		want        string
	}{
		{
			name:        "replaces the original segment",
			originalKey: "users/u-1/media/m-1/original/photo.png",
			versionType: "thumbnail",
			ext:         ".png",
			want:        "users/u-1/media/m-1/thumbnail/photo.png",
		},
		{
			name:        "multi dot name keeps base without last extension",
			originalKey: "users/u/media/m/original/archive.tar.gz",
			versionType: "compressed",
			ext:         ".gz",
			want:        "users/u/media/m/compressed/archive.tar.gz",
		},
		{
			name:        "appends when there is no original segment",
			originalKey: "flat/photo.png",
			versionType: "medium",
			ext:         ".png",
			want:        "flat/photo.png_medium.png",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildVersionKey(tc.originalKey, tc.versionType, tc.ext); got != tc.want {
				t.Fatalf("buildVersionKey(%q, %q, %q) = %q, want %q",
					tc.originalKey, tc.versionType, tc.ext, got, tc.want)
			}
		})
	}
}

func TestResizeImage(t *testing.T) {
	cases := []struct {
		name         string
		srcW, srcH   int
		maxW, maxH   int
		wantW, wantH int
	}{
		{name: "wide source scaled by width", srcW: 400, srcH: 100, maxW: 200, maxH: 200, wantW: 200, wantH: 50},
		{name: "tall source scaled by height", srcW: 100, srcH: 400, maxW: 200, maxH: 200, wantW: 50, wantH: 200},
		{name: "small source is upscaled", srcW: 50, srcH: 50, maxW: 200, maxH: 200, wantW: 200, wantH: 200},
		{name: "square source into wide box", srcW: 300, srcH: 300, maxW: 800, maxH: 600, wantW: 600, wantH: 600},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, tc.srcW, tc.srcH))
			got := resizeImage(src, tc.maxW, tc.maxH)
			if got.Bounds().Dx() != tc.wantW || got.Bounds().Dy() != tc.wantH {
				t.Fatalf("resizeImage(%dx%d, %d, %d) = %dx%d, want %dx%d",
					tc.srcW, tc.srcH, tc.maxW, tc.maxH,
					got.Bounds().Dx(), got.Bounds().Dy(), tc.wantW, tc.wantH)
			}
		})
	}
}
