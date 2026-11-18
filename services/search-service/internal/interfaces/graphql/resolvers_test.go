package graphql

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/auth"
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
	gql "github.com/graphql-go/graphql"
)

// ---------- helpers ----------

func authCtx(userID, role string) context.Context {
	ctx := context.Background()
	ctx = sharedlogging.WithLogContext(ctx, sharedlogging.LogContext{UserID: userID})
	if role != "" {
		ctx = context.WithValue(ctx, "x-user-role", role)
	}
	return ctx
}

type testStack struct {
	repo   *testutil.FakeRepository
	search *appSearch.Service
	reidx  *reindex.Service
	r      *Resolver
	mr     *miniredis.Miniredis
}

func newStack(t *testing.T) *testStack {
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
	return &testStack{
		repo:   repo,
		search: searchSvc,
		reidx:  reidxSvc,
		r:      NewResolver(searchSvc, reidxSvc),
		mr:     mr,
	}
}

func params(ctx context.Context, args map[string]interface{}) gql.ResolveParams {
	return gql.ResolveParams{Context: ctx, Args: args}
}

func inputArgs(fields map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"input": fields}
}

// makeBearer builds a valid HS256 JWT for the auth-header fallback path.
func makeBearer(t *testing.T, sub, id string, exp int64) string {
	t.Helper()
	secret := "test-secret"
	t.Setenv("JWT_SECRET", secret)

	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := sharedauth.Claims{Subject: sub, ID: id, Role: "user", ExpiresAt: exp}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	body := head + "." + base64.RawURLEncoding.EncodeToString(payload)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return "Bearer " + body + "." + sig
}

// ---------- getAuthContext ----------

func TestGetAuthContext_MissingUserID_Unauthorized(t *testing.T) {
	r := &Resolver{}
	_, _, err := r.getAuthContext(context.Background())
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("err = %v", err)
	}
}

func TestGetAuthContext_ArabicLang_UnauthorizedMessage(t *testing.T) {
	r := &Resolver{}
	ctx := i18n.WithLanguage(context.Background(), "ar")
	_, _, err := r.getAuthContext(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "تسجيل الدخول") {
		t.Errorf("err = %v", err)
	}
}

func TestGetAuthContext_AuthHeaderFallback(t *testing.T) {
	r := &Resolver{}
	token := makeBearer(t, "user-1", "", time.Now().Add(time.Hour).Unix())
	ctx := context.WithValue(context.Background(), "authorization", token)

	userID, role, err := r.getAuthContext(ctx)
	if err != nil {
		t.Fatalf("getAuthContext: %v", err)
	}
	if userID != "user-1" {
		t.Errorf("userID = %q", userID)
	}
	if role != "" {
		t.Errorf("role = %q", role)
	}
}

func TestGetAuthContext_AuthHeaderFallsBackToIDClaim(t *testing.T) {
	r := &Resolver{}
	token := makeBearer(t, "", "id-only", time.Now().Add(time.Hour).Unix())
	ctx := context.WithValue(context.Background(), "authorization", token)

	userID, _, err := r.getAuthContext(ctx)
	if err != nil {
		t.Fatalf("getAuthContext: %v", err)
	}
	if userID != "id-only" {
		t.Errorf("userID = %q", userID)
	}
}

func TestGetAuthContext_InvalidBearer_StillUnauthorized(t *testing.T) {
	r := &Resolver{}
	ctx := context.WithValue(context.Background(), "authorization", "Bearer not.a.jwt")
	_, _, err := r.getAuthContext(ctx)
	if err == nil {
		t.Fatal("expected unauthorized")
	}

	// malformed scheme
	ctx = context.WithValue(context.Background(), "authorization", "Basic abc")
	_, _, err = r.getAuthContext(ctx)
	if err == nil {
		t.Fatal("expected unauthorized for Basic scheme")
	}
}

func TestGetAuthContext_ExpiredToken_Unauthorized(t *testing.T) {
	r := &Resolver{}
	token := makeBearer(t, "u1", "", time.Now().Add(-time.Hour).Unix())
	ctx := context.WithValue(context.Background(), "authorization", token)
	_, _, err := r.getAuthContext(ctx)
	if err == nil {
		t.Fatal("expected unauthorized for expired token")
	}
}

func TestGetAuthHeader_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), "authorization", 42)
	if got := getAuthHeader(ctx); got != "" {
		t.Errorf("got %q", got)
	}
	if got := getAuthHeader(context.Background()); got != "" {
		t.Errorf("empty ctx got %q", got)
	}
}

