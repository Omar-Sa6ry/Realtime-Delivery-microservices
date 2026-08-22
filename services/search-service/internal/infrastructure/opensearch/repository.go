package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

type Repository struct {
	client *Client
}

func NewRepository(client *Client) *Repository {
	return &Repository{client: client}
}

//  Query Operations 

func (r *Repository) SearchDeliveries(ctx context.Context, q search.DeliverySearchQuery) (search.SearchResult[search.DeliveryDocument], error) {
	queryDsl := buildDeliveryQuery(q)
	return executeSearch[search.DeliveryDocument](ctx, r.client, "deliveries", queryDsl, q.Pagination)
}

func (r *Repository) SearchDrivers(ctx context.Context, q search.DriverSearchQuery) (search.SearchResult[search.DriverDocument], error) {
	queryDsl := buildDriverQuery(q)
	return executeSearch[search.DriverDocument](ctx, r.client, "drivers", queryDsl, q.Pagination)
}

func (r *Repository) SearchMedia(ctx context.Context, q search.MediaSearchQuery) (search.SearchResult[search.MediaDocument], error) {
	queryDsl := buildMediaQuery(q)
	return executeSearch[search.MediaDocument](ctx, r.client, "media", queryDsl, q.Pagination)
}

func (r *Repository) Autocomplete(ctx context.Context, q search.AutocompleteQuery) (search.AutocompleteResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}
	
	queryDsl := map[string]interface{}{
		"size": limit,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query": q.Prefix,
				"type":  "bool_prefix",
				"fields": []string{
					"pickup.city", "pickup.city._2gram", "pickup.city._3gram",
					"dropoff.city", "dropoff.city._2gram", "dropoff.city._3gram",
					"name", "name._2gram", "name._3gram",
					"file_name", "file_name._2gram", "file_name._3gram",
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(queryDsl)
	if err != nil {
		return search.AutocompleteResult{}, err
	}

	req := opensearchapi.SearchReq{
		Indices: []string{q.Index},
		Body:    bytes.NewReader(bodyBytes),
	}

	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return search.AutocompleteResult{}, fmt.Errorf("%w: %v", search.ErrSearchUnavailable, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return search.AutocompleteResult{}, fmt.Errorf("opensearch returned status: %d", res.StatusCode)
	}

	var rawRes struct {
		Hits struct {
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&rawRes); err != nil {
		return search.AutocompleteResult{}, err
	}

	suggestions := make([]string, 0, len(rawRes.Hits.Hits))
	for _, hit := range rawRes.Hits.Hits {
		var genericDoc map[string]interface{}
		if err := json.Unmarshal(hit.Source, &genericDoc); err == nil {
			if name, ok := genericDoc["name"].(string); ok && name != "" {
				suggestions = append(suggestions, name)
			} else if pickup, ok := genericDoc["pickup"].(map[string]interface{}); ok {
				if city, ok := pickup["city"].(string); ok && city != "" {
					suggestions = append(suggestions, city)
				}
			} else if fileName, ok := genericDoc["file_name"].(string); ok && fileName != "" {
				suggestions = append(suggestions, fileName)
			}
		}
	}

	return search.AutocompleteResult{Suggestions: suggestions}, nil
}

func (r *Repository) NearbyDeliveries(ctx context.Context, q search.GeoSearchQuery) (search.SearchResult[search.DeliveryDocument], error) {
	queryDsl := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{
						"geo_distance": map[string]interface{}{
							"distance": fmt.Sprintf("%.2fkm", q.RadiusKm),
							"pickup.location": map[string]interface{}{
								"lat": q.Lat,
								"lon": q.Lon,
							},
						},
					},
				},
			},
		},
		"sort": []map[string]interface{}{
			{
				"_geo_distance": map[string]interface{}{
					"pickup.location": map[string]interface{}{
						"lat": q.Lat,
						"lon": q.Lon,
					},
					"order":         "asc",
					"unit":          "km",
					"mode":          "min",
					"distance_type": "arc",
				},
			},
		},
	}
	return executeSearch[search.DeliveryDocument](ctx, r.client, "deliveries", queryDsl, q.Pagination)
}

