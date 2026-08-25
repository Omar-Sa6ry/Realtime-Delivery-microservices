package ws

import (
	"encoding/json"
	"time"

	sharedws "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/websocket"
)

// MessageHandler defines the strategy interface for handling specific client message types.
type MessageHandler interface {
	// MessageType returns the message type this handler processes.
	MessageType() sharedws.ClientMessageType

	// Handle processes the message and returns a response to send back to the client.
	Handle(conn *Connection, msg sharedws.ClientMessage) (sharedws.ServerMessage, error)
}

// handlerRegistry holds all registered message handlers.
type handlerRegistry struct {
	handlers map[sharedws.ClientMessageType]MessageHandler
}

func newHandlerRegistry() *handlerRegistry {
	return &handlerRegistry{
		handlers: make(map[sharedws.ClientMessageType]MessageHandler),
	}
}

func (r *handlerRegistry) register(h MessageHandler) {
	r.handlers[h.MessageType()] = h
}

func (r *handlerRegistry) get(msgType sharedws.ClientMessageType) (MessageHandler, bool) {
	h, ok := r.handlers[msgType]
	return h, ok
}

// PingHandler handles PING messages.
type PingHandler struct{}

func (h *PingHandler) MessageType() sharedws.ClientMessageType {
	return sharedws.ClientMessagePing
}

func (h *PingHandler) Handle(conn *Connection, msg sharedws.ClientMessage) (sharedws.ServerMessage, error) {
	return sharedws.ServerMessage{
		Type:      sharedws.ServerMessagePong,
		RequestID: msg.RequestID,
		Data:      json.RawMessage(`{"timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `"}`),
	}, nil
}

// SubscribeDeliveryHandler handles SUBSCRIBE_DELIVERY messages.
type SubscribeDeliveryHandler struct{}

func (h *SubscribeDeliveryHandler) MessageType() sharedws.ClientMessageType {
	return sharedws.ClientMessageSubscribeDelivery
}

func (h *SubscribeDeliveryHandler) Handle(conn *Connection, msg sharedws.ClientMessage) (sharedws.ServerMessage, error) {
	// For media, we could subscribe to specific media IDs
	// For now, just ack
	return sharedws.ServerMessage{
		Type:      sharedws.ServerMessageSubscribed,
		RequestID: msg.RequestID,
		Data:      json.RawMessage(`{"subscribed":true}`),
	}, nil
}

// BuildHandlerRegistry creates and populates the message handler registry.
func BuildHandlerRegistry() *handlerRegistry {
	registry := newHandlerRegistry()
	registry.register(&PingHandler{})
	registry.register(&SubscribeDeliveryHandler{})
	return registry
}