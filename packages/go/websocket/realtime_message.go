package websocket

import (
	"encoding/json"
	"fmt"
	"time"
)

// RealtimeMessageVersion is the current wire-protocol version — matches TypeScript REALTIME_MESSAGE_VERSION.
const RealtimeMessageVersion = 1

// ClientMessageType is the type of a message sent from a client to the server.
// Matches the TypeScript ClientMessageType enum.
type ClientMessageType string

const (
	ClientMessagePing                ClientMessageType = "PING"
	ClientMessageSubscribeDelivery   ClientMessageType = "SUBSCRIBE_DELIVERY"
	ClientMessageUnsubscribeDelivery ClientMessageType = "UNSUBSCRIBE_DELIVERY"
	ClientMessageLocationUpdate      ClientMessageType = "LOCATION_UPDATE"
	ClientMessageAcceptAssignment    ClientMessageType = "ACCEPT_ASSIGNMENT"
	ClientMessageRejectAssignment    ClientMessageType = "REJECT_ASSIGNMENT"
	ClientMessageCompleteDelivery    ClientMessageType = "COMPLETE_DELIVERY"
	ClientMessageAck                 ClientMessageType = "ACK"
)

// ServerMessageType is the type of a message sent from the server to a client.
// Matches the TypeScript ServerMessageType enum.
type ServerMessageType string

const (
	ServerMessageConnected               ServerMessageType = "CONNECTED"
	ServerMessageSubscribed              ServerMessageType = "SUBSCRIBED"
	ServerMessageUnsubscribed            ServerMessageType = "UNSUBSCRIBED"
	ServerMessagePong                    ServerMessageType = "PONG"
	ServerMessageAck                     ServerMessageType = "ACK"
	ServerMessageDeliveryLocationUpdated ServerMessageType = "DELIVERY_LOCATION_UPDATED"
	ServerMessageDeliveryStatusUpdated   ServerMessageType = "DELIVERY_STATUS_UPDATED"
	ServerMessageDriverAssigned          ServerMessageType = "DRIVER_ASSIGNED"
	ServerMessageDriverPresenceUpdated   ServerMessageType = "DRIVER_PRESENCE_UPDATED"
	ServerMessageDeliveryCompleted       ServerMessageType = "DELIVERY_COMPLETED"
	ServerMessageDeliveryCancelled       ServerMessageType = "DELIVERY_CANCELLED"
	ServerMessagePaymentStatusChanged    ServerMessageType = "PAYMENT_STATUS_CHANGED"
	ServerMessageNotificationReceived    ServerMessageType = "NOTIFICATION_RECEIVED"
	ServerMessageLocationUpdateRejected  ServerMessageType = "LOCATION_UPDATE_REJECTED"
	ServerMessageError                   ServerMessageType = "ERROR"
	// Media messages
	ServerMessageMediaUploadProgress     ServerMessageType = "MEDIA_UPLOAD_PROGRESS"
	ServerMessageMediaProcessingProgress ServerMessageType = "MEDIA_PROCESSING_PROGRESS"
	ServerMessageMediaReady              ServerMessageType = "MEDIA_READY"
	ServerMessageMediaDeleted            ServerMessageType = "MEDIA_DELETED"
	ServerMessageMediaFailed             ServerMessageType = "MEDIA_FAILED"
)

// MessagePriority is the delivery priority of a realtime message.
// Matches the TypeScript MessagePriority enum.
type MessagePriority string

const (
	MessagePriorityCritical           MessagePriority = "CRITICAL"
	MessagePriorityNormal             MessagePriority = "NORMAL"
	MessagePriorityHighFrequencyLossy MessagePriority = "HIGH_FREQUENCY_LOSSY"
)

// RealtimeMessage is the canonical envelope for every realtime message —
// matches the TypeScript RealtimeMessage interface.
type RealtimeMessage struct {
	MessageID string          `json:"messageId"`
	Type      string          `json:"type"`
	Version   int             `json:"version"`
	Timestamp string          `json:"timestamp"`
	Priority  MessagePriority `json:"priority"`
	Data      json.RawMessage `json:"data"`
}

// ClientMessage is a message sent from a client to the server —
// matches the TypeScript ClientMessage interface.
type ClientMessage struct {
	RequestID string            `json:"requestId,omitempty"`
	Type      ClientMessageType `json:"type"`
	Data      json.RawMessage   `json:"data"`
}

// ServerMessage is a message sent from the server to a client —
// matches the TypeScript ServerMessage interface.
type ServerMessage struct {
	RequestID string            `json:"requestId,omitempty"`
	Type      ServerMessageType `json:"type"`
	Data      json.RawMessage   `json:"data"`
}

// NewRealtimeMessage builds a RealtimeMessage with the current protocol version
// and an RFC3339 timestamp — mirrors the TS RealtimeMessage contract.
func NewRealtimeMessage(messageID, messageType string, priority MessagePriority, data interface{}) (*RealtimeMessage, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal realtime message data: %w", err)
	}
	return &RealtimeMessage{
		MessageID: messageID,
		Type:      messageType,
		Version:   RealtimeMessageVersion,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Priority:  priority,
		Data:      dataBytes,
	}, nil
}

// ParseClientMessage parses a raw WebSocket frame into a ClientMessage.
// Returns WsException(INVALID_MESSAGE) for malformed frames — mirrors the
// routeMessage parsing in the TS RealtimeWsAdapter.
func ParseClientMessage(data []byte) (*ClientMessage, error) {
	var msg ClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, NewWsException(WsErrorInvalidMessage, "Invalid JSON message", false)
	}
	if msg.Type == "" {
		return nil, NewWsException(WsErrorInvalidMessage, "Missing message type", false)
	}
	return &msg, nil
}

// DecodeData unmarshals the client message payload into out.
func (m *ClientMessage) DecodeData(out interface{}) error {
	if len(m.Data) == 0 {
		return nil
	}
	return json.Unmarshal(m.Data, out)
}

// DecodeData unmarshals the server message payload into out.
func (m *ServerMessage) DecodeData(out interface{}) error {
	if len(m.Data) == 0 {
		return nil
	}
	return json.Unmarshal(m.Data, out)
}

// Marshal serialises the message to JSON bytes ready for the wire.
func (m *ServerMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}