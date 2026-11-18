// Package testutil provides shared fakes for search-service tests.
package testutil

import (
	"context"
	"sync"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
)

// FakeRepository is an in-memory search.SearchRepository for unit tests.
type FakeRepository struct {
	mu sync.Mutex

	// Search results / errors
	DeliveriesResult    search.SearchResult[search.DeliveryDocument]
	DeliveriesErr       error
	DriversResult       search.SearchResult[search.DriverDocument]
	DriversErr          error
	MediaResult         search.SearchResult[search.MediaDocument]
	MediaErr            error
	UsersResult         search.SearchResult[search.UserDocument]
	UsersErr            error
	AutocompleteResult  search.AutocompleteResult
	AutocompleteErr     error
	NearbyDelivResult   search.SearchResult[search.DeliveryDocument]
	NearbyDelivErr      error
	NearbyDriversResult search.SearchResult[search.DriverDocument]
	NearbyDriversErr    error

	// Indexing errors
	UpsertDeliveryErr error
	DeleteDeliveryErr error
	UpsertDriverErr   error
	DeleteDriverErr   error
	UpsertMediaErr    error
	DeleteMediaErr    error
	UpsertUserErr     error
	DeleteUserErr     error
	BulkDeliveriesErr error
	BulkDriversErr    error
	BulkMediaErr      error

	// Index management
	Version    int64
	VersionErr error
	Exists     bool
	ExistsErr  error
	Count      int64
	CountErr   error
	ReindexErr error
	AliasErr   error
	PingErr    error
	Health     string
	HealthErr  error

	// Recorded calls
	DeliveriesQueries []search.DeliverySearchQuery
	DriversQueries    []search.DriverSearchQuery
	MediaQueries      []search.MediaSearchQuery
	UsersQueries      []search.UserSearchQuery
	AutocompleteQs    []search.AutocompleteQuery
	NearbyDelivQs     []search.GeoSearchQuery
	NearbyDriversQs   []search.GeoSearchQuery
	UpsertDeliveries  []search.DeliveryDocument
	DeletedDeliveries []string
	UpsertDrivers     []search.DriverDocument
	DeletedDrivers    []string
	UpsertMediaDocs   []search.MediaDocument
	DeletedMedia      []string
	UpsertUsers       []search.UserDocument
	DeletedUsers      []string
	ReindexCalls      [][2]string
	SwitchAliasCalls  [][3]string

	// ReindexDone is closed (if non-nil) when Reindex is invoked.
	ReindexDone chan struct{}
}

func NewFakeRepository() *FakeRepository {
	return &FakeRepository{
		DeliveriesResult: search.SearchResult[search.DeliveryDocument]{Items: []search.DeliveryDocument{}},
		DriversResult:    search.SearchResult[search.DriverDocument]{Items: []search.DriverDocument{}},
		MediaResult:      search.SearchResult[search.MediaDocument]{Items: []search.MediaDocument{}},
		UsersResult:      search.SearchResult[search.UserDocument]{Items: []search.UserDocument{}},
		NearbyDelivResult: search.SearchResult[search.DeliveryDocument]{
			Items: []search.DeliveryDocument{},
		},
		NearbyDriversResult: search.SearchResult[search.DriverDocument]{
			Items: []search.DriverDocument{},
		},
		Health: "green",
	}
}

func (f *FakeRepository) SearchDeliveries(_ context.Context, q search.DeliverySearchQuery) (search.SearchResult[search.DeliveryDocument], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DeliveriesQueries = append(f.DeliveriesQueries, q)
	return f.DeliveriesResult, f.DeliveriesErr
}

func (f *FakeRepository) SearchDrivers(_ context.Context, q search.DriverSearchQuery) (search.SearchResult[search.DriverDocument], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DriversQueries = append(f.DriversQueries, q)
	return f.DriversResult, f.DriversErr
}

func (f *FakeRepository) SearchMedia(_ context.Context, q search.MediaSearchQuery) (search.SearchResult[search.MediaDocument], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.MediaQueries = append(f.MediaQueries, q)
	return f.MediaResult, f.MediaErr
}

func (f *FakeRepository) SearchUsers(_ context.Context, q search.UserSearchQuery) (search.SearchResult[search.UserDocument], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UsersQueries = append(f.UsersQueries, q)
	return f.UsersResult, f.UsersErr
}

