package automation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// WebhookPayload represents the standard webhook message format for Discord/Slack
type WebhookPayload struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

// TriggerAlert sends an automated alert message to Discord/Slack webhook configured in ALERT_WEBHOOK_URL
func TriggerAlert(title, message, severity string) (bool, error) {
	if severity == "" {
		severity = "WARNING"
	}

	webhookURL := os.Getenv("ALERT_WEBHOOK_URL")
	if webhookURL == "" {
		slog.Warn("Alert triggered but no ALERT_WEBHOOK_URL is configured",
			"severity", severity,
			"title", title,
			"message", message,
		)
		return false, fmt.Errorf("ALERT_WEBHOOK_URL is not set")
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	content := fmt.Sprintf("**[%s] %s**\n%s\nTimestamp: %s", severity, title, message, timestamp)

	payload := WebhookPayload{
		Username: "System Alert Bot",
		Content:  content,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal alert webhook payload: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return false, fmt.Errorf("failed to create webhook HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to dispatch automated webhook alert", "title", title, "error", err)
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("webhook responded with status: %d", resp.StatusCode)
	}

	slog.Info("Alert notification dispatched successfully", "title", title)
	return true, nil
}
