package govern

import (
	"context"
	"fmt"
	"strings"

	"github.com/aileun/engram/internal/models"
	"github.com/aileun/engram/internal/policy"
	"github.com/aileun/engram/pkg/contracts"
)

const protectedConfidenceThreshold = 0.90

type memoryGovernor interface {
	GetMemoryObject(ctx context.Context, objectID string) (models.MemoryObject, error)
	CorrectMemoryObject(ctx context.Context, objectID, content, reason string, sourceRefs []string, env contracts.MutationEnvelope) (models.MemoryObject, error)
	DeprecateMemoryObject(ctx context.Context, objectID, reason string, env contracts.MutationEnvelope) (models.MemoryObject, error)
	ListMemoryObjectEvents(ctx context.Context, objectID, action string, beforeID, limit int) ([]map[string]any, error)
}

type Service struct {
	store memoryGovernor
}

func NewService(store memoryGovernor) *Service {
	return &Service{store: store}
}

func (s *Service) Get(ctx context.Context, objectID string) (models.MemoryObject, error) {
	if s == nil || s.store == nil {
		return models.MemoryObject{}, fmt.Errorf("govern service is not initialized")
	}
	objectID = strings.TrimSpace(objectID)
	if objectID == "" {
		return models.MemoryObject{}, fmt.Errorf("object_id is required")
	}
	return s.store.GetMemoryObject(ctx, objectID)
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
	if err := validateEnvelope(req.Envelope); err != nil {
		return models.MemoryObject{}, err
	}
	if len(req.SourceRefs) > 0 {
		if err := policy.ValidateSourceRefs(req.SourceRefs); err != nil {
			return models.MemoryObject{}, err
		}
	}

	obj, err := s.store.GetMemoryObject(ctx, objectID)
	if err != nil {
		return models.MemoryObject{}, err
	}
	if obj.Confidence >= protectedConfidenceThreshold && !req.Force {
		return models.MemoryObject{}, fmt.Errorf("high-confidence memory requires force=true")
	}

	return s.store.CorrectMemoryObject(ctx, objectID, req.Content, req.Reason, req.SourceRefs, req.Envelope)
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
	if err := validateEnvelope(req.Envelope); err != nil {
		return models.MemoryObject{}, err
	}

	obj, err := s.store.GetMemoryObject(ctx, objectID)
	if err != nil {
		return models.MemoryObject{}, err
	}
	if obj.Confidence >= protectedConfidenceThreshold && !req.Force {
		return models.MemoryObject{}, fmt.Errorf("high-confidence memory requires force=true")
	}

	return s.store.DeprecateMemoryObject(ctx, objectID, req.Reason, req.Envelope)
}

func (s *Service) History(ctx context.Context, objectID string, req contracts.MemoryHistoryRequest) ([]map[string]any, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("govern service is not initialized")
	}
	objectID = strings.TrimSpace(objectID)
	if objectID == "" {
		return nil, fmt.Errorf("object_id is required")
	}
	action := strings.TrimSpace(strings.ToLower(req.Action))
	if action != "" {
		allowed := map[string]struct{}{"curated": {}, "corrected": {}, "deprecated": {}}
		if _, ok := allowed[action]; !ok {
			return nil, fmt.Errorf("invalid action filter")
		}
	}
	return s.store.ListMemoryObjectEvents(ctx, objectID, action, req.Before, req.Limit)
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

func validateEnvelope(env contracts.MutationEnvelope) error {
	if strings.TrimSpace(env.ActorID) == "" {
		return fmt.Errorf("envelope.actor_id is required")
	}
	if strings.TrimSpace(env.MutationID) == "" {
		return fmt.Errorf("envelope.mutation_id is required")
	}
	if strings.TrimSpace(env.Signature) == "" {
		return fmt.Errorf("envelope.signature is required")
	}
	return nil
}
