package testutil

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
)

func TestFakeRepository_SearchCallsRecordAndReturnConfigured(t *testing.T) {
	f := NewFakeRepository()
	ctx := context.Background()

	f.DeliveriesResult = search.SearchResult[search.DeliveryDocument]{
		Items: []search.DeliveryDocument{{DeliveryID: "d1"}},
	}
	if _, err := f.SearchDeliveries(ctx, search.DeliverySearchQuery{Query: "q"}); err != nil {
		t.Fatal(err)
	}
	if len(f.DeliveriesQueries) != 1 || f.DeliveriesQueries[0].Query != "q" {
		t.Errorf("DeliveriesQueries = %v", f.DeliveriesQueries)
	}

	f.DeliveriesErr = errors.New("boom")
	if _, err := f.SearchDeliveries(ctx, search.DeliverySearchQuery{}); err == nil {
		t.Error("expected DeliveriesErr")
	}

	if _, err := f.SearchDrivers(ctx, search.DriverSearchQuery{}); err != nil {
		t.Fatal(err)
	}
	if len(f.DriversQueries) != 1 {
		t.Errorf("DriversQueries = %v", f.DriversQueries)
	}
	f.DriversErr = errors.New("boom")
	if _, err := f.SearchDrivers(ctx, search.DriverSearchQuery{}); err == nil {
		t.Error("expected DriversErr")
	}

	if _, err := f.SearchMedia(ctx, search.MediaSearchQuery{}); err != nil {
		t.Fatal(err)
	}
	if len(f.MediaQueries) != 1 {
		t.Errorf("MediaQueries = %v", f.MediaQueries)
	}
	f.MediaErr = errors.New("boom")
	if _, err := f.SearchMedia(ctx, search.MediaSearchQuery{}); err == nil {
		t.Error("expected MediaErr")
	}

	if _, err := f.SearchUsers(ctx, search.UserSearchQuery{}); err != nil {
		t.Fatal(err)
	}
	if len(f.UsersQueries) != 1 {
		t.Errorf("UsersQueries = %v", f.UsersQueries)
	}
	f.UsersErr = errors.New("boom")
	if _, err := f.SearchUsers(ctx, search.UserSearchQuery{}); err == nil {
		t.Error("expected UsersErr")
	}

	if _, err := f.Autocomplete(ctx, search.AutocompleteQuery{Prefix: "a"}); err != nil {
		t.Fatal(err)
	}
	if len(f.AutocompleteQs) != 1 || f.AutocompleteQs[0].Prefix != "a" {
		t.Errorf("AutocompleteQs = %v", f.AutocompleteQs)
	}
	f.AutocompleteErr = errors.New("boom")
	if _, err := f.Autocomplete(ctx, search.AutocompleteQuery{}); err == nil {
		t.Error("expected AutocompleteErr")
	}

	if _, err := f.NearbyDeliveries(ctx, search.GeoSearchQuery{}); err != nil {
		t.Fatal(err)
	}
	if len(f.NearbyDelivQs) != 1 {
		t.Errorf("NearbyDelivQs = %v", f.NearbyDelivQs)
	}
	f.NearbyDelivErr = errors.New("boom")
	if _, err := f.NearbyDeliveries(ctx, search.GeoSearchQuery{}); err == nil {
		t.Error("expected NearbyDelivErr")
	}

	if _, err := f.NearbyDrivers(ctx, search.GeoSearchQuery{}); err != nil {
		t.Fatal(err)
	}
	if len(f.NearbyDriversQs) != 1 {
		t.Errorf("NearbyDriversQs = %v", f.NearbyDriversQs)
	}
	f.NearbyDriversErr = errors.New("boom")
	if _, err := f.NearbyDrivers(ctx, search.GeoSearchQuery{}); err == nil {
		t.Error("expected NearbyDriversErr")
	}
}

