package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear environment variables to test defaults
	envVars := []string{
		"PORT_GRAPHQL", "PORT_GRPC", "PORT_METRICS", "NODE_ENV",
		"CLICKHOUSE_HOST", "CLICKHOUSE_PORT", "CLICKHOUSE_DB", "CLICKHOUSE_USER", "CLICKHOUSE_PASSWORD",
		"REDIS_HOST", "REDIS_PORT",
		"KAFKA_BROKERS", "KAFKA_GROUP_ID", "KAFKA_CLIENT_ID",
		"NATS_URL",
		"SNOWFLAKE_WORKER_ID",
		"BATCH_SIZE", "BATCH_FLUSH_MS", "MAX_RETRY_ATTEMPTS", "DLQ_TOPIC", "CACHE_TTL_SECONDS",
		"RECONCILE_INTERVAL_SEC",
		"OTEL_ENDPOINT",
	}

	for _, v := range envVars {
		os.Unsetenv(v)
	}
	defer func() {
		for _, v := range envVars {
			os.Unsetenv(v)
		}
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "4009", cfg.PortGraphQL)
	assert.Equal(t, "50057", cfg.PortGRPC)
	assert.Equal(t, "9107", cfg.PortMetrics)
	assert.Equal(t, "development", cfg.NodeEnv)

	assert.Equal(t, "localhost", cfg.ClickHouseHost)
	assert.Equal(t, "9000", cfg.ClickHousePort)
	assert.Equal(t, "analytics", cfg.ClickHouseDB)
	assert.Equal(t, "default", cfg.ClickHouseUser)
	assert.Equal(t, "clickhouse", cfg.ClickHousePassword)

	assert.Equal(t, "localhost", cfg.RedisHost)
	assert.Equal(t, "6379", cfg.RedisPort)

	assert.Equal(t, "localhost:9092", cfg.KafkaBrokers)
	assert.Equal(t, "analytics-service", cfg.KafkaGroupID)
	assert.Equal(t, "analytics-service", cfg.KafkaClientID)

	assert.Equal(t, "nats://localhost:4222", cfg.NATSURL)

	assert.Equal(t, int64(5), cfg.SnowflakeWorkerID)

	assert.Equal(t, 500, cfg.BatchSize)
	assert.Equal(t, 500, cfg.BatchFlushMS)
	assert.Equal(t, 5, cfg.MaxRetryAttempts)
	assert.Equal(t, "analytics.dlq", cfg.DLQTopic)
	assert.Equal(t, 300, cfg.CacheTTLSeconds)

	assert.Equal(t, 300, cfg.ReconcileIntervalSec)

	assert.Equal(t, "", cfg.OTELEndpoint)
}

func TestLoad_WithEnvVars(t *testing.T) {
	envVars := map[string]string{
		"PORT_GRAPHQL":          "4010",
		"PORT_GRPC":             "50058",
		"PORT_METRICS":          "9108",
		"NODE_ENV":              "production",
		"CLICKHOUSE_HOST":       "clickhouse-host",
		"CLICKHOUSE_PORT":       "9001",
		"CLICKHOUSE_DB":         "analytics_prod",
		"CLICKHOUSE_USER":       "admin",
		"CLICKHOUSE_PASSWORD":   "secret",
		"REDIS_HOST":            "redis-host",
		"REDIS_PORT":            "6380",
		"KAFKA_BROKERS":         "kafka1:9092,kafka2:9092",
		"KAFKA_GROUP_ID":        "analytics-prod",
		"KAFKA_CLIENT_ID":       "analytics-prod-1",
		"NATS_URL":              "nats://nats-host:4222",
		"SNOWFLAKE_WORKER_ID":   "10",
		"BATCH_SIZE":            "1000",
		"BATCH_FLUSH_MS":        "1000",
		"MAX_RETRY_ATTEMPTS":    "10",
		"DLQ_TOPIC":             "analytics.dlq.prod",
		"CACHE_TTL_SECONDS":     "600",
		"RECONCILE_INTERVAL_SEC": "600",
		"OTEL_ENDPOINT":         "otel-collector:4317",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "4010", cfg.PortGraphQL)
	assert.Equal(t, "50058", cfg.PortGRPC)
	assert.Equal(t, "9108", cfg.PortMetrics)
	assert.Equal(t, "production", cfg.NodeEnv)

	assert.Equal(t, "clickhouse-host", cfg.ClickHouseHost)
	assert.Equal(t, "9001", cfg.ClickHousePort)
	assert.Equal(t, "analytics_prod", cfg.ClickHouseDB)
	assert.Equal(t, "admin", cfg.ClickHouseUser)
	assert.Equal(t, "secret", cfg.ClickHousePassword)

	assert.Equal(t, "redis-host", cfg.RedisHost)
	assert.Equal(t, "6380", cfg.RedisPort)

	assert.Equal(t, "kafka1:9092,kafka2:9092", cfg.KafkaBrokers)
	assert.Equal(t, "analytics-prod", cfg.KafkaGroupID)
	assert.Equal(t, "analytics-prod-1", cfg.KafkaClientID)

	assert.Equal(t, "nats://nats-host:4222", cfg.NATSURL)

	assert.Equal(t, int64(10), cfg.SnowflakeWorkerID)

	assert.Equal(t, 1000, cfg.BatchSize)
	assert.Equal(t, 1000, cfg.BatchFlushMS)
	assert.Equal(t, 10, cfg.MaxRetryAttempts)
	assert.Equal(t, "analytics.dlq.prod", cfg.DLQTopic)
	assert.Equal(t, 600, cfg.CacheTTLSeconds)

	assert.Equal(t, 600, cfg.ReconcileIntervalSec)

	assert.Equal(t, "otel-collector:4317", cfg.OTELEndpoint)
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_KEY", "test-value")
	defer os.Unsetenv("TEST_KEY")

	assert.Equal(t, "test-value", getEnv("TEST_KEY", "default"))
	assert.Equal(t, "default", getEnv("NONEXISTENT_KEY", "default"))
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	assert.Equal(t, 42, getEnvInt("TEST_INT", 0))
	assert.Equal(t, 0, getEnvInt("NONEXISTENT_INT", 0))

	os.Setenv("TEST_INT_INVALID", "not-a-number")
	defer os.Unsetenv("TEST_INT_INVALID")
	assert.Equal(t, 0, getEnvInt("TEST_INT_INVALID", 0))
}

func TestGetEnvInt64(t *testing.T) {
	os.Setenv("TEST_INT64", "1234567890")
	defer os.Unsetenv("TEST_INT64")

	assert.Equal(t, int64(1234567890), getEnvInt64("TEST_INT64", 0))
	assert.Equal(t, int64(0), getEnvInt64("NONEXISTENT_INT64", 0))

	os.Setenv("TEST_INT64_INVALID", "not-a-number")
	defer os.Unsetenv("TEST_INT64_INVALID")
	assert.Equal(t, int64(0), getEnvInt64("TEST_INT64_INVALID", 0))
}