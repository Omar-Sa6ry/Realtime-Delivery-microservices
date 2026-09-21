package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

type DriverAnalyticsService struct {
	repo  ports.AnalyticsQueryRepository
	cache ports.CacheRepository
	ttl   time.Duration
}

func NewDriverAnalyticsService(repo ports.AnalyticsQueryRepository, cache ports.CacheRepository, ttl time.Duration) *DriverAnalyticsService {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &DriverAnalyticsService{repo: repo, cache: cache, ttl: ttl}
}

func (s *DriverAnalyticsService) GetDriverAnalytics(ctx context.Context, f ports.DriverAnalyticsFilter, scope string) (*ports.DriverAnalytics, error) {
	if err := f.Range.Validate(); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("driver:scope=%s:from=%s:to=%s:gran=%s:driver=%s:%s",
		scope, f.Range.From.UTC().Format(time.RFC3339), f.Range.To.UTC().Format(time.RFC3339),
		f.Range.Granularity, f.DriverID, cacheVersion)
	return fetchCached(ctx, s.cache, key, s.ttl, func(ctx context.Context) (*ports.DriverAnalytics, error) {
		return s.repo.DriverAnalytics(ctx, f)
	})
}

func (s *DriverAnalyticsService) GetTopDrivers(ctx context.Context, r ports.TimeRange, limit int, scope string) (*ports.DriverAnalyticsPage, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	key := fmt.Sprintf("topdrivers:scope=%s:from=%s:to=%s:gran=%s:limit=%d:%s",
		scope, r.From.UTC().Format(time.RFC3339), r.To.UTC().Format(time.RFC3339),
		r.Granularity, limit, cacheVersion)
	return fetchCached(ctx, s.cache, key, s.ttl, func(ctx context.Context) (*ports.DriverAnalyticsPage, error) {
		return s.repo.TopDrivers(ctx, r, limit)
	})
}
