package dynamodb

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

func newOutboxForTest(id string, status domain.OutboxStatus, createdAt time.Time) *domain.OutboxEvent {
	return &domain.OutboxEvent{
		EventID:     id,
		AggregateID: "m1",
		EventType:   "media.created",
		Payload:     []byte(`{"mediaId":"m1"}`),
		Status:      status,
		TraceID:     "trace-1",
		CreatedAt:   createdAt,
		TTL:         createdAt.Add(24 * time.Hour).Unix(),
	}
}

func TestOutboxRepositoryCreate(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewOutboxRepository(newTestClient(t, f), testTable)

	if err := repo.Create(context.Background(), newOutboxForTest("e1", domain.OutboxStatusPending, fixedTime(5))); err != nil {
		t.Fatalf("Create: %v", err)
	}
	body := decodePutItem(t, lastTargetRequest(t, f, "PutItem").Body)
	cases := map[string]string{
		"PK":          "OUTBOX#e1",
		"SK":          "EVENT",
		"GSI3_PK":     "OUTBOX_STATUS#PENDING",
		"GSI3_SK":     "CREATED#" + fixedTime(5).Format(time.RFC3339Nano),
		"EventID":     "e1",
		"AggregateID": "m1",
		"EventType":   "media.created",
		"Status":      "PENDING",
		"TraceID":     "trace-1",
	}
	for attr, want := range cases {
		if got := avString(t, body.Item, attr); got != want {
			t.Errorf("%s = %q, want %q", attr, got, want)
		}
	}
	if got := avNumber(t, body.Item, "TTL"); got != strconv.FormatInt(fixedTime(5).Add(24*time.Hour).Unix(), 10) {
		t.Errorf("TTL = %q", got)
	}
}

func TestOutboxRepositoryCreateError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("PutItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	err := NewOutboxRepository(newTestClient(t, f), testTable).
		Create(context.Background(), newOutboxForTest("e1", domain.OutboxStatusPending, fixedTime(5)))
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB PutItem outbox event")
}

func TestOutboxRepositoryListPending(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewOutboxRepository(newTestClient(t, f), testTable)
	ctx := context.Background()

	if err := repo.Create(ctx, newOutboxForTest("e1", domain.OutboxStatusPending, fixedTime(5))); err != nil {
		t.Fatalf("Create e1: %v", err)
	}
	if err := repo.Create(ctx, newOutboxForTest("e2", domain.OutboxStatusPending, fixedTime(6))); err != nil {
		t.Fatalf("Create e2: %v", err)
	}
	if err := repo.Create(ctx, newOutboxForTest("e3", domain.OutboxStatusPublished, fixedTime(7))); err != nil {
		t.Fatalf("Create e3: %v", err)
	}

	events, err := repo.ListPending(ctx, 10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d pending events, want 2: %v", len(events), events)
	}
	if events[0].EventID != "e1" || events[1].EventID != "e2" {
		t.Fatalf("expected oldest first, got %s, %s", events[0].EventID, events[1].EventID)
	}
	if events[0].Status != domain.OutboxStatusPending || events[0].TraceID != "trace-1" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
	if !bytes.Equal(events[0].Payload, []byte(`{"mediaId":"m1"}`)) {
		t.Fatalf("payload = %q", events[0].Payload)
	}
	if !events[0].CreatedAt.Equal(fixedTime(5)) {
		t.Fatalf("CreatedAt = %v", events[0].CreatedAt)
	}
}

func TestOutboxRepositoryListPendingError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("Query", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	_, err := NewOutboxRepository(newTestClient(t, f), testTable).ListPending(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB Query outbox PENDING")
}

func TestOutboxRepositoryListPendingSkipsUnmarshalableItems(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetQueryOverride([]map[string]interface{}{
		{
			"PK":      map[string]interface{}{"S": "OUTBOX#x"},
			"SK":      map[string]interface{}{"S": "EVENT"},
			"EventID": map[string]interface{}{"L": []interface{}{}},
		},
	})

	events, err := NewOutboxRepository(newTestClient(t, f), testTable).ListPending(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected unmarshalable item skipped, got %v", events)
	}
}

func TestOutboxRepositoryMarkPublished(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewOutboxRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newOutboxForTest("e1", domain.OutboxStatusPending, fixedTime(5))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.MarkPublished(ctx, "e1"); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	body := decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
	if body.UpdateExpression != "SET #st = :published, GSI3_PK = :gsi3pk, PublishedAt = :now" {
		t.Errorf("UpdateExpression = %q", body.UpdateExpression)
	}
	if body.ConditionExpression != "#st = :pending" {
		t.Errorf("ConditionExpression = %q", body.ConditionExpression)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":published"); got != "PUBLISHED" {
		t.Errorf(":published = %q", got)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":gsi3pk"); got != "OUTBOX_STATUS#PUBLISHED" {
		t.Errorf(":gsi3pk = %q", got)
	}
}

func TestOutboxRepositoryMarkPublishedConditionFailure(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewOutboxRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newOutboxForTest("e1", domain.OutboxStatusPublished, fixedTime(5))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := repo.MarkPublished(ctx, "e1")
	if err == nil {
		t.Fatal("expected conditional failure when status is not PENDING")
	}
	requireContains(t, err.Error(), "DynamoDB MarkPublished outbox")

	err = repo.MarkPublished(ctx, "missing")
	if err == nil {
		t.Fatal("expected conditional failure for missing event")
	}
}

func TestOutboxRepositoryMarkPublishedError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	if err := NewOutboxRepository(newTestClient(t, f), testTable).MarkPublished(context.Background(), "e1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOutboxRepositoryMarkFailed(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewOutboxRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newOutboxForTest("e1", domain.OutboxStatusPending, fixedTime(5))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.MarkFailed(ctx, "e1", "broker down"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	body := decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
	if body.UpdateExpression != "SET #st = :failed, GSI3_PK = :gsi3pk, LastError = :reason, Attempts = Attempts + :one" {
		t.Errorf("UpdateExpression = %q", body.UpdateExpression)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":failed"); got != "FAILED" {
		t.Errorf(":failed = %q", got)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":reason"); got != "broker down" {
		t.Errorf(":reason = %q", got)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":gsi3pk"); got != "OUTBOX_STATUS#FAILED" {
		t.Errorf(":gsi3pk = %q", got)
	}
}

func TestOutboxRepositoryMarkFailedError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	err := NewOutboxRepository(newTestClient(t, f), testTable).MarkFailed(context.Background(), "e1", "boom")
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB MarkFailed outbox")
}
