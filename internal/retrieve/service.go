package retrieve

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aileun/engram/internal/citations"
)

type SearchResult = map[string]any

type Query struct {
	Text          string
	Status        string
	MinConfidence float64
	Limit         int
	Cursor        string
	IncludeEvents bool
}

type Response struct {
	Results    []SearchResult
	NextCursor string
}

type memorySearcher interface {
	SearchMemoryObjects(ctx context.Context, q Query) ([]SearchResult, string, error)
}

type eventSearcher interface {
	SearchEvents(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

type semanticSearcher interface {
	SearchSemantic(ctx context.Context, q Query) ([]SearchResult, error)
}

type latencyRecorder interface {
	RecordLatency(ctx context.Context, operation string, latency time.Duration)
}

type Service struct {
	memory   memorySearcher
	events   eventSearcher
	semantic semanticSearcher
	recorder latencyRecorder
}

func NewService(memory memorySearcher, events eventSearcher, semantic ...semanticSearcher) *Service {
	svc := &Service{memory: memory, events: events}
	if len(semantic) > 0 {
		svc.semantic = semantic[0]
	}
	return svc
}

func (s *Service) SetLatencyRecorder(recorder latencyRecorder) {
	if s == nil {
		return
	}
	s.recorder = recorder
}

func (s *Service) Search(ctx context.Context, q Query) (Response, error) {
	start := time.Now()
	defer func() {
		if s != nil && s.recorder != nil {
			s.recorder.RecordLatency(ctx, "retrieve.search", time.Since(start))
		}
	}()

	if s == nil || s.memory == nil {
		return Response{Results: []SearchResult{}}, nil
	}

	memoryResults, nextCursor, err := s.memory.SearchMemoryObjects(ctx, q)
	if err != nil {
		return Response{}, err
	}
	for i := range memoryResults {
		memoryResults[i]["rank_score"] = scoreMemory(memoryResults[i], q)
	}

	results := make([]SearchResult, 0, len(memoryResults)+5)
	seen := map[string]struct{}{}
	if s.semantic != nil && strings.TrimSpace(q.Text) != "" {
		semanticResults, err := s.semantic.SearchSemantic(ctx, q)
		if err != nil {
			return Response{}, err
		}
		for _, result := range semanticResults {
			if objID := asString(result["object_id"]); objID != "" {
				seen[objID] = struct{}{}
			}
			results = append(results, result)
		}
	}
	for _, result := range memoryResults {
		objID := asString(result["object_id"])
		if objID != "" {
			if _, ok := seen[objID]; ok {
				continue
			}
			seen[objID] = struct{}{}
		}
		results = append(results, result)
	}

	if q.IncludeEvents && s.events != nil {
		eventLimit := q.Limit / 2
		if eventLimit < 1 {
			eventLimit = 3
		}
		if eventLimit > 10 {
			eventLimit = 10
		}

		eventResults, err := s.events.SearchEvents(ctx, q.Text, eventLimit)
		if err != nil {
			return Response{}, err
		}
		for _, ev := range eventResults {
			eventID := strings.TrimSpace(asString(ev["event_id"]))
			ev["retrieval_source"] = "ingested_events"
			ev["rank_score"] = scoreEvent(ev, q)
			ev["citations"] = []map[string]any{citations.Make("event", "ingested_events/"+eventID)}
			results = append(results, ev)
		}
	}

	return Response{Results: results, NextCursor: nextCursor}, nil
}

func scoreMemory(r SearchResult, q Query) float64 {
	score := 0.0
	if c, ok := r["confidence"].(float64); ok {
		score += c * 0.7
	}
	if strings.EqualFold(asString(r["status"]), "accepted") {
		score += 0.2
	}
	text := strings.ToLower(strings.TrimSpace(q.Text))
	if text != "" {
		if strings.Contains(strings.ToLower(asString(r["content"])), text) {
			score += 0.1
		}
	}
	return clamp(score, 0, 1)
}

func scoreEvent(r SearchResult, q Query) float64 {
	score := 0.25
	text := strings.ToLower(strings.TrimSpace(q.Text))
	if text != "" {
		if strings.Contains(strings.ToLower(asString(r["event_type"])), text) || strings.Contains(strings.ToLower(asString(r["environment_id"])), text) {
			score += 0.2
		}
	}
	return clamp(score, 0, 1)
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
