package grpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	sharedratelimiter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/ratelimiter"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/download"
	appMedia "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/media"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/grpc/pb"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func newTestRateLimiter(t *testing.T, limit int) *sharedratelimiter.RateLimiter {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return sharedratelimiter.NewRateLimiter(client, limit, time.Minute)
}

type serverFixture struct {
	server    *Server
	mediaRepo *testutil.MockMediaRepository
	storage   *testutil.MemStorage
}

func newServerFixture(t *testing.T, rateLimit int) *serverFixture {
	t.Helper()
	mediaRepo := testutil.NewMockMediaRepository()
	versionRepo := testutil.NewMockVersionRepository()
	storage := testutil.NewMemStorage()
	limiter := newTestRateLimiter(t, rateLimit)
	dl := download.NewGetDownloadUrlUseCase(mediaRepo, versionRepo, storage, limiter, time.Hour)
	gm := appMedia.NewGetMediaUseCase(mediaRepo, versionRepo)
	return &serverFixture{server: NewServer(dl, gm), mediaRepo: mediaRepo, storage: storage}
}

func (f *serverFixture) userCtx(userID string) context.Context {
	return sharedlogging.WithUserID(context.Background(), userID)
}

func TestNewServer(t *testing.T) {
	fx := newServerFixture(t, 10)
	if fx.server == nil {
		t.Fatal("NewServer returned nil")
	}
	if fx.server.getDownloadURL == nil {
		t.Error("getDownloadURL not wired")
	}
	if fx.server.getMedia == nil {
		t.Error("getMedia not wired")
	}
	if fx.server.errorMapper == nil {
		t.Error("errorMapper not wired")
	}
}

func TestNewServerOptions(t *testing.T) {
	opts := NewServerOptions()
	if len(opts) == 0 {
		t.Fatal("expected at least one server option")
	}
	for i, opt := range opts {
		if opt == nil {
			t.Fatalf("option %d is nil", i)
		}
	}
}

func TestNewListener(t *testing.T) {
	lis, err := NewListener("127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewListener: %v", err)
	}
	defer lis.Close()
	if lis.Addr() == nil {
		t.Fatal("listener has nil address")
	}
	if _, err := NewListener("no-such-host:99999999"); err == nil {
		t.Fatal("expected error for invalid address")
	}
}

func TestUnaryInterceptor(t *testing.T) {
	tests := []struct {
		name      string
		md        metadata.MD
		wantUser  string
		wantTrace string
	}{
		{name: "no metadata"},
		{
			name:      "requester and correlation ids",
			md:        metadata.Pairs("x-requester-id", "u1", "x-correlation-id", "trace-1"),
			wantUser:  "u1",
			wantTrace: "trace-1",
		},
		{
			name:      "x-user-id overrides x-requester-id",
			md:        metadata.Pairs("x-requester-id", "u1", "x-user-id", "u2"),
			wantUser:  "u2",
			wantTrace: "",
		},
		{
			name:      "correlation id only",
			md:        metadata.Pairs("x-correlation-id", "trace-2"),
			wantUser:  "",
			wantTrace: "trace-2",
		},
	}

	info := &ggrpc.UnaryServerInfo{FullMethod: "/media.MediaService/GetMedia"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.md)
			}

			var gotCtx context.Context
			resp, err := UnaryInterceptor(ctx, "req", info, func(ctx context.Context, req interface{}) (interface{}, error) {
				gotCtx = ctx
				if req != "req" {
					t.Errorf("handler got req %v", req)
				}
				return "resp", nil
			})
			if err != nil {
				t.Fatalf("interceptor error: %v", err)
			}
			if resp != "resp" {
				t.Fatalf("handler response not propagated: %v", resp)
			}
			if got := sharedlogging.GetUserID(gotCtx); got != tt.wantUser {
				t.Errorf("userID = %q, want %q", got, tt.wantUser)
			}
			if got := sharedlogging.GetTraceID(gotCtx); got != tt.wantTrace {
				t.Errorf("traceID = %q, want %q", got, tt.wantTrace)
			}
			if got := sharedlogging.GetMethod(gotCtx); got != info.FullMethod {
				t.Errorf("method = %q, want %q", got, info.FullMethod)
			}
			if got := sharedlogging.GetPath(gotCtx); got != info.FullMethod {
				t.Errorf("path = %q, want %q", got, info.FullMethod)
			}
		})
	}
}

