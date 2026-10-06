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

type HealthStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type SystemStats struct {
	Uptime       float64     `json:"uptime"`
	NumCPU       int         `json:"numCpu"`
	NumGoroutine int         `json:"numGoroutine"`
	Memory       MemoryStats `json:"memory"`
}

type MemoryStats struct {
	HeapAllocMB  uint64 `json:"heapAllocMB"`
	TotalAllocMB uint64 `json:"totalAllocMB"`
	SysMB        uint64 `json:"sysMB"`
	NumGC        uint32 `json:"numGC"`
}

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
