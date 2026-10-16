package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

func fixedTime(sec int) time.Time {
	return time.Date(2026, 1, 2, 3, 4, sec, 0, time.UTC)
}

func newMediaForTest(id, owner string, status domain.MediaStatus, createdAt time.Time) *domain.Media {
	m := testutil.NewMedia(id, owner)
	m.Status = status
	m.CreatedAt = createdAt
	m.UpdatedAt = createdAt
	return m
}

func TestMediaRepositoryCreate(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewMediaRepository(newTestClient(t, f), testTable)

	if err := repo.Create(context.Background(), newMediaForTest("m1", "u1", domain.MediaStatusReady, fixedTime(5))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	body := decodePutItem(t, lastTargetRequest(t, f, "PutItem").Body)
	wantPrefix := "STATUS#READY#CREATED#" + fixedTime(5).Format(time.RFC3339Nano)
	cases := map[string]string{
		"PK":        "MEDIA#m1",
		"SK":        "MEDIA",
		"GSI1_PK":   "OWNER#u1",
		"GSI1_SK":   wantPrefix,
		"GSI2_PK":   "STATUS#READY",
		"GSI2_SK":   "CREATED#" + fixedTime(5).Format(time.RFC3339Nano),
		"MediaID":   "m1",
		"OwnerID":   "u1",
		"Status":    "READY",
		"ObjectKey": "users/u1/media/m1/original/photo.jpg",
	}
	for attr, want := range cases {
		if got := avString(t, body.Item, attr); got != want {
			t.Errorf("%s = %q, want %q", attr, got, want)
		}
	}
	if got := avNumber(t, body.Item, "Size"); got != "1024" {
		t.Errorf("Size = %q, want 1024", got)
	}
	if body.ConditionExpression != "attribute_not_exists(PK)" {
		t.Errorf("ConditionExpression = %q", body.ConditionExpression)
	}
}

func TestMediaRepositoryCreateDuplicate(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewMediaRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	m := newMediaForTest("m1", "u1", domain.MediaStatusPending, fixedTime(5))

	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	err := repo.Create(ctx, m)
	if err == nil {
		t.Fatal("expected conditional failure on duplicate create")
	}
	requireContains(t, err.Error(), "DynamoDB PutItem media")
	requireContains(t, err.Error(), "ConditionalCheckFailed")
}

func TestMediaRepositoryCreateError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("PutItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad item", Status: 400})

	err := NewMediaRepository(newTestClient(t, f), testTable).
		Create(context.Background(), newMediaForTest("m1", "u1", domain.MediaStatusPending, fixedTime(5)))
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB PutItem media")
}

func TestMediaRepositoryGetByID(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewMediaRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	m := newMediaForTest("m1", "u1", domain.MediaStatusReady, fixedTime(5))
	m.Checksum = "abc123"
	m.DeliveryID = "d1"

	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByID(ctx, "m1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.MediaID != "m1" || got.OwnerID != "u1" || got.Status != domain.MediaStatusReady {
		t.Fatalf("unexpected media: %+v", got)
	}
	if got.Checksum != "abc123" || got.DeliveryID != "d1" {
		t.Fatalf("optional fields not round-tripped: %+v", got)
	}
	if !got.CreatedAt.Equal(m.CreatedAt) {
		t.Fatalf("CreatedAt = %v, want %v", got.CreatedAt, m.CreatedAt)
	}
	if got.FileName != m.FileName || got.ContentType != m.ContentType || got.Size != m.Size || got.ObjectKey != m.ObjectKey {
		t.Fatalf("fields mismatch: %+v", got)
	}
}

func TestMediaRepositoryGetByIDNotFound(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewMediaRepository(newTestClient(t, f), testTable)

	_, err := repo.GetByID(context.Background(), "missing")
	if !errors.Is(err, domain.ErrMediaNotFound) {
		t.Fatalf("err = %v, want ErrMediaNotFound", err)
	}
}

func TestMediaRepositoryGetByIDError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("GetItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	_, err := NewMediaRepository(newTestClient(t, f), testTable).GetByID(context.Background(), "m1")
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB GetItem media")
}

func TestMediaRepositoryUpdateStatus(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewMediaRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newMediaForTest("m1", "u1", domain.MediaStatusPending, fixedTime(5))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateStatus(ctx, "m1", domain.MediaStatusPending, domain.MediaStatusUploading); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	body := decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
	if body.UpdateExpression != "SET #st = :next, UpdatedAt = :now, GSI2_PK = :gsi2pk" {
		t.Errorf("UpdateExpression = %q", body.UpdateExpression)
	}
	if body.ConditionExpression != "#st = :current" {
		t.Errorf("ConditionExpression = %q", body.ConditionExpression)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":current"); got != "PENDING" {
		t.Errorf(":current = %q", got)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":next"); got != "UPLOADING" {
		t.Errorf(":next = %q", got)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":gsi2pk"); got != "STATUS#UPLOADING" {
		t.Errorf(":gsi2pk = %q", got)
	}

	got, err := repo.GetByID(ctx, "m1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != domain.MediaStatusUploading {
		t.Fatalf("status = %q, want UPLOADING", got.Status)
	}
}

