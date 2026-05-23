package quality

import (
	"context"
	"testing"
	"time"
)

type fakeStore struct{}

func (fakeStore) EventCount(context.Context) (int, error)        { return 2, nil }
func (fakeStore) MemoryObjectCount(context.Context) (int, error) { return 4, nil }
func (fakeStore) MemoryStatusCounts(context.Context) (map[string]int, error) {
	return map[string]int{"accepted": 3, "deprecated": 1}, nil
}
func (fakeStore) MemoryAuditActionCounts(context.Context) (map[string]int, error) {
	return map[string]int{"curated": 4, "corrected": 2, "deprecated": 1}, nil
}
func (fakeStore) LatestIngestedEventAt(context.Context) (string, bool, error) {
	return "2026-05-23T03:59:40Z", true, nil
}
func (fakeStore) LatestMemoryUpdatedAt(context.Context) (string, bool, error) {
	return "2026-05-23T03:59:45Z", true, nil
}
func (fakeStore) LatencyP95MS(context.Context, string, time.Time) (float64, bool, error) {
	return 180, true, nil
}
func (fakeStore) CorrectionApplyLatencyP95MS(context.Context, time.Time) (float64, bool, error) {
	return 1000, true, nil
}
func (fakeStore) StaleMemoryRate(context.Context, time.Time) (float64, bool, error) {
	return 0.1, true, nil
}

func TestMetricsComputesRatesAndFreshness(t *testing.T) {
	svc := NewService(fakeStore{})
	svc.now = func() time.Time { return time.Date(2026, 5, 23, 4, 0, 0, 0, time.UTC) }

	metrics, err := svc.Metrics(context.Background())
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	if metrics.EventCount != 2 || metrics.MemoryObjectCount != 4 {
		t.Fatalf("unexpected counts: %+v", metrics)
	}
	if metrics.CorrectionRate != 0.5 {
		t.Fatalf("expected correction rate 0.5, got %v", metrics.CorrectionRate)
	}
	if metrics.DeprecatedMemoryRate != 0.25 {
		t.Fatalf("expected deprecated rate 0.25, got %v", metrics.DeprecatedMemoryRate)
	}
	if metrics.IngestionLagSeconds == nil || *metrics.IngestionLagSeconds != 20 {
		t.Fatalf("expected ingestion lag 20s, got %v", metrics.IngestionLagSeconds)
	}
	if metrics.IngestionFreshnessState != "within_slo" {
		t.Fatalf("expected within_slo, got %s", metrics.IngestionFreshnessState)
	}
}

func TestReportIncludesSLOValues(t *testing.T) {
	svc := NewService(fakeStore{})
	svc.now = func() time.Time { return time.Date(2026, 5, 23, 4, 0, 0, 0, time.UTC) }

	report, err := svc.Report(context.Background())
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if len(report.SLO) != 5 {
		t.Fatalf("expected 5 SLO metrics, got %d", len(report.SLO))
	}
	if report.SLO[1].Name != "retrieval_p95" || report.SLO[1].Value == nil || *report.SLO[1].Value != 180 {
		t.Fatalf("unexpected retrieval slo metric: %+v", report.SLO[1])
	}
	if report.SLO[4].Name != "stale_memory_rate" || report.SLO[4].State != "within_slo" {
		t.Fatalf("unexpected stale_memory_rate metric: %+v", report.SLO[4])
	}
}
