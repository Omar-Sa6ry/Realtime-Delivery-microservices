package media

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

func TestGetMedia_GetByID(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	stored := testutil.NewMedia("media-1", "user-1")
	if err := repo.Create(testutil.Context("user-1", "trace-1"), stored); err != nil {
		t.Fatalf("seed media: %v", err)
	}
	uc := NewGetMediaUseCase(repo, testutil.NewMockVersionRepository())

	m, err := uc.GetByID(testutil.Context("user-1", "trace-1"), "media-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if m.MediaID != "media-1" {
		t.Fatalf("unexpected media: %+v", m)
	}

	repo.GetErr = errors.New("ddb down")
	if _, err := uc.GetByID(testutil.Context("user-1", "trace-1"), "other"); err == nil {
		t.Fatal("expected repository error")
	}
}

func TestGetMedia_ExecuteOwnerSeesVersions(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	versions := testutil.NewMockVersionRepository()
	ctx := testutil.Context("user-1", "trace-1")
	if err := repo.Create(ctx, testutil.NewMedia("media-1", "user-1")); err != nil {
		t.Fatalf("seed media: %v", err)
	}
	versions.Versions["media-1"] = []*domain.MediaVersion{
		{VersionID: "v1", MediaID: "media-1", VersionType: domain.VersionTypeThumbnail, ObjectKey: "k/thumb.webp", ContentType: "image/webp"},
	}

	out, err := NewGetMediaUseCase(repo, versions).Execute(ctx, "user-1", "media-1")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Media == nil || out.Media.MediaID != "media-1" {
		t.Fatalf("unexpected media: %+v", out.Media)
	}
	if len(out.Versions) != 1 || out.Versions[0].VersionID != "v1" {
		t.Fatalf("unexpected versions: %+v", out.Versions)
	}
}

