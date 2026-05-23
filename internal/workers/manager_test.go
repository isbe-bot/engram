package workers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aileun/engram/internal/config"
)

type fakeStore struct {
	mu          sync.Mutex
	queue       []Job
	retries     map[int64]int
	done        map[int64]int
	dead        map[int64]int
	checkpoints map[int64]int
}

func newFakeStore(jobs ...Job) *fakeStore {
	return &fakeStore{
		queue:       append([]Job{}, jobs...),
		retries:     map[int64]int{},
		done:        map[int64]int{},
		dead:        map[int64]int{},
		checkpoints: map[int64]int{},
	}
}

func (f *fakeStore) ClaimWorkerJob(context.Context, string, time.Time) (Job, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.queue) == 0 {
		return Job{}, false, nil
	}
	job := f.queue[0]
	f.queue = f.queue[1:]
	return job, true, nil
}

func (f *fakeStore) AppendWorkerCheckpoint(_ context.Context, jobID int64, _, _ string, _ map[string]any, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkpoints[jobID]++
	return nil
}

func (f *fakeStore) MarkWorkerJobDone(_ context.Context, jobID int64, attempt int, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.done[jobID] = attempt
	return nil
}

func (f *fakeStore) MarkWorkerJobRetry(_ context.Context, jobID int64, attempt int, _ string, _ time.Time, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retries[jobID] = attempt
	return nil
}

func (f *fakeStore) MarkWorkerJobDeadLetter(_ context.Context, jobID int64, attempt int, _ string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dead[jobID] = attempt
	return nil
}

func TestManagerProcessesSuccess(t *testing.T) {
	store := newFakeStore(Job{ID: 1, Type: "ingest", Attempts: 0, MaxAttempts: 3})
	cfg := config.Config{}
	cfg.Ingestion.WorkerCount = 1

	processed := make(chan struct{}, 1)
	mgr := NewManager(cfg,
		WithStore(store),
		WithHandler("ingest", func(context.Context, Job) error {
			processed <- struct{}{}
			return nil
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case <-processed:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not process job")
	}
	mgr.Stop(context.Background())

	if got := store.done[1]; got != 1 {
		t.Fatalf("expected done attempt=1, got %d", got)
	}
	if got := store.checkpoints[1]; got < 2 {
		t.Fatalf("expected at least two checkpoints, got %d", got)
	}
}

func TestManagerMovesToDeadLetterAtMaxAttempts(t *testing.T) {
	store := newFakeStore(Job{ID: 2, Type: "curation", Attempts: 2, MaxAttempts: 3})
	cfg := config.Config{}
	cfg.Ingestion.WorkerCount = 1

	mgr := NewManager(cfg,
		WithStore(store),
		WithHandler("curation", func(context.Context, Job) error { return errors.New("boom") }),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	mgr.Stop(context.Background())

	if got := store.dead[2]; got != 3 {
		t.Fatalf("expected dead-letter attempt=3, got %d", got)
	}
	if _, ok := store.retries[2]; ok {
		t.Fatal("did not expect retry for max-attempt failure")
	}
}
