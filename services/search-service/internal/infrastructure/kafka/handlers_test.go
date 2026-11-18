package kafka

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
	kafkago "github.com/segmentio/kafka-go"
)

func newTestSvc(t *testing.T) (*indexing.Service, *testutil.FakeRepository) {
	t.Helper()
	repo := testutil.NewFakeRepository()
	return indexing.NewService(repo), repo
}

func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestRegistry_HasAllEventTypes(t *testing.T) {
	svc, _ := newTestSvc(t)
	reg := BuildRegistry(svc)
	expected := []string{
		string(events.DeliveryCreated),
		string(events.DeliveryDriverAssigned),
		string(events.DeliveryDriverAccepted),
		string(events.DeliveryPickedUp),
		string(events.DeliveryInTransit),
		string(events.DeliveryCompleted),
		string(events.DeliveryCancelled),
		string(events.DeliveryDeleted),
		string(events.DriverCreated),
		string(events.DriverUpdated),
		string(events.DriverDeleted),
		string(events.MediaUploadCreated),
		string(events.MediaUploadCompleted),
		string(events.MediaReady),
		string(events.MediaDeleted),
		string(events.UserCreated),
		string(events.UserUpdated),
		string(events.UserDeleted),
	}
	for _, et := range expected {
		if _, ok := reg.get(et); !ok {
			t.Errorf("missing handler for %q", et)
		}
	}
	if _, ok := reg.get("unknown.event"); ok {
		t.Error("unexpected handler for unknown event")
	}
}

func TestHandlerRegistry_RegisterAndGet(t *testing.T) {
	svc, _ := newTestSvc(t)
	reg := newHandlerRegistry()
	h := NewDeliveryCreatedHandler(svc)
	reg.register(h)
	got, ok := reg.get(string(events.DeliveryCreated))
	if !ok || got != h {
		t.Fatal("register/get mismatch")
	}
	reg.registerForTypes(h, []string{"a", "b"})
	if _, ok := reg.get("a"); !ok {
		t.Error("registerForTypes a")
	}
	if _, ok := reg.get("b"); !ok {
		t.Error("registerForTypes b")
	}
}

