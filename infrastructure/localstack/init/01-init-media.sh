#!/bin/sh
set -eu

REGION="${AWS_DEFAULT_REGION:-us-east-1}"
BUCKET="${S3_BUCKET_NAME:-delivery-media-development}"
TABLE="${DYNAMODB_TABLE_NAME:-media-service}"

awslocal s3 mb "s3://${BUCKET}" --region "${REGION}" 2>/dev/null || true

if ! awslocal dynamodb describe-table --table-name "${TABLE}" --region "${REGION}" >/dev/null 2>&1; then
  awslocal dynamodb create-table \
    --table-name "${TABLE}" \
    --billing-mode PAY_PER_REQUEST \
    --attribute-definitions \
      AttributeName=PK,AttributeType=S \
      AttributeName=SK,AttributeType=S \
      AttributeName=GSI1_PK,AttributeType=S \
      AttributeName=GSI1_SK,AttributeType=S \
      AttributeName=GSI2_PK,AttributeType=S \
      AttributeName=GSI2_SK,AttributeType=S \
    --key-schema \
      AttributeName=PK,KeyType=HASH \
      AttributeName=SK,KeyType=RANGE \
    --global-secondary-indexes '[
      {"IndexName":"GSI1","KeySchema":[{"AttributeName":"GSI1_PK","KeyType":"HASH"},{"AttributeName":"GSI1_SK","KeyType":"RANGE"}],"Projection":{"ProjectionType":"ALL"}},
      {"IndexName":"GSI2","KeySchema":[{"AttributeName":"GSI2_PK","KeyType":"HASH"},{"AttributeName":"GSI2_SK","KeyType":"RANGE"}],"Projection":{"ProjectionType":"ALL"}}
    ]' \
    --region "${REGION}"
fi

echo "LocalStack initialized: bucket=${BUCKET}, table=${TABLE}"
