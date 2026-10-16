package s3

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const testBucket = "media-bucket"

var presignExpiry = 15 * time.Minute

func itoaInt(v int) string { return strconv.Itoa(v) }

type failingCredentialsProvider struct{}

func (failingCredentialsProvider) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{}, errors.New("no credentials available")
}

// newTestAdapterWithFailingCreds builds an adapter whose requests cannot be
// signed, so presign operations fail.
func newTestAdapterWithFailingCreds(t *testing.T, f *testutil.FakeS3) *StorageAdapter {
	t.Helper()
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(failingCredentialsProvider{}),
		awsconfig.WithRetryMaxAttempts(1),
	)
	if err != nil {
		t.Fatalf("LoadDefaultConfig: %v", err)
	}
	s3c := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(f.Endpoint())
		o.UsePathStyle = true
	})
	return NewStorageAdapter(&Client{
		s3Client:   s3c,
		presigner:  s3.NewPresignClient(s3c),
		bucketName: testBucket,
	})
}

func newTestSDKClient(t *testing.T, endpoint string) *s3.Client {
	t.Helper()
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("key", "secret", "")),
		awsconfig.WithRetryMaxAttempts(1),
	)
	if err != nil {
		t.Fatalf("LoadDefaultConfig: %v", err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
}

func newTestAdapter(t *testing.T, f *testutil.FakeS3) (*StorageAdapter, *Client) {
	t.Helper()
	s3c := newTestSDKClient(t, f.Endpoint())
	c := &Client{
		s3Client:   s3c,
		presigner:  s3.NewPresignClient(s3c),
		bucketName: testBucket,
	}
	return NewStorageAdapter(c), c
}

func lastS3Request(t *testing.T, f *testutil.FakeS3, op string) testutil.S3Request {
	t.Helper()
	reqs := f.Requests()
	for i := len(reqs) - 1; i >= 0; i-- {
		if reqs[i].Op == op {
			return reqs[i]
		}
	}
	t.Fatalf("no recorded %s request", op)
	return testutil.S3Request{}
}
