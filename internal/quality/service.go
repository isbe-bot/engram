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

type Store interface {
	EventCount(ctx context.Context) (int, error)
	MemoryObjectCount(ctx context.Context) (int, error)
	MemoryStatusCounts(ctx context.Context) (map[string]int, error)
	MemoryAuditActionCounts(ctx context.Context) (map[string]int, error)
	LatestIngestedEventAt(ctx context.Context) (string, bool, error)
	LatestMemoryUpdatedAt(ctx context.Context) (string, bool, error)
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
