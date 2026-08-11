package ratelimiter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// RateLimiter implements a Redis sliding window rate limiter
type RateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

// Result holds the details of a rate-limit check
type Result struct {
	Allowed           bool
	Limit             int
	Remaining         int
	ResetAtSeconds    int64
	RetryAfterSeconds int64
}

// NewRateLimiter creates a new RateLimiter instance
func NewRateLimiter(client *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client: client,
		limit:  limit,
		window: window,
	}
}

// Limit checks if the given key has exceeded the allowed rate limit
func (r *RateLimiter) Limit(ctx context.Context, key string) (*Result, error) {
	now := time.Now()
	nowMs := now.UnixNano() / int64(time.Millisecond)
	windowMs := int64(r.window / time.Millisecond)
	clearBefore := nowMs - windowMs

	member := fmt.Sprintf("%d:%s", nowMs, uuid.NewString())
	redisKey := fmt.Sprintf("ratelimit:%s", key)

	pipe := r.client.TxPipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", clearBefore))
	zCard := pipe.ZCard(ctx, redisKey)
	pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(nowMs), Member: member})
	pipe.Expire(ctx, redisKey, r.window)
	zRange := pipe.ZRangeWithScores(ctx, redisKey, 0, 0)

	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	count := int(zCard.Val())
	allowed := true
	remaining := r.limit - count

	if count >= r.limit {
		allowed = false
		remaining = 0
		// Remove the newly added member to prevent artificial inflation
		r.client.ZRem(ctx, redisKey, member)
	}

	var retryAfter int64
	var resetAt int64

	if len(zRange.Val()) > 0 {
		oldestMs := int64(zRange.Val()[0].Score)
		resetAtMs := oldestMs + windowMs
		resetAt = resetAtMs / 1000
		retryAfter = (resetAtMs - nowMs) / 1000
		if retryAfter < 0 {
			retryAfter = 0
		}
	} else {
		resetAt = (nowMs + windowMs) / 1000
		retryAfter = windowMs / 1000
	}

	return &Result{
		Allowed:           allowed,
		Limit:             r.limit,
		Remaining:         remaining,
		ResetAtSeconds:    resetAt,
		RetryAfterSeconds: retryAfter,
	}, nil
}

// UnaryServerInterceptor returns a gRPC unary interceptor that applies rate limiting
func (r *RateLimiter) UnaryServerInterceptor(keyExtractor func(context.Context) string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		var key string
		if keyExtractor != nil {
			key = keyExtractor(ctx)
		} else {
			key = extractDefaultGRPCKey(ctx)
		}

		res, err := r.Limit(ctx, key)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "rate limiter check error: %v", err)
		}

		// Inject rate limiting headers into outbound metadata if headers context is available
		header := metadata.Pairs(
			"X-RateLimit-Limit", strconv.Itoa(res.Limit),
			"X-RateLimit-Remaining", strconv.Itoa(res.Remaining),
			"X-RateLimit-Reset", strconv.FormatInt(res.ResetAtSeconds, 10),
		)
		if !res.Allowed {
			header.Set("Retry-After", strconv.FormatInt(res.RetryAfterSeconds, 10))
			grpc.SendHeader(ctx, header)
			return nil, status.Error(codes.ResourceExhausted, "too many requests: rate limit exceeded")
		}

		grpc.SendHeader(ctx, header)
		return handler(ctx, req)
	}
}

// HTTPMiddleware returns an HTTP middleware that applies rate limiting
func (r *RateLimiter) HTTPMiddleware(keyExtractor func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			var key string
			if keyExtractor != nil {
				key = keyExtractor(req)
			} else {
				key = extractDefaultHTTPKey(req)
			}

			res, err := r.Limit(req.Context(), key)
			if err != nil {
				http.Error(w, "Rate limiter error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(res.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(res.ResetAtSeconds, 10))

			if !res.Allowed {
				w.Header().Set("Retry-After", strconv.FormatInt(res.RetryAfterSeconds, 10))
				http.Error(w, "Too many requests, please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

// Helper to extract default key from gRPC context (User ID or Remote IP)
func extractDefaultGRPCKey(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if vals := md.Get("x-user-id"); len(vals) > 0 && vals[0] != "" {
			return fmt.Sprintf("user:%s", vals[0])
		}
		if vals := md.Get("x-forwarded-for"); len(vals) > 0 && vals[0] != "" {
			ips := strings.Split(vals[0], ",")
			return fmt.Sprintf("ip:%s", strings.TrimSpace(ips[0]))
		}
	}

	p, ok := peer.FromContext(ctx)
	if ok && p != nil {
		host, _, err := net.SplitHostPort(p.Addr.String())
		if err == nil {
			return fmt.Sprintf("ip:%s", host)
		}
		return fmt.Sprintf("ip:%s", p.Addr.String())
	}

	return "ip:unknown"
}

// Helper to extract default key from HTTP request (User ID or Remote IP)
func extractDefaultHTTPKey(r *http.Request) string {
	if userID := r.Header.Get("x-user-id"); userID != "" {
		return fmt.Sprintf("user:%s", userID)
	}

	if xff := r.Header.Get("x-forwarded-for"); xff != "" {
		ips := strings.Split(xff, ",")
		return fmt.Sprintf("ip:%s", strings.TrimSpace(ips[0]))
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return fmt.Sprintf("ip:%s", ip)
	}
	return fmt.Sprintf("ip:%s", r.RemoteAddr)
}
