package health

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/redis"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
	miniredis "github.com/alicebob/miniredis/v2"
)

func newTestHandler(t *testing.T, repo *testutil.FakeRepository) (*Handler, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	cache := redis.NewCache(&config.Config{
		RedisHost: mr.Host(),
		RedisPort: mr.Port(),
		RedisTTL:  time.Minute,
	})
	return NewHandler(repo, cache), mr
}

func TestLivenessHandler(t *testing.T) {
	repo := testutil.NewFakeRepository()
	h, _ := newTestHandler(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	h.LivenessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "UP" {
		t.Errorf("status = %q", body["status"])
	}
	if body["timestamp"] == "" {
		t.Error("missing timestamp")
	}
	if _, err := time.Parse(time.RFC3339, body["timestamp"]); err != nil {
		t.Errorf("timestamp not RFC3339: %v", err)
	}
}

func TestReadinessHandler_AllUp(t *testing.T) {
	repo := testutil.NewFakeRepository()
	h, _ := newTestHandler(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	h.ReadinessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Status       string            `json:"status"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Status != "UP" {
		t.Errorf("status = %q", body.Status)
	}
	if body.Dependencies["opensearch"] != "UP" || body.Dependencies["redis"] != "UP" {
		t.Errorf("deps = %v", body.Dependencies)
	}
}

func TestReadinessHandler_OpenSearchDown(t *testing.T) {
	repo := testutil.NewFakeRepository()
	repo.PingErr = errors.New("os down")
	h, _ := newTestHandler(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	h.ReadinessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Status       string            `json:"status"`
		Dependencies map[string]string `json:"dependencies"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Status != "DOWN" {
		t.Errorf("status = %q", body.Status)
	}
	if body.Dependencies["opensearch"] != "DOWN: os down" {
		t.Errorf("os dep = %q", body.Dependencies["opensearch"])
	}
	if body.Dependencies["redis"] != "UP" {
		t.Errorf("redis dep = %q", body.Dependencies["redis"])
	}
}

func TestReadinessHandler_RedisDown(t *testing.T) {
	repo := testutil.NewFakeRepository()
	h, mr := newTestHandler(t, repo)
	mr.Close()

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	h.ReadinessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Status       string            `json:"status"`
		Dependencies map[string]string `json:"dependencies"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Status != "DOWN" {
		t.Errorf("status = %q", body.Status)
	}
	if body.Dependencies["opensearch"] != "UP" {
		t.Errorf("os dep = %q", body.Dependencies["opensearch"])
	}
}

func TestFormatStatus(t *testing.T) {
	if formatStatus(nil) != "UP" {
		t.Error("nil should be UP")
	}
	if formatStatus(errors.New("x")) != "DOWN: x" {
		t.Error("err formatting")
	}
}
