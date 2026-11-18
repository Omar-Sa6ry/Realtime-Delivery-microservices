package opensearch

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
)

type osFixture struct {
	repo *Repository
	srv  *testutil.FakeOpenSearch
	im   *IndexManager
}

func newOSFixture(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body []byte) bool) *osFixture {
	t.Helper()
	srv := testutil.NewFakeOpenSearch(t, handler)
	client, err := NewClient(testConfig(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	im := NewIndexManager(client)
	return &osFixture{repo: NewRepository(client, im), srv: srv, im: im}
}

// newClosedOSFixture builds a repository whose client points at a closed port
// (connection refused on every request).
func newClosedOSFixture(t *testing.T) *Repository {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	client, err := NewClient(testConfig("http://" + addr))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewRepository(client, NewIndexManager(client))
}

// ── Search operations ──────────────────────────────────────────────

func TestSearchDeliveries_Success(t *testing.T) {
	now := time.Now().UTC()
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/deliveries/_search") {
			testutil.WriteSearchResponse(w, 200, 1, map[string]interface{}{
				"_source": map[string]interface{}{
					"delivery_id":    "d1",
					"customer_id":    "c1",
					"driver_id":      "dr1",
					"status":         "PENDING",
					"pickup":         map[string]interface{}{"city": "Cairo", "country": "EG", "location": map[string]interface{}{"lat": 1.0, "lon": 2.0}},
					"dropoff":        map[string]interface{}{"city": "Giza", "country": "EG", "location": map[string]interface{}{"lat": 3.0, "lon": 4.0}},
					"created_at":     now.Format(time.RFC3339),
					"updated_at":     now.Format(time.RFC3339),
					"source_version": 1,
				},
				"sort": []interface{}{float64(1), "d1"},
			})
			return true
		}
		return false
	})

	res, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{
		Query:      "cairo",
		Status:     "PENDING",
		City:       "Cairo",
		DriverID:   "dr1",
		CustomerID: "c1",
		Pagination: search.PaginationInput{Limit: 1},
	})
	if err != nil {
		t.Fatalf("SearchDeliveries: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].DeliveryID != "d1" {
		t.Fatalf("items = %+v", res.Items)
	}
	if res.PageInfo.Total != 1 || !res.PageInfo.HasNextPage || res.PageInfo.Cursor == "" {
		t.Errorf("pageInfo = %+v", res.PageInfo)
	}
}

func TestSearchDeliveries_DefaultLimitAndDefaultSort(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{})
	if err != nil {
		t.Fatalf("SearchDeliveries: %v", err)
	}
	if body["size"] != float64(10) {
		t.Errorf("default size = %v", body["size"])
	}
	sort, ok := body["sort"].([]interface{})
	if !ok || len(sort) != 2 {
		t.Fatalf("default sort = %v", body["sort"])
	}
}

func TestSearchDeliveries_WithCursorSearchAfter(t *testing.T) {
	cursor, _ := search.EncodeCursor([]interface{}{float64(1), "abc"})
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{
		Pagination: search.PaginationInput{Cursor: cursor},
	})
	if err != nil {
		t.Fatalf("SearchDeliveries: %v", err)
	}
	sa, ok := body["search_after"].([]interface{})
	if !ok || len(sa) != 2 {
		t.Errorf("search_after = %v", body["search_after"])
	}
}

func TestSearchDeliveries_InvalidCursorIgnored(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{
		Pagination: search.PaginationInput{Cursor: "!!!bad!!!"},
	})
	if err != nil {
		t.Fatalf("SearchDeliveries: %v", err)
	}
	if _, has := body["search_after"]; has {
		t.Error("invalid cursor should not set search_after")
	}
}

func TestSearchDeliveries_EmptyCursorValuesIgnored(t *testing.T) {
	cursor, _ := search.EncodeCursor(nil)
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{
		Pagination: search.PaginationInput{Cursor: cursor},
	})
	if err != nil {
		t.Fatalf("SearchDeliveries: %v", err)
	}
	if _, has := body["search_after"]; has {
		t.Error("empty sort values should not set search_after")
	}
}

func TestSearchDeliveries_404AutoHeals(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodHead { // index exists check during EnsureIndices
			w.WriteHeader(http.StatusOK)
			return true
		}
		if strings.Contains(r.URL.Path, "/_search") {
			testutil.WriteJSON(w, http.StatusNotFound, map[string]interface{}{
				"error": map[string]interface{}{"type": "index_not_found_exception"},
			})
			return true
		}
		if r.Method == http.MethodPost && r.URL.Path == "/_aliases" {
			testutil.WriteJSON(w, http.StatusOK, map[string]interface{}{"acknowledged": true})
			return true
		}
		return false
	})
	res, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Items) != 0 || res.PageInfo.Total != 0 || res.PageInfo.HasNextPage {
		t.Errorf("expected empty result, got %+v", res)
	}
}

