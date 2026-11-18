package graphql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	sharedconstants "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/constants"
	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/reindex"
	appSearch "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/i18n"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/redis"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
	miniredis "github.com/alicebob/miniredis/v2"
)

type serverStack struct {
	srv    *Server
	repo   *testutil.FakeRepository
	search *appSearch.Service
	reidx  *reindex.Service
	mr     *miniredis.Miniredis
}

func newServerStack(t *testing.T) *serverStack {
	t.Helper()
	mr := miniredis.RunT(t)
	cache := redis.NewCache(&config.Config{
		RedisHost: mr.Host(),
		RedisPort: mr.Port(),
		RedisTTL:  time.Minute,
	})
	repo := testutil.NewFakeRepository()
	searchSvc := appSearch.NewService(repo, cache)
	reidxSvc := reindex.NewService(repo)
	srv, err := NewServer(searchSvc, reidxSvc, "4007")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return &serverStack{srv: srv, repo: repo, search: searchSvc, reidx: reidxSvc, mr: mr}
}

func doJSON(t *testing.T, h http.HandlerFunc, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
	}
	return m
}

const searchHealthQuery = `{ searchHealth { success statusCode message data { status timestamp } } }`

// ---------- handler routing ----------

func TestHandler_SearchHealthEndpoint(t *testing.T) {
	s := newServerStack(t)
	body, _ := json.Marshal(map[string]string{"query": searchHealthQuery})
	rec := doJSON(t, s.srv.Handler(), http.MethodPost, "/search/graphql", string(body), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
	m := decodeBody(t, rec)
	if _, ok := m["errors"]; ok {
		t.Fatalf("errors = %v", m["errors"])
	}
	data := m["data"].(map[string]interface{})
	health := data["searchHealth"].(map[string]interface{})
	if health["success"] != true || health["statusCode"] != float64(200) {
		t.Errorf("health = %+v", health)
	}
	if health["message"] != "Search service is healthy" {
		t.Errorf("message = %v", health["message"])
	}
	inner := health["data"].(map[string]interface{})
	if inner["status"] != "UP" {
		t.Errorf("status = %v", inner["status"])
	}
}

func TestHandler_GETWithQuery(t *testing.T) {
	s := newServerStack(t)
	q := url.Values{}
	q.Set("query", searchHealthQuery)
	rec := doJSON(t, s.srv.Handler(), http.MethodGet, "/search/graphql?"+q.Encode(), "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if _, ok := m["errors"]; ok {
		t.Fatalf("errors = %v", m["errors"])
	}
}

func TestHandler_GETWithVariables(t *testing.T) {
	s := newServerStack(t)
	q := url.Values{}
	q.Set("query", `query Q($input: DeliverySearchInput!) { searchDeliveries(input: $input) { success } }`)
	q.Set("variables", `{"input":{"pagination":{"limit":5}}}`)
	q.Set("operationName", "Q")
	rec := doJSON(t, s.srv.Handler(), http.MethodGet, "/search/graphql?"+q.Encode(), "", map[string]string{
		sharedconstants.HeaderXUserId: "u1",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	// Auth succeeds via header; repo returns empty items.
	if _, ok := m["errors"]; ok {
		t.Fatalf("errors = %v", m["errors"])
	}
	data := m["data"].(map[string]interface{})
	d := data["searchDeliveries"].(map[string]interface{})
	if d["success"] != true {
		t.Errorf("result = %+v", d)
	}
}

func TestHandler_GETInvalidVariables(t *testing.T) {
	s := newServerStack(t)
	q := url.Values{}
	q.Set("query", searchHealthQuery)
	q.Set("variables", "{not json}")
	rec := doJSON(t, s.srv.Handler(), http.MethodGet, "/search/graphql?"+q.Encode(), "", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	m := decodeBody(t, rec)
	errs := m["errors"].([]interface{})
	if !strings.Contains(errs[0].(map[string]interface{})["message"].(string), "invalid GraphQL variables") {
		t.Errorf("errors = %v", errs)
	}
}

func TestHandler_POSTInvalidJSON(t *testing.T) {
	s := newServerStack(t)
	rec := doJSON(t, s.srv.Handler(), http.MethodPost, "/search/graphql", "{bad json", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	m := decodeBody(t, rec)
	msg := m["errors"].([]interface{})[0].(map[string]interface{})["message"].(string)
	if !strings.Contains(msg, "invalid GraphQL request") {
		t.Errorf("message = %q", msg)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	s := newServerStack(t)
	rec := doJSON(t, s.srv.Handler(), http.MethodPut, "/search/graphql", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET, POST" {
		t.Errorf("allow = %q", allow)
	}
	m := decodeBody(t, rec)
	msg := m["errors"].([]interface{})[0].(map[string]interface{})["message"].(string)
	if !strings.Contains(msg, "only GET and POST") {
		t.Errorf("message = %q", msg)
	}
}

// ---------- end-to-end search through HTTP ----------

func TestHandler_SearchDeliveriesE2E(t *testing.T) {
	s := newServerStack(t)
	now := time.Now().UTC()
	s.repo.DeliveriesResult = search.SearchResult[search.DeliveryDocument]{
		Items: []search.DeliveryDocument{{
			DeliveryID: "d1", CustomerID: "c1", Status: "PENDING",
			Pickup:    search.GeoAddress{City: "Cairo", Country: "EG", Location: search.GeoPoint{Lat: 1, Lon: 2}},
			Dropoff:   search.GeoAddress{City: "Giza", Country: "EG", Location: search.GeoPoint{Lat: 3, Lon: 4}},
			CreatedAt: now, UpdatedAt: now, SourceVersion: 1,
		}},
		PageInfo: search.PageInfo{Total: 1},
	}

	body, _ := json.Marshal(map[string]string{
		"query": `query { searchDeliveries(input: { query: "cairo", pagination: { limit: 10 } }) {
			success message
			data { items { delivery_id status pickup { city } } paginationInfo { totalItems currentPage } }
		} }`,
	})
	rec := doJSON(t, s.srv.Handler(), http.MethodPost, "/search/graphql", string(body), map[string]string{
		sharedconstants.HeaderXUserId:        "u1",
		sharedconstants.HeaderXUserRole:      "user",
		sharedconstants.HeaderXCorrelationId: "trace-1",
		"x-lang":                             "en",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if _, ok := m["errors"]; ok {
		t.Fatalf("errors = %v", m["errors"])
	}
	d := m["data"].(map[string]interface{})["searchDeliveries"].(map[string]interface{})
	if d["message"] != "Deliveries fetched successfully" {
		t.Errorf("message = %v", d["message"])
	}
	data := d["data"].(map[string]interface{})
	item := data["items"].([]interface{})[0].(map[string]interface{})
	if item["delivery_id"] != "d1" {
		t.Errorf("item = %+v", item)
	}
	pi := data["paginationInfo"].(map[string]interface{})
	if pi["totalItems"] != float64(1) || pi["currentPage"] != float64(1) {
		t.Errorf("pagination = %+v", pi)
	}
	if got := s.repo.DeliveriesQueries[0]; got.UserID != "u1" || got.UserRole != "user" {
		t.Errorf("query = %+v", got)
	}
}

func TestHandler_ArabicLangMessages(t *testing.T) {
	s := newServerStack(t)
	body, _ := json.Marshal(map[string]string{"query": searchHealthQuery})

	rec := doJSON(t, s.srv.Handler(), http.MethodPost, "/search/graphql", string(body), map[string]string{
		"Accept-Language": "ar",
	})
	m := decodeBody(t, rec)
	health := m["data"].(map[string]interface{})["searchHealth"].(map[string]interface{})
	if health["message"] != "خدمة البحث تعمل بصحة جيدة" {
		t.Errorf("message = %v", health["message"])
	}

	// query-string lang
	rec = doJSON(t, s.srv.Handler(), http.MethodPost, "/search/graphql?lang=ar", string(body), nil)
	m = decodeBody(t, rec)
	health = m["data"].(map[string]interface{})["searchHealth"].(map[string]interface{})
	if health["message"] != "خدمة البحث تعمل بصحة جيدة" {
		t.Errorf("message = %v", health["message"])
	}
}

func TestHandler_UnauthorizedOverHTTP(t *testing.T) {
	s := newServerStack(t)
	body, _ := json.Marshal(map[string]string{
		"query": `{ searchDeliveries(input: {}) { success } }`,
	})
	rec := doJSON(t, s.srv.Handler(), http.MethodPost, "/search/graphql", string(body), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	m := decodeBody(t, rec)
	errs, ok := m["errors"].([]interface{})
	if !ok || len(errs) == 0 {
		t.Fatalf("expected graphql errors, got %v", m)
	}
	msg := errs[0].(map[string]interface{})["message"].(string)
	if !strings.Contains(msg, "authentication required") {
		t.Errorf("message = %q", msg)
	}
}

func TestHandler_StartReindexMutationE2E(t *testing.T) {
	s := newServerStack(t)
	s.repo.ReindexDone = make(chan struct{})
	body, _ := json.Marshal(map[string]string{
		"query": `mutation { startReindex(index: "deliveries") {
			success message data { jobId index status startedAt completedAt error }
		} }`,
	})
	rec := doJSON(t, s.srv.Handler(), http.MethodPost, "/search/graphql", string(body), map[string]string{
		sharedconstants.HeaderXUserId:   "admin",
		sharedconstants.HeaderXUserRole: "admin",
	})
	m := decodeBody(t, rec)
	if _, ok := m["errors"]; ok {
		t.Fatalf("errors = %v", m["errors"])
	}
	data := m["data"].(map[string]interface{})["startReindex"].(map[string]interface{})
	if data["message"] != "Reindex job started successfully" {
		t.Errorf("message = %v", data["message"])
	}
	inner := data["data"].(map[string]interface{})
	if inner["status"] != "IN_PROGRESS" || inner["index"] != "deliveries" {
		t.Errorf("job = %+v", inner)
	}
	select {
	case <-s.repo.ReindexDone:
	case <-time.After(2 * time.Second):
		t.Fatal("reindex not invoked")
	}
}

// ---------- requestContext ----------

func TestRequestContext_PopulatesValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/search/graphql?lang=fr", nil)
	req.Header.Set(sharedconstants.HeaderXUserId, "u1")
	req.Header.Set(sharedconstants.HeaderXUserRole, "admin")
	req.Header.Set(sharedconstants.HeaderXCorrelationId, "trace-1")
	req.Header.Set("Authorization", "Bearer x.y.z")
	req.Header.Set("X-Lang", "ar")

	ctx := requestContext(req)

	if got := sharedGetUserID(ctx); got != "u1" {
		t.Errorf("userID = %q", got)
	}
	if v, _ := ctx.Value("x-user-role").(string); v != "admin" {
		t.Errorf("role = %q", v)
	}
	if v, _ := ctx.Value("authorization").(string); v != "Bearer x.y.z" {
		t.Errorf("auth = %q", v)
	}
	if ts, _ := ctx.Value("timestamp").(string); ts == "" {
		t.Error("missing timestamp")
	}
	// X-Lang header wins over query param in header precedence order:
	// header x-lang checked before query "lang".
	if lang := langOf(ctx); lang != "ar" {
		t.Errorf("lang = %q", lang)
	}
}

func TestRequestContext_LanguageFallbacks(t *testing.T) {
	// Query param lang when no headers.
	req := httptest.NewRequest(http.MethodGet, "/search/graphql?lang=ar", nil)
	if lang := langOf(requestContext(req)); lang != "ar" {
		t.Errorf("query lang = %q", lang)
	}

	// Accept-Language fallback.
	req = httptest.NewRequest(http.MethodGet, "/search/graphql", nil)
	req.Header.Set("Accept-Language", "ar-EG")
	if lang := langOf(requestContext(req)); lang != "ar" {
		t.Errorf("accept-language = %q", lang)
	}

	// Default when nothing set.
	req = httptest.NewRequest(http.MethodGet, "/search/graphql", nil)
	if lang := langOf(requestContext(req)); lang != "en" {
		t.Errorf("default lang = %q", lang)
	}
}

func sharedGetUserID(ctx context.Context) string { return sharedlogging.GetUserID(ctx) }

func langOf(ctx context.Context) string { return i18n.FromContext(ctx) }

// ---------- parseVariables / writeError ----------

func TestParseVariables(t *testing.T) {
	if v, err := parseVariables(""); err != nil || v != nil {
		t.Errorf("empty: %v %v", v, err)
	}
	v, err := parseVariables(`{"a":1}`)
	if err != nil || v["a"] != float64(1) {
		t.Errorf("valid: %v %v", v, err)
	}
	if _, err := parseVariables("{oops"); err == nil {
		t.Error("expected error")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, 418, errors2("teapot"))
	if rec.Code != 418 {
		t.Errorf("code = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("ct = %q", ct)
	}
	m := decodeBody(t, rec)
	msg := m["errors"].([]interface{})[0].(map[string]interface{})["message"].(string)
	if msg != "teapot" {
		t.Errorf("msg = %q", msg)
	}
}

type errors2 string

func (e errors2) Error() string { return string(e) }

func TestWriteJSON_EncodeErrorIsLoggedAndDoesNotPanic(t *testing.T) {
	rec := httptest.NewRecorder()
	// chan is not JSON-encodable → Encode fails; writeJSON must swallow the error.
	writeJSON(rec, make(chan int))
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("ct = %q", ct)
	}
}
