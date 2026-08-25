package graphql

import (
	"context"
	"errors"
	"time"

	sharedauth "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/auth"
	sharedconstants "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/constants"
	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	appSearch "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/reindex"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	gql "github.com/graphql-go/graphql"
)

type Resolver struct {
	searchService  *appSearch.Service
	reindexService *reindex.Service
}

func NewResolver(searchService *appSearch.Service, reindexService *reindex.Service) *Resolver {
	return &Resolver{
		searchService:  searchService,
		reindexService: reindexService,
	}
}

func (r *Resolver) getAuthContext(ctx context.Context) (userID, userRole string, err error) {
	userID = sharedlogging.GetUserID(ctx)
	if userID == "" {
		if authHeader := getAuthHeader(ctx); authHeader != "" {
			if claims, err := sharedauth.Authenticate(authHeader); err == nil {
				userID = claims.UserID()
			}
		}
	}

	userRole = getUserRole(ctx)

	if userID == "" {
		return "", "", errors.New("authentication required: missing x-user-id header")
	}
	return userID, userRole, nil
}

func getAuthHeader(ctx context.Context) string {
	if val := ctx.Value("authorization"); val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func getUserRole(ctx context.Context) string {
	if val := ctx.Value("x-user-role"); val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func (r *Resolver) ResolveSearchDeliveries(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, userRole, err := r.getAuthContext(ctx)
	if err != nil {
		return nil, err
	}

	input, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid input")
	}

	query := buildDeliverySearchQuery(input, userID, userRole)
	res, err := r.searchService.SearchDeliveries(ctx, query)
	if err != nil {
		return nil, err
	}

	return mapSearchResult(res), nil
}

func (r *Resolver) ResolveSearchDrivers(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, userRole, err := r.getAuthContext(ctx)
	if err != nil {
		return nil, err
	}

	input, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid input")
	}

	query := buildDriverSearchQuery(input, userID, userRole)
	res, err := r.searchService.SearchDrivers(ctx, query)
	if err != nil {
		return nil, err
	}

	return mapSearchResult(res), nil
}

func (r *Resolver) ResolveSearchMedia(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, userRole, err := r.getAuthContext(ctx)
	if err != nil {
		return nil, err
	}

	input, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid input")
	}

	query := buildMediaSearchQuery(input, userID, userRole)
	res, err := r.searchService.SearchMedia(ctx, query)
	if err != nil {
		return nil, err
	}

	return mapSearchResult(res), nil
}

func (r *Resolver) ResolveSearchUsers(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	_, userRole, err := r.getAuthContext(ctx)
	if err != nil {
		return nil, err
	}

	// Only admin can search users
	if userRole != string(sharedconstants.RoleAdmin) {
		return nil, errors.New("forbidden: admin access required")
	}

	input, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid input")
	}

	query := buildUserSearchQuery(input)
	res, err := r.searchService.SearchUsers(ctx, query)
	if err != nil {
		return nil, err
	}

	return mapSearchResult(res), nil
}

func (r *Resolver) ResolveAutocomplete(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, _, err := r.getAuthContext(ctx)
	if err != nil {
		return nil, err
	}
	_ = userID // currently not used but kept for future auth

	input, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid input")
	}

	query := search.AutocompleteQuery{
		Prefix: getString(input, "prefix"),
		Index:  getString(input, "index"),
		Limit:  getInt(input, "limit"),
	}

	res, err := r.searchService.Autocomplete(ctx, query)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"data": map[string]interface{}{
			"suggestions": res.Suggestions,
		},
	}, nil
}

func (r *Resolver) ResolveNearbyDeliveries(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, userRole, err := r.getAuthContext(ctx)
	if err != nil {
		return nil, err
	}

	input, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid input")
	}

	query := buildGeoSearchQuery(input, userID, userRole)
	res, err := r.searchService.NearbyDeliveries(ctx, query)
	if err != nil {
		return nil, err
	}

	return mapSearchResult(res), nil
}

func (r *Resolver) ResolveNearbyDrivers(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	userID, userRole, err := r.getAuthContext(ctx)
	if err != nil {
		return nil, err
	}

	input, ok := p.Args["input"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid input")
	}

	query := buildGeoSearchQuery(input, userID, userRole)
	res, err := r.searchService.NearbyDrivers(ctx, query)
	if err != nil {
		return nil, err
	}

	return mapSearchResult(res), nil
}

