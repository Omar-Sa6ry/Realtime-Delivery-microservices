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

type versionItem struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"`
	VersionID   string `dynamodbav:"VersionID"`
	MediaID     string `dynamodbav:"MediaID"`
	VersionType string `dynamodbav:"VersionType"`
	ObjectKey   string `dynamodbav:"ObjectKey"`
	ContentType string `dynamodbav:"ContentType"`
	Size        int64  `dynamodbav:"Size"`
	Checksum    string `dynamodbav:"Checksum,omitempty"`
	Width       int32  `dynamodbav:"Width,omitempty"`
	Height      int32  `dynamodbav:"Height,omitempty"`
	DurationMS  int64  `dynamodbav:"DurationMS,omitempty"`
	CreatedAt   string `dynamodbav:"CreatedAt"`
}

// VersionRepository implements ports.VersionRepository using DynamoDB.
type VersionRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewVersionRepository creates a new VersionRepository.
func NewVersionRepository(client *dynamodb.Client, tableName string) *VersionRepository {
	return &VersionRepository{client: client, tableName: tableName}
}

func versionSK(versionType string) string { return "VERSION#" + versionType }

// Create inserts a MediaVersion record under the parent media's PK.
func (r *VersionRepository) Create(ctx context.Context, v *domain.MediaVersion) error {
	now := v.CreatedAt.UTC().Format(time.RFC3339Nano)
	item := versionItem{
		PK:          mediaPK(v.MediaID),
		SK:          versionSK(string(v.VersionType)),
		VersionID:   v.VersionID,
		MediaID:     v.MediaID,
		VersionType: string(v.VersionType),
		ObjectKey:   v.ObjectKey,
		ContentType: v.ContentType,
		Size:        v.Size,
		Checksum:    v.Checksum,
		Width:       v.Width,
		Height:      v.Height,
		DurationMS:  v.DurationMS,
		CreatedAt:   now,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshal version item: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})
	return err
}

// ListByMedia queries all VERSION# items under a media's partition key.
func (r *VersionRepository) ListByMedia(ctx context.Context, mediaID string) ([]*domain.MediaVersion, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: mediaPK(mediaID)},
			":prefix": &types.AttributeValueMemberS{Value: "VERSION#"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("DynamoDB Query versions for media %q: %w", mediaID, err)
	}

	versions := make([]*domain.MediaVersion, 0, len(out.Items))
	for _, raw := range out.Items {
		var vi versionItem
		if err := attributevalue.UnmarshalMap(raw, &vi); err != nil {
			continue
		}
		createdAt, _ := time.Parse(time.RFC3339Nano, vi.CreatedAt)
		versions = append(versions, &domain.MediaVersion{
			VersionID:   vi.VersionID,
			MediaID:     vi.MediaID,
			VersionType: domain.VersionType(vi.VersionType),
			ObjectKey:   vi.ObjectKey,
			ContentType: vi.ContentType,
			Size:        vi.Size,
			Checksum:    vi.Checksum,
			Width:       vi.Width,
			Height:      vi.Height,
			DurationMS:  vi.DurationMS,
			CreatedAt:   createdAt,
		})
	}
	return versions, nil
}
