package graphql

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
)

type contextKey string

const loadersKey contextKey = "gql_loaders"

type DriverStub struct {
	ID string
}

type DeliveryStub struct {
	ID string
}

type Loaders struct {
	DriverByID   *dataloader.Loader[string, *DriverStub]
	DeliveryByID *dataloader.Loader[string, *DeliveryStub]
}

func NewLoaders() *Loaders {
	return &Loaders{
		DriverByID: dataloader.NewBatchedLoader(
			func(ctx context.Context, ids []string) []*dataloader.Result[*DriverStub] {
				results := make([]*dataloader.Result[*DriverStub], len(ids))
				for i, id := range ids {
					results[i] = &dataloader.Result[*DriverStub]{Data: &DriverStub{ID: id}}
				}
				return results
			},
			dataloader.WithCache[string, *DriverStub](&dataloader.NoCache[string, *DriverStub]{}),
		),
		DeliveryByID: dataloader.NewBatchedLoader(
			func(ctx context.Context, ids []string) []*dataloader.Result[*DeliveryStub] {
				results := make([]*dataloader.Result[*DeliveryStub], len(ids))
				for i, id := range ids {
					results[i] = &dataloader.Result[*DeliveryStub]{Data: &DeliveryStub{ID: id}}
				}
				return results
			},
			dataloader.WithCache[string, *DeliveryStub](&dataloader.NoCache[string, *DeliveryStub]{}),
		),
	}
}

func WithLoaders(ctx context.Context, loaders *Loaders) context.Context {
	return context.WithValue(ctx, loadersKey, loaders)
}

func GetLoaders(ctx context.Context) *Loaders {
	if l, ok := ctx.Value(loadersKey).(*Loaders); ok {
		return l
	}
	return nil
}

func LoadDriver(ctx context.Context, id string) (*DriverStub, error) {
	if l := GetLoaders(ctx); l != nil {
		return l.DriverByID.Load(ctx, id)()
	}
	return &DriverStub{ID: id}, nil
}

func LoadDelivery(ctx context.Context, id string) (*DeliveryStub, error) {
	if l := GetLoaders(ctx); l != nil {
		return l.DeliveryByID.Load(ctx, id)()
	}
	return &DeliveryStub{ID: id}, nil
}
