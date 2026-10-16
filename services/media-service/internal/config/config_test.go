package config

import (
	"strings"
	"testing"
	"time"
)

// loadEnvKeys lists every environment variable consulted by Load (plus
// NATS_URL, read by Config.NATSURL). Tests clear all of them so host
// environment values cannot influence assertions.
var loadEnvKeys = []string{
	"PORT_MEDIA_GRAPHQL",
	"PORT_MEDIA_METRICS",
	"PORT_MEDIA_WS",
	"PORT_MEDIA_GRPC",
	"NODE_ENV",
	"AWS_REGION",
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"S3_BUCKET_NAME",
	"AWS_ENDPOINT",
	"S3_PUBLIC_ENDPOINT",
	"S3_PRESIGNED_URL_EXPIRY",
	"S3_MULTIPART_MIN_PART_SIZE",
	"S3_LIFECYCLE_ABORT_DAYS",
	"DYNAMODB_TABLE_NAME",
	"DYNAMODB_ENDPOINT",
	"REDIS_HOST",
	"REDIS_PORT",
	"REDIS_PASSWORD",
	"REDIS_TTL",
	"KAFKA_BROKERS",
	"KAFKA_GROUP_ID",
	"KAFKA_TOPIC_PREFIX",
	"CLAMAV_HOST",
	"CLAMAV_PORT",
	"FFMPEG_BINARY",
	"MEDIA_MAX_FILE_SIZE_BYTES",
	"MEDIA_MAX_CONCURRENT_UPLOADS",
	"MEDIA_STORAGE_QUOTA_BYTES",
	"MEDIA_ALLOWED_TYPES",
	"MEDIA_UPLOAD_SESSION_TTL",
	"MEDIA_SCAN_WORKERS",
	"MEDIA_IMAGE_WORKERS",
	"MEDIA_VIDEO_WORKERS",
	"MEDIA_DELETE_WORKERS",
	"MEDIA_OUTBOX_WORKERS",
	"MEDIA_COMPRESSION_WORKERS",
	"MEDIA_METADATA_WORKERS",
	"MEDIA_RECONCILE_INTERVAL",
	"MEDIA_STUCK_UPLOAD_TIMEOUT",
	"MEDIA_STUCK_PROCESSING_TIMEOUT",
	"MEDIA_RATE_LIMIT_UPLOAD_PER_MINUTE",
	"MEDIA_RATE_LIMIT_DOWNLOAD_PER_MINUTE",
	"MEDIA_WS_RATE_LIMIT_PER_SECOND",
	"MEDIA_WS_RATE_LIMIT_BURST",
	"JAEGER_ENDPOINT",
	"OTEL_EXPORTER_OTLP_ENDPOINT",
	"TRACE_SAMPLE_RATIO",
	"NATS_URL",
}

