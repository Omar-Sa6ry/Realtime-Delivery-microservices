package automation

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTriggerAlertWithoutAlertmanagerURL(t *testing.T) {
	t.Setenv("ALERTMANAGER_URL", "")

	ok, err := TriggerAlert("media-service", "Disk full", "90% used", "CRITICAL")

	assert.False(t, ok)
	require.Error(t, err)
	assert.Equal(t, "ALERTMANAGER_URL is not set", err.Error())
}

func TestTriggerAlertDeliversPayload(t *testing.T) {
	type captured struct {
		path    string
		ct      string
		payload []AlertmanagerAlert
	}
	got := make(chan captured, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload []AlertmanagerAlert
		require.NoError(t, json.Unmarshal(body, &payload))
		got <- captured{path: r.URL.Path, ct: r.Header.Get("Content-Type"), payload: payload}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("ALERTMANAGER_URL", srv.URL+"/")

	ok, err := TriggerAlert("analytics-service", "CPU spike", "usage 99%", "")
	require.NoError(t, err)
	assert.True(t, ok)

	c := <-got
	assert.Equal(t, "/api/v2/alerts", c.path)
	assert.Equal(t, "application/json", c.ct)
	require.Len(t, c.payload, 1)

	labels := c.payload[0].Labels
	assert.Equal(t, "ApplicationAlert", labels["alertname"])
	assert.Equal(t, "analytics-service", labels["service"])
	assert.Equal(t, "warning", labels["severity"])
	assert.Equal(t, "CPU spike", labels["title"])
	assert.Equal(t, "CPU spike", c.payload[0].Annotations["summary"])
	assert.Equal(t, "usage 99%", c.payload[0].Annotations["description"])
	assert.NotEmpty(t, c.payload[0].StartsAt)
}

func TestTriggerAlertDefaultsServiceLabel(t *testing.T) {
	var payload []AlertmanagerAlert
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &payload))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("ALERTMANAGER_URL", srv.URL)

	ok, err := TriggerAlert("", "no service given", "m", "WARNING")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "application", payload[0].Labels["service"])
}

func TestTriggerAlertMapsSeverityAndService(t *testing.T) {
	var payload []AlertmanagerAlert
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &payload))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("ALERTMANAGER_URL", srv.URL)

	for _, tc := range []struct {
		in       string
		expected string
	}{
		{"CRITICAL", "critical"},
		{"ERROR", "critical"},
		{"WARNING", "warning"},
		{"WARN", "warning"},
		{"INFO", "info"},
		{"", "warning"},
		{"something-else", "warning"},
	} {
		ok, err := TriggerAlert("payment-service", "title-"+tc.in, "message", tc.in)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, tc.expected, payload[0].Labels["severity"])
		assert.Equal(t, "payment-service", payload[0].Labels["service"])
	}
}

func TestTriggerAlertRejectsNon2xxResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	t.Setenv("ALERTMANAGER_URL", srv.URL)

	ok, err := TriggerAlert("driver-service", "t", "m", "WARNING")
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alertmanager responded with status: 502")
}

func TestTriggerAlertUnreachableAlertmanager(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	t.Setenv("ALERTMANAGER_URL", "http://"+addr)

	ok, err := TriggerAlert("search-service", "t", "m", "WARNING")
	assert.False(t, ok)
	require.Error(t, err)
}

func TestTriggerAlertInvalidAlertmanagerURL(t *testing.T) {
	t.Setenv("ALERTMANAGER_URL", "http://%zz")

	ok, err := TriggerAlert("media-service", "t", "m", "WARNING")
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create alert HTTP request")
}

// --- database health ---

type healthStubConn struct{ failPing bool }

func (c *healthStubConn) Prepare(query string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}
func (c *healthStubConn) Close() error              { return nil }
func (c *healthStubConn) Begin() (driver.Tx, error) { return nil, driver.ErrSkip }
func (c *healthStubConn) Ping(ctx context.Context) error {
	if c.failPing {
		return errors.New("connection refused")
	}
	return nil
}

type healthStubDriver struct{}

func (healthStubDriver) Open(name string) (driver.Conn, error) {
	return &healthStubConn{failPing: name == "fail"}, nil
}

func init() {
	sql.Register("health-stub", healthStubDriver{})
}

func TestCheckDatabaseUp(t *testing.T) {
	db, err := sql.Open("health-stub", "ok")
	require.NoError(t, err)
	defer db.Close()

	status := CheckDatabase(context.Background(), db)
	assert.Equal(t, "UP", status.Status)
	assert.Empty(t, status.Message)
}

func TestCheckDatabasePingFailure(t *testing.T) {
	db, err := sql.Open("health-stub", "fail")
	require.NoError(t, err)
	defer db.Close()

	status := CheckDatabase(context.Background(), db)
	assert.Equal(t, "DOWN", status.Status)
	assert.Equal(t, "connection refused", status.Message)
}

// --- system stats ---

func TestGetSystemStatsPopulatesAllFields(t *testing.T) {
	stats := GetSystemStats()

	assert.GreaterOrEqual(t, stats.Uptime, 0.0)
	assert.Greater(t, stats.NumCPU, 0)
	assert.Greater(t, stats.NumGoroutine, 0)
	assert.Greater(t, stats.Memory.SysMB, uint64(0))
}

func TestFormatStatsString(t *testing.T) {
	stats := SystemStats{
		Uptime:       12.5,
		NumCPU:       8,
		NumGoroutine: 3,
		Memory: MemoryStats{
			HeapAllocMB:  1,
			TotalAllocMB: 2,
			SysMB:        3,
			NumGC:        4,
		},
	}

	out := FormatStatsString(stats)
	assert.Contains(t, out, "Uptime: 12.50s")
	assert.Contains(t, out, "CPUs: 8")
	assert.Contains(t, out, "Goroutines: 3")
	assert.Contains(t, out, "HeapAlloc: 1MB")
	assert.Contains(t, out, "TotalAlloc: 2MB")
	assert.Contains(t, out, "Sys: 3MB")
	assert.Contains(t, out, "GCs: 4")
}

func TestHealthStatusShape(t *testing.T) {
	raw, err := json.Marshal(HealthStatus{Status: "DOWN", Message: "x"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"DOWN","message":"x"}`, string(raw))

	raw, err = json.Marshal(HealthStatus{Status: "UP"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"UP"}`, string(raw))
}

func TestSystemStatsJSONTags(t *testing.T) {
	raw, err := json.Marshal(SystemStats{NumCPU: 2})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"numCpu":2`)
	assert.Contains(t, string(raw), `"numGoroutine"`)
	assert.Contains(t, string(raw), `"heapAllocMB"`)
}
