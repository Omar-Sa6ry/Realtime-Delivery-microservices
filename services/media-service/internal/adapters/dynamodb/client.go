package dynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// If endpoint is non-empty it overrides the AWS endpoint (for DynamoDB Local / LocalStack).
func NewClient(ctx context.Context, region, endpoint, accessKeyID, secretKey string) (*dynamodb.Client, error) {
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
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	var clientOpts []func(*dynamodb.Options)
	if endpoint != "" {
		clientOpts = append(clientOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	return dynamodb.NewFromConfig(cfg, clientOpts...), nil
}

// EnsureTableExists creates the DynamoDB table and GSIs if they do not already exist.
// This is idempotent — calling it multiple times is safe.
func EnsureTableExists(ctx context.Context, client *dynamodb.Client, tableName string) error {
	_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		return nil // table already exists
	}

	_, createErr := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(tableName),
		BillingMode: types.BillingModePayPerRequest,

		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("SK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("GSI1_PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("GSI1_SK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("GSI2_PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("GSI2_SK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("GSI3_PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("GSI3_SK"), AttributeType: types.ScalarAttributeTypeS},
		},

		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("PK"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("SK"), KeyType: types.KeyTypeRange},
		},

		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			// GSI1: query media by owner + status
			{
				IndexName: aws.String("GSI1"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("GSI1_PK"), KeyType: types.KeyTypeHash},
					{AttributeName: aws.String("GSI1_SK"), KeyType: types.KeyTypeRange},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			},
			// GSI2: reconciliation queries (status + createdAt)
			{
				IndexName: aws.String("GSI2"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("GSI2_PK"), KeyType: types.KeyTypeHash},
					{AttributeName: aws.String("GSI2_SK"), KeyType: types.KeyTypeRange},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			},
			// GSI3: outbox publisher (outboxStatus + createdAt)
			{
				IndexName: aws.String("GSI3"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("GSI3_PK"), KeyType: types.KeyTypeHash},
					{AttributeName: aws.String("GSI3_SK"), KeyType: types.KeyTypeRange},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			},
		},
	})

	if createErr != nil {
		return fmt.Errorf("create DynamoDB table %q: %w", tableName, createErr)
	}
	return nil
}
