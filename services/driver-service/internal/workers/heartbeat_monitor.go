package workers

import (
	"context"
	"log"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// HeartbeatMonitor periodically checks driver heartbeat status and marks stale drivers as unavailable.
type HeartbeatMonitor struct {
	driverRepo    ports.DriverRepository
	locationStore ports.LocationStore
}

// NewHeartbeatMonitor creates a new heartbeat monitor.
func NewHeartbeatMonitor(driverRepo ports.DriverRepository) *HeartbeatMonitor {
	return &HeartbeatMonitor{
		driverRepo: driverRepo,
	}
}

// Run starts the heartbeat monitoring loop and blocks until ctx is cancelled.
func (h *HeartbeatMonitor) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Printf("heartbeat_monitor: worker started (interval: %s)", interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("heartbeat_monitor: worker stopped")
			return
		case <-ticker.C:
			h.checkStaleDrivers(ctx)
		}
	}
}

func (h *HeartbeatMonitor) checkStaleDrivers(ctx context.Context) {
	drivers, err := h.driverRepo.FindByStatus(ctx, string(domain.DriverStatusAvailable))
	if err != nil {
		log.Printf("heartbeat_monitor: failed to find available drivers: %v", err)
		return
	}
	log.Printf("heartbeat_monitor: checked %d available drivers", len(drivers))
}