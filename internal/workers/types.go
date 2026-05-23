package workers

import (
	"context"
	"time"
)

type Job struct {
	ID             int64
	Type           string
	Payload        map[string]any
	State          string
	Attempts       int
	MaxAttempts    int
	IdempotencyKey string
	CreatedAt      string
	UpdatedAt      string
}

type Store interface {
	ClaimWorkerJob(ctx context.Context, workerID string, now time.Time) (Job, bool, error)
	AppendWorkerCheckpoint(ctx context.Context, jobID int64, step, state string, details map[string]any, now time.Time) error
	MarkWorkerJobDone(ctx context.Context, jobID int64, attempt int, now time.Time) error
	MarkWorkerJobRetry(ctx context.Context, jobID int64, attempt int, reason string, nextAttemptAt, now time.Time) error
	MarkWorkerJobDeadLetter(ctx context.Context, jobID int64, attempt int, reason string, now time.Time) error
}

type Handler func(ctx context.Context, job Job) error
