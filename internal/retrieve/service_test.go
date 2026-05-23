package retrieve

import (
	"context"
	"testing"
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
