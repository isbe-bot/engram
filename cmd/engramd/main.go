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
	"github.com/aileun/engram/internal/govern"
	"github.com/aileun/engram/internal/ingest"
	"github.com/aileun/engram/internal/retrieve"
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

	ingestSvc := ingest.NewService(store)
	curateSvc := curate.NewService(store)
	governSvc := govern.NewService(store)
	retrieveSvc := retrieve.NewService(store)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	w := workers.NewManager(cfg)
	if err := w.Start(ctx); err != nil {
		log.Fatalf("start workers: %v", err)
	}
	defer w.Stop(context.Background())

	srv := api.NewServer(cfg, api.Dependencies{
		Ingest: ingestSvc,
		Curate: curateSvc,
		Govern: governSvc,
		Search: retrieveSvc,
		Health: store,
	})
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