func (r *Repository) NearbyDrivers(ctx context.Context, q search.GeoSearchQuery) (search.SearchResult[search.DriverDocument], error) {
	queryDsl := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"status": "AVAILABLE",
						},
					},
					{
						"geo_distance": map[string]interface{}{
							"distance": fmt.Sprintf("%.2fkm", q.RadiusKm),
							"location": map[string]interface{}{
								"lat": q.Lat,
								"lon": q.Lon,
							},
						},
					},
				},
			},
		},
		"sort": []map[string]interface{}{
			{
				"_geo_distance": map[string]interface{}{
					"location": map[string]interface{}{
						"lat": q.Lat,
						"lon": q.Lon,
					},
					"order":         "asc",
					"unit":          "km",
					"mode":          "min",
					"distance_type": "arc",
				},
			},
		},
	}
	return executeSearch[search.DriverDocument](ctx, r.client, "drivers", queryDsl, q.Pagination)
}

//  Indexing Operations 

func (r *Repository) UpsertDelivery(ctx context.Context, doc search.DeliveryDocument) error {
	doc.IndexedAt = time.Now().UTC()
	return r.upsertDoc(ctx, "deliveries", doc.DeliveryID, doc, doc.SourceVersion)
}

func (r *Repository) DeleteDelivery(ctx context.Context, id string) error {
	return r.deleteDoc(ctx, "deliveries", id)
}

func (r *Repository) UpsertDriver(ctx context.Context, doc search.DriverDocument) error {
	doc.IndexedAt = time.Now().UTC()
	return r.upsertDoc(ctx, "drivers", doc.DriverID, doc, doc.SourceVersion)
}

func (r *Repository) DeleteDriver(ctx context.Context, id string) error {
	return r.deleteDoc(ctx, "drivers", id)
}

func (r *Repository) UpsertMedia(ctx context.Context, doc search.MediaDocument) error {
	doc.IndexedAt = time.Now().UTC()
	return r.upsertDoc(ctx, "media", doc.MediaID, doc, doc.SourceVersion)
}

func (r *Repository) DeleteMedia(ctx context.Context, id string) error {
	return r.deleteDoc(ctx, "media", id)
}

func (r *Repository) BulkUpsertDeliveries(ctx context.Context, docs []search.DeliveryDocument) error {
	var buf bytes.Buffer
	for _, doc := range docs {
		doc.IndexedAt = time.Now().UTC()
		meta := fmt.Sprintf(`{"index":{"_index":"deliveries","_id":"%s"}}`+"\n", doc.DeliveryID)
		docBytes, err := json.Marshal(doc)
		if err != nil {
			continue
		}
		buf.WriteString(meta)
		buf.Write(docBytes)
		buf.WriteString("\n")
	}
	return r.executeBulk(ctx, &buf)
}

func (r *Repository) BulkUpsertDrivers(ctx context.Context, docs []search.DriverDocument) error {
	var buf bytes.Buffer
	for _, doc := range docs {
		doc.IndexedAt = time.Now().UTC()
		meta := fmt.Sprintf(`{"index":{"_index":"drivers","_id":"%s"}}`+"\n", doc.DriverID)
		docBytes, err := json.Marshal(doc)
		if err != nil {
			continue
		}
		buf.WriteString(meta)
		buf.Write(docBytes)
		buf.WriteString("\n")
	}
	return r.executeBulk(ctx, &buf)
}

func (r *Repository) BulkUpsertMedia(ctx context.Context, docs []search.MediaDocument) error {
	var buf bytes.Buffer
	for _, doc := range docs {
		doc.IndexedAt = time.Now().UTC()
		meta := fmt.Sprintf(`{"index":{"_index":"media","_id":"%s"}}`+"\n", doc.MediaID)
		docBytes, err := json.Marshal(doc)
		if err != nil {
			continue
		}
		buf.WriteString(meta)
		buf.Write(docBytes)
		buf.WriteString("\n")
	}
	return r.executeBulk(ctx, &buf)
}

