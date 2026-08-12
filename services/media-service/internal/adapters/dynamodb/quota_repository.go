package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
)

// QuotaRepository implements ports.QuotaRepository using DynamoDB atomic counters.
// DynamoDB ADD expressions are used for concurrency-safe increments/decrements.
type QuotaRepository struct {
	client    *dynamodb.Client
	tableName string
	maxQuota  int64
	maxConc   int
}

// NewQuotaRepository creates a new QuotaRepository.
func NewQuotaRepository(client *dynamodb.Client, tableName string, maxQuotaBytes int64, maxConcurrent int) *QuotaRepository {
	return &QuotaRepository{
		client:    client,
		tableName: tableName,
		maxQuota:  maxQuotaBytes,
		maxConc:   maxConcurrent,
	}
}

func quotaPK(userID string) string { return "USER#" + userID }
func quotaSK() string              { return "QUOTA" }

type quotaItem struct {
	PK            string `dynamodbav:"PK"`
	SK            string `dynamodbav:"SK"`
	UserID        string `dynamodbav:"UserID"`
	UsedBytes     int64  `dynamodbav:"UsedBytes"`
	ActiveUploads int    `dynamodbav:"ActiveUploads"`
	UpdatedAt     string `dynamodbav:"UpdatedAt"`
}

// GetUsage retrieves the current quota usage for a user.
// If no record exists, returns a zero-usage quota (first-time user).
func (r *QuotaRepository) GetUsage(ctx context.Context, userID string) (*domain.QuotaUsage, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: quotaPK(userID)},
			"SK": &types.AttributeValueMemberS{Value: quotaSK()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("DynamoDB GetItem quota for %q: %w", userID, err)
	}

	if out.Item == nil {
		return &domain.QuotaUsage{
			UserID:        userID,
			UsedBytes:     0,
			QuotaBytes:    r.maxQuota,
			ActiveUploads: 0,
			MaxConcurrent: r.maxConc,
		}, nil
	}

	var item quotaItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshal quota item: %w", err)
	}
	return &domain.QuotaUsage{
		UserID:        userID,
		UsedBytes:     item.UsedBytes,
		QuotaBytes:    r.maxQuota,
		ActiveUploads: item.ActiveUploads,
		MaxConcurrent: r.maxConc,
	}, nil
}

// IncrementUsage atomically increments the active upload counter.
// Uses DynamoDB ADD which is atomic even under concurrent writes.
func (r *QuotaRepository) IncrementUsage(ctx context.Context, userID string, _ int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: quotaPK(userID)},
			"SK": &types.AttributeValueMemberS{Value: quotaSK()},
		},
		UpdateExpression: aws.String("SET UserID = if_not_exists(UserID, :uid), UpdatedAt = :now ADD ActiveUploads :one"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one": &types.AttributeValueMemberN{Value: "1"},
			":uid": &types.AttributeValueMemberS{Value: userID},
			":now": &types.AttributeValueMemberS{Value: now},
		},
	})
	return err
}

// DecrementActiveUpload atomically decrements the active upload counter.
func (r *QuotaRepository) DecrementActiveUpload(ctx context.Context, userID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: quotaPK(userID)},
			"SK": &types.AttributeValueMemberS{Value: quotaSK()},
		},
		UpdateExpression: aws.String("SET UpdatedAt = :now ADD ActiveUploads :negOne"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":negOne": &types.AttributeValueMemberN{Value: "-1"},
			":now":    &types.AttributeValueMemberS{Value: now},
		},
	})
	return err
}

// AddUsedBytes atomically adds bytes to the committed storage usage.
func (r *QuotaRepository) AddUsedBytes(ctx context.Context, userID string, bytes int64) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: quotaPK(userID)},
			"SK": &types.AttributeValueMemberS{Value: quotaSK()},
		},
		UpdateExpression: aws.String("ADD UsedBytes :bytes"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":bytes": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", bytes)},
		},
	})
	return err
}

// SubtractUsedBytes atomically subtracts bytes from the committed storage usage.
func (r *QuotaRepository) SubtractUsedBytes(ctx context.Context, userID string, bytes int64) error {
	return r.AddUsedBytes(ctx, userID, -bytes)
}
