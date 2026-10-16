package scan

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

type fixture struct {
	worker    *Worker
	mediaRepo *testutil.MockMediaRepository
	storage   *testutil.MemStorage
	pub       *testutil.MockPublisher
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		mediaRepo: testutil.NewMockMediaRepository(),
		storage:   testutil.NewMemStorage(),
		pub:       testutil.NewMockPublisher(),
	}
	f.worker = NewWorker(f.mediaRepo, f.storage, f.pub, closedPortConfig(t))
	return f
}

// closedPortConfig points ClamAV at a TCP port that nobody listens on so the
// dial fails fast and the worker falls back to a clean scan.
func closedPortConfig(t *testing.T) *config.Config {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split reservation address: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("close reservation listener: %v", err)
	}
	return &config.Config{ClamAVHost: "127.0.0.1", ClamAVPort: port}
}

// startFakeClamAV runs an in-process clamd stand-in that consumes the INSTREAM
// protocol and answers with reply.
func startFakeClamAV(t *testing.T, reply string) *config.Config {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		reader := bufio.NewReader(conn)
		if _, err := reader.ReadString('\n'); err != nil {
			return
		}
		var lenBuf [4]byte
		for {
			if _, err := io.ReadFull(reader, lenBuf[:]); err != nil {
				return
			}
			if binary.BigEndian.Uint32(lenBuf[:]) == 0 {
				break
			}
			if _, err := io.CopyN(io.Discard, reader, int64(binary.BigEndian.Uint32(lenBuf[:]))); err != nil {
				return
			}
		}
		_, _ = conn.Write([]byte(reply))
	}()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split fake clamav address: %v", err)
	}
	return &config.Config{ClamAVHost: "127.0.0.1", ClamAVPort: port}
}

func uploadPayload(mediaType, objectKey string) map[string]interface{} {
	return map[string]interface{}{
		"mediaId":     "m-1",
		"userId":      "u-1",
		"fileName":    "photo.jpg",
		"objectKey":   objectKey,
		"size":        int64(11),
		"contentType": "image/jpeg",
		"mediaType":   mediaType,
	}
}

func message(t *testing.T, mediaType, objectKey string) kafkago.Message {
	t.Helper()
	return testutil.EnvelopeMessage(t, kafkaadapter.TopicUploadCompleted, "trace-1", uploadPayload(mediaType, objectKey))
}

func statusSeq(calls []testutil.MediaStatusCall) []domain.MediaStatus {
	out := make([]domain.MediaStatus, 0, len(calls))
	for _, c := range calls {
		out = append(out, c.Next)
	}
	return out
}

