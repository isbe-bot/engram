package index

import (
	"context"
	"testing"
	"time"

	"github.com/aileun/engram/internal/embedding"
	"github.com/aileun/engram/internal/models"
	"github.com/aileun/engram/internal/retrieve"
	"github.com/aileun/engram/internal/storage/qdrant"
)

type fakeVectorStore struct {
	ensured int
	upserts []qdrant.Point
	search  []qdrant.SearchResult
}

func (f *fakeVectorStore) EnsureCollection(context.Context, int) error {
	f.ensured++
	return nil
}

func (f *fakeVectorStore) Upsert(_ context.Context, points []qdrant.Point) error {
	f.upserts = append(f.upserts, points...)
	return nil
}

func (f *fakeVectorStore) Search(context.Context, []float32, int, map[string]any) ([]qdrant.SearchResult, error) {
	return f.search, nil
}

func TestServiceIndexMemory(t *testing.T) {
	vectors := &fakeVectorStore{}
	svc := NewService(vectors, embedding.NewHashProvider(16), 16)
	if err := svc.Ensure(context.Background()); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if vectors.ensured != 1 {
		t.Fatalf("expected ensure call")
	}
	obj := models.MemoryObject{ObjectID: "mem-1", Type: "decision", Content: "Use semantic search", SourceRefs: []string{"spec:index"}, Confidence: 0.8, Classification: "product"}
	if err := obj.NormalizeAndValidate(testNow()); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if err := svc.IndexMemory(context.Background(), obj); err != nil {
		t.Fatalf("index memory: %v", err)
	}
	if len(vectors.upserts) != 1 {
		t.Fatalf("expected one upsert, got %d", len(vectors.upserts))
	}
	if vectors.upserts[0].Payload["object_id"] != "mem-1" {
		t.Fatalf("unexpected payload: %+v", vectors.upserts[0].Payload)
	}
}

func TestServiceSearchSemantic(t *testing.T) {
	vectors := &fakeVectorStore{search: []qdrant.SearchResult{{
		ID:    qdrant.PointID("mem-1"),
		Score: 0.75,
		Payload: map[string]any{
			"object_id":   "mem-1",
			"content":     "Semantic search result",
			"source_refs": []any{"spec:index"},
			"confidence":  0.8,
			"status":      "accepted",
		},
	}}}
	svc := NewService(vectors, embedding.NewHashProvider(16), 16)
	results, err := svc.SearchSemantic(context.Background(), retrieve.Query{Text: "semantic", Status: "accepted", MinConfidence: 0.7, Limit: 5})
	if err != nil {
		t.Fatalf("search semantic: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if results[0]["retrieval_source"] != "qdrant" {
		t.Fatalf("expected qdrant retrieval source, got %+v", results[0])
	}
	if _, ok := results[0]["citations"].([]map[string]any); !ok {
		t.Fatalf("expected citations, got %T", results[0]["citations"])
	}
}

func testNow() time.Time { return time.Date(2026, 5, 23, 18, 30, 0, 0, time.UTC) }
