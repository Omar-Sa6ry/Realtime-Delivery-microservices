package graphql

import (
	"context"
	"fmt"

	"github.com/graph-gophers/dataloader/v7"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/ports"
)

type contextKey string

const loadersKey contextKey = "gql_loaders"
type Loaders struct {
	PaymentByID *dataloader.Loader[string, *domain.Payment]
}

func NewLoaders(paymentRepo ports.PaymentRepository) *Loaders {
	return &Loaders{
		PaymentByID: dataloader.NewBatchedLoader(
			newPaymentBatchFn(paymentRepo),
			dataloader.WithCache[string, *domain.Payment](&dataloader.NoCache[string, *domain.Payment]{}),
		),
	}
}

func newPaymentBatchFn(repo ports.PaymentRepository) dataloader.BatchFunc[string, *domain.Payment] {
	return func(ctx context.Context, ids []string) []*dataloader.Result[*domain.Payment] {
		payments, err := repo.FindByIDs(ctx, ids)

		paymentMap := make(map[string]*domain.Payment, len(payments))
		if err == nil {
			for _, p := range payments {
				paymentMap[p.ID] = p
			}
		}

		results := make([]*dataloader.Result[*domain.Payment], len(ids))
		for i, id := range ids {
			if err != nil {
				results[i] = &dataloader.Result[*domain.Payment]{Error: fmt.Errorf("payment batch load failed: %w", err)}
				continue
			}
			p, ok := paymentMap[id]
			if !ok {
				results[i] = &dataloader.Result[*domain.Payment]{Error: domain.ErrPaymentNotFound}
			} else {
				results[i] = &dataloader.Result[*domain.Payment]{Data: p}
			}
		}
		return results
	}
}

func WithLoaders(ctx context.Context, loaders *Loaders) context.Context {
	return context.WithValue(ctx, loadersKey, loaders)
}

func For(ctx context.Context) *Loaders {
	if l, ok := ctx.Value(loadersKey).(*Loaders); ok {
		return l
	}
	return nil
}

func LoadPayment(ctx context.Context, id string) (*domain.Payment, error) {
	loaders := For(ctx)
	if loaders == nil {
		return nil, fmt.Errorf("no dataloaders in context")
	}
	thunk := loaders.PaymentByID.Load(ctx, id)
	return thunk()
}
