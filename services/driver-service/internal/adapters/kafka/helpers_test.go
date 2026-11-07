package kafka_test

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	pkgkafka "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/kafka"
	"github.com/segmentio/kafka-go"
)

const kafkaBroker = "127.0.0.1:9094"

const composeHint = "run `docker compose -f docker-compose.e2e.yml -p driver-e2e up -d --wait` from the service directory"

// probeKafka skips the test when the live Kafka broker is unreachable so a
// missing Docker daemon never fails the build.
func probeKafka(t *testing.T) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", kafkaBroker, time.Second)
	if err != nil {
		t.Skipf("Kafka at %s is not reachable (%v); %s", kafkaBroker, err, composeHint)
	}
	_ = conn.Close()
}

// uniqueName builds a Kafka-safe name from the test name plus a random-ish
// suffix so repeated runs never collide on topics or consumer groups.
func uniqueName(t *testing.T) string {
	t.Helper()
	s := strings.NewReplacer("/", "-", " ", "-", "\\", "-", "\x00", "-").Replace(t.Name())
	return fmt.Sprintf("%s.%d", s, time.Now().UnixNano())
}

// ensureTopics creates the given topics (single partition) ignoring the
// already-exists case, then blocks until every topic is actually visible in
// the cluster metadata. CreateTopics (and auto-creation) can return before the
// broker serves the new topic, which would make the very next produce fail
// with UNKNOWN_TOPIC_OR_PARTITION.
func ensureTopics(t *testing.T, topics ...string) {
	t.Helper()
	if err := pkgkafka.EnsureTopics([]string{kafkaBroker}, topics, 1, 1); err != nil {
		t.Fatalf("EnsureTopics %v: %v", topics, err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		missing := missingTopics(topics)
		if len(missing) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("topics %v not visible in cluster metadata 15s after creation", missing)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// missingTopics reports which of the given topics are not yet servable. The
// metadata request itself also nudges auto-creation on brokers that allow it.
func missingTopics(topics []string) []string {
	conn, err := kafka.Dial("tcp", kafkaBroker)
	if err != nil {
		return topics
	}
	defer conn.Close()
	var missing []string
	for _, topic := range topics {
		if _, err := conn.ReadPartitions(topic); err != nil {
			missing = append(missing, topic)
		}
	}
	return missing
}

// deleteTopics removes topics the test created, retrying briefly because a
// still-running consumer reader can keep the topic alive; failures are logged
// only so a topic in use elsewhere never fails the run.
func deleteTopics(t *testing.T, topics ...string) {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		conn, err := kafka.Dial("tcp", kafkaBroker)
		if err != nil {
			t.Logf("topic cleanup: dial failed: %v", err)
			return
		}
		lastErr = conn.DeleteTopics(topics...)
		_ = conn.Close()
		if lastErr == nil {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Logf("topic cleanup: delete %v: %v", topics, lastErr)
}

// eventually is a hand-rolled polling helper (no external dependencies).
func eventually(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for: %s", timeout, msg)
}
