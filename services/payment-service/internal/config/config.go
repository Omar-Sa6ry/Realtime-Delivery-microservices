package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	PortGraphQL       string
	PortGRPC          string
	PortMetrics       string
	PortWebhook       string
	NodeEnv           string

	// PostgreSQL
	PostgresDSN string

	// Redis
	RedisHost string
	RedisPort string

	// Kafka
	KafkaBrokers  string
	KafkaGroupID  string
	KafkaClientID string

	// NATS
	NATSUrl string

	// Snowflake
	SnowflakeWorkerID int64

	// Workers
	ReconcileIntervalSec time.Duration
	StuckThresholdSec    time.Duration
	OutboxBatchSize      int

	// Provider
	ProviderMockMode string

	// Webhook
	WebhookSecret string

	// Observability
	OTELEndpoint string

	// Stripe
	StripeSecretKey       string
	StripeWebhookSecret   string
	StripeSuccessURL      string
	StripeFailURL         string

	// Delivery Service
	DeliveryServiceURL string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	getEnv := func(key, defaultValue string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return defaultValue
	}

	getEnvInt := func(key string, defaultValue int) int {
		if v := os.Getenv(key); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
		return defaultValue
	}

	getEnvInt64 := func(key string, defaultValue int64) int64 {
		if v := os.Getenv(key); v != "" {
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return i
			}
		}
		return defaultValue
	}

	getEnvDuration := func(key string, defaultValue time.Duration) time.Duration {
		if v := os.Getenv(key); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				return time.Duration(i) * time.Second
			}
		}
		return defaultValue
	}

	return &Config{
		PortGraphQL:       getEnv("PORT_GRAPHQL", "4002"),
		PortGRPC:          getEnv("PORT_GRPC", "50056"),
		PortMetrics:       getEnv("PORT_METRICS", "9106"),
		PortWebhook:       getEnv("PORT_WEBHOOK", "4012"),
		NodeEnv:           getEnv("NODE_ENV", "development"),
		PostgresDSN:       getEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/payment_db?sslmode=disable"),
		RedisHost:         getEnv("REDIS_HOST", "localhost"),
		RedisPort:         getEnv("REDIS_PORT", "6379"),
		KafkaBrokers:      getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaGroupID:      getEnv("KAFKA_GROUP_ID", "payment-service"),
		KafkaClientID:     getEnv("KAFKA_CLIENT_ID", "payment-service"),
		NATSUrl:           getEnv("NATS_URL", "nats://localhost:4222"),
		SnowflakeWorkerID: getEnvInt64("SNOWFLAKE_WORKER_ID", 2),
		ReconcileIntervalSec: getEnvDuration("RECONCILE_INTERVAL_SEC", 60*time.Second),
		StuckThresholdSec:    getEnvDuration("STUCK_THRESHOLD_SEC", 30*time.Second),
		OutboxBatchSize:      getEnvInt("OUTBOX_BATCH_SIZE", 100),
		ProviderMockMode:     getEnv("PROVIDER_MOCK_MODE", "success"),
		WebhookSecret:        getEnv("WEBHOOK_SECRET", "dev-webhook-secret"),
		OTELEndpoint:         getEnv("OTEL_ENDPOINT", ""),
		StripeSecretKey:      getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret:  getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripeSuccessURL:     getEnv("STRIPE_WEBHOOK_SUCCESSURL", ""),
		StripeFailURL:        getEnv("STRIPE_WEBHOOK_FAILURL", ""),
		DeliveryServiceURL:   getEnv("DELIVERY_SERVICE_URL", "http://localhost:4003"),
	}, nil
}