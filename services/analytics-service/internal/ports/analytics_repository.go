package ports

import (
	"context"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
)

type AnalyticsGranularity string

const (
	GranularityHour  AnalyticsGranularity = "HOUR"
	GranularityDay   AnalyticsGranularity = "DAY"
	GranularityWeek  AnalyticsGranularity = "WEEK"
	GranularityMonth AnalyticsGranularity = "MONTH"
)

func (g AnalyticsGranularity) IsValid() bool {
	switch g {
	case GranularityHour, GranularityDay, GranularityWeek, GranularityMonth:
		return true
	default:
		return false
	}
}

const MaxQueryRange = 90 * 24 * time.Hour

type TimeRange struct {
	From        time.Time
	To          time.Time
	Granularity AnalyticsGranularity
}

func (r TimeRange) Validate() error {
	if r.From.IsZero() || r.To.IsZero() {
		return domain.ErrInvalidOccurredAt
	}
	if r.To.Before(r.From) {
		return domain.ErrOutOfOrderTimestamps
	}
	if r.To.Sub(r.From) > MaxQueryRange {
		return fmt.Errorf("range exceeds maximum of %s", MaxQueryRange)
	}
	if !r.Granularity.IsValid() {
		return fmt.Errorf("invalid granularity: %s", r.Granularity)
	}
	return nil
}

type DeliveryAnalyticsFilter struct {
	Range    TimeRange
	CityID   string
	DriverID string
}

type DriverAnalyticsFilter struct {
	Range    TimeRange
	DriverID string
}
type PaymentAnalyticsFilter struct {
	Range    TimeRange
	Provider string
}

type Pagination struct {
	TotalItems  int
	CurrentPage int
	NextPage    *int
}

type PlatformOverview struct {
	TotalDeliveries          int64
	CompletedDeliveries      int64
	CancelledDeliveries      int64
	FailedDeliveries         int64
	CompletionRate           float64
	AverageDeliveryDurationS float64
	PaymentCapturedAmount    string
	RefundedAmount           string
	DriverAcceptanceRate     float64
	DataAsOf                 time.Time
}

type DeliveryMetricBucket struct {
	Bucket             time.Time
	Total              int64
	Completed          int64
	Cancelled          int64
	Failed             int64
	AvgDurationSeconds float64
}

type DeliveryAnalytics struct {
	Total                  int64
	Completed              int64
	Cancelled              int64
	Failed                 int64
	CompletionRate         float64
	AverageDurationSeconds float64
	P50DurationSeconds     float64
	P95DurationSeconds     float64
	P99DurationSeconds     float64
	AverageAssignmentTimeS float64
	Buckets                []DeliveryMetricBucket
	DataAsOf               time.Time
}

type DriverAnalytics struct {
	DriverID              string
	Offers                int64
	Accepted              int64
	Rejected              int64
	Expired               int64
	AcceptanceRate        float64
	AverageResponseTimeMs float64
	CompletedDeliveries   int64
	DataAsOf              time.Time
}

type DriverAnalyticsPage struct {
	Pagination Pagination
	Items      []DriverAnalytics
}

type PaymentAnalytics struct {
	AuthorizationCount       int64
	AuthorizationSuccessRate float64
	CaptureCount             int64
	CapturedAmount           string
	RefundCount              int64
	RefundedAmount           string
	RefundRate               float64
	AverageProviderLatencyMs float64
	DataAsOf                 time.Time
}

type RawEventsPage struct {
	Pagination Pagination
	Items      []*domain.RawEventLanding
}

type DataQualityIssuesPage struct {
	Pagination Pagination
	Items      []*domain.DataQualityIssue
}

type AnalyticsQueryRepository interface {
	PlatformOverview(ctx context.Context, r TimeRange) (*PlatformOverview, error)
	DeliveryAnalytics(ctx context.Context, f DeliveryAnalyticsFilter) (*DeliveryAnalytics, error)
	DriverAnalytics(ctx context.Context, f DriverAnalyticsFilter) (*DriverAnalytics, error)
	TopDrivers(ctx context.Context, r TimeRange, limit int) (*DriverAnalyticsPage, error)
	PaymentAnalytics(ctx context.Context, f PaymentAnalyticsFilter) (*PaymentAnalytics, error)
	RawEvents(ctx context.Context, page, limit int, eventType string, from, to *time.Time) (*RawEventsPage, error)
	DataQualityIssues(ctx context.Context, page, limit int, severity string) (*DataQualityIssuesPage, error)
	Close() error
}
