package events

type AnalyticsEventType string

const (
	AnalyticsRawEvent       AnalyticsEventType = "analytics.raw_event"
	AnalyticsDataQuality    AnalyticsEventType = "analytics.data_quality"
	AnalyticsReplayStarted  AnalyticsEventType = "analytics.replay.started"
	AnalyticsReplayCompleted AnalyticsEventType = "analytics.replay.completed"
	AnalyticsDlq            AnalyticsEventType = "analytics.dlq"
)

type AnalyticsEventEnvelope = EventEnvelope

type AnalyticsDLQPayload struct {
	EventID      string    `json:"eventId"`
	EventType    string    `json:"eventType"`
	OccurredAt   string    `json:"occurredAt"`
	Producer     string    `json:"producer"`
	Error        string    `json:"error"`
	AttemptCount int       `json:"attemptCount"`
	FailedAt     string    `json:"failedAt"`
	CorrelationID string   `json:"correlationId,omitempty"`
}

type NotificationEventType string

const (
	NotificationCreated  NotificationEventType = "notification.created"
	NotificationSent     NotificationEventType = "notification.sent"
	NotificationDelivered NotificationEventType = "notification.delivered"
	NotificationFailed   NotificationEventType = "notification.failed"
	NotificationRetrying NotificationEventType = "notification.retrying"
)

type NotificationPayload struct {
	NotificationID string    `json:"notificationId"`
	RecipientType  string    `json:"recipientType"` // email, push, sms, in_app
	RecipientValue string    `json:"recipientValue"`
	Channel        string    `json:"channel"`
	TemplateID     string    `json:"templateId,omitempty"`
	TemplateData   interface{} `json:"templateData,omitempty"`
	SentAt         string    `json:"sentAt,omitempty"`
	DeliveredAt    string    `json:"deliveredAt,omitempty"`
}

type NotificationEventEnvelope = EventEnvelope