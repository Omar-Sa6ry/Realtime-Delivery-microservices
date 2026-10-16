package dynamodb

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

const testTable = "media-table"

func newTestClient(t *testing.T, f *testutil.FakeDynamoDB) *dynamodb.Client {
	t.Helper()
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("key", "secret", "")),
		awsconfig.WithRetryMaxAttempts(1),
	)
	if err != nil {
		t.Fatalf("LoadDefaultConfig: %v", err)
	}
	return dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(f.Endpoint())
	})
}

func lastTargetRequest(t *testing.T, f *testutil.FakeDynamoDB, target string) testutil.DynamoRequest {
	t.Helper()
	reqs := f.Requests()
	for i := len(reqs) - 1; i >= 0; i-- {
		if reqs[i].Target == target {
			return reqs[i]
		}
	}
	t.Fatalf("no recorded %s request", target)
	return testutil.DynamoRequest{}
}

type putItemBody struct {
	Item                map[string]map[string]interface{} `json:"Item"`
	ConditionExpression string                            `json:"ConditionExpression"`
}

func decodePutItem(t *testing.T, raw []byte) putItemBody {
	t.Helper()
	var body putItemBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal PutItem body: %v", err)
	}
	return body
}

type updateItemBody struct {
	Key                       map[string]map[string]interface{} `json:"Key"`
	UpdateExpression          string                            `json:"UpdateExpression"`
	ConditionExpression       string                            `json:"ConditionExpression"`
	ExpressionAttributeValues map[string]map[string]interface{} `json:"ExpressionAttributeValues"`
	ExpressionAttributeNames  map[string]string                 `json:"ExpressionAttributeNames"`
}

func decodeUpdateItem(t *testing.T, raw []byte) updateItemBody {
	t.Helper()
	var body updateItemBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal UpdateItem body: %v", err)
	}
	return body
}

func avString(t *testing.T, item map[string]map[string]interface{}, key string) string {
	t.Helper()
	attr, ok := item[key]
	if !ok {
		t.Fatalf("attr %q missing: %#v", key, item)
	}
	s, ok := attr["S"].(string)
	if !ok {
		t.Fatalf("attr %q is not S: %#v", key, attr)
	}
	return s
}

func avNumber(t *testing.T, item map[string]map[string]interface{}, key string) string {
	t.Helper()
	attr, ok := item[key]
	if !ok {
		t.Fatalf("attr %q missing: %#v", key, item)
	}
	s, ok := attr["N"].(string)
	if !ok {
		t.Fatalf("attr %q is not N: %#v", key, attr)
	}
	return s
}

func requireContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected %q to contain %q", s, substr)
	}
}
