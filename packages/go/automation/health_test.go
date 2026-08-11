package automation

import (
	"context"
	"testing"
)

func TestGetSystemStats(t *testing.T) {
	stats := GetSystemStats()

	if stats.Uptime < 0 {
		t.Errorf("expected uptime to be non-negative, got %f", stats.Uptime)
	}
	if stats.NumCPU <= 0 {
		t.Errorf("expected CPU count to be greater than 0, got %d", stats.NumCPU)
	}
}

func TestCheckDatabaseNil(t *testing.T) {
	status := CheckDatabase(context.Background(), nil)
	if status.Status != "DOWN" {
		t.Errorf("expected DOWN for nil database connection, got %s", status.Status)
	}
}

func TestCheckRedisNil(t *testing.T) {
	status := CheckRedis(context.Background(), nil)
	if status.Status != "DOWN" {
		t.Errorf("expected DOWN for nil redis client, got %s", status.Status)
	}
}
