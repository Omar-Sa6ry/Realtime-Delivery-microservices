// Package testutil provides reusable test doubles (mocks and in-memory fakes)
// for every port defined in internal/ports, plus small helpers for building
// contexts and Kafka envelopes used across the media-service test suites.
//
// All helpers are intentionally stdlib-only so that the test suites never
// depend on a third-party assertion library.
package testutil

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	kafkago "github.com/segmentio/kafka-go"
)

// Context returns a background context carrying the given user (and trace) ids,
// mirroring what the transport layers inject before reaching a use case.
func Context(userID, traceID string) context.Context {
	return sharedlogging.WithLogContext(context.Background(), sharedlogging.LogContext{
		UserID:  userID,
		TraceID: traceID,
	})
}

// Background is shorthand for context.Background().
func Background() context.Context { return context.Background() }

// Timeout returns a context bound to d, failing the test if d elapses.
func Timeout(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

// Envelope marshals payload into the Kafka EventEnvelope wire format.
func Envelope(t *testing.T, eventType, traceID string, payload interface{}) []byte {
	t.Helper()
	raw, err := kafkaadapter.MarshalEnvelope("evt-1", eventType, traceID, payload)
	if err != nil {
		t.Fatalf("MarshalEnvelope(%s): %v", eventType, err)
	}
	return raw
}

// EnvelopeMessage wraps payload into a kafka-go message carrying a valid envelope.
func EnvelopeMessage(t *testing.T, eventType, traceID string, payload interface{}) kafkago.Message {
	t.Helper()
	return kafkago.Message{
		Topic: eventType,
		Key:   []byte("key-1"),
		Value: Envelope(t, eventType, traceID, payload),
	}
}

// MustJSON marshals v or fails the test.
func MustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// Decode unmarshals raw into out or fails the test.
func Decode(t *testing.T, raw []byte, out interface{}) {
	t.Helper()
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("json.Unmarshal(%s): %v", string(raw), err)
	}
}

// DecodeEnvelope unwraps a Kafka envelope into out.
func DecodeEnvelope(t *testing.T, raw []byte, out interface{}) kafkaadapter.EventEnvelope {
	t.Helper()
	var env kafkaadapter.EventEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if out != nil {
		if err := json.Unmarshal(env.Payload, out); err != nil {
			t.Fatalf("unmarshal envelope payload: %v", err)
		}
	}
	return env
}

// NewMedia builds a Media in READY state with sensible defaults.
func NewMedia(id, owner string) *domain.Media {
	now := time.Now().UTC()
	return &domain.Media{
		MediaID:     id,
		OwnerID:     owner,
		FileName:    "photo.jpg",
		ContentType: "image/jpeg",
		MediaType:   domain.MediaTypeImage,
		Size:        1024,
		ObjectKey:   "users/" + owner + "/media/" + id + "/original/photo.jpg",
		Status:      domain.MediaStatusReady,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewUploadSession builds an upload session that is not expired.
func NewUploadSession(uploadID, mediaID, userID string) *domain.UploadSession {
	now := time.Now().UTC()
	return &domain.UploadSession{
		UploadID:   uploadID,
		MediaID:    mediaID,
		UserID:     userID,
		S3UploadID: "s3-upload-" + uploadID,
		ObjectKey:  "users/" + userID + "/media/" + mediaID + "/original/file.bin",
		TotalParts: 2,
		PartSize:   5 << 20,
		Status:     domain.UploadStatusUploading,
		ExpiresAt:  now.Add(time.Hour),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
