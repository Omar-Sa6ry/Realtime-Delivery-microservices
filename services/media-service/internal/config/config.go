package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the media-service.
// Values are loaded from environment variables; no defaults are hard-coded
// for sensitive fields. Non-sensitive fields carry reasonable defaults.
type Config struct {
	// Server
	GraphQLPort  string // GraphQL federation subgraph port (default :4005)
	MetricsPort  string
	WSPort       string // WebSocket server port (default :4003)
	Environment  string

	// AWS
	AWSRegion          string
	AWSAccessKeyID     string
	AWSSecretAccessKey string

	// S3
	S3BucketName          string
	S3PresignedURLExpiry  time.Duration
	S3MultipartMinPartSize int64
	S3LifecycleAbortDays  int

	// DynamoDB
	DynamoDBTableName  string
	DynamoDBEndpoint   string // empty = real AWS; set for localstack/local

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisTTL      time.Duration

	// Kafka
	KafkaBrokers     []string
	KafkaGroupID     string
	KafkaTopicPrefix string

	// ClamAV (Phase 6)
	ClamAVHost string
	ClamAVPort string

	// FFmpeg (Phase 6)
	FFmpegBinary string

	// Upload limits
	MaxFileSizeBytes         int64
	MaxConcurrentUploads     int
	StorageQuotaBytes        int64
	AllowedContentTypes      map[string]struct{}
	UploadSessionTTL         time.Duration

	// Worker pool sizes
	ScanWorkers         int
	ImageWorkers        int
	VideoWorkers        int
	DeleteWorkers       int
	OutboxWorkers       int
	CompressionWorkers  int
	MetadataWorkers     int

	// Reconciliation
	ReconcileInterval         time.Duration
	StuckUploadTimeout        time.Duration
	StuckProcessingTimeout    time.Duration

	// Rate limiting (per user, per minute)
	RateLimitUploadPerMinute   int
	RateLimitDownloadPerMinute int
}

