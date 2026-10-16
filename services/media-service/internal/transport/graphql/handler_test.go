package graphql

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	sharedratelimiter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/ratelimiter"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/download"
	appMedia "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/media"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/upload"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/i18n"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/graphql/dlq"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/validation"
	"github.com/alicebob/miniredis/v2"
	gql "github.com/graphql-go/graphql"
	"github.com/redis/go-redis/v9"
)

var errBoom = errors.New("boom")

func TestMain(m *testing.M) {
	observability.RegisterMediaMetrics()
	os.Exit(m.Run())
}

type fixture struct {
	h      *Handler
	schema gql.Schema

	mediaRepo   *testutil.MockMediaRepository
	uploadRepo  *testutil.MockUploadRepository
	quotaRepo   *testutil.MockQuotaRepository
	outboxRepo  *testutil.MockOutboxRepository
	versionRepo *testutil.MockVersionRepository
	cache       *testutil.MockCache
	storage     *testutil.MemStorage
	publisher   *testutil.MockPublisher
	dlqMgr      *dlq.DLQManager
}

func newRateLimiter(t *testing.T, limit int) *sharedratelimiter.RateLimiter {
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

func newFixture(t *testing.T, rateLimit int) *fixture {
	t.Helper()

	mediaRepo := testutil.NewMockMediaRepository()
	uploadRepo := testutil.NewMockUploadRepository()
	quotaRepo := testutil.NewMockQuotaRepository()
	outboxRepo := testutil.NewMockOutboxRepository()
	versionRepo := testutil.NewMockVersionRepository()
	cache := testutil.NewMockCache()
	storage := testutil.NewMemStorage()
	publisher := testutil.NewMockPublisher()
	limiter := newRateLimiter(t, rateLimit)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dlqMgr := dlq.NewDLQManager(publisher, logger)

	allowed := map[string]struct{}{"image/jpeg": {}}
	validator := validation.NewValidator(allowed, storage, 1<<30)

	createSession := upload.NewCreateSessionUseCase(
		mediaRepo, uploadRepo, quotaRepo, storage, cache, validator, limiter,
		time.Hour, 15*time.Minute, 5<<20, 1<<30,
	)
	completeUpload := upload.NewCompleteUploadUseCase(
		uploadRepo, mediaRepo, outboxRepo, quotaRepo, storage, cache, publisher, validator,
	)
	abortUpload := upload.NewAbortUploadUseCase(uploadRepo, mediaRepo, quotaRepo, storage, cache)
	getUploadStatus := upload.NewGetUploadStatusUseCase(uploadRepo, storage)
	renewPresigned := upload.NewRenewPresignedUseCase(uploadRepo, storage, cache, 15*time.Minute)
	getMedia := appMedia.NewGetMediaUseCase(mediaRepo, versionRepo)
	listMedia := appMedia.NewListMediaUseCase(mediaRepo)
	deleteMedia := appMedia.NewDeleteMediaUseCase(mediaRepo, outboxRepo, quotaRepo, cache)
	getDownloadURL := download.NewGetDownloadUrlUseCase(mediaRepo, versionRepo, storage, limiter, time.Hour)

	handler := NewHandler(
		createSession, completeUpload, abortUpload, getUploadStatus, renewPresigned,
		getMedia, listMedia, deleteMedia, getDownloadURL,
		quotaRepo, dlqMgr, []string{"127.0.0.1:1"},
	)
	schema, err := buildSchema(handler)
	if err != nil {
		t.Fatalf("buildSchema: %v", err)
	}
	return &fixture{
		h: handler, schema: schema,
		mediaRepo: mediaRepo, uploadRepo: uploadRepo, quotaRepo: quotaRepo,
		outboxRepo: outboxRepo, versionRepo: versionRepo, cache: cache,
		storage: storage, publisher: publisher, dlqMgr: dlqMgr,
	}
}

func userCtx(userID string) context.Context {
	return sharedlogging.WithUserID(context.Background(), userID)
}

func adminCtx(userID string) context.Context {
	return context.WithValue(userCtx(userID), "user_role", "admin")
}

func runGQL(t *testing.T, f *fixture, ctx context.Context, query string) *gql.Result {
	t.Helper()
	return gql.Do(gql.Params{Schema: f.schema, RequestString: query, Context: ctx})
}

func dataMap(t *testing.T, result *gql.Result, field string) map[string]interface{} {
	t.Helper()
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	root, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is %T, want map", result.Data)
	}
	outer, ok := root[field].(map[string]interface{})
	if !ok {
		t.Fatalf("%s is %T, want map", field, root[field])
	}
	return outer
}

func mustContainError(t *testing.T, result *gql.Result, needle string) {
	t.Helper()
	if len(result.Errors) == 0 {
		t.Fatal("expected errors, got none")
	}
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Error(), needle) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no error contains %q; errors: %v", needle, result.Errors)
	}
}