func TestFakeRepository_IndexingCalls(t *testing.T) {
	f := NewFakeRepository()
	ctx := context.Background()

	if err := f.UpsertDelivery(ctx, search.DeliveryDocument{DeliveryID: "d1"}); err != nil {
		t.Fatal(err)
	}
	if err := f.DeleteDelivery(ctx, "d1"); err != nil {
		t.Fatal(err)
	}
	if err := f.UpsertDriver(ctx, search.DriverDocument{DriverID: "dr1"}); err != nil {
		t.Fatal(err)
	}
	if err := f.DeleteDriver(ctx, "dr1"); err != nil {
		t.Fatal(err)
	}
	if err := f.UpsertMedia(ctx, search.MediaDocument{MediaID: "m1"}); err != nil {
		t.Fatal(err)
	}
	if err := f.DeleteMedia(ctx, "m1"); err != nil {
		t.Fatal(err)
	}
	if err := f.UpsertUser(ctx, search.UserDocument{ID: "u1"}); err != nil {
		t.Fatal(err)
	}
	if err := f.DeleteUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}

	if len(f.UpsertDeliveries) != 1 || f.UpsertDeliveries[0].DeliveryID != "d1" {
		t.Errorf("UpsertDeliveries = %v", f.UpsertDeliveries)
	}
	if len(f.DeletedDeliveries) != 1 || f.DeletedDeliveries[0] != "d1" {
		t.Errorf("DeletedDeliveries = %v", f.DeletedDeliveries)
	}
	if len(f.UpsertDrivers) != 1 || f.DeletedDrivers[0] != "dr1" {
		t.Errorf("drivers = %v / %v", f.UpsertDrivers, f.DeletedDrivers)
	}
	if len(f.UpsertMediaDocs) != 1 || f.DeletedMedia[0] != "m1" {
		t.Errorf("media = %v / %v", f.UpsertMediaDocs, f.DeletedMedia)
	}
	if len(f.UpsertUsers) != 1 || f.DeletedUsers[0] != "u1" {
		t.Errorf("users = %v / %v", f.UpsertUsers, f.DeletedUsers)
	}

	// Error paths
	errPairs := []struct {
		set  func()
		call func() error
		name string
	}{
		{func() { f.UpsertDeliveryErr = errors.New("x") }, func() error { return f.UpsertDelivery(ctx, search.DeliveryDocument{}) }, "UpsertDeliveryErr"},
		{func() { f.DeleteDeliveryErr = errors.New("x") }, func() error { return f.DeleteDelivery(ctx, "id") }, "DeleteDeliveryErr"},
		{func() { f.UpsertDriverErr = errors.New("x") }, func() error { return f.UpsertDriver(ctx, search.DriverDocument{}) }, "UpsertDriverErr"},
		{func() { f.DeleteDriverErr = errors.New("x") }, func() error { return f.DeleteDriver(ctx, "id") }, "DeleteDriverErr"},
		{func() { f.UpsertMediaErr = errors.New("x") }, func() error { return f.UpsertMedia(ctx, search.MediaDocument{}) }, "UpsertMediaErr"},
		{func() { f.DeleteMediaErr = errors.New("x") }, func() error { return f.DeleteMedia(ctx, "id") }, "DeleteMediaErr"},
		{func() { f.UpsertUserErr = errors.New("x") }, func() error { return f.UpsertUser(ctx, search.UserDocument{}) }, "UpsertUserErr"},
		{func() { f.DeleteUserErr = errors.New("x") }, func() error { return f.DeleteUser(ctx, "id") }, "DeleteUserErr"},
	}
	for _, p := range errPairs {
		p.set()
		if err := p.call(); err == nil {
			t.Errorf("%s should surface", p.name)
		}
	}
}

func TestFakeRepository_BulkUpserts(t *testing.T) {
	f := NewFakeRepository()
	ctx := context.Background()

	if err := f.BulkUpsertDeliveries(ctx, []search.DeliveryDocument{{DeliveryID: "d1"}, {DeliveryID: "d2"}}); err != nil {
		t.Fatal(err)
	}
	if err := f.BulkUpsertDrivers(ctx, []search.DriverDocument{{DriverID: "dr1"}}); err != nil {
		t.Fatal(err)
	}
	if err := f.BulkUpsertMedia(ctx, []search.MediaDocument{{MediaID: "m1"}}); err != nil {
		t.Fatal(err)
	}
	if len(f.UpsertDeliveries) != 2 || len(f.UpsertDrivers) != 1 || len(f.UpsertMediaDocs) != 1 {
		t.Errorf("bulk append failed: %d %d %d", len(f.UpsertDeliveries), len(f.UpsertDrivers), len(f.UpsertMediaDocs))
	}

	f.BulkDeliveriesErr = errors.New("x")
	if err := f.BulkUpsertDeliveries(ctx, nil); err == nil {
		t.Error("expected BulkDeliveriesErr")
	}
	f.BulkDriversErr = errors.New("x")
	if err := f.BulkUpsertDrivers(ctx, nil); err == nil {
		t.Error("expected BulkDriversErr")
	}
	f.BulkMediaErr = errors.New("x")
	if err := f.BulkUpsertMedia(ctx, nil); err == nil {
		t.Error("expected BulkMediaErr")
	}
}

