package workers

import (
	"context"
	"log/slog"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/reconciliation"
)


type ReconciliationWorker struct {
	service *reconciliation.Service
}

func NewReconciliationWorker(service *reconciliation.Service) *ReconciliationWorker {
	return &ReconciliationWorker{service: service}
}

func (w *ReconciliationWorker) Run(ctx context.Context) {
	rep, err := w.service.Check(ctx)
	if err != nil {
		slog.Error("reconciliation pass failed", "error", err)
		return
	}
	attrs := []any{
		"freshness_seconds", rep.FreshnessSeconds,
		"freshness", string(rep.Freshness),
		"raw_total", rep.RawTotal,
		"fact_total", rep.FactTotal,
		"coverage_ratio", rep.CoverageRatio,
		"gap_detected", rep.GapDetected,
		"duplicates", rep.DuplicateCount,
		"open_issues", rep.OpenIssues,
	}
	if len(rep.UnknownTypes) > 0 {
		attrs = append(attrs, "unknown_types", rep.UnknownTypes)
	}
	switch {
	case rep.GapDetected || rep.Freshness == reconciliation.FreshnessDegraded:
		slog.Error("reconciliation pass: action required", attrs...)
	case rep.Freshness == reconciliation.FreshnessWarning || len(rep.UnknownTypes) > 0:
		slog.Warn("reconciliation pass: attention", attrs...)
	default:
		slog.Info("reconciliation pass: healthy", attrs...)
	}
}
