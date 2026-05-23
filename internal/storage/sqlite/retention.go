package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/isbe-bot/engram/internal/models"
	"github.com/isbe-bot/engram/internal/retention"
)

func (s *Store) RetentionCandidates(ctx context.Context, policy retention.Policy, now time.Time) ([]retention.Candidate, retention.Summary, error) {
	if s == nil || s.db == nil {
		return nil, retention.Summary{}, fmt.Errorf("sqlite store is not initialized")
	}
	policy = retention.NormalizePolicy(policy)
	candidates := make([]retention.Candidate, 0)
	summary := retention.Summary{}

	eventCutoff := now.Add(-time.Duration(policy.EventRetentionDays) * 24 * time.Hour)
	deprecatedCutoff := now.Add(-time.Duration(policy.DeprecatedMemoryRetentionDays) * 24 * time.Hour)
	staleCutoff := now.Add(-time.Duration(policy.StaleMemoryDays) * 24 * time.Hour)

	eventCount, err := s.countBefore(ctx, `SELECT COUNT(1) FROM ingested_events WHERE occurred_at < ?`, eventCutoff)
	if err != nil {
		return nil, retention.Summary{}, err
	}
	summary.RawEventDeleteCandidates = eventCount

	deprecatedCount, err := s.countBefore(ctx, `SELECT COUNT(1) FROM memory_objects WHERE status = ? AND updated_at < ?`, deprecatedCutoff, models.MemoryStatusDeprecated)
	if err != nil {
		return nil, retention.Summary{}, err
	}
	summary.DeprecatedMemoryDeleteCandidates = deprecatedCount

	staleCount, err := s.countBefore(ctx, `SELECT COUNT(1) FROM memory_objects WHERE status = ? AND updated_at < ?`, staleCutoff, models.MemoryStatusAccepted)
	if err != nil {
		return nil, retention.Summary{}, err
	}
	summary.StaleMemoryReviewCandidates = staleCount
	summary.TotalDeleteCandidates = summary.RawEventDeleteCandidates + summary.DeprecatedMemoryDeleteCandidates
	summary.TotalReviewCandidates = summary.StaleMemoryReviewCandidates

	remaining := policy.MaxCandidates
	appendEvents := func(kind, action, reason, query string, cutoff time.Time, args ...any) error {
		if remaining <= 0 {
			return nil
		}
		queryArgs := append(args, cutoff.Format(time.RFC3339), remaining)
		rows, err := s.db.QueryContext(ctx, query, queryArgs...)
		if err != nil {
			return fmt.Errorf("query retention candidates: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, ts string
			if err := rows.Scan(&id, &ts); err != nil {
				return fmt.Errorf("scan retention candidate: %w", err)
			}
			candidates = append(candidates, retention.Candidate{Kind: kind, ID: id, Reason: reason, Timestamp: ts, AgeDays: ageDays(now, ts), Action: action})
			remaining--
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate retention candidates: %w", err)
		}
		return nil
	}

	if err := appendEvents(
		"raw_event",
		"delete",
		"event occurred before retention cutoff",
		`SELECT event_id, occurred_at FROM ingested_events WHERE occurred_at < ? ORDER BY occurred_at ASC LIMIT ?`,
		eventCutoff,
	); err != nil {
		return nil, retention.Summary{}, err
	}
	if err := appendEvents(
		"memory_object",
		"delete",
		"deprecated memory updated before retention cutoff",
		`SELECT object_id, updated_at FROM memory_objects WHERE status = ? AND updated_at < ? ORDER BY updated_at ASC LIMIT ?`,
		deprecatedCutoff,
		models.MemoryStatusDeprecated,
	); err != nil {
		return nil, retention.Summary{}, err
	}
	if err := appendEvents(
		"memory_object",
		"review",
		"accepted memory is stale and should be reviewed before compaction",
		`SELECT object_id, updated_at FROM memory_objects WHERE status = ? AND updated_at < ? ORDER BY updated_at ASC LIMIT ?`,
		staleCutoff,
		models.MemoryStatusAccepted,
	); err != nil {
		return nil, retention.Summary{}, err
	}

	return candidates, summary, nil
}

func (s *Store) ApplyRetention(ctx context.Context, policy retention.Policy, now time.Time) (retention.Applied, error) {
	if s == nil || s.db == nil {
		return retention.Applied{}, fmt.Errorf("sqlite store is not initialized")
	}
	policy = retention.NormalizePolicy(policy)
	eventCutoff := now.Add(-time.Duration(policy.EventRetentionDays) * 24 * time.Hour).Format(time.RFC3339)
	deprecatedCutoff := now.Add(-time.Duration(policy.DeprecatedMemoryRetentionDays) * 24 * time.Hour).Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return retention.Applied{}, fmt.Errorf("begin retention transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var deprecatedIDs []string
	rows, err := tx.QueryContext(ctx, `SELECT object_id FROM memory_objects WHERE status = ? AND updated_at < ?`, models.MemoryStatusDeprecated, deprecatedCutoff)
	if err != nil {
		return retention.Applied{}, fmt.Errorf("select deprecated retention candidates: %w", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return retention.Applied{}, fmt.Errorf("scan deprecated retention candidate: %w", err)
		}
		deprecatedIDs = append(deprecatedIDs, id)
	}
	if err := rows.Close(); err != nil {
		return retention.Applied{}, fmt.Errorf("close deprecated candidate rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return retention.Applied{}, fmt.Errorf("iterate deprecated retention candidates: %w", err)
	}

	eventRes, err := tx.ExecContext(ctx, `DELETE FROM ingested_events WHERE occurred_at < ?`, eventCutoff)
	if err != nil {
		return retention.Applied{}, fmt.Errorf("delete retained events: %w", err)
	}
	deletedEvents, _ := eventRes.RowsAffected()

	for _, id := range deprecatedIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM memory_object_events WHERE object_id = ?`, id); err != nil {
			return retention.Applied{}, fmt.Errorf("delete memory audit events for %s: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM memory_objects WHERE object_id = ?`, id); err != nil {
			return retention.Applied{}, fmt.Errorf("delete deprecated memory %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return retention.Applied{}, fmt.Errorf("commit retention transaction: %w", err)
	}
	return retention.Applied{DeletedRawEvents: int(deletedEvents), DeletedDeprecatedMemory: len(deprecatedIDs)}, nil
}

func (s *Store) countBefore(ctx context.Context, query string, cutoff time.Time, args ...any) (int, error) {
	queryArgs := append(args, cutoff.Format(time.RFC3339))
	var count int
	if err := s.db.QueryRowContext(ctx, query, queryArgs...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count retention candidates: %w", err)
	}
	return count, nil
}

func ageDays(now time.Time, timestamp string) int {
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return 0
	}
	days := int(now.Sub(parsed).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
