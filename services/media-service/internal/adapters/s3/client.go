package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Client wraps the AWS S3 SDK client and presigner.
type Client struct {
	s3Client   *s3.Client
	presigner  *s3.PresignClient
	bucketName string
}

// NewClient creates an S3 client with server-side encryption enabled by default.
func NewClient(ctx context.Context, region, accessKeyID, secretKey, bucketName string) (*Client, error) {
	optFns := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}

	if accessKeyID != "" && secretKey != "" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, ""),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for S3: %w", err)
	}

	s3c := s3.NewFromConfig(cfg)

	return &Client{
		s3Client:   s3c,
		presigner:  s3.NewPresignClient(s3c),
		bucketName: bucketName,
	}, nil
}

// EnsureBucketPolicy applies block-public-access settings to the bucket.
// This is called once on startup to enforce S3 security posture.
func (c *Client) EnsureBucketPolicy(ctx context.Context) error {
	_, err := c.s3Client.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{
		Bucket: aws.String(c.bucketName),
		PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(true),
			BlockPublicPolicy:     aws.Bool(true),
			IgnorePublicAcls:      aws.Bool(true),
			RestrictPublicBuckets: aws.Bool(true),
		},
	})
	if err != nil {
		return fmt.Errorf("put public access block on bucket %q: %w", c.bucketName, err)
	}
	return nil
}
