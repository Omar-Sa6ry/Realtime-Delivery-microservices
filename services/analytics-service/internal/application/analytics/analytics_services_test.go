package analytics

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchCached_CacheHit(t *testing.T) {
	cache := testutil.NewMockCacheRepository()

	expected := &ports.PlatformOverview{
		TotalDeliveries:     100,
		CompletedDeliveries: 80,
		DataAsOf:            time.Now().UTC(),
	}
	data, _ := json.Marshal(expected)
	cache.Set(context.Background(), "test-key", data, time.Minute)

	result, err := fetchCached(context.Background(), cache, "test-key", time.Minute, func(ctx context.Context) (*ports.PlatformOverview, error) {
		t.Fatal("load function should not be called on cache hit")
		return nil, nil
	})

	require.NoError(t, err)
	assert.Equal(t, expected.TotalDeliveries, result.TotalDeliveries)
	assert.Equal(t, expected.CompletedDeliveries, result.CompletedDeliveries)
}

func TestFetchCached_CacheMiss(t *testing.T) {
	cache := testutil.NewMockCacheRepository()

	expected := &ports.PlatformOverview{
		TotalDeliveries:     100,
		CompletedDeliveries: 80,
		DataAsOf:            time.Now().UTC(),
	}

	result, err := fetchCached(context.Background(), cache, "test-key", time.Minute, func(ctx context.Context) (*ports.PlatformOverview, error) {
		return expected, nil
	})

	require.NoError(t, err)
	assert.Equal(t, expected.TotalDeliveries, result.TotalDeliveries)
	assert.Equal(t, expected.CompletedDeliveries, result.CompletedDeliveries)

	// Verify it was cached
	val, hit, _ := cache.Get(context.Background(), "test-key")
	assert.True(t, hit)
	var cached ports.PlatformOverview
	json.Unmarshal(val, &cached)
	assert.Equal(t, expected.TotalDeliveries, cached.TotalDeliveries)
}

func TestFetchCached_LoadError(t *testing.T) {
	cache := testutil.NewMockCacheRepository()

	_, err := fetchCached(context.Background(), cache, "test-key", time.Minute, func(ctx context.Context) (*ports.PlatformOverview, error) {
		return nil, assert.AnError
	})

	assert.Error(t, err)
}

func TestFetchCached_NilCache(t *testing.T) {
	expected := &ports.PlatformOverview{
		TotalDeliveries:     100,
		CompletedDeliveries: 80,
		DataAsOf:            time.Now().UTC(),
	}

	result, err := fetchCached(context.Background(), nil, "test-key", time.Minute, func(ctx context.Context) (*ports.PlatformOverview, error) {
		return expected, nil
	})

	require.NoError(t, err)
	assert.Equal(t, expected.TotalDeliveries, result.TotalDeliveries)
}

func TestFetchCached_CacheGetError(t *testing.T) {
	cache := testutil.NewMockCacheRepository()
	// We can't easily simulate Get error with current mock, so skip this test
}

func TestNewDeliveryAnalyticsService(t *testing.T) {
	cache := testutil.NewMockCacheRepository()
	repo := testutil.NewMockAnalyticsQueryRepository()

	svc := NewDeliveryAnalyticsService(repo, cache, -1)
	assert.NotNil(t, svc)
	assert.Equal(t, 5*time.Minute, svc.ttl)

	svc2 := NewDeliveryAnalyticsService(repo, cache, 10*time.Minute)
	assert.Equal(t, 10*time.Minute, svc2.ttl)
}

func TestDeliveryAnalyticsService_GetDeliveryAnalytics(t *testing.T) {
	cache := testutil.NewMockCacheRepository()
	repo := testutil.NewMockAnalyticsQueryRepository()

	expected := &ports.DeliveryAnalytics{
		Total:          100,
		Completed:      80,
		Cancelled:      10,
		Failed:         10,
		CompletionRate: 0.8,
		DataAsOf:       time.Now().UTC(),
	}
	repo.SetDeliveryAnalytics(expected)

	svc := NewDeliveryAnalyticsService(repo, cache, 5*time.Minute)

	filter := ports.DeliveryAnalyticsFilter{
		Range: ports.TimeRange{
			From:        time.Now().UTC().Add(-24 * time.Hour),
			To:          time.Now().UTC(),
			Granularity: ports.GranularityDay,
		},
		CityID:   "city-1",
		DriverID: "driver-1",
	}

	result, err := svc.GetDeliveryAnalytics(context.Background(), filter, "test-scope")

	require.NoError(t, err)
	assert.Equal(t, expected.Total, result.Total)
	assert.Equal(t, expected.Completed, result.Completed)
}

