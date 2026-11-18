package indexing

import (
	"context"
	"errors"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/domain/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/testutil"
)

func TestUpsertDelivery_Delegates(t *testing.T) {
	repo := testutil.NewFakeRepository()
	svc := NewService(repo)
	doc := search.DeliveryDocument{DeliveryID: "d1", Status: "PENDING", SourceVersion: 1}
	if err := svc.UpsertDelivery(context.Background(), doc); err != nil {
		t.Fatalf("UpsertDelivery: %v", err)
	}
	if len(repo.UpsertDeliveries) != 1 || repo.UpsertDeliveries[0].DeliveryID != "d1" {
		t.Errorf("recorded = %+v", repo.UpsertDeliveries)
	}

	repo.UpsertDeliveryErr = errors.New("err")
	if err := svc.UpsertDelivery(context.Background(), doc); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteDelivery_Delegates(t *testing.T) {
	repo := testutil.NewFakeRepository()
	svc := NewService(repo)
	if err := svc.DeleteDelivery(context.Background(), "d1"); err != nil {
		t.Fatal(err)
	}
	if len(repo.DeletedDeliveries) != 1 || repo.DeletedDeliveries[0] != "d1" {
		t.Errorf("deleted = %v", repo.DeletedDeliveries)
	}
	repo.DeleteDeliveryErr = errors.New("err")
	if err := svc.DeleteDelivery(context.Background(), "d1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpsertDriver_NoName_NoUserLookup(t *testing.T) {
	repo := testutil.NewFakeRepository()
	svc := NewService(repo)
	if err := svc.UpsertDriver(context.Background(), search.DriverDocument{DriverID: "d1"}); err != nil {
		t.Fatal(err)
	}
	if len(repo.UsersQueries) != 0 {
		t.Errorf("should not search users when Name empty, queries = %d", len(repo.UsersQueries))
	}
}

func TestUpsertDriver_NameMatchesUserID_ReplacedWithFullName(t *testing.T) {
	repo := testutil.NewFakeRepository()
	repo.UsersResult = search.SearchResult[search.UserDocument]{
		Items: []search.UserDocument{
			{ID: "someone-else", FirstName: "X", LastName: "Y"},
			{ID: "u1", FirstName: "John", LastName: "Doe"},
		},
	}
	svc := NewService(repo)
	if err := svc.UpsertDriver(context.Background(), search.DriverDocument{DriverID: "d1", Name: "u1"}); err != nil {
		t.Fatal(err)
	}
	if len(repo.UpsertDrivers) != 1 {
		t.Fatal("no upsert recorded")
	}
	if repo.UpsertDrivers[0].Name != "John Doe" {
		t.Errorf("name = %q, want 'John Doe'", repo.UpsertDrivers[0].Name)
	}
}

func TestUpsertDriver_EmptyFullName_NotReplaced(t *testing.T) {
	repo := testutil.NewFakeRepository()
	repo.UsersResult = search.SearchResult[search.UserDocument]{
		Items: []search.UserDocument{{ID: "u1", FirstName: "", LastName: ""}},
	}
	svc := NewService(repo)
	if err := svc.UpsertDriver(context.Background(), search.DriverDocument{DriverID: "d1", Name: "u1"}); err != nil {
		t.Fatal(err)
	}
	// fullName would be " " → keep original name
	if repo.UpsertDrivers[0].Name != "u1" {
		t.Errorf("name = %q, want original 'u1'", repo.UpsertDrivers[0].Name)
	}
}

func TestUpsertDriver_NoMatch_NoReplace(t *testing.T) {
	repo := testutil.NewFakeRepository()
	repo.UsersResult = search.SearchResult[search.UserDocument]{
		Items: []search.UserDocument{{ID: "other", FirstName: "A", LastName: "B"}},
	}
	svc := NewService(repo)
	if err := svc.UpsertDriver(context.Background(), search.DriverDocument{DriverID: "d1", Name: "NotAUser"}); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertDrivers[0].Name != "NotAUser" {
		t.Errorf("name = %q", repo.UpsertDrivers[0].Name)
	}
}

func TestUpsertDriver_SearchUsersError_Ignored(t *testing.T) {
	repo := testutil.NewFakeRepository()
	repo.UsersErr = errors.New("search down")
	svc := NewService(repo)
	if err := svc.UpsertDriver(context.Background(), search.DriverDocument{DriverID: "d1", Name: "u1"}); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertDrivers[0].Name != "u1" {
		t.Errorf("name = %q", repo.UpsertDrivers[0].Name)
	}
}

func TestUpsertDriver_UpstreamError(t *testing.T) {
	repo := testutil.NewFakeRepository()
	repo.UpsertDriverErr = errors.New("err")
	svc := NewService(repo)
	if err := svc.UpsertDriver(context.Background(), search.DriverDocument{DriverID: "d1"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteDriver_Delegates(t *testing.T) {
	repo := testutil.NewFakeRepository()
	svc := NewService(repo)
	if err := svc.DeleteDriver(context.Background(), "d1"); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedDrivers[0] != "d1" {
		t.Errorf("deleted = %v", repo.DeletedDrivers)
	}
	repo.DeleteDriverErr = errors.New("err")
	if err := svc.DeleteDriver(context.Background(), "d1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpsertMedia_Delegates(t *testing.T) {
	repo := testutil.NewFakeRepository()
	svc := NewService(repo)
	if err := svc.UpsertMedia(context.Background(), search.MediaDocument{MediaID: "m1"}); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertMediaDocs[0].MediaID != "m1" {
		t.Errorf("doc = %+v", repo.UpsertMediaDocs)
	}
	repo.UpsertMediaErr = errors.New("err")
	if err := svc.UpsertMedia(context.Background(), search.MediaDocument{MediaID: "m1"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteMedia_Delegates(t *testing.T) {
	repo := testutil.NewFakeRepository()
	svc := NewService(repo)
	if err := svc.DeleteMedia(context.Background(), "m1"); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedMedia[0] != "m1" {
		t.Errorf("deleted = %v", repo.DeletedMedia)
	}
	repo.DeleteMediaErr = errors.New("err")
	if err := svc.DeleteMedia(context.Background(), "m1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpsertUser_Delegates(t *testing.T) {
	repo := testutil.NewFakeRepository()
	svc := NewService(repo)
	if err := svc.UpsertUser(context.Background(), search.UserDocument{ID: "u1", Email: "a@b.c"}); err != nil {
		t.Fatal(err)
	}
	if repo.UpsertUsers[0].ID != "u1" {
		t.Errorf("user = %+v", repo.UpsertUsers)
	}
	repo.UpsertUserErr = errors.New("err")
	if err := svc.UpsertUser(context.Background(), search.UserDocument{ID: "u1"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteUser_Delegates(t *testing.T) {
	repo := testutil.NewFakeRepository()
	svc := NewService(repo)
	if err := svc.DeleteUser(context.Background(), "u1"); err != nil {
		t.Fatal(err)
	}
	if repo.DeletedUsers[0] != "u1" {
		t.Errorf("deleted = %v", repo.DeletedUsers)
	}
	repo.DeleteUserErr = errors.New("err")
	if err := svc.DeleteUser(context.Background(), "u1"); err == nil {
		t.Fatal("expected error")
	}
}
