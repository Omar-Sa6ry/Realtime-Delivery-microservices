# --- localstack init (run once) ---
aws --endpoint-url http://localhost:4566 s3 mb s3://delivery-media-development 2>/dev/null || true
aws --endpoint-url http://localhost:4566 s3api put-bucket-cors --bucket delivery-media-development \
  --cors-configuration file://scripts/cors.json