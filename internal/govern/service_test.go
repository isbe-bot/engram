package govern

import (
	"context"
	"testing"

	"github.com/aileun/engram/internal/models"
	"github.com/aileun/engram/pkg/contracts"
)

type fakeStore struct{}

func (f fakeStore) CorrectMemoryObject(ctx context.Context, objectID, content, reason string, sourceRefs []string) (models.MemoryObject, error) {
	_ = ctx
	_ = sourceRefs
	return models.MemoryObject{ObjectID: objectID, Content: content, Status: models.MemoryStatusAccepted}, nil
}

func (f fakeStore) DeprecateMemoryObject(ctx context.Context, objectID, reason string) (models.MemoryObject, error) {
	_ = ctx
	_ = reason
	return models.MemoryObject{ObjectID: objectID, Status: models.MemoryStatusDeprecated}, nil
}

func (f fakeStore) ListMemoryObjectEvents(ctx context.Context, objectID string, limit int) ([]map[string]any, error) {
	_ = ctx
	_ = limit
	return []map[string]any{{"object_id": objectID, "action": "curated"}}, nil
}

func TestReasonQualityGuard(t *testing.T) {
	svc := NewService(fakeStore{})
	if _, err := svc.Correct(context.Background(), "mem-1", contracts.MemoryCorrectRequest{Content: "x", Reason: "fix"}); err == nil {
		t.Fatal("expected reason quality error")
	}
	if _, err := svc.Deprecate(context.Background(), "mem-1", contracts.MemoryDeprecateRequest{Reason: "obsolete due to revised policy"}); err != nil {
		t.Fatalf("unexpected deprecate error: %v", err)
	}
}

func TestHistory(t *testing.T) {
	svc := NewService(fakeStore{})
	events, err := svc.History(context.Background(), "mem-1", 10)
	if err != nil {
		t.Fatalf("history error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
}
