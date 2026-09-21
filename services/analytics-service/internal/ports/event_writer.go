package ports

import (
	"context"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

type ClickHouseWriter interface {
	WriteRawEvents(ctx context.Context, rows []*domain.RawEventLanding) error
	WriteDeliveryEvents(ctx context.Context, rows []*domain.FactDeliveryEvent) error
	WriteDeliveryCompleted(ctx context.Context, rows []*domain.FactDeliveryCompleted) error
	WriteDriverAssignments(ctx context.Context, rows []*domain.FactDriverAssignment) error
	WritePaymentTransactions(ctx context.Context, rows []*domain.FactPaymentTransaction) error
	WriteNotificationEvents(ctx context.Context, rows []*domain.FactNotificationEvent) error
	WriteDataQualityIssues(ctx context.Context, rows []*domain.DataQualityIssue) error
	Close() error
}
