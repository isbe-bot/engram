package govern

import (
	"context"
	"fmt"
	"strings"

	"github.com/aileun/engram/internal/models"
	"github.com/aileun/engram/pkg/contracts"
)

type memoryGovernor interface {
	CorrectMemoryObject(ctx context.Context, objectID, content, reason string, sourceRefs []string) (models.MemoryObject, error)
	DeprecateMemoryObject(ctx context.Context, objectID, reason string) (models.MemoryObject, error)
	ListMemoryObjectEvents(ctx context.Context, objectID string, limit int) ([]map[string]any, error)
}

type Service struct {
	store memoryGovernor
}

func NewService(store memoryGovernor) *Service {
	return &Service{store: store}
}

func (s *Service) Correct(ctx context.Context, objectID string, req contracts.MemoryCorrectRequest) (models.MemoryObject, error) {
	if s == nil || s.store == nil {
		return models.MemoryObject{}, fmt.Errorf("govern service is not initialized")
	}
	objectID = strings.TrimSpace(objectID)
	if objectID == "" {
		return models.MemoryObject{}, fmt.Errorf("object_id is required")
	}
	if err := validateReason(req.Reason); err != nil {
		return models.MemoryObject{}, err
	}
	return s.store.CorrectMemoryObject(ctx, objectID, req.Content, req.Reason, req.SourceRefs)
}

func (s *Service) Deprecate(ctx context.Context, objectID string, req contracts.MemoryDeprecateRequest) (models.MemoryObject, error) {
	if s == nil || s.store == nil {
		return models.MemoryObject{}, fmt.Errorf("govern service is not initialized")
	}
	objectID = strings.TrimSpace(objectID)
	if objectID == "" {
		return models.MemoryObject{}, fmt.Errorf("object_id is required")
	}
	if err := validateReason(req.Reason); err != nil {
		return models.MemoryObject{}, err
	}
	return s.store.DeprecateMemoryObject(ctx, objectID, req.Reason)
}

func (s *Service) History(ctx context.Context, objectID string, limit int) ([]map[string]any, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("govern service is not initialized")
	}
	objectID = strings.TrimSpace(objectID)
	if objectID == "" {
		return nil, fmt.Errorf("object_id is required")
	}
	return s.store.ListMemoryObjectEvents(ctx, objectID, limit)
}

func validateReason(reason string) error {
	r := strings.TrimSpace(reason)
	if len(r) < 12 {
		return fmt.Errorf("reason must be at least 12 characters")
	}
	lower := strings.ToLower(r)
	bad := []string{"fix", "update", "n/a", "na", "none", "test"}
	for _, token := range bad {
		if lower == token {
			return fmt.Errorf("reason is too vague; provide context")
		}
	}
	if len(strings.Fields(r)) < 2 {
		return fmt.Errorf("reason must contain enough detail")
	}
	return nil
}
