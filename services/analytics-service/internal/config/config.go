package config

import (
	"os"
	"strconv"
)

type Config struct {
	PortGraphQL string
	PortGRPC    string
	PortMetrics string
	NodeEnv     string

	ClickHouseHost     string
	ClickHousePort     string
	ClickHouseDB       string
	ClickHouseUser     string
	ClickHousePassword string

	RedisHost string
	RedisPort string

	KafkaBrokers  string
	KafkaGroupID  string
	KafkaClientID string

	NATSURL string

	SnowflakeWorkerID int64

	BatchSize        int
	BatchFlushMS     int
	MaxRetryAttempts int
	DLQTopic         string
	CacheTTLSeconds  int

	OTELEndpoint string
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return def
}

func Load() (*Config, error) {
	return &Config{
		PortGraphQL: getEnv("PORT_GRAPHQL", "4009"),
		PortGRPC:    getEnv("PORT_GRPC", "50057"),
		PortMetrics: getEnv("PORT_METRICS", "9107"),
		NodeEnv:     getEnv("NODE_ENV", "development"),

		ClickHouseHost:     getEnv("CLICKHOUSE_HOST", "localhost"),
		ClickHousePort:     getEnv("CLICKHOUSE_PORT", "9000"),
		ClickHouseDB:       getEnv("CLICKHOUSE_DB", "analytics"),
		ClickHouseUser:     getEnv("CLICKHOUSE_USER", "default"),
		ClickHousePassword: getEnv("CLICKHOUSE_PASSWORD", "clickhouse"),

		RedisHost: getEnv("REDIS_HOST", "localhost"),
		RedisPort: getEnv("REDIS_PORT", "6379"),

		KafkaBrokers:  getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaGroupID:  getEnv("KAFKA_GROUP_ID", "analytics-service"),
		KafkaClientID: getEnv("KAFKA_CLIENT_ID", "analytics-service"),

		NATSURL: getEnv("NATS_URL", "nats://localhost:4222"),

		SnowflakeWorkerID: getEnvInt64("SNOWFLAKE_WORKER_ID", 5),

		BatchSize:        getEnvInt("BATCH_SIZE", 500),
		BatchFlushMS:     getEnvInt("BATCH_FLUSH_MS", 500),
		MaxRetryAttempts: getEnvInt("MAX_RETRY_ATTEMPTS", 5),
		DLQTopic:         getEnv("DLQ_TOPIC", "analytics.dlq"),
		CacheTTLSeconds:  getEnvInt("CACHE_TTL_SECONDS", 300),

		OTELEndpoint: getEnv("OTEL_ENDPOINT", ""),
	}, nil
}