func equalStatuses(got []domain.MediaStatus, want ...domain.MediaStatus) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func topicsOf(pub *testutil.MockPublisher) []string {
	out := make([]string, 0, len(pub.Messages))
	for _, m := range pub.Messages {
		out = append(out, m.Topic)
	}
	return out
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestHandleInvalidEnvelope(t *testing.T) {
	f := newFixture(t)
	err := f.worker.Handle(context.Background(), kafkago.Message{Value: []byte("not-json")})
	if !errors.Is(err, kafkaadapter.ErrPermanent) {
		t.Fatalf("Handle = %v, want ErrPermanent", err)
	}
}

func TestHandleInvalidPayload(t *testing.T) {
	f := newFixture(t)
	raw := []byte(`{"eventId":"e-1","eventType":"media.upload.completed","timestamp":1,"payload":"not-an-object"}`)
	err := f.worker.Handle(context.Background(), kafkago.Message{Value: raw})
	if !errors.Is(err, kafkaadapter.ErrPermanent) {
		t.Fatalf("Handle = %v, want ErrPermanent", err)
	}
}

func TestScanSkipsWhenTransitionFails(t *testing.T) {
	f := newFixture(t)
	f.mediaRepo.UpdateStatusErr = errors.New("conditional check failed")

	if err := f.worker.Handle(context.Background(), message(t, "IMAGE", "k")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	if len(f.mediaRepo.UpdateStatuses) != 1 {
		t.Fatalf("UpdateStatuses = %+v, want a single attempt", f.mediaRepo.UpdateStatuses)
	}
	if len(f.pub.Messages) != 0 {
		t.Fatalf("published %d events, want none when the media was already taken", len(f.pub.Messages))
	}
}

func TestScanClamAVUnavailableFallsBackToCleanDocument(t *testing.T) {
	f := newFixture(t)
	if err := f.worker.Handle(context.Background(), message(t, "DOCUMENT", "docs/readme.txt")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	if got := statusSeq(f.mediaRepo.UpdateStatuses); !equalStatuses(got,
		domain.MediaStatusScanning, domain.MediaStatusProcessing, domain.MediaStatusReady) {
		t.Fatalf("status transitions = %v, want SCANNING PROCESSING READY", got)
	}
	want := []string{
		kafkaadapter.TopicScanStarted,
		kafkaadapter.TopicScanCompleted,
		kafkaadapter.TopicMediaReady,
	}
	if !equalStrings(topicsOf(f.pub), want) {
		t.Fatalf("published topics = %v, want %v", topicsOf(f.pub), want)
	}
}

func TestScanCleanImagePublishesScanCompletedOnly(t *testing.T) {
	f := newFixture(t)
	f.worker.cfg = startFakeClamAV(t, "stream: OK\r\n")
	f.storage.SetObject("users/u/media/m/original/photo.jpg", []byte("clean-bytes"))

	if err := f.worker.Handle(context.Background(), message(t, "IMAGE", "users/u/media/m/original/photo.jpg")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	if got := statusSeq(f.mediaRepo.UpdateStatuses); !equalStatuses(got,
		domain.MediaStatusScanning, domain.MediaStatusProcessing) {
		t.Fatalf("status transitions = %v, want SCANNING PROCESSING", got)
	}
	want := []string{kafkaadapter.TopicScanStarted, kafkaadapter.TopicScanCompleted}
	if !equalStrings(topicsOf(f.pub), want) {
		t.Fatalf("published topics = %v, want %v", topicsOf(f.pub), want)
	}

	var completed struct {
		MediaID   string `json:"mediaId"`
		FileName  string `json:"fileName"`
		ObjectKey string `json:"objectKey"`
	}
	msgs := f.pub.ForTopic(kafkaadapter.TopicScanCompleted)
	if len(msgs) != 1 {
		t.Fatalf("scan.completed count = %d, want 1", len(msgs))
	}
	testutil.DecodeEnvelope(t, msgs[0].Payload, &completed)
	if completed.MediaID != "m-1" || completed.ObjectKey != "users/u/media/m/original/photo.jpg" {
		t.Fatalf("unexpected scan.completed payload: %+v", completed)
	}
}

func TestScanInfectedQuarantinesMedia(t *testing.T) {
	f := newFixture(t)
	f.worker.cfg = startFakeClamAV(t, "stream: Eicar-Test-Signature FOUND\r\n")
	f.storage.SetObject("users/u/media/m/original/photo.jpg", []byte("eicar"))

	if err := f.worker.Handle(context.Background(), message(t, "IMAGE", "users/u/media/m/original/photo.jpg")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	if got := statusSeq(f.mediaRepo.UpdateStatuses); !equalStatuses(got,
		domain.MediaStatusScanning, domain.MediaStatusQuarantined) {
		t.Fatalf("status transitions = %v, want SCANNING QUARANTINED", got)
	}
	want := []string{kafkaadapter.TopicScanStarted, kafkaadapter.TopicScanFailed}
	if !equalStrings(topicsOf(f.pub), want) {
		t.Fatalf("published topics = %v, want %v", topicsOf(f.pub), want)
	}

	var failed struct {
		Infected bool   `json:"infected"`
		Threat   string `json:"threat"`
	}
	msgs := f.pub.ForTopic(kafkaadapter.TopicScanFailed)
	if len(msgs) != 1 {
		t.Fatalf("scan.failed count = %d, want 1", len(msgs))
	}
	testutil.DecodeEnvelope(t, msgs[0].Payload, &failed)
	if !failed.Infected || failed.Threat != "Eicar-Test-Signature" {
		t.Fatalf("scan.failed payload = %+v, want infected with threat name", failed)
	}
}

func TestScanInfectedQuarantineTransitionFailureIsTolerated(t *testing.T) {
	f := newFixture(t)
	f.worker.cfg = startFakeClamAV(t, "stream: Eicar-Test-Signature FOUND\r\n")
	f.storage.SetObject("k", []byte("eicar"))
	f.mediaRepo.OnUpdateStatus = func(call testutil.MediaStatusCall) error {
		if call.Next == domain.MediaStatusQuarantined {
			return errors.New("stale status")
		}
		return nil
	}

	if err := f.worker.Handle(context.Background(), message(t, "IMAGE", "k")); err != nil {
		t.Fatalf("Handle = %v, want nil despite the quarantine transition failure", err)
	}
	if got := statusSeq(f.mediaRepo.UpdateStatuses); !equalStatuses(got,
		domain.MediaStatusScanning, domain.MediaStatusQuarantined) {
		t.Fatalf("status transitions = %v, want both attempts", got)
	}
	if f.pub.Count(kafkaadapter.TopicScanFailed) != 1 {
		t.Fatalf("scan.failed count = %d, want 1", f.pub.Count(kafkaadapter.TopicScanFailed))
	}
}

func TestScanInfectedWithBareFoundReply(t *testing.T) {
	f := newFixture(t)
	f.worker.cfg = startFakeClamAV(t, "FOUND\r\n")
	f.storage.SetObject("k", []byte("x"))

	if err := f.worker.Handle(context.Background(), message(t, "IMAGE", "k")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	if got := statusSeq(f.mediaRepo.UpdateStatuses); !equalStatuses(got,
		domain.MediaStatusScanning, domain.MediaStatusQuarantined) {
		t.Fatalf("status transitions = %v, want SCANNING QUARANTINED", got)
	}
	var failed struct {
		Infected bool   `json:"infected"`
		Threat   string `json:"threat"`
	}
	msgs := f.pub.ForTopic(kafkaadapter.TopicScanFailed)
	testutil.DecodeEnvelope(t, msgs[0].Payload, &failed)
	if !failed.Infected || failed.Threat != "" {
		t.Fatalf("scan.failed payload = %+v, want infected with empty threat", failed)
	}
}

func TestScanUnexpectedClamAVReplyTreatedAsClean(t *testing.T) {
	f := newFixture(t)
	f.worker.cfg = startFakeClamAV(t, "some unexpected response\r\n")
	f.storage.SetObject("k", []byte("x"))

	if err := f.worker.Handle(context.Background(), message(t, "IMAGE", "k")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	if got := statusSeq(f.mediaRepo.UpdateStatuses); !equalStatuses(got,
		domain.MediaStatusScanning, domain.MediaStatusProcessing) {
		t.Fatalf("status transitions = %v, want SCANNING PROCESSING", got)
	}
}

func TestScanEngineErrorIsRetriable(t *testing.T) {
	f := newFixture(t)
	f.worker.cfg = startFakeClamAV(t, "stream: OK\r\n")
	f.storage.SetError("GetObject", errors.New("s3 timeout"))

	err := f.worker.Handle(context.Background(), message(t, "IMAGE", "missing-object"))
	if err == nil {
		t.Fatal("Handle = nil, want a retriable scan engine error")
	}
	if !strings.Contains(err.Error(), "scan engine error") {
		t.Fatalf("Handle = %v, want scan engine error", err)
	}
	if errors.Is(err, kafkaadapter.ErrPermanent) {
		t.Fatalf("Handle = %v, must not be permanent", err)
	}
	if got := statusSeq(f.mediaRepo.UpdateStatuses); !equalStatuses(got, domain.MediaStatusScanning) {
		t.Fatalf("status transitions = %v, want only the SCANNING transition", got)
	}
}

func TestScanPublisherErrorsAreSwallowed(t *testing.T) {
	f := newFixture(t)
	f.pub.PublishErr = errors.New("broker down")
	f.storage.SetObject("k", []byte("clean"))

	if err := f.worker.Handle(context.Background(), message(t, "DOCUMENT", "k")); err != nil {
		t.Fatalf("Handle = %v, want nil even when publishing fails", err)
	}
	want := []string{
		kafkaadapter.TopicScanStarted,
		kafkaadapter.TopicScanCompleted,
		kafkaadapter.TopicMediaReady,
	}
	if !equalStrings(topicsOf(f.pub), want) {
		t.Fatalf("published topics = %v, want %v", topicsOf(f.pub), want)
	}
}

func TestScanOtherMediaTypeIsStillProcessed(t *testing.T) {
	f := newFixture(t)
	f.worker.cfg = startFakeClamAV(t, "stream: OK\r\n")
	f.storage.SetObject("k", []byte("archive"))

	if err := f.worker.Handle(context.Background(), message(t, "OTHER", "k")); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	if got := statusSeq(f.mediaRepo.UpdateStatuses); !equalStatuses(got,
		domain.MediaStatusScanning, domain.MediaStatusProcessing, domain.MediaStatusReady) {
		t.Fatalf("status transitions = %v, want SCANNING PROCESSING READY", got)
	}
}

func TestScanProcessingTransitionFailureIsRetriable(t *testing.T) {
	f := newFixture(t)
	f.worker.cfg = startFakeClamAV(t, "stream: OK\r\n")
	f.storage.SetObject("k", []byte("clean"))
	f.mediaRepo.OnUpdateStatus = func(call testutil.MediaStatusCall) error {
		if call.Next == domain.MediaStatusProcessing {
			return errors.New("conditional check failed")
		}
		return nil
	}

	err := f.worker.Handle(context.Background(), message(t, "IMAGE", "k"))
	if err == nil {
		t.Fatal("Handle = nil, want a retriable transition error")
	}
	if !strings.Contains(err.Error(), "transition to PROCESSING") {
		t.Fatalf("Handle = %v, want transition to PROCESSING error", err)
	}
	if f.pub.Count(kafkaadapter.TopicScanStarted) != 1 {
		t.Fatal("scan.started was not published")
	}
	if f.pub.Count(kafkaadapter.TopicScanCompleted) != 0 {
		t.Fatal("scan.completed must not be published after a failed transition")
	}
}

func TestScanDoesNotReadyImageOrVideo(t *testing.T) {
	for _, mediaType := range []string{"IMAGE", "VIDEO"} {
		t.Run(mediaType, func(t *testing.T) {
			f := newFixture(t)
			f.worker.cfg = startFakeClamAV(t, "stream: OK\r\n")
			f.storage.SetObject("k", []byte("x"))

			if err := f.worker.Handle(context.Background(), message(t, mediaType, "k")); err != nil {
				t.Fatalf("Handle = %v, want nil", err)
			}
			if f.pub.Count(kafkaadapter.TopicMediaReady) != 0 {
				t.Fatalf("media.ready published %d times for %s, want 0",
					f.pub.Count(kafkaadapter.TopicMediaReady), mediaType)
			}
		})
	}
}

func TestHandlePayloadTypeMismatchIsPermanent(t *testing.T) {
	f := newFixture(t)
	payload, err := json.Marshal(map[string]interface{}{"mediaId": 12345})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	raw, err := kafkaadapter.MarshalEnvelope("e-1", kafkaadapter.TopicUploadCompleted, "trace-1", json.RawMessage(payload))
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	handleErr := f.worker.Handle(context.Background(), kafkago.Message{Value: raw})
	if !errors.Is(handleErr, kafkaadapter.ErrPermanent) {
		t.Fatalf("Handle = %v, want ErrPermanent", handleErr)
	}
}