func TestSearchDeliveries_IndexNotFoundInBody(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			// 200 with index_not_found_exception still hits the non-200 branch? No — status 200.
			// Use 500 containing the marker to exercise strings.Contains path.
			testutil.WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error": map[string]interface{}{"type": "index_not_found_exception"},
			})
			return true
		}
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return true
		}
		if r.URL.Path == "/_aliases" {
			testutil.WriteJSON(w, http.StatusOK, map[string]interface{}{"acknowledged": true})
			return true
		}
		return false
	})
	res, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PageInfo.Total != 0 {
		t.Errorf("expected empty, got %+v", res)
	}
}

func TestSearchDeliveries_ServerError(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			testutil.WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "boom"})
			return true
		}
		return false
	})
	_, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{})
	if err == nil || !strings.Contains(err.Error(), "search error (500)") {
		t.Fatalf("expected search error, got %v", err)
	}
}

func TestSearchDeliveries_TransportError(t *testing.T) {
	fx := newOSFixture(t, nil)
	fx.srv.Close()
	_, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{})
	if err == nil || !strings.Contains(err.Error(), "search engine is temporarily unavailable") {
		t.Fatalf("expected ErrSearchUnavailable wrap, got %v", err)
	}
	if !strings.Contains(err.Error(), "search engine is temporarily unavailable") {
		t.Fatalf("error should wrap sentinel: %v", err)
	}
}

func TestSearchDeliveries_InvalidJSONResponse(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not-json"))
			return true
		}
		return false
	})
	_, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestSearchDeliveries_InvalidHitSourceSkipped(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[{"_source":{"delivery_id":123},"sort":[1]}]}}`))
			return true
		}
		return false
	})
	// delivery_id as number fails to unmarshal into string → item skipped
	res, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{})
	if err != nil {
		t.Fatalf("SearchDeliveries: %v", err)
	}
	if len(res.Items) != 0 {
		t.Errorf("expected 0 items after skip, got %d", len(res.Items))
	}
}

func TestSearchDeliveries_FullPageNoSortValues_NoCursor(t *testing.T) {
	// hasNextPage true (len==limit) but hit has no sort → no cursor
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"hits":{"total":{"value":10},"hits":[{"_source":{"delivery_id":"d1","customer_id":"c","status":"S","pickup":{"city":"a","country":"b","location":{"lat":0,"lon":0}},"dropoff":{"city":"a","country":"b","location":{"lat":0,"lon":0}},"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z","source_version":1}}]}}`))
			return true
		}
		return false
	})
	res, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{Pagination: search.PaginationInput{Limit: 1}})
	if err != nil {
		t.Fatalf("SearchDeliveries: %v", err)
	}
	if !res.PageInfo.HasNextPage {
		t.Error("expected hasNextPage")
	}
	if res.PageInfo.Cursor != "" {
		t.Errorf("expected empty cursor, got %q", res.PageInfo.Cursor)
	}
}

func TestSearchDeliveries_DateRangeAndGeoAndSort(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{
		FromDate: &from,
		ToDate:   &to,
		Geo:      &search.GeoDistanceFilter{Lat: 1, Lon: 2, RadiusKm: 5},
		Sort:     []search.DeliverySearchSort{{Field: "created_at", Order: search.SortDesc}},
	})
	if err != nil {
		t.Fatalf("SearchDeliveries: %v", err)
	}
	q := body["query"].(map[string]interface{})["bool"].(map[string]interface{})
	filters := q["filter"].([]interface{})
	if len(filters) != 2 {
		t.Fatalf("filters = %v", filters)
	}
	if _, ok := body["sort"].([]interface{}); !ok {
		t.Errorf("sort missing: %v", body["sort"])
	}
}

func TestSearchDeliveries_OnlyFromDate(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{FromDate: &from})
	if err != nil {
		t.Fatalf("SearchDeliveries: %v", err)
	}
	q := body["query"].(map[string]interface{})["bool"].(map[string]interface{})
	if len(q["filter"].([]interface{})) != 1 {
		t.Errorf("filters = %v", q["filter"])
	}
}

