package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/aileun/engram/internal/events"
	"github.com/aileun/engram/internal/models"
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

func TestMemoryObjectLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "engram.sqlite"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.ApplyMigrations(); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	obj := models.MemoryObject{
		ObjectID:       "mem-1",
		Type:           "decision",
		SchemaVer:      "v1",
		Content:        "Use Go for ENGRAM core",
		SourceRefs:     []string{"meeting:2026-05-23"},
		Confidence:     0.95,
		Classification: "product",
		Status:         models.MemoryStatusAccepted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	created, err := store.CreateMemoryObject(ctx, obj)
	if err != nil {
		t.Fatalf("create memory object: %v", err)
	}
	if created.ObjectID != obj.ObjectID {
		t.Fatalf("unexpected object id: %s", created.ObjectID)
	}

	if _, err := store.CreateMemoryObject(ctx, obj); err == nil {
		t.Fatal("expected duplicate memory object error")
	}

	corrected, err := store.CorrectMemoryObject(ctx, obj.ObjectID, "Use Go + SQLite for ENGRAM core", "clarified architecture", []string{"adr:0009"})
	if err != nil {
		t.Fatalf("correct memory object: %v", err)
	}
	if corrected.Content != "Use Go + SQLite for ENGRAM core" {
		t.Fatalf("unexpected corrected content: %s", corrected.Content)
	}

	deprecated, err := store.DeprecateMemoryObject(ctx, obj.ObjectID, "superseded by v2 policy")
	if err != nil {
		t.Fatalf("deprecate memory object: %v", err)
	}
	if deprecated.Status != models.MemoryStatusDeprecated {
		t.Fatalf("expected deprecated status, got %s", deprecated.Status)
	}
}
