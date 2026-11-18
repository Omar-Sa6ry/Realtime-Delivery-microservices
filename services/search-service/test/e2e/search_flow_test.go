package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	appsearch "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/redis"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_SearchIndexingAndQueryFlow(t *testing.T) {
	ctx := context.Background()

	// 1. Setup in-memory mock repository & embedded Redis cache
	repo := testutil.NewFakeRepository()
	mr := miniredis.RunT(t)
	cache := redis.NewCache(&config.Config{
		RedisHost: mr.Host(),
		RedisPort: mr.Port(),
		RedisTTL:  60 * time.Second,
	})

	indexingSvc := indexing.NewService(repo)
	searchSvc := appsearch.NewService(repo, cache)

	// 2. Ingest / Index a new Delivery document
	now := time.Now().UTC()
	doc := search.DeliveryDocument{
		DeliveryID: "del_e2e_999",
		Status:     "DELIVERED",
		Pickup: search.GeoAddress{
			City:    "Cairo",
			Country: "Egypt",
		},
		Dropoff: search.GeoAddress{
			City:    "Giza",
			Country: "Egypt",
		},
		DriverID:      "drv_e2e_888",
		CustomerID:    "usr_e2e_777",
		SourceVersion: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
		IndexedAt:     now,
	}

	err := indexingSvc.UpsertDelivery(ctx, doc)
	require.NoError(t, err)

	// Assert indexed into the repository store
	require.Len(t, repo.UpsertDeliveries, 1)
	assert.Equal(t, "del_e2e_999", repo.UpsertDeliveries[0].DeliveryID)
	assert.Equal(t, "DELIVERED", repo.UpsertDeliveries[0].Status)

	// 3. Prepare search response simulation
	repo.DeliveriesResult = search.SearchResult[search.DeliveryDocument]{
		Items: []search.DeliveryDocument{doc},
		PageInfo: search.PageInfo{
			Total:       1,
			HasNextPage: false,
		},
	}

	// 4. Query search service (First pass - Cache Miss)
	query := search.DeliverySearchQuery{
		Query:  "Cairo",
		Status: "DELIVERED",
		Pagination: search.PaginationInput{
			Limit: 10,
		},
	}

	res1, err := searchSvc.SearchDeliveries(ctx, query)
	require.NoError(t, err)
	require.NotNil(t, res1)
	assert.Equal(t, int64(1), res1.PageInfo.Total)
	require.Len(t, res1.Items, 1)
	assert.Equal(t, "del_e2e_999", res1.Items[0].DeliveryID)
	assert.Equal(t, 1, len(repo.DeliveriesQueries), "Expected repository to be hit once")

	// 5. Query search service again (Second pass - Cache Hit)
	res2, err := searchSvc.SearchDeliveries(ctx, query)
	require.NoError(t, err)
	require.NotNil(t, res2)
	assert.Equal(t, int64(1), res2.PageInfo.Total)
	assert.Equal(t, "del_e2e_999", res2.Items[0].DeliveryID)
	assert.Equal(t, 1, len(repo.DeliveriesQueries), "Expected repository not to be queried again due to cache hit")
}
