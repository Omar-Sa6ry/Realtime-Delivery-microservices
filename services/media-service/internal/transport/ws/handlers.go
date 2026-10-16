package ws

import (
	"encoding/json"
	"time"

	sharedws "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/websocket"
)

type MessageHandler interface {
	MessageType() sharedws.ClientMessageType
	Handle(conn *Connection, msg sharedws.ClientMessage) (sharedws.ServerMessage, error)
}

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

type SubscribeDeliveryHandler struct{}

func (h *SubscribeDeliveryHandler) MessageType() sharedws.ClientMessageType {
	return sharedws.ClientMessageSubscribeDelivery
}

func (h *SubscribeDeliveryHandler) Handle(conn *Connection, msg sharedws.ClientMessage) (sharedws.ServerMessage, error) {
	return sharedws.ServerMessage{
		Type:      sharedws.ServerMessageSubscribed,
		RequestID: msg.RequestID,
		Data:      json.RawMessage(`{"subscribed":true}`),
	}, nil
}

func BuildHandlerRegistry() *handlerRegistry {
	registry := newHandlerRegistry()
	registry.register(&PingHandler{})
	registry.register(&SubscribeDeliveryHandler{})
	return registry
}