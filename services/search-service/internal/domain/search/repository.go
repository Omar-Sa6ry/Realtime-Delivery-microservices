package search

import (
	"context"
)

type SearchRepository interface {
	// SearchDeliveries executes a full-text/filter/geo search against the deliveries index.
	SearchDeliveries(ctx context.Context, q DeliverySearchQuery) (SearchResult[DeliveryDocument], error)

	// SearchDrivers executes a search against the drivers index.
	SearchDrivers(ctx context.Context, q DriverSearchQuery) (SearchResult[DriverDocument], error)

	// SearchMedia executes a search against the media index.
	SearchMedia(ctx context.Context, q MediaSearchQuery) (SearchResult[MediaDocument], error)

	// Autocomplete returns prefix-matched suggestions for a given index.
	Autocomplete(ctx context.Context, q AutocompleteQuery) (AutocompleteResult, error)

	// NearbyDeliveries performs a geo-distance query on the deliveries index.
	NearbyDeliveries(ctx context.Context, q GeoSearchQuery) (SearchResult[DeliveryDocument], error)

	// NearbyDrivers performs a geo-distance query on the drivers index.
	NearbyDrivers(ctx context.Context, q GeoSearchQuery) (SearchResult[DriverDocument], error)

	// Indexing Operations

	// UpsertDelivery creates or updates a delivery search document (idempotent).
	UpsertDelivery(ctx context.Context, doc DeliveryDocument) error

	// DeleteDelivery removes a delivery document from the search index.
	DeleteDelivery(ctx context.Context, id string) error

	// UpsertDriver creates or updates a driver search document (idempotent).
	UpsertDriver(ctx context.Context, doc DriverDocument) error

	// DeleteDriver removes a driver document from the search index.
	DeleteDriver(ctx context.Context, id string) error

	// UpsertMedia creates or updates a media search document (idempotent).
	UpsertMedia(ctx context.Context, doc MediaDocument) error

	// DeleteMedia removes a media document from the search index.
	DeleteMedia(ctx context.Context, id string) error

	// BulkUpsertDeliveries indexes a batch of delivery documents atomically.
	BulkUpsertDeliveries(ctx context.Context, docs []DeliveryDocument) error

	// BulkUpsertDrivers indexes a batch of driver documents atomically.
	BulkUpsertDrivers(ctx context.Context, docs []DriverDocument) error

	// BulkUpsertMedia indexes a batch of media documents atomically.
	BulkUpsertMedia(ctx context.Context, docs []MediaDocument) error

	// Index Management

	// GetDocumentVersion returns the source_version of the indexed document.
	// Returns -1 if the document does not exist.
	GetDocumentVersion(ctx context.Context, index, id string) (int64, error)

	// IndexExists reports whether the given index alias or concrete index exists.
	IndexExists(ctx context.Context, index string) (bool, error)

	// CountDocuments returns the total number of documents in an index.
	CountDocuments(ctx context.Context, index string) (int64, error)

	// Reindex re-indexes all documents from source to target index.
	Reindex(ctx context.Context, source, target string) error

	// SwitchAlias atomically points an alias from oldIndex to newIndex.
	SwitchAlias(ctx context.Context, alias, oldIndex, newIndex string) error

	// Health

	// Ping checks whether the search engine is reachable.
	Ping(ctx context.Context) error

	// ClusterHealth returns the cluster health status string (green/yellow/red).
	ClusterHealth(ctx context.Context) (string, error)
}
