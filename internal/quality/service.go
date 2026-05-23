package quality

import (
	"context"
	"time"
)

// Snapshot captures local memory-platform quality and observability metrics.
type Snapshot struct {
	GeneratedAt             string         `json:"generated_at"`
	EventCount              int            `json:"event_count"`
	MemoryObjectCount       int            `json:"memory_object_count"`
	MemoryStatusCounts      map[string]int `json:"memory_status_counts"`
	AuditActionCounts       map[string]int `json:"audit_action_counts"`
	CorrectionCount         int            `json:"correction_count"`
	DeprecationCount        int            `json:"deprecation_count"`
	CorrectionRate          float64        `json:"correction_rate"`
	DeprecatedMemoryRate    float64        `json:"deprecated_memory_rate"`
	LatestIngestedEventAt   string         `json:"latest_ingested_event_at,omitempty"`
	LatestMemoryUpdatedAt   string         `json:"latest_memory_updated_at,omitempty"`
	IngestionLagSeconds     *int64         `json:"ingestion_lag_seconds,omitempty"`
	IngestionFreshnessState string         `json:"ingestion_freshness_state"`
}

type SLOMetric struct {
	Name   string   `json:"name"`
	Value  *float64 `json:"value,omitempty"`
	Target *float64 `json:"target,omitempty"`
	Unit   string   `json:"unit"`
	State  string   `json:"state"`
}

type Report struct {
	GeneratedAt string      `json:"generated_at"`
	WindowHours int         `json:"window_hours"`
	Summary     Snapshot    `json:"summary"`
	SLO         []SLOMetric `json:"slo"`
}

type Store interface {
	EventCount(ctx context.Context) (int, error)
	MemoryObjectCount(ctx context.Context) (int, error)
	MemoryStatusCounts(ctx context.Context) (map[string]int, error)
	MemoryAuditActionCounts(ctx context.Context) (map[string]int, error)
	LatestIngestedEventAt(ctx context.Context) (string, bool, error)
	LatestMemoryUpdatedAt(ctx context.Context) (string, bool, error)
}

type latencyReader interface {
	LatencyP95MS(ctx context.Context, operation string, since time.Time) (float64, bool, error)
}

type correctionLatencyReader interface {
	CorrectionApplyLatencyP95MS(ctx context.Context, since time.Time) (float64, bool, error)
}

