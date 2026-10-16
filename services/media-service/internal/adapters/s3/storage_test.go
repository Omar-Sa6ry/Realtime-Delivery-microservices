package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestCreateMultipartUpload(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, _ := newTestAdapter(t, f)

	id, err := adapter.CreateMultipartUpload(context.Background(), "users/u1/media/m1/original/file.bin", "application/pdf")
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty upload id")
	}
	if !f.HasMultipartUpload(id) {
		t.Fatalf("upload %q not open on the fake", id)
	}
}

func TestCreateMultipartUploadSSEHeader(t *testing.T) {
	tests := []struct {
		name    string
		sseKMS  bool
		wantSSE string
	}{
		{name: "sseKMS enabled sets aws:kms header", sseKMS: true, wantSSE: "aws:kms"},
		{name: "sseKMS disabled sets no header", sseKMS: false, wantSSE: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotSSE string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotSSE = r.Header.Get("x-amz-server-side-encryption")
				w.Header().Set("Content-Type", "application/xml")
				_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><InitiateMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Bucket>`+
					testBucket+`</Bucket><Key>k</Key><UploadId>up-sse</UploadId></InitiateMultipartUploadResult>`)
			}))
			t.Cleanup(srv.Close)

			s3c := newTestSDKClient(t, srv.URL)
			adapter := NewStorageAdapter(&Client{
				s3Client:   s3c,
				presigner:  awss3.NewPresignClient(s3c),
				bucketName: testBucket,
				sseKMS:     tt.sseKMS,
			})

			id, err := adapter.CreateMultipartUpload(context.Background(), "k", "application/pdf")
			if err != nil {
				t.Fatalf("CreateMultipartUpload: %v", err)
			}
			if id != "up-sse" {
				t.Errorf("upload id = %q", id)
			}
			if gotSSE != tt.wantSSE {
				t.Errorf("x-amz-server-side-encryption = %q, want %q", gotSSE, tt.wantSSE)
			}
		})
	}
}

func TestCreateMultipartUploadError(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.SetError("CreateMultipartUpload", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	adapter, _ := newTestAdapter(t, f)

	_, err := adapter.CreateMultipartUpload(context.Background(), "k", "application/pdf")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "S3 CreateMultipartUpload") {
		t.Fatalf("err = %v", err)
	}
}

func TestCompleteMultipartUploadSortsParts(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, _ := newTestAdapter(t, f)
	ctx := context.Background()

	id, err := adapter.CreateMultipartUpload(ctx, "k", "application/pdf")
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}
	parts := []domain.UploadPart{
		{PartNumber: 3, ETag: `"e3"`},
		{PartNumber: 1, ETag: `"e1"`},
		{PartNumber: 2, ETag: `"e2"`},
	}
	if err := adapter.CompleteMultipartUpload(ctx, "k", id, parts); err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}
	if f.HasMultipartUpload(id) {
		t.Fatal("upload should be closed after complete")
	}

	body := string(lastS3Request(t, f, "CompleteMultipartUpload").Body)
	i1 := strings.Index(body, "<PartNumber>1</PartNumber>")
	i2 := strings.Index(body, "<PartNumber>2</PartNumber>")
	i3 := strings.Index(body, "<PartNumber>3</PartNumber>")
	if i1 < 0 || i2 < 0 || i3 < 0 {
		t.Fatalf("missing parts in body: %s", body)
	}
	if !(i1 < i2 && i2 < i3) {
		t.Fatalf("parts not sorted in body: %s", body)
	}
}

func TestCompleteMultipartUploadUnknownUpload(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, _ := newTestAdapter(t, f)

	err := adapter.CompleteMultipartUpload(context.Background(), "k", "nope", []domain.UploadPart{{PartNumber: 1, ETag: `"e1"`}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "S3 CompleteMultipartUpload") {
		t.Fatalf("err = %v", err)
	}
}

func TestCompleteMultipartUploadError(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.SetError("CompleteMultipartUpload", &testutil.S3Error{Code: "InternalError", Message: "no", Status: 400})
	adapter, _ := newTestAdapter(t, f)

	err := adapter.CompleteMultipartUpload(context.Background(), "k", "up", []domain.UploadPart{{PartNumber: 1, ETag: `"e1"`}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAbortMultipartUpload(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, _ := newTestAdapter(t, f)
	ctx := context.Background()

	id, err := adapter.CreateMultipartUpload(ctx, "k", "application/pdf")
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}
	if f.OpenUploads() != 1 {
		t.Fatalf("OpenUploads = %d", f.OpenUploads())
	}
	if err := adapter.AbortMultipartUpload(ctx, "k", id); err != nil {
		t.Fatalf("AbortMultipartUpload: %v", err)
	}
	if f.OpenUploads() != 0 {
		t.Fatal("upload should be aborted")
	}
}

func TestAbortMultipartUploadError(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.SetError("AbortMultipartUpload", &testutil.S3Error{Code: "NoSuchUpload", Message: "gone", Status: 404})
	adapter, _ := newTestAdapter(t, f)

	err := adapter.AbortMultipartUpload(context.Background(), "k", "up")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "S3 AbortMultipartUpload") {
		t.Fatalf("err = %v", err)
	}
}

func TestHeadObject(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.PutObject("users/u1/photo.jpg", "image/jpeg", []byte("hello"))
	adapter, _ := newTestAdapter(t, f)

	info, err := adapter.HeadObject(context.Background(), "users/u1/photo.jpg")
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if info.Size != 5 || info.ContentType != "image/jpeg" || info.ETag == "" {
		t.Fatalf("unexpected ObjectInfo: %+v", info)
	}
	if info.LastModified.IsZero() {
		t.Fatal("LastModified must be set")
	}
}

func TestHeadObjectNotFound(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, _ := newTestAdapter(t, f)

	_, err := adapter.HeadObject(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "S3 HeadObject") || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v", err)
	}
}

func TestHeadObjectError(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.SetError("HeadObject", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	adapter, _ := newTestAdapter(t, f)

	if _, err := adapter.HeadObject(context.Background(), "k"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteObject(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.PutObject("k", "text/plain", []byte("data"))
	adapter, _ := newTestAdapter(t, f)

	if err := adapter.DeleteObject(context.Background(), "k"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if _, ok := f.GetObject("k"); ok {
		t.Fatal("object should be deleted")
	}

	f.SetError("DeleteObject", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	delErr := adapter.DeleteObject(context.Background(), "k2")
	if delErr == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(delErr.Error(), "S3 DeleteObject") {
		t.Fatalf("err = %v", delErr)
	}
}

func TestDeleteObjects(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.PutObject("a", "text/plain", []byte("1"))
	f.PutObject("b", "text/plain", []byte("2"))
	f.PutObject("c", "text/plain", []byte("3"))
	adapter, _ := newTestAdapter(t, f)

	if err := adapter.DeleteObjects(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatalf("DeleteObjects: %v", err)
	}
	if _, ok := f.GetObject("a"); ok {
		t.Fatal("a should be deleted")
	}
	if _, ok := f.GetObject("b"); ok {
		t.Fatal("b should be deleted")
	}
	if _, ok := f.GetObject("c"); !ok {
		t.Fatal("c must survive")
	}
}

func TestDeleteObjectsEmpty(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, _ := newTestAdapter(t, f)

	if err := adapter.DeleteObjects(context.Background(), nil); err != nil {
		t.Fatalf("DeleteObjects(nil): %v", err)
	}
	if len(f.Requests()) != 0 {
		t.Fatalf("empty DeleteObjects must not hit the endpoint, got %d requests", len(f.Requests()))
	}
}

func TestDeleteObjectsError(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.SetError("DeleteObjects", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	adapter, _ := newTestAdapter(t, f)

	err := adapter.DeleteObjects(context.Background(), []string{"a"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "S3 DeleteObjects batch") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetObject(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.PutObject("k", "text/plain", []byte("hello world"))
	adapter, _ := newTestAdapter(t, f)

	rc, err := adapter.GetObject(context.Background(), "k")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("body = %q", data)
	}
}

func TestGetObjectWithRange(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.PutObject("k", "text/plain", []byte("hello world"))
	adapter, _ := newTestAdapter(t, f)

	rc, err := adapter.GetObjectWithRange(context.Background(), "k", "bytes=0-4")
	if err != nil {
		t.Fatalf("GetObjectWithRange: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("body = %q", data)
	}
	if f.OpCounts()["GetObject"] != 1 {
		t.Fatalf("unexpected GetObject count: %d", f.OpCounts()["GetObject"])
	}
}

func TestGetObjectNotFound(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, _ := newTestAdapter(t, f)

	_, err := adapter.GetObject(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "S3 GetObject") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetObjectError(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.SetError("GetObject", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	adapter, _ := newTestAdapter(t, f)

	if _, err := adapter.GetObject(context.Background(), "k"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPutObject(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, _ := newTestAdapter(t, f)

	if err := adapter.PutObject(context.Background(), "k", "text/plain", []byte("payload")); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	data, ok := f.GetObject("k")
	if !ok || string(data) != "payload" {
		t.Fatalf("stored data = %q, ok=%v", data, ok)
	}

	f.SetError("PutObject", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	err := adapter.PutObject(context.Background(), "k2", "text/plain", []byte("x"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "S3 PutObject") {
		t.Fatalf("err = %v", err)
	}
}

func TestListObjectsWithPrefix(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.PutObject("users/u1/a.txt", "text/plain", []byte("1"))
	f.PutObject("users/u1/b.bin", "application/octet-stream", []byte("22"))
	f.PutObject("users/u2/c.txt", "text/plain", []byte("3"))
	adapter, _ := newTestAdapter(t, f)

	keys, err := adapter.ListObjectsWithPrefix(context.Background(), "users/u1/")
	if err != nil {
		t.Fatalf("ListObjectsWithPrefix: %v", err)
	}
	if len(keys) != 2 || keys[0] != "users/u1/a.txt" || keys[1] != "users/u1/b.bin" {
		t.Fatalf("keys = %v", keys)
	}

	none, err := adapter.ListObjectsWithPrefix(context.Background(), "nobody/")
	if err != nil {
		t.Fatalf("ListObjectsWithPrefix none: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no keys, got %v", none)
	}
}

func TestListObjectsWithPrefixError(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.SetError("ListObjectsV2", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	adapter, _ := newTestAdapter(t, f)

	_, err := adapter.ListObjectsWithPrefix(context.Background(), "users/")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "S3 ListObjectsV2") {
		t.Fatalf("err = %v", err)
	}
}

func TestGeneratePresignedPartsPresignError(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter := newTestAdapterWithFailingCreds(t, f)

	_, err := adapter.GeneratePresignedParts(context.Background(), "k", "up", 1, 1024, presignExpiry)
	if err == nil {
		t.Fatal("expected presign error")
	}
	if !strings.Contains(err.Error(), "presign part 1") {
		t.Fatalf("err = %v", err)
	}
}

func TestGeneratePresignedGETPresignError(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter := newTestAdapterWithFailingCreds(t, f)

	_, err := adapter.GeneratePresignedGET(context.Background(), "k", presignExpiry)
	if err == nil {
		t.Fatal("expected presign error")
	}
	if !strings.Contains(err.Error(), "presign GET") {
		t.Fatalf("err = %v", err)
	}
}
