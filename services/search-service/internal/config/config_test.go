package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("NODE_ENV", "")
	t.Setenv("PORT_SEARCH_GRAPHQL", "")
	t.Setenv("PORT_SEARCH_METRICS", "")
	t.Setenv("OPENSEARCH_URL", "")
	t.Setenv("OPENSEARCH_USERNAME", "")
	t.Setenv("OPENSEARCH_PASSWORD", "")
	t.Setenv("OPENSEARCH_INSECURE", "")
	t.Setenv("KAFKA_BROKERS", "")
	t.Setenv("KAFKA_GROUP_ID_SEARCH", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("SEARCH_CACHE_TTL", "")
	t.Setenv("NATS_URL", "")
	t.Setenv("RABBITMQ_URL", "")
	t.Setenv("SEARCH_MAX_PAGE_SIZE", "")
	t.Setenv("SEARCH_MAX_QUERY_LENGTH", "")
	t.Setenv("OTEL_ENDPOINT", "")
	t.Setenv("OTEL_SAMPLING_RATE", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("SEARCH_INDEX_INIT_MAX_RETRIES", "")
	t.Setenv("SEARCH_INDEX_INIT_RETRY_DELAY_MS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.IndexInitMaxRetries != 30 || cfg.IndexInitRetryDelay != 3*time.Second {
		t.Errorf("IndexInit defaults = %d / %v", cfg.IndexInitMaxRetries, cfg.IndexInitRetryDelay)
	}
	if cfg.AppEnv != "development" || cfg.NodeEnv != "development" {
		t.Errorf("AppEnv/NodeEnv = %q/%q", cfg.AppEnv, cfg.NodeEnv)
	}
	if cfg.PortGraphQL != "4007" || cfg.PortMetrics != "9103" {
		t.Errorf("ports = %q/%q", cfg.PortGraphQL, cfg.PortMetrics)
	}
	if len(cfg.OpenSearchAddresses) != 1 || cfg.OpenSearchAddresses[0] != "http://opensearch-srv:9200" {
		t.Errorf("OpenSearchAddresses = %v", cfg.OpenSearchAddresses)
	}
	if cfg.OpenSearchUsername != "admin" || cfg.OpenSearchPassword != "admin" {
		t.Errorf("OpenSearch credentials = %q/%q", cfg.OpenSearchUsername, cfg.OpenSearchPassword)
	}
	if !cfg.OpenSearchInsecure {
		t.Error("OpenSearchInsecure should default true")
	}
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "kafka-srv:9092" {
		t.Errorf("KafkaBrokers = %v", cfg.KafkaBrokers)
	}
	if cfg.KafkaGroupID != "search-service" {
		t.Errorf("KafkaGroupID = %q", cfg.KafkaGroupID)
	}
	if cfg.RedisHost != "redis-srv" || cfg.RedisPort != "6379" || cfg.RedisDB != 0 {
		t.Errorf("Redis = %q:%q db=%d", cfg.RedisHost, cfg.RedisPort, cfg.RedisDB)
	}
	if cfg.RedisTTL != 120*time.Second {
		t.Errorf("RedisTTL = %v", cfg.RedisTTL)
	}
	if cfg.NATSUrl != "nats://nats-srv:4222" {
		t.Errorf("NATSUrl = %q", cfg.NATSUrl)
	}
	if cfg.RabbitMQURL != "" {
		t.Errorf("RabbitMQURL = %q", cfg.RabbitMQURL)
	}
	if cfg.MaxPageSize != 100 || cfg.MaxQueryLength != 500 {
		t.Errorf("limits = %d/%d", cfg.MaxPageSize, cfg.MaxQueryLength)
	}
	if cfg.OTELEndpoint != "" || cfg.OTELSamplingRate != 1.0 {
		t.Errorf("OTEL = %q/%v", cfg.OTELEndpoint, cfg.OTELSamplingRate)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q", cfg.LogLevel)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("PORT_SEARCH_GRAPHQL", "5001")
	t.Setenv("PORT_SEARCH_METRICS", "9201")
	t.Setenv("OPENSEARCH_URL", "http://os1:9200,http://os2:9200")
	t.Setenv("OPENSEARCH_USERNAME", "u")
	t.Setenv("OPENSEARCH_PASSWORD", "p")
	t.Setenv("OPENSEARCH_INSECURE", "false")
	t.Setenv("KAFKA_BROKERS", "k1:9092,k2:9092")
	t.Setenv("KAFKA_GROUP_ID_SEARCH", "grp")
	t.Setenv("REDIS_HOST", "rh")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "rp")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("SEARCH_CACHE_TTL", "45")
	t.Setenv("NATS_URL", "nats://x")
	t.Setenv("RABBITMQ_URL", "amqp://x")
	t.Setenv("SEARCH_MAX_PAGE_SIZE", "25")
	t.Setenv("SEARCH_MAX_QUERY_LENGTH", "100")
	t.Setenv("OTEL_ENDPOINT", "otel:4317")
	t.Setenv("OTEL_SAMPLING_RATE", "0.5")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("SEARCH_INDEX_INIT_MAX_RETRIES", "5")
	t.Setenv("SEARCH_INDEX_INIT_RETRY_DELAY_MS", "100")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.IndexInitMaxRetries != 5 || cfg.IndexInitRetryDelay != 100*time.Millisecond {
		t.Errorf("IndexInit from env = %d / %v", cfg.IndexInitMaxRetries, cfg.IndexInitRetryDelay)
	}
	if cfg.AppEnv != "test" || cfg.NodeEnv != "test" {
		t.Errorf("AppEnv = %q/%q", cfg.AppEnv, cfg.NodeEnv)
	}
	if cfg.PortGraphQL != "5001" || cfg.PortMetrics != "9201" {
		t.Errorf("ports = %q/%q", cfg.PortGraphQL, cfg.PortMetrics)
	}
	if len(cfg.OpenSearchAddresses) != 2 {
		t.Errorf("OpenSearchAddresses = %v", cfg.OpenSearchAddresses)
	}
	if cfg.OpenSearchInsecure {
		t.Error("OpenSearchInsecure should be false")
	}
	if len(cfg.KafkaBrokers) != 2 || cfg.KafkaGroupID != "grp" {
		t.Errorf("kafka = %v / %q", cfg.KafkaBrokers, cfg.KafkaGroupID)
	}
	if cfg.RedisHost != "rh" || cfg.RedisPort != "6380" || cfg.RedisPassword != "rp" || cfg.RedisDB != 3 {
		t.Errorf("redis = %v", cfg)
	}
	if cfg.RedisTTL != 45*time.Second {
		t.Errorf("RedisTTL = %v", cfg.RedisTTL)
	}
	if cfg.RabbitMQURL != "amqp://x" || cfg.NATSUrl != "nats://x" {
		t.Errorf("messaging = %q / %q", cfg.RabbitMQURL, cfg.NATSUrl)
	}
	if cfg.MaxPageSize != 25 || cfg.MaxQueryLength != 100 {
		t.Errorf("limits = %d/%d", cfg.MaxPageSize, cfg.MaxQueryLength)
	}
	if cfg.OTELEndpoint != "otel:4317" || cfg.OTELSamplingRate != 0.5 {
		t.Errorf("OTEL = %q/%v", cfg.OTELEndpoint, cfg.OTELSamplingRate)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q", cfg.LogLevel)
	}
}

func TestLoad_ClampsInvalidValues(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("NODE_ENV", "production") // APP_ENV wins
	t.Setenv("SEARCH_MAX_PAGE_SIZE", "0")
	t.Setenv("SEARCH_MAX_QUERY_LENGTH", "-5")
	t.Setenv("OTEL_SAMPLING_RATE", "not-a-float")
	t.Setenv("REDIS_DB", "not-an-int")
	t.Setenv("SEARCH_CACHE_TTL", "not-an-int")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AppEnv != "test" {
		t.Errorf("APP_ENV should win, got %q", cfg.AppEnv)
	}
	if cfg.MaxPageSize != 100 {
		t.Errorf("MaxPageSize clamp = %d", cfg.MaxPageSize)
	}
	if cfg.MaxQueryLength != 500 {
		t.Errorf("MaxQueryLength clamp = %d", cfg.MaxQueryLength)
	}
	if cfg.OTELSamplingRate != 1.0 {
		t.Errorf("OTELSamplingRate clamp = %v", cfg.OTELSamplingRate)
	}
	if cfg.RedisDB != 0 || cfg.RedisTTL != 0 {
		t.Errorf("invalid ints should be zero, got db=%d ttl=%v", cfg.RedisDB, cfg.RedisTTL)
	}
}

func TestLoad_SamplingRateClamps(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OTEL_SAMPLING_RATE", "-1")
	cfg, _ := Load()
	if cfg.OTELSamplingRate != 1.0 {
		t.Errorf("negative sampling rate = %v", cfg.OTELSamplingRate)
	}

	t.Setenv("OTEL_SAMPLING_RATE", "5")
	cfg, _ = Load()
	if cfg.OTELSamplingRate != 1 {
		t.Errorf("sampling rate > 1 = %v", cfg.OTELSamplingRate)
	}

	t.Setenv("OTEL_SAMPLING_RATE", "0")
	cfg, _ = Load()
	if cfg.OTELSamplingRate != 0 {
		t.Errorf("zero sampling rate = %v", cfg.OTELSamplingRate)
	}
}

func TestLoad_InvalidOpenSearchURL(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OPENSEARCH_URL", "http://[::1")
	if _, err := Load(); err == nil {
		t.Fatal("invalid OPENSEARCH_URL should error")
	}
}

func TestLoad_IndexInitClamps(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("SEARCH_INDEX_INIT_MAX_RETRIES", "0")
	t.Setenv("SEARCH_INDEX_INIT_RETRY_DELAY_MS", "-10")
	t.Setenv("RABBITMQ_MAX_ATTEMPTS", "0")
	t.Setenv("RABBITMQ_BASE_DELAY_MS", "-5")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.IndexInitMaxRetries != 1 {
		t.Errorf("retries clamp = %d", cfg.IndexInitMaxRetries)
	}
	if cfg.IndexInitRetryDelay != 0 {
		t.Errorf("delay clamp = %v", cfg.IndexInitRetryDelay)
	}
	if cfg.RabbitMQMaxAttempts != 1 {
		t.Errorf("rmq attempts clamp = %d", cfg.RabbitMQMaxAttempts)
	}
	if cfg.RabbitMQBaseDelay != 0 {
		t.Errorf("rmq delay clamp = %v", cfg.RabbitMQBaseDelay)
	}
}

func TestLoad_RabbitMQDefaultsAndFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("RABBITMQ_MAX_ATTEMPTS", "")
	t.Setenv("RABBITMQ_BASE_DELAY_MS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RabbitMQMaxAttempts != 15 || cfg.RabbitMQBaseDelay != time.Second {
		t.Errorf("rmq defaults = %d / %v", cfg.RabbitMQMaxAttempts, cfg.RabbitMQBaseDelay)
	}

	t.Setenv("RABBITMQ_MAX_ATTEMPTS", "3")
	t.Setenv("RABBITMQ_BASE_DELAY_MS", "250")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RabbitMQMaxAttempts != 3 || cfg.RabbitMQBaseDelay != 250*time.Millisecond {
		t.Errorf("rmq from env = %d / %v", cfg.RabbitMQMaxAttempts, cfg.RabbitMQBaseDelay)
	}
}

