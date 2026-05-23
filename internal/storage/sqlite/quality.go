package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *Store) MemoryStatusCounts(ctx context.Context) (map[string]int, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store is not initialized")
	}
	return groupedCounts(ctx, s.db, `SELECT status, COUNT(1) FROM memory_objects GROUP BY status`)
}

func (s *Store) MemoryAuditActionCounts(ctx context.Context) (map[string]int, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store is not initialized")
	}
	return groupedCounts(ctx, s.db, `SELECT action, COUNT(1) FROM memory_object_events GROUP BY action`)
}

func (s *Store) LatestIngestedEventAt(ctx context.Context) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, fmt.Errorf("sqlite store is not initialized")
	}
	return latestString(ctx, s.db, `SELECT occurred_at FROM ingested_events ORDER BY occurred_at DESC LIMIT 1`)
}

func (s *Store) LatestMemoryUpdatedAt(ctx context.Context) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, fmt.Errorf("sqlite store is not initialized")
	}
	return latestString(ctx, s.db, `SELECT updated_at FROM memory_objects ORDER BY updated_at DESC LIMIT 1`)
}

func groupedCounts(ctx context.Context, db *sql.DB, query string) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query grouped counts: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return nil, fmt.Errorf("scan grouped count: %w", err)
		}
		counts[key] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate grouped counts: %w", err)
	}
	return counts, nil
}

func latestString(ctx context.Context, db *sql.DB, query string) (string, bool, error) {
	var value string
	if err := db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("query latest value: %w", err)
	}
	return value, true, nil
}
