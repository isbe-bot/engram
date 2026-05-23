package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/isbe-bot/engram/internal/config"
)

type Dependencies struct {
	Ingest             ingestor
	Curate             curator
	Govern             governor
	Search             searcher
	Quality            qualityReporter
	Health             pinger
	APIKey             string
	APIKeys            []config.APIKeyConfig
	MaxBodyBytes       int64
	RateLimitPerMinute int
	Logger             *slog.Logger
}

type Server struct {
	http *http.Server
}

func NewServer(cfg config.Config, deps Dependencies) *Server {
	deps.APIKey = cfg.Server.APIKey
	deps.APIKeys = cfg.Server.APIKeys
	deps.MaxBodyBytes = cfg.Server.MaxBodyBytes
	deps.RateLimitPerMinute = cfg.Server.RateLimitPerMinute
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	mux := http.NewServeMux()
	registerRoutes(mux, deps)
	return &Server{http: &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.Port), Handler: loggingMiddleware(deps.Logger, mux)}}
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
