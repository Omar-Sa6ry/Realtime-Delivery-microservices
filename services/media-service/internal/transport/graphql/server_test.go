package graphql

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/i18n"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

func makeBearerToken(t *testing.T, secret, subject, role string, expiresAt int64) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(map[string]interface{}{
		"sub":  subject,
		"role": role,
		"exp":  expiresAt,
	})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(header + "." + encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return header + "." + encoded + "." + signature
}

func newGraphQLServer(t *testing.T, rateLimit int) (*Server, *fixture) {
	t.Helper()
	f := newFixture(t, rateLimit)
	srv, err := NewServer(f.h, "0")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv, f
}

func TestHandleGraphQLPostValid(t *testing.T) {
	srv, f := newGraphQLServer(t, 10)
	f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "u1")

	body := `{"query":"query($id: String!){ media(mediaId: $id){ success statusCode data { mediaId ownerId } } }","variables":{"id":"m1"}}`
	req := httptest.NewRequest(http.MethodPost, graphqlPath, strings.NewReader(body))
	req.Header.Set("x-user-id", "u1")
	w := httptest.NewRecorder()

	srv.handleGraphQL(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content type = %q", ct)
	}
	var payload struct {
		Data struct {
			Media struct {
				Success    bool `json:"success"`
				StatusCode int  `json:"statusCode"`
				Data       struct {
					MediaID string `json:"mediaId"`
					OwnerID string `json:"ownerId"`
				} `json:"data"`
			} `json:"media"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v (body %s)", err, w.Body.String())
	}
	if !payload.Data.Media.Success || payload.Data.Media.Data.MediaID != "m1" {
		t.Fatalf("unexpected payload: %s", w.Body.String())
	}
}

func TestHandleGraphQLPostBadJSON(t *testing.T) {
	srv, _ := newGraphQLServer(t, 10)
	req := httptest.NewRequest(http.MethodPost, graphqlPath, strings.NewReader("{not-json"))
	w := httptest.NewRecorder()

	srv.handleGraphQL(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid GraphQL request") {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestHandleGraphQLPostBadVariables(t *testing.T) {
	srv, f := newGraphQLServer(t, 10)
	f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "u1")

	// A variable that cannot be coerced to String! yields a GraphQL error.
	body := `{"query":"query($id: String!){ media(mediaId: $id){ success } }","variables":{"id":123}}`
	req := httptest.NewRequest(http.MethodPost, graphqlPath, strings.NewReader(body))
	req.Header.Set("x-user-id", "u1")
	w := httptest.NewRecorder()

	srv.handleGraphQL(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var payload struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Errors) == 0 {
		t.Fatalf("expected coercion errors, body = %s", w.Body.String())
	}
}

func TestHandleGraphQLGet(t *testing.T) {
	srv, _ := newGraphQLServer(t, 10)
	query := url.QueryEscape(`{ _service { sdl } }`)
	req := httptest.NewRequest(http.MethodGet, graphqlPath+"?query="+query, nil)
	w := httptest.NewRecorder()

	srv.handleGraphQL(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "sdl") {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestHandleGraphQLGetBadVariables(t *testing.T) {
	srv, _ := newGraphQLServer(t, 10)
	req := httptest.NewRequest(http.MethodGet, graphqlPath+"?query=%7B_service%7Bsdl%7D%7D&variables=%7Bbroken", nil)
	w := httptest.NewRecorder()

	srv.handleGraphQL(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid GraphQL variables") {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestHandleGraphQLWrongMethod(t *testing.T) {
	srv, _ := newGraphQLServer(t, 10)
	req := httptest.NewRequest(http.MethodPut, graphqlPath, nil)
	w := httptest.NewRecorder()

	srv.handleGraphQL(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
	if got := w.Header().Get("Allow"); got != "GET, POST" {
		t.Errorf("Allow = %q", got)
	}
	if !strings.Contains(w.Body.String(), "only GET and POST") {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestParseVariables(t *testing.T) {
	if got, err := parseVariables(""); err != nil || got != nil {
		t.Fatalf("empty = %v, %v", got, err)
	}
	got, err := parseVariables(`{"a":1}`)
	if err != nil {
		t.Fatalf("valid: %v", err)
	}
	if v, ok := got["a"].(float64); !ok || v != 1 {
		t.Fatalf("got = %v", got)
	}
	if _, err := parseVariables("{broken"); err == nil {
		t.Fatal("expected error for malformed json")
	} else if !strings.Contains(err.Error(), "invalid GraphQL variables") {
		t.Errorf("err = %v", err)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusTeapot, errBoom)

	if w.Code != http.StatusTeapot {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content type = %q", ct)
	}
	var payload struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Errors) != 1 || payload.Errors[0].Message != "boom" {
		t.Fatalf("payload = %s", w.Body.String())
	}
}

func TestRequestContextIdentityFromHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, graphqlPath, nil)
	req.Header.Set("x-user-id", "u1")
	req.Header.Set("x-user-role", "admin")
	req.Header.Set("x-correlation-id", "trace-1")

	ctx := requestContext(req)
	if got := sharedlogging.GetUserID(ctx); got != "u1" {
		t.Errorf("userID = %q", got)
	}
	if got := sharedlogging.GetTraceID(ctx); got != "trace-1" {
		t.Errorf("traceID = %q", got)
	}
	if got, _ := ctx.Value("user_role").(string); got != "admin" {
		t.Errorf("role = %q", got)
	}
	if sharedlogging.GetMethod(ctx) != http.MethodPost {
		t.Errorf("method = %q", sharedlogging.GetMethod(ctx))
	}
	if sharedlogging.GetPath(ctx) != graphqlPath {
		t.Errorf("path = %q", sharedlogging.GetPath(ctx))
	}
}

func TestRequestContextIdentityFromJWT(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	token := makeBearerToken(t, "test-secret", "u9", "admin", time.Now().Add(time.Hour).Unix())

	req := httptest.NewRequest(http.MethodPost, graphqlPath, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	ctx := requestContext(req)
	if got := sharedlogging.GetUserID(ctx); got != "u9" {
		t.Errorf("userID = %q", got)
	}
	if got, _ := ctx.Value("user_role").(string); got != "admin" {
		t.Errorf("role = %q", got)
	}
}

func TestRequestContextInvalidJWTIgnored(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	req := httptest.NewRequest(http.MethodPost, graphqlPath, nil)
	req.Header.Set("Authorization", "Bearer not.a.token")

	ctx := requestContext(req)
	if got := sharedlogging.GetUserID(ctx); got != "" {
		t.Errorf("userID = %q, want empty", got)
	}
}

func TestRequestContextLanguageFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		query    string
		wantLang string
	}{
		{name: "x-lang header", header: "ar", wantLang: "ar"},
		{name: "query param", query: "ar", wantLang: "ar"},
		{name: "accept-language", wantLang: "ar"},
		{name: "default", wantLang: "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := graphqlPath
			if tt.query != "" {
				target += "?lang=" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, target, nil)
			if tt.header != "" {
				req.Header.Set("x-lang", tt.header)
			}
			if tt.name == "accept-language" {
				req.Header.Set("Accept-Language", "ar-EG,ar;q=0.9")
			}
			ctx := requestContext(req)
			if got := i18n.FromContext(ctx); got != tt.wantLang {
				t.Errorf("lang = %q, want %q", got, tt.wantLang)
			}
		})
	}
}

func TestServeLifecycle(t *testing.T) {
	f := newFixture(t, 10)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := strconv.Itoa(lis.Addr().(*net.TCPAddr).Port)
	_ = lis.Close()

	srv, err := NewServer(f.h, port)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx) }()

	baseURL := "http://127.0.0.1:" + port
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, getErr := http.Get(baseURL + "/healthz")
		if getErr == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not become healthy in time")
		}
		select {
		case serveErr := <-errCh:
			t.Fatalf("Serve exited early: %v", serveErr)
		default:
		}
		time.Sleep(25 * time.Millisecond)
	}

	resp, err := http.Post(baseURL+graphqlPath, "application/json",
		strings.NewReader(`{"query":"{ _service { sdl } }"}`))
	if err != nil {
		t.Fatalf("graphql post: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("graphql status = %d, body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "sdl") {
		t.Fatalf("unexpected graphql body: %s", body)
	}

	cancel()
	select {
	case serveErr := <-errCh:
		if serveErr != nil {
			t.Fatalf("Serve returned error: %v", serveErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
}
