package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	maxEmbedsPerMessage = 10
	maxFieldsPerEmbed   = 25
	maxFieldValueLen    = 1024
	maxTitleLen         = 256
	maxDescLen          = 4096
	maxContentLen       = 2000

	colorCritical = 0xE74C3C
	colorWarning  = 0xE67E22
	colorInfo     = 0x3498DB
	colorResolved = 0x2ECC71
)

type amWebhook struct {
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"`
	Alerts            []amAlert         `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
}

type amAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       *time.Time        `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

type discordMessage struct {
	Username string         `json:"username,omitempty"`
	Content  string         `json:"content,omitempty"`
	Embeds   []discordEmbed `json:"embeds,omitempty"`
}

type discordEmbed struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	URL         string         `json:"url,omitempty"`
	Color       int            `json:"color,omitempty"`
	Fields      []discordField `json:"fields,omitempty"`
	Footer      *discordFooter `json:"footer,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type discordFooter struct {
	Text string `json:"text"`
}

var (
	webhookURL string
	username   string
	client     = &http.Client{Timeout: 10 * time.Second}
)

func main() {
	webhookURL = os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		slog.Error("DISCORD_WEBHOOK_URL is required")
		os.Exit(1)
	}
	username = os.Getenv("DISCORD_USERNAME")
	if username == "" {
		username = "Delivery Alertmanager"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP","service":"alertmanager-discord"}`))
	})
	mux.HandleFunc("POST /webhook", handleWebhook)

	slog.Info("alertmanager-discord relay listening", "port", port, "version", "url-fix-v1")
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		slog.Error("read body", "error", err)
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	var payload amWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("decode payload", "error", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if len(payload.Alerts) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for _, msg := range buildMessages(&payload) {
		if err := sendDiscord(msg); err != nil {
			slog.Error("discord delivery failed", "error", err, "alerts", len(payload.Alerts))
			http.Error(w, "discord delivery failed", http.StatusBadGateway)
			return
		}
	}
	slog.Info("forwarded alerts to discord",
		"count", len(payload.Alerts),
		"status", payload.Status,
		"receiver", payload.Receiver,
	)
	w.WriteHeader(http.StatusOK)
}

func buildMessages(p *amWebhook) []discordMessage {
	alerts := p.Alerts
	truncated := 0
	if len(alerts) > maxEmbedsPerMessage {
		truncated = len(alerts) - maxEmbedsPerMessage
		alerts = alerts[:maxEmbedsPerMessage]
	}

	embeds := make([]discordEmbed, 0, len(alerts))
	for _, a := range alerts {
		embeds = append(embeds, buildEmbed(a, p.Receiver))
	}

	content := ""
	switch {
	case truncated > 0:
		content = fmt.Sprintf("â€¦and %d more alert(s) not shown", truncated)
	case len(alerts) > 1:
		content = fmt.Sprintf("%d alerts grouped by Alertmanager", len(alerts))
	}

	return []discordMessage{{
		Username: username,
		Content:  truncate(content, maxContentLen),
		Embeds:   embeds,
	}}
}

func buildEmbed(a amAlert, receiver string) discordEmbed {
	severity := a.Labels["severity"]
	alertname := a.Labels["alertname"]
	if alertname == "" {
		alertname = "UnnamedAlert"
	}

	state := strings.ToUpper(a.Status)
	if state == "" {
		state = "FIRING"
	}

	embed := discordEmbed{
		Title: truncate(fmt.Sprintf("%s: %s", state, alertname), maxTitleLen),
		Color: colorFor(a.Status, severity),
		Footer: &discordFooter{
			Text: fmt.Sprintf("Alertmanager Â· %s", receiver),
		},
	}
	// Discord rejects embed URLs whose hostname is a single label without dots
	// (e.g. http://alertmanager-srv:9093/...) with 400 {"embeds":["0"]}, so only
	// include the generator URL when the host looks like a valid public name.
	if u := safeEmbedURL(a.GeneratorURL); u != "" {
		embed.URL = u
	}

	if !a.StartsAt.IsZero() {
		if a.Status == "resolved" && a.EndsAt != nil {
			embed.Timestamp = a.EndsAt.UTC().Format(time.RFC3339)
		} else {
			embed.Timestamp = a.StartsAt.UTC().Format(time.RFC3339)
		}
	}

	var descParts []string
	if d := a.Annotations["description"]; d != "" {
		descParts = append(descParts, d)
	}
	if s := a.Annotations["summary"]; s != "" && !strings.Contains(strings.Join(descParts, "\n"), s) {
		descParts = append(descParts, s)
	}
	embed.Description = truncate(strings.Join(descParts, "\n\n"), maxDescLen)

	addField := func(name, value string, inline bool) {
		if value == "" || len(embed.Fields) >= maxFieldsPerEmbed {
			return
		}
		embed.Fields = append(embed.Fields, discordField{
			Name:   truncate(name, maxTitleLen),
			Value:  truncate(value, maxFieldValueLen),
			Inline: inline,
		})
	}

	addField("Severity", severity, true)
	addField("Service", firstOf(a.Labels["service"], a.Labels["job"]), true)
	addField("Instance", a.Labels["instance"], true)
	if a.Status == "resolved" && a.EndsAt != nil {
		addField("Resolved", a.EndsAt.UTC().Format(time.RFC3339), true)
	} else if !a.StartsAt.IsZero() {
		addField("Started", a.StartsAt.UTC().Format(time.RFC3339), true)
	}

	return embed
}

func sendDiscord(msg discordMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("discord returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		if resp.StatusCode == http.StatusTooManyRequests {
			if wait := parseRetryAfter(respBody); wait > 0 {
				time.Sleep(wait)
			}
			continue
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return lastErr
		}
	}
	return lastErr
}

func parseRetryAfter(body []byte) time.Duration {
	var rl struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.Unmarshal(body, &rl); err != nil || rl.RetryAfter <= 0 {
		return 0
	}
	return time.Duration(rl.RetryAfter * float64(time.Second))
}

func colorFor(status, severity string) int {
	if status == "resolved" {
		return colorResolved
	}
	switch severity {
	case "critical":
		return colorCritical
	case "warning":
		return colorWarning
	default:
		return colorInfo
	}
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "â€¦"
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// safeEmbedURL returns raw only when its hostname is acceptable to Discord's
// embed URL validation (dotted host, localhost, or an IP literal), otherwise "".
func safeEmbedURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	host := u.Hostname()
	if host == "" {
		return ""
	}
	if host == "localhost" || strings.Contains(host, ".") {
		return raw
	}
	if net.ParseIP(host) != nil {
		return raw
	}
	return ""
}
