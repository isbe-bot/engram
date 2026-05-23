package retrieve

import (
	"context"
	"testing"
	"time"
)

type fakeMemorySearcher struct {
	results    []map[string]any
	nextCursor string
}

func (f fakeMemorySearcher) SearchMemoryObjects(ctx context.Context, q Query) ([]map[string]any, string, error) {
	_ = ctx
	_ = q
	return f.results, f.nextCursor, nil
}

type fakeSemanticSearcher struct {
	results []map[string]any
}

func (f fakeSemanticSearcher) SearchSemantic(ctx context.Context, q Query) ([]map[string]any, error) {
	_ = ctx
	_ = q
	return f.results, nil
}

type fakeEventSearcher struct {
	results []map[string]any
}

func (f fakeEventSearcher) SearchEvents(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	_ = ctx
	_ = query
	if limit < len(f.results) {
		return f.results[:limit], nil
	}
	return f.results, nil
}

type fakeLatencyRecorder struct {
	count int
	last  time.Duration
}

func (f *fakeLatencyRecorder) RecordLatency(_ context.Context, _ string, latency time.Duration) {
	f.count++
	f.last = latency
}

func TestSearchDeduplicatesSemanticAndSQLiteMemory(t *testing.T) {
	svc := NewService(
		fakeMemorySearcher{results: []map[string]any{{"object_id": "mem-1", "content": "sqlite duplicate"}, {"object_id": "mem-2", "content": "sqlite unique"}}},
		nil,
		fakeSemanticSearcher{results: []map[string]any{{"object_id": "mem-1", "content": "semantic hit", "retrieval_source": "qdrant", "rank_score": 0.9}}},
	)
	resp, err := svc.Search(context.Background(), Query{Text: "semantic", Limit: 5})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected semantic mem-1 plus sqlite mem-2, got %d: %+v", len(resp.Results), resp.Results)
	}
	if resp.Results[0]["retrieval_source"] != "qdrant" {
		t.Fatalf("expected qdrant result first, got %+v", resp.Results[0])
	}
}

func TestSearchIncludesEventsAndScores(t *testing.T) {
	svc := NewService(
		fakeMemorySearcher{results: []map[string]any{{
			"object_id":   "mem-1",
			"content":     "Use Go for ENGRAM",
			"status":      "accepted",
			"confidence":  0.9,
			"citations":   []map[string]any{{"kind": "memory_object", "path": "memory_objects/mem-1"}},
			"type":        "decision",
			"updated_at":  "2026-05-23T00:00:00Z",
			"source_refs": []string{"adr:0009"},
		}}, nextCursor: "5"},
		fakeEventSearcher{results: []map[string]any{{
			"event_id":       "evt-1",
			"event_type":     "task.completed",
			"environment_id": "client-a",
		}}},
	)

	resp, err := svc.Search(context.Background(), Query{Text: "go", Limit: 10, IncludeEvents: true})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
	if resp.NextCursor != "5" {
		t.Fatalf("expected next cursor 5, got %s", resp.NextCursor)
	}
	if _, ok := resp.Results[0]["rank_score"]; !ok {
		t.Fatal("expected rank_score for memory result")
	}
	if src := resp.Results[1]["retrieval_source"]; src != "ingested_events" {
		t.Fatalf("expected ingested_events source, got %v", src)
	}
	if _, ok := resp.Results[1]["citations"]; !ok {
		t.Fatal("expected citations on event result")
	}
}

func TestSearchRecordsLatency(t *testing.T) {
	recorder := &fakeLatencyRecorder{}
	svc := NewService(fakeMemorySearcher{results: []map[string]any{}}, nil)
	svc.SetLatencyRecorder(recorder)

	if _, err := svc.Search(context.Background(), Query{Limit: 1}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if recorder.count != 1 {
		t.Fatalf("expected one latency record, got %d", recorder.count)
	}
	if recorder.last < 0 {
		t.Fatalf("expected non-negative latency, got %v", recorder.last)
	}
}