func TestFakeRepository_IndexManagement(t *testing.T) {
	f := NewFakeRepository()
	ctx := context.Background()

	f.Version = 7
	if v, err := f.GetDocumentVersion(ctx, "i", "id"); err != nil || v != 7 {
		t.Errorf("version = %d, %v", v, err)
	}
	f.VersionErr = errors.New("x")
	if _, err := f.GetDocumentVersion(ctx, "i", "id"); err == nil {
		t.Error("expected VersionErr")
	}

	f.Exists = true
	if ok, err := f.IndexExists(ctx, "i"); err != nil || !ok {
		t.Errorf("exists = %v, %v", ok, err)
	}
	f.ExistsErr = errors.New("x")
	if _, err := f.IndexExists(ctx, "i"); err == nil {
		t.Error("expected ExistsErr")
	}

	f.Count = 42
	if n, err := f.CountDocuments(ctx, "i"); err != nil || n != 42 {
		t.Errorf("count = %d, %v", n, err)
	}
	f.CountErr = errors.New("x")
	if _, err := f.CountDocuments(ctx, "i"); err == nil {
		t.Error("expected CountErr")
	}

	f.ReindexDone = make(chan struct{})
	if err := f.Reindex(ctx, "a", "b"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.ReindexDone:
	default:
		t.Error("ReindexDone not closed")
	}
	// Second call with closed channel must not panic.
	if err := f.Reindex(ctx, "a", "c"); err != nil {
		t.Fatal(err)
	}
	if len(f.ReindexCalls) != 2 || f.ReindexCalls[0] != [2]string{"a", "b"} || f.ReindexCalls[1] != [2]string{"a", "c"} {
		t.Errorf("ReindexCalls = %v", f.ReindexCalls)
	}
	f.ReindexErr = errors.New("x")
	if err := f.Reindex(ctx, "a", "b"); err == nil {
		t.Error("expected ReindexErr")
	}

	if err := f.SwitchAlias(ctx, "alias", "old", "new"); err != nil {
		t.Fatal(err)
	}
	if len(f.SwitchAliasCalls) != 1 || f.SwitchAliasCalls[0] != [3]string{"alias", "old", "new"} {
		t.Errorf("SwitchAliasCalls = %v", f.SwitchAliasCalls)
	}
	f.AliasErr = errors.New("x")
	if err := f.SwitchAlias(ctx, "alias", "old", "new"); err == nil {
		t.Error("expected AliasErr")
	}

	if err := f.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	f.PingErr = errors.New("x")
	if err := f.Ping(ctx); err == nil {
		t.Error("expected PingErr")
	}

	if h, err := f.ClusterHealth(ctx); err != nil || h != "green" {
		t.Errorf("health = %q, %v", h, err)
	}
	f.HealthErr = errors.New("x")
	if _, err := f.ClusterHealth(ctx); err == nil {
		t.Error("expected HealthErr")
	}
}

func TestFakeOpenSearch_DefaultBehaviour(t *testing.T) {
	f := NewFakeOpenSearch(t, nil)
	resp, err := http.Get(f.URL + "/_cluster/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
	reqs := f.Requests()
	if len(reqs) != 1 || reqs[0].Method != http.MethodGet || reqs[0].Path != "/_cluster/health" {
		t.Errorf("requests = %+v", reqs)
	}
	last, ok := f.LastRequest()
	if !ok || last.Path != "/_cluster/health" {
		t.Errorf("last = %+v %v", last, ok)
	}
	f.Reset()
	if _, ok := f.LastRequest(); ok {
		t.Error("Reset did not clear requests")
	}
}

func TestFakeOpenSearch_ProgrammableHandler(t *testing.T) {
	f := NewFakeOpenSearch(t, nil)
	f.SetHandler(func(w http.ResponseWriter, r *http.Request, body []byte) bool {
		if PathContains(r, "deliveries") {
			WriteSearchResponse(w, http.StatusOK, 1, map[string]interface{}{
				"_source": map[string]interface{}{"delivery_id": "d1"},
				"sort":    []interface{}{1.0, "d1"},
			})
			return true
		}
		WriteJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "nope"})
		return true
	})

	// Handled search path
	resp, err := http.Get(f.URL + "/deliveries/_search")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("search status = %d", resp.StatusCode)
	}
	last, _ := f.LastRequest()
	if last.Path != "/deliveries/_search" {
		t.Errorf("path = %q", last.Path)
	}

	// Other path hits the second branch.
	resp2, err := http.Post(f.URL+"/users/_search", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("users status = %d", resp2.StatusCode)
	}
}

func TestWriteSearchResponse_SortsInlineSource(t *testing.T) {
	rec := httptest.NewRecorder()
	// Hit without _source wrapper: the whole map becomes the source.
	WriteSearchResponse(rec, http.StatusOK, 2, map[string]interface{}{"delivery_id": "d2"})
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d", rec.Code)
	}
}
