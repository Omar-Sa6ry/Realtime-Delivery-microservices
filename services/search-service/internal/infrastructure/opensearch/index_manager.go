package opensearch

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

type IndexManager struct {
	client *Client
}

func NewIndexManager(client *Client) *IndexManager {
	return &IndexManager{client: client}
}

func (im *IndexManager) EnsureIndices(ctx context.Context) error {
	indices := []struct {
		Alias   string
		Version string
		Mapping string
	}{
		{
			Alias:   "deliveries",
			Version: "deliveries-v1",
			Mapping: deliveriesMapping,
		},
		{
			Alias:   "drivers",
			Version: "drivers-v1",
			Mapping: driversMapping,
		},
		{
			Alias:   "media",
			Version: "media-v1",
			Mapping: mediaMapping,
		},
		{
			Alias:   "users",
			Version: "users-v1",
			Mapping: usersMapping,
		},
	}

	for _, idx := range indices {
		if err := im.createIndexIfNotExists(ctx, idx.Version, idx.Alias, idx.Mapping); err != nil {
			return fmt.Errorf("failed to ensure index %s: %w", idx.Alias, err)
		}
	}
	return nil
}

func (im *IndexManager) createIndexIfNotExists(ctx context.Context, concreteName, aliasName, mappingJson string) error {
	existsReq := opensearchapi.IndicesExistsReq{
		Indices: []string{concreteName},
	}
	res, err := im.client.Do(ctx, existsReq, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		slog.Info("Index already exists", "index", concreteName)
		return nil
	}

	createReq := opensearchapi.IndicesCreateReq{
		Index: concreteName,
		Body:  bytes.NewReader([]byte(mappingJson)),
	}
	createRes, err := im.client.Do(ctx, createReq, nil)
	if err != nil {
		return err
	}
	defer createRes.Body.Close()

	if createRes.StatusCode >= 400 {
		return fmt.Errorf("create index status %d", createRes.StatusCode)
	}

	aliasReq := opensearchapi.AliasesReq{
		Body: bytes.NewReader([]byte(fmt.Sprintf(`{"actions":[{"add":{"index":"%s","alias":"%s"}}]}`, concreteName, aliasName))),
	}
	aliasRes, err := im.client.Do(ctx, aliasReq, nil)
	if err != nil {
		return err
	}
	defer aliasRes.Body.Close()

	slog.Info("Successfully created index with alias", "index", concreteName, "alias", aliasName)
	return nil
}

const deliveriesMapping = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "autocomplete_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "edge_ngram_filter"]
        }
      },
      "filter": {
        "edge_ngram_filter": {
          "type": "edge_ngram",
          "min_gram": 2,
          "max_gram": 20
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "delivery_id": { "type": "keyword" },
      "customer_id": { "type": "keyword" },
      "driver_id": { "type": "keyword" },
      "status": { "type": "keyword" },
      "pickup": {
        "properties": {
          "city": {
            "type": "text",
            "analyzer": "autocomplete_analyzer",
            "search_analyzer": "standard",
            "fields": { "keyword": { "type": "keyword" } }
          },
          "country": { "type": "keyword" },
          "location": { "type": "geo_point" }
        }
      },
      "dropoff": {
        "properties": {
          "city": {
            "type": "text",
            "analyzer": "autocomplete_analyzer",
            "search_analyzer": "standard",
            "fields": { "keyword": { "type": "keyword" } }
          },
          "country": { "type": "keyword" },
          "location": { "type": "geo_point" }
        }
      },
      "created_at": { "type": "date" },
      "updated_at": { "type": "date" },
      "indexed_at": { "type": "date" },
      "source_version": { "type": "long" }
    }
  }
}`

const driversMapping = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "autocomplete_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "edge_ngram_filter"]
        }
      },
      "filter": {
        "edge_ngram_filter": {
          "type": "edge_ngram",
          "min_gram": 2,
          "max_gram": 20
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "driver_id": { "type": "keyword" },
      "name": {
        "type": "text",
        "analyzer": "autocomplete_analyzer",
        "search_analyzer": "standard",
        "fields": { "keyword": { "type": "keyword" } }
      },
      "status": { "type": "keyword" },
      "vehicle_type": { "type": "keyword" },
      "rating": { "type": "float" },
      "location": { "type": "geo_point" },
      "updated_at": { "type": "date" },
      "indexed_at": { "type": "date" },
      "source_version": { "type": "long" }
    }
  }
}`

const usersMapping = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "user_autocomplete": { "type": "custom", "tokenizer": "standard", "filter": ["lowercase", "user_edge"] }
      },
      "filter": { "user_edge": { "type": "edge_ngram", "min_gram": 2, "max_gram": 20 } }
    }
  },
  "mappings": { "properties": {
    "id": { "type": "keyword" },
    "first_name": { "type": "text", "analyzer": "user_autocomplete", "search_analyzer": "standard" },
    "last_name": { "type": "text", "analyzer": "user_autocomplete", "search_analyzer": "standard" },
    "email": { "type": "keyword" },
    "role": { "type": "keyword" },
    "is_active": { "type": "boolean" },
    "created_at": { "type": "date" }
  } }
}`

const mediaMapping = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "autocomplete_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "edge_ngram_filter"]
        }
      },
      "filter": {
        "edge_ngram_filter": {
          "type": "edge_ngram",
          "min_gram": 2,
          "max_gram": 20
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "media_id": { "type": "keyword" },
      "owner_id": { "type": "keyword" },
      "file_name": {
        "type": "text",
        "analyzer": "autocomplete_analyzer",
        "search_analyzer": "standard",
        "fields": { "keyword": { "type": "keyword" } }
      },
      "mime_type": { "type": "keyword" },
      "media_type": { "type": "keyword" },
      "status": { "type": "keyword" },
      "size": { "type": "long" },
      "created_at": { "type": "date" },
      "indexed_at": { "type": "date" },
      "source_version": { "type": "long" }
    }
  }
}`
