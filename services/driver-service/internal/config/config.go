package config

import (
	"os"
)

type Config struct {
	PortGraphQL          string
	PortGRPC             string
	PortMetrics          string
	MongoDBURI           string
	MongoDBDatabase      string
	DispatchSearchRadius string
	AssignmentTimeout    string
	LocationStaleSeconds string
	LockTTLMs            string
	MaxDispatchAttempts  string
	KafkaGroupID         string
	KafkaClientID        string
	NATSURL              string
}

func Load() *Config {
	return &Config{
		PortGraphQL:          getEnv("PORT_DRIVER_GRAPHQL", "4008"),
		PortGRPC:             getEnv("PORT_DRIVER_GRPC", "50055"),
		PortMetrics:          getEnv("PORT_DRIVER_METRICS", "9105"),
		MongoDBURI:           getEnv("MONGODB_URI", "mongodb://localhost:27017"),
		MongoDBDatabase:      getEnv("MONGODB_DATABASE", "driver_db"),
		DispatchSearchRadius: getEnv("DISPATCH_SEARCH_RADIUS_KM", "5"),
		AssignmentTimeout:    getEnv("ASSIGNMENT_TIMEOUT_SECONDS", "20"),
		LocationStaleSeconds: getEnv("LOCATION_STALE_SECONDS", "30"),
		LockTTLMs:            getEnv("LOCK_TTL_MS", "5000"),
		MaxDispatchAttempts:  getEnv("MAX_DISPATCH_ATTEMPTS", "5"),
		KafkaGroupID:         getEnv("KAFKA_GROUP_ID", "driver-service"),
		KafkaClientID:        getEnv("KAFKA_CLIENT_ID", "driver-service"),
		NATSURL:              getEnv("NATS_URL", "nats://localhost:4222"),
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}