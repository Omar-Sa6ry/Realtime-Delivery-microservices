package e2e

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// e2eConfig holds every endpoint the suite talks to. All values are overridable
// through environment variables so the same suite can target the in-cluster
// driver-service (default) or a locally instrumented instance.
type e2eConfig struct {
	GraphQLBase string
	GraphQLURL  string
	WSURL       string
	GRPCAddr    string
	MongoURI    string
	MongoDB     string
	RedisAddr   string
	NATSURL     string
	KafkaBroker string
	RabbitMQURL string

	JWTSecret string
}

var cfg e2eConfig

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loadConfig() e2eConfig {
	return e2eConfig{
		GraphQLBase: envOr("DRIVER_E2E_GRAPHQL_BASE", "http://127.0.0.1:4008"),
		GraphQLURL:  envOr("DRIVER_E2E_GRAPHQL", "http://127.0.0.1:4008/driver/graphql"),
		WSURL:       envOr("DRIVER_E2E_WS", "ws://127.0.0.1:4008/ws"),
		GRPCAddr:    envOr("DRIVER_E2E_GRPC", "127.0.0.1:50055"),
		MongoURI:    envOr("DRIVER_E2E_MONGO", "mongodb://127.0.0.1:27018"),
		MongoDB:     envOr("DRIVER_E2E_MONGO_DB", "driver_db_e2e_test"),
		RedisAddr:   envOr("DRIVER_E2E_REDIS", "127.0.0.1:6380"),
		NATSURL:     envOr("DRIVER_E2E_NATS", "nats://127.0.0.1:4223"),
		KafkaBroker: envOr("DRIVER_E2E_KAFKA", "127.0.0.1:9094"),
		RabbitMQURL: envOr("DRIVER_E2E_RABBITMQ", "amqp://guest:guest@127.0.0.1:5673/"),

		JWTSecret: envOr("DRIVER_E2E_JWT_SECRET", "f3c12be5d179b8d5690d4735a0b48b5d68e7b68e3cda6b3"),
	}
}

func enabled() bool { return os.Getenv("DRIVER_E2E") == "1" }

func TestMain(m *testing.M) {
	if !enabled() {
		fmt.Println("driver-service e2e: skipped (set DRIVER_E2E=1 to run against live infrastructure)")
		os.Exit(0)
	}
	cfg = loadConfig()

	if err := waitForInfrastructure(90 * time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "driver-service e2e: infrastructure not ready: %v\n", err)
		os.Exit(1)
	}

	if err := ensureInfra(); err != nil {
		fmt.Fprintf(os.Stderr, "driver-service e2e: infra setup failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// waitForInfrastructure polls every backing service so tests fail fast with a
// single clear error instead of dozens of timeouts.
func waitForInfrastructure(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	checks := []struct {
		name string
		fn   func() error
	}{
		{"graphql /healthz", func() error { return httpOK(cfg.GraphQLBase + "/healthz") }},
		{"mongo ping", func() error { return checkMongo() }},
		{"redis PING", checkRedis},
		{"nats INFO", checkNATS},
		{"kafka tcp", checkKafka},
		{"grpc tcp", func() error { return checkTCP(cfg.GRPCAddr) }},
		{"rabbitmq tcp", func() error { return checkTCP(hostPortOf(cfg.RabbitMQURL)) }},
	}
	var lastErr error
	for time.Now().Before(deadline) {
		allOK := true
		for _, c := range checks {
			if err := c.fn(); err != nil {
				lastErr = fmt.Errorf("%s: %w", c.name, err)
				allOK = false
				break
			}
		}
		if allOK {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return lastErr
}

func ensureInfra() error {
	// Ensure Kafka topics exist
	// Topics are auto-created by the service on startup, but we can pre-create them here if needed
	return nil
}

func httpOK(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func checkTCP(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return err
	}
	return conn.Close()
}

func checkRedis() error {
	conn, err := net.DialTimeout("tcp", cfg.RedisAddr, 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if len(line) < 4 || line[:4] != "+PON" {
		return fmt.Errorf("unexpected reply %q", line)
	}
	return nil
}

func checkNATS() error {
	conn, err := net.DialTimeout("tcp", hostPortOf(cfg.NATSURL), 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	if n < 5 || string(buf[:5]) != "INFO " {
		return fmt.Errorf("unexpected handshake %q", string(buf[:min(n, 40)]))
	}
	return nil
}

func checkKafka() error { return checkTCP(cfg.KafkaBroker) }

func checkMongo() error {
	// Simple TCP check for now; actual MongoDB ping would need mongo client
	return checkTCP(hostPortOf(cfg.MongoURI))
}

func hostPortOf(u string) string {
	addr := u
	if len(u) > 10 && u[:10] == "mongodb://" {
		addr = u[10:]
	} else if len(u) > 7 && u[:7] == "nats://" {
		addr = u[7:]
	} else if len(u) > 7 && u[:7] == "amqp://" {
		// strip user:pass@ and any trailing path
		rest := u[7:]
		if j := findAt(rest); j >= 0 {
			rest = rest[j+1:]
		}
		addr = strings.SplitN(rest, "/", 2)[0]
	} else if len(u) > 8 && u[:8] == "redis://" {
		addr = u[8:]
	}
	return addr
}

func findAt(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '@' {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}