package automation

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/redis/go-redis/v9"
)

var startTime time.Time

func init() {
	startTime = time.Now()
}

// HealthStatus represents the return status of health checks
type HealthStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// SystemStats holds standard Go system resource statistics
type SystemStats struct {
	Uptime       float64     `json:"uptime"`
	NumCPU       int         `json:"numCpu"`
	NumGoroutine int         `json:"numGoroutine"`
	Memory       MemoryStats `json:"memory"`
}

// MemoryStats holds details about Go runtime memory allocation in MB
type MemoryStats struct {
	HeapAllocMB      uint64 `json:"heapAllocMB"`
	TotalAllocMB     uint64 `json:"totalAllocMB"`
	SysMB            uint64 `json:"sysMB"`
	NumGC            uint32 `json:"numGC"`
}

// CheckDatabase checks database connectivity by pinging the *sql.DB connection
func CheckDatabase(ctx context.Context, db *sql.DB) HealthStatus {
	if db == nil {
		return HealthStatus{Status: "DOWN", Message: "Database driver not initialized"}
	}

	err := db.PingContext(ctx)
	if err != nil {
		slog.Error("Health Check: Database connection failed", "error", err)
		return HealthStatus{Status: "DOWN", Message: err.Error()}
	}

	return HealthStatus{Status: "UP"}
}

// CheckRedis checks redis connectivity by pinging the *redis.Client connection
func CheckRedis(ctx context.Context, client *redis.Client) HealthStatus {
	if client == nil {
		return HealthStatus{Status: "DOWN", Message: "Redis client not initialized"}
	}

	_, err := client.Ping(ctx).Result()
	if err != nil {
		slog.Error("Health Check: Redis connection failed", "error", err)
		return HealthStatus{Status: "DOWN", Message: err.Error()}
	}

	return HealthStatus{Status: "UP"}
}

// GetSystemStats returns standard Go system resource metrics
func GetSystemStats() SystemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemStats{
		Uptime:       time.Since(startTime).Seconds(),
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		Memory: MemoryStats{
			HeapAllocMB:  m.HeapAlloc / 1024 / 1024,
			TotalAllocMB: m.TotalAlloc / 1024 / 1024,
			SysMB:        m.Sys / 1024 / 1024,
			NumGC:        m.NumGC,
		},
	}
}

// FormatStatsString returns system stats formatted as a log string
func FormatStatsString(stats SystemStats) string {
	return fmt.Sprintf("Uptime: %.2fs, CPUs: %d, Goroutines: %d, Memory: [HeapAlloc: %dMB, TotalAlloc: %dMB, Sys: %dMB, GCs: %d]",
		stats.Uptime,
		stats.NumCPU,
		stats.NumGoroutine,
		stats.Memory.HeapAllocMB,
		stats.Memory.TotalAllocMB,
		stats.Memory.SysMB,
		stats.Memory.NumGC,
	)
}