//  Helpers 

func (r *Repository) upsertDoc(ctx context.Context, index, id string, doc interface{}, incomingVersion int64) error {
	// Version check guard
	currentVersion, err := r.GetDocumentVersion(ctx, index, id)
	if err == nil && currentVersion >= incomingVersion && currentVersion != -1 {
		slog.Warn("Skipping stale document indexing", "index", index, "id", id, "currentVersion", currentVersion, "incomingVersion", incomingVersion)
		return nil
	}

	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	req := opensearchapi.IndexReq{
		Index:      index,
		DocumentID: id,
		Body:       bytes.NewReader(data),
	}

	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", search.ErrSearchUnavailable, err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		respBody, _ := io.ReadAll(res.Body)
		return fmt.Errorf("indexing error (%d): %s", res.StatusCode, string(respBody))
	}
	return nil
}

func (r *Repository) deleteDoc(ctx context.Context, index, id string) error {
	req := opensearchapi.DocumentDeleteReq{
		Index:      index,
		DocumentID: id,
	}

	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", search.ErrSearchUnavailable, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete failed with status %d", res.StatusCode)
	}
	return nil
}

func (r *Repository) executeBulk(ctx context.Context, body io.Reader) error {
	req := opensearchapi.BulkReq{
		Body: body,
	}

	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", search.ErrSearchUnavailable, err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("bulk indexing failed (%d): %s", res.StatusCode, string(b))
	}
	return nil
}

func (r *Repository) GetDocumentVersion(ctx context.Context, index, id string) (int64, error) {
	req := opensearchapi.DocumentGetReq{
		Index:      index,
		DocumentID: id,
	}

	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return -1, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return -1, nil
	}

	var hit struct {
		Source struct {
			SourceVersion int64 `json:"source_version"`
		} `json:"_source"`
	}

	if err := json.NewDecoder(res.Body).Decode(&hit); err != nil {
		return -1, err
	}
	return hit.Source.SourceVersion, nil
}

func (r *Repository) IndexExists(ctx context.Context, index string) (bool, error) {
	req := opensearchapi.IndicesExistsReq{
		Indices: []string{index},
	}
	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK, nil
}

func (r *Repository) CountDocuments(ctx context.Context, index string) (int64, error) {
	req := opensearchapi.IndicesCountReq{
		Indices: []string{index},
	}
	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	var result struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Count, nil
}

func (r *Repository) Reindex(ctx context.Context, source, target string) error {
	reindexBody := map[string]interface{}{
		"source": map[string]interface{}{"index": source},
		"dest":   map[string]interface{}{"index": target},
	}
	b, err := json.Marshal(reindexBody)
	if err != nil {
		return err
	}

	req := opensearchapi.ReindexReq{
		Body: bytes.NewReader(b),
	}
	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		resp, _ := io.ReadAll(res.Body)
		return fmt.Errorf("reindex failed: %s", string(resp))
	}
	return nil
}

func (r *Repository) SwitchAlias(ctx context.Context, alias, oldIndex, newIndex string) error {
	actions := []map[string]interface{}{
		{
			"add": map[string]interface{}{
				"index": newIndex,
				"alias": alias,
			},
		},
	}
	if oldIndex != "" {
		actions = append([]map[string]interface{}{
			{
				"remove": map[string]interface{}{
					"index": oldIndex,
					"alias": alias,
				},
			},
		}, actions...)
	}

	body := map[string]interface{}{
		"actions": actions,
	}
	b, _ := json.Marshal(body)

	req := opensearchapi.AliasesReq{
		Body: bytes.NewReader(b),
	}
	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func (r *Repository) Ping(ctx context.Context) error {
	req := opensearchapi.PingReq{}
	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("ping returned %d", res.StatusCode)
	}
	return nil
}