func TestResolveMediaUrl(t *testing.T) {
	t.Run("missing requester id", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		_, err := fx.server.ResolveMediaUrl(context.Background(), &pb.ResolveMediaUrlRequest{MediaId: "m1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("code = %v, want Unauthenticated (err=%v)", status.Code(err), err)
		}
	})

	t.Run("requester from request field", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		fx.mediaRepo.Media = testutil.NewMedia("m1", "u1")

		out, err := fx.server.ResolveMediaUrl(context.Background(), &pb.ResolveMediaUrlRequest{
			MediaId:       "m1",
			RequesterId:   "u1",
			ExpirySeconds: 600,
		})
		if err != nil {
			t.Fatalf("ResolveMediaUrl: %v", err)
		}
		if out.GetUrl() == "" {
			t.Error("expected non-empty url")
		}
		if out.GetMediaId() != "m1" {
			t.Errorf("mediaId = %q, want m1", out.GetMediaId())
		}
		if out.GetExpiresAtSeconds() <= 0 {
			t.Errorf("expiresAtSeconds = %d, want > 0", out.GetExpiresAtSeconds())
		}
		if out.GetContentType() != "image/jpeg" {
			t.Errorf("contentType = %q, want image/jpeg", out.GetContentType())
		}
	})

	t.Run("requester from context wins over request field", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		fx.mediaRepo.Media = testutil.NewMedia("m1", "u1")

		_, err := fx.server.ResolveMediaUrl(fx.userCtx("u1"), &pb.ResolveMediaUrlRequest{
			MediaId:     "m1",
			RequesterId: "someone-else",
		})
		if err != nil {
			t.Fatalf("ResolveMediaUrl: %v", err)
		}
	})

	t.Run("media not found", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		fx.mediaRepo.GetErr = domain.ErrMediaNotFound

		_, err := fx.server.ResolveMediaUrl(fx.userCtx("u1"), &pb.ResolveMediaUrlRequest{MediaId: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound (err=%v)", status.Code(err), err)
		}
	})

	t.Run("not owner", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		fx.mediaRepo.Media = testutil.NewMedia("m1", "owner")

		_, err := fx.server.ResolveMediaUrl(fx.userCtx("intruder"), &pb.ResolveMediaUrlRequest{MediaId: "m1"})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied (err=%v)", status.Code(err), err)
		}
	})

	t.Run("quarantined media", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		m := testutil.NewMedia("m1", "u1")
		m.Status = domain.MediaStatusQuarantined
		fx.mediaRepo.Media = m

		_, err := fx.server.ResolveMediaUrl(fx.userCtx("u1"), &pb.ResolveMediaUrlRequest{MediaId: "m1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("code = %v, want FailedPrecondition (err=%v)", status.Code(err), err)
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		fx := newServerFixture(t, 0)
		fx.mediaRepo.Media = testutil.NewMedia("m1", "u1")

		_, err := fx.server.ResolveMediaUrl(fx.userCtx("u1"), &pb.ResolveMediaUrlRequest{MediaId: "m1"})
		if status.Code(err) != codes.ResourceExhausted {
			t.Fatalf("code = %v, want ResourceExhausted (err=%v)", status.Code(err), err)
		}
	})

	t.Run("storage failure maps to internal", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		fx.mediaRepo.Media = testutil.NewMedia("m1", "u1")
		fx.storage.SetError("GeneratePresignedGETWithRange", errors.New("s3 down"))

		_, err := fx.server.ResolveMediaUrl(fx.userCtx("u1"), &pb.ResolveMediaUrlRequest{MediaId: "m1"})
		if status.Code(err) != codes.Internal {
			t.Fatalf("code = %v, want Internal (err=%v)", status.Code(err), err)
		}
	})

	t.Run("version type uses version object key", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		fx.mediaRepo.Media = testutil.NewMedia("m1", "u1")

		out, err := fx.server.ResolveMediaUrl(fx.userCtx("u1"), &pb.ResolveMediaUrlRequest{
			MediaId:     "m1",
			VersionType: "original",
		})
		if err != nil {
			t.Fatalf("ResolveMediaUrl: %v", err)
		}
		if out.GetUrl() == "" {
			t.Error("expected non-empty url")
		}
	})
}

