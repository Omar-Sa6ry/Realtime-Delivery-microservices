package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type DynamoError struct {
	Code    string
	Message string
	Status  int
}

func (e *DynamoError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type DynamoRequest struct {
	Target  string
	Body    []byte
	Handled bool
}

type FakeDynamoDB struct {
	server *httptest.Server

	mu       sync.Mutex
	items    map[string]map[string]interface{} // "PK\x00SK" -> item
	tables   map[string]bool
	errs     map[string]error // op (X-Amz-Target suffix) -> error
	requests []DynamoRequest
	// QueryOverride, when non-nil, replaces the item list of the next Query.
	queryOverride []map[string]interface{}
}

func NewFakeDynamoDB(t *testing.T) *FakeDynamoDB {
	t.Helper()
	f := &FakeDynamoDB{
		items:  make(map[string]map[string]interface{}),
		tables: make(map[string]bool),
		errs:   make(map[string]error),
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *FakeDynamoDB) Close() { f.server.Close() }

func (f *FakeDynamoDB) Endpoint() string { return f.server.URL }

func (f *FakeDynamoDB) SetError(op string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs[op] = err
}

func (f *FakeDynamoDB) ClearErrors() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = make(map[string]error)
}

func (f *FakeDynamoDB) CreateOrReplaceTable(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tables[name] = true
}

func (f *FakeDynamoDB) Requests() []DynamoRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]DynamoRequest(nil), f.requests...)
}

func (f *FakeDynamoDB) TargetCounts() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]int)
	for _, r := range f.requests {
		out[r.Target]++
	}
	return out
}

// SetQueryOverride forces Query responses (each entry is one item).
func (f *FakeDynamoDB) SetQueryOverride(items []map[string]interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryOverride = items
}

// SeedItem inserts a raw attribute-value item under its PK/SK.
func (f *FakeDynamoDB) SeedItem(item map[string]interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[dynamoKeyOf(item)] = item
}

// SeedStringItem builds an item from flat string attributes (nested values must
// already be encoded as attribute-value maps).
func (f *FakeDynamoDB) SeedStringItem(attrs map[string]string) map[string]interface{} {
	item := make(map[string]interface{}, len(attrs))
	for k, v := range attrs {
		item[k] = map[string]interface{}{"S": v}
	}
	return item
}

func (f *FakeDynamoDB) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	target := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "DynamoDB_20120810.")
	if idx := strings.Index(target, "."); idx >= 0 {
		target = target[idx+1:]
	}

	f.mu.Lock()
	f.requests = append(f.requests, DynamoRequest{Target: target, Body: body})
	injected := f.errs[target]
	f.mu.Unlock()

	if injected != nil {
		de, ok := injected.(*DynamoError)
		if !ok {
			de = &DynamoError{Code: "InternalServerError", Message: injected.Error(), Status: 500}
		}
		writeDynamoError(w, de)
		return
	}

	resp, apiErr := f.dispatch(target, body)
	if apiErr != nil {
		writeDynamoError(w, apiErr)
		return
	}
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("X-Amzn-RequestId", "fake-request-id")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeDynamoError(w http.ResponseWriter, e *DynamoError) {
	status := e.Status
	if status == 0 {
		status = 400
	}
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"__type":  "com.amazonaws.dynamodb.v20120810#" + e.Code,
		"Message": e.Message,
		"message": e.Message,
	})
}

func (f *FakeDynamoDB) dispatch(target string, body []byte) (interface{}, *DynamoError) {
	switch target {
	case "PutItem":
		return f.putItem(body)
	case "GetItem":
		return f.getItem(body)
	case "UpdateItem":
		return f.updateItem(body)
	case "DeleteItem":
		return f.deleteItem(body)
	case "Query":
		return f.query(body)
	case "DescribeTable":
		return f.describeTable(body)
	case "CreateTable":
		return f.createTable(body)
	case "UpdateTimeToLive":
		return map[string]interface{}{}, nil
	default:
		return nil, &DynamoError{Code: "UnknownOperationException", Message: target, Status: 400}
	}
}

func attrString(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	if s, ok := m["S"].(string); ok {
		return s
	}
	return ""
}

func dynamoKeyOf(item map[string]interface{}) string {
	pk := attrString(item["PK"])
	sk := attrString(item["SK"])
	return pk + "\x00" + sk
}

