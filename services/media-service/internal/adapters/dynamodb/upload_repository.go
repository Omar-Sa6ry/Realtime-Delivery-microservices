package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type uploadItem struct {
	PK             string `dynamodbav:"PK"`
	SK             string `dynamodbav:"SK"`
	GSI2PK         string `dynamodbav:"GSI2_PK"`
	GSI2SK         string `dynamodbav:"GSI2_SK"`
	UploadID       string `dynamodbav:"UploadID"`
	MediaID        string `dynamodbav:"MediaID"`
	UserID         string `dynamodbav:"UserID"`
	S3UploadID     string `dynamodbav:"S3UploadID"`
	ObjectKey      string `dynamodbav:"ObjectKey"`
	TotalParts     int    `dynamodbav:"TotalParts"`
	PartSize       int64  `dynamodbav:"PartSize"`
	CompletedParts []struct {
		PartNumber int    `dynamodbav:"PartNumber"`
		ETag       string `dynamodbav:"ETag"`
	} `dynamodbav:"CompletedParts"`
	Status    string `dynamodbav:"Status"`
	ExpiresAt string `dynamodbav:"ExpiresAt"`
	CreatedAt string `dynamodbav:"CreatedAt"`
	UpdatedAt string `dynamodbav:"UpdatedAt"`
	// DynamoDB TTL — set to ExpiresAt unix timestamp for auto-cleanup.
	// This is the Redis-TTL-equivalent the user requested: no cron needed.
	TTL int64 `dynamodbav:"TTL"`
}

// UploadRepository implements ports.UploadRepository using DynamoDB.
type UploadRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewUploadRepository creates a new UploadRepository.
func NewUploadRepository(client *dynamodb.Client, tableName string) *UploadRepository {
	return &UploadRepository{client: client, tableName: tableName}
}

func uploadPK(uploadID string) string { return "UPLOAD#" + uploadID }
func uploadSK() string                { return "SESSION" }

func (r *UploadRepository) Create(ctx context.Context, s *domain.UploadSession) error {
	now := s.CreatedAt.UTC().Format(time.RFC3339Nano)
	expiresAt := s.ExpiresAt.UTC().Format(time.RFC3339Nano)

	completedParts := make([]struct {
		PartNumber int    `dynamodbav:"PartNumber"`
		ETag       string `dynamodbav:"ETag"`
	}, len(s.CompletedParts))
	for i, p := range s.CompletedParts {
		completedParts[i] = struct {
			PartNumber int    `dynamodbav:"PartNumber"`
			ETag       string `dynamodbav:"ETag"`
		}{PartNumber: p.PartNumber, ETag: p.ETag}
	}

	item := uploadItem{
		PK:             uploadPK(s.UploadID),
		SK:             uploadSK(),
		GSI2PK:         "UPLOAD_STATUS#" + string(s.Status),
		GSI2SK:         "EXPIRES#" + expiresAt,
		UploadID:       s.UploadID,
		MediaID:        s.MediaID,
		UserID:         s.UserID,
		S3UploadID:     s.S3UploadID,
		ObjectKey:      s.ObjectKey,
		TotalParts:     s.TotalParts,
		PartSize:       s.PartSize,
		CompletedParts: completedParts,
		Status:         string(s.Status),
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
		TTL:            s.ExpiresAt.Unix(), // DynamoDB TTL auto-cleanup — no cron needed
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshal upload session item: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	return err
}

func (r *UploadRepository) GetByID(ctx context.Context, uploadID string) (*domain.UploadSession, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: uploadPK(uploadID)},
			"SK": &types.AttributeValueMemberS{Value: uploadSK()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("DynamoDB GetItem upload %q: %w", uploadID, err)
	}
	if out.Item == nil {
		return nil, domain.ErrUploadSessionNotFound
	}

	var item uploadItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshal upload item: %w", err)
	}
	return itemToUpload(&item), nil
}

