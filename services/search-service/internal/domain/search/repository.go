package search

import (
	"context"
)

type SearchRepository interface {
	SearchDeliveries(ctx context.Context, q DeliverySearchQuery) (SearchResult[DeliveryDocument], error)
	SearchDrivers(ctx context.Context, q DriverSearchQuery) (SearchResult[DriverDocument], error)
	SearchMedia(ctx context.Context, q MediaSearchQuery) (SearchResult[MediaDocument], error)
	SearchUsers(ctx context.Context, q UserSearchQuery) (SearchResult[UserDocument], error)
	Autocomplete(ctx context.Context, q AutocompleteQuery) (AutocompleteResult, error)
	NearbyDeliveries(ctx context.Context, q GeoSearchQuery) (SearchResult[DeliveryDocument], error)
	NearbyDrivers(ctx context.Context, q GeoSearchQuery) (SearchResult[DriverDocument], error)

	// Indexing Operations

	UpsertDelivery(ctx context.Context, doc DeliveryDocument) error
	DeleteDelivery(ctx context.Context, id string) error
	UpsertDriver(ctx context.Context, doc DriverDocument) error
	DeleteDriver(ctx context.Context, id string) error
	UpsertMedia(ctx context.Context, doc MediaDocument) error
	DeleteMedia(ctx context.Context, id string) error
	UpsertUser(ctx context.Context, doc UserDocument) error
	DeleteUser(ctx context.Context, id string) error
	BulkUpsertDeliveries(ctx context.Context, docs []DeliveryDocument) error
	BulkUpsertDrivers(ctx context.Context, docs []DriverDocument) error
	BulkUpsertMedia(ctx context.Context, docs []MediaDocument) error

	// Index Management

	GetDocumentVersion(ctx context.Context, index, id string) (int64, error)
	IndexExists(ctx context.Context, index string) (bool, error)
	CountDocuments(ctx context.Context, index string) (int64, error)
	Reindex(ctx context.Context, source, target string) error
	SwitchAlias(ctx context.Context, alias, oldIndex, newIndex string) error

	// Health

	Ping(ctx context.Context) error
	ClusterHealth(ctx context.Context) (string, error)
}
