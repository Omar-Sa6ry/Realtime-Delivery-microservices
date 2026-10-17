package search

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)


type PaginationInput struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	Cursor      string `json:"cursor,omitempty"`
	Total       int64  `json:"total"`
}

type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type GeoAddress struct {
	City     string   `json:"city"`
	Country  string   `json:"country"`
	Location GeoPoint `json:"location"`
}

type DeliveryDocument struct {
	DeliveryID    string     `json:"delivery_id"`
	CustomerID    string     `json:"customer_id"`
	DriverID      string     `json:"driver_id,omitempty"`
	Status        string     `json:"status"`
	Pickup        GeoAddress `json:"pickup"`
	Dropoff       GeoAddress `json:"dropoff"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	IndexedAt     time.Time  `json:"indexed_at"`
	SourceVersion int64      `json:"source_version"`
}

type DriverDocument struct {
	DriverID      string    `json:"driver_id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`       // AVAILABLE | BUSY | OFFLINE
	VehicleType   string    `json:"vehicle_type"` // CAR | MOTORCYCLE | TRUCK | BICYCLE
	Location      *GeoPoint `json:"location,omitempty"`
	Rating        float64   `json:"rating"`
	UpdatedAt     time.Time `json:"updated_at"`
	IndexedAt     time.Time `json:"indexed_at"`
	SourceVersion int64     `json:"source_version"`
}

type MediaDocument struct {
	MediaID       string    `json:"media_id"`
	OwnerID       string    `json:"owner_id"`
	FileName      string    `json:"file_name"`
	MimeType      string    `json:"mime_type"`
	MediaType     string    `json:"media_type"` // IMAGE | VIDEO | DOCUMENT | ...
	Status        string    `json:"status"`     // READY | ...
	Size          int64     `json:"size"`
	CreatedAt     time.Time `json:"created_at"`
	IndexedAt     time.Time `json:"indexed_at"`
	SourceVersion int64     `json:"source_version"`
}

type UserDocument struct {
	ID        string    `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type DeliverySearchSort struct {
	Field string    `json:"field"` // created_at | updated_at | status
	Order SortOrder `json:"order"`
}

type DeliverySearchQuery struct {
	Query      string               `json:"query,omitempty"`
	Status     string               `json:"status,omitempty"`
	City       string               `json:"city,omitempty"`
	DriverID   string               `json:"driverId,omitempty"`
	CustomerID string               `json:"customerId,omitempty"` // enforced for non-admin
	FromDate   *time.Time           `json:"fromDate,omitempty"`
	ToDate     *time.Time           `json:"toDate,omitempty"`
	Geo        *GeoDistanceFilter   `json:"geo,omitempty"`
	Sort       []DeliverySearchSort `json:"sort,omitempty"`
	Pagination PaginationInput      `json:"pagination"`

	// Authorization context — always set by application layer, never by client.
	UserID   string `json:"-"`
	UserRole string `json:"-"`
}

type DriverSearchSort struct {
	Field string    `json:"field"` // rating | updated_at | distance
	Order SortOrder `json:"order"`
}

type DriverSearchQuery struct {
	Query       string             `json:"query,omitempty"`
	Status      string             `json:"status,omitempty"`
	VehicleType string             `json:"vehicleType,omitempty"`
	MinRating   float64            `json:"minRating,omitempty"`
	Geo         *GeoDistanceFilter `json:"geo,omitempty"`
	Sort        []DriverSearchSort `json:"sort,omitempty"`
	Pagination  PaginationInput    `json:"pagination"`

	// Authorization context.
	UserID   string `json:"-"`
	UserRole string `json:"-"`
}

type MediaSearchQuery struct {
	Query      string          `json:"query,omitempty"`
	MediaType  string          `json:"mediaType,omitempty"`
	MimeType   string          `json:"mimeType,omitempty"`
	OwnerID    string          `json:"ownerId,omitempty"` // enforced for non-admin
	Pagination PaginationInput `json:"pagination"`

	// Authorization context.
	UserID   string `json:"-"`
	UserRole string `json:"-"`
}

type GeoDistanceFilter struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	RadiusKm float64 `json:"radiusKm"`
}

type GeoSearchQuery struct {
	Lat        float64         `json:"lat"`
	Lon        float64         `json:"lon"`
	RadiusKm   float64         `json:"radiusKm"`
	Pagination PaginationInput `json:"pagination"`
	UserID     string          `json:"-"`
	UserRole   string          `json:"-"`
}

type AutocompleteQuery struct {
	Prefix string `json:"prefix"`
	Index  string `json:"index"` // deliveries | drivers | media
	Limit  int    `json:"limit"`
}

type UserSearchQuery struct {
	Query      string          `json:"query,omitempty"`
	Role       string          `json:"role,omitempty"`
	IsActive   *bool           `json:"isActive,omitempty"`
	Pagination PaginationInput `json:"pagination"`
}

type SearchResult[T any] struct {
	Items    []T      `json:"items"`
	PageInfo PageInfo `json:"pageInfo"`
}

type AutocompleteResult struct {
	Suggestions []string `json:"suggestions"`
}

//  Cursor Encoding/Decoding ─

// searchCursorPayload holds the OpenSearch sort values for search_after pagination.
type searchCursorPayload struct {
	SortValues []interface{} `json:"sv"`
}

func EncodeCursor(sortValues []interface{}) (string, error) {
	cp := searchCursorPayload{SortValues: sortValues}
	data, err := json.Marshal(cp)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func DecodeCursor(cursor string) ([]interface{}, error) {
	if cursor == "" {
		return nil, nil
	}
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("decode cursor (invalid base64): %w", err)
	}
	var cp searchCursorPayload
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("decode cursor (invalid json): %w", err)
	}
	return cp.SortValues, nil
}
