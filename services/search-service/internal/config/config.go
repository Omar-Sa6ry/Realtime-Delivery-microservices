package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv      string
	NodeEnv     string
	PortGraphQL string
	PortMetrics string
	LogLevel    string

	// Tracing (OTLP -> otel-collector -> Tempo). Empty endpoint disables tracing.
	OTELEndpoint     string
	OTELSamplingRate float64

	// OpenSearch
	OpenSearchAddresses []string
	OpenSearchUsername  string
	OpenSearchPassword  string
	OpenSearchInsecure  bool

	// Kafka
	KafkaBrokers []string
	KafkaGroupID string

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
	RedisTTL      time.Duration

	// NATS
	NATSUrl string

	// RabbitMQ
	RabbitMQURL         string
	RabbitMQMaxAttempts int
	RabbitMQBaseDelay   time.Duration

	// Search limits
	MaxPageSize    int
	MaxQueryLength int

	// Index readiness (run() waits for OpenSearch indices before consumers).
	IndexInitMaxRetries int
	IndexInitRetryDelay time.Duration
}

func Load() (*Config, error) {
	appEnv := getEnv("APP_ENV", getEnv("NODE_ENV", "development"))

	portGraphQL := getEnv("PORT_SEARCH_GRAPHQL", "4007")
	portMetrics := getEnv("PORT_SEARCH_METRICS", "9103")

	osURL := getEnv("OPENSEARCH_URL", "http://opensearch-srv:9200")
	addresses := strings.Split(osURL, ",")
	for _, addr := range addresses {
		if _, err := url.ParseRequestURI(strings.TrimSpace(addr)); err != nil {
			return nil, fmt.Errorf("invalid OPENSEARCH_URL %q: %w", addr, err)
		}
	}

	retries, _ := strconv.Atoi(getEnv("SEARCH_INDEX_INIT_MAX_RETRIES", "30"))
	if retries < 1 {
		retries = 1
	}
	retryDelayMs, _ := strconv.Atoi(getEnv("SEARCH_INDEX_INIT_RETRY_DELAY_MS", "3000"))
	if retryDelayMs < 0 {
		retryDelayMs = 0
	}

	rmqAttempts, _ := strconv.Atoi(getEnv("RABBITMQ_MAX_ATTEMPTS", "15"))
	if rmqAttempts < 1 {
		rmqAttempts = 1
	}
	rmqDelayMs, _ := strconv.Atoi(getEnv("RABBITMQ_BASE_DELAY_MS", "1000"))
	if rmqDelayMs < 0 {
		rmqDelayMs = 0
	}

	kafkaBrokersStr := getEnv("KAFKA_BROKERS", "kafka-srv:9092")
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	redisTTLSec, _ := strconv.Atoi(getEnv("SEARCH_CACHE_TTL", "120"))

	maxPageSize, _ := strconv.Atoi(getEnv("SEARCH_MAX_PAGE_SIZE", "100"))
	if maxPageSize <= 0 {
		maxPageSize = 100
	}

	maxQueryLength, _ := strconv.Atoi(getEnv("SEARCH_MAX_QUERY_LENGTH", "500"))
	if maxQueryLength <= 0 {
		maxQueryLength = 500
	}

	samplingRate, err := strconv.ParseFloat(getEnv("OTEL_SAMPLING_RATE", "1.0"), 64)
	if err != nil || samplingRate < 0 {
		samplingRate = 1.0
	}
	if samplingRate > 1 {
		samplingRate = 1
	}

	return &Config{
		AppEnv:              appEnv,
		NodeEnv:             appEnv,
		PortGraphQL:         portGraphQL,
		PortMetrics:         portMetrics,
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		OTELEndpoint:        getEnv("OTEL_ENDPOINT", ""),
		OTELSamplingRate:    samplingRate,
		OpenSearchAddresses: addresses,
		OpenSearchUsername:  getEnv("OPENSEARCH_USERNAME", "admin"),
		OpenSearchPassword:  getEnv("OPENSEARCH_PASSWORD", "admin"),
		OpenSearchInsecure:  getEnvAsBool("OPENSEARCH_INSECURE", true),
		KafkaBrokers:        kafkaBrokers,
		KafkaGroupID:        getEnv("KAFKA_GROUP_ID_SEARCH", "search-service"),
		RedisHost:           getEnv("REDIS_HOST", "redis-srv"),
		RedisPort:           getEnv("REDIS_PORT", "6379"),
		RedisPassword:       getEnv("REDIS_PASSWORD", ""),
		RedisDB:             redisDB,
		RedisTTL:            time.Duration(redisTTLSec) * time.Second,
		NATSUrl:             getEnv("NATS_URL", "nats://nats-srv:4222"),
		RabbitMQURL:         getEnv("RABBITMQ_URL", ""),
		RabbitMQMaxAttempts: rmqAttempts,
		RabbitMQBaseDelay:   time.Duration(rmqDelayMs) * time.Millisecond,
		MaxPageSize:         maxPageSize,
		MaxQueryLength:      maxQueryLength,
		IndexInitMaxRetries: retries,
		IndexInitRetryDelay: time.Duration(retryDelayMs) * time.Millisecond,
	}, nil
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}