func TestDeliveryAnalyticsService_GetDeliveryAnalytics_InvalidRange(t *testing.T) {
	repo := testutil.NewMockAnalyticsQueryRepository()
	cache := testutil.NewMockCacheRepository()
	svc := NewDeliveryAnalyticsService(repo, cache, 5*time.Minute)

	filter := ports.DeliveryAnalyticsFilter{
		Range: ports.TimeRange{
			From: time.Now().UTC(),
			To:   time.Now().UTC().Add(-24 * time.Hour), // to before from
		},
	}

	_, err := svc.GetDeliveryAnalytics(context.Background(), filter, "test-scope")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of order timestamps")
}

func TestNewDriverAnalyticsService(t *testing.T) {
	cache := testutil.NewMockCacheRepository()

	svc := NewDriverAnalyticsService(nil, cache, -1)
	assert.NotNil(t, svc)
	assert.Equal(t, 5*time.Minute, svc.ttl)
}

func TestDriverAnalyticsService_GetDriverAnalytics(t *testing.T) {
	repo := testutil.NewMockAnalyticsQueryRepository()
	cache := testutil.NewMockCacheRepository()

	expected := &ports.DriverAnalytics{
		DriverID:             "driver-1",
		Offers:               50,
		Accepted:             40,
		Rejected:             5,
		Expired:              5,
		AcceptanceRate:       0.8,
		CompletedDeliveries:  35,
		DataAsOf:             time.Now().UTC(),
	}
	repo.SetDriverAnalytics(expected)

	svc := NewDriverAnalyticsService(repo, cache, 5*time.Minute)

	filter := ports.DriverAnalyticsFilter{
		Range: ports.TimeRange{
			From:        time.Now().UTC().Add(-24 * time.Hour),
			To:          time.Now().UTC(),
			Granularity: ports.GranularityDay,
		},
		DriverID: "driver-1",
	}

	result, err := svc.GetDriverAnalytics(context.Background(), filter, "test-scope")

	require.NoError(t, err)
	assert.Equal(t, expected.DriverID, result.DriverID)
	assert.Equal(t, expected.Offers, result.Offers)
}

func TestDriverAnalyticsService_GetTopDrivers(t *testing.T) {
	repo := testutil.NewMockAnalyticsQueryRepository()
	cache := testutil.NewMockCacheRepository()

	expected := &ports.DriverAnalyticsPage{
		Pagination: ports.Pagination{TotalItems: 10, CurrentPage: 1},
		Items: []ports.DriverAnalytics{
			{DriverID: "driver-1", Offers: 50, Accepted: 40},
			{DriverID: "driver-2", Offers: 40, Accepted: 30},
		},
	}
	repo.SetTopDriversPage(expected)

	svc := NewDriverAnalyticsService(repo, cache, 5*time.Minute)

	result, err := svc.GetTopDrivers(context.Background(), ports.TimeRange{
		From:        time.Now().UTC().Add(-24 * time.Hour),
		To:          time.Now().UTC(),
		Granularity: ports.GranularityDay,
	}, 10, "test-scope")

	require.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "driver-1", result.Items[0].DriverID)
	assert.Equal(t, 10, result.Pagination.TotalItems)
}