type staleRateReader interface {
	StaleMemoryRate(ctx context.Context, staleBefore time.Time) (float64, bool, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Metrics(ctx context.Context) (Snapshot, error) {
	now := time.Now().UTC()
	if s != nil && s.now != nil {
		now = s.now().UTC()
	}

	snap := Snapshot{
		GeneratedAt:             now.Format(time.RFC3339),
		MemoryStatusCounts:      map[string]int{},
		AuditActionCounts:       map[string]int{},
		IngestionFreshnessState: "unknown",
	}
	if s == nil || s.store == nil {
		return snap, nil
	}

	var err error
	if snap.EventCount, err = s.store.EventCount(ctx); err != nil {
		return Snapshot{}, err
	}
	if snap.MemoryObjectCount, err = s.store.MemoryObjectCount(ctx); err != nil {
		return Snapshot{}, err
	}
	if snap.MemoryStatusCounts, err = s.store.MemoryStatusCounts(ctx); err != nil {
		return Snapshot{}, err
	}
	if snap.AuditActionCounts, err = s.store.MemoryAuditActionCounts(ctx); err != nil {
		return Snapshot{}, err
	}

	snap.CorrectionCount = snap.AuditActionCounts["corrected"]
	snap.DeprecationCount = snap.AuditActionCounts["deprecated"]
	if snap.MemoryObjectCount > 0 {
		snap.CorrectionRate = float64(snap.CorrectionCount) / float64(snap.MemoryObjectCount)
		snap.DeprecatedMemoryRate = float64(snap.MemoryStatusCounts["deprecated"]) / float64(snap.MemoryObjectCount)
	}

	if latest, ok, err := s.store.LatestIngestedEventAt(ctx); err != nil {
		return Snapshot{}, err
	} else if ok {
		snap.LatestIngestedEventAt = latest
		if parsed, parseErr := time.Parse(time.RFC3339, latest); parseErr == nil {
			lag := int64(now.Sub(parsed).Seconds())
			if lag < 0 {
				lag = 0
			}
			snap.IngestionLagSeconds = &lag
			snap.IngestionFreshnessState = freshnessState(lag)
		}
	}
	if latest, ok, err := s.store.LatestMemoryUpdatedAt(ctx); err != nil {
		return Snapshot{}, err
	} else if ok {
		snap.LatestMemoryUpdatedAt = latest
	}

	return snap, nil
}

func (s *Service) Report(ctx context.Context) (Report, error) {
	now := time.Now().UTC()
	if s != nil && s.now != nil {
		now = s.now().UTC()
	}
	summary, err := s.Metrics(ctx)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		GeneratedAt: now.Format(time.RFC3339),
		WindowHours: 24,
		Summary:     summary,
		SLO:         make([]SLOMetric, 0, 5),
	}

	var ingestLag *float64
	if summary.IngestionLagSeconds != nil {
		v := float64(*summary.IngestionLagSeconds)
		ingestLag = &v
	}
	report.SLO = append(report.SLO, buildSLO("ingest_lag_p95", ingestLag, float64Ptr(30), "seconds"))

	latReader, ok := s.store.(latencyReader)
	if ok {
		if p95, found, err := latReader.LatencyP95MS(ctx, "retrieve.search", now.Add(-24*time.Hour)); err != nil {
			return Report{}, err
		} else {
			report.SLO = append(report.SLO, buildSLO("retrieval_p95", valuePtr(found, p95), float64Ptr(250), "ms"))
		}
	} else {
		report.SLO = append(report.SLO, buildSLO("retrieval_p95", nil, float64Ptr(250), "ms"))
	}

	if summary.LatestMemoryUpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, summary.LatestMemoryUpdatedAt); err == nil {
			v := now.Sub(t).Seconds()
			report.SLO = append(report.SLO, buildSLO("consolidation_freshness", float64Ptr(v), float64Ptr(3600), "seconds"))
		} else {
			report.SLO = append(report.SLO, buildSLO("consolidation_freshness", nil, float64Ptr(3600), "seconds"))
		}
	} else {
		report.SLO = append(report.SLO, buildSLO("consolidation_freshness", nil, float64Ptr(3600), "seconds"))
	}

	corrReader, ok := s.store.(correctionLatencyReader)
	if ok {
		if p95, found, err := corrReader.CorrectionApplyLatencyP95MS(ctx, now.Add(-30*24*time.Hour)); err != nil {
			return Report{}, err
		} else {
			report.SLO = append(report.SLO, buildSLO("correction_apply_latency_p95", valuePtr(found, p95), float64Ptr(86400000), "ms"))
		}
	} else {
		report.SLO = append(report.SLO, buildSLO("correction_apply_latency_p95", nil, float64Ptr(86400000), "ms"))
	}

	staleReader, ok := s.store.(staleRateReader)
	if ok {
		if rate, found, err := staleReader.StaleMemoryRate(ctx, now.Add(-30*24*time.Hour)); err != nil {
			return Report{}, err
		} else {
			report.SLO = append(report.SLO, buildSLO("stale_memory_rate", valuePtr(found, rate), float64Ptr(0.2), "ratio"))
		}
	} else {
		report.SLO = append(report.SLO, buildSLO("stale_memory_rate", nil, float64Ptr(0.2), "ratio"))
	}

	return report, nil
}

func buildSLO(name string, value *float64, target *float64, unit string) SLOMetric {
	metric := SLOMetric{Name: name, Value: value, Target: target, Unit: unit, State: "unknown"}
	if value == nil || target == nil {
		return metric
	}
	if *value <= *target {
		metric.State = "within_slo"
	} else if *value <= (*target * 1.5) {
		metric.State = "watch"
	} else {
		metric.State = "breach"
	}
	return metric
}

func valuePtr(ok bool, v float64) *float64 {
	if !ok {
		return nil
	}
	return &v
}

func float64Ptr(v float64) *float64 { return &v }

func freshnessState(lagSeconds int64) string {
	switch {
	case lagSeconds <= 30:
		return "within_slo"
	case lagSeconds <= 300:
		return "watch"
	default:
		return "stale"
	}
}
