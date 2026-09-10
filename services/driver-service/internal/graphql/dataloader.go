package graphql

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/ports"
	"github.com/graph-gophers/dataloader/v7"
)

type Loaders struct {
	DriverLoader *dataloader.Loader[string, *domain.Driver]
}

func NewLoaders(driverRepo ports.DriverRepository) *Loaders {
	batchGetDrivers := func(ctx context.Context, keys []string) []*dataloader.Result[*domain.Driver] {
		results := make([]*dataloader.Result[*domain.Driver], len(keys))
		if len(keys) == 0 {
			return results
		}

		drivers, err := driverRepo.FindByIDs(ctx, keys)
		if err != nil {
			for i := range keys {
				results[i] = &dataloader.Result[*domain.Driver]{Error: err}
			}
			return results
		}

		driverMap := make(map[string]*domain.Driver, len(drivers))
		for _, d := range drivers {
			if d != nil {
				driverMap[d.ID] = d
			}
		}

		for i, id := range keys {
			results[i] = &dataloader.Result[*domain.Driver]{
				Data: driverMap[id],
			}
		}

		return results
	}

	driverLoader := dataloader.NewBatchedLoader(batchGetDrivers)

	return &Loaders{
		DriverLoader: driverLoader,
	}
}

type loadersKey struct{}

func WithLoaders(ctx context.Context, loaders *Loaders) context.Context {
	return context.WithValue(ctx, loadersKey{}, loaders)
}

func GetLoaders(ctx context.Context) *Loaders {
	loaders, ok := ctx.Value(loadersKey{}).(*Loaders)
	if !ok {
		return nil
	}
	return loaders
}
