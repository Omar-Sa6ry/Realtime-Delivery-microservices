package opensearch

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	opensearch "github.com/opensearch-project/opensearch-go/v4"
)

type Client struct {
	*opensearch.Client
}

func NewClient(cfg *config.Config) (*Client, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.OpenSearchInsecure,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: cfg.OpenSearchAddresses,
		Username:  cfg.OpenSearchUsername,
		Password:  cfg.OpenSearchPassword,
		Transport: transport,
	})
	if err != nil {
		return nil, err
	}

	return &Client{Client: client}, nil
}
