package metadata

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
	"testing"

	kafkago "github.com/segmentio/kafka-go"

	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

const ffprobeOKScript = "@echo off\r\n" +
	`echo {"streams":[{"codec_type":"audio","codec_name":"aac"},{"codec_type":"video","codec_name":"h264","width":640,"height":360}],"format":{"duration":"12.500000","format_name":"mov,mp4"}}` + "\r\n"

const ffprobeBadJSONScript = "@echo off\r\necho this is not json\r\n"

type fixture struct {
	worker      *Worker
	mediaRepo   *testutil.MockMediaRepository
	versionRepo *testutil.MockVersionRepository
	storage     *testutil.MemStorage
	pub         *testutil.MockPublisher
}

func newFixture() *fixture {
	f := &fixture{
		mediaRepo:   testutil.NewMockMediaRepository(),
		versionRepo: testutil.NewMockVersionRepository(),
		storage:     testutil.NewMemStorage(),
		pub:         testutil.NewMockPublisher(),
	}
	f.worker = NewWorker(f.mediaRepo, f.versionRepo, f.storage, f.pub)
	return f
}

// fakeFFProbeDir installs a fake ffprobe executable on PATH.
func fakeFFProbeDir(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ffprobe.bat"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}
	return dir
}

func encodedImage(t *testing.T, encode func(*bytes.Buffer, image.Image) error) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := encode(&buf, img); err != nil {
		t.Fatalf("encode image: %v", err)
	}
	return buf.Bytes()
}

func scanMessage(t *testing.T, mediaType, objectKey string) kafkago.Message {
	t.Helper()
	return testutil.EnvelopeMessage(t, kafkaadapter.TopicScanCompleted, "trace-1", map[string]interface{}{
		"mediaId":     "m-1",
		"userId":      "u-1",
		"objectKey":   objectKey,
		"contentType": "image/png",
		"mediaType":   mediaType,
	})
}

type processingEvent struct {
	MediaID    string `json:"mediaId"`
	WorkerType string `json:"workerType"`
	Width      int32  `json:"width"`
	Height     int32  `json:"height"`
	DurationMS int64  `json:"durationMs"`
	Format     string `json:"format"`
}

func lastProcessingEvent(t *testing.T, pub *testutil.MockPublisher) processingEvent {
	t.Helper()
	msgs := pub.ForTopic(kafkaadapter.TopicProcessingCompleted)
	if len(msgs) == 0 {
		t.Fatal("no processing.completed event published")
	}
	var evt processingEvent
	testutil.DecodeEnvelope(t, msgs[len(msgs)-1].Payload, &evt)
	return evt
}

func onlyVersion(t *testing.T, repo *testutil.MockVersionRepository) *domain.MediaVersion {
	t.Helper()
	if len(repo.Created) != 1 {
		t.Fatalf("created %d versions, want 1", len(repo.Created))
	}
	return repo.Created[0]
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

func TestImageMetadataExtraction(t *testing.T) {
	cases := []struct {
		name       string
		encode     func(*bytes.Buffer, image.Image) error
		wantFormat string
	}{
		{name: "png", encode: func(buf *bytes.Buffer, img image.Image) error { return png.Encode(buf, img) }, wantFormat: "png"},
		{name: "jpeg", encode: func(buf *bytes.Buffer, img image.Image) error { return jpeg.Encode(buf, img, nil) }, wantFormat: "jpeg"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			f.storage.SetObject("img-"+tc.name, encodedImage(t, tc.encode))

			if err := f.worker.Handle(context.Background(), scanMessage(t, "IMAGE", "img-"+tc.name)); err != nil {
				t.Fatalf("Handle = %v, want nil", err)
			}

			v := onlyVersion(t, f.versionRepo)
			if v.VersionType != domain.VersionType("metadata") {
				t.Fatalf("VersionType = %q, want metadata", v.VersionType)
			}
			if v.Width != 4 || v.Height != 3 {
				t.Fatalf("dimensions = %dx%d, want 4x3", v.Width, v.Height)
			}
			if v.Checksum != tc.wantFormat {
				t.Fatalf("Checksum(format) = %q, want %q", v.Checksum, tc.wantFormat)
			}
			if v.ObjectKey != "img-"+tc.name {
				t.Fatalf("ObjectKey = %q, want the original key", v.ObjectKey)
			}

			evt := lastProcessingEvent(t, f.pub)
			if evt.MediaID != "m-1" || evt.WorkerType != "metadata" {
				t.Fatalf("event = %+v, want media m-1 from the metadata worker", evt)
			}
			if evt.Width != 4 || evt.Height != 3 || evt.Format != tc.wantFormat {
				t.Fatalf("event = %+v, want 4x3 %s", evt, tc.wantFormat)
			}
		})
	}
}

