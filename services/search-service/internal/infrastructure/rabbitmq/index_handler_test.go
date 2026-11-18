package rabbitmq

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
	shared "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
)

func newSvc(t *testing.T) (*indexing.Service, *testutil.FakeRepository) {
	t.Helper()
	repo := testutil.NewFakeRepository()
	return indexing.NewService(repo), repo
}

func envelope(t *testing.T, eventType string, payload interface{}) *shared.EventEnvelope {
	t.Helper()
	data, err := events.MarshalEnvelope("e1", eventType, "t1", payload)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	e, err := events.UnmarshalEnvelope(data)
	if err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return e
}

func TestHandleIndexEvent_NilEnvelope(t *testing.T) {
	err := HandleIndexEvent(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "permanent processing failure") {
		t.Errorf("expected permanent error: %v", err)
	}
}

func TestHandleIndexEvent_NilService(t *testing.T) {
	e := envelope(t, string(events.DeliveryCreated), events.DeliveryCreatedPayload{DeliveryID: "d1"})
	if err := HandleIndexEvent(context.Background(), nil, e); err != nil {
		t.Fatalf("nil svc should return nil, got %v", err)
	}
}

func TestHandleIndexEvent_UnknownType(t *testing.T) {
	svc, _ := newSvc(t)
	e := envelope(t, "nope.event", map[string]string{})
	if err := HandleIndexEvent(context.Background(), svc, e); err != nil {
		t.Fatalf("unknown type should return nil, got %v", err)
	}
}

func TestHandleIndexEvent_DeliveryCreated(t *testing.T) {
	svc, repo := newSvc(t)
	e := envelope(t, string(events.DeliveryCreated), events.DeliveryCreatedPayload{
		DeliveryID: "d1", CustomerID: "c1", Status: "PENDING",
		Pickup:  events.DeliveryAddress{City: "Cairo", Country: "EG", Location: events.DeliveryLocation{Lat: 1, Lon: 2}},
		Dropoff: events.DeliveryAddress{City: "Giza", Country: "EG", Location: events.DeliveryLocation{Lat: 3, Lon: 4}},
	})
	if err := HandleIndexEvent(context.Background(), svc, e); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertDeliveries[0].DeliveryID != "d1" || repo.UpsertDeliveries[0].Pickup.City != "Cairo" {
		t.Errorf("doc = %+v", repo.UpsertDeliveries[0])
	}
}

func TestHandleIndexEvent_DeliveryCreatedBadPayload(t *testing.T) {
	svc, _ := newSvc(t)
	e := &shared.EventEnvelope{EventType: string(events.DeliveryCreated), Payload: []byte("bad")}
	err := HandleIndexEvent(context.Background(), svc, e)
	if err == nil || !strings.Contains(err.Error(), "permanent processing failure") {
		t.Fatalf("expected permanent error: %v", err)
	}
}

func TestHandleIndexEvent_DeliveryUpdates(t *testing.T) {
	types := []string{
		string(events.DeliveryDriverAssigned),
		string(events.DeliveryDriverAccepted),
		string(events.DeliveryPickedUp),
		string(events.DeliveryInTransit),
		string(events.DeliveryCompleted),
		string(events.DeliveryCancelled),
	}
	for _, et := range types {
		t.Run(et, func(t *testing.T) {
			svc, repo := newSvc(t)
			e := envelope(t, et, events.DeliveryUpdatedPayload{
				DeliveryID: "d1", Status: "X", SourceVersion: 2,
			})
			if err := HandleIndexEvent(context.Background(), svc, e); err != nil {
				t.Fatal(err)
			}
			if repo.UpsertDeliveries[0].Status != "X" {
				t.Errorf("status = %q", repo.UpsertDeliveries[0].Status)
			}
		})
	}
}

