package s3

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

func TestNewClientWithEndpoint(t *testing.T) {
	f := testutil.NewFakeS3(t)

	c, err := NewClient(context.Background(), "us-east-1", "key", "secret", testBucket, f.Endpoint(), "https://cdn.example.com/")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.bucketName != testBucket {
		t.Errorf("bucketName = %q", c.bucketName)
	}
	if c.publicEndpoint != "https://cdn.example.com" {
		t.Errorf("publicEndpoint = %q, want trailing slash trimmed", c.publicEndpoint)
	}
	if c.sseKMS {
		t.Error("sseKMS must be false when a custom endpoint is configured")
	}
	if c.s3Client == nil || c.presigner == nil {
		t.Fatal("s3Client/presigner must be set")
	}

	// the produced client must actually work against the fake endpoint
	if err := c.EnsureBucketExists(context.Background()); err != nil {
		t.Fatalf("EnsureBucketExists through NewClient: %v", err)
	}
	if f.OpCounts()["CreateBucket"] != 1 {
		t.Fatal("expected CreateBucket against the fake")
	}
}

func TestNewClientWithoutEndpoint(t *testing.T) {
	c, err := NewClient(context.Background(), "us-east-1", "key", "secret", testBucket, "", "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if !c.sseKMS {
		t.Error("sseKMS must be true when no custom endpoint is configured")
	}
	if c.publicEndpoint != "" {
		t.Errorf("publicEndpoint = %q", c.publicEndpoint)
	}
}

