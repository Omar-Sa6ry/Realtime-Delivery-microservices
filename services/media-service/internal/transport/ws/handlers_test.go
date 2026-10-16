package ws

import (
	"testing"

	sharedws "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/websocket"
)

func TestHandlerRegistryRegistersAndGets(t *testing.T) {
	r := BuildHandlerRegistry()

	ping, ok := r.get(sharedws.ClientMessagePing)
	if !ok {
		t.Fatal("PING handler not registered")
	}
	if ping.MessageType() != sharedws.ClientMessagePing {
		t.Errorf("ping type = %q", ping.MessageType())
	}

	sub, ok := r.get(sharedws.ClientMessageSubscribeDelivery)
	if !ok {
		t.Fatal("SUBSCRIBE_DELIVERY handler not registered")
	}
	if sub.MessageType() != sharedws.ClientMessageSubscribeDelivery {
		t.Errorf("subscribe type = %q", sub.MessageType())
	}
}

func TestHandlerRegistryGetUnknownReturnsFalse(t *testing.T) {
	r := BuildHandlerRegistry()
	if h, ok := r.get("UNKNOWN_TYPE"); ok || h != nil {
		t.Errorf("get(unknown) = %v, %v; want nil, false", h, ok)
	}
}

func TestHandlerRegistryOverwrite(t *testing.T) {
	r := newHandlerRegistry()
	first := &stubHandler{typ: sharedws.ClientMessagePing}
	r.register(first)
	second := &stubHandler{typ: sharedws.ClientMessagePing, marker: "second"}
	r.register(second)

	got, ok := r.get(sharedws.ClientMessagePing)
	if !ok {
		t.Fatal("handler not found")
	}
	if got.(*stubHandler).marker != "second" {
		t.Error("register should overwrite handler for same type")
	}
}

func TestPingHandlerHandle(t *testing.T) {
	h := &PingHandler{}
	msg := sharedws.ClientMessage{RequestID: "req-1"}

	resp, err := h.Handle(&Connection{}, msg)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if resp.Type != sharedws.ServerMessagePong {
		t.Errorf("type = %q, want PONG", resp.Type)
	}
	if resp.RequestID != "req-1" {
		t.Errorf("requestId = %q", resp.RequestID)
	}
	if len(resp.Data) == 0 {
		t.Error("expected timestamp data")
	}
}

func TestSubscribeDeliveryHandlerHandle(t *testing.T) {
	h := &SubscribeDeliveryHandler{}
	msg := sharedws.ClientMessage{RequestID: "req-2"}

	resp, err := h.Handle(&Connection{}, msg)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if resp.Type != sharedws.ServerMessageSubscribed {
		t.Errorf("type = %q, want SUBSCRIBED", resp.Type)
	}
	if resp.RequestID != "req-2" {
		t.Errorf("requestId = %q", resp.RequestID)
	}
	if string(resp.Data) != `{"subscribed":true}` {
		t.Errorf("data = %s", resp.Data)
	}
}

type stubHandler struct {
	typ    sharedws.ClientMessageType
	marker string
}

func (s *stubHandler) MessageType() sharedws.ClientMessageType { return s.typ }

func (s *stubHandler) Handle(conn *Connection, msg sharedws.ClientMessage) (sharedws.ServerMessage, error) {
	return sharedws.ServerMessage{RequestID: msg.RequestID}, nil
}