func keyFromAttr(key map[string]interface{}) string {
	return attrString(key["PK"]) + "\x00" + attrString(key["SK"])
}

func (f *FakeDynamoDB) putItem(body []byte) (interface{}, *DynamoError) {
	var req struct {
		Item                map[string]interface{} `json:"Item"`
		ConditionExpression string                 `json:"ConditionExpression"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &DynamoError{Code: "ValidationException", Message: err.Error()}
	}
	k := dynamoKeyOf(req.Item)
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.Contains(req.ConditionExpression, "attribute_not_exists(PK)") {
		if _, exists := f.items[k]; exists {
			return nil, &DynamoError{
				Code:    "ConditionalCheckFailedException",
				Message: "The conditional request failed",
				Status:  400,
			}
		}
	}
	cp := make(map[string]interface{}, len(req.Item))
	for key, val := range req.Item {
		cp[key] = val
	}
	f.items[k] = cp
	return map[string]interface{}{}, nil
}

func (f *FakeDynamoDB) getItem(body []byte) (interface{}, *DynamoError) {
	var req struct {
		Key map[string]interface{} `json:"Key"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &DynamoError{Code: "ValidationException", Message: err.Error()}
	}
	k := keyFromAttr(req.Key)
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.items[k]
	if !ok {
		return map[string]interface{}{}, nil
	}
	return map[string]interface{}{"Item": item}, nil
}

