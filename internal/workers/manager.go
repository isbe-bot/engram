package workers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/isbe-bot/engram/internal/config"
)

type Manager struct {
	cfg      config.Config
	store    Store
	handlers map[string]Handler
	now      func() time.Time

	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
}

type Option func(*Manager)

func WithStore(store Store) Option {
	return func(m *Manager) { m.store = store }
}

func WithHandler(jobType string, handler Handler) Option {
	return func(m *Manager) {
		if m.handlers == nil {
			m.handlers = map[string]Handler{}
		}
		m.handlers[jobType] = handler
	}
}

func NewManager(cfg config.Config, opts ...Option) *Manager {
	m := &Manager{
		cfg:      cfg,
		handlers: map[string]Handler{},
		now:      func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return fmt.Errorf("worker manager already started")
	}
	if m.store == nil {
		m.started = true
		return nil
	}

	workerCount := m.cfg.Ingestion.WorkerCount
	if workerCount <= 0 {
		workerCount = 1
	}

	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.started = true

	for i := 0; i < workerCount; i++ {
		workerID := fmt.Sprintf("worker-%d", i+1)
		m.wg.Add(1)
		go func(id string) {
			defer m.wg.Done()
			m.runWorker(runCtx, id)
		}(workerID)
	}

	return nil
}

func (m *Manager) Stop(ctx context.Context) {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	cancel := m.cancel
	m.started = false
	m.cancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	if ctx == nil {
		<-done
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (m *Manager) runWorker(ctx context.Context, workerID string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		now := m.now()
		job, ok, err := m.store.ClaimWorkerJob(ctx, workerID, now)
		if err != nil {
			if !sleepOrDone(ctx, 300*time.Millisecond) {
				return
			}
			continue
		}
		if !ok {
			if !sleepOrDone(ctx, 150*time.Millisecond) {
				return
			}
			continue
		}

		attempt := job.Attempts + 1
		_ = m.store.AppendWorkerCheckpoint(ctx, job.ID, "claimed", "running", map[string]any{"worker_id": workerID, "attempt": attempt}, now)

		handler := m.handlers[job.Type]
		if handler == nil {
			handler = defaultNoopHandler
		}
		if err := handler(ctx, job); err != nil {
			next := m.retryDelay(attempt)
			reason := err.Error()
			if attempt >= maxAttempts(job.MaxAttempts) {
				_ = m.store.MarkWorkerJobDeadLetter(ctx, job.ID, attempt, reason, m.now())
				_ = m.store.AppendWorkerCheckpoint(ctx, job.ID, "failed", "dead_letter", map[string]any{"reason": reason, "attempt": attempt}, m.now())
				continue
			}
			nextAt := m.now().Add(next)
			_ = m.store.MarkWorkerJobRetry(ctx, job.ID, attempt, reason, nextAt, m.now())
			_ = m.store.AppendWorkerCheckpoint(ctx, job.ID, "failed", "retry_scheduled", map[string]any{"reason": reason, "attempt": attempt, "retry_in_ms": next.Milliseconds()}, m.now())
			continue
		}

		_ = m.store.MarkWorkerJobDone(ctx, job.ID, attempt, m.now())
		_ = m.store.AppendWorkerCheckpoint(ctx, job.ID, "completed", "done", map[string]any{"worker_id": workerID, "attempt": attempt}, m.now())
	}
}

func (m *Manager) retryDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return time.Second
	}
	d := time.Duration(1<<(attempt-1)) * time.Second
	if d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}

func maxAttempts(v int) int {
	if v <= 0 {
		return 5
	}
	return v
}

func defaultNoopHandler(context.Context, Job) error { return nil }

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
