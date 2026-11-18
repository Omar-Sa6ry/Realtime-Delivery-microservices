package opensearch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
)

func newTestIndexManager(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body []byte) bool) (*IndexManager, *testutil.FakeOpenSearch) {
	t.Helper()
	srv := testutil.NewFakeOpenSearch(t, handler)
	client, err := NewClient(testConfig(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewIndexManager(client), srv
}

func TestEnsureIndices_AllCreated(t *testing.T) {
	var created, aliased []string
	im, _ := newTestIndexManager(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		switch {
		case r.Method == http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
			return true
		case r.Method == http.MethodPut && strings.Count(r.URL.Path, "/") == 1:
			created = append(created, r.URL.Path)
			testutil.WriteJSON(w, http.StatusOK, map[string]interface{}{"acknowledged": true})
			return true
		case r.Method == http.MethodPost && r.URL.Path == "/_aliases":
			b, _ := io.ReadAll(r.Body)
			aliased = append(aliased, string(b))
			testutil.WriteJSON(w, http.StatusOK, map[string]interface{}{"acknowledged": true})
			return true
		}
		return false
	})

	if err := im.EnsureIndices(context.Background()); err != nil {
		t.Fatalf("EnsureIndices: %v", err)
	}
	if len(created) != 4 {
		t.Fatalf("created %d indices, want 4: %v", len(created), created)
	}
	if len(aliased) != 4 {
		t.Fatalf("aliased %d times, want 4", len(aliased))
	}
}

func TestEnsureIndices_ExistingIndices(t *testing.T) {
	var aliasCalls int
	im, _ := newTestIndexManager(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return true
		}
		if r.Method == http.MethodPost && r.URL.Path == "/_aliases" {
			aliasCalls++
			testutil.WriteJSON(w, http.StatusOK, map[string]interface{}{"acknowledged": true})
			return true
		}
		return false
	})
	if err := im.EnsureIndices(context.Background()); err != nil {
		t.Fatalf("EnsureIndices: %v", err)
	}
	if aliasCalls != 4 {
		t.Errorf("alias calls = %d, want 4", aliasCalls)
	}
}

func TestEnsureIndices_CreateError(t *testing.T) {
	im, _ := newTestIndexManager(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		if r.Method == http.MethodPut {
			testutil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "boom"})
			return true
		}
		return false
	})
	err := im.EnsureIndices(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ensure indices completed with 4 error(s)") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnsureIndices_ExistsTransportPathAndAliasError(t *testing.T) {
	// Alias returns 400 for already-existing index path
	im, _ := newTestIndexManager(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return true
		}
		if r.Method == http.MethodPost && r.URL.Path == "/_aliases" {
			testutil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "alias boom"})
			return true
		}
		return false
	})
	err := im.EnsureIndices(context.Background())
	if err == nil || !strings.Contains(err.Error(), "4 error(s)") {
		t.Fatalf("expected aggregated alias errors, got %v", err)
	}
}

func TestEnsureIndices_CreateThenAliasError(t *testing.T) {
	im, _ := newTestIndexManager(t, func(w http.ResponseWriter, r *http.Request, _ []byte) bool {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		if r.Method == http.MethodPut {
			testutil.WriteJSON(w, http.StatusOK, map[string]interface{}{"acknowledged": true})
			return true
		}
		if r.Method == http.MethodPost && r.URL.Path == "/_aliases" {
			testutil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "alias fail"})
			return true
		}
		return false
	})
	err := im.EnsureIndices(context.Background())
	if err == nil || !strings.Contains(err.Error(), "4 error(s)") {
		t.Fatalf("expected aggregated errors, got %v", err)
	}
}

// hangingServer accepts one connection, replies to the first request with
// firstStatus, then goes silent (never answers subsequent requests).
func hangingServer(t *testing.T, firstStatus int) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		br := bufio.NewReader(conn)
		if _, err := http.ReadRequest(br); err != nil {
			return
		}
		_, _ = fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Length: 2\r\n\r\n{}", firstStatus, http.StatusText(firstStatus))
		// Keep the connection open but never answer the next request.
		_, _ = br.ReadByte()
	}()
	return ln.Addr().String()
}

func newTestIndexManagerAddr(t *testing.T, addr string) *IndexManager {
	t.Helper()
	client, err := NewClient(testConfig("http://" + addr))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewIndexManager(client)
}

func TestCreateIndexIfNotExists_ExistsRequestTransportError(t *testing.T) {
	// Connection refused: no listener at all.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	im := newTestIndexManagerAddr(t, addr)
	err = im.createIndexIfNotExists(context.Background(), "deliveries-v1", "deliveries", "{}")
	if err == nil {
		t.Fatal("expected transport error from exists check")
	}
}

func TestCreateIndexIfNotExists_CreateRequestHangDeadline(t *testing.T) {
	// First (HEAD) answers 404 → create path; the create request hangs →
	// short context deadline turns it into a Do error.
	addr := hangingServer(t, http.StatusNotFound)
	im := newTestIndexManagerAddr(t, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := im.createIndexIfNotExists(ctx, "deliveries-v1", "deliveries", "{}"); err == nil {
		t.Fatal("expected deadline error from create request")
	} else if !strings.Contains(err.Error(), "context deadline") && !strings.Contains(err.Error(), "Client.Timeout") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnsureAlias_TransportError(t *testing.T) {
	// Connection refused on the alias request.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	im := newTestIndexManagerAddr(t, addr)
	err = im.ensureAlias(context.Background(), "deliveries-v1", "deliveries")
	if err == nil {
		t.Fatal("expected transport error from alias request")
	}
}
