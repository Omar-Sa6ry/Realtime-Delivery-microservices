package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type DeliveryAnalyticsService struct {
	repo  ports.AnalyticsQueryRepository
	cache ports.CacheRepository
	ttl   time.Duration
}

func NewDeliveryAnalyticsService(repo ports.AnalyticsQueryRepository, cache ports.CacheRepository, ttl time.Duration) *DeliveryAnalyticsService {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &DeliveryAnalyticsService{repo: repo, cache: cache, ttl: ttl}
}

func (s *DeliveryAnalyticsService) GetDeliveryAnalytics(ctx context.Context, f ports.DeliveryAnalyticsFilter, scope string) (*ports.DeliveryAnalytics, error) {
	if err := f.Range.Validate(); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("delivery:scope=%s:from=%s:to=%s:gran=%s:city=%s:driver=%s:%s",
		scope, f.Range.From.UTC().Format(time.RFC3339), f.Range.To.UTC().Format(time.RFC3339),
		f.Range.Granularity, f.CityID, f.DriverID, cacheVersion)
	return fetchCached(ctx, s.cache, key, s.ttl, func(ctx context.Context) (*ports.DeliveryAnalytics, error) {
		return s.repo.DeliveryAnalytics(ctx, f)
	})
}
