package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aileun/engram/internal/config"
)

type Dependencies struct {
	Ingest  ingestor
	Curate  curator
	Govern  governor
	Search  searcher
	Quality qualityReporter
	Health  pinger
}

type Server struct {
	http *http.Server
}

func NewServer(cfg config.Config, deps Dependencies) *Server {
	mux := http.NewServeMux()
	registerRoutes(mux, deps)
	return &Server{http: &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.Port), Handler: mux}}
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() { errCh <- s.http.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.http.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
