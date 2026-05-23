package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/aileun/engram/internal/events"
)

func TestStoreInsertAndSearch(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "engram.sqlite"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.ApplyMigrations(); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	env := events.Envelope{
		EventID:       "evt-1",
		EventType:     "task.completed",
		EnvironmentID: "client-alpha",
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Data: map[string]any{
			"title": "Ship ENGRAM v1",
		},
	}

	if err := store.InsertEvent(ctx, env); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	if err := store.InsertEvent(ctx, env); err == nil {
		t.Fatalf("expected duplicate insert error")
	}

	results, err := store.SearchEvents(ctx, "ENGRAM", 10)
	if err != nil {
		t.Fatalf("search events: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	count, err := store.EventCount(ctx)
	if err != nil {
		t.Fatalf("event count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected event_count=1 got %d", count)
	}
}
