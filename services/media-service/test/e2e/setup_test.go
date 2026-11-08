package e2e

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// e2eConfig holds every endpoint the suite talks to. All values are overridable
// through environment variables so the same suite can target the in-cluster
// media-service (default) or a locally instrumented instance.
type e2eConfig struct {
	GraphQLBase string
	GraphQLURL  string
	WSURL       string
	GRPCAddr    string
	S3Endpoint  string
	DDBEndpoint string
	RedisAddr   string
	NATSURL     string
	KafkaBroker string

	JWTSecret string
	AWSKey    string
	AWSSecret string
	Region    string
	Bucket    string
	Table     string
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
		GraphQLBase: envOr("MEDIA_E2E_GRAPHQL_BASE", "http://127.0.0.1:4005"),
		GraphQLURL:  envOr("MEDIA_E2E_GRAPHQL", "http://127.0.0.1:4005/media/graphql"),
		WSURL:       envOr("MEDIA_E2E_WS", "ws://127.0.0.1:4003/ws"),
		GRPCAddr:    envOr("MEDIA_E2E_GRPC", "127.0.0.1:50052"),
		S3Endpoint:  envOr("MEDIA_E2E_S3", "http://127.0.0.1:4566"),
		DDBEndpoint: envOr("MEDIA_E2E_DDB", "http://127.0.0.1:4566"),
		RedisAddr:   envOr("MEDIA_E2E_REDIS", "127.0.0.1:6379"),
		NATSURL:     envOr("MEDIA_E2E_NATS", "nats://127.0.0.1:4222"),
		KafkaBroker: envOr("MEDIA_E2E_KAFKA", "127.0.0.1:9092"),

		JWTSecret: envOr("MEDIA_E2E_JWT_SECRET", "f3c12be5d179b8d5690d4735a0b48b5d68e7b68e3cda6b3"),
		AWSKey:    envOr("MEDIA_E2E_AWS_KEY", "test"),
		AWSSecret: envOr("MEDIA_E2E_AWS_SECRET", "test"),
		Region:    envOr("MEDIA_E2E_AWS_REGION", "us-east-1"),
		Bucket:    envOr("MEDIA_E2E_S3_BUCKET", "delivery-micro"),
		Table:     envOr("MEDIA_E2E_DDB_TABLE", "media-service"),
	}
}

func enabled() bool { return os.Getenv("MEDIA_E2E") == "1" }

func TestMain(m *testing.M) {
	if !enabled() {
		fmt.Println("media-service e2e: skipped (set MEDIA_E2E=1 to run against live infrastructure)")
		os.Exit(0)
	}
	cfg = loadConfig()

	if err := waitForInfrastructure(60 * time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "media-service e2e: infrastructure not ready: %v\n", err)
		os.Exit(1)
	}

	if err := ensureInfra(); err != nil {
		fmt.Fprintf(os.Stderr, "media-service e2e: infra setup failed: %v\n", err)
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
		{"localstack /_localstack/health", func() error { return httpOK(cfg.S3Endpoint + "/_localstack/health") }},
		{"redis PING", checkRedis},
		{"nats INFO", checkNATS},
		{"kafka tcp", checkKafka},
		{"grpc tcp", func() error { return checkTCP(cfg.GRPCAddr) }},
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

func hostPortOf(url string) string {
	addr := url
	if i := len("nats://"); len(url) > i && url[:i] == "nats://" {
		addr = url[i:]
	}
	return addr
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
