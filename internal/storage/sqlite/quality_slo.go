package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func (s *Store) RecordLatency(ctx context.Context, operation string, latency time.Duration) {
	_ = s.RecordLatencySample(ctx, operation, latency, time.Now().UTC())
}

func (s *Store) RecordLatencySample(ctx context.Context, operation string, latency time.Duration, now time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store is not initialized")
	}
	op := strings.TrimSpace(operation)
	if op == "" {
		return fmt.Errorf("operation is required")
	}
	latencyMs := float64(latency.Milliseconds())
	if latencyMs < 0 {
		latencyMs = 0
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO quality_latency_samples (operation, latency_ms, created_at)
		VALUES (?, ?, ?)
	`, op, latencyMs, now.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert latency sample: %w", err)
	}
	return nil
}

func (s *Store) LatencyP95MS(ctx context.Context, operation string, since time.Time) (float64, bool, error) {
	return s.latencyPercentileMS(ctx, operation, since, 0.95)
}

func (s *Store) CorrectionApplyLatencyP95MS(ctx context.Context, since time.Time) (float64, bool, error) {
	if s == nil || s.db == nil {
		return 0, false, fmt.Errorf("sqlite store is not initialized")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.created_at, r.created_at
		FROM memory_object_events c
		JOIN memory_object_events r ON r.object_id = c.object_id AND r.action = 'corrected'
		WHERE c.action = 'curated' AND c.created_at >= ?
		GROUP BY c.object_id
	`, since.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, false, fmt.Errorf("query correction latency samples: %w", err)
	}
	defer rows.Close()

	samples := make([]float64, 0)
	for rows.Next() {
		var curatedAt string
		var correctedAt string
		if err := rows.Scan(&curatedAt, &correctedAt); err != nil {
			return 0, false, fmt.Errorf("scan correction latency sample: %w", err)
		}
		curatedTime, err := time.Parse(time.RFC3339, curatedAt)
		if err != nil {
			continue
		}
		correctedTime, err := time.Parse(time.RFC3339, correctedAt)
		if err != nil {
			continue
		}
		if correctedTime.Before(curatedTime) {
			continue
		}
		samples = append(samples, float64(correctedTime.Sub(curatedTime).Milliseconds()))
	}
	if err := rows.Err(); err != nil {
		return 0, false, fmt.Errorf("iterate correction latency samples: %w", err)
	}
	if len(samples) == 0 {
		return 0, false, nil
	}
	sort.Float64s(samples)
	return percentileValue(samples, 0.95), true, nil
}

func (s *Store) StaleMemoryRate(ctx context.Context, staleBefore time.Time) (float64, bool, error) {
	if s == nil || s.db == nil {
		return 0, false, fmt.Errorf("sqlite store is not initialized")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM memory_objects WHERE status = 'accepted'`).Scan(&total); err != nil {
		return 0, false, fmt.Errorf("count accepted memory objects: %w", err)
	}
	if total == 0 {
		return 0, false, nil
	}
	var stale int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM memory_objects WHERE status = 'accepted' AND updated_at < ?`, staleBefore.UTC().Format(time.RFC3339)).Scan(&stale); err != nil {
		return 0, false, fmt.Errorf("count stale memory objects: %w", err)
	}
	return float64(stale) / float64(total), true, nil
}

func (s *Store) latencyPercentileMS(ctx context.Context, operation string, since time.Time, percentile float64) (float64, bool, error) {
	if s == nil || s.db == nil {
		return 0, false, fmt.Errorf("sqlite store is not initialized")
	}
	op := strings.TrimSpace(operation)
	if op == "" {
		return 0, false, fmt.Errorf("operation is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT latency_ms
		FROM quality_latency_samples
		WHERE operation = ? AND created_at >= ?
		ORDER BY latency_ms ASC
	`, op, since.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, false, fmt.Errorf("query latency samples: %w", err)
	}
	defer rows.Close()

	samples := make([]float64, 0)
	for rows.Next() {
		var sample sql.NullFloat64
		if err := rows.Scan(&sample); err != nil {
			return 0, false, fmt.Errorf("scan latency sample: %w", err)
		}
		if sample.Valid {
			samples = append(samples, sample.Float64)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, false, fmt.Errorf("iterate latency samples: %w", err)
	}
	if len(samples) == 0 {
		return 0, false, nil
	}
	return percentileValue(samples, percentile), true, nil
}

func percentileValue(sortedSamples []float64, percentile float64) float64 {
	if len(sortedSamples) == 0 {
		return 0
	}
	if percentile <= 0 {
		return sortedSamples[0]
	}
	if percentile >= 1 {
		return sortedSamples[len(sortedSamples)-1]
	}
	index := int(math.Ceil(float64(len(sortedSamples))*percentile)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sortedSamples) {
		index = len(sortedSamples) - 1
	}
	return sortedSamples[index]
}
