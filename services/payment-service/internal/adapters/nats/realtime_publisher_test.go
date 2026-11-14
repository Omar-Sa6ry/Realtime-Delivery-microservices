package nats

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startEmbeddedNATS(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true}
	ns, err := server.NewServer(opts)
	require.NoError(t, err)
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("embedded nats server did not start")
	}
	t.Cleanup(ns.Shutdown)
	return ns
}

func samplePayload() []byte {
	return []byte(`{"paymentId":"pay_1","deliveryId":"del_1","userId":"usr_1","status":"captured","updatedAt":"2026-01-02T03:04:05Z"}`)
}

func TestNewRealtimePublisherSuccess(t *testing.T) {
	ns := startEmbeddedNATS(t)

	p, err := NewRealtimePublisher(ns.ClientURL())
	require.NoError(t, err)
	require.NotNil(t, p)
	p.Close()
}

func TestNewRealtimePublisherConnectFailure(t *testing.T) {
	p, err := NewRealtimePublisher("nats://127.0.0.1:1")
	require.Error(t, err)
	assert.Nil(t, p)
	assert.ErrorContains(t, err, "failed to connect to NATS after 15 attempts")
}

func TestPublishPaymentStatusUpdatedSuccess(t *testing.T) {
	ns := startEmbeddedNATS(t)
	p, err := NewRealtimePublisher(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(p.Close)

	sub, err := p.client.Conn().SubscribeSync("payment.status.updated")
	require.NoError(t, err)
	require.NoError(t, p.client.Conn().Flush())

	require.NoError(t, p.PublishPaymentStatusUpdated(context.Background(), samplePayload()))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)

	var req struct {
		Pattern string          `json:"pattern"`
		ID      string          `json:"id"`
		Data    json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(msg.Data, &req))
	assert.Equal(t, "payment.status.updated", req.Pattern)
	assert.NotEmpty(t, req.ID)

	var update PaymentStatusUpdate
	require.NoError(t, json.Unmarshal(req.Data, &update))
	assert.Equal(t, "pay_1", update.PaymentID)
	assert.Equal(t, "del_1", update.DeliveryID)
	assert.Equal(t, "usr_1", update.UserID)
	assert.Equal(t, "captured", update.Status)
	assert.Equal(t, 2026, update.UpdatedAt.Year())
}

func TestPublishPaymentStatusUpdatedInvalidJSON(t *testing.T) {
	ns := startEmbeddedNATS(t)
	p, err := NewRealtimePublisher(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(p.Close)

	err = p.PublishPaymentStatusUpdated(context.Background(), []byte("{not json"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "unmarshal payload")
}

func TestPublishPaymentStatusUpdatedSwallowsPublishError(t *testing.T) {
	ns := startEmbeddedNATS(t)
	p, err := NewRealtimePublisher(ns.ClientURL())
	require.NoError(t, err)

	p.Close()

	require.NoError(t, p.PublishPaymentStatusUpdated(context.Background(), samplePayload()))
}

func TestPublishDirect(t *testing.T) {
	ns := startEmbeddedNATS(t)
	p, err := NewRealtimePublisher(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(p.Close)

	sub, err := p.client.Conn().SubscribeSync("custom.subject")
	require.NoError(t, err)
	require.NoError(t, p.client.Conn().Flush())

	require.NoError(t, p.PublishDirect("custom.subject", map[string]string{"a": "b"}))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	assert.Contains(t, string(msg.Data), `"pattern":"custom.subject"`)
	assert.Contains(t, string(msg.Data), `"a":"b"`)
}

func TestPublishDirectFailure(t *testing.T) {
	ns := startEmbeddedNATS(t)
	p, err := NewRealtimePublisher(ns.ClientURL())
	require.NoError(t, err)
	p.Close()

	require.Error(t, p.PublishDirect("custom.subject", map[string]string{"a": "b"}))
}

func TestPaymentStatusUpdateJSONShape(t *testing.T) {
	raw, err := json.Marshal(PaymentStatusUpdate{
		PaymentID:  "p",
		DeliveryID: "d",
		UserID:     "u",
		Status:     "s",
		UpdatedAt:  time.Unix(0, 0).UTC(),
	})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"paymentId":"p"`)
	assert.Contains(t, string(raw), `"deliveryId":"d"`)
	assert.Contains(t, string(raw), `"userId":"u"`)
	assert.Contains(t, string(raw), `"updatedAt"`)
}
