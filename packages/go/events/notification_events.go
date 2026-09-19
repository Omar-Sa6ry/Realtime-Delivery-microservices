package events

import "time"

// NotificationCreatedPayload is emitted when a notification is created.
type NotificationCreatedPayload struct {
	NotificationID string    `json:"notificationId"`
	RecipientType  string    `json:"recipientType"` // email, push, sms, in_app
	RecipientValue string    `json:"recipientValue"`
	Channel        string    `json:"channel"`
	TemplateID     string    `json:"templateId,omitempty"`
	TemplateData   interface{} `json:"templateData,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// NotificationSentPayload is emitted when a notification is sent.
type NotificationSentPayload struct {
	NotificationID string    `json:"notificationId"`
	SentAt         time.Time `json:"sentAt"`
}

// NotificationDeliveredPayload is emitted when a notification is delivered.
type NotificationDeliveredPayload struct {
	NotificationID string    `json:"notificationId"`
	DeliveredAt    time.Time `json:"deliveredAt"`
	Channel        string    `json:"channel"`
}

// NotificationFailedPayload is emitted when a notification fails.
type NotificationFailedPayload struct {
	NotificationID string    `json:"notificationId"`
	Error          string    `json:"error"`
	FailedAt       time.Time `json:"failedAt"`
	RetryCount     int       `json:"retryCount"`
}

// NotificationRetryingPayload is emitted when a notification is being retried.
type NotificationRetryingPayload struct {
	NotificationID string    `json:"notificationId"`
	Attempt        int       `json:"attempt"`
	RetryAt        time.Time `json:"retryAt"`
}