package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/aileun/engram/internal/events"
)

type eventWriter interface {
	InsertEvent(ctx context.Context, env events.Envelope) error
}

type Service struct {
	store eventWriter
	now   func() time.Time
}

func NewService(store eventWriter) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Ingest(ctx context.Context, env events.Envelope) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("ingest service is not initialized")
	}

	if err := env.NormalizeAndValidate(s.now()); err != nil {
		return err
	}

	if err := s.store.InsertEvent(ctx, env); err != nil {
		return err
	}
	return nil
}
