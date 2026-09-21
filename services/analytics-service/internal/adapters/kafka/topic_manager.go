package kafka

import (
	pkgkafka "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/kafka"
)

const (
	TopicDeliveryEvents     = "delivery.events"
	TopicDriverEvents       = "driver.events"
	TopicPaymentEvents      = "payment.events"
	TopicNotificationEvents = "notification.events"
)

const ConsumerGroup = "analytics-service"

func AnalyticsTopics() []string {
	return []string{
		TopicDeliveryEvents,
		TopicDriverEvents,
		TopicPaymentEvents,
		TopicNotificationEvents,
	}
}

func EnsureAnalyticsTopics(brokers []string, dlqTopic string, numPartitions, replicationFactor int) error {
	topics := append(AnalyticsTopics(), dlqTopic)
	return pkgkafka.EnsureTopics(brokers, topics, numPartitions, replicationFactor)
}
