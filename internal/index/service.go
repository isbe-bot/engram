package index

import (
	"context"
	"fmt"

	"github.com/aileun/engram/internal/citations"
	"github.com/aileun/engram/internal/embedding"
	"github.com/aileun/engram/internal/models"
	"github.com/aileun/engram/internal/retrieve"
	"github.com/aileun/engram/internal/storage/qdrant"
)

type Embedder interface {
	Embed(text string) []float32
}

type VectorStore interface {
	EnsureCollection(ctx context.Context, vectorSize int) error
	Upsert(ctx context.Context, points []qdrant.Point) error
	Search(ctx context.Context, vector []float32, limit int, filter map[string]any) ([]qdrant.SearchResult, error)
}

type Service struct {
	vectors    VectorStore
	embedder   Embedder
	vectorSize int
}

func NewService(vectors VectorStore, embedder Embedder, vectorSize int) *Service {
	if vectorSize <= 0 {
		vectorSize = embedding.HashVectorSize
	}
	return &Service{vectors: vectors, embedder: embedder, vectorSize: vectorSize}
}

func (s *Service) Ensure(ctx context.Context) error {
	if s == nil || s.vectors == nil {
		return fmt.Errorf("index service is not initialized")
	}
	return s.vectors.EnsureCollection(ctx, s.vectorSize)
}

func (s *Service) IndexMemory(ctx context.Context, obj models.MemoryObject) error {
	if s == nil || s.vectors == nil || s.embedder == nil {
		return nil
	}
	point := qdrant.Point{
		ID:     qdrant.PointID(obj.ObjectID),
		Vector: s.embedder.Embed(memoryText(obj)),
		Payload: map[string]any{
			"object_id":       obj.ObjectID,
			"type":            obj.Type,
			"schema_version":  obj.SchemaVer,
			"content":         obj.Content,
			"source_refs":     obj.SourceRefs,
			"confidence":      obj.Confidence,
			"classification":  obj.Classification,
			"scope":           obj.Scope,
			"provenance_hash": obj.ProvenanceHash,
			"status":          obj.Status,
			"created_at":      obj.CreatedAt,
			"updated_at":      obj.UpdatedAt,
		},
	}
	return s.vectors.Upsert(ctx, []qdrant.Point{point})
}

func (s *Service) SearchSemantic(ctx context.Context, q retrieve.Query) ([]retrieve.SearchResult, error) {
	if s == nil || s.vectors == nil || s.embedder == nil || q.Text == "" {
		return nil, nil
	}
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	filter := qdrantFilter(q)
	results, err := s.vectors.Search(ctx, s.embedder.Embed(q.Text), limit, filter)
	if err != nil {
		return nil, err
	}
	out := make([]retrieve.SearchResult, 0, len(results))
	for _, result := range results {
		payload := result.Payload
		objID := asString(payload["object_id"])
		sourceRefs := asStringSlice(payload["source_refs"])
		citationsList := make([]map[string]any, 0, len(sourceRefs)+1)
		if objID != "" {
			citationsList = append(citationsList, citations.Make("memory_object", "memory_objects/"+objID))
		}
		for _, ref := range sourceRefs {
			citationsList = append(citationsList, citations.Make("source_ref", ref))
		}
		payload["rank_score"] = result.Score
		payload["semantic_score"] = result.Score
		payload["retrieval_source"] = "qdrant"
		payload["citations"] = citationsList
		out = append(out, payload)
	}
	return out, nil
}

func memoryText(obj models.MemoryObject) string {
	return obj.Type + " " + obj.Classification + " " + obj.Scope + " " + obj.Content
}

func qdrantFilter(q retrieve.Query) map[string]any {
	must := make([]map[string]any, 0)
	if q.Status != "" {
		must = append(must, matchFilter("status", q.Status))
	}
	if q.MinConfidence > 0 {
		must = append(must, map[string]any{"key": "confidence", "range": map[string]any{"gte": q.MinConfidence}})
	}
	if len(must) == 0 {
		return nil
	}
	return map[string]any{"must": must}
}

func matchFilter(key, value string) map[string]any {
	return map[string]any{"key": key, "match": map[string]any{"value": value}}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asStringSlice(v any) []string {
	switch typed := v.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return []string{}
	}
}