func TestQueryMedia(t *testing.T) {
	t.Run("success for owner", func(t *testing.T) {
		f := newFixture(t, 10)
		media := testutil.NewMedia("m1", "u1")
		f.mediaRepo.Store["m1"] = media
		f.versionRepo.Versions["m1"] = []*domain.MediaVersion{{
			MediaID: "m1", VersionType: "original", ObjectKey: media.ObjectKey,
			ContentType: "image/jpeg", Size: 1024, Width: 100, Height: 80,
		}}

		result := runGQL(t, f, userCtx("u1"), `{ media(mediaId: "m1") { success statusCode message data { mediaId ownerId fileName contentType mediaType size status objectKey versions { versionType objectKey width height } } } }`)
		outer := dataMap(t, result, "media")
		if outer["success"] != true || outer["statusCode"] != 200 {
			t.Fatalf("unexpected envelope: %v", outer)
		}
		if outer["message"] != i18n.T("en", "media.fetched") {
			t.Errorf("message = %v", outer["message"])
		}
		inner, ok := outer["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("data is %T", outer["data"])
		}
		if inner["mediaId"] != "m1" || inner["ownerId"] != "u1" || inner["fileName"] != "photo.jpg" {
			t.Errorf("unexpected media: %v", inner)
		}
		if inner["mediaType"] != string(domain.MediaTypeImage) || inner["status"] != string(domain.MediaStatusReady) {
			t.Errorf("unexpected mediaType/status: %v", inner)
		}
		versions, ok := inner["versions"].([]interface{})
		if !ok || len(versions) != 1 {
			t.Fatalf("versions = %v", inner["versions"])
		}
		v := versions[0].(map[string]interface{})
		if v["versionType"] != "original" || v["width"] != 100 {
			t.Errorf("unexpected version: %v", v)
		}
	})

	t.Run("non owner is rejected", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "owner")

		result := runGQL(t, f, userCtx("intruder"), `{ media(mediaId: "m1") { success } }`)
		mustContainError(t, result, domain.ErrUnauthorized.Error())
	})

	t.Run("admin may read foreign media", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "owner")

		result := runGQL(t, f, adminCtx("boss"), `{ media(mediaId: "m1") { success data { mediaId ownerId } } }`)
		outer := dataMap(t, result, "media")
		inner := outer["data"].(map[string]interface{})
		if inner["ownerId"] != "owner" {
			t.Errorf("unexpected owner: %v", inner)
		}
	})

	t.Run("repository error surfaces", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.GetErr = errBoom

		result := runGQL(t, f, userCtx("u1"), `{ media(mediaId: "m1") { success } }`)
		mustContainError(t, result, "boom")
	})

	t.Run("unauthenticated request", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, context.Background(), `{ media(mediaId: "m1") { success } }`)
		mustContainError(t, result, "x-user-id")
	})
}

func TestQueryListMedia(t *testing.T) {
	t.Run("defaults to requesting user and no next page", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.ListResults = []*domain.Media{testutil.NewMedia("m1", "u1")}

		result := runGQL(t, f, userCtx("u1"), `{ listMedia(ownerId: "u1", limit: 10) { success data { paginationInfo { totalItems currentPage nextPage } items { mediaId } } } }`)
		outer := dataMap(t, result, "listMedia")
		inner := outer["data"].(map[string]interface{})
		page := inner["paginationInfo"].(map[string]interface{})
		if page["totalItems"] != 1 || page["currentPage"] != 1 {
			t.Errorf("unexpected pagination: %v", page)
		}
		if page["nextPage"] != nil {
			t.Errorf("nextPage = %v, want nil", page["nextPage"])
		}
		items := inner["items"].([]interface{})
		if len(items) != 1 || items[0].(map[string]interface{})["mediaId"] != "m1" {
			t.Errorf("unexpected items: %v", items)
		}
		if calls := f.mediaRepo.ListOwnerCalls; len(calls) != 1 || calls[0].OwnerID != "u1" || calls[0].Limit != 10 {
			t.Errorf("ListOwnerCalls = %+v", calls)
		}
	})

	t.Run("cursor sets nextPage", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.ListResults = []*domain.Media{testutil.NewMedia("m2", "u1")}
		f.mediaRepo.ListCursor = "cursor-1"

		result := runGQL(t, f, userCtx("u1"), `{ listMedia(ownerId: "u1", cursor: "cursor-1") { success data { paginationInfo { nextPage } } } }`)
		outer := dataMap(t, result, "listMedia")
		page := outer["data"].(map[string]interface{})["paginationInfo"].(map[string]interface{})
		if page["nextPage"] != 2 {
			t.Errorf("nextPage = %v, want 2", page["nextPage"])
		}
		if calls := f.mediaRepo.ListOwnerCalls; calls[0].Cursor != "cursor-1" {
			t.Errorf("cursor not forwarded: %+v", calls)
		}
	})

	t.Run("non admin cannot list another owner", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, userCtx("u1"), `{ listMedia(ownerId: "someone-else") { success } }`)
		mustContainError(t, result, "authentication required")
	})

	t.Run("admin may list another owner", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.ListResults = []*domain.Media{testutil.NewMedia("m1", "other")}

		result := runGQL(t, f, adminCtx("boss"), `{ listMedia(ownerId: "other", statusFilter: "READY") { success data { items { mediaId } } } }`)
		outer := dataMap(t, result, "listMedia")
		if len(outer["data"].(map[string]interface{})["items"].([]interface{})) != 1 {
			t.Errorf("unexpected data: %v", outer)
		}
		if calls := f.mediaRepo.ListOwnerCalls; calls[0].OwnerID != "other" {
			t.Errorf("owner not forwarded: %+v", calls)
		}
	})

	t.Run("repository error surfaces", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.ListErr = errBoom

		result := runGQL(t, f, userCtx("u1"), `{ listMedia(ownerId: "u1") { success } }`)
		mustContainError(t, result, "boom")
	})
}

func TestQueryUploadStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := newFixture(t, 10)
		session := testutil.NewUploadSession("up1", "m1", "u1")
		session.CompletedParts = []domain.UploadPart{{PartNumber: 1, ETag: "etag-1"}}
		f.uploadRepo.Store["up1"] = session

		result := runGQL(t, f, userCtx("u1"), `{ uploadStatus(uploadId: "up1") { success data { uploadId status totalParts completedParts missingParts expiresAt } } }`)
		outer := dataMap(t, result, "uploadStatus")
		inner := outer["data"].(map[string]interface{})
		if inner["uploadId"] != "up1" || inner["status"] != string(domain.UploadStatusUploading) {
			t.Errorf("unexpected status payload: %v", inner)
		}
		if inner["totalParts"] != 2 || inner["completedParts"] != 1 {
			t.Errorf("unexpected parts: %v", inner)
		}
		missing := inner["missingParts"].([]interface{})
		if len(missing) != 1 || missing[0] != 2 {
			t.Errorf("missingParts = %v, want [2]", missing)
		}
		if inner["expiresAt"].(float64) <= 0 {
			t.Errorf("expiresAt = %v", inner["expiresAt"])
		}
	})

	t.Run("other user's session rejected", func(t *testing.T) {
		f := newFixture(t, 10)
		f.uploadRepo.Store["up1"] = testutil.NewUploadSession("up1", "m1", "owner")

		result := runGQL(t, f, userCtx("intruder"), `{ uploadStatus(uploadId: "up1") { success } }`)
		mustContainError(t, result, domain.ErrUnauthorized.Error())
	})

	t.Run("missing session", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, userCtx("u1"), `{ uploadStatus(uploadId: "nope") { success } }`)
		mustContainError(t, result, domain.ErrUploadSessionNotFound.Error())
	})
}

func TestQueryDownloadUrl(t *testing.T) {
	t.Run("success for owner", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "u1")

		result := runGQL(t, f, userCtx("u1"), `{ downloadUrl(mediaId: "m1", expirySeconds: 600) { success data { url expiresAt contentType } } }`)
		outer := dataMap(t, result, "downloadUrl")
		inner := outer["data"].(map[string]interface{})
		if inner["url"] == "" || inner["contentType"] != "image/jpeg" {
			t.Errorf("unexpected payload: %v", inner)
		}
		if inner["expiresAt"].(float64) <= 0 {
			t.Errorf("expiresAt = %v", inner["expiresAt"])
		}
	})

	t.Run("non owner is rejected", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "owner")

		result := runGQL(t, f, userCtx("intruder"), `{ downloadUrl(mediaId: "m1") { success } }`)
		mustContainError(t, result, domain.ErrUnauthorized.Error())
	})

	t.Run("quarantined media is rejected", func(t *testing.T) {
		f := newFixture(t, 10)
		m := testutil.NewMedia("m1", "u1")
		m.Status = domain.MediaStatusQuarantined
		f.mediaRepo.Store["m1"] = m

		result := runGQL(t, f, userCtx("u1"), `{ downloadUrl(mediaId: "m1") { success } }`)
		mustContainError(t, result, domain.ErrMediaQuarantined.Error())
	})

	t.Run("rate limited", func(t *testing.T) {
		f := newFixture(t, 0)
		f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "u1")

		result := runGQL(t, f, userCtx("u1"), `{ downloadUrl(mediaId: "m1") { success } }`)
		mustContainError(t, result, domain.ErrRateLimitExceeded.Error())
	})
}

