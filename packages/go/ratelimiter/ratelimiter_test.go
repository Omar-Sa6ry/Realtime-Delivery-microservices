package ratelimiter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"net"
)

type fakePipeline struct {
	zCard      int64
	zRange     []redis.Z
	execErr    error
	rangeCalls int
}

func (p *fakePipeline) ZRemRangeByScore(ctx context.Context, key, min, max string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(0)
	return cmd
}

func (p *fakePipeline) ZCard(ctx context.Context, key string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(p.zCard)
	return cmd
}

func (p *fakePipeline) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(int64(len(members)))
	return cmd
}

func (p *fakePipeline) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx)
	cmd.SetVal(true)
	return cmd
}

func (p *fakePipeline) ZRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd {
	p.rangeCalls++
	cmd := redis.NewZSliceCmd(ctx)
	cmd.SetVal(p.zRange)
	return cmd
}

func (p *fakePipeline) Exec(ctx context.Context) ([]redis.Cmder, error) {
	if p.execErr != nil {
		return nil, p.execErr
	}
	return nil, nil
}

type fakeStore struct {
	pipe      *fakePipeline
	zRemKeys  []string
	zRemErr   error
	pipeCalls int
}

func (s *fakeStore) TxPipeline() RedisPipeline {
	s.pipeCalls++
	return s.pipe
}

func (s *fakeStore) ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	s.zRemKeys = append(s.zRemKeys, key)
	cmd := redis.NewIntCmd(ctx)
	if s.zRemErr != nil {
		cmd.SetErr(s.zRemErr)
		return cmd
	}
	cmd.SetVal(int64(len(members)))
	return cmd
}

func newTestLimiter(store *fakeStore, limit int, window time.Duration) *RateLimiter {
	return newRateLimiter(store, limit, window)
}

func TestLimitAllowsRequestsBelowThreshold(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{zCard: 1}}
	limiter := newTestLimiter(store, 5, time.Minute)

	res, err := limiter.Limit(context.Background(), "user:1")
	require.NoError(t, err)

	assert.True(t, res.Allowed)
	assert.Equal(t, 5, res.Limit)
	assert.Equal(t, 4, res.Remaining)
	assert.Positive(t, res.ResetAtSeconds)
	assert.Positive(t, res.RetryAfterSeconds)
	assert.Equal(t, 1, store.pipeCalls)
}

func TestLimitRejectsWhenThresholdReached(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{zCard: 5}}
	limiter := newTestLimiter(store, 5, time.Minute)

	res, err := limiter.Limit(context.Background(), "user:1")
	require.NoError(t, err)

	assert.False(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)
	assert.Equal(t, "ratelimit:user:1", store.zRemKeys[0])
}

func TestLimitComputesResetFromOldestEntry(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{
		zCard:  1,
		zRange: []redis.Z{{Score: 1, Member: "old"}},
	}}
	limiter := newTestLimiter(store, 5, time.Minute)

	res, err := limiter.Limit(context.Background(), "k")
	require.NoError(t, err)

	// Oldest entry is ancient, so the retry horizon collapses to zero.
	assert.Equal(t, int64(0), res.RetryAfterSeconds)
	assert.Equal(t, int64((1+int64(time.Minute/time.Millisecond))/1000), res.ResetAtSeconds)
}

func TestLimitPropagatesPipelineError(t *testing.T) {
	boom := errors.New("redis is down")
	store := &fakeStore{pipe: &fakePipeline{execErr: boom}}
	limiter := newTestLimiter(store, 5, time.Minute)

	res, err := limiter.Limit(context.Background(), "k")
	require.Error(t, err)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, boom)
}

func TestLimitIgnoresRedisNilFromPipeline(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{execErr: redis.Nil, zCard: 2}}
	limiter := newTestLimiter(store, 5, time.Minute)

	res, err := limiter.Limit(context.Background(), "k")
	require.NoError(t, err)
	assert.True(t, res.Allowed)
}

func TestLimitKeyPrefixing(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{zCard: 1}}
	limiter := newTestLimiter(store, 1, time.Second)

	res, err := limiter.Limit(context.Background(), "ip:1.2.3.4")
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	require.Len(t, store.zRemKeys, 1)
	assert.Equal(t, "ratelimit:ip:1.2.3.4", store.zRemKeys[0])
}

// --- HTTP middleware ---

func TestHTTPMiddlewareAllowsAndForwards(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{zCard: 0}}
	limiter := newTestLimiter(store, 3, time.Minute)

	var called bool
	handler := limiter.HTTPMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/x", nil))

	assert.True(t, called)
	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Equal(t, "3", rec.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "3", rec.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, rec.Header().Get("X-RateLimit-Reset"))
	assert.Empty(t, rec.Header().Get("Retry-After"))
}

func TestHTTPMiddlewareRejectsWith429(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{zCard: 3, zRange: []redis.Z{{Score: 1, Member: "old"}}}}
	limiter := newTestLimiter(store, 3, time.Minute)

	var called bool
	handler := limiter.HTTPMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/x", nil))

	assert.False(t, called)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "0", rec.Header().Get("X-RateLimit-Remaining"))
	assert.Equal(t, "0", rec.Header().Get("Retry-After"))
	assert.Contains(t, rec.Body.String(), "Too many requests")
}

