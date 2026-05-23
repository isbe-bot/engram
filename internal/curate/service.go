package curate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aileun/engram/internal/models"
	"github.com/aileun/engram/pkg/contracts"
)

type memoryCreator interface {
	CreateMemoryObject(ctx context.Context, m models.MemoryObject, env contracts.MutationEnvelope) (models.MemoryObject, error)
}

type Service struct {
	store memoryCreator
	now   func() time.Time
}

func NewService(store memoryCreator) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Curate(ctx context.Context, req contracts.MemoryWriteRequest) (models.MemoryObject, error) {
	if s == nil || s.store == nil {
		return models.MemoryObject{}, fmt.Errorf("curate service is not initialized")
	}

	now := s.now()
	obj := models.MemoryObject{
		ObjectID:       strings.TrimSpace(req.ObjectID),
		Type:           req.Type,
		SchemaVer:      req.SchemaVersion,
		Content:        req.Content,
		SourceRefs:     req.SourceRefs,
		Confidence:     req.Confidence,
		Classification: req.Classification,
		Scope:          req.Scope,
		Status:         models.MemoryStatusAccepted,
		CreatedAt:      now.UTC().Format(time.RFC3339),
		UpdatedAt:      now.UTC().Format(time.RFC3339),
	}
	if obj.ObjectID == "" {
		obj.ObjectID = fmt.Sprintf("mem-%d", now.UTC().UnixNano())
	}
	if err := obj.NormalizeAndValidate(now); err != nil {
		return models.MemoryObject{}, err
	}

	env := req.Envelope
	if strings.TrimSpace(env.ActorID) == "" {
		env.ActorID = "system"
	}
	if strings.TrimSpace(env.MutationID) == "" {
		env.MutationID = fmt.Sprintf("mut-%d", now.UTC().UnixNano())
	}
	if strings.TrimSpace(env.Signature) == "" {
		env.Signature = "unsigned"
	}

	return s.store.CreateMemoryObject(ctx, obj, env)
}
