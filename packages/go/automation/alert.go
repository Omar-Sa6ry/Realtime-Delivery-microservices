package automation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

type AlertmanagerAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    string            `json:"startsAt"`
}

const (
	alertRequestTimeout = 5 * time.Second
	defaultAlertService = "application"
)

func normalizeSeverity(severity string) string {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "CRITICAL", "ERROR", "FATAL", "EMERG", "ALERT":
		return "critical"
	case "INFO", "DEBUG", "NOTICE":
		return "info"
	case "WARNING", "WARN", "":
		return "warning"
	default:
		return "warning"
	}
}

func TriggerAlert(service, title, message, severity string) (bool, error) {
	if service == "" {
		service = defaultAlertService
	}

	alertmanagerURL := strings.TrimRight(os.Getenv("ALERTMANAGER_URL"), "/")
	if alertmanagerURL == "" {
		slog.Warn("Alert triggered but no ALERTMANAGER_URL is configured",
			"severity", severity,
			"title", title,
			"message", message,
		)
		return false, fmt.Errorf("ALERTMANAGER_URL is not set")
	}

	payload := []AlertmanagerAlert{
		{
			Labels: map[string]string{
				"alertname": "ApplicationAlert",
				"severity":  normalizeSeverity(severity),
				"source":    "application",
				"service":   service,
				"title":     title,
			},
			Annotations: map[string]string{
				"summary":     title,
				"description": message,
			},
			StartsAt: time.Now().UTC().Format(time.RFC3339),
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal alert payload: %w", err)
	}

	client := &http.Client{
		Timeout: alertRequestTimeout,
	}

	req, err := http.NewRequest(http.MethodPost, alertmanagerURL+"/api/v2/alerts", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return false, fmt.Errorf("failed to create alert HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to dispatch automated alert", "title", title, "error", err)
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("alertmanager responded with status: %d", resp.StatusCode)
	}

	slog.Info("Alert dispatched to Alertmanager", "title", title, "service", service)
	return true, nil
}
