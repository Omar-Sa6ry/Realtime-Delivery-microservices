package queries

import (
	"context"
	"log"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

type FindAvailableDriversQuery struct {
	Lat          float64
	Lng          float64
	RadiusKm     float64
	VehicleType  domain.VehicleType
	DeliveryID   string
	driverRepo   ports.DriverRepository
	locationStore ports.LocationStore
}

func NewFindAvailableDriversQuery(lat, lng, radiusKm float64, vehicleType domain.VehicleType, deliveryID string,
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

func (q *FindAvailableDriversQuery) Execute(ctx context.Context) ([]domain.Candidate, error) {
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