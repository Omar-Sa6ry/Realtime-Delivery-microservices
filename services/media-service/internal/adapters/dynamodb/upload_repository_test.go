package dynamodb

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

func newUploadForTest(id string, status domain.UploadStatus, expiresAt time.Time) *domain.UploadSession {
	s := testutil.NewUploadSession(id, "m1", "u1")
	s.Status = status
	s.ExpiresAt = expiresAt
	s.CreatedAt = fixedTime(1)
	s.UpdatedAt = fixedTime(1)
	return s
}

func TestUploadRepositoryCreate(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewUploadRepository(newTestClient(t, f), testTable)
	expires := fixedTime(1).Add(time.Hour)

	s := newUploadForTest("up1", domain.UploadStatusUploading, expires)
	s.CompletedParts = []domain.UploadPart{{PartNumber: 1, ETag: "etag-1"}}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	body := decodePutItem(t, lastTargetRequest(t, f, "PutItem").Body)
	cases := map[string]string{
		"PK":         "UPLOAD#up1",
		"SK":         "SESSION",
		"GSI2_PK":    "UPLOAD_STATUS#UPLOADING",
		"GSI2_SK":    "EXPIRES#" + expires.Format(time.RFC3339Nano),
		"UploadID":   "up1",
		"MediaID":    "m1",
		"UserID":     "u1",
		"S3UploadID": "s3-upload-up1",
		"Status":     "UPLOADING",
		"ExpiresAt":  expires.Format(time.RFC3339Nano),
	}
	for attr, want := range cases {
		if got := avString(t, body.Item, attr); got != want {
			t.Errorf("%s = %q, want %q", attr, got, want)
		}
	}
	if got := avNumber(t, body.Item, "TTL"); got != strconv.FormatInt(expires.Unix(), 10) {
		t.Errorf("TTL = %q, want %d", got, expires.Unix())
	}
	if body.ConditionExpression != "attribute_not_exists(PK)" {
		t.Errorf("ConditionExpression = %q", body.ConditionExpression)
	}
}

func TestUploadRepositoryCreateError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("PutItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	err := NewUploadRepository(newTestClient(t, f), testTable).
		Create(context.Background(), newUploadForTest("up1", domain.UploadStatusUploading, fixedTime(1)))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUploadRepositoryGetByID(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewUploadRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	s := newUploadForTest("up1", domain.UploadStatusUploading, fixedTime(1).Add(time.Hour))
	s.CompletedParts = []domain.UploadPart{{PartNumber: 1, ETag: "etag-1"}, {PartNumber: 2, ETag: "etag-2"}}
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "up1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UploadID != "up1" || got.MediaID != "m1" || got.UserID != "u1" || got.S3UploadID != "s3-upload-up1" {
		t.Fatalf("unexpected session: %+v", got)
	}
	if got.Status != domain.UploadStatusUploading || got.TotalParts != 2 || got.PartSize != s.PartSize {
		t.Fatalf("unexpected session: %+v", got)
	}
	if !got.ExpiresAt.Equal(s.ExpiresAt) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, s.ExpiresAt)
	}
	if len(got.CompletedParts) != 2 || got.CompletedParts[0].ETag != "etag-1" || got.CompletedParts[1].PartNumber != 2 {
		t.Fatalf("CompletedParts = %+v", got.CompletedParts)
	}
}

func TestUploadRepositoryGetByIDNotFound(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	_, err := NewUploadRepository(newTestClient(t, f), testTable).GetByID(context.Background(), "missing")
	if !errors.Is(err, domain.ErrUploadSessionNotFound) {
		t.Fatalf("err = %v, want ErrUploadSessionNotFound", err)
	}
}

func TestUploadRepositoryGetByIDError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("GetItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	_, err := NewUploadRepository(newTestClient(t, f), testTable).GetByID(context.Background(), "up1")
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB GetItem upload")
}

func TestUploadRepositoryUpdateStatus(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewUploadRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newUploadForTest("up1", domain.UploadStatusUploading, fixedTime(1).Add(time.Hour))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateStatus(ctx, "up1", domain.UploadStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	body := decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
	if body.UpdateExpression != "SET #st = :st, GSI2_PK = :gsi2pk, UpdatedAt = :now" {
		t.Errorf("UpdateExpression = %q", body.UpdateExpression)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":st"); got != "COMPLETED" {
		t.Errorf(":st = %q", got)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":gsi2pk"); got != "UPLOAD_STATUS#COMPLETED" {
		t.Errorf(":gsi2pk = %q", got)
	}

	got, err := repo.GetByID(ctx, "up1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != domain.UploadStatusCompleted {
		t.Fatalf("status = %q, want COMPLETED", got.Status)
	}
}

func TestUploadRepositoryUpdateStatusError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	if err := NewUploadRepository(newTestClient(t, f), testTable).
		UpdateStatus(context.Background(), "up1", domain.UploadStatusAborted); err == nil {
		t.Fatal("expected error")
	}
}

