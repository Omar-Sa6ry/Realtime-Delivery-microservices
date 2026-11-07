package graphql_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/i18n"
)

func TestExtractLanguage(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		rawURL  string
		want    string
	}{
		{name: "x-lang header ar", headers: map[string]string{"x-lang": "ar"}, want: "ar"},
		{name: "x-lang header normalized", headers: map[string]string{"x-lang": "AR-sa"}, want: "ar"},
		{name: "unsupported x-lang falls back to en", headers: map[string]string{"x-lang": "fr"}, want: "en"},
		{name: "empty x-lang falls through to query param", headers: map[string]string{"x-lang": ""}, rawURL: "/graphql?lang=ar", want: "ar"},
		{name: "query param lang", rawURL: "/graphql?lang=ar", want: "ar"},
		{name: "query param ignored when x-lang present", headers: map[string]string{"x-lang": "ar"}, rawURL: "/graphql?lang=fr", want: "ar"},
		{name: "empty query param falls through to accept-language", headers: map[string]string{"Accept-Language": "ar"}, rawURL: "/graphql?lang=", want: "ar"},
		{name: "accept-language header", headers: map[string]string{"Accept-Language": "ar-EG"}, want: "ar"},
		{name: "unsupported accept-language", headers: map[string]string{"Accept-Language": "de"}, want: "en"},
		{name: "no language anywhere defaults to en", rawURL: "/graphql", want: "en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.rawURL
			if target == "" {
				target = "/graphql"
			}
			req := httptest.NewRequest(http.MethodPost, target, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if got := graphql.ExtractLanguage(req); got != tt.want {
				t.Errorf("ExtractLanguage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractUserID(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{name: "present", headers: map[string]string{"x-user-id": "user-9"}, want: "user-9"},
		{name: "missing", headers: nil, want: ""},
		{name: "empty value", headers: map[string]string{"x-user-id": ""}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if got := graphql.ExtractUserID(req); got != tt.want {
				t.Errorf("ExtractUserID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractUserRole(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{name: "present", headers: map[string]string{"x-user-role": "admin"}, want: "admin"},
		{name: "missing", headers: nil, want: ""},
		{name: "empty value", headers: map[string]string{"x-user-role": ""}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if got := graphql.ExtractUserRole(req); got != tt.want {
				t.Errorf("ExtractUserRole() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		wantMessage string
	}{
		{name: "default english", headers: nil, wantMessage: i18n.T("en", "server.healthy")},
		{name: "arabic via x-lang", headers: map[string]string{"x-lang": "ar"}, wantMessage: i18n.T("ar", "server.healthy")},
		{name: "arabic via accept-language", headers: map[string]string{"Accept-Language": "ar"}, wantMessage: i18n.T("ar", "server.healthy")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			graphql.HealthHandler(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
			}
			var body map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response is not JSON: %v (body: %s)", err, rec.Body.String())
			}
			if body["status"] != "ok" {
				t.Errorf("status field = %v, want \"ok\"", body["status"])
			}
			if body["service"] != "driver-service" {
				t.Errorf("service field = %v, want driver-service", body["service"])
			}
			if body["message"] != tt.wantMessage {
				t.Errorf("message = %v, want %q", body["message"], tt.wantMessage)
			}
		})
	}
}

func TestServeHTTP_GET(t *testing.T) {
	e := newEnv(t)
	res := e.request(http.MethodGet, "/graphql", "", nil)

	if res.status != http.StatusOK {
		t.Errorf("status = %d, want %d", res.status, http.StatusOK)
	}
	if res.json["status"] != "healthy" {
		t.Errorf("status = %v, want healthy", res.json["status"])
	}
	if res.json["service"] != "driver-subgraph" {
		t.Errorf("service = %v, want driver-subgraph", res.json["service"])
	}
}

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		headers     map[string]string
		wantMessage string
	}{
		{name: "put english", method: http.MethodPut, wantMessage: "Method not allowed"},
		{name: "put arabic", method: http.MethodPut, headers: map[string]string{"x-lang": "ar"}, wantMessage: i18n.T("ar", "server.method_not_allowed")},
		{name: "delete english", method: http.MethodDelete, wantMessage: "Method not allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			res := e.request(tt.method, "/graphql", "", tt.headers)
			if res.status != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", res.status, http.StatusMethodNotAllowed)
			}
			var body map[string]interface{}
			if err := json.Unmarshal([]byte(res.body), &body); err != nil {
				t.Fatalf("response is not JSON: %v", err)
			}
			if body["error"] != tt.wantMessage {
				t.Errorf("error = %v, want %q", body["error"], tt.wantMessage)
			}
		})
	}
}

func TestServeHTTP_MalformedJSON(t *testing.T) {
	e := newEnv(t)
	res := e.request(http.MethodPost, "/graphql", "this is not json", nil)

	if res.status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", res.status, http.StatusBadRequest)
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(res.body), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	errs, ok := body["errors"].([]interface{})
	if !ok || len(errs) != 1 {
		t.Fatalf("errors = %v, want exactly one entry", body["errors"])
	}
	first, ok := errs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("errors[0] = %v, want object", errs[0])
	}
	if first["message"] != "Invalid JSON body" {
		t.Errorf("errors[0].message = %v, want \"Invalid JSON body\"", first["message"])
	}
}

// TestServeHTTP_LoadersBranch covers both sides of `if h.loaders != nil`.
func TestServeHTTP_LoadersBranch(t *testing.T) {
	t.Run("with loaders in context", func(t *testing.T) {
		e := newEnv(t)
		res := e.gql(`{ _service { sdl } }`, nil, nil)
		if res.status != http.StatusOK {
			t.Errorf("status = %d, want %d", res.status, http.StatusOK)
		}
		wantNoErrors(t, res)
		wantStr(t, res.json, graphql.DriverSubgraphSDL, "data", "_service", "sdl")
	})

	t.Run("without loaders", func(t *testing.T) {
		e := newEnv(t, withoutLoaders())
		res := e.gql(`{ _service { sdl } }`, nil, nil)
		if res.status != http.StatusOK {
			t.Errorf("status = %d, want %d", res.status, http.StatusOK)
		}
		wantNoErrors(t, res)
		wantStr(t, res.json, graphql.DriverSubgraphSDL, "data", "_service", "sdl")
	})
}

// TestServeHTTP_ContextHeaders checks that identity headers reach the resolvers.
func TestServeHTTP_ContextHeaders(t *testing.T) {
	e := newEnv(t)
	e.mockDrivers.SetDriver(newRichDriver("drv-1", "user-1"))

	// Without x-user-id the profile query fails authorization.
	res := e.gql(`{ myDriverProfile { success statusCode } }`, nil, nil)
	wantNum(t, res.json, 401, "data", "myDriverProfile", "statusCode")

	// With x-user-id the same query resolves through the context value.
	res = e.gql(`{ myDriverProfile { success statusCode data { id } } }`, nil,
		map[string]string{"x-user-id": "user-1"})
	wantNum(t, res.json, 200, "data", "myDriverProfile", "statusCode")
	wantStr(t, res.json, "drv-1", "data", "myDriverProfile", "data", "id")
}
