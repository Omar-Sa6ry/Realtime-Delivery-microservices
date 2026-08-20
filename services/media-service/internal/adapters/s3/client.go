package s3

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Client struct {
	s3Client   *s3.Client
	presigner  *s3.PresignClient
	bucketName string
	publicEndpoint string
	sseKMS bool
}

func NewClient(ctx context.Context, region, accessKeyID, secretKey, bucketName, endpoint, publicEndpoint string) (*Client, error) {
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

	var s3Options []func(*s3.Options)
	if endpoint != "" {
		s3Options = append(s3Options, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	}

	s3c := s3.NewFromConfig(cfg, s3Options...)

	return &Client{
		s3Client:       s3c,
		presigner:      s3.NewPresignClient(s3c),
		bucketName:     bucketName,
		publicEndpoint: strings.TrimSuffix(publicEndpoint, "/"),
		sseKMS: endpoint == "",
	}, nil
}

func (c *Client) presignableURL(raw string) string {
	if c.publicEndpoint == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	pub, err := url.Parse(c.publicEndpoint)
	if err != nil {
		return raw
	}
	u.Scheme = pub.Scheme
	u.Host = pub.Host
	return u.String()
}

func (c *Client) EnsureBucketExists(ctx context.Context) error {
	_, err := c.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(c.bucketName),
	})
	if err != nil {
		var owned *types.BucketAlreadyOwnedByYou
		var exists *types.BucketAlreadyExists
		if errors.As(err, &owned) || errors.As(err, &exists) {
			return nil
		}
		return fmt.Errorf("create bucket %q: %w", c.bucketName, err)
	}
	return nil
}

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

func (c *Client) EnsureBucketCORS(ctx context.Context) error {
	_, err := c.s3Client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(c.bucketName),
		CORSConfiguration: &types.CORSConfiguration{
			CORSRules: []types.CORSRule{
				{
					AllowedOrigins: []string{"http://localhost:5173", "http://localhost:3000"},
					AllowedMethods: []string{"GET", "PUT", "POST", "HEAD"},
					AllowedHeaders: []string{"*"},
					ExposeHeaders:  []string{"ETag", "x-amz-request-id"},
					MaxAgeSeconds:  aws.Int32(3600),
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("put CORS on bucket %q: %w", c.bucketName, err)
	}
	return nil
}