func TestQueryQuota(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := newFixture(t, 10)
		f.quotaRepo.Usage = &domain.QuotaUsage{UsedBytes: 1000, QuotaBytes: 5000, ActiveUploads: 1, MaxConcurrent: 3}

		result := runGQL(t, f, userCtx("u1"), `{ quota { success data { usedBytes quotaBytes remainingBytes activeUploads maxConcurrentUploads } } }`)
		outer := dataMap(t, result, "quota")
		inner := outer["data"].(map[string]interface{})
		if inner["usedBytes"] != float64(1000) || inner["quotaBytes"] != float64(5000) {
			t.Errorf("unexpected quota: %v", inner)
		}
		if inner["remainingBytes"] != float64(4000) {
			t.Errorf("remainingBytes = %v", inner["remainingBytes"])
		}
		if inner["activeUploads"] != 1 || inner["maxConcurrentUploads"] != 3 {
			t.Errorf("unexpected concurrency: %v", inner)
		}
	})

	t.Run("repository error surfaces", func(t *testing.T) {
		f := newFixture(t, 10)
		f.quotaRepo.GetErr = errBoom

		result := runGQL(t, f, userCtx("u1"), `{ quota { success } }`)
		mustContainError(t, result, "boom")
	})
}

func TestMutationCreateUploadSession(t *testing.T) {
	t.Run("validation error", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, userCtx("u1"),
			`mutation { createUploadSession(input: {fileName: "evil.exe", contentType: "application/x-msdownload", size: 100}) { success } }`)
		mustContainError(t, result, domain.ErrInvalidFileType.Error())
	})

	t.Run("success", func(t *testing.T) {
		f := newFixture(t, 10)
		query := `mutation { createUploadSession(input: {fileName: "photo.jpg", contentType: "image/jpeg", size: 10485760, idempotencyKey: "idem-1"}) { success statusCode data { mediaId uploadId s3UploadId partSize totalParts presignedParts { partNumber presignedUrl } expiresAt } } }`

		result := runGQL(t, f, userCtx("u1"), query)
		outer := dataMap(t, result, "createUploadSession")
		if outer["statusCode"] != 201 {
			t.Errorf("statusCode = %v, want 201", outer["statusCode"])
		}
		inner := outer["data"].(map[string]interface{})
		if inner["mediaId"] == "" || inner["uploadId"] == "" || inner["s3UploadId"] == "" {
			t.Errorf("ids missing: %v", inner)
		}
		if totalParts, ok := inner["totalParts"].(int); !ok || totalParts < 1 {
			t.Errorf("totalParts = %v", inner["totalParts"])
		}
		parts := inner["presignedParts"].([]interface{})
		if len(parts) == 0 {
			t.Error("expected presigned parts")
		}
		if len(f.mediaRepo.Created) != 1 || len(f.uploadRepo.Created) != 1 {
			t.Error("expected media + upload session persisted")
		}
	})

	t.Run("idempotency conflict", func(t *testing.T) {
		f := newFixture(t, 10)
		f.cache.IdemExisting["u1:idem-1"] = true

		result := runGQL(t, f, userCtx("u1"),
			`mutation { createUploadSession(input: {fileName: "photo.jpg", contentType: "image/jpeg", size: 1000, idempotencyKey: "idem-1"}) { success } }`)
		mustContainError(t, result, domain.ErrIdempotencyConflict.Error())
	})
}