func TestSearchDrivers_SuccessWithAllFilters(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/drivers/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 1, map[string]interface{}{
				"_source": map[string]interface{}{
					"driver_id": "d1", "name": "Ali", "status": "AVAILABLE",
					"vehicle_type": "CAR", "rating": 4.5,
					"updated_at":     time.Now().UTC().Format(time.RFC3339),
					"source_version": 1,
				},
				"sort": []interface{}{"d1"},
			})
			return true
		}
		return false
	})
	res, err := fx.repo.SearchDrivers(context.Background(), search.DriverSearchQuery{
		Query:       "ali",
		Status:      "AVAILABLE",
		VehicleType: "CAR",
		MinRating:   4,
		Geo:         &search.GeoDistanceFilter{Lat: 1, Lon: 2, RadiusKm: 3},
		Sort:        []search.DriverSearchSort{{Field: "rating", Order: search.SortDesc}},
	})
	if err != nil {
		t.Fatalf("SearchDrivers: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].DriverID != "d1" {
		t.Errorf("items = %+v", res.Items)
	}
	q := body["query"].(map[string]interface{})["bool"].(map[string]interface{})
	if len(q["filter"].([]interface{})) != 4 {
		t.Errorf("filters = %v", q["filter"])
	}
	if _, ok := body["sort"]; !ok {
		t.Error("sort missing")
	}
}

func TestSearchDrivers_NoQueryMatchAll(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.SearchDrivers(context.Background(), search.DriverSearchQuery{Query: "   "})
	if err != nil {
		t.Fatalf("SearchDrivers: %v", err)
	}
	q := body["query"].(map[string]interface{})["bool"].(map[string]interface{})
	must := q["must"].(map[string]interface{})
	if _, ok := must["match_all"]; !ok {
		t.Errorf("expected match_all, got %v", must)
	}
}

func TestSearchMedia_Success(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/media/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 1, map[string]interface{}{
				"_source": map[string]interface{}{
					"media_id": "m1", "owner_id": "o1", "file_name": "a.png",
					"mime_type": "image/png", "media_type": "IMAGE", "status": "READY",
					"size": 100, "created_at": time.Now().UTC().Format(time.RFC3339),
					"source_version": 1,
				},
			})
			return true
		}
		return false
	})
	res, err := fx.repo.SearchMedia(context.Background(), search.MediaSearchQuery{
		Query:     "png",
		MediaType: "image",
		MimeType:  "image/png",
		OwnerID:   "o1",
	})
	if err != nil {
		t.Fatalf("SearchMedia: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].MediaID != "m1" {
		t.Errorf("items = %+v", res.Items)
	}
	q := body["query"].(map[string]interface{})["bool"].(map[string]interface{})
	filters := q["filter"].([]interface{})
	if len(filters) != 3 {
		t.Errorf("filters = %v", filters)
	}
	// MediaType should be uppercased in filter
	f0 := filters[0].(map[string]interface{})["term"].(map[string]interface{})
	if f0["media_type"] != "IMAGE" {
		t.Errorf("media_type filter = %v", f0["media_type"])
	}
	// sort always present on media
	if _, ok := body["sort"]; !ok {
		t.Error("media sort missing")
	}
}

func TestSearchMedia_NoQueryMatchAll(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.SearchMedia(context.Background(), search.MediaSearchQuery{})
	if err != nil {
		t.Fatalf("SearchMedia: %v", err)
	}
	q := body["query"].(map[string]interface{})["bool"].(map[string]interface{})
	must := q["must"].(map[string]interface{})
	if _, ok := must["match_all"]; !ok {
		t.Errorf("expected match_all, got %v", must)
	}
}

func TestSearchUsers_Success(t *testing.T) {
	active := true
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/users/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 1, map[string]interface{}{
				"_source": map[string]interface{}{
					"id": "u1", "first_name": "A", "last_name": "B",
					"email": "a@b.c", "role": "user", "is_active": true,
					"created_at": time.Now().UTC().Format(time.RFC3339),
				},
			})
			return true
		}
		return false
	})
	res, err := fx.repo.SearchUsers(context.Background(), search.UserSearchQuery{
		Query:    "a",
		Role:     "USER",
		IsActive: &active,
	})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "u1" {
		t.Errorf("items = %+v", res.Items)
	}
	q := body["query"].(map[string]interface{})["bool"].(map[string]interface{})
	filters := q["filter"].([]interface{})
	f0 := filters[0].(map[string]interface{})["term"].(map[string]interface{})
	if f0["role"] != "user" {
		t.Errorf("role should be lowercased, got %v", f0["role"])
	}
	if _, ok := body["sort"]; !ok {
		t.Error("users always sorted")
	}
}

func TestSearchUsers_NoQueryNoFilters(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.SearchUsers(context.Background(), search.UserSearchQuery{})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	q := body["query"].(map[string]interface{})["bool"].(map[string]interface{})
	must := q["must"].(map[string]interface{})
	if _, ok := must["match_all"]; !ok {
		t.Errorf("expected match_all, got %v", must)
	}
}