func clearLoadEnv(t *testing.T) {
	t.Helper()
	for _, k := range loadEnvKeys {
		t.Setenv(k, "")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearLoadEnv(t)
	t.Setenv("S3_BUCKET_NAME", "test-bucket")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if c.GraphQLPort != "4005" {
		t.Errorf("GraphQLPort = %q, want 4005", c.GraphQLPort)
	}
	if c.MetricsPort != "9102" {
		t.Errorf("MetricsPort = %q, want 9102", c.MetricsPort)
	}
	if c.WSPort != "4003" {
		t.Errorf("WSPort = %q, want 4003", c.WSPort)
	}
	if c.GRPCPort != "50052" {
		t.Errorf("GRPCPort = %q, want 50052", c.GRPCPort)
	}
	if c.Environment != "development" {
		t.Errorf("Environment = %q, want development", c.Environment)
	}
	if c.AWSRegion != "us-east-1" {
		t.Errorf("AWSRegion = %q, want us-east-1", c.AWSRegion)
	}
	if c.AWSAccessKeyID != "" || c.AWSSecretAccessKey != "" {
		t.Errorf("AWS credentials should default to empty, got %q/%q", c.AWSAccessKeyID, c.AWSSecretAccessKey)
	}
	if c.S3BucketName != "test-bucket" {
		t.Errorf("S3BucketName = %q, want test-bucket", c.S3BucketName)
	}
	if c.S3Endpoint != "http://localstack-srv:4566" {
		t.Errorf("S3Endpoint = %q", c.S3Endpoint)
	}
	if c.S3PublicEndpoint != "" {
		t.Errorf("S3PublicEndpoint = %q, want empty", c.S3PublicEndpoint)
	}
	if c.S3PresignedURLExpiry != 3600*time.Second {
		t.Errorf("S3PresignedURLExpiry = %v, want 3600s", c.S3PresignedURLExpiry)
	}
	if c.S3MultipartMinPartSize != 5242880 {
		t.Errorf("S3MultipartMinPartSize = %d, want 5242880", c.S3MultipartMinPartSize)
	}
	if c.S3LifecycleAbortDays != 7 {
		t.Errorf("S3LifecycleAbortDays = %d, want 7", c.S3LifecycleAbortDays)
	}
	if c.DynamoDBTableName != "media-service" {
		t.Errorf("DynamoDBTableName = %q", c.DynamoDBTableName)
	}
	if c.DynamoDBEndpoint != "http://localstack-srv:4566" {
		t.Errorf("DynamoDBEndpoint = %q", c.DynamoDBEndpoint)
	}
	if c.RedisHost != "redis-srv" || c.RedisPort != "6379" {
		t.Errorf("Redis = %s:%s, want redis-srv:6379", c.RedisHost, c.RedisPort)
	}
	if c.RedisPassword != "" {
		t.Errorf("RedisPassword = %q, want empty", c.RedisPassword)
	}
	if c.RedisTTL != 3600*time.Second {
		t.Errorf("RedisTTL = %v, want 3600s", c.RedisTTL)
	}
	if len(c.KafkaBrokers) != 1 || c.KafkaBrokers[0] != "kafka-srv:9092" {
		t.Errorf("KafkaBrokers = %v, want [kafka-srv:9092]", c.KafkaBrokers)
	}
	if c.KafkaGroupID != "media-service" || c.KafkaTopicPrefix != "media" {
		t.Errorf("Kafka group/prefix = %q/%q", c.KafkaGroupID, c.KafkaTopicPrefix)
	}
	if c.ClamAVHost != "clamav-srv" || c.ClamAVPort != "3310" {
		t.Errorf("ClamAV = %s:%s", c.ClamAVHost, c.ClamAVPort)
	}
	if c.FFmpegBinary != "/usr/bin/ffmpeg" {
		t.Errorf("FFmpegBinary = %q", c.FFmpegBinary)
	}
	if c.MaxFileSizeBytes != 53687091200 {
		t.Errorf("MaxFileSizeBytes = %d", c.MaxFileSizeBytes)
	}
	if c.MaxConcurrentUploads != 5 {
		t.Errorf("MaxConcurrentUploads = %d, want 5", c.MaxConcurrentUploads)
	}
	if c.StorageQuotaBytes != 53687091200 {
		t.Errorf("StorageQuotaBytes = %d", c.StorageQuotaBytes)
	}
	if len(c.AllowedContentTypes) == 0 {
		t.Error("AllowedContentTypes should not be empty by default")
	}
	if !c.IsAllowedContentType("image/png") || !c.IsAllowedContentType("application/pdf") {
		t.Errorf("default allowed types missing expected entries: %v", c.AllowedContentTypes)
	}
	if c.IsAllowedContentType("text/html") {
		t.Error("text/html must not be allowed by default")
	}
	if c.UploadSessionTTL != 86400*time.Second {
		t.Errorf("UploadSessionTTL = %v, want 24h", c.UploadSessionTTL)
	}
	if c.ScanWorkers != 4 || c.ImageWorkers != 4 || c.VideoWorkers != 2 || c.DeleteWorkers != 4 {
		t.Errorf("worker sizes = scan:%d image:%d video:%d delete:%d", c.ScanWorkers, c.ImageWorkers, c.VideoWorkers, c.DeleteWorkers)
	}
	if c.OutboxWorkers != 2 || c.CompressionWorkers != 4 || c.MetadataWorkers != 4 {
		t.Errorf("worker sizes = outbox:%d compression:%d metadata:%d", c.OutboxWorkers, c.CompressionWorkers, c.MetadataWorkers)
	}
	if c.ReconcileInterval != 300*time.Second {
		t.Errorf("ReconcileInterval = %v, want 300s", c.ReconcileInterval)
	}
	if c.StuckUploadTimeout != 3600*time.Second || c.StuckProcessingTimeout != 7200*time.Second {
		t.Errorf("stuck timeouts = %v/%v", c.StuckUploadTimeout, c.StuckProcessingTimeout)
	}
	if c.RateLimitUploadPerMinute != 10 || c.RateLimitDownloadPerMinute != 60 {
		t.Errorf("rate limits = %d/%d", c.RateLimitUploadPerMinute, c.RateLimitDownloadPerMinute)
	}
	if c.WSRateLimitPerSecond != 50 || c.WSRateLimitBurst != 100 {
		t.Errorf("ws rate limit = %v burst %d", c.WSRateLimitPerSecond, c.WSRateLimitBurst)
	}
	if c.JaegerEndpoint != "" || c.OTLPEndpoint != "" {
		t.Errorf("telemetry endpoints should default empty, got %q/%q", c.JaegerEndpoint, c.OTLPEndpoint)
	}
	if c.TraceSampleRatio != 0.1 {
		t.Errorf("TraceSampleRatio = %v, want 0.1", c.TraceSampleRatio)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	clearLoadEnv(t)
	overrides := map[string]string{
		"PORT_MEDIA_GRAPHQL":                   "8001",
		"PORT_MEDIA_METRICS":                   "9103",
		"PORT_MEDIA_WS":                        "8002",
		"PORT_MEDIA_GRPC":                      "50053",
		"NODE_ENV":                             "production",
		"AWS_REGION":                           "eu-west-1",
		"AWS_ACCESS_KEY_ID":                    "AKIA_TEST",
		"AWS_SECRET_ACCESS_KEY":                "secret",
		"S3_BUCKET_NAME":                       "bucket-1",
		"AWS_ENDPOINT":                         "http://localhost:4566",
		"S3_PUBLIC_ENDPOINT":                   "https://cdn.example.com",
		"S3_PRESIGNED_URL_EXPIRY":              "900",
		"S3_MULTIPART_MIN_PART_SIZE":           "8388608",
		"S3_LIFECYCLE_ABORT_DAYS":              "30",
		"DYNAMODB_TABLE_NAME":                  "media-test",
		"DYNAMODB_ENDPOINT":                    "http://ddb:8000",
		"REDIS_HOST":                           "redis.local",
		"REDIS_PORT":                           "6380",
		"REDIS_PASSWORD":                       "pw",
		"REDIS_TTL":                            "120",
		"KAFKA_BROKERS":                        "k1:9092,k2:9092",
		"KAFKA_GROUP_ID":                       "grp",
		"KAFKA_TOPIC_PREFIX":                   "tp",
		"CLAMAV_HOST":                          "clam.local",
		"CLAMAV_PORT":                          "3311",
		"FFMPEG_BINARY":                        "/opt/ffmpeg",
		"MEDIA_MAX_FILE_SIZE_BYTES":            "1000",
		"MEDIA_MAX_CONCURRENT_UPLOADS":         "3",
		"MEDIA_STORAGE_QUOTA_BYTES":            "2000",
		"MEDIA_ALLOWED_TYPES":                  " image/png , image/gif ,, text/plain ",
		"MEDIA_UPLOAD_SESSION_TTL":             "60",
		"MEDIA_SCAN_WORKERS":                   "1",
		"MEDIA_IMAGE_WORKERS":                  "2",
		"MEDIA_VIDEO_WORKERS":                  "3",
		"MEDIA_DELETE_WORKERS":                 "4",
		"MEDIA_OUTBOX_WORKERS":                 "5",
		"MEDIA_COMPRESSION_WORKERS":            "6",
		"MEDIA_METADATA_WORKERS":               "7",
		"MEDIA_RECONCILE_INTERVAL":             "10",
		"MEDIA_STUCK_UPLOAD_TIMEOUT":           "20",
		"MEDIA_STUCK_PROCESSING_TIMEOUT":       "30",
		"MEDIA_RATE_LIMIT_UPLOAD_PER_MINUTE":   "1",
		"MEDIA_RATE_LIMIT_DOWNLOAD_PER_MINUTE": "2",
		"MEDIA_WS_RATE_LIMIT_PER_SECOND":       "12.5",
		"MEDIA_WS_RATE_LIMIT_BURST":            "7",
		"JAEGER_ENDPOINT":                      "jaeger:6831",
		"OTEL_EXPORTER_OTLP_ENDPOINT":          "otel:4317",
		"TRACE_SAMPLE_RATIO":                   "0.5",
		"NATS_URL":                             "nats://custom:4222",
	}
	for k, v := range overrides {
		t.Setenv(k, v)
	}

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if c.GraphQLPort != "8001" || c.MetricsPort != "9103" || c.WSPort != "8002" || c.GRPCPort != "50053" {
		t.Errorf("ports = %q/%q/%q/%q", c.GraphQLPort, c.MetricsPort, c.WSPort, c.GRPCPort)
	}
	if c.Environment != "production" {
		t.Errorf("Environment = %q, want production", c.Environment)
	}
	if c.AWSRegion != "eu-west-1" || c.AWSAccessKeyID != "AKIA_TEST" || c.AWSSecretAccessKey != "secret" {
		t.Errorf("AWS wiring = %q/%q/%q", c.AWSRegion, c.AWSAccessKeyID, c.AWSSecretAccessKey)
	}
	if c.S3BucketName != "bucket-1" || c.S3Endpoint != "http://localhost:4566" || c.S3PublicEndpoint != "https://cdn.example.com" {
		t.Errorf("S3 wiring = %q/%q/%q", c.S3BucketName, c.S3Endpoint, c.S3PublicEndpoint)
	}
	if c.S3PresignedURLExpiry != 900*time.Second {
		t.Errorf("S3PresignedURLExpiry = %v", c.S3PresignedURLExpiry)
	}
	if c.S3MultipartMinPartSize != 8388608 {
		t.Errorf("S3MultipartMinPartSize = %d", c.S3MultipartMinPartSize)
	}
	if c.S3LifecycleAbortDays != 30 {
		t.Errorf("S3LifecycleAbortDays = %d", c.S3LifecycleAbortDays)
	}
	if c.DynamoDBTableName != "media-test" || c.DynamoDBEndpoint != "http://ddb:8000" {
		t.Errorf("DynamoDB wiring = %q/%q", c.DynamoDBTableName, c.DynamoDBEndpoint)
	}
	if c.RedisHost != "redis.local" || c.RedisPort != "6380" || c.RedisPassword != "pw" || c.RedisTTL != 120*time.Second {
		t.Errorf("Redis wiring = %s:%s ttl %v", c.RedisHost, c.RedisPort, c.RedisTTL)
	}
	if got := c.RedisAddr(); got != "redis.local:6380" {
		t.Errorf("RedisAddr() = %q, want redis.local:6380", got)
	}
	if len(c.KafkaBrokers) != 2 || c.KafkaBrokers[0] != "k1:9092" || c.KafkaBrokers[1] != "k2:9092" {
		t.Errorf("KafkaBrokers = %v", c.KafkaBrokers)
	}
	if c.KafkaGroupID != "grp" || c.KafkaTopicPrefix != "tp" {
		t.Errorf("Kafka group/prefix = %q/%q", c.KafkaGroupID, c.KafkaTopicPrefix)
	}
	if c.ClamAVHost != "clam.local" || c.ClamAVPort != "3311" || c.FFmpegBinary != "/opt/ffmpeg" {
		t.Errorf("ClamAV/FFmpeg wiring = %s:%s %q", c.ClamAVHost, c.ClamAVPort, c.FFmpegBinary)
	}
	if c.MaxFileSizeBytes != 1000 || c.MaxConcurrentUploads != 3 || c.StorageQuotaBytes != 2000 {
		t.Errorf("upload limits = %d/%d/%d", c.MaxFileSizeBytes, c.MaxConcurrentUploads, c.StorageQuotaBytes)
	}
	if !c.IsAllowedContentType("image/png") || !c.IsAllowedContentType("image/gif") || !c.IsAllowedContentType("text/plain") {
		t.Errorf("AllowedContentTypes = %v, want image/png, image/gif and text/plain", c.AllowedContentTypes)
	}
	if c.IsAllowedContentType("image/jpeg") {
		t.Error("image/jpeg was not in the override list")
	}
	if c.UploadSessionTTL != 60*time.Second {
		t.Errorf("UploadSessionTTL = %v", c.UploadSessionTTL)
	}
	if c.ScanWorkers != 1 || c.ImageWorkers != 2 || c.VideoWorkers != 3 || c.DeleteWorkers != 4 ||
		c.OutboxWorkers != 5 || c.CompressionWorkers != 6 || c.MetadataWorkers != 7 {
		t.Errorf("worker sizes = %d/%d/%d/%d/%d/%d/%d",
			c.ScanWorkers, c.ImageWorkers, c.VideoWorkers, c.DeleteWorkers,
			c.OutboxWorkers, c.CompressionWorkers, c.MetadataWorkers)
	}
	if c.ReconcileInterval != 10*time.Second || c.StuckUploadTimeout != 20*time.Second || c.StuckProcessingTimeout != 30*time.Second {
		t.Errorf("reconciliation = %v/%v/%v", c.ReconcileInterval, c.StuckUploadTimeout, c.StuckProcessingTimeout)
	}
	if c.RateLimitUploadPerMinute != 1 || c.RateLimitDownloadPerMinute != 2 {
		t.Errorf("rate limits = %d/%d", c.RateLimitUploadPerMinute, c.RateLimitDownloadPerMinute)
	}
	if c.WSRateLimitPerSecond != 12.5 || c.WSRateLimitBurst != 7 {
		t.Errorf("ws rate limit = %v burst %d", c.WSRateLimitPerSecond, c.WSRateLimitBurst)
	}
	if c.JaegerEndpoint != "jaeger:6831" || c.OTLPEndpoint != "otel:4317" || c.TraceSampleRatio != 0.5 {
		t.Errorf("telemetry = %q/%q/%v", c.JaegerEndpoint, c.OTLPEndpoint, c.TraceSampleRatio)
	}
	if got := c.NATSURL(); got != "nats://custom:4222" {
		t.Errorf("NATSURL() = %q, want nats://custom:4222", got)
	}
}

func TestLoad_InvalidNumericEnv(t *testing.T) {
	cases := []string{
		"S3_PRESIGNED_URL_EXPIRY",
		"S3_MULTIPART_MIN_PART_SIZE",
		"S3_LIFECYCLE_ABORT_DAYS",
		"REDIS_TTL",
		"MEDIA_MAX_FILE_SIZE_BYTES",
		"MEDIA_MAX_CONCURRENT_UPLOADS",
		"MEDIA_STORAGE_QUOTA_BYTES",
		"MEDIA_UPLOAD_SESSION_TTL",
		"MEDIA_SCAN_WORKERS",
		"MEDIA_IMAGE_WORKERS",
		"MEDIA_VIDEO_WORKERS",
		"MEDIA_DELETE_WORKERS",
		"MEDIA_OUTBOX_WORKERS",
		"MEDIA_COMPRESSION_WORKERS",
		"MEDIA_METADATA_WORKERS",
		"MEDIA_RECONCILE_INTERVAL",
		"MEDIA_STUCK_UPLOAD_TIMEOUT",
		"MEDIA_STUCK_PROCESSING_TIMEOUT",
		"MEDIA_RATE_LIMIT_UPLOAD_PER_MINUTE",
		"MEDIA_RATE_LIMIT_DOWNLOAD_PER_MINUTE",
		"MEDIA_WS_RATE_LIMIT_PER_SECOND",
		"MEDIA_WS_RATE_LIMIT_BURST",
		"TRACE_SAMPLE_RATIO",
	}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			clearLoadEnv(t)
			t.Setenv("S3_BUCKET_NAME", "test-bucket")
			t.Setenv(key, "not-a-number")

			c, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want error for invalid %s", key)
			}
			if c != nil {
				t.Errorf("Load() config = %v, want nil on error", c)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error %q does not mention %s", err, key)
			}
		})
	}
}