func TestHTTPMiddlewareRedisFailure(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{execErr: errors.New("redis down")}}
	limiter := newTestLimiter(store, 3, time.Minute)

	handler := limiter.HTTPMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run when the limiter errors")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/x", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "Rate limiter error")
}

func TestExtractDefaultHTTPKey(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*http.Request)
		want   string
	}{
		{"user id header wins", func(r *http.Request) {
			r.Header.Set("x-user-id", "u-9")
			r.Header.Set("x-forwarded-for", "1.1.1.1")
		}, "user:u-9"},
		{"first forwarded ip", func(r *http.Request) {
			r.Header.Set("x-forwarded-for", " 9.9.9.9 , 8.8.8.8")
		}, "ip:9.9.9.9"},
		{"remote addr ip", func(r *http.Request) {
			r.RemoteAddr = "10.0.0.5:1234"
		}, "ip:10.0.0.5"},
		{"remote addr without port", func(r *http.Request) {
			r.RemoteAddr = "10.0.0.5"
		}, "ip:10.0.0.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.mutate(req)
			assert.Equal(t, tt.want, extractDefaultHTTPKey(req))
		})
	}
}

func TestHTTPMiddlewareCustomKeyExtractor(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{zCard: 0}}
	limiter := newTestLimiter(store, 3, time.Minute)

	var gotKey string
	handler := limiter.HTTPMiddleware(func(r *http.Request) string {
		gotKey = "tenant:" + r.Header.Get("X-Tenant")
		return gotKey
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant", "acme")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "tenant:acme", gotKey)
}

// --- gRPC interceptor ---

func TestGRPCInterceptorAllows(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{zCard: 0}}
	limiter := newTestLimiter(store, 2, time.Minute)

	interceptor := limiter.UnaryServerInterceptor(nil)
	called := false

	resp, err := interceptor(
		metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "u-1")),
		"req", &grpc.UnaryServerInfo{FullMethod: "/svc/Call"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			called = true
			return "ok", nil
		})

	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "ok", resp)
}

func TestGRPCInterceptorRejects(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{zCard: 9}}
	limiter := newTestLimiter(store, 2, time.Minute)

	interceptor := limiter.UnaryServerInterceptor(nil)
	called := false

	_, err := interceptor(context.Background(), "req",
		&grpc.UnaryServerInfo{FullMethod: "/svc/Call"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			called = true
			return nil, nil
		})

	require.Error(t, err)
	assert.False(t, called)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "too many requests")
}

func TestGRPCInterceptorRedisFailure(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{execErr: errors.New("redis down")}}
	limiter := newTestLimiter(store, 2, time.Minute)

	_, err := limiter.UnaryServerInterceptor(nil)(context.Background(), "req",
		&grpc.UnaryServerInfo{FullMethod: "/svc/Call"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGRPCInterceptorCustomKeyExtractor(t *testing.T) {
	store := &fakeStore{pipe: &fakePipeline{zCard: 0}}
	limiter := newTestLimiter(store, 2, time.Minute)

	var gotKey string
	_, err := limiter.UnaryServerInterceptor(func(ctx context.Context) string {
		gotKey = "custom-key"
		return gotKey
	})(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Call"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})

	require.NoError(t, err)
	assert.Equal(t, "custom-key", gotKey)
}

func TestExtractDefaultGRPCKey(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() context.Context
		want string
	}{
		{
			name: "user id metadata",
			ctx: func() context.Context {
				return metadata.NewIncomingContext(context.Background(),
					metadata.Pairs("x-user-id", "u-1", "x-forwarded-for", "1.1.1.1"))
			},
			want: "user:u-1",
		},
		{
			name: "forwarded for metadata",
			ctx: func() context.Context {
				return metadata.NewIncomingContext(context.Background(),
					metadata.Pairs("x-forwarded-for", " 2.2.2.2 , 3.3.3.3"))
			},
			want: "ip:2.2.2.2",
		},
		{
			name: "peer address with port",
			ctx: func() context.Context {
				return peer.NewContext(context.Background(),
					&peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP("10.1.2.3"), Port: 7777}})
			},
			want: "ip:10.1.2.3",
		},
		{
			name: "unparseable peer address",
			ctx: func() context.Context {
				return peer.NewContext(context.Background(),
					&peer.Peer{Addr: stringAddr("weird-addr")})
			},
			want: "ip:weird-addr",
		},
		{
			name: "no metadata no peer",
			ctx:  func() context.Context { return context.Background() },
			want: "ip:unknown",
		},
		{
			name: "empty user id falls through to forwarded for",
			ctx: func() context.Context {
				return metadata.NewIncomingContext(context.Background(),
					metadata.Pairs("x-user-id", "", "x-forwarded-for", "4.4.4.4"))
			},
			want: "ip:4.4.4.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractDefaultGRPCKey(tt.ctx()))
		})
	}
}

type stringAddr string

func (a stringAddr) Network() string { return "test" }
func (a stringAddr) String() string  { return string(a) }
