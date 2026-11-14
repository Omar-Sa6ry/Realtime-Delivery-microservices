package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/config"
)

var allKeys = []string{
	"PORT_GRAPHQL", "PORT_GRPC", "PORT_METRICS", "PORT_WEBHOOK", "NODE_ENV",
	"POSTGRES_USER", "DB_USER", "POSTGRES_PASSWORD", "DB_PASSWORD",
	"POSTGRES_HOST", "DB_HOST", "POSTGRES_PORT", "DB_PORT",
	"POSTGRES_DB", "DB_NAME", "POSTGRES_SSLMODE", "DB_SSLMODE", "POSTGRES_DSN",
	"REDIS_HOST", "REDIS_PORT",
	"KAFKA_BROKERS", "KAFKA_GROUP_ID", "KAFKA_CLIENT_ID",
	"NATS_URL", "SNOWFLAKE_WORKER_ID",
	"RECONCILE_INTERVAL_SEC", "STUCK_THRESHOLD_SEC", "OUTBOX_BATCH_SIZE",
	"WEBHOOK_SECRET", "OTEL_ENDPOINT",
	"STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET",
	"STRIPE_WEBHOOK_SUCCESSURL", "STRIPE_WEBHOOK_FAILURL",
	"DELIVERY_SERVICE_URL",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range allKeys {
		prev, had := os.LookupEnv(k)
		assert.NoError(t, os.Unsetenv(k))
		k, prev, had := k, prev, had
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(k, prev)
			} else {
				_ = os.Unsetenv(k)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	cfg := config.Load()

	assert.Equal(t, "4002", cfg.PortGraphQL)
	assert.Equal(t, "50056", cfg.PortGRPC)
	assert.Equal(t, "9106", cfg.PortMetrics)
	assert.Equal(t, "4012", cfg.PortWebhook)
	assert.Equal(t, "development", cfg.NodeEnv)
	assert.Equal(t, "postgres", cfg.DBUser)
	assert.Equal(t, "postgres", cfg.DBPassword)
	assert.Equal(t, "payment-db-srv", cfg.DBHost)
	assert.Equal(t, "5432", cfg.DBPort)
	assert.Equal(t, "delivery_payment_db", cfg.DBName)
	assert.Equal(t, "disable", cfg.DBSSLMode)
	assert.Equal(t, "localhost", cfg.RedisHost)
	assert.Equal(t, "6379", cfg.RedisPort)
	assert.Equal(t, "localhost:9092", cfg.KafkaBrokers)
	assert.Equal(t, "payment-service", cfg.KafkaGroupID)
	assert.Equal(t, "payment-service", cfg.KafkaClientID)
	assert.Equal(t, "nats://localhost:4222", cfg.NATSUrl)
	assert.Equal(t, int64(2), cfg.SnowflakeWorkerID)
	assert.Equal(t, 60*time.Second, cfg.ReconcileIntervalSec)
	assert.Equal(t, 30*time.Second, cfg.StuckThresholdSec)
	assert.Equal(t, 100, cfg.OutboxBatchSize)
	assert.Equal(t, "dev-webhook-secret", cfg.WebhookSecret)
	assert.Equal(t, "", cfg.OTELEndpoint)
	assert.Equal(t, "", cfg.StripeSecretKey)
	assert.Equal(t, "", cfg.StripeWebhookSecret)
	assert.Equal(t, "", cfg.StripeSuccessURL)
	assert.Equal(t, "", cfg.StripeFailURL)
	assert.Equal(t, "http://localhost:4003", cfg.DeliveryServiceURL)
	assert.Equal(t,
		"postgres://postgres:postgres@payment-db-srv:5432/delivery_payment_db?sslmode=disable",
		cfg.PostgresDSN)
}

func TestLoadPrimaryEnvWinsOverPostgresAlias(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_USER", "primaryuser")
	t.Setenv("DB_PASSWORD", "primarypass")
	t.Setenv("DB_HOST", "primaryhost")
	t.Setenv("DB_PORT", "6543")
	t.Setenv("DB_NAME", "primarydb")
	t.Setenv("DB_SSLMODE", "require")

	cfg := config.Load()
	assert.Equal(t, "primaryuser", cfg.DBUser)
	assert.Equal(t, "primarypass", cfg.DBPassword)
	assert.Equal(t, "primaryhost", cfg.DBHost)
	assert.Equal(t, "6543", cfg.DBPort)
	assert.Equal(t, "primarydb", cfg.DBName)
	assert.Equal(t, "require", cfg.DBSSLMode)
	assert.Equal(t,
		"postgres://primaryuser:primarypass@primaryhost:6543/primarydb?sslmode=require",
		cfg.PostgresDSN)
}

func TestLoadPostgresAliasEnvFallback(t *testing.T) {
	clearEnv(t)
	t.Setenv("POSTGRES_USER", "aliasuser")
	t.Setenv("POSTGRES_PASSWORD", "aliaspass")
	t.Setenv("POSTGRES_HOST", "aliashost")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_DB", "aliasdb")
	t.Setenv("POSTGRES_SSLMODE", "verify-full")

	cfg := config.Load()
	assert.Equal(t, "aliasuser", cfg.DBUser)
	assert.Equal(t, "aliaspass", cfg.DBPassword)
	assert.Equal(t, "aliashost", cfg.DBHost)
	assert.Equal(t, "5433", cfg.DBPort)
	assert.Equal(t, "aliasdb", cfg.DBName)
	assert.Equal(t, "verify-full", cfg.DBSSLMode)
}

func TestLoadExplicitDSN(t *testing.T) {
	clearEnv(t)
	t.Setenv("POSTGRES_DSN", "postgres://custom:pw@db:5432/other?sslmode=disable")
	cfg := config.Load()
	assert.Equal(t, "postgres://custom:pw@db:5432/other?sslmode=disable", cfg.PostgresDSN)
}

func TestLoadDSNWithPlaceholderFallsBackToConstructedDSN(t *testing.T) {
	clearEnv(t)
	t.Setenv("POSTGRES_DSN", "postgres://$(POSTGRES_USER)@host:5432/db")
	cfg := config.Load()
	assert.NotContains(t, cfg.PostgresDSN, "$(")
	assert.Contains(t, cfg.PostgresDSN, "postgres://postgres:postgres@payment-db-srv:5432/")
}

func TestLoadInvalidNumbersFallBackToDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("SNOWFLAKE_WORKER_ID", "not-a-number")
	t.Setenv("OUTBOX_BATCH_SIZE", "abc")
	t.Setenv("RECONCILE_INTERVAL_SEC", "xyz")
	t.Setenv("STUCK_THRESHOLD_SEC", "12.5")

	cfg := config.Load()
	assert.Equal(t, int64(2), cfg.SnowflakeWorkerID)
	assert.Equal(t, 100, cfg.OutboxBatchSize)
	assert.Equal(t, 60*time.Second, cfg.ReconcileIntervalSec)
	assert.Equal(t, 30*time.Second, cfg.StuckThresholdSec)
}

func TestLoadTypedValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("SNOWFLAKE_WORKER_ID", "9")
	t.Setenv("OUTBOX_BATCH_SIZE", "7")
	t.Setenv("RECONCILE_INTERVAL_SEC", "15")
	t.Setenv("STUCK_THRESHOLD_SEC", "45")
	t.Setenv("NODE_ENV", "production")
	t.Setenv("PORT_GRAPHQL", "8000")
	t.Setenv("KAFKA_BROKERS", "k1:9092,k2:9092")
	t.Setenv("NATS_URL", "nats://nats:4222")
	t.Setenv("OTEL_ENDPOINT", "otel:4317")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	t.Setenv("STRIPE_WEBHOOK_SUCCESSURL", "https://ok")
	t.Setenv("STRIPE_WEBHOOK_FAILURL", "https://fail")
	t.Setenv("DELIVERY_SERVICE_URL", "http://delivery:4003")
	t.Setenv("WEBHOOK_SECRET", "shh")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("KAFKA_GROUP_ID", "grp")
	t.Setenv("KAFKA_CLIENT_ID", "cli")

	cfg := config.Load()
	assert.Equal(t, int64(9), cfg.SnowflakeWorkerID)
	assert.Equal(t, 7, cfg.OutboxBatchSize)
	assert.Equal(t, 15*time.Second, cfg.ReconcileIntervalSec)
	assert.Equal(t, 45*time.Second, cfg.StuckThresholdSec)
	assert.Equal(t, "production", cfg.NodeEnv)
	assert.Equal(t, "8000", cfg.PortGraphQL)
	assert.Equal(t, "k1:9092,k2:9092", cfg.KafkaBrokers)
	assert.Equal(t, "nats://nats:4222", cfg.NATSUrl)
	assert.Equal(t, "otel:4317", cfg.OTELEndpoint)
	assert.Equal(t, "sk_test_123", cfg.StripeSecretKey)
	assert.Equal(t, "whsec_123", cfg.StripeWebhookSecret)
	assert.Equal(t, "https://ok", cfg.StripeSuccessURL)
	assert.Equal(t, "https://fail", cfg.StripeFailURL)
	assert.Equal(t, "http://delivery:4003", cfg.DeliveryServiceURL)
	assert.Equal(t, "shh", cfg.WebhookSecret)
	assert.Equal(t, "redis", cfg.RedisHost)
	assert.Equal(t, "6380", cfg.RedisPort)
	assert.Equal(t, "grp", cfg.KafkaGroupID)
	assert.Equal(t, "cli", cfg.KafkaClientID)
}
