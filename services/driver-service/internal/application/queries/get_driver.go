package queries

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
)

type GetDriverQuery struct {
	DriverID string
	driverRepo ports.DriverRepository
}

func NewGetDriverQuery(driverID string, driverRepo ports.DriverRepository) *GetDriverQuery {
	return &GetDriverQuery{
		DriverID: driverID,
		driverRepo: driverRepo,
	}
}

func (q *GetDriverQuery) Execute(ctx context.Context) (*domain.Driver, error) {
	driver, err := q.driverRepo.FindByID(ctx, q.DriverID)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		return nil, domain.ErrDriverNotFound
	}
	return driver, nil
}