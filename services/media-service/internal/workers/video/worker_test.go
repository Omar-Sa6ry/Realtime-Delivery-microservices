package video

import (
	"context"
	"errors"
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

// fakeFFmpegScript writes the requested output file, a sibling .ts segment and
// a sibling directory so every branch of the HLS upload loop is exercised.
const fakeFFmpegScript = "@echo off\r\n" +
	"setlocal\r\nset \"target=\"\r\n:loop\r\nif \"%~1\"==\"\" goto done\r\nset \"target=%~1\"\r\nshift\r\ngoto loop\r\n:done\r\n" +
	"echo fake-media-output>\"%target%\"\r\n" +
	"echo fake-segment>\"%target%.ts\"\r\n" +
	"md \"%target%.d\" 2>nul\r\n" +
	"exit /b 0\r\n"

type fixture struct {
	worker      *Worker
	mediaRepo   *testutil.MockMediaRepository
	versionRepo *testutil.MockVersionRepository
	storage     *testutil.MemStorage
	pub         *testutil.MockPublisher
	realtime    *testutil.MockRealtimePublisher
}

func newFixture() *fixture {
	f := &fixture{
		mediaRepo:   testutil.NewMockMediaRepository(),
		versionRepo: testutil.NewMockVersionRepository(),
		storage:     testutil.NewMemStorage(),
		pub:         testutil.NewMockPublisher(),
		realtime:    &testutil.MockRealtimePublisher{},
	}
	f.worker = NewWorker(f.mediaRepo, f.versionRepo, f.storage, f.pub, f.realtime)
	return f
}

func hideFFmpeg(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func installFakeFFmpeg(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg.bat"), []byte(fakeFFmpegScript), 0o755); err != nil {
		t.Fatalf("write fake ffmpeg: %v", err)
	}
	t.Setenv("PATH", dir)
}

func videoMessage(t *testing.T, mediaType, objectKey string, extra map[string]interface{}) kafkago.Message {
	t.Helper()
	payload := map[string]interface{}{
		"mediaId":     "m-1",
		"userId":      "u-1",
		"fileName":    "clip.mp4",
		"objectKey":   objectKey,
		"size":        int64(4096),
		"contentType": "video/mp4",
		"mediaType":   mediaType,
	}
	for k, v := range extra {
		payload[k] = v
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

func TestHandleSkipsNonVideo(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)

	if err := f.worker.Handle(context.Background(), videoMessage(t, "IMAGE", "photo.png", nil)); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	if len(f.storage.Calls()) != 0 {
		t.Fatalf("storage calls = %v, want none for non-video media", f.storage.Calls())
	}
	if len(f.pub.Messages) != 0 {
		t.Fatalf("published %d events, want none for non-video media", len(f.pub.Messages))
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

func TestProcessVideoDownloadFailure(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)

	err := f.worker.Handle(context.Background(), videoMessage(t, "VIDEO", "missing.mp4", nil))
	if err == nil {
		t.Fatal("Handle = nil, want a download error")
	}
	if !strings.Contains(err.Error(), "download original video") {
		t.Fatalf("Handle = %v, want download original video error", err)
	}
}

func TestProcessVideoWithoutFFmpegStillBecomesReady(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)
	f.storage.SetObjectWithContentType("users/u-1/media/m-1/original/clip.mp4", "video/mp4", []byte("fake video bytes"))

	msg := videoMessage(t, "VIDEO", "users/u-1/media/m-1/original/clip.mp4", nil)
	if err := f.worker.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil even without ffmpeg", err)
	}

	if len(f.versionRepo.Created) != 0 {
		t.Fatalf("created %d versions, want none when every profile fails", len(f.versionRepo.Created))
	}
	if len(f.mediaRepo.UpdateStatuses) < 2 {
		t.Fatalf("UpdateStatuses = %+v, want at least PROCESSING and READY", f.mediaRepo.UpdateStatuses)
	}
	last := f.mediaRepo.UpdateStatuses[len(f.mediaRepo.UpdateStatuses)-1]
	if last.Current != domain.MediaStatusProcessing || last.Next != domain.MediaStatusReady {
		t.Fatalf("last transition = %+v, want PROCESSING->READY", last)
	}
	if f.pub.Count(kafkaadapter.TopicMediaReady) != 1 {
		t.Fatalf("media.ready count = %d, want 1", f.pub.Count(kafkaadapter.TopicMediaReady))
	}
	if f.realtime.Count(sharednats.RealtimeMediaReady) != 1 {
		t.Fatalf("realtime ready events = %d, want 1", f.realtime.Count(sharednats.RealtimeMediaReady))
	}
	if f.realtime.Count(sharednats.RealtimeMediaProcessingProgress) < 6 {
		t.Fatalf("realtime progress events = %d, want the full progress sequence", f.realtime.Count(sharednats.RealtimeMediaProcessingProgress))
	}
}

func TestProcessVideoWithFakeFFmpegProducesRenditions(t *testing.T) {
	f := newFixture()
	installFakeFFmpeg(t)
	f.storage.SetObjectWithContentType("users/u-1/media/m-1/original/clip.mp4", "video/mp4", []byte("fake video bytes"))

	msg := videoMessage(t, "VIDEO", "users/u-1/media/m-1/original/clip.mp4", nil)
	if err := f.worker.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	versions := versionsByType(f.versionRepo)
	wantTypes := []domain.VersionType{
		domain.VersionTypeThumbnail,
		domain.VersionType360p,
		domain.VersionType720p,
		domain.VersionType1080p,
	}
	for _, vt := range wantTypes {
		v, ok := versions[vt]
		if !ok {
			t.Fatalf("versions %v, want a %s rendition", versions, vt)
		}
		if data, found := f.storage.Object(v.ObjectKey); !found || len(data) == 0 {
			t.Fatalf("%s object %q missing in storage", vt, v.ObjectKey)
		}
	}
	if got := versions[domain.VersionTypeThumbnail].ObjectKey; got != "users/u-1/media/m-1/thumbnail/clip.jpg" {
		t.Fatalf("thumbnail key = %q", got)
	}
	if got := versions[domain.VersionType360p].ObjectKey; got != "users/u-1/media/m-1/360p/clip.mp4" {
		t.Fatalf("360p key = %q", got)
	}

	hlsPrefix := "users/u-1/media/m-1/hls/"
	hlsKeys, err := f.storage.ListObjectsWithPrefix(context.Background(), hlsPrefix)
	if err != nil {
		t.Fatalf("list HLS objects: %v", err)
	}
	if len(hlsKeys) != 2 {
		t.Fatalf("HLS objects = %v, want the playlist and the segment", hlsKeys)
	}
	playlistFound, segmentFound := false, false
	for _, k := range hlsKeys {
		switch {
		case strings.HasSuffix(k, ".m3u8"):
			playlistFound = true
		case strings.HasSuffix(k, ".ts"):
			segmentFound = true
		}
	}
	if !playlistFound || !segmentFound {
		t.Fatalf("HLS objects = %v, want both a playlist and a segment", hlsKeys)
	}
}

func TestProcessVideoTransitionFailure(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)
	f.storage.SetObject("clip.mp4", []byte("x"))
	f.mediaRepo.UpdateStatusErr = errors.New("stale status")

	err := f.worker.Handle(context.Background(), videoMessage(t, "VIDEO", "clip.mp4", nil))
	if err == nil {
		t.Fatal("Handle = nil, want a transition error")
	}
	if !strings.Contains(err.Error(), "transition to READY") {
		t.Fatalf("Handle = %v, want transition to READY error", err)
	}
}