func TestGetMedia_ExecuteUnauthorizedForOtherOwner(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	ctx := testutil.Context("user-2", "trace-1")
	if err := repo.Create(ctx, testutil.NewMedia("media-1", "user-1")); err != nil {
		t.Fatalf("seed media: %v", err)
	}
	_, err := NewGetMediaUseCase(repo, testutil.NewMockVersionRepository()).Execute(ctx, "user-2", "media-1")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestGetMedia_ExecuteNotFound(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	_, err := NewGetMediaUseCase(repo, testutil.NewMockVersionRepository()).Execute(
		testutil.Context("user-1", "trace-1"), "user-1", "missing")
	if !errors.Is(err, domain.ErrMediaNotFound) {
		t.Fatalf("err = %v, want ErrMediaNotFound", err)
	}
}

func TestGetMedia_ExecuteForMediaVersionRepoErrorTolerated(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	versions := testutil.NewMockVersionRepository()
	versions.ListErr = errors.New("ddb down")

	out, err := NewGetMediaUseCase(repo, versions).ExecuteForMedia(
		testutil.Context("user-1", "trace-1"), testutil.NewMedia("media-1", "user-1"))
	if err != nil {
		t.Fatalf("ExecuteForMedia must tolerate version repo errors: %v", err)
	}
	if out.Versions == nil || len(out.Versions) != 0 {
		t.Fatalf("versions = %v, want empty slice", out.Versions)
	}
}

func TestListMedia_LimitNormalization(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		wantLimit int
	}{
		{"zero defaults to 20", 0, 20},
		{"negative defaults to 20", -1, 20},
		{"above max defaults to 20", 101, 20},
		{"max accepted", 100, 100},
		{"custom limit kept", 50, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.NewMockMediaRepository()
			repo.ListResults = []*domain.Media{testutil.NewMedia("media-1", "user-1")}
			repo.ListCursor = "next-1"
			uc := NewListMediaUseCase(repo)

			items, cursor, err := uc.Execute(testutil.Context("user-1", "trace-1"), ListMediaInput{
				OwnerID: "user-1", Limit: tt.limit, Cursor: "cur-1",
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if len(items) != 1 || cursor != "next-1" {
				t.Fatalf("items = %d, cursor = %q", len(items), cursor)
			}
			if len(repo.ListOwnerCalls) != 1 {
				t.Fatalf("list calls = %d", len(repo.ListOwnerCalls))
			}
			call := repo.ListOwnerCalls[0]
			if call.OwnerID != "user-1" || call.Limit != tt.wantLimit || call.Cursor != "cur-1" {
				t.Fatalf("unexpected list call: %+v", call)
			}
		})
	}
}

func TestListMedia_RepositoryError(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	repo.ListErr = errors.New("ddb down")
	_, _, err := NewListMediaUseCase(repo).Execute(testutil.Context("user-1", "trace-1"), ListMediaInput{OwnerID: "user-1"})
	if err == nil || !strings.Contains(err.Error(), "ddb down") {
		t.Fatalf("err = %v, want repository error", err)
	}
}

func TestDeleteMedia_OwnerSuccess(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	outbox := testutil.NewMockOutboxRepository()
	quota := testutil.NewMockQuotaRepository()
	cache := testutil.NewMockCache()
	ctx := testutil.Context("user-1", "trace-1")
	if err := repo.Create(ctx, testutil.NewMedia("media-1", "user-1")); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	uc := NewDeleteMediaUseCase(repo, outbox, quota, cache)
	if err := uc.Execute(ctx, "user-1", "media-1", ""); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(repo.UpdateStatuses) != 1 {
		t.Fatalf("status updates = %d, want 1", len(repo.UpdateStatuses))
	}
	call := repo.UpdateStatuses[0]
	if call.Current != domain.MediaStatusReady || call.Next != domain.MediaStatusDeleting {
		t.Fatalf("unexpected status call: %+v", call)
	}
	if len(outbox.Created) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(outbox.Created))
	}
	ev := outbox.Created[0]
	if ev.EventType != kafka.TopicDeleteRequested || ev.AggregateID != "media-1" || ev.Status != domain.OutboxStatusPending {
		t.Fatalf("unexpected outbox event: %+v", ev)
	}
	var payload struct {
		MediaID   string `json:"mediaId"`
		UserID    string `json:"userId"`
		ObjectKey string `json:"objectKey"`
	}
	testutil.DecodeEnvelope(t, ev.Payload, &payload)
	if payload.MediaID != "media-1" || payload.UserID != "user-1" || payload.ObjectKey == "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestDeleteMedia_AdminBypassesOwnership(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	outbox := testutil.NewMockOutboxRepository()
	ctx := testutil.Context("admin-1", "trace-1")
	if err := repo.Create(ctx, testutil.NewMedia("media-1", "user-1")); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	uc := NewDeleteMediaUseCase(repo, outbox, testutil.NewMockQuotaRepository(), testutil.NewMockCache())
	if err := uc.ExecuteWithAdmin(ctx, "admin-1", "media-1", "", true); err != nil {
		t.Fatalf("ExecuteWithAdmin: %v", err)
	}
	if len(repo.UpdateStatuses) != 1 || repo.UpdateStatuses[0].Next != domain.MediaStatusDeleting {
		t.Fatalf("status updates = %+v", repo.UpdateStatuses)
	}
}

