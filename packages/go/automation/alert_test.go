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

func TestTriggerAlertWithoutWebhookURL(t *testing.T) {
	t.Setenv("ALERT_WEBHOOK_URL", "")

	ok, err := TriggerAlert("Disk full", "90% used", "CRITICAL")

	assert.False(t, ok)
	require.Error(t, err)
	assert.Equal(t, "ALERT_WEBHOOK_URL is not set", err.Error())
}

func TestTriggerAlertDeliversPayload(t *testing.T) {
	type captured struct {
		username string
		content  string
		ct       string
	}
	got := make(chan captured, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload WebhookPayload
		require.NoError(t, json.Unmarshal(body, &payload))
		got <- captured{username: payload.Username, content: payload.Content, ct: r.Header.Get("Content-Type")}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("ALERT_WEBHOOK_URL", srv.URL)

	ok, err := TriggerAlert("CPU spike", "usage 99%", "")
	require.NoError(t, err)
	assert.True(t, ok)

	c := <-got
	assert.Equal(t, "System Alert Bot", c.username)
	assert.Equal(t, "application/json", c.ct)
	assert.Contains(t, c.content, "[WARNING] CPU spike")
	assert.Contains(t, c.content, "usage 99%")
	assert.Contains(t, c.content, "Timestamp:")
}

func TestTriggerAlertPreservesSeverity(t *testing.T) {
	var content string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload WebhookPayload
		require.NoError(t, json.Unmarshal(body, &payload))
		content = payload.Content
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("ALERT_WEBHOOK_URL", srv.URL)

	ok, err := TriggerAlert("title", "message", "ERROR")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, content, "[ERROR] title")
}

func TestTriggerAlertRejectsNon2xxResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	t.Setenv("ALERT_WEBHOOK_URL", srv.URL)

	ok, err := TriggerAlert("t", "m", "WARNING")
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook responded with status: 502")
}

func TestTriggerAlertUnreachableWebhook(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	t.Setenv("ALERT_WEBHOOK_URL", "http://"+addr)

	ok, err := TriggerAlert("t", "m", "WARNING")
	assert.False(t, ok)
	require.Error(t, err)
}

func TestTriggerAlertInvalidWebhookURL(t *testing.T) {
	t.Setenv("ALERT_WEBHOOK_URL", "http://%zz")

	ok, err := TriggerAlert("t", "m", "WARNING")
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create webhook HTTP request")
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