// Load reads all required configuration values from environment variables.
// It returns an error if any required field is missing or invalid.
func Load() (*Config, error) {
	c := &Config{}

	// Server
	c.GraphQLPort = getEnvOrDefault("PORT_MEDIA_GRAPHQL", "4005")
	c.MetricsPort = getEnvOrDefault("PORT_MEDIA_METRICS", "9102")
	c.WSPort = getEnvOrDefault("PORT_MEDIA_WS", "4003")
	c.Environment = getEnvOrDefault("NODE_ENV", "development")

	// AWS
	c.AWSRegion = getEnvOrDefault("AWS_REGION", "us-east-1")
	c.AWSAccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	c.AWSSecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")

	// S3
	c.S3BucketName = requireEnv("S3_BUCKET_NAME")
	expirySec, err := getEnvInt("S3_PRESIGNED_URL_EXPIRY", 3600)
	if err != nil {
		return nil, fmt.Errorf("S3_PRESIGNED_URL_EXPIRY: %w", err)
	}
	c.S3PresignedURLExpiry = time.Duration(expirySec) * time.Second

	c.S3MultipartMinPartSize, err = getEnvInt64("S3_MULTIPART_MIN_PART_SIZE", 5242880) // 5 MB
	if err != nil {
		return nil, fmt.Errorf("S3_MULTIPART_MIN_PART_SIZE: %w", err)
	}

	c.S3LifecycleAbortDays, err = getEnvInt("S3_LIFECYCLE_ABORT_DAYS", 7)
	if err != nil {
		return nil, fmt.Errorf("S3_LIFECYCLE_ABORT_DAYS: %w", err)
	}

	// DynamoDB
	c.DynamoDBTableName = getEnvOrDefault("DYNAMODB_TABLE_NAME", "media-service")
	// Default to the in-cluster LocalStack endpoint. Override with a real AWS
	// endpoint (no value or empty string) for production deployments.
	c.DynamoDBEndpoint = getEnvOrDefault("DYNAMODB_ENDPOINT", "http://localstack-srv:4566")

	// Redis
	c.RedisHost = getEnvOrDefault("REDIS_HOST", "redis-srv")
	c.RedisPort = getEnvOrDefault("REDIS_PORT", "6379")
	c.RedisPassword = os.Getenv("REDIS_PASSWORD")
	redisTTLSec, err := getEnvInt("REDIS_TTL", 3600)
	if err != nil {
		return nil, fmt.Errorf("REDIS_TTL: %w", err)
	}
	c.RedisTTL = time.Duration(redisTTLSec) * time.Second

	// Kafka
	brokersRaw := getEnvOrDefault("KAFKA_BROKERS", "kafka-srv:9092")
	c.KafkaBrokers = strings.Split(brokersRaw, ",")
	c.KafkaGroupID = getEnvOrDefault("KAFKA_GROUP_ID", "media-service")
	c.KafkaTopicPrefix = getEnvOrDefault("KAFKA_TOPIC_PREFIX", "media")

	// ClamAV
	c.ClamAVHost = getEnvOrDefault("CLAMAV_HOST", "clamav-srv")
	c.ClamAVPort = getEnvOrDefault("CLAMAV_PORT", "3310")

	// FFmpeg
	c.FFmpegBinary = getEnvOrDefault("FFMPEG_BINARY", "/usr/bin/ffmpeg")

	// Upload limits
	c.MaxFileSizeBytes, err = getEnvInt64("MEDIA_MAX_FILE_SIZE_BYTES", 53687091200) // 50 GB
	if err != nil {
		return nil, fmt.Errorf("MEDIA_MAX_FILE_SIZE_BYTES: %w", err)
	}
	c.MaxConcurrentUploads, err = getEnvInt("MEDIA_MAX_CONCURRENT_UPLOADS", 5)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_MAX_CONCURRENT_UPLOADS: %w", err)
	}
	c.StorageQuotaBytes, err = getEnvInt64("MEDIA_STORAGE_QUOTA_BYTES", 53687091200)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_STORAGE_QUOTA_BYTES: %w", err)
	}

	allowedRaw := getEnvOrDefault("MEDIA_ALLOWED_TYPES",
		"image/jpeg,image/png,image/gif,image/webp,video/mp4,video/quicktime,video/x-msvideo,application/pdf,application/zip,text/plain")
	c.AllowedContentTypes = parseCSVSet(allowedRaw)

	sessionTTLSec, err := getEnvInt("MEDIA_UPLOAD_SESSION_TTL", 86400) // 24h
	if err != nil {
		return nil, fmt.Errorf("MEDIA_UPLOAD_SESSION_TTL: %w", err)
	}
	c.UploadSessionTTL = time.Duration(sessionTTLSec) * time.Second

	// Worker sizes
	c.ScanWorkers, err = getEnvInt("MEDIA_SCAN_WORKERS", 4)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_SCAN_WORKERS: %w", err)
	}
	c.ImageWorkers, err = getEnvInt("MEDIA_IMAGE_WORKERS", 4)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_IMAGE_WORKERS: %w", err)
	}
	c.VideoWorkers, err = getEnvInt("MEDIA_VIDEO_WORKERS", 2)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_VIDEO_WORKERS: %w", err)
	}
	c.DeleteWorkers, err = getEnvInt("MEDIA_DELETE_WORKERS", 4)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_DELETE_WORKERS: %w", err)
	}
	c.OutboxWorkers, err = getEnvInt("MEDIA_OUTBOX_WORKERS", 2)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_OUTBOX_WORKERS: %w", err)
	}
	c.CompressionWorkers, err = getEnvInt("MEDIA_COMPRESSION_WORKERS", 4)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_COMPRESSION_WORKERS: %w", err)
	}
	c.MetadataWorkers, err = getEnvInt("MEDIA_METADATA_WORKERS", 4)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_METADATA_WORKERS: %w", err)
	}

	// Reconciliation
	reconcileIntervalSec, err := getEnvInt("MEDIA_RECONCILE_INTERVAL", 300)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_RECONCILE_INTERVAL: %w", err)
	}
	c.ReconcileInterval = time.Duration(reconcileIntervalSec) * time.Second

	stuckUploadSec, err := getEnvInt("MEDIA_STUCK_UPLOAD_TIMEOUT", 3600)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_STUCK_UPLOAD_TIMEOUT: %w", err)
	}
	c.StuckUploadTimeout = time.Duration(stuckUploadSec) * time.Second

	stuckProcessingSec, err := getEnvInt("MEDIA_STUCK_PROCESSING_TIMEOUT", 7200)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_STUCK_PROCESSING_TIMEOUT: %w", err)
	}
	c.StuckProcessingTimeout = time.Duration(stuckProcessingSec) * time.Second

	// Rate limiting
	c.RateLimitUploadPerMinute, err = getEnvInt("MEDIA_RATE_LIMIT_UPLOAD_PER_MINUTE", 10)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_RATE_LIMIT_UPLOAD_PER_MINUTE: %w", err)
	}
	c.RateLimitDownloadPerMinute, err = getEnvInt("MEDIA_RATE_LIMIT_DOWNLOAD_PER_MINUTE", 60)
	if err != nil {
		return nil, fmt.Errorf("MEDIA_RATE_LIMIT_DOWNLOAD_PER_MINUTE: %w", err)
	}

	return c, nil
}

// IsDev returns true when the environment is not production.
func (c *Config) IsDev() bool {
	return c.Environment != "production"
}

// RedisAddr returns the Redis address in host:port format.
func (c *Config) RedisAddr() string {
	return c.RedisHost + ":" + c.RedisPort
}

// IsAllowedContentType checks if a MIME type is in the whitelist.
func (c *Config) IsAllowedContentType(ct string) bool {
	_, ok := c.AllowedContentTypes[ct]
	return ok
}

// helper functions

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value %q: %w", raw, err)
	}
	return v, nil
}

func getEnvInt64(key string, def int64) (int64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid int64 value %q: %w", raw, err)
	}
	return v, nil
}

func parseCSVSet(raw string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			m[part] = struct{}{}
		}
	}
	return m
}