func TestMediaRepositoryUpdateStatusWrongCurrent(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewMediaRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newMediaForTest("m1", "u1", domain.MediaStatusPending, fixedTime(5))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := repo.UpdateStatus(ctx, "m1", domain.MediaStatusReady, domain.MediaStatusScanning)
	if err == nil {
		t.Fatal("expected conditional failure")
	}
	requireContains(t, err.Error(), "DynamoDB UpdateStatus media")
	requireContains(t, err.Error(), "ConditionalCheckFailed")
}

func TestMediaRepositoryUpdateStatusMissingItem(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	err := NewMediaRepository(newTestClient(t, f), testTable).
		UpdateStatus(context.Background(), "missing", domain.MediaStatusPending, domain.MediaStatusUploading)
	if err == nil {
		t.Fatal("expected conditional failure for missing item")
	}
	requireContains(t, err.Error(), "ConditionalCheckFailed")
}

func TestMediaRepositoryUpdateStatusError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	err := NewMediaRepository(newTestClient(t, f), testTable).
		UpdateStatus(context.Background(), "m1", domain.MediaStatusPending, domain.MediaStatusUploading)
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB UpdateStatus media")
}

func TestMediaRepositoryListByOwner(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewMediaRepository(newTestClient(t, f), testTable)
	ctx := context.Background()

	for i, id := range []string{"m1", "m2", "m3"} {
		if err := repo.Create(ctx, newMediaForTest(id, "u1", domain.MediaStatusReady, fixedTime(5+i))); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}
	if err := repo.Create(ctx, newMediaForTest("other", "u2", domain.MediaStatusReady, fixedTime(30))); err != nil {
		t.Fatalf("Create other: %v", err)
	}

	items, cursor, err := repo.ListByOwner(ctx, "u1", 10, "")
	if err != nil {
		t.Fatalf("ListByOwner: %v", err)
	}
	if len(items) != 3 || cursor != "" {
		t.Fatalf("got %d items, cursor %q", len(items), cursor)
	}
	if items[0].MediaID != "m3" || items[1].MediaID != "m2" || items[2].MediaID != "m1" {
		t.Fatalf("expected newest first, got %s,%s,%s", items[0].MediaID, items[1].MediaID, items[2].MediaID)
	}

	page1, next1, err := repo.ListByOwner(ctx, "u1", 1, "")
	if err != nil {
		t.Fatalf("ListByOwner page1: %v", err)
	}
	if len(page1) != 1 || page1[0].MediaID != "m3" || next1 != "MEDIA#m3" {
		t.Fatalf("page1 = %v, next %q", page1, next1)
	}
	page2, next2, err := repo.ListByOwner(ctx, "u1", 1, next1)
	if err != nil {
		t.Fatalf("ListByOwner page2: %v", err)
	}
	if len(page2) != 1 || page2[0].MediaID != "m2" || next2 != "MEDIA#m2" {
		t.Fatalf("page2 = %v, next %q", page2, next2)
	}
	page3, next3, err := repo.ListByOwner(ctx, "u1", 1, next2)
	if err != nil {
		t.Fatalf("ListByOwner page3: %v", err)
	}
	if len(page3) != 1 || page3[0].MediaID != "m1" || next3 != "" {
		t.Fatalf("page3 = %v, next %q", page3, next3)
	}

	other, _, err := repo.ListByOwner(ctx, "u2", 10, "")
	if err != nil {
		t.Fatalf("ListByOwner u2: %v", err)
	}
	if len(other) != 1 || other[0].MediaID != "other" {
		t.Fatalf("u2 items = %v", other)
	}

	empty, emptyCursor, err := repo.ListByOwner(ctx, "nobody", 10, "")
	if err != nil {
		t.Fatalf("ListByOwner nobody: %v", err)
	}
	if len(empty) != 0 || emptyCursor != "" {
		t.Fatalf("expected empty page, got %v cursor %q", empty, emptyCursor)
	}
}

func TestMediaRepositoryListByOwnerError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("Query", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	_, _, err := NewMediaRepository(newTestClient(t, f), testTable).ListByOwner(context.Background(), "u1", 10, "")
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB Query ListByOwner")
}

func TestMediaRepositoryListByOwnerSkipsUnmarshalableItems(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetQueryOverride([]map[string]interface{}{
		{
			"PK":      map[string]interface{}{"S": "MEDIA#x"},
			"SK":      map[string]interface{}{"S": "MEDIA"},
			"MediaID": map[string]interface{}{"L": []interface{}{}},
		},
	})

	items, _, err := NewMediaRepository(newTestClient(t, f), testTable).ListByOwner(context.Background(), "u1", 10, "")
	if err != nil {
		t.Fatalf("ListByOwner: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected unmarshalable item to be skipped, got %v", items)
	}
}

func TestMediaRepositoryDelete(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewMediaRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newMediaForTest("m1", "u1", domain.MediaStatusDeleting, fixedTime(5))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, "m1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err := repo.GetByID(ctx, "m1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != domain.MediaStatusDeleted {
		t.Fatalf("status = %q, want DELETED", got.Status)
	}
}

func TestMediaRepositoryDeleteWrongStatus(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewMediaRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newMediaForTest("m1", "u1", domain.MediaStatusReady, fixedTime(5))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := repo.Delete(ctx, "m1")
	if err == nil {
		t.Fatal("expected conditional failure when status is not DELETING")
	}
	requireContains(t, err.Error(), "ConditionalCheckFailed")
}
