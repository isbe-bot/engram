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
	"github.com/aileun/engram/internal/workers"
)

func main() {
	configPath := flag.String("config", "./configs/example.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	w := workers.NewManager(cfg)
	if err := w.Start(ctx); err != nil {
		log.Fatalf("start workers: %v", err)
	}
	defer w.Stop(context.Background())

	srv := api.NewServer(cfg)
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
