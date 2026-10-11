package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMReturnsTheSameInstance(t *testing.T) {
	if M() != M() {
		t.Fatal("M() must return a single shared instance (promauto panics on duplicate registration)")
	}
}

func TestRecordHelpersIncrementCounters(t *testing.T) {
	m := M()

	createdBefore := testutil.ToFloat64(m.PaymentsCreatedTotal.WithLabelValues("usd", "card"))
	m.RecordPaymentCreated("usd", "card")
	if got := testutil.ToFloat64(m.PaymentsCreatedTotal.WithLabelValues("usd", "card")); got != createdBefore+1 {
		t.Fatalf("payments_created_total = %v, want %v", got, createdBefore+1)
	}

	failedBefore := testutil.ToFloat64(m.PaymentsFailedTotal.WithLabelValues("usd", "card", "timeout"))
	m.RecordPaymentFailed("usd", "card", "timeout")
	if got := testutil.ToFloat64(m.PaymentsFailedTotal.WithLabelValues("usd", "card", "timeout")); got != failedBefore+1 {
		t.Fatalf("payments_failed_total = %v, want %v", got, failedBefore+1)
	}
}

func TestMCountersAreExportable(t *testing.T) {
	m := M()
	m.RecordPaymentCaptured("eur", "card")

	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "payments_captured_total" {
			found = true
		}
	}
	if !found {
		t.Fatal("payments_captured_total was not registered on the default registry")
	}
}
