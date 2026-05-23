package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aileun/engram/internal/config"
	"github.com/aileun/engram/internal/quality"
	sqlitestore "github.com/aileun/engram/internal/storage/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: engramctl <status|migrate|quality> [--config path]")
		os.Exit(2)
	}

	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	configPath := fs.String("config", "./configs/example.yaml", "path to config file")
	_ = fs.Parse(os.Args[2:])

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	store, err := sqlitestore.New(cfg.Storage.SQLitePath)
	if err != nil {
		log.Fatalf("init sqlite store: %v", err)
	}
	defer func() { _ = store.Close() }()

	switch cmd {
	case "status":
		if err := store.ApplyMigrations(); err != nil {
			log.Fatalf("migrations: %v", err)
		}
		eventCount, err := store.EventCount(context.Background())
		if err != nil {
			log.Fatalf("count events: %v", err)
		}
		memoryCount, err := store.MemoryObjectCount(context.Background())
		if err != nil {
			log.Fatalf("count memory objects: %v", err)
		}
		fmt.Printf("engramctl status OK\nserver=%s:%d sqlite=%s\nevent_count=%d\nmemory_object_count=%d\n", cfg.Server.Bind, cfg.Server.Port, cfg.Storage.SQLitePath, eventCount, memoryCount)
	case "migrate":
		if err := store.ApplyMigrations(); err != nil {
			log.Fatalf("migrations: %v", err)
		}
		fmt.Printf("engramctl migrate OK (sqlite=%s)\n", cfg.Storage.SQLitePath)
	case "quality":
		if err := store.ApplyMigrations(); err != nil {
			log.Fatalf("migrations: %v", err)
		}
		svc := quality.NewService(store)
		metrics, err := svc.Metrics(context.Background())
		if err != nil {
			log.Fatalf("quality metrics: %v", err)
		}
		out, err := json.MarshalIndent(metrics, "", "  ")
		if err != nil {
			log.Fatalf("marshal metrics: %v", err)
		}
		fmt.Println(string(out))
	default:
		log.Fatalf("unknown command: %s", cmd)
	}
}
