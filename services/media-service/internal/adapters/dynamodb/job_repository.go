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

type jobItem struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"`
	GSI2PK      string `dynamodbav:"GSI2_PK"`
	GSI2SK      string `dynamodbav:"GSI2_SK"`
	JobID       string `dynamodbav:"JobID"`
	MediaID     string `dynamodbav:"MediaID"`
	JobType     string `dynamodbav:"JobType"`
	Status      string `dynamodbav:"Status"`
	Attempts    int    `dynamodbav:"Attempts"`
	MaxAttempts int    `dynamodbav:"MaxAttempts"`
	LastError   string `dynamodbav:"LastError,omitempty"`
	StartedAt   string `dynamodbav:"StartedAt,omitempty"`
	CompletedAt string `dynamodbav:"CompletedAt,omitempty"`
	CreatedAt   string `dynamodbav:"CreatedAt"`
	UpdatedAt   string `dynamodbav:"UpdatedAt"`
}

// JobRepository implements ports.JobRepository using DynamoDB.
type JobRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewJobRepository creates a new JobRepository.
func NewJobRepository(client *dynamodb.Client, tableName string) *JobRepository {
	return &JobRepository{client: client, tableName: tableName}
}

func jobPK(jobID string) string { return "JOB#" + jobID }
func jobSK() string             { return "JOB" }

func (r *JobRepository) Create(ctx context.Context, j *domain.MediaJob) error {
	now := j.CreatedAt.UTC().Format(time.RFC3339Nano)
	item := jobItem{
		PK:          jobPK(j.JobID),
		SK:          jobSK(),
		GSI2PK:      "JOB_STATUS#" + string(j.Status),
		GSI2SK:      "CREATED#" + now,
		JobID:       j.JobID,
		MediaID:     j.MediaID,
		JobType:     string(j.JobType),
		Status:      string(j.Status),
		Attempts:    j.Attempts,
		MaxAttempts: j.MaxAttempts,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshal job item: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})
	return err
}

func (r *JobRepository) GetByID(ctx context.Context, jobID string) (*domain.MediaJob, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: jobPK(jobID)},
			"SK": &types.AttributeValueMemberS{Value: jobSK()},
		},
		TableName: aws.String(r.tableName),
	})
	if err != nil {
		return nil, fmt.Errorf("DynamoDB GetItem job %q: %w", jobID, err)
	}
	if out.Item == nil {
		return nil, domain.ErrJobNotFound
	}

	var item jobItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshal job item: %w", err)
	}
	return itemToJob(&item), nil
}

func (r *JobRepository) UpdateStatus(ctx context.Context, jobID string, status domain.JobStatus, lastError string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	
	updateExpr := "SET #st = :st, GSI2_PK = :gsi2pk, UpdatedAt = :now, Attempts = Attempts + :one"
	exprValues := map[string]types.AttributeValue{
		":st":     &types.AttributeValueMemberS{Value: string(status)},
		":gsi2pk": &types.AttributeValueMemberS{Value: "JOB_STATUS#" + string(status)},
		":now":    &types.AttributeValueMemberS{Value: now},
		":one":    &types.AttributeValueMemberN{Value: "1"},
	}

	if lastError != "" {
		updateExpr += ", LastError = :err"
		exprValues[":err"] = &types.AttributeValueMemberS{Value: lastError}
	}

	if status == domain.JobStatusRunning {
		updateExpr += ", StartedAt = :now"
	} else if status == domain.JobStatusCompleted || status == domain.JobStatusFailed {
		updateExpr += ", CompletedAt = :now"
	}

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: jobPK(jobID)},
			"SK": &types.AttributeValueMemberS{Value: jobSK()},
		},
		UpdateExpression:    aws.String(updateExpr),
		ExpressionAttributeNames: map[string]string{"#st": "Status"},
		ExpressionAttributeValues: exprValues,
		TableName:           aws.String(r.tableName),
	})
	return err
}

func (r *JobRepository) ListStuck(ctx context.Context, stuckBefore time.Time) ([]*domain.MediaJob, error) {
	stuckStr := stuckBefore.UTC().Format(time.RFC3339Nano)
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		IndexName:              aws.String("GSI2"),
		KeyConditionExpression: aws.String("GSI2_PK = :pk AND GSI2_SK <= :sk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "JOB_STATUS#RUNNING"},
			":sk": &types.AttributeValueMemberS{Value: "CREATED#" + stuckStr}, // using GSI2_SK
		},
		TableName: aws.String(r.tableName),
	})
	if err != nil {
		return nil, fmt.Errorf("DynamoDB Query stuck jobs: %w", err)
	}

	jobs := make([]*domain.MediaJob, 0, len(out.Items))
	for _, raw := range out.Items {
		var item jobItem
		if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
			continue
		}
		jobs = append(jobs, itemToJob(&item))
	}
	return jobs, nil
}

func itemToJob(item *jobItem) *domain.MediaJob {
	createdAt, _ := time.Parse(time.RFC3339Nano, item.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339Nano, item.UpdatedAt)
	startedAt, _ := time.Parse(time.RFC3339Nano, item.StartedAt)
	completedAt, _ := time.Parse(time.RFC3339Nano, item.CompletedAt)

	return &domain.MediaJob{
		JobID:       item.JobID,
		MediaID:     item.MediaID,
		JobType:     domain.JobType(item.JobType),
		Status:      domain.JobStatus(item.Status),
		Attempts:    item.Attempts,
		MaxAttempts: item.MaxAttempts,
		LastError:   item.LastError,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}
