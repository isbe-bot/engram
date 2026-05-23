package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/isbe-bot/engram/internal/workers"
)

func (s *Store) EnqueueWorkerJob(ctx context.Context, jobType string, payload map[string]any, idempotencyKey string, maxAttempts int) (int64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("sqlite store is not initialized")
	}
	jobType = strings.TrimSpace(jobType)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if jobType == "" {
		return 0, fmt.Errorf("job_type is required")
	}
	if idempotencyKey == "" {
		return 0, fmt.Errorf("idempotency_key is required")
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO worker_jobs (
			job_type, payload_json, idempotency_key, state, attempts, max_attempts,
			lease_owner, lease_until, next_attempt_at, last_error, created_at, updated_at
		)
		VALUES (?, ?, ?, 'pending', 0, ?, '', '', ?, '', ?, ?)
	`, jobType, string(payloadJSON), idempotencyKey, maxAttempts, now, now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			var existingID int64
			if qErr := s.db.QueryRowContext(ctx, `SELECT id FROM worker_jobs WHERE idempotency_key = ?`, idempotencyKey).Scan(&existingID); qErr != nil {
				return 0, fmt.Errorf("lookup duplicate worker job: %w", qErr)
			}
			return existingID, nil
		}
		return 0, fmt.Errorf("enqueue worker job: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("worker job last insert id: %w", err)
	}
	return id, nil
}

func (s *Store) ClaimWorkerJob(ctx context.Context, workerID string, now time.Time) (workers.Job, bool, error) {
	if s == nil || s.db == nil {
		return workers.Job{}, false, fmt.Errorf("sqlite store is not initialized")
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return workers.Job{}, false, fmt.Errorf("worker_id is required")
	}
	now = now.UTC()
	nowStr := now.Format(time.RFC3339)
	leaseUntil := now.Add(30 * time.Second).Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workers.Job{}, false, fmt.Errorf("begin claim tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		jobID          int64
		jobType        string
		payloadJSON    string
		state          string
		attempts       int
		maxAttempts    int
		idempotencyKey string
		createdAt      string
		updatedAt      string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT id, job_type, payload_json, state, attempts, max_attempts, idempotency_key, created_at, updated_at
		FROM worker_jobs
		WHERE state IN ('pending', 'retry_scheduled')
		  AND next_attempt_at <= ?
		ORDER BY id ASC
		LIMIT 1
	`, nowStr).Scan(&jobID, &jobType, &payloadJSON, &state, &attempts, &maxAttempts, &idempotencyKey, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			if cErr := tx.Commit(); cErr != nil {
				return workers.Job{}, false, fmt.Errorf("commit empty claim tx: %w", cErr)
			}
			return workers.Job{}, false, nil
		}
		return workers.Job{}, false, fmt.Errorf("select worker job: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE worker_jobs
		SET state = 'running', lease_owner = ?, lease_until = ?, updated_at = ?
		WHERE id = ?
	`, workerID, leaseUntil, nowStr, jobID); err != nil {
		return workers.Job{}, false, fmt.Errorf("claim worker job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return workers.Job{}, false, fmt.Errorf("commit claim tx: %w", err)
	}

	payload := map[string]any{}
	if strings.TrimSpace(payloadJSON) != "" {
		_ = json.Unmarshal([]byte(payloadJSON), &payload)
	}

	return workers.Job{
		ID:             jobID,
		Type:           jobType,
		Payload:        payload,
		State:          state,
		Attempts:       attempts,
		MaxAttempts:    maxAttempts,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, true, nil
}

func (s *Store) AppendWorkerCheckpoint(ctx context.Context, jobID int64, step, state string, details map[string]any, now time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store is not initialized")
	}
	if jobID <= 0 {
		return fmt.Errorf("job_id is required")
	}
	if details == nil {
		details = map[string]any{}
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal checkpoint details: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO worker_checkpoints (job_id, step, state, details_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, jobID, strings.TrimSpace(step), strings.TrimSpace(state), string(detailsJSON), now.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("append worker checkpoint: %w", err)
	}
	return nil
}

func (s *Store) MarkWorkerJobDone(ctx context.Context, jobID int64, attempt int, now time.Time) error {
	return s.updateWorkerJobState(ctx, jobID, "done", attempt, "", now, now)
}

func (s *Store) MarkWorkerJobRetry(ctx context.Context, jobID int64, attempt int, reason string, nextAttemptAt, now time.Time) error {
	return s.updateWorkerJobState(ctx, jobID, "retry_scheduled", attempt, reason, nextAttemptAt, now)
}

func (s *Store) MarkWorkerJobDeadLetter(ctx context.Context, jobID int64, attempt int, reason string, now time.Time) error {
	return s.updateWorkerJobState(ctx, jobID, "dead_letter", attempt, reason, now, now)
}

func (s *Store) updateWorkerJobState(ctx context.Context, jobID int64, state string, attempt int, reason string, nextAttemptAt, now time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store is not initialized")
	}
	if jobID <= 0 {
		return fmt.Errorf("job_id is required")
	}
	nowStr := now.UTC().Format(time.RFC3339)
	nextAttempt := nextAttemptAt.UTC().Format(time.RFC3339)
	completedAt := ""
	if state == "done" || state == "dead_letter" {
		completedAt = nowStr
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE worker_jobs
		SET state = ?, attempts = ?, last_error = ?, lease_owner = '', lease_until = '',
		    next_attempt_at = ?, completed_at = ?, updated_at = ?
		WHERE id = ?
	`, state, attempt, strings.TrimSpace(reason), nextAttempt, completedAt, nowStr, jobID)
	if err != nil {
		return fmt.Errorf("update worker job state: %w", err)
	}
	return nil
}

func (s *Store) WorkerJobState(ctx context.Context, jobID int64) (string, int, error) {
	if s == nil || s.db == nil {
		return "", 0, fmt.Errorf("sqlite store is not initialized")
	}
	var state string
	var attempts int
	if err := s.db.QueryRowContext(ctx, `SELECT state, attempts FROM worker_jobs WHERE id = ?`, jobID).Scan(&state, &attempts); err != nil {
		if err == sql.ErrNoRows {
			return "", 0, fmt.Errorf("worker job not found: %d", jobID)
		}
		return "", 0, fmt.Errorf("query worker job state: %w", err)
	}
	return state, attempts, nil
}