// ── Autocomplete ───────────────────────────────────────────────────

func TestAutocomplete_SuggestionPriority(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"hits":{"hits":[
				{"_source":{"name":"Driver Name"}},
				{"_source":{"first_name":"John","last_name":"Doe"}},
				{"_source":{"pickup":{"city":"Cairo"}}},
				{"_source":{"file_name":"photo.png"}},
				{"_source":{"other":"skip"}}
			]}}`))
			return true
		}
		return false
	})
	res, err := fx.repo.Autocomplete(context.Background(), search.AutocompleteQuery{Prefix: "x", Index: "drivers", Limit: 10})
	if err != nil {
		t.Fatalf("Autocomplete: %v", err)
	}
	want := []string{"Driver Name", "John Doe", "Cairo", "photo.png"}
	if len(res.Suggestions) != len(want) {
		t.Fatalf("suggestions = %v", res.Suggestions)
	}
	for i, w := range want {
		if res.Suggestions[i] != w {
			t.Errorf("suggestions[%d] = %q, want %q", i, res.Suggestions[i], w)
		}
	}
}

func TestAutocomplete_DefaultLimit(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.Autocomplete(context.Background(), search.AutocompleteQuery{Prefix: "x", Index: "media"})
	if err != nil {
		t.Fatalf("Autocomplete: %v", err)
	}
	if body["size"] != float64(10) {
		t.Errorf("default size = %v", body["size"])
	}
}

func TestAutocomplete_404AutoHeals(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return true
		}
		if r.URL.Path == "/_aliases" {
			testutil.WriteJSON(w, http.StatusOK, map[string]interface{}{"acknowledged": true})
			return true
		}
		return false
	})
	res, err := fx.repo.Autocomplete(context.Background(), search.AutocompleteQuery{Index: "media"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Suggestions == nil || len(res.Suggestions) != 0 {
		t.Errorf("expected empty suggestions, got %v", res.Suggestions)
	}
}

func TestAutocomplete_ServerError(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			testutil.WriteJSON(w, http.StatusBadGateway, map[string]interface{}{"error": "x"})
			return true
		}
		return false
	})
	_, err := fx.repo.Autocomplete(context.Background(), search.AutocompleteQuery{Index: "media"})
	if err == nil || !strings.Contains(err.Error(), "opensearch returned status: 502") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestAutocomplete_TransportError(t *testing.T) {
	fx := newOSFixture(t, nil)
	fx.srv.Close()
	_, err := fx.repo.Autocomplete(context.Background(), search.AutocompleteQuery{Index: "media"})
	if err == nil {
		t.Fatal("expected transport error")
	}
}

func TestAutocomplete_DecodeError(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not-json"))
			return true
		}
		return false
	})
	_, err := fx.repo.Autocomplete(context.Background(), search.AutocompleteQuery{Index: "media"})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestAutocomplete_InvalidHitSourceSkipped(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_search") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"hits":{"hits":[{"_source":"plain-string"},{"_source":{"name":123}}]}}`))
			return true
		}
		return false
	})
	res, err := fx.repo.Autocomplete(context.Background(), search.AutocompleteQuery{Index: "media"})
	if err != nil {
		t.Fatalf("Autocomplete: %v", err)
	}
	if len(res.Suggestions) != 0 {
		t.Errorf("expected no suggestions, got %v", res.Suggestions)
	}
}

// ── Nearby ─────────────────────────────────────────────────────────

func TestNearbyDeliveries(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/deliveries/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.NearbyDeliveries(context.Background(), search.GeoSearchQuery{Lat: 30, Lon: 31, RadiusKm: 10})
	if err != nil {
		t.Fatalf("NearbyDeliveries: %v", err)
	}
	if _, ok := body["sort"].([]interface{}); !ok {
		t.Errorf("geo sort missing: %v", body["sort"])
	}
	q := body["query"].(map[string]interface{})["bool"].(map[string]interface{})
	if len(q["filter"].([]interface{})) != 1 {
		t.Errorf("filters = %v", q["filter"])
	}
}

func TestNearbyDrivers_IncludesAvailableFilter(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if strings.Contains(r.URL.Path, "/drivers/_search") {
			_ = json.Unmarshal(b, &body)
			testutil.WriteSearchResponse(w, 200, 0)
			return true
		}
		return false
	})
	_, err := fx.repo.NearbyDrivers(context.Background(), search.GeoSearchQuery{Lat: 30, Lon: 31, RadiusKm: 7})
	if err != nil {
		t.Fatalf("NearbyDrivers: %v", err)
	}
	q := body["query"].(map[string]interface{})["bool"].(map[string]interface{})
	filters := q["filter"].([]interface{})
	if len(filters) != 2 {
		t.Fatalf("filters = %v", filters)
	}
	f0 := filters[0].(map[string]interface{})["term"].(map[string]interface{})
	if f0["status"] != "AVAILABLE" {
		t.Errorf("status filter = %v", f0["status"])
	}
}

