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
	return s.store.DeprecateMemoryObject(ctx, objectID, req.Reason)
}
