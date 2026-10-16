package delete

import (
	"context"
	"errors"
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
	quotaRepo   *testutil.MockQuotaRepository
	storage     *testutil.MemStorage
	pub         *testutil.MockPublisher
}

func newFixture() *fixture {
	f := &fixture{
		mediaRepo:   testutil.NewMockMediaRepository(),
		versionRepo: testutil.NewMockVersionRepository(),
		quotaRepo:   testutil.NewMockQuotaRepository(),
		storage:     testutil.NewMemStorage(),
		pub:         testutil.NewMockPublisher(),
	}
	f.worker = NewWorker(f.mediaRepo, f.versionRepo, f.quotaRepo, f.storage, f.pub)
	return f
}

func deleteMessage(t *testing.T) kafkago.Message {
	t.Helper()
	return testutil.EnvelopeMessage(t, kafkaadapter.TopicDeleteRequested, "trace-1", map[string]interface{}{
		"mediaId":   "m-1",
		"userId":    "u-1",
		"objectKey": "users/u-1/media/m-1/original/photo.jpg",
	})
}

func seedMediaObjects(f *fixture) {
	f.storage.SetObjectWithContentType("users/u-1/media/m-1/original/photo.jpg", "image/jpeg", []byte("original"))
	f.storage.SetObjectWithContentType("users/u-1/media/m-1/thumbnail/photo.jpg", "image/jpeg", []byte("thumb"))
	f.mediaRepo.Media = testutil.NewMedia("m-1", "u-1")
}

type deletedEvent struct {
	MediaID string `json:"mediaId"`
	UserID  string `json:"userId"`
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
	raw := []byte(`{"eventId":"e-1","eventType":"media.delete.requested","timestamp":1,"payload":"not-an-object"}`)
	err := f.worker.Handle(context.Background(), kafkago.Message{Value: raw})
	if !errors.Is(err, kafkaadapter.ErrPermanent) {
		t.Fatalf("Handle = %v, want ErrPermanent", err)
	}
}

func TestProcessDeleteMediaNotFound(t *testing.T) {
	f := newFixture()

	err := f.worker.Handle(context.Background(), deleteMessage(t))
	if err == nil {
		t.Fatal("Handle = nil, want a media not found error")
	}
	if !errors.Is(err, domain.ErrMediaNotFound) {
		t.Fatalf("Handle = %v, want ErrMediaNotFound", err)
	}
	if len(f.pub.Messages) != 0 {
		t.Fatalf("published %d events, want none when the media does not exist", len(f.pub.Messages))
	}
}

func TestProcessDeleteHappyPath(t *testing.T) {
	f := newFixture()
	seedMediaObjects(f)

	if err := f.worker.Handle(context.Background(), deleteMessage(t)); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}

	if _, found := f.storage.Object("users/u-1/media/m-1/original/photo.jpg"); found {
		t.Fatal("original object was not deleted")
	}
	if _, found := f.storage.Object("users/u-1/media/m-1/thumbnail/photo.jpg"); found {
		t.Fatal("thumbnail object was not deleted")
	}

	listed := false
	for _, c := range f.storage.Calls() {
		if c == "ListObjectsWithPrefix:users/u-1/media/m-1/" {
			listed = true
		}
	}
	if !listed {
		t.Fatalf("storage calls = %v, want a prefix listing for users/u-1/media/m-1/", f.storage.Calls())
	}

	if len(f.mediaRepo.UpdateStatuses) != 1 ||
		f.mediaRepo.UpdateStatuses[0] != (testutil.MediaStatusCall{
			MediaID: "m-1", Current: domain.MediaStatusDeleting, Next: domain.MediaStatusDeleted,
		}) {
		t.Fatalf("UpdateStatuses = %+v, want DELETING->DELETED", f.mediaRepo.UpdateStatuses)
	}

	if len(f.quotaRepo.SubtractedBytes) != 1 || f.quotaRepo.SubtractedBytes[0] != 1024 {
		t.Fatalf("SubtractedBytes = %v, want the media size 1024", f.quotaRepo.SubtractedBytes)
	}

	msgs := f.pub.ForTopic(kafkaadapter.TopicMediaDeleted)
	if len(msgs) != 1 {
		t.Fatalf("media.deleted count = %d, want 1", len(msgs))
	}
	var evt deletedEvent
	testutil.DecodeEnvelope(t, msgs[0].Payload, &evt)
	if evt.MediaID != "m-1" || evt.UserID != "u-1" {
		t.Fatalf("media.deleted payload = %+v, want media m-1 of user u-1", evt)
	}
	if msgs[0].Key != "m-1" || msgs[0].TraceID != "trace-1" {
		t.Fatalf("unexpected publish metadata: %+v", msgs[0])
	}
}

