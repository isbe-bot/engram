package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/isbe-bot/engram/internal/events"
	"github.com/isbe-bot/engram/internal/models"
	"github.com/isbe-bot/engram/internal/retrieve"
	"github.com/isbe-bot/engram/pkg/contracts"
)

func testEnv(id string) contracts.MutationEnvelope {
	return contracts.MutationEnvelope{ActorID: "tester", MutationID: id, Signature: "sig-" + id}
}

func TestApplyMigrationsCreatesContractColumns(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "engram.sqlite"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.ApplyMigrations(); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	rows, err := store.db.Query(`PRAGMA table_info(memory_objects)`)
	if err != nil {
		t.Fatalf("table info: %v", err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typeName string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table info: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info: %v", err)
	}
	for _, required := range []string{"scope", "provenance_hash"} {
		if !columns[required] {
			t.Fatalf("expected memory_objects.%s column", required)
		}
	}
}

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

func TestListEvents(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "engram.sqlite"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.ApplyMigrations(); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	if err := store.InsertEvent(ctx, events.Envelope{EventID: "evt-list-1", EventType: "test.one", EnvironmentID: "env", OccurredAt: "2026-05-23T19:45:00Z", Data: map[string]any{"ok": true}}); err != nil {
		t.Fatalf("insert event1: %v", err)
	}
	if err := store.InsertEvent(ctx, events.Envelope{EventID: "evt-list-2", EventType: "test.two", EnvironmentID: "env", OccurredAt: "2026-05-23T19:46:00Z", Data: map[string]any{"n": float64(2)}}); err != nil {
		t.Fatalf("insert event2: %v", err)
	}
	eventsOut, err := store.ListEvents(ctx, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(eventsOut) != 2 || eventsOut[0].EventID != "evt-list-1" || eventsOut[1].EventID != "evt-list-2" {
		t.Fatalf("unexpected events: %+v", eventsOut)
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

	created, err := store.CreateMemoryObject(ctx, obj, testEnv("mut-1"))
	if err != nil {
		t.Fatalf("create memory object: %v", err)
	}
	if created.ObjectID != obj.ObjectID {
		t.Fatalf("unexpected object id: %s", created.ObjectID)
	}
	if created.Scope != "local" {
		t.Fatalf("expected default scope local, got %q", created.Scope)
	}
	if created.ProvenanceHash == "" {
		t.Fatal("expected provenance hash")
	}

	if _, err := store.CreateMemoryObject(ctx, obj, testEnv("mut-2")); err == nil {
		t.Fatal("expected duplicate memory object error")
	}

	corrected, err := store.CorrectMemoryObject(ctx, obj.ObjectID, "Use Go + SQLite for ENGRAM core", "clarified architecture", []string{"adr:0009"}, testEnv("mut-3"))
	if err != nil {
		t.Fatalf("correct memory object: %v", err)
	}
	if corrected.Content != "Use Go + SQLite for ENGRAM core" {
		t.Fatalf("unexpected corrected content: %s", corrected.Content)
	}
	if corrected.ProvenanceHash == created.ProvenanceHash {
		t.Fatal("expected correction to update provenance hash")
	}

	deprecated, err := store.DeprecateMemoryObject(ctx, obj.ObjectID, "superseded by v2 policy", testEnv("mut-4"))
	if err != nil {
		t.Fatalf("deprecate memory object: %v", err)
	}
	if deprecated.Status != models.MemoryStatusDeprecated {
		t.Fatalf("expected deprecated status, got %s", deprecated.Status)
	}
}

func TestSearchMemoryObjectsWithFiltersAndCitations(t *testing.T) {
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
	_, err = store.CreateMemoryObject(ctx, models.MemoryObject{
		ObjectID:       "mem-a",
		Type:           "decision",
		SchemaVer:      "v1",
		Content:        "Use Go for ENGRAM core",
		SourceRefs:     []string{"adr:0009"},
		Confidence:     0.95,
		Classification: "product",
		Status:         models.MemoryStatusAccepted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, testEnv("mut-a"))
	if err != nil {
		t.Fatalf("create mem-a: %v", err)
	}

	_, err = store.CreateMemoryObject(ctx, models.MemoryObject{
		ObjectID:       "mem-b",
		Type:           "preference",
		SchemaVer:      "v1",
		Content:        "Prefer concise status updates",
		SourceRefs:     []string{"chat:3811"},
		Confidence:     0.40,
		Classification: "communication",
		Status:         models.MemoryStatusDeprecated,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, testEnv("mut-b"))
	if err != nil {
		t.Fatalf("create mem-b: %v", err)
	}

	results, nextCursor, err := store.SearchMemoryObjects(ctx, retrieve.Query{Text: "Go", Status: models.MemoryStatusAccepted, MinConfidence: 0.9, Limit: 1})
	if err != nil {
		t.Fatalf("search memory objects: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if got := results[0]["object_id"]; got != "mem-a" {
		t.Fatalf("unexpected object_id %v", got)
	}
	if nextCursor != "" {
		t.Fatalf("did not expect next cursor for single-match page, got %q", nextCursor)
	}
	if _, ok := results[0]["citations"].([]map[string]any); !ok {
		if alt, ok2 := results[0]["citations"].([]any); ok2 {
			if len(alt) == 0 {
				t.Fatal("expected citations to be populated")
			}
		} else {
			t.Fatalf("expected citations array, got %T", results[0]["citations"])
		}
	}

	page1, page1Cursor, err := store.SearchMemoryObjects(ctx, retrieve.Query{Limit: 1})
	if err != nil {
		t.Fatalf("search memory objects page1: %v", err)
	}
	if len(page1) != 1 || page1Cursor == "" {
		t.Fatalf("expected page1 len=1 and non-empty cursor, got len=%d cursor=%q", len(page1), page1Cursor)
	}

	page2, page2Cursor, err := store.SearchMemoryObjects(ctx, retrieve.Query{Limit: 1, Cursor: page1Cursor})
	if err != nil {
		t.Fatalf("search memory objects page2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("expected page2 len=1, got %d", len(page2))
	}
	if page2Cursor != "" {
		t.Fatalf("expected empty page2 cursor, got %q", page2Cursor)
	}

	objects, err := store.ListMemoryObjects(ctx, 100)
	if err != nil {
		t.Fatalf("list memory objects: %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("expected two listed memory objects, got %d", len(objects))
	}
	if objects[0].ObjectID == "" || objects[0].ProvenanceHash == "" {
		t.Fatalf("expected hydrated listed objects, got %+v", objects[0])
	}
}

func TestQualityMetricQueries(t *testing.T) {
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
	if err := store.InsertEvent(ctx, events.Envelope{EventID: "evt-q1", EventType: "task.completed", EnvironmentID: "env-q", OccurredAt: now, Data: map[string]any{"ok": true}}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	obj := models.MemoryObject{
		ObjectID:       "mem-q1",
		Type:           "decision",
		SchemaVer:      "v1",
		Content:        "Track quality metrics",
		SourceRefs:     []string{"spec:quality"},
		Confidence:     0.88,
		Classification: "product",
		Status:         models.MemoryStatusAccepted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if _, err := store.CreateMemoryObject(ctx, obj, testEnv("mut-q1")); err != nil {
		t.Fatalf("create memory object: %v", err)
	}
	if _, err := store.CorrectMemoryObject(ctx, obj.ObjectID, "Track quality and freshness metrics", "Clarify quality metric coverage", []string{"spec:quality"}, testEnv("mut-q2")); err != nil {
		t.Fatalf("correct memory object: %v", err)
	}

	statusCounts, err := store.MemoryStatusCounts(ctx)
	if err != nil {
		t.Fatalf("status counts: %v", err)
	}
	if statusCounts[models.MemoryStatusAccepted] != 1 {
		t.Fatalf("expected accepted count=1, got %+v", statusCounts)
	}
	actionCounts, err := store.MemoryAuditActionCounts(ctx)
	if err != nil {
		t.Fatalf("action counts: %v", err)
	}
	if actionCounts["curated"] != 1 || actionCounts["corrected"] != 1 {
		t.Fatalf("unexpected action counts: %+v", actionCounts)
	}
	if latest, ok, err := store.LatestIngestedEventAt(ctx); err != nil || !ok || latest != now {
		t.Fatalf("unexpected latest ingested event: latest=%q ok=%v err=%v", latest, ok, err)
	}
	if latest, ok, err := store.LatestMemoryUpdatedAt(ctx); err != nil || !ok || latest == "" {
		t.Fatalf("unexpected latest memory update: latest=%q ok=%v err=%v", latest, ok, err)
	}
}

func TestListMemoryObjectEventsFilters(t *testing.T) {
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
		ObjectID:       "mem-h1",
		Type:           "decision",
		SchemaVer:      "v1",
		Content:        "Use policy checks",
		SourceRefs:     []string{"adr:0011"},
		Confidence:     0.75,
		Classification: "product",
		Status:         models.MemoryStatusAccepted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if _, err := store.CreateMemoryObject(ctx, obj, testEnv("mut-h1")); err != nil {
		t.Fatalf("create memory object: %v", err)
	}
	if _, err := store.CorrectMemoryObject(ctx, obj.ObjectID, "Use strict policy checks", "Clarify enforcement details", []string{"adr:0011", "spec:m6"}, testEnv("mut-h2")); err != nil {
		t.Fatalf("correct memory object: %v", err)
	}

	allEvents, err := store.ListMemoryObjectEvents(ctx, obj.ObjectID, "", 0, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(allEvents) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(allEvents))
	}

	correctedOnly, err := store.ListMemoryObjectEvents(ctx, obj.ObjectID, "corrected", 0, 10)
	if err != nil {
		t.Fatalf("list corrected events: %v", err)
	}
	if len(correctedOnly) != 1 {
		t.Fatalf("expected 1 corrected event, got %d", len(correctedOnly))
	}
	if correctedOnly[0]["action"] != "corrected" {
		t.Fatalf("unexpected action: %v", correctedOnly[0]["action"])
	}

	if correctedOnly[0]["event_hash"] == "" {
		t.Fatal("expected event_hash on corrected event")
	}
	if correctedOnly[0]["prev_hash"] == "" {
		t.Fatal("expected prev_hash on corrected event")
	}
}

func TestWorkerJobQueueLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "engram.sqlite"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.ApplyMigrations(); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	jobID, err := store.EnqueueWorkerJob(ctx, "quality_eval", map[string]any{"scope": "daily"}, "idem-1", 3)
	if err != nil {
		t.Fatalf("enqueue worker job: %v", err)
	}
	dupID, err := store.EnqueueWorkerJob(ctx, "quality_eval", map[string]any{"scope": "daily"}, "idem-1", 3)
	if err != nil {
		t.Fatalf("enqueue duplicate idempotency key: %v", err)
	}
	if dupID != jobID {
		t.Fatalf("expected duplicate idempotency key to return same id, got job=%d dup=%d", jobID, dupID)
	}

	claimed, ok, err := store.ClaimWorkerJob(ctx, "worker-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("claim worker job: %v", err)
	}
	if !ok || claimed.ID != jobID {
		t.Fatalf("expected claimed job %d, got ok=%v job=%+v", jobID, ok, claimed)
	}
	if err := store.AppendWorkerCheckpoint(ctx, jobID, "claimed", "running", map[string]any{"worker": "worker-1"}, time.Now().UTC()); err != nil {
		t.Fatalf("append worker checkpoint: %v", err)
	}
	if err := store.MarkWorkerJobRetry(ctx, jobID, 1, "temporary error", time.Now().UTC().Add(time.Second), time.Now().UTC()); err != nil {
		t.Fatalf("mark retry: %v", err)
	}
	if err := store.MarkWorkerJobDeadLetter(ctx, jobID, 3, "permanent failure", time.Now().UTC()); err != nil {
		t.Fatalf("mark dead-letter: %v", err)
	}
	state, attempts, err := store.WorkerJobState(ctx, jobID)
	if err != nil {
		t.Fatalf("worker job state: %v", err)
	}
	if state != "dead_letter" || attempts != 3 {
		t.Fatalf("expected dead_letter/3, got state=%s attempts=%d", state, attempts)
	}
}

func TestQualitySLOQueries(t *testing.T) {
	ctx := context.Background()
	store, err := New(filepath.Join(t.TempDir(), "engram.sqlite"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.ApplyMigrations(); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	now := time.Now().UTC()
	if err := store.RecordLatencySample(ctx, "retrieve.search", 100*time.Millisecond, now); err != nil {
		t.Fatalf("record latency sample1: %v", err)
	}
	if err := store.RecordLatencySample(ctx, "retrieve.search", 240*time.Millisecond, now); err != nil {
		t.Fatalf("record latency sample2: %v", err)
	}
	if err := store.RecordLatencySample(ctx, "retrieve.search", 50*time.Millisecond, now); err != nil {
		t.Fatalf("record latency sample3: %v", err)
	}
	p95, ok, err := store.LatencyP95MS(ctx, "retrieve.search", now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("latency p95: %v", err)
	}
	if !ok || p95 < 200 {
		t.Fatalf("expected p95 >= 200ms, got ok=%v p95=%.2f", ok, p95)
	}

	createdAt := now.Add(-2 * time.Hour).Format(time.RFC3339)
	updatedAt := now.Add(-40 * 24 * time.Hour).Format(time.RFC3339)
	obj := models.MemoryObject{
		ObjectID:       "mem-slo-1",
		Type:           "decision",
		SchemaVer:      "v1",
		Content:        "Baseline",
		SourceRefs:     []string{"spec:slo"},
		Confidence:     0.7,
		Classification: "product",
		Status:         models.MemoryStatusAccepted,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
	if _, err := store.CreateMemoryObject(ctx, obj, testEnv("mut-slo-1")); err != nil {
		t.Fatalf("create memory object: %v", err)
	}
	if _, err := store.CorrectMemoryObject(ctx, obj.ObjectID, "Baseline updated", "Latency correction for slo metrics", []string{"spec:slo"}, testEnv("mut-slo-2")); err != nil {
		t.Fatalf("correct memory object: %v", err)
	}

	corrP95, corrOK, err := store.CorrectionApplyLatencyP95MS(ctx, now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("correction apply latency p95: %v", err)
	}
	if !corrOK || corrP95 < 0 {
		t.Fatalf("expected correction latency sample, got ok=%v p95=%v", corrOK, corrP95)
	}
	staleRate, staleOK, err := store.StaleMemoryRate(ctx, now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("stale memory rate: %v", err)
	}
	if !staleOK || staleRate < 0 || staleRate > 1 {
		t.Fatalf("unexpected stale memory rate: ok=%v rate=%v", staleOK, staleRate)
	}
}