func (r *Repository) ClusterHealth(ctx context.Context) (string, error) {
	req := opensearchapi.ClusterHealthReq{}
	res, err := r.client.Do(ctx, req, nil)
	if err != nil {
		return "red", err
	}
	defer res.Body.Close()

	var health struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&health); err != nil {
		return "unknown", err
	}
	return health.Status, nil
}

func executeSearch[T any](ctx context.Context, client *Client, index string, queryDsl map[string]interface{}, p search.PaginationInput) (search.SearchResult[T], error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 10
	}
	queryDsl["size"] = limit

	if p.Cursor != "" {
		sortValues, err := search.DecodeCursor(p.Cursor)
		if err == nil && len(sortValues) > 0 {
			queryDsl["search_after"] = sortValues
		}
	}

	// Always ensure secondary sort for deterministic cursor pagination
	if _, ok := queryDsl["sort"]; !ok {
		queryDsl["sort"] = []map[string]interface{}{
			{"_score": "desc"},
			{"_id": "asc"},
		}
	}

	b, err := json.Marshal(queryDsl)
	if err != nil {
		return search.SearchResult[T]{}, err
	}

	req := opensearchapi.SearchReq{
		Indices: []string{index},
		Body:    bytes.NewReader(b),
	}

	res, err := client.Do(ctx, req, nil)
	if err != nil {
		return search.SearchResult[T]{}, fmt.Errorf("%w: %v", search.ErrSearchUnavailable, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(res.Body)
		return search.SearchResult[T]{}, fmt.Errorf("search error (%d): %s", res.StatusCode, string(respBody))
	}

	var osResponse struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source json.RawMessage `json:"_source"`
				Sort   []interface{}   `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&osResponse); err != nil {
		return search.SearchResult[T]{}, err
	}

	items := make([]T, 0, len(osResponse.Hits.Hits))
	var lastSort []interface{}

	for _, hit := range osResponse.Hits.Hits {
		var item T
		if err := json.Unmarshal(hit.Source, &item); err == nil {
			items = append(items, item)
			lastSort = hit.Sort
		}
	}

	var nextCursor string
	hasNextPage := len(items) == limit
	if hasNextPage && len(lastSort) > 0 {
		nextCursor, _ = search.EncodeCursor(lastSort)
	}

	return search.SearchResult[T]{
		Items: items,
		PageInfo: search.PageInfo{
			HasNextPage: hasNextPage,
			Cursor:      nextCursor,
			Total:       osResponse.Hits.Total.Value,
		},
	}, nil
}

func buildDeliveryQuery(q search.DeliverySearchQuery) map[string]interface{} {
	must := []map[string]interface{}{}
	filter := []map[string]interface{}{}

	if strings.TrimSpace(q.Query) != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     q.Query,
				"fields":    []string{"delivery_id^3", "pickup.city^2", "dropoff.city^2", "status"},
				"fuzziness": "AUTO",
			},
		})
	}

	if q.Status != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"status": q.Status},
		})
	}

	if q.City != "" {
		filter = append(filter, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"pickup.city.keyword": q.City}},
					{"term": map[string]interface{}{"dropoff.city.keyword": q.City}},
				},
				"minimum_should_match": 1,
			},
		})
	}

	if q.DriverID != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"driver_id": q.DriverID},
		})
	}

	if q.CustomerID != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"customer_id": q.CustomerID},
		})
	}

	if q.FromDate != nil || q.ToDate != nil {
		rangeQuery := map[string]interface{}{}
		if q.FromDate != nil {
			rangeQuery["gte"] = q.FromDate.Format(time.RFC3339)
		}
		if q.ToDate != nil {
			rangeQuery["lte"] = q.ToDate.Format(time.RFC3339)
		}
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{"created_at": rangeQuery},
		})
	}

	if q.Geo != nil {
		filter = append(filter, map[string]interface{}{
			"geo_distance": map[string]interface{}{
				"distance": fmt.Sprintf("%.2fkm", q.Geo.RadiusKm),
				"pickup.location": map[string]interface{}{
					"lat": q.Geo.Lat,
					"lon": q.Geo.Lon,
				},
			},
		})
	}

	boolQuery := map[string]interface{}{}
	if len(must) > 0 {
		boolQuery["must"] = must
	} else {
		boolQuery["must"] = map[string]interface{}{"match_all": map[string]interface{}{}}
	}
	if len(filter) > 0 {
		boolQuery["filter"] = filter
	}

	queryDsl := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQuery,
		},
	}

	if len(q.Sort) > 0 {
		sorts := make([]map[string]interface{}, 0, len(q.Sort)+1)
		for _, s := range q.Sort {
			sorts = append(sorts, map[string]interface{}{
				s.Field: map[string]interface{}{"order": string(s.Order)},
			})
		}
		sorts = append(sorts, map[string]interface{}{"_id": map[string]interface{}{"order": "asc"}})
		queryDsl["sort"] = sorts
	}

	return queryDsl
}

func buildDriverQuery(q search.DriverSearchQuery) map[string]interface{} {
	must := []map[string]interface{}{}
	filter := []map[string]interface{}{}

	if strings.TrimSpace(q.Query) != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     q.Query,
				"fields":    []string{"name^3", "driver_id^2", "vehicle_type"},
				"fuzziness": "AUTO",
			},
		})
	}

	if q.Status != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"status": q.Status},
		})
	}

	if q.VehicleType != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"vehicle_type": q.VehicleType},
		})
	}

	if q.MinRating > 0 {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{
				"rating": map[string]interface{}{"gte": q.MinRating},
			},
		})
	}

	if q.Geo != nil {
		filter = append(filter, map[string]interface{}{
			"geo_distance": map[string]interface{}{
				"distance": fmt.Sprintf("%.2fkm", q.Geo.RadiusKm),
				"location": map[string]interface{}{
					"lat": q.Geo.Lat,
					"lon": q.Geo.Lon,
				},
			},
		})
	}

	boolQuery := map[string]interface{}{}
	if len(must) > 0 {
		boolQuery["must"] = must
	} else {
		boolQuery["must"] = map[string]interface{}{"match_all": map[string]interface{}{}}
	}
	if len(filter) > 0 {
		boolQuery["filter"] = filter
	}

	queryDsl := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQuery,
		},
	}

	if len(q.Sort) > 0 {
		sorts := make([]map[string]interface{}, 0, len(q.Sort)+1)
		for _, s := range q.Sort {
			sorts = append(sorts, map[string]interface{}{
				s.Field: map[string]interface{}{"order": string(s.Order)},
			})
		}
		sorts = append(sorts, map[string]interface{}{"_id": map[string]interface{}{"order": "asc"}})
		queryDsl["sort"] = sorts
	}

	return queryDsl
}

func buildMediaQuery(q search.MediaSearchQuery) map[string]interface{} {
	must := []map[string]interface{}{}
	filter := []map[string]interface{}{}

	if strings.TrimSpace(q.Query) != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     q.Query,
				"fields":    []string{"file_name^3", "media_id^2", "mime_type"},
				"fuzziness": "AUTO",
			},
		})
	}

	if q.MediaType != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"media_type": q.MediaType},
		})
	}

	if q.MimeType != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"mime_type": q.MimeType},
		})
	}

	if q.OwnerID != "" {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"owner_id": q.OwnerID},
		})
	}

	boolQuery := map[string]interface{}{}
	if len(must) > 0 {
		boolQuery["must"] = must
	} else {
		boolQuery["must"] = map[string]interface{}{"match_all": map[string]interface{}{}}
	}
	if len(filter) > 0 {
		boolQuery["filter"] = filter
	}

	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQuery,
		},
	}
}