func (r *Resolver) ResolveStartReindex(p gql.ResolveParams) (interface{}, error) {
	ctx := p.Context
	_, userRole, err := r.getAuthContext(ctx)
	if err != nil {
		return nil, err
	}

	// Only admin can trigger reindex
	if userRole != string(sharedconstants.RoleAdmin) {
		return nil, errors.New("forbidden: admin access required")
	}

	index := getString(p.Args, "index")
	job, err := r.reindexService.StartReindex(ctx, index)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"jobId":       job.JobID,
		"index":       job.Index,
		"status":      job.Status,
		"startedAt":   job.StartedAt.Format(time.RFC3339),
		"completedAt": formatTimePtr(job.CompletedAt),
		"error":       job.Error,
	}, nil
}

func buildDeliverySearchQuery(input map[string]interface{}, userID, userRole string) search.DeliverySearchQuery {
	q := search.DeliverySearchQuery{
		Query:      getString(input, "query"),
		Status:     getString(input, "status"),
		City:       getString(input, "city"),
		DriverID:   getString(input, "driverId"),
		CustomerID: getString(input, "customerId"),
		Pagination: buildPaginationInput(input),
		UserID:     userID,
		UserRole:   userRole,
	}

	if fromDate := getString(input, "fromDate"); fromDate != "" {
		if t, err := time.Parse(time.RFC3339, fromDate); err == nil {
			q.FromDate = &t
		}
	}
	if toDate := getString(input, "toDate"); toDate != "" {
		if t, err := time.Parse(time.RFC3339, toDate); err == nil {
			q.ToDate = &t
		}
	}

	if geo := getMap(input, "geo"); geo != nil {
		q.Geo = &search.GeoDistanceFilter{
			Lat:      getFloat(geo, "lat"),
			Lon:      getFloat(geo, "lon"),
			RadiusKm: getFloat(geo, "radiusKm"),
		}
	}

	if sorts := getSlice(input, "sort"); sorts != nil {
		q.Sort = make([]search.DeliverySearchSort, 0, len(sorts))
		for _, s := range sorts {
			if sm, ok := s.(map[string]interface{}); ok {
				q.Sort = append(q.Sort, search.DeliverySearchSort{
					Field: getString(sm, "field"),
					Order: search.SortOrder(getString(sm, "order")),
				})
			}
		}
	}

	return q
}

func buildDriverSearchQuery(input map[string]interface{}, userID, userRole string) search.DriverSearchQuery {
	q := search.DriverSearchQuery{
		Query:       getString(input, "query"),
		Status:      getString(input, "status"),
		VehicleType: getString(input, "vehicleType"),
		MinRating:   getFloat(input, "minRating"),
		Pagination:  buildPaginationInput(input),
		UserID:      userID,
		UserRole:    userRole,
	}

	if geo := getMap(input, "geo"); geo != nil {
		q.Geo = &search.GeoDistanceFilter{
			Lat:      getFloat(geo, "lat"),
			Lon:      getFloat(geo, "lon"),
			RadiusKm: getFloat(geo, "radiusKm"),
		}
	}

	if sorts := getSlice(input, "sort"); sorts != nil {
		q.Sort = make([]search.DriverSearchSort, 0, len(sorts))
		for _, s := range sorts {
			if sm, ok := s.(map[string]interface{}); ok {
				q.Sort = append(q.Sort, search.DriverSearchSort{
					Field: getString(sm, "field"),
					Order: search.SortOrder(getString(sm, "order")),
				})
			}
		}
	}

	return q
}

func buildMediaSearchQuery(input map[string]interface{}, userID, userRole string) search.MediaSearchQuery {
	return search.MediaSearchQuery{
		Query:      getString(input, "query"),
		MediaType:  getString(input, "mediaType"),
		MimeType:   getString(input, "mimeType"),
		OwnerID:    getString(input, "ownerId"),
		Pagination: buildPaginationInput(input),
		UserID:     userID,
		UserRole:   userRole,
	}
}