func TestLoad_MissingRequiredBucketPanics(t *testing.T) {
	clearLoadEnv(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for missing S3_BUCKET_NAME")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "S3_BUCKET_NAME") {
			t.Errorf("panic = %v, want message mentioning S3_BUCKET_NAME", r)
		}
	}()

	_, _ = Load()
}

func TestConfig_IsDev(t *testing.T) {
	cases := []struct {
		environment string
		want        bool
	}{
		{"development", true},
		{"staging", true},
		{"production", false},
		{"", true},
	}
	for _, tc := range cases {
		t.Run("env_"+tc.environment, func(t *testing.T) {
			c := &Config{Environment: tc.environment}
			if got := c.IsDev(); got != tc.want {
				t.Errorf("IsDev() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestConfig_NATSURL_Default(t *testing.T) {
	clearLoadEnv(t)
	c := &Config{}
	if got := c.NATSURL(); got != "nats://nats-srv:4222" {
		t.Errorf("NATSURL() = %q, want nats://nats-srv:4222", got)
	}
}

func TestConfig_IsAllowedContentType(t *testing.T) {
	c := &Config{AllowedContentTypes: parseCSVSet("image/png, text/plain")}
	if !c.IsAllowedContentType("image/png") {
		t.Error("IsAllowedContentType(image/png) = false, want true")
	}
	if c.IsAllowedContentType("image/gif") {
		t.Error("IsAllowedContentType(image/gif) = true, want false")
	}
}

func TestParseCSVSet(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want map[string]struct{}
	}{
		{"empty", "", map[string]struct{}{}},
		{"single", "image/png", map[string]struct{}{"image/png": {}}},
		{"with_spaces_and_empties", "a, b ,,c,", map[string]struct{}{"a": {}, "b": {}, "c": {}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCSVSet(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("parseCSVSet(%q) = %v, want %v", tc.raw, got, tc.want)
			}
			for k := range tc.want {
				if _, ok := got[k]; !ok {
					t.Errorf("parseCSVSet(%q) missing %q", tc.raw, k)
				}
			}
		})
	}
}