func TestGetUserRole_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), "x-user-role", 7)
	if got := getUserRole(ctx); got != "" {
		t.Errorf("got %q", got)
	}
	if got := getUserRole(context.Background()); got != "" {
		t.Errorf("empty ctx got %q", got)
	}
}

// ---------- searchDeliveries ----------

func TestResolveSearchDeliveries_Success(t *testing.T) {
	s := newStack(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.repo.DeliveriesResult = search.SearchResult[search.DeliveryDocument]{
		Items: []search.DeliveryDocument{{
			DeliveryID: "d1", CustomerID: "c1", Status: "PENDING",
			Pickup:    search.GeoAddress{City: "Cairo", Country: "EG", Location: search.GeoPoint{Lat: 1, Lon: 2}},
			Dropoff:   search.GeoAddress{City: "Giza", Country: "EG", Location: search.GeoPoint{Lat: 3, Lon: 4}},
			CreatedAt: now, UpdatedAt: now, SourceVersion: 1,
		}},
		PageInfo: search.PageInfo{Total: 1, HasNextPage: true},
	}

	ctx := authCtx("u1", string(sharedconstants.RoleUser))
	res, err := s.r.ResolveSearchDeliveries(params(ctx, inputArgs(map[string]interface{}{
		"query": "cairo",
		"sort":  []interface{}{map[string]interface{}{"field": "created_at", "order": "desc"}},
		"pagination": map[string]interface{}{
			"limit": 10, "cursor": "c",
		},
	})))
	if err != nil {
		t.Fatalf("ResolveSearchDeliveries: %v", err)
	}
	m := res.(map[string]interface{})
	if m["success"] != true || m["statusCode"] != 200 {
		t.Errorf("meta = %+v", m)
	}
	if m["message"] != "Deliveries fetched successfully" {
		t.Errorf("message = %v", m["message"])
	}
	data := m["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	doc := items[0].(map[string]interface{})
	if doc["delivery_id"] != "d1" || doc["status"] != "PENDING" {
		t.Errorf("doc = %+v", doc)
	}
	pickup := doc["pickup"].(map[string]interface{})
	if pickup["city"] != "Cairo" {
		t.Errorf("pickup = %+v", pickup)
	}
	pi := data["paginationInfo"].(map[string]interface{})
	if pi["totalItems"] != 1 || pi["currentPage"] != 1 {
		t.Errorf("paginationInfo = %+v", pi)
	}
	// Direct resolver output keeps the raw *int nextPage.
	np, ok := pi["nextPage"].(*int)
	if !ok || np == nil || *np != 2 {
		t.Errorf("nextPage = %v", pi["nextPage"])
	}

	// Query fields must have been passed down.
	got := s.repo.DeliveriesQueries[0]
	if got.Query != "cairo" || got.UserID != "u1" || got.UserRole != string(sharedconstants.RoleUser) {
		t.Errorf("query = %+v", got)
	}
	if len(got.Sort) != 1 || got.Sort[0].Field != "created_at" || got.Sort[0].Order != "desc" {
		t.Errorf("sort = %+v", got.Sort)
	}
	if got.Pagination.Limit != 10 || got.Pagination.Cursor != "c" {
		t.Errorf("pagination = %+v", got.Pagination)
	}
}

func TestResolveSearchDeliveries_NoNextPage(t *testing.T) {
	s := newStack(t)
	s.repo.DeliveriesResult = search.SearchResult[search.DeliveryDocument]{
		Items:    []search.DeliveryDocument{{DeliveryID: "d1"}},
		PageInfo: search.PageInfo{Total: 1},
	}
	res, err := s.r.ResolveSearchDeliveries(params(authCtx("u1", ""), inputArgs(map[string]interface{}{})))
	if err != nil {
		t.Fatal(err)
	}
	pi := res.(map[string]interface{})["data"].(map[string]interface{})["paginationInfo"].(map[string]interface{})
	if v, ok := pi["nextPage"].(*int); ok && v != nil {
		t.Errorf("nextPage = %v", *v)
	}
}

func TestResolveSearchDeliveries_Unauthorized(t *testing.T) {
	s := newStack(t)
	_, err := s.r.ResolveSearchDeliveries(params(context.Background(), inputArgs(map[string]interface{}{})))
	if err == nil || !strings.Contains(err.Error(), "authentication required") {
		t.Fatalf("err = %v", err)
	}
	if len(s.repo.DeliveriesQueries) != 0 {
		t.Error("repo must not be called without auth")
	}
}

func TestResolveSearchDeliveries_InvalidInput(t *testing.T) {
	s := newStack(t)
	_, err := s.r.ResolveSearchDeliveries(params(authCtx("u1", ""), map[string]interface{}{
		"input": "not-a-map",
	}))
	if err == nil || err.Error() != "invalid input" {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveSearchDeliveries_RepoError(t *testing.T) {
	s := newStack(t)
	s.repo.DeliveriesErr = errors.New("os down")
	_, err := s.r.ResolveSearchDeliveries(params(authCtx("u1", ""), inputArgs(map[string]interface{}{})))
	if err == nil || err.Error() != "os down" {
		t.Fatalf("err = %v", err)
	}
}

// ---------- searchDrivers ----------

func TestResolveSearchDrivers_Success(t *testing.T) {
	s := newStack(t)
	now := time.Now().UTC()
	s.repo.DriversResult = search.SearchResult[search.DriverDocument]{
		Items: []search.DriverDocument{{
			DriverID: "d1", Name: "Ali", Status: "AVAILABLE", VehicleType: "CAR",
			Rating: 4.5, Location: &search.GeoPoint{Lat: 1, Lon: 2},
			UpdatedAt: now, SourceVersion: 1,
		}},
		PageInfo: search.PageInfo{Total: 1},
	}
	res, err := s.r.ResolveSearchDrivers(params(authCtx("u1", ""), inputArgs(map[string]interface{}{
		"status": "AVAILABLE", "vehicleType": "CAR", "minRating": 4,
		"geo":  map[string]interface{}{"lat": 1.0, "lon": 2.0, "radiusKm": 5.0},
		"sort": []interface{}{map[string]interface{}{"field": "rating", "order": "desc"}},
	})))
	if err != nil {
		t.Fatal(err)
	}
	data := res.(map[string]interface{})["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	doc := items[0].(map[string]interface{})
	if doc["name"] != "Ali" || doc["rating"] != 4.5 {
		t.Errorf("doc = %+v", doc)
	}
	loc := doc["location"].(map[string]interface{})
	if loc["lat"] != 1.0 || loc["lon"] != 2.0 {
		t.Errorf("location = %+v", loc)
	}
	got := s.repo.DriversQueries[0]
	if got.Status != "AVAILABLE" || got.VehicleType != "CAR" || got.MinRating != 4 {
		t.Errorf("query = %+v", got)
	}
	if got.Geo == nil || got.Geo.RadiusKm != 5 {
		t.Errorf("geo = %+v", got.Geo)
	}
}

func TestResolveSearchDrivers_UnauthorizedInvalidInputError(t *testing.T) {
	s := newStack(t)
	if _, err := s.r.ResolveSearchDrivers(params(context.Background(), inputArgs(nil))); err == nil {
		t.Error("expected auth error")
	}
	if _, err := s.r.ResolveSearchDrivers(params(authCtx("u1", ""), map[string]interface{}{"input": 42})); err == nil {
		t.Error("expected invalid input error")
	}
	s.repo.DriversErr = errors.New("err")
	if _, err := s.r.ResolveSearchDrivers(params(authCtx("u1", ""), inputArgs(nil))); err == nil {
		t.Error("expected repo error")
	}
}

// ---------- searchMedia ----------

func TestResolveSearchMedia_Success(t *testing.T) {
	s := newStack(t)
	s.repo.MediaResult = search.SearchResult[search.MediaDocument]{
		Items: []search.MediaDocument{{
			MediaID: "m1", OwnerID: "u1", FileName: "a.png",
			MimeType: "image/png", MediaType: "IMAGE", Status: "READY", Size: 10,
			CreatedAt: time.Now().UTC(),
		}},
		PageInfo: search.PageInfo{Total: 1},
	}
	res, err := s.r.ResolveSearchMedia(params(authCtx("u1", ""), inputArgs(map[string]interface{}{
		"query": "png", "mediaType": "IMAGE", "mimeType": "image/png", "ownerId": "u1",
	})))
	if err != nil {
		t.Fatal(err)
	}
	items := res.(map[string]interface{})["data"].(map[string]interface{})["items"].([]interface{})
	doc := items[0].(map[string]interface{})
	if doc["media_id"] != "m1" || doc["size"] != int64(10) {
		t.Errorf("doc = %+v", doc)
	}
	got := s.repo.MediaQueries[0]
	if got.MediaType != "IMAGE" || got.OwnerID != "u1" {
		t.Errorf("query = %+v", got)
	}
}

func TestResolveSearchMedia_ErrorPaths(t *testing.T) {
	s := newStack(t)
	if _, err := s.r.ResolveSearchMedia(params(context.Background(), inputArgs(nil))); err == nil {
		t.Error("expected auth error")
	}
	if _, err := s.r.ResolveSearchMedia(params(authCtx("u1", ""), map[string]interface{}{"input": true})); err == nil {
		t.Error("expected invalid input error")
	}
	s.repo.MediaErr = errors.New("err")
	if _, err := s.r.ResolveSearchMedia(params(authCtx("u1", ""), inputArgs(nil))); err == nil {
		t.Error("expected repo error")
	}
}

// ---------- searchUsers ----------

func TestResolveSearchUsers_SuccessForAdmin(t *testing.T) {
	s := newStack(t)
	s.repo.UsersResult = search.SearchResult[search.UserDocument]{
		Items: []search.UserDocument{{
			ID: "u1", FirstName: "A", LastName: "B", Email: "a@b.c",
			Role: "user", IsActive: true, CreatedAt: time.Now().UTC(),
		}},
		PageInfo: search.PageInfo{Total: 1},
	}
	active := true
	res, err := s.r.ResolveSearchUsers(params(authCtx("admin", string(sharedconstants.RoleAdmin)), inputArgs(map[string]interface{}{
		"query": "a", "role": "user", "isActive": active,
	})))
	if err != nil {
		t.Fatal(err)
	}
	items := res.(map[string]interface{})["data"].(map[string]interface{})["items"].([]interface{})
	if items[0].(map[string]interface{})["email"] != "a@b.c" {
		t.Errorf("item = %+v", items[0])
	}
	got := s.repo.UsersQueries[0]
	if got.IsActive == nil || !*got.IsActive {
		t.Errorf("isActive = %v", got.IsActive)
	}
}

func TestResolveSearchUsers_ForbiddenForNonAdmin(t *testing.T) {
	s := newStack(t)
	_, err := s.r.ResolveSearchUsers(params(authCtx("u1", string(sharedconstants.RoleUser)), inputArgs(nil)))
	if err == nil || !strings.Contains(err.Error(), "admin access required") {
		t.Fatalf("err = %v", err)
	}
	if len(s.repo.UsersQueries) != 0 {
		t.Error("repo must not be called")
	}
}

func TestResolveSearchUsers_ErrorPaths(t *testing.T) {
	s := newStack(t)
	if _, err := s.r.ResolveSearchUsers(params(context.Background(), inputArgs(nil))); err == nil {
		t.Error("expected auth error")
	}
	admin := authCtx("a", string(sharedconstants.RoleAdmin))
	if _, err := s.r.ResolveSearchUsers(params(admin, map[string]interface{}{"input": nil})); err == nil {
		t.Error("expected invalid input error")
	}
	s.repo.UsersErr = errors.New("err")
	if _, err := s.r.ResolveSearchUsers(params(admin, inputArgs(nil))); err == nil {
		t.Error("expected repo error")
	}
}

// ---------- autocomplete ----------

func TestResolveAutocomplete_Success(t *testing.T) {
	s := newStack(t)
	s.repo.AutocompleteResult = search.AutocompleteResult{Suggestions: []string{"Cairo", "Giza"}}
	res, err := s.r.ResolveAutocomplete(params(authCtx("u1", ""), inputArgs(map[string]interface{}{
		"prefix": "Ca", "index": "deliveries", "limit": 5,
	})))
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]interface{})
	if m["success"] != true || m["statusCode"] != 200 {
		t.Errorf("meta = %+v", m)
	}
	if m["message"] != "Autocomplete suggestions fetched successfully" {
		t.Errorf("message = %v", m["message"])
	}
	data := m["data"].(map[string]interface{})
	sug := data["suggestions"].([]string)
	if len(sug) != 2 || sug[0] != "Cairo" {
		t.Errorf("suggestions = %v", sug)
	}
	q := s.repo.AutocompleteQs[0]
	if q.Prefix != "Ca" || q.Index != "deliveries" || q.Limit != 5 {
		t.Errorf("query = %+v", q)
	}
}

func TestResolveAutocomplete_ErrorPaths(t *testing.T) {
	s := newStack(t)
	if _, err := s.r.ResolveAutocomplete(params(context.Background(), inputArgs(nil))); err == nil {
		t.Error("expected auth error")
	}
	if _, err := s.r.ResolveAutocomplete(params(authCtx("u1", ""), map[string]interface{}{"input": 1})); err == nil {
		t.Error("expected invalid input error")
	}
	s.repo.AutocompleteErr = errors.New("err")
	if _, err := s.r.ResolveAutocomplete(params(authCtx("u1", ""), inputArgs(nil))); err == nil {
		t.Error("expected repo error")
	}
}

// ---------- geo queries ----------

func TestResolveNearbyDeliveries_Success(t *testing.T) {
	s := newStack(t)
	s.repo.NearbyDelivResult = search.SearchResult[search.DeliveryDocument]{
		Items:    []search.DeliveryDocument{{DeliveryID: "d1"}},
		PageInfo: search.PageInfo{Total: 1},
	}
	res, err := s.r.ResolveNearbyDeliveries(params(authCtx("u1", ""), inputArgs(map[string]interface{}{
		"lat": 1.0, "lon": 2.0, "radiusKm": 3.0,
	})))
	if err != nil {
		t.Fatal(err)
	}
	if res.(map[string]interface{})["message"] != "Nearby deliveries fetched successfully" {
		t.Errorf("message = %v", res.(map[string]interface{})["message"])
	}
	q := s.repo.NearbyDelivQs[0]
	if q.Lat != 1 || q.Lon != 2 || q.RadiusKm != 3 || q.UserID != "u1" {
		t.Errorf("query = %+v", q)
	}
}

func TestResolveNearbyDeliveries_ErrorPaths(t *testing.T) {
	s := newStack(t)
	if _, err := s.r.ResolveNearbyDeliveries(params(context.Background(), inputArgs(nil))); err == nil {
		t.Error("expected auth error")
	}
	if _, err := s.r.ResolveNearbyDeliveries(params(authCtx("u1", ""), map[string]interface{}{"input": nil})); err == nil {
		t.Error("expected invalid input error")
	}
	s.repo.NearbyDelivErr = errors.New("err")
	if _, err := s.r.ResolveNearbyDeliveries(params(authCtx("u1", ""), inputArgs(nil))); err == nil {
		t.Error("expected repo error")
	}
}

func TestResolveNearbyDrivers_Success(t *testing.T) {
	s := newStack(t)
	s.repo.NearbyDriversResult = search.SearchResult[search.DriverDocument]{
		Items:    []search.DriverDocument{{DriverID: "d1"}},
		PageInfo: search.PageInfo{Total: 1},
	}
	res, err := s.r.ResolveNearbyDrivers(params(authCtx("u1", ""), inputArgs(map[string]interface{}{
		"lat": 1.0, "lon": 2.0, "radiusKm": 3.0,
	})))
	if err != nil {
		t.Fatal(err)
	}
	if res.(map[string]interface{})["message"] != "Nearby drivers fetched successfully" {
		t.Errorf("message = %v", res.(map[string]interface{})["message"])
	}
	if len(s.repo.NearbyDriversQs) != 1 {
		t.Error("repo not called")
	}
}

func TestResolveNearbyDrivers_ErrorPaths(t *testing.T) {
	s := newStack(t)
	if _, err := s.r.ResolveNearbyDrivers(params(context.Background(), inputArgs(nil))); err == nil {
		t.Error("expected auth error")
	}
	if _, err := s.r.ResolveNearbyDrivers(params(authCtx("u1", ""), map[string]interface{}{"input": 9})); err == nil {
		t.Error("expected invalid input error")
	}
	s.repo.NearbyDriversErr = errors.New("err")
	if _, err := s.r.ResolveNearbyDrivers(params(authCtx("u1", ""), inputArgs(nil))); err == nil {
		t.Error("expected repo error")
	}
}

// ---------- searchHealth ----------

func TestResolveSearchHealth_UsesContextTimestamp(t *testing.T) {
	s := newStack(t)
	ctx := context.WithValue(context.Background(), "timestamp", "2024-01-02T03:04:05Z")
	res, err := s.r.ResolveSearchHealth(params(ctx, nil))
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]interface{})
	if m["message"] != "Search service is healthy" {
		t.Errorf("message = %v", m["message"])
	}
	data := m["data"].(map[string]interface{})
	if data["status"] != "UP" || data["timestamp"] != "2024-01-02T03:04:05Z" {
		t.Errorf("data = %+v", data)
	}
}

func TestResolveSearchHealth_FallsBackToNow(t *testing.T) {
	s := newStack(t)
	res, err := s.r.ResolveSearchHealth(params(context.Background(), nil))
	if err != nil {
		t.Fatal(err)
	}
	data := res.(map[string]interface{})["data"].(map[string]interface{})
	if _, err := time.Parse(time.RFC3339, data["timestamp"].(string)); err != nil {
		t.Errorf("timestamp not RFC3339: %v", err)
	}
}

// ---------- startReindex ----------

func TestResolveStartReindex_ForbiddenForNonAdmin(t *testing.T) {
	s := newStack(t)
	_, err := s.r.ResolveStartReindex(params(authCtx("u1", string(sharedconstants.RoleUser)), map[string]interface{}{
		"index": "deliveries",
	}))
	if err == nil || !strings.Contains(err.Error(), "admin access required") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveStartReindex_Unauthorized(t *testing.T) {
	s := newStack(t)
	_, err := s.r.ResolveStartReindex(params(context.Background(), map[string]interface{}{
		"index": "deliveries",
	}))
	if err == nil {
		t.Fatal("expected auth error")
	}
}

func TestResolveStartReindex_EmptyIndexError(t *testing.T) {
	s := newStack(t)
	ctx := authCtx("admin", string(sharedconstants.RoleAdmin))
	_, err := s.r.ResolveStartReindex(params(ctx, map[string]interface{}{"index": ""}))
	if err == nil || err.Error() != "index name is required" {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveStartReindex_AdminSuccess(t *testing.T) {
	s := newStack(t)
	s.repo.ReindexDone = make(chan struct{})
	ctx := authCtx("admin", string(sharedconstants.RoleAdmin))
	res, err := s.r.ResolveStartReindex(params(ctx, map[string]interface{}{"index": "deliveries"}))
	if err != nil {
		t.Fatalf("ResolveStartReindex: %v", err)
	}
	m := res.(map[string]interface{})
	if m["message"] != "Reindex job started successfully" {
		t.Errorf("message = %v", m["message"])
	}
	data := m["data"].(map[string]interface{})
	if data["index"] != "deliveries" || data["status"] != "IN_PROGRESS" {
		t.Errorf("data = %+v", data)
	}
	if !strings.HasPrefix(data["jobId"].(string), "reindex-deliveries-") {
		t.Errorf("jobId = %v", data["jobId"])
	}
	if data["completedAt"] != "" {
		t.Errorf("completedAt = %v", data["completedAt"])
	}
	select {
	case <-s.repo.ReindexDone:
	case <-time.After(2 * time.Second):
		t.Fatal("background reindex not invoked")
	}
}

// ---------- build*Query helpers ----------

func TestBuildDeliverySearchQuery_FullInput(t *testing.T) {
	input := map[string]interface{}{
		"query": "q", "status": "PENDING", "city": "Cairo",
		"driverId": "d1", "customerId": "c1",
		"fromDate": "2024-01-01T00:00:00Z",
		"toDate":   "2024-02-01T00:00:00Z",
		"geo":      map[string]interface{}{"lat": 1.5, "lon": 2.5, "radiusKm": 3.5},
		"sort":     []interface{}{map[string]interface{}{"field": "status", "order": "asc"}, "junk"},
		"pagination": map[string]interface{}{
			"limit": 10.0, "cursor": "cur",
		},
	}
	q := buildDeliverySearchQuery(input, "u1", "user")
	if q.Query != "q" || q.Status != "PENDING" || q.City != "Cairo" ||
		q.DriverID != "d1" || q.CustomerID != "c1" || q.UserID != "u1" || q.UserRole != "user" {
		t.Errorf("query = %+v", q)
	}
	if q.FromDate == nil || q.FromDate.Year() != 2024 {
		t.Errorf("fromDate = %v", q.FromDate)
	}
	if q.ToDate == nil || q.ToDate.Month() != 2 {
		t.Errorf("toDate = %v", q.ToDate)
	}
	if q.Geo == nil || q.Geo.RadiusKm != 3.5 {
		t.Errorf("geo = %+v", q.Geo)
	}
	if len(q.Sort) != 1 || q.Sort[0].Field != "status" || q.Sort[0].Order != "asc" {
		t.Errorf("sort = %+v", q.Sort)
	}
	if q.Pagination.Limit != 10 || q.Pagination.Cursor != "cur" {
		t.Errorf("pagination = %+v", q.Pagination)
	}
}

func TestBuildDeliverySearchQuery_InvalidDatesIgnored(t *testing.T) {
	q := buildDeliverySearchQuery(map[string]interface{}{
		"fromDate": "not-a-date",
		"toDate":   "",
	}, "", "")
	if q.FromDate != nil || q.ToDate != nil {
		t.Errorf("dates should be nil: %+v", q)
	}
}

func TestBuildDriverSearchQuery_FullInput(t *testing.T) {
	input := map[string]interface{}{
		"query": "q", "status": "AVAILABLE", "vehicleType": "CAR", "minRating": 4.5,
		"geo":  map[string]interface{}{"lat": 1.0, "lon": 2.0, "radiusKm": 3.0},
		"sort": []interface{}{map[string]interface{}{"field": "rating", "order": "desc"}},
	}
	q := buildDriverSearchQuery(input, "u1", "driver")
	if q.Query != "q" || q.Status != "AVAILABLE" || q.VehicleType != "CAR" ||
		q.MinRating != 4.5 || q.UserID != "u1" || q.UserRole != "driver" {
		t.Errorf("query = %+v", q)
	}
	if q.Geo == nil || len(q.Sort) != 1 || q.Sort[0].Field != "rating" {
		t.Errorf("geo/sort = %+v / %+v", q.Geo, q.Sort)
	}
}

func TestBuildMediaAndUserAndGeoQueries(t *testing.T) {
	m := buildMediaSearchQuery(map[string]interface{}{
		"query": "q", "mediaType": "IMAGE", "mimeType": "image/png", "ownerId": "o1",
	}, "u1", "user")
	if m.MediaType != "IMAGE" || m.OwnerID != "o1" || m.UserID != "u1" {
		t.Errorf("media = %+v", m)
	}

	u := buildUserSearchQuery(map[string]interface{}{"query": "q", "role": "user"})
	if u.Role != "user" || u.IsActive != nil {
		t.Errorf("user = %+v", u)
	}
	active := false
	u2 := buildUserSearchQuery(map[string]interface{}{"isActive": active})
	if u2.IsActive == nil || *u2.IsActive {
		t.Errorf("user2 = %+v", u2.IsActive)
	}

	g := buildGeoSearchQuery(map[string]interface{}{"lat": 1, "lon": 2, "radiusKm": 3}, "u", "r")
	if g.Lat != 1 || g.Lon != 2 || g.RadiusKm != 3 || g.UserID != "u" || g.UserRole != "r" {
		t.Errorf("geo = %+v", g)
	}
}

func TestBuildPaginationInput_NoPagination(t *testing.T) {
	p := buildPaginationInput(map[string]interface{}{})
	if p.Limit != 0 || p.Cursor != "" {
		t.Errorf("pagination = %+v", p)
	}
	p = buildPaginationInput(nil)
	if p.Limit != 0 {
		t.Errorf("pagination = %+v", p)
	}
}

// ---------- mapping helpers ----------

func TestMapSearchResult(t *testing.T) {
	res := mapSearchResult(search.SearchResult[search.DeliveryDocument]{
		Items:    []search.DeliveryDocument{{DeliveryID: "d1"}},
		PageInfo: search.PageInfo{Total: 42},
	}, "msg")
	if res["success"] != true || res["statusCode"] != 200 || res["message"] != "msg" {
		t.Errorf("res = %+v", res)
	}
	if _, ok := res["timeStamp"].(string); !ok {
		t.Errorf("timeStamp type = %T", res["timeStamp"])
	}
	data := res["data"].(map[string]interface{})
	if len(data["items"].([]interface{})) != 1 {
		t.Error("items length")
	}
}

func TestMapToInterface_AllDocumentTypes(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	d := mapToInterface(search.DeliveryDocument{
		DeliveryID: "d1", CustomerID: "c1", DriverID: "dr1", Status: "PENDING",
		Pickup:    search.GeoAddress{City: "C", Country: "EG", Location: search.GeoPoint{Lat: 1, Lon: 2}},
		Dropoff:   search.GeoAddress{City: "D", Country: "EG", Location: search.GeoPoint{Lat: 3, Lon: 4}},
		CreatedAt: now, UpdatedAt: now, SourceVersion: 3,
	})
	if d["delivery_id"] != "d1" || d["driver_id"] != "dr1" || d["source_version"] != int64(3) {
		t.Errorf("delivery = %+v", d)
	}
	if d["created_at"] != now.Format(time.RFC3339) {
		t.Errorf("created_at = %v", d["created_at"])
	}
	if d["pickup"].(map[string]interface{})["city"] != "C" {
		t.Errorf("pickup = %+v", d["pickup"])
	}

	drv := mapToInterface(search.DriverDocument{
		DriverID: "d1", Name: "Ali", Status: "A", VehicleType: "CAR", Rating: 5,
		Location: &search.GeoPoint{Lat: 1, Lon: 2}, UpdatedAt: now, SourceVersion: 1,
	})
	if drv["name"] != "Ali" || drv["rating"] != 5.0 {
		t.Errorf("driver = %+v", drv)
	}
	if drv["location"].(map[string]interface{})["lat"] != 1.0 {
		t.Errorf("location = %+v", drv["location"])
	}
	if drv["updated_at"] != now.Format(time.RFC3339) {
		t.Errorf("updated_at = %v", drv["updated_at"])
	}

	drvNilLoc := mapToInterface(search.DriverDocument{DriverID: "d2"})
	if locVal, _ := drvNilLoc["location"].(map[string]interface{}); locVal != nil {
		t.Errorf("location = %v", drvNilLoc["location"])
	}

	med := mapToInterface(search.MediaDocument{
		MediaID: "m1", OwnerID: "o1", FileName: "f", MimeType: "image/png",
		MediaType: "IMAGE", Status: "READY", Size: 10, CreatedAt: now,
	})
	if med["media_id"] != "m1" || med["size"] != int64(10) || med["created_at"] != now.Format(time.RFC3339) {
		t.Errorf("media = %+v", med)
	}

	usr := mapToInterface(search.UserDocument{
		ID: "u1", FirstName: "A", LastName: "B", Email: "e", Role: "user",
		IsActive: true, CreatedAt: now,
	})
	if usr["id"] != "u1" || usr["is_active"] != true || usr["created_at"] != now.Format(time.RFC3339) {
		t.Errorf("user = %+v", usr)
	}

	// Unknown type → empty map.
	if got := mapToInterface("junk"); len(got) != 0 {
		t.Errorf("unknown type map = %+v", got)
	}
}

func TestFormatTimePtr(t *testing.T) {
	if formatTimePtr(nil) != "" {
		t.Error("nil should be empty")
	}
	ts := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if formatTimePtr(&ts) != ts.Format(time.RFC3339) {
		t.Errorf("got %q", formatTimePtr(&ts))
	}
}

// ---------- value getters ----------

func TestGetString(t *testing.T) {
	if getString(map[string]interface{}{"k": "v"}, "k") != "v" {
		t.Error("string")
	}
	if getString(map[string]interface{}{"k": 1}, "k") != "" {
		t.Error("non-string")
	}
	if getString(nil, "k") != "" {
		t.Error("nil map")
	}
}

func TestGetInt(t *testing.T) {
	cases := []struct {
		in   interface{}
		want int
	}{
		{1, 1},
		{int64(2), 2},
		{float64(3.7), 3},
		{"nope", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := getInt(map[string]interface{}{"k": c.in}, "k"); got != c.want {
			t.Errorf("getInt(%v) = %d, want %d", c.in, got, c.want)
		}
	}
	if getInt(map[string]interface{}{}, "missing") != 0 {
		t.Error("missing key")
	}
}

func TestGetFloat(t *testing.T) {
	cases := []struct {
		in   interface{}
		want float64
	}{
		{1.5, 1.5},
		{float32(2.5), 2.5},
		{3, 3.0},
		{int64(4), 4.0},
		{"x", 0},
	}
	for _, c := range cases {
		if got := getFloat(map[string]interface{}{"k": c.in}, "k"); got != c.want {
			t.Errorf("getFloat(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	if getFloat(nil, "k") != 0 {
		t.Error("nil map")
	}
}

func TestGetBool_DeadCodeBranches(t *testing.T) {
	if !getBool(map[string]interface{}{"k": true}, "k") {
		t.Error("true")
	}
	if getBool(map[string]interface{}{"k": false}, "k") {
		t.Error("false")
	}
	if getBool(map[string]interface{}{"k": "yes"}, "k") {
		t.Error("non-bool must default false")
	}
	if getBool(map[string]interface{}{}, "missing") {
		t.Error("missing")
	}
}

func TestGetMapAndGetSlice(t *testing.T) {
	inner := map[string]interface{}{"a": 1}
	if getMap(map[string]interface{}{"k": inner}, "k") == nil {
		t.Error("map")
	}
	if getMap(map[string]interface{}{"k": "str"}, "k") != nil {
		t.Error("non-map")
	}
	if getMap(nil, "k") != nil {
		t.Error("nil map")
	}
	sl := []interface{}{1}
	if getSlice(map[string]interface{}{"k": sl}, "k") == nil {
		t.Error("slice")
	}
	if getSlice(map[string]interface{}{"k": map[string]interface{}{}}, "k") != nil {
		t.Error("non-slice")
	}
	if getSlice(nil, "k") != nil {
		t.Error("nil map")
	}
}

// ---------- misc ----------

func TestNewResolver(t *testing.T) {
	s := newStack(t)
	r := NewResolver(s.search, s.reidx)
	if r == nil || r.searchService != s.search || r.reindexService != s.reidx {
		t.Fatalf("resolver = %+v", r)
	}
}