// ── Indexing ───────────────────────────────────────────────────────

func TestUpsertDelivery_NewDoc(t *testing.T) {
	var indexed bool
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/deliveries/_doc/") {
			w.WriteHeader(http.StatusNotFound) // no existing version
			return true
		}
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/deliveries/_doc/") {
			indexed = true
			testutil.WriteJSON(w, http.StatusOK, map[string]interface{}{"result": "created"})
			return true
		}
		return false
	})
	err := fx.repo.UpsertDelivery(context.Background(), search.DeliveryDocument{
		DeliveryID: "d1", SourceVersion: 1,
	})
	if err != nil {
		t.Fatalf("UpsertDelivery: %v", err)
	}
	if !indexed {
		t.Fatal("document was not indexed")
	}
}

func TestUpsertDelivery_StaleVersionSkipped(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_doc/") {
			testutil.WriteJSON(w, http.StatusOK, map[string]interface{}{
				"_source": map[string]interface{}{"source_version": 5},
			})
			return true
		}
		return false
	})
	err := fx.repo.UpsertDelivery(context.Background(), search.DeliveryDocument{
		DeliveryID: "d1", SourceVersion: 3,
	})
	if err != nil {
		t.Fatalf("stale upsert should return nil, got %v", err)
	}
}

func TestUpsertDelivery_IndexError(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		if r.Method == http.MethodPut {
			testutil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "bad"})
			return true
		}
		return false
	})
	err := fx.repo.UpsertDelivery(context.Background(), search.DeliveryDocument{DeliveryID: "d1", SourceVersion: 1})
	if err == nil || !strings.Contains(err.Error(), "indexing error (400)") {
		t.Fatalf("expected indexing error, got %v", err)
	}
}

func TestUpsertDelivery_TransportErrorOnIndex(t *testing.T) {
	// Every request (version GET + index PUT) hits a closed port.
	repo := newClosedOSFixture(t)
	err := repo.UpsertDelivery(context.Background(), search.DeliveryDocument{DeliveryID: "d1", SourceVersion: 1})
	if err == nil || !strings.Contains(err.Error(), search.ErrSearchUnavailable.Error()) {
		t.Fatalf("expected ErrSearchUnavailable, got %v", err)
	}
}

func TestReindex_TransportError(t *testing.T) {
	repo := newClosedOSFixture(t)
	err := repo.Reindex(context.Background(), "a-v1", "a-v2")
	if err == nil {
		t.Fatal("expected transport error from Reindex")
	}
}

func TestSearchDeliveries_ExecuteSearchTransportError(t *testing.T) {
	repo := newClosedOSFixture(t)
	_, err := repo.SearchDeliveries(context.Background(), search.DeliverySearchQuery{})
	if err == nil || !strings.Contains(err.Error(), search.ErrSearchUnavailable.Error()) {
		t.Fatalf("expected ErrSearchUnavailable, got %v", err)
	}
}

func TestUpsertDelivery_VersionGetErrorStillIndexes(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("bad"))
			return true
		}
		return false
	})
	err := fx.repo.UpsertDelivery(context.Background(), search.DeliveryDocument{DeliveryID: "d1", SourceVersion: 1})
	if err != nil {
		t.Fatalf("version get error should not block upsert: %v", err)
	}
}

func TestUpsertDriver(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		return false
	})
	err := fx.repo.UpsertDriver(context.Background(), search.DriverDocument{DriverID: "d1", SourceVersion: 2})
	if err != nil {
		t.Fatalf("UpsertDriver: %v", err)
	}
}

func TestUpsertUser_AlwaysVersion1(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if r.Method == http.MethodGet {
			// existing version 1 → current >= incoming(1) → skip? currentVersion(1) >= 1 → skip!
			// To force index, return 404.
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		if r.Method == http.MethodPut {
			_ = json.Unmarshal(b, &body)
			testutil.WriteJSON(w, 200, map[string]interface{}{"result": "created"})
			return true
		}
		return false
	})
	err := fx.repo.UpsertUser(context.Background(), search.UserDocument{ID: "u1"})
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if body == nil {
		t.Fatal("no body")
	}
}