func TestDeliveryCreatedHandler(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewDeliveryCreatedHandler(svc)
	if h.EventType() != string(events.DeliveryCreated) {
		t.Errorf("eventType = %q", h.EventType())
	}

	now := time.Now().UTC()
	payload := events.DeliveryCreatedPayload{
		DeliveryID:    "d1",
		CustomerID:    "c1",
		DriverID:      "dr1",
		Status:        "PENDING",
		Pickup:        events.DeliveryAddress{City: "Cairo", Country: "EG", Location: events.DeliveryLocation{Lat: 1, Lon: 2}},
		Dropoff:       events.DeliveryAddress{City: "Giza", Country: "EG", Location: events.DeliveryLocation{Lat: 3, Lon: 4}},
		CreatedAt:     now,
		UpdatedAt:     now,
		SourceVersion: 1,
	}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), "trace"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(repo.UpsertDeliveries) != 1 {
		t.Fatalf("upserts = %d", len(repo.UpsertDeliveries))
	}
	d := repo.UpsertDeliveries[0]
	if d.DeliveryID != "d1" || d.Pickup.City != "Cairo" || d.Dropoff.Location.Lon != 4 {
		t.Errorf("doc = %+v", d)
	}

	if err := h.Handle(context.Background(), json.RawMessage("bad"), "trace"); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestDeliveryUpdatedHandler(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewDeliveryUpdatedHandler(svc)
	payload := events.DeliveryUpdatedPayload{
		DeliveryID: "d1", CustomerID: "c1", DriverID: "dr1",
		Status: "IN_TRANSIT", UpdatedAt: time.Now(), SourceVersion: 2,
	}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if repo.UpsertDeliveries[0].Status != "IN_TRANSIT" {
		t.Errorf("status = %q", repo.UpsertDeliveries[0].Status)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestDeliveryDeletedHandler(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewDeliveryDeletedHandler(svc)
	payload := events.DeliveryDeletedPayload{DeliveryID: "d1"}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedDeliveries[0] != "d1" {
		t.Errorf("deleted = %v", repo.DeletedDeliveries)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestDriverCreatedHandler_WithLocation(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewDriverCreatedHandler(svc)
	payload := events.DriverCreatedPayload{
		DriverID: "d1", Name: "Ali", Status: "AVAILABLE", VehicleType: "CAR",
		Rating: 4.5, Location: &events.DriverGeoPoint{Lat: 1, Lon: 2},
		UpdatedAt: time.Now(), SourceVersion: 1,
	}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	d := repo.UpsertDrivers[0]
	if d.Location == nil || d.Location.Lat != 1 {
		t.Errorf("doc = %+v", d)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestDriverCreatedHandler_NilLocation(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewDriverCreatedHandler(svc)
	payload := events.DriverCreatedPayload{DriverID: "d1", Name: "Ali"}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertDrivers[0].Location != nil {
		t.Error("expected nil location")
	}
}

func TestDriverDeletedHandler(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewDriverDeletedHandler(svc)
	if err := h.Handle(context.Background(), mustMarshal(t, events.DriverDeletedPayload{DriverID: "d1"}), ""); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedDrivers[0] != "d1" {
		t.Errorf("deleted = %v", repo.DeletedDrivers)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestMediaUploadCreatedHandler(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewMediaUploadCreatedHandler(svc)
	payload := events.MediaUploadCreatedPayload{
		MediaID: "m1", UserID: "u1", FileName: "a.png",
		ContentType: "image/png", MediaType: "IMAGE", Size: 100,
	}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	d := repo.UpsertMediaDocs[0]
	if d.Status != "UPLOADING" || d.OwnerID != "u1" || d.SourceVersion != 1 {
		t.Errorf("doc = %+v", d)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestMediaUploadCompletedHandler(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewMediaUploadCompletedHandler(svc)
	payload := events.MediaUploadCompletedPayload{
		MediaID: "m1", UserID: "u1", FileName: "a.png",
		ContentType: "image/png", MediaType: "IMAGE", Size: 100,
	}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertMediaDocs[0].Status != "UPLOADED" {
		t.Errorf("status = %q", repo.UpsertMediaDocs[0].Status)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestMediaReadyHandler_WithSize(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewMediaReadyHandler(svc)
	payload := events.MediaReadyPayload{
		MediaID: "m1", UserID: "u1", FileName: "a.png",
		Size: 500, MediaType: "IMAGE", ContentType: "image/png",
	}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	d := repo.UpsertMediaDocs[0]
	if d.Status != "READY" || d.Size != 500 {
		t.Errorf("doc = %+v", d)
	}
}

func TestMediaReadyHandler_SizeFallsBackToVersions(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewMediaReadyHandler(svc)
	payload := events.MediaReadyPayload{
		MediaID: "m1", UserID: "u1", FileName: "a.png",
		Size:     0,
		Versions: []events.MediaVersionInfo{{Size: 999}},
	}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertMediaDocs[0].Size != 999 {
		t.Errorf("size = %d", repo.UpsertMediaDocs[0].Size)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestMediaDeletedHandler(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewMediaDeletedHandler(svc)
	if err := h.Handle(context.Background(), mustMarshal(t, events.MediaDeletedPayload{MediaID: "m1"}), ""); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedMedia[0] != "m1" {
		t.Errorf("deleted = %v", repo.DeletedMedia)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestUserCreatedHandler(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewUserCreatedHandler(svc)
	payload := events.UserCreatedPayload{
		UserID: "u1", Email: "a@b.c", FirstName: "A", LastName: "B", Role: "user",
	}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	d := repo.UpsertUsers[0]
	if !d.IsActive || d.CreatedAt.IsZero() {
		t.Errorf("doc = %+v", d)
	}

	payload.CreatedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	if !repo.UpsertUsers[1].CreatedAt.Equal(payload.CreatedAt) {
		t.Errorf("createdAt = %v", repo.UpsertUsers[1].CreatedAt)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestUserUpdatedHandler(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewUserUpdatedHandler(svc)
	payload := events.UserUpdatedPayload{
		UserID: "u1", Email: "a@b.c", FirstName: "A", LastName: "B",
		Role: "user", IsActive: true,
	}
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	if !repo.UpsertUsers[0].IsActive || repo.UpsertUsers[0].CreatedAt.IsZero() {
		t.Errorf("doc = %+v", repo.UpsertUsers[0])
	}

	payload.CreatedAt = time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := h.Handle(context.Background(), mustMarshal(t, payload), ""); err != nil {
		t.Fatal(err)
	}
	if !repo.UpsertUsers[1].CreatedAt.Equal(payload.CreatedAt) {
		t.Errorf("createdAt = %v", repo.UpsertUsers[1].CreatedAt)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestUserDeletedHandler(t *testing.T) {
	svc, repo := newTestSvc(t)
	h := NewUserDeletedHandler(svc)
	if err := h.Handle(context.Background(), mustMarshal(t, events.UserDeletedPayload{UserID: "u1"}), ""); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedUsers[0] != "u1" {
		t.Errorf("deleted = %v", repo.DeletedUsers)
	}
	if err := h.Handle(context.Background(), json.RawMessage("bad"), ""); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestHandleEvent_UnknownTypeReturnsNil(t *testing.T) {
	svc, _ := newTestSvc(t)
	cm := &ConsumerManager{}
	reg := BuildRegistry(svc)
	msg := kafkago.Message{Value: []byte(`{"eventId":"1","eventType":"nope.event","payload":{}}`)}
	if err := cm.HandleEvent(context.Background(), reg, msg); err != nil {
		t.Fatalf("unknown event should return nil, got %v", err)
	}
}

func TestHandleEvent_InvalidEnvelope(t *testing.T) {
	svc, _ := newTestSvc(t)
	cm := &ConsumerManager{}
	reg := BuildRegistry(svc)
	err := cm.HandleEvent(context.Background(), reg, kafkago.Message{Value: []byte("bad")})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "permanent processing failure") {
		t.Errorf("expected ErrPermanent wrap: %v", err)
	}
}

func TestHandleEvent_DeliveryCreated(t *testing.T) {
	svc, repo := newTestSvc(t)
	cm := &ConsumerManager{}
	reg := BuildRegistry(svc)
	env, err := events.MarshalEnvelope("e1", string(events.DeliveryCreated), "t1", events.DeliveryCreatedPayload{
		DeliveryID: "d1", CustomerID: "c1", Status: "PENDING",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cm.HandleEvent(context.Background(), reg, kafkago.Message{Topic: "delivery.created", Value: env}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(repo.UpsertDeliveries) != 1 {
		t.Errorf("upserts = %d", len(repo.UpsertDeliveries))
	}
}

func TestHandleEvent_ForwardError(t *testing.T) {
	svc, repo := newTestSvc(t)
	repo.UpsertDeliveryErr = errTest
	cm := &ConsumerManager{}
	reg := BuildRegistry(svc)
	env, _ := events.MarshalEnvelope("e1", string(events.DeliveryCreated), "", events.DeliveryCreatedPayload{DeliveryID: "d1"})
	if err := cm.HandleEvent(context.Background(), reg, kafkago.Message{Value: env}); err == nil {
		t.Fatal("expected forwarded error")
	}
}

var errTest = errStr("test")

type errStr string

func (e errStr) Error() string { return string(e) }

func TestBuildRegistry_EventTypeKeys(t *testing.T) {
	svc, _ := newTestSvc(t)
	reg := BuildRegistry(svc)
	if len(reg.handlers) != 18 {
		t.Errorf("registry size = %d, want 18", len(reg.handlers))
	}
}