func (f *FakeDynamoDB) deleteItem(body []byte) (interface{}, *DynamoError) {
	var req struct {
		Key map[string]interface{} `json:"Key"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &DynamoError{Code: "ValidationException", Message: err.Error()}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.items, keyFromAttr(req.Key))
	return map[string]interface{}{}, nil
}

func (f *FakeDynamoDB) updateItem(body []byte) (interface{}, *DynamoError) {
	var req struct {
		Key                       map[string]interface{} `json:"Key"`
		UpdateExpression          string                 `json:"UpdateExpression"`
		ConditionExpression       string                 `json:"ConditionExpression"`
		ExpressionAttributeValues map[string]interface{} `json:"ExpressionAttributeValues"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &DynamoError{Code: "ValidationException", Message: err.Error()}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	k := keyFromAttr(req.Key)
	item, exists := f.items[k]

	if cond := req.ConditionExpression; cond != "" {
		if strings.Contains(cond, "attribute_not_exists(PK)") && exists {
			return conditionalFailed()
		}
		if strings.Contains(cond, "attribute_exists(PK)") && !exists {
			return conditionalFailed()
		}
		if strings.Contains(cond, "#st = :current") {
			if !exists || attrString(item["Status"]) != attrString(req.ExpressionAttributeValues[":current"]) {
				return conditionalFailed()
			}
		}
		if strings.Contains(cond, "#st = :uploading") {
			if !exists || attrString(item["Status"]) != attrString(req.ExpressionAttributeValues[":uploading"]) {
				return conditionalFailed()
			}
		}
		if strings.Contains(cond, "#st = :pending") {
			if !exists || attrString(item["Status"]) != attrString(req.ExpressionAttributeValues[":pending"]) {
				return conditionalFailed()
			}
		}
	}

	if exists {
		if v, ok := req.ExpressionAttributeValues[":st"]; ok {
			item["Status"] = v
		}
		if v, ok := req.ExpressionAttributeValues[":next"]; ok {
			item["Status"] = v
		}
		if v, ok := req.ExpressionAttributeValues[":gsi2pk"]; ok {
			item["GSI2_PK"] = v
		}
		if v, ok := req.ExpressionAttributeValues[":gsi3pk"]; ok {
			item["GSI3_PK"] = v
		}
		if v, ok := req.ExpressionAttributeValues[":st"]; ok {
			if s := attrString(v); s == "RUNNING" || s == "COMPLETED" || s == "FAILED" {
				item["GSI2_PK"] = map[string]interface{}{"S": "JOB_STATUS#" + s}
			}
		}
	}
	return map[string]interface{}{}, nil
}

func conditionalFailed() (interface{}, *DynamoError) {
	return nil, &DynamoError{
		Code:    "ConditionalCheckFailedException",
		Message: "The conditional request failed",
		Status:  400,
	}
}

type dynamoQueryRequest struct {
	TableName                 string                 `json:"TableName"`
	IndexName                 string                 `json:"IndexName"`
	KeyConditionExpression    string                 `json:"KeyConditionExpression"`
	ExpressionAttributeValues map[string]interface{} `json:"ExpressionAttributeValues"`
	Limit                     int32                  `json:"Limit"`
	ExclusiveStartKey         map[string]interface{} `json:"ExclusiveStartKey"`
	ScanIndexForward          *bool                  `json:"ScanIndexForward"`
}

func (f *FakeDynamoDB) query(body []byte) (interface{}, *DynamoError) {
	var req dynamoQueryRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &DynamoError{Code: "ValidationException", Message: err.Error()}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.queryOverride != nil {
		items := f.queryOverride
		f.queryOverride = nil
		return map[string]interface{}{"Items": items, "Count": len(items), "ScannedCount": len(items)}, nil
	}

	pkAttr := "PK"
	skAttr := "SK"
	switch req.IndexName {
	case "GSI1":
		pkAttr, skAttr = "GSI1_PK", "GSI1_SK"
	case "GSI2":
		pkAttr, skAttr = "GSI2_PK", "GSI2_SK"
	case "GSI3":
		pkAttr, skAttr = "GSI3_PK", "GSI3_SK"
	}

	wantPK := attrString(req.ExpressionAttributeValues[":pk"])
	wantSK := attrString(req.ExpressionAttributeValues[":sk"])
	prefix := attrString(req.ExpressionAttributeValues[":prefix"])

	var matched []map[string]interface{}
	for _, item := range f.items {
		if wantPK != "" && attrString(item[pkAttr]) != wantPK {
			continue
		}
		if prefix != "" && !strings.HasPrefix(attrString(item[skAttr]), prefix) {
			continue
		}
		if wantSK != "" && strings.Contains(req.KeyConditionExpression, "<=") {
			if attrString(item[skAttr]) > wantSK {
				continue
			}
		}
		matched = append(matched, item)
	}

	// Stable ordering by the sort key (and PK as a tiebreaker).
	sortByKeyAttr(matched, skAttr, req.ScanIndexForward)

	if req.ExclusiveStartKey != nil {
		startPK := attrString(req.ExclusiveStartKey["PK"])
		for i, item := range matched {
			if attrString(item["PK"]) == startPK {
				matched = matched[i+1:]
				break
			}
		}
	}

	total := len(matched)
	var lastEvaluated map[string]interface{}
	if req.Limit > 0 && int32(total) > req.Limit {
		matched = matched[:req.Limit]
		last := matched[len(matched)-1]
		lastEvaluated = map[string]interface{}{"PK": last["PK"], "SK": last["SK"]}
	}

	resp := map[string]interface{}{
		"Items":        matched,
		"Count":        len(matched),
		"ScannedCount": total,
	}
	if lastEvaluated != nil {
		resp["LastEvaluatedKey"] = lastEvaluated
	}
	return resp, nil
}

func sortByKeyAttr(items []map[string]interface{}, skAttr string, forward *bool) {
	descending := forward != nil && !*forward
	// Insertion order is not deterministic across runs; sort for stability.
	for i := 1; i < len(items); i++ {
		for j := i; j > 0; j-- {
			a := items[j-1]
			b := items[j]
			keyA := attrString(a[skAttr]) + attrString(a["PK"])
			keyB := attrString(b[skAttr]) + attrString(b["PK"])
			swap := keyB < keyA
			if descending {
				swap = keyB > keyA
			}
			if swap {
				items[j-1], items[j] = items[j], items[j-1]
			} else {
				break
			}
		}
	}
}

func (f *FakeDynamoDB) describeTable(body []byte) (interface{}, *DynamoError) {
	var req struct {
		TableName string `json:"TableName"`
	}
	_ = json.Unmarshal(body, &req)
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.tables[req.TableName] {
		return nil, &DynamoError{
			Code:    "ResourceNotFoundException",
			Message: "Requested table not found: " + req.TableName,
			Status:  400,
		}
	}
	return map[string]interface{}{
		"Table": map[string]interface{}{"TableName": req.TableName, "TableStatus": "ACTIVE"},
	}, nil
}

func (f *FakeDynamoDB) createTable(body []byte) (interface{}, *DynamoError) {
	var req struct {
		TableName string `json:"TableName"`
	}
	_ = json.Unmarshal(body, &req)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tables[req.TableName] = true
	return map[string]interface{}{
		"TableDescription": map[string]interface{}{"TableName": req.TableName, "TableStatus": "CREATING"},
	}, nil
}