func TestUpsertMedia_EmptyFileNameInherits(t *testing.T) {
	var lastBody []byte
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/media/_doc/") {
			testutil.WriteJSON(w, http.StatusOK, map[string]interface{}{
				"_source": map[string]interface{}{"file_name": "existing.png", "size": 42},
			})
			return true
		}
		if r.Method == http.MethodPut {
			lastBody = b
			testutil.WriteJSON(w, 200, map[string]interface{}{"result": "created"})
			return true
		}
		return false
	})
	err := fx.repo.UpsertMedia(context.Background(), search.MediaDocument{
		MediaID: "m1", SourceVersion: 1, // FileName empty, Size 0
	})
	if err != nil {
		t.Fatalf("UpsertMedia: %v", err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(lastBody, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc["file_name"] != "existing.png" {
		t.Errorf("file_name = %v", doc["file_name"])
	}
	if doc["size"] != float64(42) {
		t.Errorf("size = %v", doc["size"])
	}
}

func TestUpsertMedia_EmptyFileNameGetFails(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusInternalServerError)
			return true
		}
		return false
	})
	err := fx.repo.UpsertMedia(context.Background(), search.MediaDocument{MediaID: "m1", SourceVersion: 1})
	if err != nil {
		t.Fatalf("get failure should be ignored: %v", err)
	}
}

func TestUpsertMedia_ExistingBadJSONIgnored(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not-json"))
			return true
		}
		return false
	})
	err := fx.repo.UpsertMedia(context.Background(), search.MediaDocument{MediaID: "m1", SourceVersion: 1})
	if err != nil {
		t.Fatalf("bad existing JSON should be ignored: %v", err)
	}
}

func TestUpsertMedia_WithFileNameSkipsGet(t *testing.T) {
	var gets int
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet {
			gets++
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		return false
	})
	err := fx.repo.UpsertMedia(context.Background(), search.MediaDocument{
		MediaID: "m1", FileName: "x.png", Size: 1, SourceVersion: 1,
	})
	if err != nil {
		t.Fatalf("UpsertMedia: %v", err)
	}
	if gets != 1 { // only the version-check GET
		t.Errorf("GET count = %d, want 1", gets)
	}
}

func TestDeleteDoc_AllIndices(t *testing.T) {
	tests := []struct {
		name string
		call func(*Repository) error
		path string
	}{
		{"delivery", func(r *Repository) error { return r.DeleteDelivery(ctxbg(), "id") }, "/deliveries/_doc/id"},
		{"driver", func(r *Repository) error { return r.DeleteDriver(ctxbg(), "id") }, "/drivers/_doc/id"},
		{"media", func(r *Repository) error { return r.DeleteMedia(ctxbg(), "id") }, "/media/_doc/id"},
		{"user", func(r *Repository) error { return r.DeleteUser(ctxbg(), "id") }, "/users/_doc/id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusNotFound) // idempotent
					return true
				}
				return false
			})
			if err := tt.call(fx.repo); err != nil {
				t.Fatalf("delete: %v", err)
			}
		})
	}
}

func TestDeleteDoc_Success(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodDelete {
			testutil.WriteJSON(w, 200, map[string]interface{}{"result": "deleted"})
			return true
		}
		return false
	})
	if err := fx.repo.DeleteDelivery(ctxbg(), "id"); err != nil {
		t.Fatalf("DeleteDelivery: %v", err)
	}
}

func TestDeleteDoc_ErrorStatus(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodDelete {
			testutil.WriteJSON(w, 500, map[string]interface{}{"error": "x"})
			return true
		}
		return false
	})
	err := fx.repo.DeleteDelivery(ctxbg(), "id")
	if err == nil || !strings.Contains(err.Error(), "delete failed with status 500") {
		t.Fatalf("expected delete error, got %v", err)
	}
}

func TestDeleteDoc_TransportError(t *testing.T) {
	fx := newOSFixture(t, nil)
	fx.srv.Close()
	err := fx.repo.DeleteDelivery(ctxbg(), "id")
	if err == nil {
		t.Fatal("expected transport error")
	}
}

func TestBulkUpsertDeliveries(t *testing.T) {
	var body string
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if r.Method == http.MethodPost && r.URL.Path == "/_bulk" {
			body = string(b)
			testutil.WriteJSON(w, 200, map[string]interface{}{"errors": false})
			return true
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		return false
	})
	docs := []search.DeliveryDocument{
		{DeliveryID: "d1", SourceVersion: 1},
		{DeliveryID: "d2", SourceVersion: 1},
	}
	if err := fx.repo.BulkUpsertDeliveries(ctxbg(), docs); err != nil {
		t.Fatalf("BulkUpsertDeliveries: %v", err)
	}
	if !strings.Contains(body, `{"index":{"_index":"deliveries","_id":"d1"}}`) {
		t.Errorf("bulk body = %s", body)
	}
	if strings.Count(body, "\n") < 4 {
		t.Errorf("expected NDJSON pairs, got: %s", body)
	}
}