func buildUserSearchQuery(input map[string]interface{}) search.UserSearchQuery {
	var isActive *bool
	if v, ok := input["isActive"].(bool); ok {
		isActive = &v
	}

	return search.UserSearchQuery{
		Query:      getString(input, "query"),
		Role:       getString(input, "role"),
		IsActive:   isActive,
		Pagination: buildPaginationInput(input),
	}
}

func buildGeoSearchQuery(input map[string]interface{}, userID, userRole string) search.GeoSearchQuery {
	return search.GeoSearchQuery{
		Lat:        getFloat(input, "lat"),
		Lon:        getFloat(input, "lon"),
		RadiusKm:   getFloat(input, "radiusKm"),
		Pagination: buildPaginationInput(input),
		UserID:     userID,
		UserRole:   userRole,
	}
}

func buildPaginationInput(input map[string]interface{}) search.PaginationInput {
	if pagination := getMap(input, "pagination"); pagination != nil {
		return search.PaginationInput{
			Limit:  getInt(pagination, "limit"),
			Cursor: getString(pagination, "cursor"),
		}
	}
	return search.PaginationInput{}
}

func mapSearchResult[T any](res search.SearchResult[T]) map[string]interface{} {
	items := make([]interface{}, 0, len(res.Items))
	for _, item := range res.Items {
		items = append(items, mapToInterface(item))
	}

	return map[string]interface{}{
		"items": items,
		"pageInfo": map[string]interface{}{
			"hasNextPage": res.PageInfo.HasNextPage,
			"cursor":      res.PageInfo.Cursor,
			"total":       res.PageInfo.Total,
		},
	}
}

func mapToInterface(v interface{}) map[string]interface{} {
	// Use JSON marshaling for automatic conversion
	// This is a simple approach; for production consider a proper mapper
	switch doc := v.(type) {
	case search.DeliveryDocument:
		return map[string]interface{}{
			"delivery_id":    doc.DeliveryID,
			"customer_id":    doc.CustomerID,
			"driver_id":      doc.DriverID,
			"status":         doc.Status,
			"pickup":         mapGeoAddress(doc.Pickup),
			"dropoff":        mapGeoAddress(doc.Dropoff),
			"created_at":     doc.CreatedAt.Format(time.RFC3339),
			"updated_at":     doc.UpdatedAt.Format(time.RFC3339),
			"source_version": doc.SourceVersion,
		}
	case search.DriverDocument:
		return map[string]interface{}{
			"driver_id":      doc.DriverID,
			"name":           doc.Name,
			"status":         doc.Status,
			"vehicle_type":   doc.VehicleType,
			"rating":         doc.Rating,
			"location":       mapGeoPoint(doc.Location),
			"updated_at":     doc.UpdatedAt.Format(time.RFC3339),
			"source_version": doc.SourceVersion,
		}
	case search.MediaDocument:
		return map[string]interface{}{
			"media_id":   doc.MediaID,
			"owner_id":   doc.OwnerID,
			"file_name":  doc.FileName,
			"mime_type":  doc.MimeType,
			"media_type": doc.MediaType,
			"status":     doc.Status,
			"size":       doc.Size,
			"created_at": doc.CreatedAt.Format(time.RFC3339),
		}
	case search.UserDocument:
		return map[string]interface{}{
			"id":         doc.ID,
			"first_name": doc.FirstName,
			"last_name":  doc.LastName,
			"email":      doc.Email,
			"role":       doc.Role,
			"is_active":  doc.IsActive,
			"created_at": doc.CreatedAt.Format(time.RFC3339),
		}
	}
	return map[string]interface{}{}
}

func mapGeoAddress(addr search.GeoAddress) map[string]interface{} {
	// addr.Location is a value, need to pass pointer
	loc := addr.Location
	return map[string]interface{}{
		"city":     addr.City,
		"country":  addr.Country,
		"location": mapGeoPoint(&loc),
	}
}

func mapGeoPoint(pt *search.GeoPoint) map[string]interface{} {
	if pt == nil {
		return nil
	}
	return map[string]interface{}{
		"lat": pt.Lat,
		"lon": pt.Lon,
	}
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func getFloat(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func getSlice(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key].([]interface{}); ok {
		return v
	}
	return nil
}