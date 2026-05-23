package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aileun/engram/internal/api"
	"github.com/aileun/engram/internal/config"
	"github.com/aileun/engram/internal/curate"
	"github.com/aileun/engram/internal/embedding"
	"github.com/aileun/engram/internal/govern"
	"github.com/aileun/engram/internal/index"
	"github.com/aileun/engram/internal/ingest"
	"github.com/aileun/engram/internal/quality"
	"github.com/aileun/engram/internal/retrieve"
	qdrantstore "github.com/aileun/engram/internal/storage/qdrant"
	sqlitestore "github.com/aileun/engram/internal/storage/sqlite"
	"github.com/aileun/engram/internal/workers"
)

func main() {
	configPath := flag.String("config", "./configs/example.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	store, err := sqlitestore.New(cfg.Storage.SQLitePath)
	if err != nil {
		log.Fatalf("init sqlite store: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.ApplyMigrations(); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	var indexSvc *index.Service
	if cfg.Storage.QdrantURL != "" {
		qdrantClient, err := qdrantstore.New(cfg.Storage.QdrantURL, cfg.Storage.QdrantCollection)
		if err != nil {
			log.Fatalf("init qdrant client: %v", err)
		}
		indexSvc = index.NewService(qdrantClient, embedding.NewHashProvider(embedding.HashVectorSize), embedding.HashVectorSize)
		if err := indexSvc.Ensure(context.Background()); err != nil {
			log.Fatalf("ensure qdrant collection: %v", err)
		}
	}

	ingestSvc := ingest.NewService(store)
	curateSvc := curate.NewService(store, indexSvc)
	governSvc := govern.NewService(store, indexSvc)
	retrieveSvc := retrieve.NewService(store, store, indexSvc)
	retrieveSvc.SetLatencyRecorder(quality.NewLatencyRecorder(store))
	qualitySvc := quality.NewService(store)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	w := workers.NewManager(cfg, workers.WithStore(store))
	if err := w.Start(ctx); err != nil {
		log.Fatalf("start workers: %v", err)
	}
	defer w.Stop(context.Background())

	srv := api.NewServer(cfg, api.Dependencies{
		Ingest:  ingestSvc,
		Curate:  curateSvc,
		Govern:  governSvc,
		Search:  retrieveSvc,
		Quality: qualitySvc,
		Health:  store,
	})
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
