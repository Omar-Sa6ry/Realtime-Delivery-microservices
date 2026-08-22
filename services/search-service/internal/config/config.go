package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv              string
	NodeEnv             string
	PortGraphQL         string
	PortMetrics         string
	LogLevel            string
	
	// OpenSearch
	OpenSearchAddresses []string
	OpenSearchUsername  string
	OpenSearchPassword  string
	OpenSearchInsecure  bool

	// Kafka
	KafkaBrokers        []string
	KafkaGroupID        string

	// Redis
	RedisHost           string
	RedisPort           string
	RedisPassword       string
	RedisDB             int
	RedisTTL            time.Duration

	// NATS
	NATSUrl             string

	// Search limits
	MaxPageSize         int
	MaxQueryLength      int
}

func Load() (*Config, error) {
	appEnv := getEnv("APP_ENV", getEnv("NODE_ENV", "development"))
	
	portGraphQL := getEnv("PORT_SEARCH_GRAPHQL", "4007")
	portMetrics := getEnv("PORT_SEARCH_METRICS", "9103")
	
	osURL := getEnv("OPENSEARCH_URL", "http://opensearch-srv:9200")
	addresses := strings.Split(osURL, ",")

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

	return &Config{
		AppEnv:              appEnv,
		NodeEnv:             appEnv,
		PortGraphQL:         portGraphQL,
		PortMetrics:         portMetrics,
		LogLevel:            getEnv("LOG_LEVEL", "info"),
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
		MaxPageSize:         maxPageSize,
		MaxQueryLength:      maxQueryLength,
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
