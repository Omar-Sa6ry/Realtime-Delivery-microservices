package dynamodb

import (
	"context"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

func newVersionForTest(id, mediaID string, vtype domain.VersionType, createdAt time.Time) *domain.MediaVersion {
	return &domain.MediaVersion{
		VersionID:   id,
		MediaID:     mediaID,
		VersionType: vtype,
		ObjectKey:   "users/u1/media/" + mediaID + "/" + string(vtype) + "/x.bin",
		ContentType: "application/octet-stream",
		Size:        100,
		Checksum:    "sum-" + id,
		CreatedAt:   createdAt,
	}
}

func TestVersionRepositoryCreate(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewVersionRepository(newTestClient(t, f), testTable)

	v := newVersionForTest("v1", "m1", domain.VersionTypeOriginal, fixedTime(5))
	if err := repo.Create(context.Background(), v); err != nil {
		t.Fatalf("Create: %v", err)
	}
	body := decodePutItem(t, lastTargetRequest(t, f, "PutItem").Body)
	cases := map[string]string{
		"PK":          "MEDIA#m1",
		"SK":          "VERSION#original",
		"VersionID":   "v1",
		"MediaID":     "m1",
		"VersionType": "original",
		"ContentType": "application/octet-stream",
		"Checksum":    "sum-v1",
	}
	for attr, want := range cases {
		if got := avString(t, body.Item, attr); got != want {
			t.Errorf("%s = %q, want %q", attr, got, want)
		}
	}
	if got := avNumber(t, body.Item, "Size"); got != "100" {
		t.Errorf("Size = %q", got)
	}
}

func TestVersionRepositoryCreateError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("PutItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	err := NewVersionRepository(newTestClient(t, f), testTable).
		Create(context.Background(), newVersionForTest("v1", "m1", domain.VersionTypeOriginal, fixedTime(5)))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVersionRepositoryListByMedia(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewVersionRepository(newTestClient(t, f), testTable)
	ctx := context.Background()

	if err := repo.Create(ctx, newVersionForTest("v1", "m1", domain.VersionTypeOriginal, fixedTime(5))); err != nil {
		t.Fatalf("Create v1: %v", err)
	}
	if err := repo.Create(ctx, newVersionForTest("v2", "m1", domain.VersionTypeThumbnail, fixedTime(6))); err != nil {
		t.Fatalf("Create v2: %v", err)
	}
	if err := repo.Create(ctx, newVersionForTest("v3", "m2", domain.VersionTypeOriginal, fixedTime(7))); err != nil {
		t.Fatalf("Create v3: %v", err)
	}

	versions, err := repo.ListByMedia(ctx, "m1")
	if err != nil {
		t.Fatalf("ListByMedia: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2: %v", len(versions), versions)
	}
	if versions[0].VersionID != "v1" || versions[1].VersionID != "v2" {
		t.Fatalf("unexpected order: %s, %s", versions[0].VersionID, versions[1].VersionID)
	}
	if versions[0].VersionType != domain.VersionTypeOriginal || versions[1].VersionType != domain.VersionTypeThumbnail {
		t.Fatalf("unexpected types: %v", versions)
	}
	if versions[0].Checksum != "sum-v1" || versions[0].Size != 100 || versions[0].ObjectKey == "" {
		t.Fatalf("unexpected version: %+v", versions[0])
	}
	if !versions[0].CreatedAt.Equal(fixedTime(5)) {
		t.Fatalf("CreatedAt = %v", versions[0].CreatedAt)
	}

	none, err := repo.ListByMedia(ctx, "nobody")
	if err != nil {
		t.Fatalf("ListByMedia nobody: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no versions, got %v", none)
	}
}

func TestVersionRepositoryListByMediaError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("Query", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})

	_, err := NewVersionRepository(newTestClient(t, f), testTable).ListByMedia(context.Background(), "m1")
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB Query versions")
}

func TestVersionRepositoryListByMediaSkipsUnmarshalableItems(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetQueryOverride([]map[string]interface{}{
		{
			"PK":        map[string]interface{}{"S": "MEDIA#m1"},
			"SK":        map[string]interface{}{"S": "VERSION#original"},
			"VersionID": map[string]interface{}{"L": []interface{}{}},
		},
	})

	versions, err := NewVersionRepository(newTestClient(t, f), testTable).ListByMedia(context.Background(), "m1")
	if err != nil {
		t.Fatalf("ListByMedia: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("expected unmarshalable item skipped, got %v", versions)
	}
}