func TestGetMedia(t *testing.T) {
	t.Run("missing requester id", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		_, err := fx.server.GetMedia(context.Background(), &pb.GetMediaRequest{MediaId: "m1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("code = %v, want Unauthenticated (err=%v)", status.Code(err), err)
		}
	})

	t.Run("success via request field", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		fx.mediaRepo.Media = testutil.NewMedia("m1", "u1")

		out, err := fx.server.GetMedia(context.Background(), &pb.GetMediaRequest{MediaId: "m1", RequesterId: "u1"})
		if err != nil {
			t.Fatalf("GetMedia: %v", err)
		}
		if out.GetMediaId() != "m1" || out.GetOwnerId() != "u1" {
			t.Errorf("unexpected identity fields: %+v", out)
		}
		if out.GetFileName() != "photo.jpg" || out.GetContentType() != "image/jpeg" {
			t.Errorf("unexpected file fields: %+v", out)
		}
		if out.GetMediaType() != string(domain.MediaTypeImage) {
			t.Errorf("mediaType = %q, want IMAGE", out.GetMediaType())
		}
		if out.GetSize() != 1024 || out.GetStatus() != string(domain.MediaStatusReady) {
			t.Errorf("unexpected size/status: %+v", out)
		}
		if out.GetObjectKey() == "" {
			t.Error("expected objectKey")
		}
		if out.GetCreatedAtSeconds() <= 0 || out.GetUpdatedAtSeconds() <= 0 {
			t.Errorf("timestamps not mapped: %+v", out)
		}
	})

	t.Run("success via context user", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		fx.mediaRepo.Media = testutil.NewMedia("m1", "u1")

		if _, err := fx.server.GetMedia(fx.userCtx("u1"), &pb.GetMediaRequest{MediaId: "m1"}); err != nil {
			t.Fatalf("GetMedia: %v", err)
		}
		if got := fx.mediaRepo.GetByIDCalls; len(got) != 1 || got[0] != "m1" {
			t.Errorf("GetByIDCalls = %v, want [m1]", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		fx.mediaRepo.GetErr = domain.ErrMediaNotFound

		_, err := fx.server.GetMedia(fx.userCtx("u1"), &pb.GetMediaRequest{MediaId: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound (err=%v)", status.Code(err), err)
		}
	})

	t.Run("not owner", func(t *testing.T) {
		fx := newServerFixture(t, 10)
		fx.mediaRepo.Media = testutil.NewMedia("m1", "owner")

		_, err := fx.server.GetMedia(fx.userCtx("intruder"), &pb.GetMediaRequest{MediaId: "m1"})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("code = %v, want PermissionDenied (err=%v)", status.Code(err), err)
		}
	})
}

func TestServerEndToEnd(t *testing.T) {
	fx := newServerFixture(t, 10)
	fx.mediaRepo.Media = testutil.NewMedia("m1", "u1")

	lis := bufconn.Listen(1 << 20)
	srv := ggrpc.NewServer(NewServerOptions()...)
	pb.RegisterMediaServiceServer(srv, fx.server)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := ggrpc.NewClient(
		"passthrough:///bufnet",
		ggrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		ggrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := pb.NewMediaServiceClient(conn)

	callCtx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("x-requester-id", "u1", "x-correlation-id", "trace-e2e"),
	)

	resolved, err := client.ResolveMediaUrl(callCtx, &pb.ResolveMediaUrlRequest{MediaId: "m1"})
	if err != nil {
		t.Fatalf("ResolveMediaUrl: %v", err)
	}
	if resolved.GetUrl() == "" || resolved.GetMediaId() != "m1" {
		t.Fatalf("unexpected ResolveMediaUrl response: %+v", resolved)
	}

	if _, err := client.GetMedia(context.Background(), &pb.GetMediaRequest{MediaId: "m1"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetMedia without requester: code = %v, want Unauthenticated (err=%v)", status.Code(err), err)
	}

	got, err := client.GetMedia(callCtx, &pb.GetMediaRequest{MediaId: "m1"})
	if err != nil {
		t.Fatalf("GetMedia: %v", err)
	}
	if got.GetMediaId() != "m1" || got.GetOwnerId() != "u1" || got.GetStatus() != string(domain.MediaStatusReady) {
		t.Fatalf("unexpected GetMedia response: %+v", got)
	}
}
