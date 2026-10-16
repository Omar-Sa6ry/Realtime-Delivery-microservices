package dynamodb

import (
	"context"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func TestNewClientWithEndpoint(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.CreateOrReplaceTable(testTable)

	client, err := NewClient(context.Background(), "us-east-1", f.Endpoint(), "key", "secret")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.DescribeTable(context.Background(), &dynamodb.DescribeTableInput{
		TableName: aws.String(testTable),
	}); err != nil {
		t.Fatalf("DescribeTable through NewClient: %v", err)
	}
}

func TestNewClientWithoutCredentials(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	client, err := NewClient(context.Background(), "us-east-1", f.Endpoint(), "", "")
	if err != nil {
		t.Fatalf("NewClient without credentials: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClientWithoutEndpoint(t *testing.T) {
	client, err := NewClient(context.Background(), "us-east-1", "", "key", "secret")
	if err != nil {
		t.Fatalf("NewClient without endpoint: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestEnsureTableExistsAlreadyPresent(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.CreateOrReplaceTable(testTable)

	if err := EnsureTableExists(context.Background(), newTestClient(t, f), testTable); err != nil {
		t.Fatalf("EnsureTableExists: %v", err)
	}
	counts := f.TargetCounts()
	if counts["CreateTable"] != 0 {
		t.Fatalf("expected no CreateTable, got %d", counts["CreateTable"])
	}
	if counts["DescribeTable"] != 1 {
		t.Fatalf("expected 1 DescribeTable, got %d", counts["DescribeTable"])
	}
}

func TestEnsureTableExistsCreatesMissingTable(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	client := newTestClient(t, f)

	if err := EnsureTableExists(context.Background(), client, "new-table"); err != nil {
		t.Fatalf("EnsureTableExists: %v", err)
	}
	if f.TargetCounts()["CreateTable"] != 1 {
		t.Fatal("expected CreateTable to be issued")
	}
	// table now exists, so a second call must not create again
	if err := EnsureTableExists(context.Background(), client, "new-table"); err != nil {
		t.Fatalf("second EnsureTableExists: %v", err)
	}
	if f.TargetCounts()["CreateTable"] != 1 {
		t.Fatalf("expected still 1 CreateTable, got %d", f.TargetCounts()["CreateTable"])
	}
}

func TestEnsureTableExistsCreateFails(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("CreateTable", &testutil.DynamoError{Code: "ValidationException", Message: "bad request", Status: 400})

	err := EnsureTableExists(context.Background(), newTestClient(t, f), "new-table")
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "create DynamoDB table")
	requireContains(t, err.Error(), "ValidationException")
}

func TestEnsureTTL(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	client := newTestClient(t, f)

	if err := EnsureTTL(context.Background(), client, testTable, "TTL"); err != nil {
		t.Fatalf("EnsureTTL: %v", err)
	}
	if f.TargetCounts()["UpdateTimeToLive"] != 1 {
		t.Fatal("expected UpdateTimeToLive to be issued")
	}
}

func TestEnsureTTLEmptyAttributeName(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	err := EnsureTTL(context.Background(), newTestClient(t, f), testTable, "")
	if err == nil {
		t.Fatal("expected error for empty attribute name")
	}
	requireContains(t, err.Error(), "TTL attribute name is empty")
	if f.TargetCounts()["UpdateTimeToLive"] != 0 {
		t.Fatal("no UpdateTimeToLive should be sent")
	}
}

func TestEnsureTTLFailure(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateTimeToLive", &testutil.DynamoError{Code: "ValidationException", Message: "nope", Status: 400})

	err := EnsureTTL(context.Background(), newTestClient(t, f), testTable, "TTL")
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "enable DynamoDB TTL")
}
