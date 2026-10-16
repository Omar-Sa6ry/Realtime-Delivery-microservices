package dynamodb

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

const (
	testMaxQuota int64 = 1 << 30
	testMaxConc        = 3
)

func newQuotaRepo(t *testing.T, f *testutil.FakeDynamoDB) *QuotaRepository {
	t.Helper()
	return NewQuotaRepository(newTestClient(t, f), testTable, testMaxQuota, testMaxConc)
}

func TestQuotaRepositoryGetUsageMissingItem(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	usage, err := newQuotaRepo(t, f).GetUsage(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetUsage: %v", err)
	}
	if usage.UserID != "u1" || usage.UsedBytes != 0 || usage.ActiveUploads != 0 {
		t.Fatalf("unexpected defaults: %+v", usage)
	}
	if usage.QuotaBytes != testMaxQuota || usage.MaxConcurrent != testMaxConc {
		t.Fatalf("limits not applied: %+v", usage)
	}
}

func TestQuotaRepositoryGetUsageFound(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SeedItem(map[string]interface{}{
		"PK":            map[string]interface{}{"S": "USER#u1"},
		"SK":            map[string]interface{}{"S": "QUOTA"},
		"UserID":        map[string]interface{}{"S": "u1"},
		"UsedBytes":     map[string]interface{}{"N": "2048"},
		"ActiveUploads": map[string]interface{}{"N": "2"},
	})

	usage, err := newQuotaRepo(t, f).GetUsage(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetUsage: %v", err)
	}
	if usage.UsedBytes != 2048 || usage.ActiveUploads != 2 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
	if usage.QuotaBytes != testMaxQuota || usage.MaxConcurrent != testMaxConc {
		t.Fatalf("limits not applied: %+v", usage)
	}
}

func TestQuotaRepositoryGetUsageError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("GetItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	_, err := newQuotaRepo(t, f).GetUsage(context.Background(), "u1")
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB GetItem quota")
}

func TestQuotaRepositoryGetUsageUnmarshalError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SeedItem(map[string]interface{}{
		"PK":        map[string]interface{}{"S": "USER#u1"},
		"SK":        map[string]interface{}{"S": "QUOTA"},
		"UsedBytes": map[string]interface{}{"S": "not-a-number"},
	})

	if _, err := newQuotaRepo(t, f).GetUsage(context.Background(), "u1"); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestQuotaRepositoryIncrementUsage(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	if err := newQuotaRepo(t, f).IncrementUsage(context.Background(), "u1", 100); err != nil {
		t.Fatalf("IncrementUsage: %v", err)
	}
	body := decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
	if body.UpdateExpression != "SET UserID = if_not_exists(UserID, :uid), UpdatedAt = :now ADD ActiveUploads :one" {
		t.Errorf("UpdateExpression = %q", body.UpdateExpression)
	}
	if got := avString(t, body.ExpressionAttributeValues, ":uid"); got != "u1" {
		t.Errorf(":uid = %q", got)
	}
	if got := avNumber(t, body.ExpressionAttributeValues, ":one"); got != "1" {
		t.Errorf(":one = %q", got)
	}
	if body.Key == nil || body.Key["PK"]["S"] != "USER#u1" || body.Key["SK"]["S"] != "QUOTA" {
		t.Errorf("Key = %v", body.Key)
	}
}

func TestQuotaRepositoryIncrementUsageError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	if err := newQuotaRepo(t, f).IncrementUsage(context.Background(), "u1", 100); err == nil {
		t.Fatal("expected error")
	}
}

func TestQuotaRepositoryDecrementActiveUpload(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	if err := newQuotaRepo(t, f).DecrementActiveUpload(context.Background(), "u1"); err != nil {
		t.Fatalf("DecrementActiveUpload: %v", err)
	}
	body := decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
	if body.UpdateExpression != "SET UpdatedAt = :now ADD ActiveUploads :negOne" {
		t.Errorf("UpdateExpression = %q", body.UpdateExpression)
	}
	if got := avNumber(t, body.ExpressionAttributeValues, ":negOne"); got != "-1" {
		t.Errorf(":negOne = %q", got)
	}
}

func TestQuotaRepositoryDecrementActiveUploadError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	if err := newQuotaRepo(t, f).DecrementActiveUpload(context.Background(), "u1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestQuotaRepositoryAddAndSubtractUsedBytes(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := newQuotaRepo(t, f)
	ctx := context.Background()

	if err := repo.AddUsedBytes(ctx, "u1", 500); err != nil {
		t.Fatalf("AddUsedBytes: %v", err)
	}
	body := decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
	if body.UpdateExpression != "ADD UsedBytes :bytes" {
		t.Errorf("UpdateExpression = %q", body.UpdateExpression)
	}
	if got := avNumber(t, body.ExpressionAttributeValues, ":bytes"); got != "500" {
		t.Errorf(":bytes = %q", got)
	}

	if err := repo.SubtractUsedBytes(ctx, "u1", 200); err != nil {
		t.Fatalf("SubtractUsedBytes: %v", err)
	}
	body = decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
	if got := avNumber(t, body.ExpressionAttributeValues, ":bytes"); got != "-200" {
		t.Errorf(":bytes = %q", got)
	}
}

func TestQuotaRepositoryAddUsedBytesError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	if err := newQuotaRepo(t, f).AddUsedBytes(context.Background(), "u1", 500); err == nil {
		t.Fatal("expected error")
	}
}