func (f *FakeRepository) Autocomplete(_ context.Context, q search.AutocompleteQuery) (search.AutocompleteResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.AutocompleteQs = append(f.AutocompleteQs, q)
	return f.AutocompleteResult, f.AutocompleteErr
}

func (f *FakeRepository) NearbyDeliveries(_ context.Context, q search.GeoSearchQuery) (search.SearchResult[search.DeliveryDocument], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.NearbyDelivQs = append(f.NearbyDelivQs, q)
	return f.NearbyDelivResult, f.NearbyDelivErr
}

func (f *FakeRepository) NearbyDrivers(_ context.Context, q search.GeoSearchQuery) (search.SearchResult[search.DriverDocument], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.NearbyDriversQs = append(f.NearbyDriversQs, q)
	return f.NearbyDriversResult, f.NearbyDriversErr
}

func (f *FakeRepository) UpsertDelivery(_ context.Context, doc search.DeliveryDocument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UpsertDeliveries = append(f.UpsertDeliveries, doc)
	return f.UpsertDeliveryErr
}

func (f *FakeRepository) DeleteDelivery(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DeletedDeliveries = append(f.DeletedDeliveries, id)
	return f.DeleteDeliveryErr
}

func (f *FakeRepository) UpsertDriver(_ context.Context, doc search.DriverDocument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UpsertDrivers = append(f.UpsertDrivers, doc)
	return f.UpsertDriverErr
}

func (f *FakeRepository) DeleteDriver(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DeletedDrivers = append(f.DeletedDrivers, id)
	return f.DeleteDriverErr
}

func (f *FakeRepository) UpsertMedia(_ context.Context, doc search.MediaDocument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UpsertMediaDocs = append(f.UpsertMediaDocs, doc)
	return f.UpsertMediaErr
}

func (f *FakeRepository) DeleteMedia(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DeletedMedia = append(f.DeletedMedia, id)
	return f.DeleteMediaErr
}

func (f *FakeRepository) UpsertUser(_ context.Context, doc search.UserDocument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UpsertUsers = append(f.UpsertUsers, doc)
	return f.UpsertUserErr
}

func (f *FakeRepository) DeleteUser(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DeletedUsers = append(f.DeletedUsers, id)
	return f.DeleteUserErr
}

func (f *FakeRepository) BulkUpsertDeliveries(_ context.Context, docs []search.DeliveryDocument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UpsertDeliveries = append(f.UpsertDeliveries, docs...)
	return f.BulkDeliveriesErr
}

func (f *FakeRepository) BulkUpsertDrivers(_ context.Context, docs []search.DriverDocument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UpsertDrivers = append(f.UpsertDrivers, docs...)
	return f.BulkDriversErr
}

func (f *FakeRepository) BulkUpsertMedia(_ context.Context, docs []search.MediaDocument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UpsertMediaDocs = append(f.UpsertMediaDocs, docs...)
	return f.BulkMediaErr
}

func (f *FakeRepository) GetDocumentVersion(_ context.Context, _, _ string) (int64, error) {
	return f.Version, f.VersionErr
}

func (f *FakeRepository) IndexExists(_ context.Context, _ string) (bool, error) {
	return f.Exists, f.ExistsErr
}

func (f *FakeRepository) CountDocuments(_ context.Context, _ string) (int64, error) {
	return f.Count, f.CountErr
}

func (f *FakeRepository) Reindex(_ context.Context, source, target string) error {
	f.mu.Lock()
	f.ReindexCalls = append(f.ReindexCalls, [2]string{source, target})
	done := f.ReindexDone
	f.mu.Unlock()
	if done != nil {
		select {
		case <-done:
		default:
			close(done)
		}
	}
	return f.ReindexErr
}

func (f *FakeRepository) SwitchAlias(_ context.Context, alias, oldIndex, newIndex string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SwitchAliasCalls = append(f.SwitchAliasCalls, [3]string{alias, oldIndex, newIndex})
	return f.AliasErr
}

func (f *FakeRepository) Ping(_ context.Context) error {
	return f.PingErr
}

func (f *FakeRepository) ClusterHealth(_ context.Context) (string, error) {
	return f.Health, f.HealthErr
}