func TestHandleIndexEvent_DeliveryUpdateBadPayload(t *testing.T) {
	svc, _ := newSvc(t)
	e := &shared.EventEnvelope{EventType: string(events.DeliveryPickedUp), Payload: []byte("bad")}
	if err := HandleIndexEvent(context.Background(), svc, e); err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleIndexEvent_DeliveryDeleted(t *testing.T) {
	svc, repo := newSvc(t)
	e := envelope(t, string(events.DeliveryDeleted), events.DeliveryDeletedPayload{DeliveryID: "d1"})
	if err := HandleIndexEvent(context.Background(), svc, e); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedDeliveries[0] != "d1" {
		t.Errorf("deleted = %v", repo.DeletedDeliveries)
	}

	e2 := &shared.EventEnvelope{EventType: string(events.DeliveryDeleted), Payload: []byte("bad")}
	if err := HandleIndexEvent(context.Background(), svc, e2); err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleIndexEvent_DriverCreated(t *testing.T) {
	svc, repo := newSvc(t)
	e := envelope(t, string(events.DriverCreated), events.DriverCreatedPayload{
		DriverID: "d1", Name: "Ali", Status: "AVAILABLE", VehicleType: "CAR",
		Rating: 5, Location: &events.DriverGeoPoint{Lat: 1, Lon: 2},
	})
	if err := HandleIndexEvent(context.Background(), svc, e); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertDrivers[0].Location == nil {
		t.Error("expected location")
	}

	eNilLoc := envelope(t, string(events.DriverCreated), events.DriverCreatedPayload{DriverID: "d2"})
	if err := HandleIndexEvent(context.Background(), svc, eNilLoc); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertDrivers[1].Location != nil {
		t.Error("expected nil location")
	}

	bad := &shared.EventEnvelope{EventType: string(events.DriverCreated), Payload: []byte("bad")}
	if err := HandleIndexEvent(context.Background(), svc, bad); err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleIndexEvent_DriverUpdated(t *testing.T) {
	svc, repo := newSvc(t)
	e := envelope(t, string(events.DriverUpdated), events.DriverUpdatedPayload{
		DriverID: "d1", Name: "Ali", Location: &events.DriverGeoPoint{Lat: 5, Lon: 6},
	})
	if err := HandleIndexEvent(context.Background(), svc, e); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertDrivers[0].Location.Lat != 5 {
		t.Errorf("loc = %+v", repo.UpsertDrivers[0].Location)
	}

	eNil := envelope(t, string(events.DriverUpdated), events.DriverUpdatedPayload{DriverID: "d2"})
	if err := HandleIndexEvent(context.Background(), svc, eNil); err != nil {
		t.Fatal(err)
	}

	bad := &shared.EventEnvelope{EventType: string(events.DriverUpdated), Payload: []byte("bad")}
	if err := HandleIndexEvent(context.Background(), svc, bad); err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleIndexEvent_DriverDeleted(t *testing.T) {
	svc, repo := newSvc(t)
	e := envelope(t, string(events.DriverDeleted), events.DriverDeletedPayload{DriverID: "d1"})
	if err := HandleIndexEvent(context.Background(), svc, e); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedDrivers[0] != "d1" {
		t.Errorf("deleted = %v", repo.DeletedDrivers)
	}
	bad := &shared.EventEnvelope{EventType: string(events.DriverDeleted), Payload: []byte("bad")}
	if err := HandleIndexEvent(context.Background(), svc, bad); err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleIndexEvent_MediaEvents(t *testing.T) {
	svc, repo := newSvc(t)

	e1 := envelope(t, string(events.MediaUploadCreated), events.MediaUploadCreatedPayload{
		MediaID: "m1", UserID: "u1", FileName: "a.png", ContentType: "image/png", MediaType: "IMAGE", Size: 1,
	})
	if err := HandleIndexEvent(context.Background(), svc, e1); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertMediaDocs[0].Status != "UPLOADING" {
		t.Errorf("status = %q", repo.UpsertMediaDocs[0].Status)
	}

	e2 := envelope(t, string(events.MediaUploadCompleted), events.MediaUploadCompletedPayload{
		MediaID: "m2", UserID: "u1", FileName: "b.png", Size: 2,
	})
	if err := HandleIndexEvent(context.Background(), svc, e2); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertMediaDocs[1].Status != "UPLOADED" {
		t.Errorf("status = %q", repo.UpsertMediaDocs[1].Status)
	}

	e3 := envelope(t, string(events.MediaReady), events.MediaReadyPayload{
		MediaID: "m3", UserID: "u1", FileName: "c.png", Size: 3,
	})
	if err := HandleIndexEvent(context.Background(), svc, e3); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertMediaDocs[2].Status != "READY" || repo.UpsertMediaDocs[2].Size != 3 {
		t.Errorf("doc = %+v", repo.UpsertMediaDocs[2])
	}

	e4 := envelope(t, string(events.MediaReady), events.MediaReadyPayload{
		MediaID: "m4", Size: 0, Versions: []events.MediaVersionInfo{{Size: 77}},
	})
	if err := HandleIndexEvent(context.Background(), svc, e4); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertMediaDocs[3].Size != 77 {
		t.Errorf("size = %d", repo.UpsertMediaDocs[3].Size)
	}

	e5 := envelope(t, string(events.MediaDeleted), events.MediaDeletedPayload{MediaID: "m1"})
	if err := HandleIndexEvent(context.Background(), svc, e5); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedMedia[0] != "m1" {
		t.Errorf("deleted = %v", repo.DeletedMedia)
	}

	for _, et := range []string{
		string(events.MediaUploadCreated),
		string(events.MediaUploadCompleted),
		string(events.MediaReady),
		string(events.MediaDeleted),
	} {
		bad := &shared.EventEnvelope{EventType: et, Payload: []byte("bad")}
		if err := HandleIndexEvent(context.Background(), svc, bad); err == nil {
			t.Errorf("%s: expected error", et)
		}
	}
}

func TestHandleIndexEvent_UserEvents(t *testing.T) {
	svc, repo := newSvc(t)

	e1 := envelope(t, string(events.UserCreated), events.UserCreatedPayload{
		UserID: "u1", Email: "a@b.c", FirstName: "A", LastName: "B", Role: "user",
	})
	if err := HandleIndexEvent(context.Background(), svc, e1); err != nil {
		t.Fatal(err)
	}
	if !repo.UpsertUsers[0].IsActive || repo.UpsertUsers[0].CreatedAt.IsZero() {
		t.Errorf("doc = %+v", repo.UpsertUsers[0])
	}

	e1b := envelope(t, string(events.UserCreated), events.UserCreatedPayload{
		UserID: "u2", CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err := HandleIndexEvent(context.Background(), svc, e1b); err != nil {
		t.Fatal(err)
	}
	if !repo.UpsertUsers[1].CreatedAt.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("createdAt = %v", repo.UpsertUsers[1].CreatedAt)
	}

	e2 := envelope(t, string(events.UserUpdated), events.UserUpdatedPayload{
		UserID: "u1", IsActive: true,
	})
	if err := HandleIndexEvent(context.Background(), svc, e2); err != nil {
		t.Fatal(err)
	}
	if !repo.UpsertUsers[2].IsActive {
		t.Error("expected active")
	}

	e2b := envelope(t, string(events.UserUpdated), events.UserUpdatedPayload{
		UserID: "u2", CreatedAt: time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
	})
	if err := HandleIndexEvent(context.Background(), svc, e2b); err != nil {
		t.Fatal(err)
	}
	if !repo.UpsertUsers[3].CreatedAt.Equal(time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("createdAt = %v", repo.UpsertUsers[3].CreatedAt)
	}

	e3 := envelope(t, string(events.UserDeleted), events.UserDeletedPayload{UserID: "u1"})
	if err := HandleIndexEvent(context.Background(), svc, e3); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedUsers[0] != "u1" {
		t.Errorf("deleted = %v", repo.DeletedUsers)
	}

	for _, et := range []string{
		string(events.UserCreated),
		string(events.UserUpdated),
		string(events.UserDeleted),
	} {
		bad := &shared.EventEnvelope{EventType: et, Payload: []byte("bad")}
		if err := HandleIndexEvent(context.Background(), svc, bad); err == nil {
			t.Errorf("%s: expected error", et)
		}
	}
}

func TestHandleIndexEvent_RepoErrorForwarded(t *testing.T) {
	svc, repo := newSvc(t)
	repo.UpsertDeliveryErr = errors.New("down")
	e := envelope(t, string(events.DeliveryCreated), events.DeliveryCreatedPayload{DeliveryID: "d1"})
	if err := HandleIndexEvent(context.Background(), svc, e); err == nil {
		t.Fatal("expected repo error")
	}
}
