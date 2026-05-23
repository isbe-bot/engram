package workers

import (
	"context"

	"github.com/aileun/engram/internal/config"
)

type Manager struct {
	cfg config.Config
}

func NewManager(cfg config.Config) *Manager { return &Manager{cfg: cfg} }

func (m *Manager) Start(ctx context.Context) error {
	_ = ctx
	_ = m.cfg
	// TODO: start ingest/curation/retrieval quality workers
	return nil
}

func (m *Manager) Stop(ctx context.Context) {
	_ = ctx
	// TODO: graceful worker shutdown
}
