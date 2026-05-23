package govern

import (
	"context"
	"fmt"
	"testing"

	"github.com/aileun/engram/internal/models"
	"github.com/aileun/engram/pkg/contracts"
)

type fakeStore struct {
	obj models.MemoryObject
}

func (f fakeStore) GetMemoryObject(ctx context.Context, objectID string) (models.MemoryObject, error) {
	_ = ctx
	if f.obj.ObjectID == "" || f.obj.ObjectID != objectID {
		return models.MemoryObject{}, fmt.Errorf("memory object not found: %s", objectID)
	}
	return f.obj, nil
}

func (f fakeStore) CorrectMemoryObject(ctx context.Context, objectID, content, reason string, sourceRefs []string) (models.MemoryObject, error) {
	_ = ctx
	_ = reason
	_ = sourceRefs
	obj := f.obj
	obj.ObjectID = objectID
	obj.Content = content
	return obj, nil
}

func (f fakeStore) DeprecateMemoryObject(ctx context.Context, objectID, reason string) (models.MemoryObject, error) {
	_ = ctx
	_ = reason
	obj := f.obj
	obj.ObjectID = objectID
	obj.Status = models.MemoryStatusDeprecated
	return obj, nil
}

func (f fakeStore) ListMemoryObjectEvents(ctx context.Context, objectID, action string, beforeID, limit int) ([]map[string]any, error) {
	_ = ctx
	_ = beforeID
	_ = limit
	return []map[string]any{{"object_id": objectID, "action": action}}, nil
}

func TestReasonQualityGuard(t *testing.T) {
	svc := NewService(fakeStore{obj: models.MemoryObject{ObjectID: "mem-1", Confidence: 0.5, SourceRefs: []string{"adr:0009"}}})
	if _, err := svc.Correct(context.Background(), "mem-1", contracts.MemoryCorrectRequest{Content: "x", Reason: "fix"}); err == nil {
		t.Fatal("expected reason quality error")
	}
	if _, err := svc.Deprecate(context.Background(), "mem-1", contracts.MemoryDeprecateRequest{Reason: "obsolete due to revised policy"}); err != nil {
		t.Fatalf("unexpected deprecate error: %v", err)
	}
}

func TestHighConfidenceRequiresForce(t *testing.T) {
	svc := NewService(fakeStore{obj: models.MemoryObject{ObjectID: "mem-1", Confidence: 0.95, SourceRefs: []string{"adr:0009"}}})

	if _, err := svc.Correct(context.Background(), "mem-1", contracts.MemoryCorrectRequest{
		Content:    "refined",
		Reason:     "Clarify architecture decisions",
		SourceRefs: []string{"adr:0010"},
	}); err == nil {
		t.Fatal("expected high-confidence force requirement")
	}

	if _, err := svc.Deprecate(context.Background(), "mem-1", contracts.MemoryDeprecateRequest{
		Reason: "Deprecated by policy revision",
	}); err == nil {
		t.Fatal("expected high-confidence force requirement on deprecate")
	}

	if _, err := svc.Correct(context.Background(), "mem-1", contracts.MemoryCorrectRequest{
		Content:    "refined",
		Reason:     "Clarify architecture decisions",
		SourceRefs: []string{"adr:0010"},
		Force:      true,
	}); err != nil {
		t.Fatalf("unexpected force-correct error: %v", err)
	}
}

func TestHistory(t *testing.T) {
	svc := NewService(fakeStore{obj: models.MemoryObject{ObjectID: "mem-1", Confidence: 0.5, SourceRefs: []string{"adr:0009"}}})
	events, err := svc.History(context.Background(), "mem-1", contracts.MemoryHistoryRequest{Action: "corrected", Limit: 10})
	if err != nil {
		t.Fatalf("history error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
}