func TestProcessVideoFallsBackToRepoMetadata(t *testing.T) {
	t.Run("payload has no file name", func(t *testing.T) {
		f := newFixture()
		hideFFmpeg(t)
		f.storage.SetObject("clip.mp4", []byte("x"))
		f.mediaRepo.Media = testutil.NewMedia("m-1", "u-1")

		msg := videoMessage(t, "VIDEO", "clip.mp4", map[string]interface{}{"fileName": "", "size": int64(0)})
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
		f.storage.SetObject("clip.mp4", []byte("x"))
		f.mediaRepo.GetErr = errors.New("dynamo down")

		msg := videoMessage(t, "VIDEO", "clip.mp4", map[string]interface{}{"fileName": "", "size": int64(0)})
		if err := f.worker.Handle(context.Background(), msg); err != nil {
			t.Fatalf("Handle = %v, want nil", err)
		}

		ready := mediaReadyPayload(t, f.pub)
		if ready["fileName"] != "" {
			t.Fatalf("fileName = %v, want empty when the media record cannot be fetched", ready["fileName"])
		}
	})
}

func TestProcessVideoPublisherErrorsAreSwallowed(t *testing.T) {
	f := newFixture()
	hideFFmpeg(t)
	f.pub.PublishErr = errors.New("broker down")
	f.storage.SetObject("clip.mp4", []byte("x"))

	if err := f.worker.Handle(context.Background(), videoMessage(t, "VIDEO", "clip.mp4", nil)); err != nil {
		t.Fatalf("Handle = %v, want nil despite the publisher error", err)
	}
}

