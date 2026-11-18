package search

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/redis"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
	miniredis "github.com/alicebob/miniredis/v2"
)

func newTestService(t *testing.T) (*Service, *testutil.FakeRepository, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	cache := redis.NewCache(&config.Config{
		RedisHost: mr.Host(),
		RedisPort: mr.Port(),
		RedisTTL:  60 * time.Second,
	})
	repo := testutil.NewFakeRepository()
	return NewService(repo, cache), repo, mr
}

func TestSearchDeliveries_CacheMissThenHit(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	repo.DeliveriesResult = search.SearchResult[search.DeliveryDocument]{
		Items:    []search.DeliveryDocument{{DeliveryID: "d1"}},
		PageInfo: search.PageInfo{Total: 1},
	}

	res1, err := svc.SearchDeliveries(ctx, search.DeliverySearchQuery{Query: "cairo"})
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(res1.Items) != 1 {
		t.Fatalf("first call items = %d", len(res1.Items))
	}
	if len(repo.DeliveriesQueries) != 1 {
		t.Fatalf("repo call count = %d", len(repo.DeliveriesQueries))
	}

	// Second identical call should hit cache — no extra repo call.
	res2, err := svc.SearchDeliveries(ctx, search.DeliverySearchQuery{Query: "cairo"})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(repo.DeliveriesQueries) != 1 {
		t.Errorf("repo should not be called on cache hit, calls = %d", len(repo.DeliveriesQueries))
	}
	if len(res2.Items) != 1 || res2.Items[0].DeliveryID != "d1" {
		t.Errorf("cached result wrong: %+v", res2)
	}
}

func TestSearchDeliveries_EmptyResultNotCached(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()
	repo.DeliveriesResult = search.SearchResult[search.DeliveryDocument]{Items: []search.DeliveryDocument{}}

	_, _ = svc.SearchDeliveries(ctx, search.DeliverySearchQuery{Query: "nothing"})
	_, _ = svc.SearchDeliveries(ctx, search.DeliverySearchQuery{Query: "nothing"})
	if len(repo.DeliveriesQueries) != 2 {
		t.Errorf("empty results should not be cached, calls = %d", len(repo.DeliveriesQueries))
	}
}

func TestSearchDeliveries_RepoError(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.DeliveriesErr = errors.New("boom")
	_, err := svc.SearchDeliveries(context.Background(), search.DeliverySearchQuery{})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
}

