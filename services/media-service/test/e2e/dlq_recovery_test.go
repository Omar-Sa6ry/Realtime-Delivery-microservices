package e2e

import (
	"strings"
	"testing"
	"time"
)

const (
	dlqListQuery = `
query { dlqTopics }`

	dlqStatsQuery = `
query($topics: [String!]) {
  dlqStats(topics: $topics) { success statusCode message data { topic messageCount } }
}`

	dlqReplayMutation = `
mutation($topic: String!, $maxMessages: Int) {
  dlqReplay(topic: $topic, maxMessages: $maxMessages) {
    success statusCode message
    data { success replayedCount errors }
  }
}`
)

// TestDLQTopicsListed asserts the admin-visible DLQ inventory.
func TestDLQTopicsListed(t *testing.T) {
	adminToken := mintToken(t, uniqueUserID(t), "admin")
	res := gqlDo(t, adminToken, dlqListQuery, nil)
	res.assertNoErrors(t)
	var root map[string]any
	if err := unmarshalJSON(res.Data, &root); err != nil {
		t.Fatalf("decode dlqTopics: %v", err)
	}
	topics, _ := root["dlqTopics"].([]any)
	joined := ""
	for _, raw := range topics {
		joined += str(raw) + ","
	}
	for _, want := range []string{
		"media.upload.completed.dlq",
		"media.scan.completed.dlq",
		"media.processing.completed.dlq",
		"media.ready.dlq",
		"media.delete.requested.dlq",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("dlqTopics missing %q (got %s)", want, joined)
		}
	}
}

// TestDLQReplayDeniedForNonAdmin ensures replay is admin-only.
func TestDLQReplayDeniedForNonAdmin(t *testing.T) {
	userToken := mintToken(t, uniqueUserID(t), "user")
	res := gqlDo(t, userToken, dlqReplayMutation, map[string]any{
		"topic": "media.upload.completed", "maxMessages": 1,
	})
	if len(res.Errors) == 0 {
		t.Fatalf("expected non-admin dlqReplay to be rejected")
	}
}

// TestDLQStatsAndReplay drives the full DLQ lifecycle:
// poison on the DLQ topic -> stats -> consumer-side DLQ routing -> admin replay.
func TestDLQStatsAndReplay(t *testing.T) {
	adminToken := mintToken(t, uniqueUserID(t), "admin")

	// Seed the DLQ topic directly so dlqStats never blocks on an empty topic
	// (GetDLQStats reads one message with no server-side deadline).
	ProduceKafka(t, "media.upload.completed.dlq", []byte("e2e-dlq-seed"), []byte(`{"poison":"seed"}`))

	seeded := waitForDLQCountAtLeast(t, adminToken, "media.upload.completed", 1, 30*time.Second)
	if seeded < 1 {
		t.Fatalf("seeded dlq count = %d, want >= 1", seeded)
	}

	// A consumer that fails to parse the payload must route it to the DLQ.
	ProduceKafka(t, "media.upload.completed", []byte("e2e-dlq-key"), []byte(`{"poison":true`))

	deadline := time.Now().Add(60 * time.Second)
	routed := false
	for time.Now().Before(deadline) && !routed {
		for _, rec := range FetchKafka(t, "media.upload.completed.dlq", 3*time.Second, 0) {
			if string(RecordKey(t, rec)) == "e2e-dlq-key" {
				routed = true
				break
			}
		}
		if !routed {
			time.Sleep(2 * time.Second)
		}
	}
	if !routed {
		t.Fatal("poison message never routed to media.upload.completed.dlq by the consumer")
	}

	// The stats endpoint still answers for a populated DLQ topic.
	if c := dlqCount(t, adminToken, "media.upload.completed"); c < 1 {
		t.Fatalf("dlqStats = %d, want >= 1", c)
	}

	// Admin replay republishes the DLQ contents back to the source topic.
	env := gqlDo(t, adminToken, dlqReplayMutation, map[string]any{
		"topic": "media.upload.completed", "maxMessages": 5,
	}).opData(t, "dlqReplay")
	replayed := num(env["replayedCount"])
	if replayed < 1 {
		t.Fatalf("replayedCount = %v, want >= 1", replayed)
	}
}

func dlqCount(t *testing.T, adminToken, topic string) int {
	t.Helper()
	rows := gqlDo(t, adminToken, dlqStatsQuery, map[string]any{"topics": []string{topic}}).opDataSlice(t, "dlqStats")
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if str(row["topic"]) == topic+".dlq" || str(row["topic"]) == topic {
			return int(num(row["messageCount"]))
		}
	}
	return 0
}

func waitForDLQCountAtLeast(t *testing.T, adminToken, topic string, want int, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := -1
	for time.Now().Before(deadline) {
		last = dlqCount(t, adminToken, topic)
		if last >= want {
			return last
		}
		time.Sleep(2 * time.Second)
	}
	return last
}