func (r *UploadRepository) UpdateStatus(ctx context.Context, uploadID string, status domain.UploadStatus) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: uploadPK(uploadID)},
			"SK": &types.AttributeValueMemberS{Value: uploadSK()},
		},
		UpdateExpression:         aws.String("SET #st = :st, GSI2_PK = :gsi2pk, UpdatedAt = :now"),
		ExpressionAttributeNames: map[string]string{"#st": "Status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":st":     &types.AttributeValueMemberS{Value: string(status)},
			":gsi2pk": &types.AttributeValueMemberS{Value: "UPLOAD_STATUS#" + string(status)},
			":now":    &types.AttributeValueMemberS{Value: now},
		},
	})
	return err
}

func (r *UploadRepository) UpdateCompletedParts(ctx context.Context, uploadID string, parts []domain.UploadPart) error {
	partsAV, err := attributevalue.MarshalList(parts)
	if err != nil {
		return fmt.Errorf("marshal completed parts: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: uploadPK(uploadID)},
			"SK": &types.AttributeValueMemberS{Value: uploadSK()},
		},
		UpdateExpression: aws.String("SET CompletedParts = :parts, UpdatedAt = :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":parts": &types.AttributeValueMemberL{Value: partsAV},
			":now":   &types.AttributeValueMemberS{Value: now},
		},
	})
	return err
}

func (r *UploadRepository) UpdateExpiry(ctx context.Context, uploadID string, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: uploadPK(uploadID)},
			"SK": &types.AttributeValueMemberS{Value: uploadSK()},
		},
		UpdateExpression:         aws.String("SET ExpiresAt = :expires, #ttl = :ttl, UpdatedAt = :now"),
		ConditionExpression:      aws.String("attribute_exists(PK) AND #st = :uploading"),
		ExpressionAttributeNames: map[string]string{"#ttl": "TTL", "#st": "Status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expires":   &types.AttributeValueMemberS{Value: expiresAt.UTC().Format(time.RFC3339Nano)},
			":ttl":       &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiresAt.Unix())},
			":now":       &types.AttributeValueMemberS{Value: now},
			":uploading": &types.AttributeValueMemberS{Value: string(domain.UploadStatusUploading)},
		},
	})
	return err
}

func (r *UploadRepository) ListExpired(ctx context.Context, before time.Time) ([]*domain.UploadSession, error) {
	beforeStr := before.UTC().Format(time.RFC3339Nano)
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("GSI2"),
		KeyConditionExpression: aws.String("GSI2_PK = :pk AND GSI2_SK <= :sk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "UPLOAD_STATUS#UPLOADING"},
			":sk": &types.AttributeValueMemberS{Value: "EXPIRES#" + beforeStr},
		},
		Limit: aws.Int32(100),
	})
	if err != nil {
		return nil, fmt.Errorf("DynamoDB Query expired uploads: %w", err)
	}

	sessions := make([]*domain.UploadSession, 0, len(out.Items))
	for _, raw := range out.Items {
		var item uploadItem
		if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
			continue
		}
		sessions = append(sessions, itemToUpload(&item))
	}
	return sessions, nil
}

func itemToUpload(item *uploadItem) *domain.UploadSession {
	createdAt, _ := time.Parse(time.RFC3339Nano, item.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339Nano, item.UpdatedAt)
	expiresAt, _ := time.Parse(time.RFC3339Nano, item.ExpiresAt)

	parts := make([]domain.UploadPart, len(item.CompletedParts))
	for i, p := range item.CompletedParts {
		parts[i] = domain.UploadPart{PartNumber: p.PartNumber, ETag: p.ETag}
	}
	return &domain.UploadSession{
		UploadID:       item.UploadID,
		MediaID:        item.MediaID,
		UserID:         item.UserID,
		S3UploadID:     item.S3UploadID,
		ObjectKey:      item.ObjectKey,
		TotalParts:     item.TotalParts,
		PartSize:       item.PartSize,
		CompletedParts: parts,
		Status:         domain.UploadStatus(item.Status),
		ExpiresAt:      expiresAt,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}