func TestLoad_NodeEnvFallback(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("NODE_ENV", "production")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AppEnv != "production" {
		t.Errorf("AppEnv from NODE_ENV = %q", cfg.AppEnv)
	}
}

func TestRedisAddr(t *testing.T) {
	c := &Config{RedisHost: "h", RedisPort: "1"}
	if got := c.RedisAddr(); got != "h:1" {
		t.Errorf("RedisAddr = %q", got)
	}
}

func TestGetEnvAsBool_InvalidFallsBack(t *testing.T) {
	t.Setenv("BOOL_KEY", "notabool")
	if got := getEnvAsBool("BOOL_KEY", true); !got {
		t.Error("invalid bool should fall back to default true")
	}
	if got := getEnvAsBool("BOOL_KEY", false); got {
		t.Error("invalid bool should fall back to default false")
	}
	t.Setenv("BOOL_KEY", "1")
	if got := getEnvAsBool("BOOL_KEY", false); !got {
		t.Error("1 should parse as true")
	}
}

func TestGetEnv_EmptyFallsBack(t *testing.T) {
	t.Setenv("STR_KEY", "")
	if got := getEnv("STR_KEY", "d"); got != "d" {
		t.Errorf("getEnv empty = %q", got)
	}
	t.Setenv("STR_KEY", "v")
	if got := getEnv("STR_KEY", "d"); got != "v" {
		t.Errorf("getEnv = %q", got)
	}
}
