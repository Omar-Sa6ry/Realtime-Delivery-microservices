package config_test

import (
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/config"
)

// allEnvVars lists every environment variable read by config.Load.
var allEnvVars = []string{
	"PORT_DRIVER_GRAPHQL",
	"PORT_DRIVER_GRPC",
	"PORT_DRIVER_METRICS",
	"MONGODB_URI",
	"MONGODB_DATABASE",
	"REDIS_HOST",
	"REDIS_PORT",
	"KAFKA_BROKERS",
	"KAFKA_GROUP_ID",
	"KAFKA_CLIENT_ID",
	"NATS_URL",
	"DISPATCH_SEARCH_RADIUS_KM",
	"ASSIGNMENT_TIMEOUT_SECONDS",
	"LOCATION_STALE_SECONDS",
	"LOCK_TTL_MS",
	"MAX_DISPATCH_ATTEMPTS",
	"USER_SERVICE_URL",
	"DELIVERY_SERVICE_URL",
	"OTEL_ENDPOINT",
}

// clearEnv pins every known variable to the empty string so that getEnv
// falls back to the documented defaults, regardless of the ambient env.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, v := range allEnvVars {
		t.Setenv(v, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg := config.Load()
	if cfg == nil {
		t.Fatalf("Load() returned nil")
	}

	tests := []struct {
		field    string
		got      string
		expected string
	}{
		{"PortGraphQL", cfg.PortGraphQL, "4008"},
		{"PortGRPC", cfg.PortGRPC, "50055"},
		{"PortMetrics", cfg.PortMetrics, "9105"},
		{"MongoDBURI", cfg.MongoDBURI, "mongodb://localhost:27017"},
		{"MongoDBDatabase", cfg.MongoDBDatabase, "driver_db"},
		{"RedisHost", cfg.RedisHost, "localhost"},
		{"RedisPort", cfg.RedisPort, "6379"},
		{"KafkaBrokers", cfg.KafkaBrokers, "localhost:9092"},
		{"KafkaGroupID", cfg.KafkaGroupID, "driver-service"},
		{"KafkaClientID", cfg.KafkaClientID, "driver-service"},
		{"NATSURL", cfg.NATSURL, "nats://localhost:4222"},
		{"DispatchSearchRadius", cfg.DispatchSearchRadius, "5"},
		{"AssignmentTimeout", cfg.AssignmentTimeout, "20"},
		{"LocationStaleSeconds", cfg.LocationStaleSeconds, "30"},
		{"LockTTLMs", cfg.LockTTLMs, "5000"},
		{"MaxDispatchAttempts", cfg.MaxDispatchAttempts, "5"},
		{"UserServiceURL", cfg.UserServiceURL, "user-srv:50051"},
		{"DeliveryServiceURL", cfg.DeliveryServiceURL, "delivery-srv:50054"},
		{"OTELEndpoint", cfg.OTELEndpoint, ""},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("Load() %s = %q, want %q", tt.field, tt.got, tt.expected)
			}
		})
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	clearEnv(t)

	overrides := map[string]string{
		"PORT_DRIVER_GRAPHQL":        "14008",
		"PORT_DRIVER_GRPC":           "150055",
		"PORT_DRIVER_METRICS":        "19105",
		"MONGODB_URI":                "mongodb://mongo-1:27017,mongo-2:27017",
		"MONGODB_DATABASE":           "driver_db_test",
		"REDIS_HOST":                 "redis.internal",
		"REDIS_PORT":                 "6380",
		"KAFKA_BROKERS":              "kafka-1:9092,kafka-2:9092",
		"KAFKA_GROUP_ID":             "driver-service-test",
		"KAFKA_CLIENT_ID":            "driver-client-test",
		"NATS_URL":                   "nats://nats.internal:4222",
		"DISPATCH_SEARCH_RADIUS_KM":  "12.5",
		"ASSIGNMENT_TIMEOUT_SECONDS": "45",
		"LOCATION_STALE_SECONDS":     "90",
		"LOCK_TTL_MS":                "7500",
		"MAX_DISPATCH_ATTEMPTS":      "9",
		"USER_SERVICE_URL":           "user.internal:50051",
		"DELIVERY_SERVICE_URL":       "delivery.internal:50054",
		"OTEL_ENDPOINT":              "otel-collector:4317",
	}

	for k, v := range overrides {
		t.Setenv(k, v)
	}

	cfg := config.Load()

	tests := []struct {
		field    string
		got      string
		expected string
	}{
		{"PortGraphQL", cfg.PortGraphQL, "14008"},
		{"PortGRPC", cfg.PortGRPC, "150055"},
		{"PortMetrics", cfg.PortMetrics, "19105"},
		{"MongoDBURI", cfg.MongoDBURI, "mongodb://mongo-1:27017,mongo-2:27017"},
		{"MongoDBDatabase", cfg.MongoDBDatabase, "driver_db_test"},
		{"RedisHost", cfg.RedisHost, "redis.internal"},
		{"RedisPort", cfg.RedisPort, "6380"},
		{"KafkaBrokers", cfg.KafkaBrokers, "kafka-1:9092,kafka-2:9092"},
		{"KafkaGroupID", cfg.KafkaGroupID, "driver-service-test"},
		{"KafkaClientID", cfg.KafkaClientID, "driver-client-test"},
		{"NATSURL", cfg.NATSURL, "nats://nats.internal:4222"},
		{"DispatchSearchRadius", cfg.DispatchSearchRadius, "12.5"},
		{"AssignmentTimeout", cfg.AssignmentTimeout, "45"},
		{"LocationStaleSeconds", cfg.LocationStaleSeconds, "90"},
		{"LockTTLMs", cfg.LockTTLMs, "7500"},
		{"MaxDispatchAttempts", cfg.MaxDispatchAttempts, "9"},
		{"UserServiceURL", cfg.UserServiceURL, "user.internal:50051"},
		{"DeliveryServiceURL", cfg.DeliveryServiceURL, "delivery.internal:50054"},
		{"OTELEndpoint", cfg.OTELEndpoint, "otel-collector:4317"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("Load() %s = %q, want %q", tt.field, tt.got, tt.expected)
			}
		})
	}
}

// TestLoadOnlyOneOverride set a single variable and verifies every other
// field still gets its default (i.e. unrelated keys do not interfere).
func TestLoadOnlyOneOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("PORT_DRIVER_GRAPHQL", "4444")

	cfg := config.Load()
	if cfg.PortGraphQL != "4444" {
		t.Errorf("PortGraphQL = %q, want %q", cfg.PortGraphQL, "4444")
	}
	if cfg.PortGRPC != "50055" {
		t.Errorf("PortGRPC = %q, want %q", cfg.PortGRPC, "50055")
	}
	if cfg.OTELEndpoint != "" {
		t.Errorf("OTELEndpoint = %q, want empty", cfg.OTELEndpoint)
	}
}
