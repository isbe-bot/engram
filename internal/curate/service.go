package curate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/isbe-bot/engram/internal/models"
	"github.com/isbe-bot/engram/pkg/contracts"
)

type memoryCreator interface {
	CreateMemoryObject(ctx context.Context, m models.MemoryObject, env contracts.MutationEnvelope) (models.MemoryObject, error)
}

type memoryIndexer interface {
	IndexMemory(ctx context.Context, obj models.MemoryObject) error
}

type Service struct {
	store   memoryCreator
	indexer memoryIndexer
	now     func() time.Time
}

func NewService(store memoryCreator, indexers ...memoryIndexer) *Service {
	svc := &Service{store: store, now: time.Now}
	if len(indexers) > 0 {
		svc.indexer = indexers[0]
	}
	return svc
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

	created, err := s.store.CreateMemoryObject(ctx, obj, env)
	if err != nil {
		return models.MemoryObject{}, err
	}
	if s.indexer != nil {
		if err := s.indexer.IndexMemory(ctx, created); err != nil {
			return models.MemoryObject{}, fmt.Errorf("index memory object: %w", err)
		}
	}
	return created, nil
}