func TestImageUndecodableBytesPersistEmptyMeta(t *testing.T) {
	f := newFixture()
	f.storage.SetObject("bad", []byte("definitely not an image"))

	if err := f.worker.Handle(context.Background(), scanMessage(t, "IMAGE", "bad")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	v := onlyVersion(t, f.versionRepo)
	if v.Width != 0 || v.Height != 0 || v.Checksum != "" {
		t.Fatalf("version = %+v, want empty metadata", v)
	}
	evt := lastProcessingEvent(t, f.pub)
	if evt.Width != 0 || evt.Height != 0 || evt.Format != "" {
		t.Fatalf("event = %+v, want empty metadata", evt)
	}
}

func TestImageMissingObjectPersistsEmptyMeta(t *testing.T) {
	f := newFixture()

	if err := f.worker.Handle(context.Background(), scanMessage(t, "IMAGE", "missing")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	v := onlyVersion(t, f.versionRepo)
	if v.Width != 0 || v.Height != 0 {
		t.Fatalf("version = %+v, want empty metadata", v)
	}
}

func TestVideoMetadataFromFakeFFProbe(t *testing.T) {
	f := newFixture()
	t.Setenv("PATH", fakeFFProbeDir(t, ffprobeOKScript))
	f.storage.SetObject("clip.mp4", []byte("fake video bytes"))

	if err := f.worker.Handle(context.Background(), scanMessage(t, "VIDEO", "clip.mp4")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	v := onlyVersion(t, f.versionRepo)
	if v.Width != 640 || v.Height != 360 {
		t.Fatalf("dimensions = %dx%d, want 640x360", v.Width, v.Height)
	}
	if v.DurationMS != 12500 {
		t.Fatalf("DurationMS = %d, want 12500", v.DurationMS)
	}
	if v.Checksum != "mov,mp4/h264" {
		t.Fatalf("Checksum(format) = %q, want \"mov,mp4/h264\"", v.Checksum)
	}
	evt := lastProcessingEvent(t, f.pub)
	if evt.DurationMS != 12500 || evt.Format != "mov,mp4/h264" {
		t.Fatalf("event = %+v, want duration 12500 and format mov,mp4/h264", evt)
	}
}

func TestVideoFFProbeMissingPersistsEmptyMeta(t *testing.T) {
	f := newFixture()
	t.Setenv("PATH", t.TempDir())

	if err := f.worker.Handle(context.Background(), scanMessage(t, "VIDEO", "clip.mp4")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	v := onlyVersion(t, f.versionRepo)
	if v.Width != 0 || v.Height != 0 || v.DurationMS != 0 {
		t.Fatalf("version = %+v, want empty metadata when ffprobe is missing", v)
	}
}

func TestVideoFFProbeBadJSONPersistsEmptyMeta(t *testing.T) {
	f := newFixture()
	t.Setenv("PATH", fakeFFProbeDir(t, ffprobeBadJSONScript))

	if err := f.worker.Handle(context.Background(), scanMessage(t, "VIDEO", "clip.mp4")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	v := onlyVersion(t, f.versionRepo)
	if v.Width != 0 || v.Height != 0 {
		t.Fatalf("version = %+v, want empty metadata on ffprobe output parse failure", v)
	}
}

func TestNonImageVideoPersistsEmptyMeta(t *testing.T) {
	f := newFixture()
	f.storage.SetObject("doc.pdf", []byte("%PDF-1.4"))

	if err := f.worker.Handle(context.Background(), scanMessage(t, "DOCUMENT", "doc.pdf")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	if len(f.storage.Calls()) != 0 {
		t.Fatalf("storage calls = %v, want none without an extractor", f.storage.Calls())
	}
	v := onlyVersion(t, f.versionRepo)
	if v.Width != 0 || v.Height != 0 || v.Checksum != "" {
		t.Fatalf("version = %+v, want empty metadata", v)
	}
	evt := lastProcessingEvent(t, f.pub)
	if evt.WorkerType != "metadata" {
		t.Fatalf("event = %+v, want the metadata worker event", evt)
	}
}

func TestVersionRepoAndPublisherErrorsAreTolerated(t *testing.T) {
	t.Run("version repo error", func(t *testing.T) {
		f := newFixture()
		f.versionRepo.CreateErr = errors.New("dynamo down")
		f.storage.SetObject("img", encodedImage(t, func(buf *bytes.Buffer, img image.Image) error {
			return png.Encode(buf, img)
		}))

		if err := f.worker.Handle(context.Background(), scanMessage(t, "IMAGE", "img")); err != nil {
			t.Fatalf("Handle = %v, want nil despite version repo error", err)
		}
		if len(f.pub.Messages) != 1 {
			t.Fatalf("published %d events, want 1", len(f.pub.Messages))
		}
	})

	t.Run("publisher error", func(t *testing.T) {
		f := newFixture()
		f.pub.PublishErr = errors.New("broker down")
		f.storage.SetObject("img", encodedImage(t, func(buf *bytes.Buffer, img image.Image) error {
			return png.Encode(buf, img)
		}))

		if err := f.worker.Handle(context.Background(), scanMessage(t, "IMAGE", "img")); err != nil {
			t.Fatalf("Handle = %v, want nil despite publisher error", err)
		}
		if len(f.versionRepo.Created) != 1 {
			t.Fatalf("created %d versions, want 1", len(f.versionRepo.Created))
		}
	})
}

func TestParseDurationMS(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"   ", 0},
		{"12.5", 12500},
		{"3", 3000},
		{" 1.25 ", 1250},
		{"0.001", 1},
		{"not-a-number", 0},
	}
	for _, tc := range cases {
		if got := parseDurationMS(tc.in); got != tc.want {
			t.Fatalf("parseDurationMS(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

type stubExtractor struct{ mediaType domain.MediaType }

func (s *stubExtractor) MediaType() domain.MediaType { return s.mediaType }

func (s *stubExtractor) Extract(ctx context.Context, objectKey string) (extractedMeta, error) {
	return extractedMeta{Width: 1, Height: 2}, nil
}

func TestExtractorRegistry(t *testing.T) {
	registry := newExtractorRegistry()
	if _, ok := registry.get(domain.MediaTypeImage); ok {
		t.Fatal("empty registry returned an image extractor")
	}

	registry.register(&stubExtractor{mediaType: domain.MediaTypeImage})
	registry.register(&stubExtractor{mediaType: domain.MediaTypeOther})

	if _, ok := registry.get(domain.MediaTypeImage); !ok {
		t.Fatal("registered image extractor was not found")
	}
	if _, ok := registry.get(domain.MediaTypeOther); !ok {
		t.Fatal("registered other extractor was not found")
	}
	if _, ok := registry.get(domain.MediaTypeVideo); ok {
		t.Fatal("unregistered video extractor was found")
	}

	built := BuildExtractorRegistry(testutil.NewMemStorage())
	for _, mt := range []domain.MediaType{domain.MediaTypeImage, domain.MediaTypeVideo} {
		if _, ok := built.get(mt); !ok {
			t.Fatalf("built registry is missing the %s extractor", mt)
		}
	}
}

func TestImageAndVideoExtractorMediaType(t *testing.T) {
	storage := testutil.NewMemStorage()
	if got := NewImageExtractor(storage).MediaType(); got != domain.MediaTypeImage {
		t.Fatalf("ImageExtractor.MediaType() = %q, want IMAGE", got)
	}
	if got := NewVideoExtractor(storage).MediaType(); got != domain.MediaTypeVideo {
		t.Fatalf("VideoExtractor.MediaType() = %q, want VIDEO", got)
	}
}
