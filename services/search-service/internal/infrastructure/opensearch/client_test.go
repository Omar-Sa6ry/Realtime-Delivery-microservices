package opensearch

import (
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
)

func testConfig(url string) *config.Config {
	return &config.Config{
		OpenSearchAddresses: []string{url},
		OpenSearchUsername:  "admin",
		OpenSearchPassword:  "admin",
		OpenSearchInsecure:  true,
	}
}

func TestNewClient_Success(t *testing.T) {
	srv := testutil.NewFakeOpenSearch(t, nil)
	client, err := NewClient(testConfig(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client == nil || client.Client == nil {
		t.Fatal("nil client")
	}
}

func TestNewClient_InvalidAddress(t *testing.T) {
	_, err := NewClient(testConfig("://bad-url"))
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

func TestNewRepository(t *testing.T) {
	srv := testutil.NewFakeOpenSearch(t, nil)
	client, err := NewClient(testConfig(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	im := NewIndexManager(client)
	repo := NewRepository(client, im)
	if repo == nil || repo.client != client || repo.indexManager != im {
		t.Fatal("NewRepository wiring broken")
	}
}
