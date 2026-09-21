package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

const cacheVersion = "v1"

func fetchCached[T any](ctx context.Context, cache ports.CacheRepository, key string, ttl time.Duration, load func(context.Context) (T, error)) (T, error) {
	var zero T
	if cache != nil {
		if data, hit, err := cache.Get(ctx, key); err == nil && hit {
			var cached T
			if err := json.Unmarshal(data, &cached); err == nil {
				return cached, nil
			}
		}
	}
	result, err := load(ctx)
	if err != nil {
		return zero, err
	}
	if cache != nil {
		if data, err := json.Marshal(result); err == nil {
			_ = cache.Set(ctx, key, data, ttl)
		}
	}
	return result, nil
}

type PlatformOverviewService struct {
	repo  ports.AnalyticsQueryRepository
	cache ports.CacheRepository
	ttl   time.Duration
}

func NewPlatformOverviewService(repo ports.AnalyticsQueryRepository, cache ports.CacheRepository, ttl time.Duration) *PlatformOverviewService {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &PlatformOverviewService{repo: repo, cache: cache, ttl: ttl}
}

func (s *PlatformOverviewService) GetPlatformOverview(ctx context.Context, r ports.TimeRange, scope string) (*ports.PlatformOverview, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("platform:scope=%s:from=%s:to=%s:gran=%s:%s",
		scope, r.From.UTC().Format(time.RFC3339), r.To.UTC().Format(time.RFC3339), r.Granularity, cacheVersion)
	return fetchCached(ctx, s.cache, key, s.ttl, func(ctx context.Context) (*ports.PlatformOverview, error) {
		return s.repo.PlatformOverview(ctx, r)
	})
}