func TestBulkUpsertDriversAndMedia(t *testing.T) {
	var paths []string
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodPost && r.URL.Path == "/_bulk" {
			paths = append(paths, r.URL.Path)
			testutil.WriteJSON(w, 200, map[string]interface{}{"errors": false})
			return true
		}
		return false
	})
	if err := fx.repo.BulkUpsertDrivers(ctxbg(), []search.DriverDocument{{DriverID: "d1"}}); err != nil {
		t.Fatalf("BulkUpsertDrivers: %v", err)
	}
	if err := fx.repo.BulkUpsertMedia(ctxbg(), []search.MediaDocument{{MediaID: "m1"}}); err != nil {
		t.Fatalf("BulkUpsertMedia: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("bulk calls = %d", len(paths))
	}
}

func TestBulkUpsert_EmptyDocsStillCallsBulk(t *testing.T) {
	var bulkCalls int
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodPost && r.URL.Path == "/_bulk" {
			bulkCalls++
			testutil.WriteJSON(w, 200, map[string]interface{}{})
			return true
		}
		return false
	})
	if err := fx.repo.BulkUpsertDeliveries(ctxbg(), nil); err != nil {
		t.Fatalf("empty bulk: %v", err)
	}
	if bulkCalls != 1 {
		t.Errorf("bulk calls = %d", bulkCalls)
	}
}

func TestBulk_ErrorStatus(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.URL.Path == "/_bulk" {
			testutil.WriteJSON(w, 400, map[string]interface{}{"errors": true})
			return true
		}
		return false
	})
	err := fx.repo.BulkUpsertDeliveries(ctxbg(), []search.DeliveryDocument{{DeliveryID: "d"}})
	if err == nil || !strings.Contains(err.Error(), "bulk indexing failed (400)") {
		t.Fatalf("expected bulk error, got %v", err)
	}
}

func TestBulk_TransportError(t *testing.T) {
	fx := newOSFixture(t, nil)
	fx.srv.Close()
	err := fx.repo.BulkUpsertDeliveries(ctxbg(), []search.DeliveryDocument{{DeliveryID: "d"}})
	if err == nil {
		t.Fatal("expected transport error")
	}
}

// ── Index management ───────────────────────────────────────────────

func TestGetDocumentVersion(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/_doc/") {
			testutil.WriteJSON(w, 200, map[string]interface{}{
				"_source": map[string]interface{}{"source_version": 7},
			})
			return true
		}
		return false
	})
	v, err := fx.repo.GetDocumentVersion(ctxbg(), "deliveries", "d1")
	if err != nil {
		t.Fatalf("GetDocumentVersion: %v", err)
	}
	if v != 7 {
		t.Errorf("version = %d", v)
	}
}

func TestGetDocumentVersion_NotFound(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		return false
	})
	v, err := fx.repo.GetDocumentVersion(ctxbg(), "deliveries", "d1")
	if err != nil || v != -1 {
		t.Fatalf("v=%d err=%v", v, err)
	}
}

func TestGetDocumentVersion_DecodeError(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not-json"))
			return true
		}
		return false
	})
	_, err := fx.repo.GetDocumentVersion(ctxbg(), "deliveries", "d1")
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestGetDocumentVersion_TransportError(t *testing.T) {
	fx := newOSFixture(t, nil)
	fx.srv.Close()
	_, err := fx.repo.GetDocumentVersion(ctxbg(), "deliveries", "d1")
	if err == nil {
		t.Fatal("expected transport error")
	}
}

func TestIndexExists(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return true
		}
		return false
	})
	ok, err := fx.repo.IndexExists(ctxbg(), "deliveries")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}

	fx2 := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		return false
	})
	ok, err = fx2.repo.IndexExists(ctxbg(), "nope")
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}

	fx3 := newOSFixture(t, nil)
	fx3.srv.Close()
	_, err = fx3.repo.IndexExists(ctxbg(), "x")
	if err == nil {
		t.Fatal("expected transport error")
	}
}

func TestCountDocuments(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.HasSuffix(r.URL.Path, "/_count") {
			testutil.WriteJSON(w, 200, map[string]interface{}{"count": 42})
			return true
		}
		return false
	})
	n, err := fx.repo.CountDocuments(ctxbg(), "deliveries")
	if err != nil || n != 42 {
		t.Fatalf("n=%d err=%v", n, err)
	}

	fx2 := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.HasSuffix(r.URL.Path, "/_count") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("bad"))
			return true
		}
		return false
	})
	_, err = fx2.repo.CountDocuments(ctxbg(), "deliveries")
	if err == nil {
		t.Fatal("expected decode error")
	}

	fx3 := newOSFixture(t, nil)
	fx3.srv.Close()
	_, err = fx3.repo.CountDocuments(ctxbg(), "x")
	if err == nil {
		t.Fatal("expected transport error")
	}
}