func TestNewClientWithoutCredentials(t *testing.T) {
	f := testutil.NewFakeS3(t)
	c, err := NewClient(context.Background(), "us-east-1", "", "", testBucket, f.Endpoint(), "")
	if err != nil {
		t.Fatalf("NewClient without credentials: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestPresignableURL(t *testing.T) {
	tests := []struct {
		name           string
		publicEndpoint string
		raw            string
		want           string
	}{
		{
			name:           "empty public endpoint returns raw",
			publicEndpoint: "",
			raw:            "http://127.0.0.1:9000/bucket/key?X-Amz-Signature=abc",
			want:           "http://127.0.0.1:9000/bucket/key?X-Amz-Signature=abc",
		},
		{
			name:           "rewrites scheme and host",
			publicEndpoint: "https://cdn.example.com",
			raw:            "http://127.0.0.1:9000/bucket/key?X-Amz-Signature=abc",
			want:           "https://cdn.example.com/bucket/key?X-Amz-Signature=abc",
		},
		{
			name:           "invalid raw url returned unchanged",
			publicEndpoint: "https://cdn.example.com",
			raw:            "://bad-url",
			want:           "://bad-url",
		},
		{
			name:           "invalid public endpoint returns raw",
			publicEndpoint: "://bad-endpoint",
			raw:            "http://127.0.0.1:9000/bucket/key",
			want:           "http://127.0.0.1:9000/bucket/key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{publicEndpoint: tt.publicEndpoint}
			if got := c.presignableURL(tt.raw); got != tt.want {
				t.Fatalf("presignableURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestGeneratePresignedParts(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, c := newTestAdapter(t, f)
	c.publicEndpoint = "https://cdn.example.com"

	parts, err := adapter.GeneratePresignedParts(context.Background(),
		"users/u1/media/m1/original/file.bin", "s3-upload-1", 3, 5<<20, presignExpiry)
	if err != nil {
		t.Fatalf("GeneratePresignedParts: %v", err)
	}
	if len(parts) != 3 {
		t.Fatalf("got %d parts, want 3", len(parts))
	}
	for i, p := range parts {
		if p.PartNumber != i+1 {
			t.Errorf("parts[%d].PartNumber = %d", i, p.PartNumber)
		}
		u, err := url.Parse(p.PresignedURL)
		if err != nil {
			t.Fatalf("parse presigned URL: %v", err)
		}
		if u.Scheme != "https" || u.Host != "cdn.example.com" {
			t.Errorf("presigned URL not rewritten: %s", p.PresignedURL)
		}
		q := u.Query()
		if q.Get("partNumber") != itoaInt(i+1) {
			t.Errorf("partNumber = %q", q.Get("partNumber"))
		}
		if q.Get("uploadId") != "s3-upload-1" {
			t.Errorf("uploadId = %q", q.Get("uploadId"))
		}
		if q.Get("X-Amz-Signature") == "" {
			t.Errorf("missing signature in %s", p.PresignedURL)
		}
	}
	if f.OpCounts()["UploadPart"] != 0 {
		t.Error("presigning must not hit the S3 endpoint")
	}
}

func TestGeneratePresignedPartsZeroParts(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, _ := newTestAdapter(t, f)

	parts, err := adapter.GeneratePresignedParts(context.Background(), "k", "up", 0, 1024, presignExpiry)
	if err != nil {
		t.Fatalf("GeneratePresignedParts: %v", err)
	}
	if len(parts) != 0 {
		t.Fatalf("expected 0 parts, got %d", len(parts))
	}
}

func TestGeneratePresignedGET(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, c := newTestAdapter(t, f)
	c.publicEndpoint = "https://cdn.example.com"

	raw, err := adapter.GeneratePresignedGET(context.Background(), "users/u1/media/m1/original/photo.jpg", presignExpiry)
	if err != nil {
		t.Fatalf("GeneratePresignedGET: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if u.Scheme != "https" || u.Host != "cdn.example.com" {
		t.Fatalf("URL not rewritten: %s", raw)
	}
	if u.Query().Get("X-Amz-Signature") == "" {
		t.Fatalf("missing signature: %s", raw)
	}
	if u.Query().Get("range") != "" {
		t.Fatalf("unexpected range param: %s", raw)
	}
}

func TestGeneratePresignedGETWithRange(t *testing.T) {
	f := testutil.NewFakeS3(t)
	adapter, _ := newTestAdapter(t, f)

	raw, err := adapter.GeneratePresignedGETWithRange(context.Background(), "k", "bytes=0-1023", presignExpiry)
	if err != nil {
		t.Fatalf("GeneratePresignedGETWithRange: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	// The SDK signs Range as a signed header rather than a query parameter.
	signedHeaders := u.Query().Get("X-Amz-SignedHeaders")
	if !strings.Contains(signedHeaders, "range") {
		t.Fatalf("range not among signed headers %q (url: %s)", signedHeaders, raw)
	}
	if u.Query().Get("X-Amz-Signature") == "" {
		t.Fatalf("missing signature: %s", raw)
	}
}

func TestEnsureBucketExists(t *testing.T) {
	f := testutil.NewFakeS3(t)
	_, c := newTestAdapter(t, f)

	if err := c.EnsureBucketExists(context.Background()); err != nil {
		t.Fatalf("EnsureBucketExists: %v", err)
	}
	if f.OpCounts()["CreateBucket"] != 1 {
		t.Fatal("expected CreateBucket")
	}
}

func TestEnsureBucketExistsAlreadyOwned(t *testing.T) {
	tests := []struct {
		code string
	}{
		{code: "BucketAlreadyOwnedByYou"},
		{code: "BucketAlreadyExists"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			f := testutil.NewFakeS3(t)
			f.SetError("CreateBucket", &testutil.S3Error{Code: tt.code, Message: "exists", Status: 409})
			_, c := newTestAdapter(t, f)

			if err := c.EnsureBucketExists(context.Background()); err != nil {
				t.Fatalf("EnsureBucketExists(%s): %v", tt.code, err)
			}
		})
	}
}

func TestEnsureBucketExistsOtherError(t *testing.T) {
	f := testutil.NewFakeS3(t)
	f.SetError("CreateBucket", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	_, c := newTestAdapter(t, f)

	err := c.EnsureBucketExists(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create bucket") {
		t.Fatalf("err = %v", err)
	}
}

func TestEnsureBucketPolicy(t *testing.T) {
	f := testutil.NewFakeS3(t)
	_, c := newTestAdapter(t, f)

	if err := c.EnsureBucketPolicy(context.Background()); err != nil {
		t.Fatalf("EnsureBucketPolicy: %v", err)
	}
	if f.OpCounts()["PutPublicAccessBlock"] != 1 {
		t.Fatal("expected PutPublicAccessBlock")
	}

	f.SetError("PutPublicAccessBlock", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	err := c.EnsureBucketPolicy(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "put public access block") {
		t.Fatalf("err = %v", err)
	}
}

func TestEnsureBucketLifecycle(t *testing.T) {
	f := testutil.NewFakeS3(t)
	_, c := newTestAdapter(t, f)

	if err := c.EnsureBucketLifecycle(context.Background(), 0); err == nil {
		t.Fatal("expected error for days < 1")
	}
	if f.OpCounts()["PutBucketLifecycle"] != 0 {
		t.Fatal("invalid days must not produce a request")
	}

	if err := c.EnsureBucketLifecycle(context.Background(), 3); err != nil {
		t.Fatalf("EnsureBucketLifecycle: %v", err)
	}
	if f.OpCounts()["PutBucketLifecycle"] != 1 {
		t.Fatal("expected PutBucketLifecycle")
	}

	f.SetError("PutBucketLifecycle", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	err := c.EnsureBucketLifecycle(context.Background(), 3)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "multipart lifecycle") {
		t.Fatalf("err = %v", err)
	}
}

func TestEnsureBucketCORS(t *testing.T) {
	f := testutil.NewFakeS3(t)
	_, c := newTestAdapter(t, f)

	if err := c.EnsureBucketCORS(context.Background()); err != nil {
		t.Fatalf("EnsureBucketCORS: %v", err)
	}
	if f.OpCounts()["PutBucketCors"] != 1 {
		t.Fatal("expected PutBucketCors")
	}

	f.SetError("PutBucketCors", &testutil.S3Error{Code: "AccessDenied", Message: "no", Status: 403})
	err := c.EnsureBucketCORS(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "put CORS") {
		t.Fatalf("err = %v", err)
	}
}