func TestProcessVideoTempDirFailure(t *testing.T) {
	f := newFixture()
	missing := filepath.Join(t.TempDir(), "missing-temp-root")
	t.Setenv("TMP", missing)
	t.Setenv("TEMP", missing)
	f.storage.SetObject("clip.mp4", []byte("x"))

	err := f.worker.Handle(context.Background(), videoMessage(t, "VIDEO", "clip.mp4", nil))
	if err == nil {
		t.Fatal("Handle = nil, want a temp dir error")
	}
	if !strings.Contains(err.Error(), "create temp dir") {
		t.Fatalf("Handle = %v, want create temp dir error", err)
	}
}

func TestProcessVideoPutObjectFailuresAreNonFatal(t *testing.T) {
	f := newFixture()
	installFakeFFmpeg(t)
	f.storage.SetObjectWithContentType("users/u-1/media/m-1/original/clip.mp4", "video/mp4", []byte("fake video bytes"))
	f.storage.SetError("PutObject", errors.New("s3 unavailable"))

	msg := videoMessage(t, "VIDEO", "users/u-1/media/m-1/original/clip.mp4", nil)
	if err := f.worker.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle = %v, want nil despite PutObject failures", err)
	}
	if len(f.versionRepo.Created) != 0 {
		t.Fatalf("created %d versions, want none when uploads fail", len(f.versionRepo.Created))
	}
	if f.pub.Count(kafkaadapter.TopicMediaReady) != 1 {
		t.Fatal("media.ready was not published")
	}
	if f.realtime.Count(sharednats.RealtimeMediaReady) != 1 {
		t.Fatal("realtime media.ready was not published")
	}
}

func TestRunFFmpegReportsFailures(t *testing.T) {
	hideFFmpeg(t)
	err := runFFmpeg(context.Background(), []string{"-y", "-i", "in.mp4", "out.mp4"})
	if err == nil {
		t.Fatal("runFFmpeg = nil, want an error when ffmpeg is missing")
	}
	if !strings.Contains(err.Error(), "ffmpeg exited with error") {
		t.Fatalf("runFFmpeg = %v, want ffmpeg exited with error", err)
	}
}

func TestStandardProfiles(t *testing.T) {
	if len(standardProfiles) != 3 {
		t.Fatalf("standardProfiles has %d entries, want 3", len(standardProfiles))
	}
	want := map[string]int{"360p": 360, "720p": 720, "1080p": 1080}
	for _, p := range standardProfiles {
		if want[p.Name] != p.Height {
			t.Fatalf("profile %s has height %d, want %d", p.Name, p.Height, want[p.Name])
		}
		if p.Bitrate == "" || p.VersionType == "" {
			t.Fatalf("profile %+v, want a bitrate and version type", p)
		}
	}
}

func TestBuildVersionKey(t *testing.T) {
	cases := []struct {
		name        string
		originalKey string
		versionName string
		ext         string
		want        string
	}{
		{
			name:        "original segment",
			originalKey: "users/u/media/m/original/clip.mp4",
			versionName: "720p",
			ext:         ".mp4",
			want:        "users/u/media/m/720p/clip.mp4",
		},
		{
			name:        "originals segment",
			originalKey: "users/u/media/m/originals/clip.mp4",
			versionName: "thumbnail",
			ext:         ".jpg",
			want:        "users/u/media/m/thumbnail/clip.jpg",
		},
		{
			name:        "nested directory keeps path",
			originalKey: "users/u/media/m/original/nested/dir/clip.mp4",
			versionName: "360p",
			ext:         ".mp4",
			want:        "users/u/media/m/360p/nested/dir/clip.mp4",
		},
		{
			name:        "no original segment falls back to suffix",
			originalKey: "flat/clip.mp4",
			versionName: "hls",
			ext:         "",
			want:        "flat/clip.mp4_hls",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildVersionKey(tc.originalKey, tc.versionName, tc.ext); got != tc.want {
				t.Fatalf("buildVersionKey(%q, %q, %q) = %q, want %q",
					tc.originalKey, tc.versionName, tc.ext, got, tc.want)
			}
		})
	}
}

func TestBuildHLSBaseKey(t *testing.T) {
	if got := buildHLSBaseKey("users/u/media/m/original/clip.mp4"); got != "users/u/media/m/hls/clip" {
		t.Fatalf("buildHLSBaseKey = %q, want users/u/media/m/hls/clip", got)
	}
	if got := buildHLSBaseKey("flat/clip.mp4"); got != "flat/clip.mp4_hls" {
		t.Fatalf("buildHLSBaseKey = %q, want the suffix fallback", got)
	}
}
