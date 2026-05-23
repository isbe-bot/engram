package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aileun/engram/internal/config"
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

	ctx := context.Background()
	switch cmd {
	case "status":
		fmt.Printf("engramctl status OK\nserver=%s:%d sqlite=%s\n", cfg.Server.Bind, cfg.Server.Port, cfg.Storage.SQLitePath)
	case "migrate":
		fmt.Printf("engramctl migrate TODO (sqlite=%s)\n", cfg.Storage.SQLitePath)
	case "quality":
		fmt.Printf("engramctl quality TODO (interval=%s)\n", cfg.Quality.EvalInterval)
	default:
		_ = ctx
		log.Fatalf("unknown command: %s", cmd)
	}
}