func TestMutationCompleteUpload(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := newFixture(t, 10)
		seedCompletedUpload(t, f, "up1", "m1", "u1")

		result := runGQL(t, f, userCtx("u1"),
			`mutation { completeUpload(input: {uploadId: "up1", parts: [{partNumber: 1, eTag: "etag-1"}]}) { success statusCode data { mediaId status } } }`)
		outer := dataMap(t, result, "completeUpload")
		inner := outer["data"].(map[string]interface{})
		if inner["mediaId"] != "m1" || inner["status"] != string(domain.MediaStatusUploaded) {
			t.Errorf("unexpected payload: %v", inner)
		}
		if len(f.outboxRepo.Created) != 1 {
			t.Error("expected outbox event")
		}
	})

	t.Run("missing session", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, userCtx("u1"), `mutation { completeUpload(input: {uploadId: "missing", parts: [{partNumber: 1, eTag: "e"}]}) { success } }`)
		mustContainError(t, result, domain.ErrUploadSessionNotFound.Error())
	})

	t.Run("malformed parts entries are tolerated", func(t *testing.T) {
		f := newFixture(t, 10)
		_, err := f.h.resolveCompleteUpload(gql.ResolveParams{
			Context: userCtx("u1"),
			Args: map[string]interface{}{
				"input": map[string]interface{}{
					"uploadId": "missing",
					"parts": []interface{}{
						"not-a-map",
						map[string]interface{}{"partNumber": float64(2), "eTag": "e"},
					},
				},
			},
		})
		if err == nil || !strings.Contains(err.Error(), domain.ErrUploadSessionNotFound.Error()) {
			t.Fatalf("err = %v, want upload session not found", err)
		}
	})

	t.Run("session of another user", func(t *testing.T) {
		f := newFixture(t, 10)
		session := testutil.NewUploadSession("up1", "m1", "owner")
		f.uploadRepo.Store["up1"] = session

		result := runGQL(t, f, userCtx("intruder"),
			`mutation { completeUpload(input: {uploadId: "up1", parts: [{partNumber: 1, eTag: "e"}]}) { success } }`)
		mustContainError(t, result, domain.ErrUnauthorized.Error())
	})

	t.Run("expired session", func(t *testing.T) {
		f := newFixture(t, 10)
		session := testutil.NewUploadSession("up1", "m1", "u1")
		session.ExpiresAt = time.Now().Add(-time.Minute)
		f.uploadRepo.Store["up1"] = session

		result := runGQL(t, f, userCtx("u1"),
			`mutation { completeUpload(input: {uploadId: "up1", parts: [{partNumber: 1, eTag: "e"}]}) { success } }`)
		mustContainError(t, result, domain.ErrUploadExpired.Error())
	})

	t.Run("size mismatch fails", func(t *testing.T) {
		f := newFixture(t, 10)
		seedCompletedUpload(t, f, "up1", "m1", "u1")
		media := f.mediaRepo.Store["m1"]
		media.Size = 999999

		result := runGQL(t, f, userCtx("u1"),
			`mutation { completeUpload(input: {uploadId: "up1", parts: [{partNumber: 1, eTag: "etag-1"}]}) { success } }`)
		mustContainError(t, result, domain.ErrChecksumMismatch.Error())
	})

	t.Run("magic bytes mismatch fails", func(t *testing.T) {
		f := newFixture(t, 10)
		seedCompletedUpload(t, f, "up1", "m1", "u1")
		wrong := make([]byte, 1024)
		copy(wrong, []byte("not-a-jpeg-file"))
		f.storage.SetObject(f.uploadRepo.Store["up1"].ObjectKey, wrong)

		result := runGQL(t, f, userCtx("u1"),
			`mutation { completeUpload(input: {uploadId: "up1", parts: [{partNumber: 1, eTag: "etag-1"}]}) { success } }`)
		mustContainError(t, result, domain.ErrInvalidMagicBytes.Error())
	})
}

// seedCompletedUpload prepares storage, media and upload session so that the
// completeUpload use case can run end to end against the in-memory fakes.
func seedCompletedUpload(t *testing.T, f *fixture, uploadID, mediaID, userID string) {
	t.Helper()
	now := time.Now().UTC()
	media := &domain.Media{
		MediaID: mediaID, OwnerID: userID, FileName: "photo.jpg", ContentType: "image/jpeg",
		MediaType: domain.MediaTypeImage, Size: 1024,
		ObjectKey: "users/" + userID + "/media/" + mediaID + "/original/photo.jpg",
		Status:    domain.MediaStatusUploading, CreatedAt: now, UpdatedAt: now,
	}
	s3ID, err := f.storage.CreateMultipartUpload(context.Background(), media.ObjectKey, "image/jpeg")
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}
	jpeg := make([]byte, 1024)
	jpeg[0], jpeg[1], jpeg[2] = 0xFF, 0xD8, 0xFF
	f.storage.SetObject(media.ObjectKey, jpeg)
	f.mediaRepo.Store[mediaID] = media
	f.uploadRepo.Store[uploadID] = &domain.UploadSession{
		UploadID: uploadID, MediaID: mediaID, UserID: userID, S3UploadID: s3ID,
		ObjectKey: media.ObjectKey, TotalParts: 1, PartSize: 1024,
		Status: domain.UploadStatusUploading, ExpiresAt: now.Add(time.Hour),
		CreatedAt: now, UpdatedAt: now,
	}
}

func TestMutationAbortUpload(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := newFixture(t, 10)
		f.uploadRepo.Store["up1"] = testutil.NewUploadSession("up1", "m1", "u1")
		f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "u1")

		result := runGQL(t, f, userCtx("u1"), `mutation { abortUpload(uploadId: "up1") { success statusCode data } }`)
		outer := dataMap(t, result, "abortUpload")
		if outer["statusCode"] != 200 || outer["data"] != "Upload aborted" {
			t.Errorf("unexpected payload: %v", outer)
		}
		if f.quotaRepo.Decrements == 0 {
			t.Error("expected quota slot release")
		}
	})

	t.Run("missing session", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, userCtx("u1"), `mutation { abortUpload(uploadId: "nope") { success } }`)
		mustContainError(t, result, domain.ErrUploadSessionNotFound.Error())
	})

	t.Run("session of another user", func(t *testing.T) {
		f := newFixture(t, 10)
		f.uploadRepo.Store["up1"] = testutil.NewUploadSession("up1", "m1", "owner")

		result := runGQL(t, f, userCtx("intruder"), `mutation { abortUpload(uploadId: "up1") { success } }`)
		mustContainError(t, result, domain.ErrUnauthorized.Error())
	})
}