func TestSearchDrivers_CacheBehavior(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()
	repo.DriversResult = search.SearchResult[search.DriverDocument]{
		Items:    []search.DriverDocument{{DriverID: "d1"}},
		PageInfo: search.PageInfo{Total: 1},
	}
	if _, err := svc.SearchDrivers(ctx, search.DriverSearchQuery{Query: "ali"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SearchDrivers(ctx, search.DriverSearchQuery{Query: "ali"}); err != nil {
		t.Fatal(err)
	}
	if len(repo.DriversQueries) != 1 {
		t.Errorf("drivers repo calls = %d", len(repo.DriversQueries))
	}

	repo.DriversErr = errors.New("err")
	if _, err := svc.SearchDrivers(ctx, search.DriverSearchQuery{Query: "other"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchDrivers_EmptyNotCached(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()
	repo.DriversResult = search.SearchResult[search.DriverDocument]{Items: []search.DriverDocument{}}
	_, _ = svc.SearchDrivers(ctx, search.DriverSearchQuery{Query: "x"})
	_, _ = svc.SearchDrivers(ctx, search.DriverSearchQuery{Query: "x"})
	if len(repo.DriversQueries) != 2 {
		t.Errorf("calls = %d", len(repo.DriversQueries))
	}
}

func TestSearchMedia_CacheBehavior(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()
	repo.MediaResult = search.SearchResult[search.MediaDocument]{
		Items:    []search.MediaDocument{{MediaID: "m1"}},
		PageInfo: search.PageInfo{Total: 1},
	}
	_, _ = svc.SearchMedia(ctx, search.MediaSearchQuery{Query: "png"})
	_, _ = svc.SearchMedia(ctx, search.MediaSearchQuery{Query: "png"})
	if len(repo.MediaQueries) != 1 {
		t.Errorf("media repo calls = %d", len(repo.MediaQueries))
	}

	repo.MediaErr = errors.New("err")
	if _, err := svc.SearchMedia(ctx, search.MediaSearchQuery{Query: "fail"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchMedia_EmptyNotCached(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()
	repo.MediaResult = search.SearchResult[search.MediaDocument]{Items: []search.MediaDocument{}}
	_, _ = svc.SearchMedia(ctx, search.MediaSearchQuery{Query: "x"})
	_, _ = svc.SearchMedia(ctx, search.MediaSearchQuery{Query: "x"})
	if len(repo.MediaQueries) != 2 {
		t.Errorf("calls = %d", len(repo.MediaQueries))
	}
}

func TestSearchUsers_CacheBehavior(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()
	repo.UsersResult = search.SearchResult[search.UserDocument]{
		Items:    []search.UserDocument{{ID: "u1"}},
		PageInfo: search.PageInfo{Total: 1},
	}
	_, _ = svc.SearchUsers(ctx, search.UserSearchQuery{Query: "a"})
	_, _ = svc.SearchUsers(ctx, search.UserSearchQuery{Query: "a"})
	if len(repo.UsersQueries) != 1 {
		t.Errorf("users repo calls = %d", len(repo.UsersQueries))
	}

	repo.UsersErr = errors.New("err")
	if _, err := svc.SearchUsers(ctx, search.UserSearchQuery{Query: "fail"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchUsers_EmptyNotCached(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()
	repo.UsersResult = search.SearchResult[search.UserDocument]{Items: []search.UserDocument{}}
	_, _ = svc.SearchUsers(ctx, search.UserSearchQuery{Query: "x"})
	_, _ = svc.SearchUsers(ctx, search.UserSearchQuery{Query: "x"})
	if len(repo.UsersQueries) != 2 {
		t.Errorf("calls = %d", len(repo.UsersQueries))
	}
}

func TestAutocomplete_AlwaysCached(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()
	repo.AutocompleteResult = search.AutocompleteResult{Suggestions: []string{"Cairo"}}

	res1, err := svc.Autocomplete(ctx, search.AutocompleteQuery{Prefix: "Ca", Index: "deliveries"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res1.Suggestions) != 1 {
		t.Fatalf("suggestions = %v", res1.Suggestions)
	}
	// even empty suggestions are cached
	repo.AutocompleteResult = search.AutocompleteResult{}
	res2, err := svc.Autocomplete(ctx, search.AutocompleteQuery{Prefix: "Ca", Index: "deliveries"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Suggestions) != 1 {
		t.Errorf("expected cached suggestions, got %v", res2.Suggestions)
	}
	if len(repo.AutocompleteQs) != 1 {
		t.Errorf("repo calls = %d", len(repo.AutocompleteQs))
	}
}

func TestAutocomplete_RepoError(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.AutocompleteErr = errors.New("err")
	if _, err := svc.Autocomplete(context.Background(), search.AutocompleteQuery{Prefix: "x"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestAutocomplete_EmptyResultStillCached(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()
	repo.AutocompleteResult = search.AutocompleteResult{}
	_, _ = svc.Autocomplete(ctx, search.AutocompleteQuery{Prefix: "zz", Index: "drivers"})
	_, _ = svc.Autocomplete(ctx, search.AutocompleteQuery{Prefix: "zz", Index: "drivers"})
	if len(repo.AutocompleteQs) != 1 {
		t.Errorf("empty autocomplete should be cached, calls = %d", len(repo.AutocompleteQs))
	}
}

func TestNearbyDeliveries_Passthrough(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.NearbyDelivResult = search.SearchResult[search.DeliveryDocument]{
		Items: []search.DeliveryDocument{{DeliveryID: "d1"}},
	}
	res, err := svc.NearbyDeliveries(context.Background(), search.GeoSearchQuery{Lat: 1, Lon: 2, RadiusKm: 3})
	if err != nil || len(res.Items) != 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(repo.NearbyDelivQs) != 1 {
		t.Errorf("nearby deliveries calls = %d", len(repo.NearbyDelivQs))
	}

	repo.NearbyDelivErr = errors.New("err")
	if _, err := svc.NearbyDeliveries(context.Background(), search.GeoSearchQuery{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNearbyDrivers_Passthrough(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.NearbyDriversResult = search.SearchResult[search.DriverDocument]{
		Items: []search.DriverDocument{{DriverID: "d1"}},
	}
	res, err := svc.NearbyDrivers(context.Background(), search.GeoSearchQuery{Lat: 1, Lon: 2, RadiusKm: 3})
	if err != nil || len(res.Items) != 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(repo.NearbyDriversQs) != 1 {
		t.Errorf("nearby drivers calls = %d", len(repo.NearbyDriversQs))
	}

	repo.NearbyDriversErr = errors.New("err")
	if _, err := svc.NearbyDrivers(context.Background(), search.GeoSearchQuery{}); err == nil {
		t.Fatal("expected error")
	}
}
