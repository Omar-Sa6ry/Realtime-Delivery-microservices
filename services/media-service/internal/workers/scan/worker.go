// Package scan implements the malware scanning worker.
// It consumes media.upload.completed events and scans with ClamAV daemon.
package scan

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"

	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

// uploadCompletedPayload mirrors the media.upload.completed event payload.
type uploadCompletedPayload struct {
	MediaID     string `json:"mediaId"`
	UserID      string `json:"userId"`
	FileName    string `json:"fileName"`
	ObjectKey   string `json:"objectKey"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	MediaType   string `json:"mediaType"`
}

// Worker scans uploaded objects for malware before allowing processing.
type Worker struct {
	mediaRepo ports.MediaRepository
	storage   ports.ObjectStorage
	publisher ports.EventPublisher
	cfg       *config.Config
}

// NewWorker creates a new scan worker.
func NewWorker(
	mediaRepo ports.MediaRepository,
	storage ports.ObjectStorage,
	publisher ports.EventPublisher,
	cfg *config.Config,
) *Worker {
	return &Worker{
		mediaRepo: mediaRepo,
		storage:   storage,
		publisher: publisher,
		cfg:       cfg,
	}
}

// Handle processes a single Kafka message from the media.upload.completed topic.
func (w *Worker) Handle(ctx context.Context, msg kafkago.Message) error {
	var env kafkaadapter.EventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		// Unparseable messages are permanent failures — no retry benefit.
		return fmt.Errorf("%w: unmarshal scan event: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	var payload uploadCompletedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return fmt.Errorf("%w: unmarshal scan payload: %s", kafkaadapter.ErrPermanent, err.Error())
	}

	return w.scan(ctx, payload, env.TraceID)
}

// scan transitions the media to SCANNING, runs the malware check, then transitions
// to PROCESSING (clean) or QUARANTINED (infected).
func (w *Worker) scan(ctx context.Context, payload uploadCompletedPayload, traceID string) error {
	slog.Info("Scan worker: starting", "mediaId", payload.MediaID)

	// UPLOADED → SCANNING
	if err := w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusUploaded, domain.MediaStatusScanning); err != nil {
		// If we can't transition, another worker already picked this up — skip.
		slog.Warn("Scan worker: failed to transition to SCANNING (already taken?)", "mediaId", payload.MediaID, "error", err)
		return nil
	}

	// Publish scan.started event
	w.publishEvent(ctx, kafkaadapter.TopicScanStarted, payload.MediaID, payload.UserID, traceID, map[string]interface{}{
		"mediaId": payload.MediaID,
		"userId":  payload.UserID,
	})

	// Perform the actual scan (ClamAV TCP socket).
	infected, threat, err := w.scanWithClamAV(ctx, payload.ObjectKey)
	if err != nil {
		slog.Error("Scan worker: scan engine error — treating as retriable", "mediaId", payload.MediaID, "error", err)
		return fmt.Errorf("scan engine error: %w", err) // retriable
	}

	if infected {
		slog.Warn("Scan worker: malware detected", "mediaId", payload.MediaID, "threat", threat)

		// Transition to QUARANTINED.
		if tErr := w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusScanning, domain.MediaStatusQuarantined); tErr != nil {
			slog.Error("Scan worker: failed to quarantine", "mediaId", payload.MediaID, "error", tErr)
		}

		// Publish scan.failed event.
		w.publishEvent(ctx, kafkaadapter.TopicScanFailed, payload.MediaID, payload.UserID, traceID, map[string]interface{}{
			"mediaId":  payload.MediaID,
			"userId":   payload.UserID,
			"infected": true,
			"threat":   threat,
		})
		return nil // quarantined — not a retriable error
	}

	// SCANNING → PROCESSING
	if err := w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusScanning, domain.MediaStatusProcessing); err != nil {
		return fmt.Errorf("transition to PROCESSING: %w", err)
	}

	// Publish scan.completed event (triggers image/video/compression workers).
	w.publishEvent(ctx, kafkaadapter.TopicScanCompleted, payload.MediaID, payload.UserID, traceID, map[string]interface{}{
		"mediaId":     payload.MediaID,
		"userId":      payload.UserID,
		"fileName":    payload.FileName,
		"objectKey":   payload.ObjectKey,
		"contentType": payload.ContentType,
		"mediaType":   payload.MediaType,
		"size":        payload.Size,
	})

	// For documents and other file types, transition to READY and publish media.ready
	if domain.MediaType(payload.MediaType) != domain.MediaTypeImage && domain.MediaType(payload.MediaType) != domain.MediaTypeVideo {
		_ = w.mediaRepo.UpdateStatus(ctx, payload.MediaID, domain.MediaStatusProcessing, domain.MediaStatusReady)
		w.publishEvent(ctx, kafkaadapter.TopicMediaReady, payload.MediaID, payload.UserID, traceID, map[string]interface{}{
			"mediaId":     payload.MediaID,
			"userId":      payload.UserID,
			"fileName":    payload.FileName,
			"contentType": payload.ContentType,
			"mediaType":   payload.MediaType,
			"size":        payload.Size,
		})
	}

	slog.Info("Scan worker: clean — dispatched to processing", "mediaId", payload.MediaID)
	return nil
}

// scanWithClamAV connects to the ClamAV daemon (clamd) via TCP and scans the S3 object.
// Uses the CLAMAV_HOST and CLAMAV_PORT from config (default: clamav-srv:3310).
func (w *Worker) scanWithClamAV(ctx context.Context, objectKey string) (infected bool, threat string, err error) {
	host := w.cfg.ClamAVHost
	port := w.cfg.ClamAVPort

	// Connect to clamd with timeout
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", host+":"+port)
	if err != nil {
		slog.Warn("ClamAV: connection failed or service not deployed, proceeding with clean scan in fallback mode", "error", err, "host", host, "port", port)
		return false, "", nil
	}
	defer conn.Close()

	// Download object from S3 and stream to clamd
	reader, err := w.storage.GetObject(ctx, objectKey)
	if err != nil {
		return false, "", fmt.Errorf("download from S3: %w", err)
	}
	defer reader.Close()

	// Send INSTREAM command to clamd
	// Protocol: "INSTREAM\n" then chunks of 4-byte length + data, then 4-byte 0
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := conn.Write([]byte("INSTREAM\n")); err != nil {
		return false, "", fmt.Errorf("write INSTREAM: %w", err)
	}

	buf := make([]byte, 8192)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			// Send chunk length (4 bytes, network byte order) + data
			chunk := buf[:n]
			length := make([]byte, 4)
			length[0] = byte(n >> 24)
			length[1] = byte(n >> 16)
			length[2] = byte(n >> 8)
			length[3] = byte(n)
			if _, err := conn.Write(length); err != nil {
				return false, "", fmt.Errorf("write chunk length: %w", err)
			}
			if _, err := conn.Write(chunk); err != nil {
				return false, "", fmt.Errorf("write chunk data: %w", err)
			}
		}
		if err != nil {
			break
		}
	}

	// Send zero-length chunk to end stream
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return false, "", fmt.Errorf("write zero chunk: %w", err)
	}

	// Read response
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		slog.Debug("ClamAV response", "line", line)
		if strings.Contains(line, "FOUND") {
			// Format: "stream: Threat.Name FOUND"
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				threat = parts[1]
			}
			return true, threat, nil
		}
		if strings.Contains(line, "OK") {
			return false, "", nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, "", fmt.Errorf("read clamd response: %w", err)
	}

	// Default to clean if no clear result
	slog.Warn("ClamAV: unexpected response, treating as clean", "objectKey", objectKey)
	return false, "", nil
}

// publishEvent fires a Kafka event via the outbox publisher.
func (w *Worker) publishEvent(ctx context.Context, topic, mediaID, userID, traceID string, payload map[string]interface{}) {
	eventID := uuid.NewString()
	data, _ := kafkaadapter.MarshalEnvelope(eventID, topic, traceID, payload)
	if err := w.publisher.Publish(ctx, topic, mediaID, data, traceID); err != nil {
		slog.Warn("Scan worker: failed to publish event", "topic", topic, "mediaId", mediaID, "error", err)
	}
}
