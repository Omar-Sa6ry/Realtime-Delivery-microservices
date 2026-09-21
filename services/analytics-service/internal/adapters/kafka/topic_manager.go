package kafka

import (
	pkgkafka "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/kafka"
)

// Delivery domain topics — one topic per event type, mirroring DeliveryKafkaTopics in the TS package.
const (
	TopicDeliveryCreated        = "delivery.created"
	TopicDeliveryDriverAssigned = "delivery.driver.assigned"
	TopicDeliveryDriverAccepted = "delivery.driver.accepted"
	TopicDeliveryPickupStarted  = "delivery.pickup.started"
	TopicDeliveryPickedUp       = "delivery.picked_up"
	TopicDeliveryInTransit      = "delivery.in_transit"
	TopicDeliveryCompleted      = "delivery.completed"
	TopicDeliveryCancelled      = "delivery.cancelled"
	TopicDeliveryFailed         = "delivery.failed"
	TopicDeliveryDeleted        = "delivery.deleted"
)

// Driver domain topics.
const (
	TopicDriverAvailable           = "driver.available"
	TopicDriverUnavailable         = "driver.unavailable"
	TopicDriverAssignmentOffered   = "driver.assignment.offered"
	TopicDriverAssignmentAccepted  = "driver.assignment.accepted"
	TopicDriverAssignmentRejected  = "driver.assignment.rejected"
	TopicDriverAssignmentExpired   = "driver.assignment.expired"
	TopicDriverAssignmentReleased  = "driver.assignment.released"
)

// Payment domain topics.
const (
	TopicPaymentCreated               = "payment.created"
	TopicPaymentAuthorizationStarted  = "payment.authorization.started"
	TopicPaymentAuthorized            = "payment.authorized"
	TopicPaymentAuthorizationFailed   = "payment.authorization.failed"
	TopicPaymentCaptureStarted        = "payment.capture.started"
	TopicPaymentCaptured              = "payment.captured"
	TopicPaymentCaptureFailed         = "payment.capture.failed"
	TopicPaymentCancelled             = "payment.cancelled"
	TopicPaymentRefundStarted         = "payment.refund.started"
	TopicPaymentRefunded              = "payment.refunded"
	TopicPaymentRefundFailed          = "payment.refund.failed"
	TopicPaymentFailed                = "payment.failed"
)

// Notification domain topics.
const (
	TopicNotificationCreated   = "notification.created"
	TopicNotificationSent      = "notification.sent"
	TopicNotificationDelivered = "notification.delivered"
	TopicNotificationFailed    = "notification.failed"
	TopicNotificationRetrying  = "notification.retrying"
)

const ConsumerGroup = "analytics-service"

// AnalyticsTopics returns all Kafka topics the analytics service consumes.
// These MUST mirror the topics published by the producer services exactly.
func AnalyticsTopics() []string {
	return []string{
		// Delivery
		TopicDeliveryCreated,
		TopicDeliveryDriverAssigned,
		TopicDeliveryDriverAccepted,
		TopicDeliveryPickupStarted,
		TopicDeliveryPickedUp,
		TopicDeliveryInTransit,
		TopicDeliveryCompleted,
		TopicDeliveryCancelled,
		TopicDeliveryFailed,
		// Driver
		TopicDriverAvailable,
		TopicDriverUnavailable,
		TopicDriverAssignmentOffered,
		TopicDriverAssignmentAccepted,
		TopicDriverAssignmentRejected,
		TopicDriverAssignmentExpired,
		TopicDriverAssignmentReleased,
		// Payment
		TopicPaymentCreated,
		TopicPaymentAuthorizationStarted,
		TopicPaymentAuthorized,
		TopicPaymentAuthorizationFailed,
		TopicPaymentCaptureStarted,
		TopicPaymentCaptured,
		TopicPaymentCaptureFailed,
		TopicPaymentCancelled,
		TopicPaymentRefundStarted,
		TopicPaymentRefunded,
		TopicPaymentRefundFailed,
		TopicPaymentFailed,
		// Notification
		TopicNotificationCreated,
		TopicNotificationSent,
		TopicNotificationDelivered,
		TopicNotificationFailed,
		TopicNotificationRetrying,
	}
}

func EnsureAnalyticsTopics(brokers []string, dlqTopic string, numPartitions, replicationFactor int) error {
	topics := append(AnalyticsTopics(), dlqTopic)
	return pkgkafka.EnsureTopics(brokers, topics, numPartitions, replicationFactor)
}