func TestMutationDeleteMedia(t *testing.T) {
	t.Run("owner deletes ready media", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "u1")

		result := runGQL(t, f, userCtx("u1"), `mutation { deleteMedia(mediaId: "m1") { success statusCode data } }`)
		outer := dataMap(t, result, "deleteMedia")
		if outer["statusCode"] != 200 || outer["data"] != "Media deleted" {
			t.Errorf("unexpected payload: %v", outer)
		}
		if len(f.outboxRepo.Created) != 1 {
			t.Error("expected delete outbox event")
		}
		calls := f.mediaRepo.UpdateStatuses
		if len(calls) != 1 || calls[0].Next != domain.MediaStatusDeleting {
			t.Errorf("unexpected status calls: %+v", calls)
		}
	})

	t.Run("admin deletes foreign media", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "owner")

		result := runGQL(t, f, adminCtx("boss"), `mutation { deleteMedia(mediaId: "m1", idempotencyKey: "k1") { success } }`)
		dataMap(t, result, "deleteMedia")
	})

	t.Run("non owner is rejected", func(t *testing.T) {
		f := newFixture(t, 10)
		f.mediaRepo.Store["m1"] = testutil.NewMedia("m1", "owner")

		result := runGQL(t, f, userCtx("intruder"), `mutation { deleteMedia(mediaId: "m1") { success } }`)
		mustContainError(t, result, domain.ErrUnauthorized.Error())
	})

	t.Run("invalid state transition", func(t *testing.T) {
		f := newFixture(t, 10)
		m := testutil.NewMedia("m1", "u1")
		m.Status = domain.MediaStatusPending
		f.mediaRepo.Store["m1"] = m

		result := runGQL(t, f, userCtx("u1"), `mutation { deleteMedia(mediaId: "m1") { success } }`)
		mustContainError(t, result, domain.ErrInvalidStateTransition.Error())
	})

	t.Run("missing media", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, userCtx("u1"), `mutation { deleteMedia(mediaId: "nope") { success } }`)
		mustContainError(t, result, domain.ErrMediaNotFound.Error())
	})
}

func TestDLQQueries(t *testing.T) {
	t.Run("dlqTopics requires admin", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, userCtx("u1"), `{ dlqTopics }`)
		mustContainError(t, result, "admin privileges")
	})

	t.Run("dlqTopics for admin", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, adminCtx("boss"), `{ dlqTopics }`)
		if len(result.Errors) > 0 {
			t.Fatalf("unexpected errors: %v", result.Errors)
		}
		topics := result.Data.(map[string]interface{})["dlqTopics"].([]interface{})
		if len(topics) != 11 {
			t.Fatalf("expected 11 topics, got %d", len(topics))
		}
	})

	t.Run("dlqStats requires admin", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, userCtx("u1"), `{ dlqStats(topics: ["media.ready"]) { success } }`)
		mustContainError(t, result, "admin privileges")
	})

	t.Run("dlqReplay requires admin", func(t *testing.T) {
		f := newFixture(t, 10)
		result := runGQL(t, f, userCtx("u1"), `mutation { dlqReplay(topic: "media.ready") { success } }`)
		mustContainError(t, result, "admin privileges")
	})

	t.Run("dlqReplay missing topic", func(t *testing.T) {
		f := newFixture(t, 10)
		_, err := f.h.resolveDLQReplay(gql.ResolveParams{Context: adminCtx("boss"), Args: map[string]interface{}{}})
		if err == nil || !strings.Contains(err.Error(), "topic is required") {
			t.Fatalf("err = %v, want topic required", err)
		}
	})

	t.Run("dlqReplay with unreachable broker replays nothing", func(t *testing.T) {
		f := newFixture(t, 10)
		// Call the resolver directly with a background context. Routing this
		// through gql.Do with a ~1s timeout races graphql-go's ctx.Done()
		// select against the Kafka dial timeout and yields a spurious
		// "context deadline exceeded" instead of the resolver's real result.
		// A short deadline makes the Kafka dial to the unreachable broker
		// give up fast instead of blocking ~10s. The resolver swallows the
		// error and returns a successful payload with replayedCount 0.
		ctx, cancel := context.WithTimeout(adminCtx("boss"), 3*time.Second)
		defer cancel()
		out, err := f.h.resolveDLQReplay(gql.ResolveParams{
			Context: ctx,
			Args:    map[string]interface{}{"topic": "media.ready", "maxMessages": 1},
		})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		payload := out.(map[string]interface{})
		if payload["success"] != true || payload["statusCode"] != 200 {
			t.Errorf("payload = %v", payload)
		}
		d := payload["data"].(map[string]interface{})
		if d["success"] != true || d["replayedCount"] != 0 {
			t.Errorf("unexpected replay data: %v", d)
		}
		if errs, ok := d["errors"].([]string); !ok || len(errs) != 0 {
			t.Errorf("expected no errors: %v", d["errors"])
		}
	})

	t.Run("dlqReplay defaults maxMessages", func(t *testing.T) {
		f := newFixture(t, 10)
		ctx, cancel := context.WithTimeout(adminCtx("boss"), time.Second)
		defer cancel()

		out, err := f.h.resolveDLQReplay(gql.ResolveParams{
			Context: ctx,
			Args:    map[string]interface{}{"topic": "media.ready"},
		})
		if err != nil {
			t.Fatalf("resolveDLQReplay: %v", err)
		}
		payload := out.(map[string]interface{})
		if payload["success"] != true || payload["statusCode"] != 200 {
			t.Errorf("unexpected payload: %v", payload)
		}
		inner := payload["data"].(map[string]interface{})
		if inner["replayedCount"] != 0 || inner["success"] != true {
			t.Errorf("unexpected data: %v", inner)
		}
	})
}

