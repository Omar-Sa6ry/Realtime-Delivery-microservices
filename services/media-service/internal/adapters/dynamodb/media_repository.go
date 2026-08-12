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

// mediaItem is the DynamoDB Single-Table record for a Media aggregate.
type mediaItem struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"`
	GSI1PK      string `dynamodbav:"GSI1_PK"`
	GSI1SK      string `dynamodbav:"GSI1_SK"`
	GSI2PK      string `dynamodbav:"GSI2_PK"`
	GSI2SK      string `dynamodbav:"GSI2_SK"`
	MediaID     string `dynamodbav:"MediaID"`
	OwnerID     string `dynamodbav:"OwnerID"`
	DeliveryID  string `dynamodbav:"DeliveryID,omitempty"`
	FileName    string `dynamodbav:"FileName"`
	ContentType string `dynamodbav:"ContentType"`
	MediaType   string `dynamodbav:"MediaType"`
	Size        int64  `dynamodbav:"Size"`
	Checksum    string `dynamodbav:"Checksum,omitempty"`
	ObjectKey   string `dynamodbav:"ObjectKey"`
	Status      string `dynamodbav:"Status"`
	CreatedAt   string `dynamodbav:"CreatedAt"`
	UpdatedAt   string `dynamodbav:"UpdatedAt"`
}

// MediaRepository implements ports.MediaRepository using DynamoDB.
type MediaRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewMediaRepository creates a new MediaRepository.
func NewMediaRepository(client *dynamodb.Client, tableName string) *MediaRepository {
	return &MediaRepository{client: client, tableName: tableName}
}

func mediaPK(mediaID string) string  { return "MEDIA#" + mediaID }
func mediaSK() string                { return "MEDIA" }
func ownerGSI1PK(ownerID string) string { return "OWNER#" + ownerID }

func (r *MediaRepository) Create(ctx context.Context, m *domain.Media) error {
	now := m.CreatedAt.UTC().Format(time.RFC3339Nano)
	item := mediaItem{
		PK:          mediaPK(m.MediaID),
		SK:          mediaSK(),
		GSI1PK:      ownerGSI1PK(m.OwnerID),
		GSI1SK:      fmt.Sprintf("STATUS#%s#CREATED#%s", m.Status, now),
		GSI2PK:      "STATUS#" + string(m.Status),
		GSI2SK:      "CREATED#" + now,
		MediaID:     m.MediaID,
		OwnerID:     m.OwnerID,
		DeliveryID:  m.DeliveryID,
		FileName:    m.FileName,
		ContentType: m.ContentType,
		MediaType:   string(m.MediaType),
		Size:        m.Size,
		Checksum:    m.Checksum,
		ObjectKey:   m.ObjectKey,
		Status:      string(m.Status),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshal media item: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		return fmt.Errorf("DynamoDB PutItem media %q: %w", m.MediaID, err)
	}
	return nil
}

func (r *MediaRepository) GetByID(ctx context.Context, mediaID string) (*domain.Media, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: mediaPK(mediaID)},
			"SK": &types.AttributeValueMemberS{Value: mediaSK()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("DynamoDB GetItem media %q: %w", mediaID, err)
	}
	if out.Item == nil {
		return nil, domain.ErrMediaNotFound
	}

	var item mediaItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshal media item: %w", err)
	}
	return itemToMedia(&item)
}

// UpdateStatus performs a conditional state transition.
// Only succeeds if the current DynamoDB status equals expectedCurrent.
// This is the core concurrency-safety mechanism — prevents race conditions between workers.
func (r *MediaRepository) UpdateStatus(ctx context.Context, mediaID string, expectedCurrent, next domain.MediaStatus) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: mediaPK(mediaID)},
			"SK": &types.AttributeValueMemberS{Value: mediaSK()},
		},
		UpdateExpression:    aws.String("SET #st = :next, UpdatedAt = :now, GSI2_PK = :gsi2pk"),
		ConditionExpression: aws.String("#st = :current"),
		ExpressionAttributeNames: map[string]string{
			"#st": "Status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":current": &types.AttributeValueMemberS{Value: string(expectedCurrent)},
			":next":    &types.AttributeValueMemberS{Value: string(next)},
			":now":     &types.AttributeValueMemberS{Value: now},
			":gsi2pk":  &types.AttributeValueMemberS{Value: "STATUS#" + string(next)},
		},
	})
	if err != nil {
		return fmt.Errorf("DynamoDB UpdateStatus media %q (%s->%s): %w", mediaID, expectedCurrent, next, err)
	}
	return nil
}

// ListByOwner uses GSI1 to query all media items for a given owner.
// cursor is the base64-encoded last evaluated key from the previous page.
func (r *MediaRepository) ListByOwner(ctx context.Context, ownerID string, limit int, cursor string) ([]*domain.Media, string, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("GSI1"),
		KeyConditionExpression: aws.String("GSI1_PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: ownerGSI1PK(ownerID)},
		},
		Limit:            aws.Int32(int32(limit)),
		ScanIndexForward: aws.Bool(false), // newest first
	}

	if cursor != "" {
		input.ExclusiveStartKey = map[string]types.AttributeValue{
			"PK":      &types.AttributeValueMemberS{Value: cursor},
			"SK":      &types.AttributeValueMemberS{Value: mediaSK()},
			"GSI1_PK": &types.AttributeValueMemberS{Value: ownerGSI1PK(ownerID)},
		}
	}

	out, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("DynamoDB Query ListByOwner %q: %w", ownerID, err)
	}

	items := make([]*domain.Media, 0, len(out.Items))
	for _, raw := range out.Items {
		var mi mediaItem
		if err := attributevalue.UnmarshalMap(raw, &mi); err != nil {
			continue
		}
		m, err := itemToMedia(&mi)
		if err == nil {
			items = append(items, m)
		}
	}

	var nextCursor string
	if out.LastEvaluatedKey != nil {
		if pk, ok := out.LastEvaluatedKey["PK"].(*types.AttributeValueMemberS); ok {
			nextCursor = pk.Value
		}
	}
	return items, nextCursor, nil
}

// Delete transitions a media item to the DELETED state.
func (r *MediaRepository) Delete(ctx context.Context, mediaID string) error {
	return r.UpdateStatus(ctx, mediaID, domain.MediaStatusDeleting, domain.MediaStatusDeleted)
}

// itemToMedia converts a DynamoDB item to a domain.Media struct.
func itemToMedia(item *mediaItem) (*domain.Media, error) {
	createdAt, _ := time.Parse(time.RFC3339Nano, item.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339Nano, item.UpdatedAt)
	return &domain.Media{
		MediaID:     item.MediaID,
		OwnerID:     item.OwnerID,
		DeliveryID:  item.DeliveryID,
		FileName:    item.FileName,
		ContentType: item.ContentType,
		MediaType:   domain.MediaType(item.MediaType),
		Size:        item.Size,
		Checksum:    item.Checksum,
		ObjectKey:   item.ObjectKey,
		Status:      domain.MediaStatus(item.Status),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}