func TestUploadRepositoryUpdateCompletedParts(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewUploadRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newUploadForTest("up1", domain.UploadStatusUploading, fixedTime(1).Add(time.Hour))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	parts := []domain.UploadPart{{PartNumber: 1, ETag: "e1"}, {PartNumber: 2, ETag: "e2"}}
	if err := repo.UpdateCompletedParts(ctx, "up1", parts); err != nil {
		t.Fatalf("UpdateCompletedParts: %v", err)
	}
	body := decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
	if body.UpdateExpression != "SET CompletedParts = :parts, UpdatedAt = :now" {
		t.Errorf("UpdateExpression = %q", body.UpdateExpression)
	}
	listAttr := body.ExpressionAttributeValues[":parts"]
	if listAttr == nil || listAttr["L"] == nil {
		t.Fatalf(":parts is not a list: %#v", body.ExpressionAttributeValues[":parts"])
	}
}

func TestUploadRepositoryUpdateCompletedPartsError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	err := NewUploadRepository(newTestClient(t, f), testTable).
		UpdateCompletedParts(context.Background(), "up1", []domain.UploadPart{{PartNumber: 1, ETag: "e1"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUploadRepositoryUpdateExpiry(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewUploadRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newUploadForTest("up1", domain.UploadStatusUploading, fixedTime(1).Add(time.Hour))); err != nil {
		t.Fatalf("Create: %v", err)
	}

	newExpiry := fixedTime(1).Add(2 * time.Hour)
	if err := repo.UpdateExpiry(ctx, "up1", newExpiry); err != nil {
		t.Fatalf("UpdateExpiry: %v", err)
	}
	body := decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
	if body.UpdateExpression != "SET ExpiresAt = :expires, #ttl = :ttl, UpdatedAt = :now" {
		t.Errorf("UpdateExpression = %q", body.UpdateExpression)
	}
	if body.ConditionExpression != "attribute_exists(PK) AND #st = :uploading" {
		t.Errorf("ConditionExpression = %q", body.ConditionExpression)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":expires"); got != newExpiry.Format(time.RFC3339Nano) {
		t.Errorf(":expires = %q", got)
	}
	if got := avNumber(t, body.ExpressionAttributeValues, ":ttl"); got != strconv.FormatInt(newExpiry.Unix(), 10) {
		t.Errorf(":ttl = %q, want %d", got, newExpiry.Unix())
	}
	if body.ExpressionAttributeNames["#ttl"] != "TTL" || body.ExpressionAttributeNames["#st"] != "Status" {
		t.Errorf("ExpressionAttributeNames = %v", body.ExpressionAttributeNames)
	}
}

func TestUploadRepositoryUpdateExpiryWrongStatus(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewUploadRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	if err := repo.Create(ctx, newUploadForTest("up1", domain.UploadStatusUploading, fixedTime(1).Add(time.Hour))); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.UpdateStatus(ctx, "up1", domain.UploadStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	err := repo.UpdateExpiry(ctx, "up1", fixedTime(1).Add(2*time.Hour))
	if err == nil {
		t.Fatal("expected conditional failure when status is not UPLOADING")
	}
}

func TestUploadRepositoryUpdateExpiryMissingItem(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	err := NewUploadRepository(newTestClient(t, f), testTable).
		UpdateExpiry(context.Background(), "missing", fixedTime(1))
	if err == nil {
		t.Fatal("expected conditional failure for missing item")
	}
}

func TestUploadRepositoryUpdateExpiryError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	if err := NewUploadRepository(newTestClient(t, f), testTable).
		UpdateExpiry(context.Background(), "up1", fixedTime(1)); err == nil {
		t.Fatal("expected error")
	}
}

func TestUploadRepositoryListExpired(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewUploadRepository(newTestClient(t, f), testTable)
	ctx := context.Background()

	exp1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	exp2 := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)
	expFuture := time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	for id, exp := range map[string]time.Time{"up1": exp1, "up2": exp2, "up3": expFuture} {
		if err := repo.Create(ctx, newUploadForTest(id, domain.UploadStatusUploading, exp)); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}

	before := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	sessions, err := repo.ListExpired(ctx, before)
	if err != nil {
		t.Fatalf("ListExpired: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d expired sessions, want 2: %v", len(sessions), sessions)
	}
	if sessions[0].UploadID != "up1" || sessions[1].UploadID != "up2" {
		t.Fatalf("unexpected order: %s, %s", sessions[0].UploadID, sessions[1].UploadID)
	}

	none, err := repo.ListExpired(ctx, time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ListExpired none: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no expired sessions, got %v", none)
	}
}

func TestUploadRepositoryListExpiredError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("Query", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	_, err := NewUploadRepository(newTestClient(t, f), testTable).ListExpired(context.Background(), fixedTime(1))
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB Query expired uploads")
}
