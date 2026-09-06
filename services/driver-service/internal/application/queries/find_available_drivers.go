package queries

import (
	"context"
	"log"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

// FindAvailableDriversQuery finds drivers available near the given coordinates.
type FindAvailableDriversQuery struct {
	Lat          float64
	Lng          float64
	RadiusKm     float64
	VehicleType  string
	DeliveryID   string
	driverRepo   ports.DriverRepository
	locationStore ports.LocationStore
}

// NewFindAvailableDriversQuery creates a new FindAvailableDriversQuery.
func NewFindAvailableDriversQuery(lat, lng, radiusKm float64, vehicleType, deliveryID string,
	driverRepo ports.DriverRepository, locationStore ports.LocationStore) *FindAvailableDriversQuery {

	return &FindAvailableDriversQuery{
		Lat:          lat,
		Lng:          lng,
		RadiusKm:     radiusKm,
		VehicleType:  vehicleType,
		DeliveryID:   deliveryID,
		driverRepo:   driverRepo,
		locationStore: locationStore,
	}
}

// Execute finds available drivers using Redis GEO search and MongoDB state filtering.
func (q *FindAvailableDriversQuery) Execute(ctx context.Context) ([]domain.Candidate, error) {
	// Use Redis GEO to find nearby drivers
	// Filter by AVAILABLE state from MongoDB
	// Rank candidates by distance

	// GEOSearch would be implemented via locationStore
	// For now, find drivers by status from MongoDB
	drivers, err := q.driverRepo.FindAvailableByLocation(ctx, q.Lat, q.Lng, q.RadiusKm, q.VehicleType)
	if err != nil {
		log.Printf("find_available_drivers: failed to find available drivers: %v", err)
		return nil, err
	}

	// Convert to candidates
	var candidates []domain.Candidate
	for _, d := range drivers {
		candidates = append(candidates, domain.Candidate{
			DriverID:       d.ID,
			DistanceMeters: 0, // would be calculated from GEOSEARCH
			VehicleType:    d.Vehicle.Type,
			Status:         d.Status,
		})
	}

	return candidates, nil
}