func TestReindex(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if r.URL.Path == "/_reindex" {
			_ = json.Unmarshal(b, &body)
			testutil.WriteJSON(w, 200, map[string]interface{}{"created": 1})
			return true
		}
		return false
	})
	if err := fx.repo.Reindex(ctxbg(), "src", "dst"); err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	src := body["source"].(map[string]interface{})
	dst := body["dest"].(map[string]interface{})
	if src["index"] != "src" || dst["index"] != "dst" {
		t.Errorf("body = %v", body)
	}

	fx2 := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.URL.Path == "/_reindex" {
			testutil.WriteJSON(w, 400, map[string]interface{}{"error": "x"})
			return true
		}
		return false
	})
	err := fx2.repo.Reindex(ctxbg(), "s", "d")
	if err == nil || !strings.Contains(err.Error(), "reindex failed") {
		t.Fatalf("expected reindex error, got %v", err)
	}

	fx3 := newOSFixture(t, nil)
	fx3.srv.Close()
	if err := fx3.repo.Reindex(ctxbg(), "s", "d"); err == nil {
		t.Fatal("expected transport error")
	}
}

func TestSwitchAlias_WithOldIndex(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if r.URL.Path == "/_aliases" {
			_ = json.Unmarshal(b, &body)
			testutil.WriteJSON(w, 200, map[string]interface{}{"acknowledged": true})
			return true
		}
		return false
	})
	if err := fx.repo.SwitchAlias(ctxbg(), "alias", "old", "new"); err != nil {
		t.Fatalf("SwitchAlias: %v", err)
	}
	actions := body["actions"].([]interface{})
	if len(actions) != 2 {
		t.Fatalf("actions = %v", actions)
	}
}

func TestSwitchAlias_WithoutOldIndex(t *testing.T) {
	var body map[string]interface{}
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		if r.URL.Path == "/_aliases" {
			_ = json.Unmarshal(b, &body)
			testutil.WriteJSON(w, 200, map[string]interface{}{"acknowledged": true})
			return true
		}
		return false
	})
	if err := fx.repo.SwitchAlias(ctxbg(), "alias", "", "new"); err != nil {
		t.Fatalf("SwitchAlias: %v", err)
	}
	actions := body["actions"].([]interface{})
	if len(actions) != 1 {
		t.Fatalf("actions = %v", actions)
	}

	fx2 := newOSFixture(t, nil)
	fx2.srv.Close()
	if err := fx2.repo.SwitchAlias(ctxbg(), "alias", "", "new"); err == nil {
		t.Fatal("expected transport error")
	}
}

func TestPing(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return true
		}
		return false
	})
	if err := fx.repo.Ping(ctxbg()); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	fx2 := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusServiceUnavailable)
			return true
		}
		return false
	})
	err := fx2.repo.Ping(ctxbg())
	if err == nil || !strings.Contains(err.Error(), "ping returned 503") {
		t.Fatalf("expected ping error, got %v", err)
	}

	fx3 := newOSFixture(t, nil)
	fx3.srv.Close()
	if err := fx3.repo.Ping(ctxbg()); err == nil {
		t.Fatal("expected transport error")
	}
}

func TestClusterHealth(t *testing.T) {
	fx := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_cluster/health") {
			testutil.WriteJSON(w, 200, map[string]interface{}{"status": "green"})
			return true
		}
		return false
	})
	status, err := fx.repo.ClusterHealth(ctxbg())
	if err != nil || status != "green" {
		t.Fatalf("status=%q err=%v", status, err)
	}

	fx2 := newOSFixture(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "/_cluster/health") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("bad"))
			return true
		}
		return false
	})
	status, err = fx2.repo.ClusterHealth(ctxbg())
	if err == nil || status != "unknown" {
		t.Fatalf("status=%q err=%v", status, err)
	}

	fx3 := newOSFixture(t, nil)
	fx3.srv.Close()
	status, err = fx3.repo.ClusterHealth(ctxbg())
	if err == nil || status != "red" {
		t.Fatalf("status=%q err=%v", status, err)
	}
}

func TestRepositoryImplementsInterface(t *testing.T) {
	var _ search.SearchRepository = (*Repository)(nil)
}

func ctxbg() context.Context { return context.Background() }
