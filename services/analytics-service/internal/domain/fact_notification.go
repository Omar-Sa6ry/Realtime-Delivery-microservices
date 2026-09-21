package domain

import (
	"fmt"
	"strings"
	"time"
)

type NotificationChannel string

const (
	NotificationChannelEmail NotificationChannel = "email"
	NotificationChannelPush  NotificationChannel = "push"
	NotificationChannelSMS   NotificationChannel = "sms"
	NotificationChannelInApp NotificationChannel = "in_app"
)

func (c NotificationChannel) IsValid() bool {
	switch c {
	case NotificationChannelEmail, NotificationChannelPush, NotificationChannelSMS, NotificationChannelInApp:
		return true
	default:
		return false
	}
}

type NotificationStatus string

const (
	NotificationStatusCreated   NotificationStatus = "created"
	NotificationStatusSent      NotificationStatus = "sent"
	NotificationStatusDelivered NotificationStatus = "delivered"
	NotificationStatusFailed    NotificationStatus = "failed"
	NotificationStatusRetrying  NotificationStatus = "retrying"
)

func (s NotificationStatus) IsValid() bool {
	switch s {
	case NotificationStatusCreated, NotificationStatusSent, NotificationStatusDelivered,
		NotificationStatusFailed, NotificationStatusRetrying:
		return true
	default:
		return false
	}
}

type FactNotificationEvent struct {
	EventID        string
	NotificationID string
	RecipientType  string
	Channel        NotificationChannel
	Status         NotificationStatus
	TemplateID     string
	SentAt         *time.Time
	DeliveredAt    *time.Time
	FailedAt       *time.Time
	RetryCount     int
	Error          string
	OccurredAt     time.Time
	IngestedAt     time.Time
}

func (f *FactNotificationEvent) Validate() error {
	if f == nil {
		return ErrInvalidEnvelope
	}
	if strings.TrimSpace(f.EventID) == "" {
		return ErrMissingEventID
	}
	if strings.TrimSpace(f.NotificationID) == "" {
		return ErrMissingNotificationID
	}
	if !f.Channel.IsValid() {
		return fmt.Errorf("%w: channel %s", ErrUnknownEventType, f.Channel)
	}
	if !f.Status.IsValid() {
		return fmt.Errorf("%w: status %s", ErrUnknownEventType, f.Status)
	}
	if f.OccurredAt.IsZero() {
		return ErrInvalidOccurredAt
	}
	if f.RetryCount < 0 {
		return ErrInvalidRetryCount
	}
	return nil
}
