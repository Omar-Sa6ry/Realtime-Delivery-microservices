package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type PaymentAnalyticsService struct {
	repo  ports.AnalyticsQueryRepository
	cache ports.CacheRepository
	ttl   time.Duration
}

func NewPaymentAnalyticsService(repo ports.AnalyticsQueryRepository, cache ports.CacheRepository, ttl time.Duration) *PaymentAnalyticsService {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &PaymentAnalyticsService{repo: repo, cache: cache, ttl: ttl}
}

func (s *PaymentAnalyticsService) GetPaymentAnalytics(ctx context.Context, f ports.PaymentAnalyticsFilter, scope string) (*ports.PaymentAnalytics, error) {
	if err := f.Range.Validate(); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("payment:scope=%s:from=%s:to=%s:gran=%s:provider=%s:%s",
		scope, f.Range.From.UTC().Format(time.RFC3339), f.Range.To.UTC().Format(time.RFC3339),
		f.Range.Granularity, f.Provider, cacheVersion)
	return fetchCached(ctx, s.cache, key, s.ttl, func(ctx context.Context) (*ports.PaymentAnalytics, error) {
		return s.repo.PaymentAnalytics(ctx, f)
	})
}