func TestDeleteMedia_NonOwnerRejected(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	ctx := testutil.Context("user-2", "trace-1")
	if err := repo.Create(ctx, testutil.NewMedia("media-1", "user-1")); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	uc := NewDeleteMediaUseCase(repo, testutil.NewMockOutboxRepository(), testutil.NewMockQuotaRepository(), testutil.NewMockCache())
	err := uc.Execute(ctx, "user-2", "media-1", "")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if err := uc.ExecuteWithAdmin(ctx, "user-2", "media-1", "", false); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestDeleteMedia_NotFound(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	uc := NewDeleteMediaUseCase(repo, testutil.NewMockOutboxRepository(), testutil.NewMockQuotaRepository(), testutil.NewMockCache())
	err := uc.Execute(testutil.Context("user-1", "trace-1"), "user-1", "missing", "")
	if !errors.Is(err, domain.ErrMediaNotFound) {
		t.Fatalf("err = %v, want ErrMediaNotFound", err)
	}
}

func TestDeleteMedia_IdempotentReplayIsNoop(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	outbox := testutil.NewMockOutboxRepository()
	cache := testutil.NewMockCache()
	ctx := testutil.Context("user-1", "trace-1")
	if err := repo.Create(ctx, testutil.NewMedia("media-1", "user-1")); err != nil {
		t.Fatalf("seed media: %v", err)
	}
	cache.IdemExisting["user-1:delete:media-1"] = true

	uc := NewDeleteMediaUseCase(repo, outbox, testutil.NewMockQuotaRepository(), cache)
	if err := uc.Execute(ctx, "user-1", "media-1", "idem-1"); err != nil {
		t.Fatalf("replay must be a no-op success: %v", err)
	}
	if len(repo.UpdateStatuses) != 0 || len(outbox.Created) != 0 {
		t.Fatal("replay must not transition state or emit events")
	}
}

func TestDeleteMedia_IdempotencyStoreErrorContinues(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	outbox := testutil.NewMockOutboxRepository()
	cache := testutil.NewMockCache()
	cache.IdemErr = errors.New("redis down")
	ctx := testutil.Context("user-1", "trace-1")
	if err := repo.Create(ctx, testutil.NewMedia("media-1", "user-1")); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	uc := NewDeleteMediaUseCase(repo, outbox, testutil.NewMockQuotaRepository(), cache)
	if err := uc.Execute(ctx, "user-1", "media-1", "idem-1"); err != nil {
		t.Fatalf("Execute must continue on idempotency store error: %v", err)
	}
	if len(repo.UpdateStatuses) != 1 {
		t.Fatalf("status updates = %+v", repo.UpdateStatuses)
	}
}

func TestDeleteMedia_InvalidStateTransition(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	ctx := testutil.Context("user-1", "trace-1")
	deleted := testutil.NewMedia("media-1", "user-1")
	deleted.Status = domain.MediaStatusDeleted
	if err := repo.Create(ctx, deleted); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	uc := NewDeleteMediaUseCase(repo, testutil.NewMockOutboxRepository(), testutil.NewMockQuotaRepository(), testutil.NewMockCache())
	err := uc.Execute(ctx, "user-1", "media-1", "")
	if !errors.Is(err, domain.ErrInvalidStateTransition) {
		t.Fatalf("err = %v, want ErrInvalidStateTransition", err)
	}
	if len(repo.UpdateStatuses) != 0 {
		t.Fatal("no transition must be attempted for terminal states")
	}
}

func TestDeleteMedia_TransitionError(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	ctx := testutil.Context("user-1", "trace-1")
	if err := repo.Create(ctx, testutil.NewMedia("media-1", "user-1")); err != nil {
		t.Fatalf("seed media: %v", err)
	}
	repo.UpdateStatusErr = errors.New("conditional check failed")

	uc := NewDeleteMediaUseCase(repo, testutil.NewMockOutboxRepository(), testutil.NewMockQuotaRepository(), testutil.NewMockCache())
	err := uc.Execute(ctx, "user-1", "media-1", "")
	if err == nil || !strings.Contains(err.Error(), "transition to DELETING") {
		t.Fatalf("err = %v, want transition to DELETING error", err)
	}
}

func TestDeleteMedia_OutboxCreateErrorTolerated(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	outbox := testutil.NewMockOutboxRepository()
	outbox.CreateErr = errors.New("ddb down")
	ctx := testutil.Context("user-1", "trace-1")
	if err := repo.Create(ctx, testutil.NewMedia("media-1", "user-1")); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	uc := NewDeleteMediaUseCase(repo, outbox, testutil.NewMockQuotaRepository(), testutil.NewMockCache())
	if err := uc.Execute(ctx, "user-1", "media-1", ""); err != nil {
		t.Fatalf("outbox failure must be tolerated: %v", err)
	}
	if len(repo.UpdateStatuses) != 1 || repo.UpdateStatuses[0].Next != domain.MediaStatusDeleting {
		t.Fatalf("status updates = %+v", repo.UpdateStatuses)
	}
}

func TestDeleteMedia_IdempotencyWindowUsesTTL(t *testing.T) {
	repo := testutil.NewMockMediaRepository()
	cache := testutil.NewMockCache()
	ctx := testutil.Context("user-1", "trace-1")
	if err := repo.Create(ctx, testutil.NewMedia("media-1", "user-1")); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	uc := NewDeleteMediaUseCase(repo, testutil.NewMockOutboxRepository(), testutil.NewMockQuotaRepository(), cache)
	if err := uc.Execute(ctx, "user-1", "media-1", "idem-1"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(cache.IdemCalls) != 1 {
		t.Fatalf("idempotency calls = %d", len(cache.IdemCalls))
	}
	call := cache.IdemCalls[0]
	if call.UserID != "user-1" || call.Key != "delete:media-1" || call.TTL != 24*time.Hour {
		t.Fatalf("unexpected idempotency call: %+v", call)
	}
}
