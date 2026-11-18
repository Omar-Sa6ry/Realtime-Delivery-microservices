package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// RecordedRequest captures one HTTP call received by FakeOpenSearch.
type RecordedRequest struct {
	Method string
	Path   string
	Body   []byte
}

// FakeOpenSearch is a programmable httptest-backed OpenSearch stand-in.
type FakeOpenSearch struct {
	*httptest.Server

	mu       sync.Mutex
	requests []RecordedRequest
	// handler is consulted first; return true when the request was handled.
	handler func(w http.ResponseWriter, r *http.Request, body []byte) bool
}

// NewFakeOpenSearch starts a fake OpenSearch server. handler may be nil for a
// default "everything OK" behaviour (200 + empty JSON object).
func NewFakeOpenSearch(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body []byte) bool) *FakeOpenSearch {
	t.Helper()
	f := &FakeOpenSearch{handler: handler}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, RecordedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Body:   body,
		})
		h := f.handler
		f.mu.Unlock()

		if h != nil && h(w, r, body) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(f.Close)
	return f
}

// SetHandler replaces the programmable handler.
func (f *FakeOpenSearch) SetHandler(h func(w http.ResponseWriter, r *http.Request, body []byte) bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handler = h
}

// Requests returns a snapshot of recorded requests.
func (f *FakeOpenSearch) Requests() []RecordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]RecordedRequest, len(f.requests))
	copy(out, f.requests)
	return out
}

// LastRequest returns the most recent request (ok=false when none).
func (f *FakeOpenSearch) LastRequest() (RecordedRequest, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) == 0 {
		return RecordedRequest{}, false
	}
	return f.requests[len(f.requests)-1], true
}

// Reset clears recorded requests.
func (f *FakeOpenSearch) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = nil
}

// WriteJSON writes a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteSearchResponse writes an OpenSearch-style search response.
func WriteSearchResponse(w http.ResponseWriter, status int, total int64, hits ...map[string]interface{}) {
	type hitWrap struct {
		Source map[string]interface{} `json:"_source"`
		Sort   []interface{}          `json:"sort,omitempty"`
	}
	wrapped := make([]hitWrap, 0, len(hits))
	for _, h := range hits {
		src, _ := h["_source"].(map[string]interface{})
		sort, _ := h["sort"].([]interface{})
		if src == nil {
			src = h
		}
		wrapped = append(wrapped, hitWrap{Source: src, Sort: sort})
	}
	WriteJSON(w, status, map[string]interface{}{
		"hits": map[string]interface{}{
			"total": map[string]interface{}{"value": total},
			"hits":  wrapped,
		},
	})
}

// PathContains reports whether the request path contains the fragment.
func PathContains(r *http.Request, fragment string) bool {
	return strings.Contains(r.URL.Path, fragment)
}