func TestDriverAnalyticsService_GetTopDrivers_LimitClamping(t *testing.T) {
	repo := testutil.NewMockAnalyticsQueryRepository()
	cache := testutil.NewMockCacheRepository()
	repo.SetTopDriversPage(&ports.DriverAnalyticsPage{Items: []ports.DriverAnalytics{}})

	svc := NewDriverAnalyticsService(repo, cache, 5*time.Minute)

	// Test limit > 100 clamped to 100
	_, err := svc.GetTopDrivers(context.Background(), ports.TimeRange{
		From:        time.Now().UTC().Add(-24 * time.Hour),
		To:          time.Now().UTC(),
		Granularity: ports.GranularityDay,
	}, 200, "test-scope")
	assert.NoError(t, err) // Should not error, just clamp

	// Test limit <= 0 defaults to 10
	_, err = svc.GetTopDrivers(context.Background(), ports.TimeRange{
		From:        time.Now().UTC().Add(-24 * time.Hour),
		To:          time.Now().UTC(),
		Granularity: ports.GranularityDay,
	}, 0, "test-scope")
	assert.NoError(t, err)
}

func TestNewPaymentAnalyticsService(t *testing.T) {
	repo := testutil.NewMockAnalyticsQueryRepository()
	cache := testutil.NewMockCacheRepository()

	svc := NewPaymentAnalyticsService(repo, cache, -1)
	assert.NotNil(t, svc)
	assert.Equal(t, 5*time.Minute, svc.ttl)
}

func TestPaymentAnalyticsService_GetPaymentAnalytics(t *testing.T) {
	repo := testutil.NewMockAnalyticsQueryRepository()
	cache := testutil.NewMockCacheRepository()

	expected := &ports.PaymentAnalytics{
		AuthorizationCount:       100,
		AuthorizationSuccessRate: 0.9,
		CaptureCount:             85,
		CapturedAmount:           "8500.00",
		RefundCount:              5,
		RefundedAmount:           "250.00",
		RefundRate:               0.05,
		DataAsOf:                 time.Now().UTC(),
	}
	repo.SetPaymentAnalytics(expected)

	svc := NewPaymentAnalyticsService(repo, cache, 5*time.Minute)

	filter := ports.PaymentAnalyticsFilter{
		Range: ports.TimeRange{
			From:        time.Now().UTC().Add(-24 * time.Hour),
			To:          time.Now().UTC(),
			Granularity: ports.GranularityDay,
		},
		Provider: "STRIPE",
	}

	result, err := svc.GetPaymentAnalytics(context.Background(), filter, "test-scope")

	require.NoError(t, err)
	assert.Equal(t, expected.AuthorizationCount, result.AuthorizationCount)
	assert.Equal(t, expected.CapturedAmount, result.CapturedAmount)
}

func TestNewPlatformOverviewService(t *testing.T) {
	repo := testutil.NewMockAnalyticsQueryRepository()
	cache := testutil.NewMockCacheRepository()

	svc := NewPlatformOverviewService(repo, cache, -1)
	assert.NotNil(t, svc)
	assert.Equal(t, 5*time.Minute, svc.ttl)
}

func TestPlatformOverviewService_GetPlatformOverview(t *testing.T) {
	repo := testutil.NewMockAnalyticsQueryRepository()
	cache := testutil.NewMockCacheRepository()

	expected := &ports.PlatformOverview{
		TotalDeliveries:          1000,
		CompletedDeliveries:      850,
		CancelledDeliveries:      100,
		FailedDeliveries:         50,
		CompletionRate:           0.85,
		AverageDeliveryDurationS: 1800.5,
		PaymentCapturedAmount:    "85000.00",
		RefundedAmount:           "2500.00",
		DriverAcceptanceRate:     0.82,
		DataAsOf:                 time.Now().UTC(),
	}
	repo.SetPlatformOverview(expected)

	svc := NewPlatformOverviewService(repo, cache, 5*time.Minute)

	result, err := svc.GetPlatformOverview(context.Background(), ports.TimeRange{
		From:        time.Now().UTC().Add(-24 * time.Hour),
		To:          time.Now().UTC(),
		Granularity: ports.GranularityDay,
	}, "test-scope")

	require.NoError(t, err)
	assert.Equal(t, expected.TotalDeliveries, result.TotalDeliveries)
	assert.Equal(t, expected.CompletionRate, result.CompletionRate)
	assert.Equal(t, expected.PaymentCapturedAmount, result.PaymentCapturedAmount)
}