func TestMutationRenewPresigned(t *testing.T) {
	t.Run("missing input", func(t *testing.T) {
		f := newFixture(t, 10)
		_, err := f.h.resolveRenewPresigned(gql.ResolveParams{
			Context: userCtx("u1"),
			Args:    map[string]interface{}{},
		})
		if err == nil || !strings.Contains(err.Error(), "invalid input") {
			t.Fatalf("err = %v, want invalid input", err)
		}
	})

	t.Run("missing uploadId", func(t *testing.T) {
		f := newFixture(t, 10)
		_, err := f.h.resolveRenewPresigned(gql.ResolveParams{
			Context: userCtx("u1"),
			Args: map[string]interface{}{
				"input": map[string]interface{}{"expirySeconds": 600},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "uploadId is required") {
			t.Fatalf("err = %v, want uploadId required", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		f := newFixture(t, 10)
		f.uploadRepo.Store["up1"] = testutil.NewUploadSession("up1", "m1", "u1")

		result := runGQL(t, f, userCtx("u1"),
			`mutation { renewPresigned(input: {uploadId: "up1", expirySeconds: 900}) { success data { uploadId s3UploadId partSize totalParts presignedParts { partNumber presignedUrl } expiresAt } } }`)
		outer := dataMap(t, result, "renewPresigned")
		inner := outer["data"].(map[string]interface{})
		if inner["uploadId"] != "up1" || inner["s3UploadId"] == "" {
			t.Errorf("unexpected payload: %v", inner)
		}
		parts := inner["presignedParts"].([]interface{})
		if len(parts) != 2 {
			t.Errorf("expected 2 parts, got %d", len(parts))
		}
		if _, ok := f.uploadRepo.ExpiryUpdates["up1"]; !ok {
			t.Error("expected expiry update")
		}
	})

	t.Run("session of another user", func(t *testing.T) {
		f := newFixture(t, 10)
		f.uploadRepo.Store["up1"] = testutil.NewUploadSession("up1", "m1", "owner")

		result := runGQL(t, f, userCtx("intruder"),
			`mutation { renewPresigned(input: {uploadId: "up1"}) { success } }`)
		mustContainError(t, result, domain.ErrUnauthorized.Error())
	})

	t.Run("session not uploading", func(t *testing.T) {
		f := newFixture(t, 10)
		session := testutil.NewUploadSession("up1", "m1", "u1")
		session.Status = domain.UploadStatusCompleted
		f.uploadRepo.Store["up1"] = session

		result := runGQL(t, f, userCtx("u1"),
			`mutation { renewPresigned(input: {uploadId: "up1"}) { success } }`)
		mustContainError(t, result, "cannot renew presigned URLs")
	})

	t.Run("expired session", func(t *testing.T) {
		f := newFixture(t, 10)
		session := testutil.NewUploadSession("up1", "m1", "u1")
		session.ExpiresAt = time.Now().Add(-time.Minute)
		f.uploadRepo.Store["up1"] = session

		result := runGQL(t, f, userCtx("u1"),
			`mutation { renewPresigned(input: {uploadId: "up1"}) { success } }`)
		mustContainError(t, result, domain.ErrUploadExpired.Error())
	})
}

func TestArgHelpers(t *testing.T) {
	t.Run("argString", func(t *testing.T) {
		if got := argString(map[string]interface{}{"a": "x"}, "a"); got != "x" {
			t.Errorf("got %q", got)
		}
		if got := argString(map[string]interface{}{"a": 1}, "a"); got != "" {
			t.Errorf("got %q for non-string", got)
		}
		if got := argString(nil, "missing"); got != "" {
			t.Errorf("got %q for missing", got)
		}
	})

	t.Run("argInt", func(t *testing.T) {
		cases := map[string]int{"int": 7, "int32": 7, "int64": 7, "float64": 7, "missing": 0, "other": 0}
		args := map[string]interface{}{
			"int": 7, "int32": int32(7), "int64": int64(7), "float64": float64(7), "other": "7",
		}
		for key, want := range cases {
			if got := argInt(args, key); got != want {
				t.Errorf("argInt(%s) = %d, want %d", key, got, want)
			}
		}
	})

	t.Run("argFloat", func(t *testing.T) {
		args := map[string]interface{}{
			"f64": 1.5, "f32": float32(1.5), "i": 2, "i64": int64(3), "other": "x",
		}
		if got := argFloat(args, "f64"); got != 1.5 {
			t.Errorf("f64 = %v", got)
		}
		if got := argFloat(args, "f32"); got != 1.5 {
			t.Errorf("f32 = %v", got)
		}
		if got := argFloat(args, "i"); got != 2 {
			t.Errorf("i = %v", got)
		}
		if got := argFloat(args, "i64"); got != 3 {
			t.Errorf("i64 = %v", got)
		}
		if got := argFloat(args, "other"); got != 0 {
			t.Errorf("other = %v", got)
		}
		if got := argFloat(nil, "missing"); got != 0 {
			t.Errorf("missing = %v", got)
		}
	})

	t.Run("intSlice", func(t *testing.T) {
		out := intSlice([]int{1, 2, 3})
		if len(out) != 3 || out[0] != 1 || out[2] != 3 {
			t.Errorf("out = %v", out)
		}
		if out := intSlice(nil); len(out) != 0 {
			t.Errorf("out = %v", out)
		}
	})
}

func TestMapMedia(t *testing.T) {
	if got := mapMedia(nil, nil); got != nil {
		t.Fatalf("mapMedia(nil) = %v", got)
	}

	m := testutil.NewMedia("m1", "u1")
	versions := []*domain.MediaVersion{
		{VersionType: "original", ObjectKey: "k", ContentType: "image/jpeg", Size: 10, Width: 1, Height: 2, DurationMS: 3},
		nil,
	}
	out := mapMedia(m, versions)
	if out["mediaId"] != "m1" || out["ownerId"] != "u1" {
		t.Errorf("out = %v", out)
	}
	if out["size"] != float64(1024) {
		t.Errorf("size = %v", out["size"])
	}
	if out["createdAt"].(float64) <= 0 || out["updatedAt"].(float64) <= 0 {
		t.Errorf("timestamps = %v", out)
	}
	list := out["versions"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("versions = %v", list)
	}
	v := list[0].(map[string]interface{})
	if v["versionType"] != "original" || v["durationMs"] != float64(3) {
		t.Errorf("version = %v", v)
	}
}

func TestRequireUserAndAdmin(t *testing.T) {
	f := newFixture(t, 10)

	if _, err := f.h.requireUser(nil); err == nil {
		t.Error("nil ctx should fail")
	} else if !strings.Contains(err.Error(), "x-user-id") {
		t.Errorf("err = %v", err)
	}
	if _, err := f.h.requireUser(context.Background()); err == nil {
		t.Error("empty ctx should fail")
	}
	if userID, err := f.h.requireUser(userCtx("u1")); err != nil || userID != "u1" {
		t.Errorf("userID = %q, err = %v", userID, err)
	}

	if err := f.h.requireAdmin(nil); err == nil {
		t.Error("nil ctx should fail")
	}
	if err := f.h.requireAdmin(context.Background()); err == nil {
		t.Error("anonymous ctx should fail")
	}
	if err := f.h.requireAdmin(userCtx("u1")); err == nil {
		t.Error("regular user should fail")
	} else if !strings.Contains(err.Error(), "admin privileges") {
		t.Errorf("err = %v", err)
	}
	if err := f.h.requireAdmin(adminCtx("boss")); err != nil {
		t.Errorf("admin ctx failed: %v", err)
	}
}

func TestIsUserAdmin(t *testing.T) {
	f := newFixture(t, 10)
	if f.h.isUserAdmin(nil) {
		t.Error("nil ctx should not be admin")
	}
	if f.h.isUserAdmin(userCtx("u1")) {
		t.Error("plain user should not be admin")
	}
	if !f.h.isUserAdmin(adminCtx("boss")) {
		t.Error("admin ctx should be admin")
	}
	if !f.h.isUserAdmin(context.WithValue(userCtx("u1"), "user_role", "Admin")) {
		t.Error("role match should be case-insensitive")
	}
}

func TestResolveDLQStatsTopicsParsing(t *testing.T) {
	f := newFixture(t, 10)
	_, err := f.h.resolveDLQStats(gql.ResolveParams{Context: userCtx("u1")})
	if err == nil || !strings.Contains(err.Error(), "admin privileges") {
		t.Fatalf("err = %v, want admin forbidden", err)
	}
}
