package opensearch

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

	var errs []error
	for _, idx := range indices {
		if err := im.createIndexIfNotExists(ctx, idx.Version, idx.Alias, idx.Mapping); err != nil {
			slog.Warn("Failed to ensure index", "alias", idx.Alias, "error", err)
			errs = append(errs, fmt.Errorf("failed to ensure index %s: %w", idx.Alias, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("ensure indices completed with %d error(s)", len(errs))
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
		return im.ensureAlias(ctx, concreteName, aliasName)
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
		bodyBytes, _ := io.ReadAll(createRes.Body)
		return fmt.Errorf("create index %s status %d: %s", concreteName, createRes.StatusCode, string(bodyBytes))
	}

	if err := im.ensureAlias(ctx, concreteName, aliasName); err != nil {
		return err
	}

	slog.Info("Successfully created index with alias", "index", concreteName, "alias", aliasName)
	return nil
}

func (im *IndexManager) ensureAlias(ctx context.Context, concreteName, aliasName string) error {
	aliasReq := opensearchapi.AliasesReq{
		Body: bytes.NewReader([]byte(fmt.Sprintf(`{"actions":[{"add":{"index":"%s","alias":"%s"}}]}`, concreteName, aliasName))),
	}
	aliasRes, err := im.client.Do(ctx, aliasReq, nil)
	if err != nil {
		return err
	}
	defer aliasRes.Body.Close()

	if aliasRes.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(aliasRes.Body)
		return fmt.Errorf("create alias %s for index %s status %d: %s", aliasName, concreteName, aliasRes.StatusCode, string(bodyBytes))
	}
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
            "analyzer": "standard",
            "fields": {
              "autocomplete": { "type": "text", "analyzer": "autocomplete_analyzer", "search_analyzer": "standard" },
              "keyword": { "type": "keyword" }
            }
          },
          "country": { "type": "keyword" },
          "location": { "type": "geo_point" }
        }
      },
      "dropoff": {
        "properties": {
          "city": {
            "type": "text",
            "analyzer": "standard",
            "fields": {
              "autocomplete": { "type": "text", "analyzer": "autocomplete_analyzer", "search_analyzer": "standard" },
              "keyword": { "type": "keyword" }
            }
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
        "analyzer": "standard",
        "fields": {
          "autocomplete": { "type": "text", "analyzer": "autocomplete_analyzer", "search_analyzer": "standard" },
          "keyword": { "type": "keyword" }
        }
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
    "first_name": {
      "type": "text",
      "analyzer": "standard",
      "fields": {
        "autocomplete": { "type": "text", "analyzer": "user_autocomplete", "search_analyzer": "standard" },
        "keyword": { "type": "keyword" }
      }
    },
    "last_name": {
      "type": "text",
      "analyzer": "standard",
      "fields": {
        "autocomplete": { "type": "text", "analyzer": "user_autocomplete", "search_analyzer": "standard" },
        "keyword": { "type": "keyword" }
      }
    },
    "email": {
      "type": "text",
      "analyzer": "standard",
      "fields": {
        "autocomplete": { "type": "text", "analyzer": "user_autocomplete", "search_analyzer": "standard" },
        "keyword": { "type": "keyword" }
      }
    },
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
        "analyzer": "standard",
        "fields": {
          "autocomplete": { "type": "text", "analyzer": "autocomplete_analyzer", "search_analyzer": "standard" },
          "keyword": { "type": "keyword" }
        }
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
