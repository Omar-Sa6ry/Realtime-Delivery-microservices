package search

import (
	"context"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/redis"
)

type Service struct {
	repo  search.SearchRepository
	cache *redis.Cache
}

func NewService(repo search.SearchRepository, cache *redis.Cache) *Service {
	return &Service{
		repo:  repo,
		cache: cache,
	}
}

func (s *Service) SearchDeliveries(ctx context.Context, q search.DeliverySearchQuery) (search.SearchResult[search.DeliveryDocument], error) {
	cacheKey := s.cache.GenerateKey("deliveries", q)
	var cached search.SearchResult[search.DeliveryDocument]
	if s.cache.Get(ctx, cacheKey, &cached) {
		return cached, nil
	}

	res, err := s.repo.SearchDeliveries(ctx, q)
	if err != nil {
		return search.SearchResult[search.DeliveryDocument]{}, err
	}

	_ = s.cache.Set(ctx, cacheKey, res, 60*time.Second)
	return res, nil
}

func (s *Service) SearchDrivers(ctx context.Context, q search.DriverSearchQuery) (search.SearchResult[search.DriverDocument], error) {
	cacheKey := s.cache.GenerateKey("drivers", q)
	var cached search.SearchResult[search.DriverDocument]
	if s.cache.Get(ctx, cacheKey, &cached) {
		return cached, nil
	}

	res, err := s.repo.SearchDrivers(ctx, q)
	if err != nil {
		return search.SearchResult[search.DriverDocument]{}, err
	}

	_ = s.cache.Set(ctx, cacheKey, res, 60*time.Second)
	return res, nil
}

func (s *Service) SearchMedia(ctx context.Context, q search.MediaSearchQuery) (search.SearchResult[search.MediaDocument], error) {
	cacheKey := s.cache.GenerateKey("media", q)
	var cached search.SearchResult[search.MediaDocument]
	if s.cache.Get(ctx, cacheKey, &cached) {
		return cached, nil
	}

	res, err := s.repo.SearchMedia(ctx, q)
	if err != nil {
		return search.SearchResult[search.MediaDocument]{}, err
	}

	_ = s.cache.Set(ctx, cacheKey, res, 60*time.Second)
	return res, nil
}

func (s *Service) Autocomplete(ctx context.Context, q search.AutocompleteQuery) (search.AutocompleteResult, error) {
	cacheKey := s.cache.GenerateKey("suggest", q)
	var cached search.AutocompleteResult
	if s.cache.Get(ctx, cacheKey, &cached) {
		return cached, nil
	}

	res, err := s.repo.Autocomplete(ctx, q)
	if err != nil {
		return search.AutocompleteResult{}, err
	}

	_ = s.cache.Set(ctx, cacheKey, res, 300*time.Second)
	return res, nil
}

func (s *Service) NearbyDeliveries(ctx context.Context, q search.GeoSearchQuery) (search.SearchResult[search.DeliveryDocument], error) {
	return s.repo.NearbyDeliveries(ctx, q)
}

func (s *Service) NearbyDrivers(ctx context.Context, q search.GeoSearchQuery) (search.SearchResult[search.DriverDocument], error) {
	return s.repo.NearbyDrivers(ctx, q)
}
