package websocket

import (
	"encoding/json"
	"testing"
)

func TestRealtimeMessageVersion(t *testing.T) {
	if RealtimeMessageVersion != 1 {
		t.Errorf("expected RealtimeMessageVersion to be 1, got %d", RealtimeMessageVersion)
	}
}

func TestNewRealtimeMessage(t *testing.T) {
	msg, err := NewRealtimeMessage("msg-1", string(ServerMessageDeliveryCompleted), MessagePriorityNormal, map[string]string{"deliveryId": "d-1"})
	if err != nil {
		t.Fatalf("NewRealtimeMessage returned error: %v", err)
	}

	if msg.MessageID != "msg-1" {
		t.Errorf("expected messageId msg-1, got %s", msg.MessageID)
	}
	if msg.Type != string(ServerMessageDeliveryCompleted) {
		t.Errorf("expected type %s, got %s", ServerMessageDeliveryCompleted, msg.Type)
	}
	if msg.Version != RealtimeMessageVersion {
		t.Errorf("expected version %d, got %d", RealtimeMessageVersion, msg.Version)
	}
	if msg.Priority != MessagePriorityNormal {
		t.Errorf("expected priority NORMAL, got %s", msg.Priority)
	}
	if msg.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}

	var payload map[string]string
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if payload["deliveryId"] != "d-1" {
		t.Errorf("expected payload deliveryId d-1, got %s", payload["deliveryId"])
	}
}

func TestParseClientMessage(t *testing.T) {
	t.Run("valid message", func(t *testing.T) {
		raw := []byte(`{"type":"LOCATION_UPDATE","data":{"lat":1.5,"lng":2.5}}`)
		msg, err := ParseClientMessage(raw)
		if err != nil {
			t.Fatalf("ParseClientMessage returned error: %v", err)
		}
		if msg.Type != ClientMessageLocationUpdate {
			t.Errorf("expected type LOCATION_UPDATE, got %s", msg.Type)
		}
		var data struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		}
		if err := msg.DecodeData(&data); err != nil {
			t.Fatalf("DecodeData returned error: %v", err)
		}
		if data.Lat != 1.5 || data.Lng != 2.5 {
			t.Errorf("unexpected decoded data: %+v", data)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := ParseClientMessage([]byte("{not-json"))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
		if wsErr, ok := err.(*WsException); !ok || wsErr.Code != WsErrorInvalidMessage {
			t.Errorf("expected WsException INVALID_MESSAGE, got %v", err)
		}
	})

	t.Run("missing type", func(t *testing.T) {
		_, err := ParseClientMessage([]byte(`{"data":{}}`))
		if err == nil {
			t.Fatal("expected error for missing type")
		}
		if wsErr, ok := err.(*WsException); !ok || wsErr.Code != WsErrorInvalidMessage {
			t.Errorf("expected WsException INVALID_MESSAGE, got %v", err)
		}
	})
}

func TestServerMessageMarshal(t *testing.T) {
	data, _ := json.Marshal(map[string]string{"status": "DELIVERED"})
	msg := ServerMessage{Type: ServerMessageDeliveryStatusUpdated, Data: data}

	raw, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if decoded["type"] != string(ServerMessageDeliveryStatusUpdated) {
		t.Errorf("expected type %s, got %v", ServerMessageDeliveryStatusUpdated, decoded["type"])
	}
}