package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

type StorageAdapter struct {
	client *Client
}

func NewStorageAdapter(client *Client) *StorageAdapter {
	return &StorageAdapter{client: client}
}

func (a *StorageAdapter) CreateMultipartUpload(ctx context.Context, objectKey, contentType string) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(a.client.bucketName),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}
	if a.client.sseKMS {
		input.ServerSideEncryption = types.ServerSideEncryptionAwsKms
	}
	out, err := a.client.s3Client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", fmt.Errorf("S3 CreateMultipartUpload %q: %w", objectKey, err)
	}
	return aws.ToString(out.UploadId), nil
}

func (a *StorageAdapter) GeneratePresignedParts(
	ctx context.Context,
	objectKey, s3UploadID string,
	totalParts int,
	_ int64, // partSize is for client guidance; S3 handles it
	expiry time.Duration,
) ([]ports.PresignedPart, error) {
	parts := make([]ports.PresignedPart, 0, totalParts)
	for i := 1; i <= totalParts; i++ {
		req, err := a.client.presigner.PresignUploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(a.client.bucketName),
			Key:        aws.String(objectKey),
			UploadId:   aws.String(s3UploadID),
			PartNumber: aws.Int32(int32(i)),
		}, func(o *s3.PresignOptions) {
			o.Expires = expiry
		})
		if err != nil {
			return nil, fmt.Errorf("presign part %d for %q: %w", i, objectKey, err)
		}
		parts = append(parts, ports.PresignedPart{
			PartNumber:   i,
			PresignedURL: a.client.presignableURL(req.URL),
		})
	}
	return parts, nil
}

func (a *StorageAdapter) GeneratePresignedGET(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	return a.GeneratePresignedGETWithRange(ctx, objectKey, "", expiry)
}

// GeneratePresignedGETWithRange generates a presigned GET URL with optional Range header
func (a *StorageAdapter) GeneratePresignedGETWithRange(ctx context.Context, objectKey, rangeHeader string, expiry time.Duration) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(a.client.bucketName),
		Key:    aws.String(objectKey),
	}
	if rangeHeader != "" {
		input.Range = aws.String(rangeHeader)
	}
	req, err := a.client.presigner.PresignGetObject(ctx, input, func(o *s3.PresignOptions) {
		o.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("presign GET for %q: %w", objectKey, err)
	}
	return a.client.presignableURL(req.URL), nil
}

func (a *StorageAdapter) CompleteMultipartUpload(ctx context.Context, objectKey, s3UploadID string, parts []domain.UploadPart) error {
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	completedParts := make([]types.CompletedPart, len(parts))
	for i, p := range parts {
		completedParts[i] = types.CompletedPart{
			PartNumber: aws.Int32(int32(p.PartNumber)),
			ETag:       aws.String(p.ETag),
		}
	}
	_, err := a.client.s3Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(a.client.bucketName),
		Key:             aws.String(objectKey),
		UploadId:        aws.String(s3UploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completedParts},
	})
	if err != nil {
		return fmt.Errorf("S3 CompleteMultipartUpload %q: %w", objectKey, err)
	}
	return nil
}

func (a *StorageAdapter) AbortMultipartUpload(ctx context.Context, objectKey, s3UploadID string) error {
	_, err := a.client.s3Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(a.client.bucketName),
		Key:      aws.String(objectKey),
		UploadId: aws.String(s3UploadID),
	})
	if err != nil {
		return fmt.Errorf("S3 AbortMultipartUpload %q: %w", objectKey, err)
	}
	return nil
}

func (a *StorageAdapter) HeadObject(ctx context.Context, objectKey string) (*ports.ObjectInfo, error) {
	out, err := a.client.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(a.client.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("S3 HeadObject %q: %w", objectKey, err)
	}
	return &ports.ObjectInfo{
		Size:         aws.ToInt64(out.ContentLength),
		ETag:         aws.ToString(out.ETag),
		ContentType:  aws.ToString(out.ContentType),
		LastModified: aws.ToTime(out.LastModified),
	}, nil
}

func (a *StorageAdapter) DeleteObject(ctx context.Context, objectKey string) error {
	_, err := a.client.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(a.client.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("S3 DeleteObject %q: %w", objectKey, err)
	}
	return nil
}

func (a *StorageAdapter) DeleteObjects(ctx context.Context, objectKeys []string) error {
	if len(objectKeys) == 0 {
		return nil
	}
	objects := make([]types.ObjectIdentifier, len(objectKeys))
	for i, k := range objectKeys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(k)}
	}
	_, err := a.client.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(a.client.bucketName),
		Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return fmt.Errorf("S3 DeleteObjects batch: %w", err)
	}
	return nil
}

func (a *StorageAdapter) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return a.GetObjectWithRange(ctx, objectKey, "")
}

// GetObjectWithRange retrieves an object with optional Range header support
func (a *StorageAdapter) GetObjectWithRange(ctx context.Context, objectKey, rangeHeader string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(a.client.bucketName),
		Key:    aws.String(objectKey),
	}
	if rangeHeader != "" {
		input.Range = aws.String(rangeHeader)
	}
	out, err := a.client.s3Client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("S3 GetObject %q: %w", objectKey, err)
	}
	return out.Body, nil
}

func (a *StorageAdapter) PutObject(ctx context.Context, objectKey, contentType string, data []byte) error {
	_, err := a.client.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.client.bucketName),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("S3 PutObject %q: %w", objectKey, err)
	}
	return nil
}

func (a *StorageAdapter) ListObjectsWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(a.client.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(a.client.bucketName),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("S3 ListObjectsV2 prefix %q: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			keys = append(keys, aws.ToString(obj.Key))
		}
	}
	return keys, nil
}