func TestProcessDeleteListFailure(t *testing.T) {
	f := newFixture()
	f.mediaRepo.Media = testutil.NewMedia("m-1", "u-1")
	f.storage.SetError("ListObjectsWithPrefix", errors.New("s3 unavailable"))

	err := f.worker.Handle(context.Background(), deleteMessage(t))
	if err == nil {
		t.Fatal("Handle = nil, want a list error")
	}
	if !strings.Contains(err.Error(), "list objects for deletion prefix") {
		t.Fatalf("Handle = %v, want list objects for deletion prefix error", err)
	}
	if len(f.mediaRepo.UpdateStatuses) != 0 {
		t.Fatalf("UpdateStatuses = %+v, want none after a list failure", f.mediaRepo.UpdateStatuses)
	}
}

func TestProcessDeleteBatchDeleteFailure(t *testing.T) {
	f := newFixture()
	seedMediaObjects(f)
	f.storage.SetError("DeleteObjects", errors.New("s3 unavailable"))

	err := f.worker.Handle(context.Background(), deleteMessage(t))
	if err == nil {
		t.Fatal("Handle = nil, want a batch delete error")
	}
	if !strings.Contains(err.Error(), "S3 batch delete") {
		t.Fatalf("Handle = %v, want S3 batch delete error", err)
	}
	if len(f.mediaRepo.UpdateStatuses) != 0 {
		t.Fatalf("UpdateStatuses = %+v, want none after a delete failure", f.mediaRepo.UpdateStatuses)
	}
}

func TestProcessDeleteStatusUpdateFailureIsTolerated(t *testing.T) {
	f := newFixture()
	seedMediaObjects(f)
	f.mediaRepo.UpdateStatusErr = errors.New("stale status")

	if err := f.worker.Handle(context.Background(), deleteMessage(t)); err != nil {
		t.Fatalf("Handle = %v, want nil despite the status update failure", err)
	}
	if f.pub.Count(kafkaadapter.TopicMediaDeleted) != 1 {
		t.Fatal("media.deleted was not published")
	}
}

func TestProcessDeleteQuotaErrorsAreTolerated(t *testing.T) {
	t.Run("subtract failure", func(t *testing.T) {
		f := newFixture()
		seedMediaObjects(f)
		f.quotaRepo.SubtractBytesErr = errors.New("dynamo down")

		if err := f.worker.Handle(context.Background(), deleteMessage(t)); err != nil {
			t.Fatalf("Handle = %v, want nil despite the quota failure", err)
		}
		if f.pub.Count(kafkaadapter.TopicMediaDeleted) != 1 {
			t.Fatal("media.deleted was not published")
		}
	})

	t.Run("usage lookup failure", func(t *testing.T) {
		f := newFixture()
		seedMediaObjects(f)
		f.quotaRepo.GetErr = errors.New("dynamo down")

		if err := f.worker.Handle(context.Background(), deleteMessage(t)); err != nil {
			t.Fatalf("Handle = %v, want nil despite the usage lookup failure", err)
		}
		if f.pub.Count(kafkaadapter.TopicMediaDeleted) != 1 {
			t.Fatal("media.deleted was not published")
		}
	})
}

func TestProcessDeletePublishFailureIsTolerated(t *testing.T) {
	f := newFixture()
	seedMediaObjects(f)
	f.pub.PublishErr = errors.New("broker down")

	if err := f.worker.Handle(context.Background(), deleteMessage(t)); err != nil {
		t.Fatalf("Handle = %v, want nil despite the publish failure", err)
	}
	if len(f.mediaRepo.UpdateStatuses) != 1 {
		t.Fatalf("UpdateStatuses = %+v, want DELETING->DELETED", f.mediaRepo.UpdateStatuses)
	}
}

func TestProcessDeleteWithoutObjectsUnderPrefix(t *testing.T) {
	f := newFixture()
	f.mediaRepo.Media = testutil.NewMedia("m-1", "u-1")

	if err := f.worker.Handle(context.Background(), deleteMessage(t)); err != nil {
		t.Fatalf("Handle = %v, want nil", err)
	}
	for _, c := range f.storage.Calls() {
		if strings.HasPrefix(c, "DeleteObjects") {
			t.Fatalf("storage calls = %v, want no DeleteObjects without keys", f.storage.Calls())
		}
	}
	if f.pub.Count(kafkaadapter.TopicMediaDeleted) != 1 {
		t.Fatal("media.deleted was not published")
	}
}

func TestBuildPrefix(t *testing.T) {
	if got := buildPrefix("u-1", "m-1"); got != "users/u-1/media/m-1/" {
		t.Fatalf("buildPrefix = %q, want users/u-1/media/m-1/", got)
	}
}
