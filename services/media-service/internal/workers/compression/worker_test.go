package compression

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"math/rand"
	"strings"
	"testing"

	kafkago "github.com/segmentio/kafka-go"

	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

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

func compressMessage(t *testing.T, contentType string) kafkago.Message {
	t.Helper()
	return testutil.EnvelopeMessage(t, kafkaadapter.TopicScanCompleted, "trace-1", map[string]interface{}{
		"mediaId":     "m-1",
		"userId":      "u-1",
		"objectKey":   "users/u-1/media/m-1/original/notes.txt",
		"contentType": contentType,
		"mediaType":   "DOCUMENT",
	})
}

type processingEvent struct {
	MediaID      string `json:"mediaId"`
	WorkerType   string `json:"workerType"`
	OriginalSize int    `json:"originalSize"`
	NewSize      int    `json:"newSize"`
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

func incompressibleBytes(n int) []byte {
	out := make([]byte, n)
	rng := rand.New(rand.NewSource(42))
	if _, err := rng.Read(out); err != nil {
		panic(err)
	}
	return out
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

func TestHandleSkipsNonCompressibleTypes(t *testing.T) {
	for _, contentType := range []string{"image/png", "image/jpeg", "video/mp4", "application/zip", "audio/mpeg"} {
		t.Run(contentType, func(t *testing.T) {
			f := newFixture()
			if err := f.worker.Handle(context.Background(), compressMessage(t, contentType)); err != nil {
				t.Fatalf("Handle = %v, want nil", err)
			}
			if len(f.storage.Calls()) != 0 {
				t.Fatalf("storage calls = %v, want none for %s", f.storage.Calls(), contentType)
			}
			if len(f.pub.Messages) != 0 {
				t.Fatalf("published %d events, want none for %s", len(f.pub.Messages), contentType)
			}
		})
	}
}

func TestCompressHappyPath(t *testing.T) {
	f := newFixture()
	original := bytes.Repeat([]byte("hello compressible world "), 4000)
	f.storage.SetObjectWithContentType("users/u-1/media/m-1/original/notes.txt", "text/plain", original)

	if err := f.worker.Handle(context.Background(), compressMessage(t, "text/plain")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	compressedKey := "users/u-1/media/m-1/original/notes.txt.gz"
	data, found := f.storage.Object(compressedKey)
	if !found {
		t.Fatalf("compressed object %q missing in storage", compressedKey)
	}
	if len(data) >= len(original) {
		t.Fatalf("compressed size %d is not smaller than original %d", len(data), len(original))
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	roundTrip, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}
	if !bytes.Equal(roundTrip, original) {
		t.Fatal("decompressed bytes differ from the original payload")
	}

	if len(f.versionRepo.Created) != 1 {
		t.Fatalf("created %d versions, want 1", len(f.versionRepo.Created))
	}
	v := f.versionRepo.Created[0]
	if v.VersionType != domain.VersionTypeCompressed {
		t.Fatalf("VersionType = %q, want %q", v.VersionType, domain.VersionTypeCompressed)
	}
	if v.ObjectKey != compressedKey || v.ContentType != "application/gzip" {
		t.Fatalf("version = %+v, want key %q with content type application/gzip", v, compressedKey)
	}
	if v.Size != int64(len(data)) {
		t.Fatalf("version Size = %d, want %d", v.Size, len(data))
	}

	evt := lastProcessingEvent(t, f.pub)
	if evt.MediaID != "m-1" || evt.WorkerType != "compression" {
		t.Fatalf("event = %+v, want media m-1 from the compression worker", evt)
	}
	if evt.OriginalSize != len(original) || evt.NewSize != len(data) {
		t.Fatalf("event = %+v, want original %d and new %d bytes", evt, len(original), len(data))
	}
}

func TestCompressSkipsWhenNoReduction(t *testing.T) {
	f := newFixture()
	original := incompressibleBytes(4096)
	f.storage.SetObjectWithContentType("users/u-1/media/m-1/original/notes.txt", "application/pdf", original)

	if err := f.worker.Handle(context.Background(), compressMessage(t, "application/pdf")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	if len(f.versionRepo.Created) != 0 {
		t.Fatalf("created %d versions, want none when gzip does not shrink the file", len(f.versionRepo.Created))
	}
	if len(f.pub.Messages) != 0 {
		t.Fatalf("published %d events, want none when gzip does not shrink the file", len(f.pub.Messages))
	}
	if _, found := f.storage.Object("users/u-1/media/m-1/original/notes.txt.gz"); found {
		t.Fatal("compressed object was stored despite no size reduction")
	}
}

func TestCompressDownloadFailure(t *testing.T) {
	f := newFixture()

	err := f.worker.Handle(context.Background(), compressMessage(t, "text/plain"))
	if err == nil {
		t.Fatal("Handle = nil, want a download error")
	}
	if !strings.Contains(err.Error(), "download original for compression") {
		t.Fatalf("Handle = %v, want download original for compression error", err)
	}
}

func TestCompressUploadFailure(t *testing.T) {
	f := newFixture()
	f.storage.SetObject("users/u-1/media/m-1/original/notes.txt", bytes.Repeat([]byte("aaaaaaaa"), 1000))
	f.storage.SetError("PutObject", errors.New("s3 unavailable"))

	err := f.worker.Handle(context.Background(), compressMessage(t, "text/plain"))
	if err == nil {
		t.Fatal("Handle = nil, want an upload error")
	}
	if !strings.Contains(err.Error(), "upload compressed version") {
		t.Fatalf("Handle = %v, want upload compressed version error", err)
	}
}

func TestCompressVersionRepoFailureStillPublishes(t *testing.T) {
	f := newFixture()
	f.storage.SetObject("users/u-1/media/m-1/original/notes.txt", bytes.Repeat([]byte("bbbb"), 5000))
	f.versionRepo.CreateErr = errors.New("dynamo down")

	if err := f.worker.Handle(context.Background(), compressMessage(t, "text/plain")); err != nil {
		t.Fatalf("Handle = %v, want nil despite the version repo error", err)
	}
	if f.pub.Count(kafkaadapter.TopicProcessingCompleted) != 1 {
		t.Fatal("processing.completed was not published")
	}
}

func TestCompressPublisherErrorsAreSwallowed(t *testing.T) {
	f := newFixture()
	f.pub.PublishErr = errors.New("broker down")
	f.storage.SetObject("users/u-1/media/m-1/original/notes.txt", bytes.Repeat([]byte("cccc"), 5000))

	if err := f.worker.Handle(context.Background(), compressMessage(t, "text/plain")); err != nil {
		t.Fatalf("Handle = %v, want nil despite the publisher error", err)
	}
	if len(f.versionRepo.Created) != 1 {
		t.Fatalf("created %d versions, want 1", len(f.versionRepo.Created))
	}
}

func TestIsCompressible(t *testing.T) {
	cases := []struct {
		contentType string
		want        bool
	}{
		{"text/plain", true},
		{"text/plain; charset=utf-8", true},
		{"text/html; charset=utf-8", true},
		{"text/csv", true},
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"application/xml", true},
		{"application/javascript", true},
		{"application/x-www-form-urlencoded", true},
		{"application/pdf", true},
		{"application/rtf", true},
		{"application/csv", true},
		{"image/png", false},
		{"image/jpeg", false},
		{"video/mp4", false},
		{"audio/mpeg", false},
		{"application/zip", false},
		{"application/octet-stream", false},
		{"text", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isCompressible(tc.contentType); got != tc.want {
			t.Fatalf("isCompressible(%q) = %v, want %v", tc.contentType, got, tc.want)
		}
	}
}

func TestCompressedObjectKey(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"users/u/media/m/original/notes.txt", "users/u/media/m/original/notes.txt.gz"},
		{"users/u/media/m/original/notes.txt/", "users/u/media/m/original/notes.txt.gz"},
	}
	for _, tc := range cases {
		if got := compressedObjectKey(tc.in); got != tc.want {
			t.Fatalf("compressedObjectKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
