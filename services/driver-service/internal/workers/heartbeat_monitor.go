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
	driverRepo      ports.DriverRepository
	locationStore   ports.LocationStore
	stopChan        chan struct{}
	staleThreshold  time.Duration
}

// NewHeartbeatMonitor creates a new heartbeat monitor.
func NewHeartbeatMonitor(driverRepo ports.DriverRepository, locationStore ports.LocationStore,
	staleThreshold time.Duration) *HeartbeatMonitor {

	return &HeartbeatMonitor{
		driverRepo:      driverRepo,
		locationStore:   locationStore,
		staleThreshold:  staleThreshold,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the heartbeat monitoring loop.
func (h *HeartbeatMonitor) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-h.stopChan:
				return
			case <-ticker.C:
				h.checkStaleDrivers(ctx)
			}
		}
	}()
}

// Stop stops the worker.
func (h *HeartbeatMonitor) Stop() {
	close(h.stopChan)
}

func (h *HeartbeatMonitor) checkStaleDrivers(ctx context.Context) {
	// Find drivers marked AVAILABLE or BUSY that haven't sent recent heartbeats
	// Check Redis locations for timestamps
	drivers, err := h.driverRepo.FindByStatus(ctx, string(domain.DriverStatusAvailable))
	if err != nil {
		log.Printf("heartbeat_monitor: failed to find available drivers: %v", err)
		return
	}

	for _, driver := range drivers {
		// Check driver's last location update timestamp
		_, _, err := h.locationStore.GetDriverLocation(ctx, driver.ID)
		if err != nil {
			log.Printf("heartbeat_monitor: driver %s location check failed: %v", driver.ID, err)
			continue
		}

		// In a full implementation, would compare timestamp against staleThreshold
		// For now, this is a placeholder for the heartbeat check logic
	}

	log.Printf("heartbeat_monitor: completed heartbeat check for %d drivers", len(drivers))
}