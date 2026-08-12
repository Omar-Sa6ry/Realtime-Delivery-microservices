// Package ws implements the WebSocket notification adapter using Redis Pub/Sub.
// It allows multiple media-service replicas to broadcast events to WebSocket
// clients connected to any pod in the cluster.
//
// Topology:
//
//	Worker → PubSubNotifier.Notify()
//	       → redis.PUBLISH ws:user:{userId}  (fan-out across all pods)
//	       → WebSocket Hub (each pod subscribes to its connected users)
//	       → Browser
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

const channelPrefix = "ws:user:"

// PubSubNotifier implements ports.Notifier by publishing events to Redis Pub/Sub.
// The WebSocket Hub subscribes to the same channels and forwards messages to browsers.
type PubSubNotifier struct {
	client *goredis.Client
}

// NewPubSubNotifier creates a new Redis-backed Pub/Sub notifier.
func NewPubSubNotifier(client *goredis.Client) *PubSubNotifier {
	return &PubSubNotifier{client: client}
}

// Notify serialises the MediaEvent to JSON and publishes it on the user's Redis channel.
// This is intentionally lightweight — the heavy fan-out happens inside the WebSocket Hub.
func (n *PubSubNotifier) Notify(ctx context.Context, userID string, event ports.MediaEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("pubsub notify: marshal event: %w", err)
	}

	channel := channelPrefix + userID
	if err := n.client.Publish(ctx, channel, data).Err(); err != nil {
		slog.Warn("PubSubNotifier: failed to publish event",
			"channel", channel,
			"eventType", event.EventType,
			"mediaId", event.MediaID,
			"error", err,
		)
		return fmt.Errorf("pubsub notify: redis publish: %w", err)
	}

	return nil
}

// Close is a no-op for the publisher — the Redis client lifecycle is managed externally.
func (n *PubSubNotifier) Close() error {
	return nil
}

// ChannelForUser returns the Redis Pub/Sub channel name for a given user.
// Exported so the WebSocket Hub can subscribe using the same naming convention.
func ChannelForUser(userID string) string {
	return channelPrefix + userID
}
