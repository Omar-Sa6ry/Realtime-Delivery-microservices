package observability

import (
	"log/slog"

	pkglogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
)

func Init() *slog.Logger {
	return pkglogging.InitLogger()
}

func EventAttrs(service, eventType, eventID, aggregateID, topic string, partition int32, offset int64, correlationID, traceID string) []any {
	return []any{
		"service", service,
		"eventType", eventType,
		"eventId", eventID,
		"aggregateId", aggregateID,
		"topic", topic,
		"partition", partition,
		"offset", offset,
		"correlationId", correlationID,
		"traceId", traceID,
	}
}
