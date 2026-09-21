package reconciliation

import (
	"context"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

const (
	DefaultFreshWarnSeconds     = 60
	DefaultFreshDegradedSeconds = 300
)

type FreshnessStatus string

const (
	FreshnessHealthy  FreshnessStatus = "healthy"
	FreshnessWarning  FreshnessStatus = "warning"
	FreshnessDegraded FreshnessStatus = "degraded"
)

type Report struct {
	CheckedAt        time.Time
	WindowFrom       time.Time
	WindowTo         time.Time
	FreshnessSeconds float64
	Freshness        FreshnessStatus
	RawTotal         int64
	FactTotal        int64
	CoverageRatio    float64
	GapDetected      bool
	DuplicateCount   int64
	UnknownTypes     []string
	OpenIssues       map[string]int64
}

type Config struct {
	WindowHours          int
	FreshWarnSeconds     int64
	FreshDegradedSeconds int64
	MinCoverageRatio     float64
}

type Service struct {
	queries ports.ReconciliationQueries
	cfg     Config
}

func NewService(queries ports.ReconciliationQueries, cfg Config) *Service {
	if cfg.WindowHours <= 0 {
		cfg.WindowHours = 1
	}
	if cfg.FreshWarnSeconds <= 0 {
		cfg.FreshWarnSeconds = DefaultFreshWarnSeconds
	}
	if cfg.FreshDegradedSeconds <= 0 {
		cfg.FreshDegradedSeconds = DefaultFreshDegradedSeconds
	}
	if cfg.MinCoverageRatio <= 0 {
		cfg.MinCoverageRatio = 0.99
	}
	return &Service{queries: queries, cfg: cfg}
}

func (s *Service) Check(ctx context.Context) (*Report, error) {
	now := time.Now().UTC()
	to := now
	from := now.Add(-time.Duration(s.cfg.WindowHours) * time.Hour)
	rep := &Report{CheckedAt: now, WindowFrom: from, WindowTo: to}

	maxOccurred, err := s.queries.MaxRawOccurredAt(ctx)
	if err != nil {
		return nil, fmt.Errorf("reconcile freshness: %w", err)
	}
	if !maxOccurred.IsZero() {
		rep.FreshnessSeconds = now.Sub(maxOccurred).Seconds()
		rep.Freshness = gradeFreshness(rep.FreshnessSeconds, s.cfg.FreshWarnSeconds, s.cfg.FreshDegradedSeconds)
	} else {
		rep.Freshness = FreshnessHealthy
		rep.FreshnessSeconds = 0
	}

	if rep.RawTotal, err = s.queries.CountRawEvents(ctx, from, to); err != nil {
		return nil, fmt.Errorf("reconcile raw count: %w", err)
	}
	if rep.FactTotal, err = s.queries.CountFactDeliveryEvents(ctx, from, to); err != nil {
		return nil, fmt.Errorf("reconcile fact count: %w", err)
	}
	if rep.RawTotal > 0 {
		rep.CoverageRatio = float64(rep.FactTotal) / float64(rep.RawTotal)
		rep.GapDetected = rep.CoverageRatio < s.cfg.MinCoverageRatio
	}

	if rep.DuplicateCount, err = s.queries.CountDataQualityByType(ctx, domain.IssueDuplicateEvent, from, to); err != nil {
		return nil, fmt.Errorf("reconcile duplicates: %w", err)
	}

	observed, err := s.queries.DistinctRawEventTypes(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("reconcile event types: %w", err)
	}
	for _, t := range observed {
		if !isKnownEventType(t) {
			rep.UnknownTypes = append(rep.UnknownTypes, t)
		}
	}

	if rep.OpenIssues, err = s.queries.CountOpenIssuesBySeverity(ctx); err != nil {
		return nil, fmt.Errorf("reconcile open issues: %w", err)
	}
	return rep, nil
}

func gradeFreshness(seconds float64, warn, degraded int64) FreshnessStatus {
	switch {
	case seconds >= float64(degraded):
		return FreshnessDegraded
	case seconds >= float64(warn):
		return FreshnessWarning
	default:
		return FreshnessHealthy
	}
}

var knownEventTypes = map[string]bool{
	"delivery.created": true, "delivery.driver.assigned": true, "delivery.driver.accepted": true,
	"delivery.pickup.started": true, "delivery.picked_up": true, "delivery.in_transit": true,
	"delivery.completed": true, "delivery.cancelled": true, "delivery.failed": true,
	"driver.available": true, "driver.unavailable": true,
	"driver.assignment.offered": true, "driver.assignment.accepted": true, "driver.assignment.rejected": true,
	"driver.assignment.expired": true, "driver.assignment.released": true,
	"payment.created": true, "payment.authorization.started": true, "payment.authorized": true,
	"payment.authorization.failed": true, "payment.capture.started": true, "payment.captured": true,
	"payment.capture.failed": true, "payment.cancelled": true, "payment.refund.started": true,
	"payment.refunded": true, "payment.refund.failed": true, "payment.failed": true,
	"notification.created": true, "notification.sent": true, "notification.delivered": true,
	"notification.failed": true, "notification.retrying": true,
}

func isKnownEventType(t string) bool {
	return knownEventTypes[t]
}
