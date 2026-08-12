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

// outboxItem is the DynamoDB Single-Table record for an OutboxEvent.
type outboxItem struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"`
	GSI3PK      string `dynamodbav:"GSI3_PK"`
	GSI3SK      string `dynamodbav:"GSI3_SK"`
	EventID     string `dynamodbav:"EventID"`
	AggregateID string `dynamodbav:"AggregateID"`
	EventType   string `dynamodbav:"EventType"`
	Payload     []byte `dynamodbav:"Payload"`
	Status      string `dynamodbav:"Status"`
	Attempts    int    `dynamodbav:"Attempts"`
	TraceID     string `dynamodbav:"TraceID,omitempty"`
	CreatedAt   string `dynamodbav:"CreatedAt"`
	TTL         int64  `dynamodbav:"TTL"`
}

// OutboxRepository implements ports.OutboxRepository using DynamoDB.
type OutboxRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewOutboxRepository creates a new OutboxRepository.
func NewOutboxRepository(client *dynamodb.Client, tableName string) *OutboxRepository {
	return &OutboxRepository{client: client, tableName: tableName}
}

func outboxPK(eventID string) string { return "OUTBOX#" + eventID }
func outboxSK() string               { return "EVENT" }

// Create inserts an outbox event into DynamoDB.
// In production, this is called inside a DynamoDB TransactWriteItems alongside the media state change.
// For simplicity, a regular PutItem is used here; callers that need transactional writes should
// use the raw DynamoDB client and build the TransactWriteItems themselves.
func (r *OutboxRepository) Create(ctx context.Context, event *domain.OutboxEvent) error {
	now := event.CreatedAt.UTC().Format(time.RFC3339Nano)
	item := outboxItem{
		PK:          outboxPK(event.EventID),
		SK:          outboxSK(),
		GSI3PK:      "OUTBOX_STATUS#" + string(event.Status),
		GSI3SK:      "CREATED#" + now,
		EventID:     event.EventID,
		AggregateID: event.AggregateID,
		EventType:   event.EventType,
		Payload:     event.Payload,
		Status:      string(event.Status),
		Attempts:    event.Attempts,
		TraceID:     event.TraceID,
		CreatedAt:   now,
		TTL:         event.TTL,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshal outbox item: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("DynamoDB PutItem outbox event %q: %w", event.EventID, err)
	}
	return nil
}

// ListPending queries GSI3 for PENDING outbox events ordered by creation time.
func (r *OutboxRepository) ListPending(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("GSI3"),
		KeyConditionExpression: aws.String("GSI3_PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "OUTBOX_STATUS#PENDING"},
		},
		Limit:            aws.Int32(int32(limit)),
		ScanIndexForward: aws.Bool(true), // oldest first — process in order
	})
	if err != nil {
		return nil, fmt.Errorf("DynamoDB Query outbox PENDING: %w", err)
	}

	events := make([]*domain.OutboxEvent, 0, len(out.Items))
	for _, raw := range out.Items {
		var oi outboxItem
		if err := attributevalue.UnmarshalMap(raw, &oi); err != nil {
			continue
		}
		createdAt, _ := time.Parse(time.RFC3339Nano, oi.CreatedAt)
		events = append(events, &domain.OutboxEvent{
			EventID:     oi.EventID,
			AggregateID: oi.AggregateID,
			EventType:   oi.EventType,
			Payload:     oi.Payload,
			Status:      domain.OutboxStatus(oi.Status),
			Attempts:    oi.Attempts,
			TraceID:     oi.TraceID,
			CreatedAt:   createdAt,
			TTL:         oi.TTL,
		})
	}
	return events, nil
}

// MarkPublished atomically transitions an outbox event from PENDING to PUBLISHED.
func (r *OutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: outboxPK(eventID)},
			"SK": &types.AttributeValueMemberS{Value: outboxSK()},
		},
		UpdateExpression:    aws.String("SET #st = :published, GSI3_PK = :gsi3pk, PublishedAt = :now"),
		ConditionExpression: aws.String("#st = :pending"),
		ExpressionAttributeNames: map[string]string{"#st": "Status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":published": &types.AttributeValueMemberS{Value: "PUBLISHED"},
			":pending":   &types.AttributeValueMemberS{Value: "PENDING"},
			":gsi3pk":    &types.AttributeValueMemberS{Value: "OUTBOX_STATUS#PUBLISHED"},
			":now":       &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		return fmt.Errorf("DynamoDB MarkPublished outbox %q: %w", eventID, err)
	}
	return nil
}

// MarkFailed transitions an outbox event to FAILED after exhausting retries.
func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID, reason string) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: outboxPK(eventID)},
			"SK": &types.AttributeValueMemberS{Value: outboxSK()},
		},
		UpdateExpression: aws.String("SET #st = :failed, GSI3_PK = :gsi3pk, LastError = :reason, Attempts = Attempts + :one"),
		ExpressionAttributeNames: map[string]string{"#st": "Status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":failed":  &types.AttributeValueMemberS{Value: "FAILED"},
			":gsi3pk":  &types.AttributeValueMemberS{Value: "OUTBOX_STATUS#FAILED"},
			":reason":  &types.AttributeValueMemberS{Value: reason},
			":one":     &types.AttributeValueMemberN{Value: "1"},
		},
	})
	if err != nil {
		return fmt.Errorf("DynamoDB MarkFailed outbox %q: %w", eventID, err)
	}
	return nil
}
