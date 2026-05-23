package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aileun/engram/internal/config"
	"github.com/aileun/engram/internal/quality"
	sqlitestore "github.com/aileun/engram/internal/storage/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	configPath := fs.String("config", "./configs/example.yaml", "path to config file")

	switch cmd {
	case "status", "migrate", "quality":
		_ = fs.Parse(os.Args[2:])
		runLocalCommand(cmd, *configPath)
	case "health":
		_ = fs.Parse(os.Args[2:])
		runAPICommand(*configPath, http.MethodGet, "/v1/health", nil)
	case "ingest":
		body := addBodyFlags(fs)
		_ = fs.Parse(os.Args[2:])
		runAPICommand(*configPath, http.MethodPost, "/v1/events/ingest", mustReadBody(*body))
	case "curate":
		body := addBodyFlags(fs)
		_ = fs.Parse(os.Args[2:])
		runAPICommand(*configPath, http.MethodPost, "/v1/memory/curate", mustReadBody(*body))
	case "search":
		query := fs.String("q", "", "search query")
		status := fs.String("status", "", "memory status filter")
		minConfidence := fs.String("min-confidence", "", "minimum confidence filter")
		limit := fs.String("limit", "", "result limit")
		cursor := fs.String("cursor", "", "pagination cursor")
		includeEvents := fs.Bool("include-events", false, "include raw ingested event hits")
		_ = fs.Parse(os.Args[2:])
		values := url.Values{}
		addQuery(values, "q", *query)
		addQuery(values, "status", *status)
		addQuery(values, "min_confidence", *minConfidence)
		addQuery(values, "limit", *limit)
		addQuery(values, "cursor", *cursor)
		if *includeEvents {
			values.Set("include_events", "true")
		}
		path := "/v1/memory/search"
		if encoded := values.Encode(); encoded != "" {
			path += "?" + encoded
		}
		runAPICommand(*configPath, http.MethodGet, path, nil)
	case "get":
		id := fs.String("id", "", "memory object id")
		_ = fs.Parse(os.Args[2:])
		if strings.TrimSpace(*id) == "" {
			log.Fatal("--id is required")
		}
		runAPICommand(*configPath, http.MethodGet, "/v1/memory/"+url.PathEscape(strings.TrimSpace(*id)), nil)
	case "correct":
		id := fs.String("id", "", "memory object id")
		body := addBodyFlags(fs)
		_ = fs.Parse(os.Args[2:])
		if strings.TrimSpace(*id) == "" {
			log.Fatal("--id is required")
		}
		runAPICommand(*configPath, http.MethodPost, "/v1/memory/"+url.PathEscape(strings.TrimSpace(*id))+"/correct", mustReadBody(*body))
	case "deprecate":
		id := fs.String("id", "", "memory object id")
		body := addBodyFlags(fs)
		_ = fs.Parse(os.Args[2:])
		if strings.TrimSpace(*id) == "" {
			log.Fatal("--id is required")
		}
		runAPICommand(*configPath, http.MethodPost, "/v1/memory/"+url.PathEscape(strings.TrimSpace(*id))+"/deprecate", mustReadBody(*body))
	case "history":
		id := fs.String("id", "", "memory object id")
		action := fs.String("action", "", "action filter")
		before := fs.String("before", "", "return events before id")
		limit := fs.String("limit", "", "result limit")
		_ = fs.Parse(os.Args[2:])
		if strings.TrimSpace(*id) == "" {
			log.Fatal("--id is required")
		}
		values := url.Values{}
		addQuery(values, "action", *action)
		addQuery(values, "before", *before)
		addQuery(values, "limit", *limit)
		path := "/v1/memory/" + url.PathEscape(strings.TrimSpace(*id)) + "/history"
		if encoded := values.Encode(); encoded != "" {
			path += "?" + encoded
		}
		runAPICommand(*configPath, http.MethodGet, path, nil)
	default:
		log.Fatalf("unknown command: %s", cmd)
	}
}

func usage() {
	fmt.Println("usage: engramctl <status|migrate|quality|health|ingest|curate|search|get|correct|deprecate|history> [--config path]")
}

func runLocalCommand(cmd, configPath string) {
	cfg, err := config.Load(configPath)
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
		writePrettyJSON(metrics)
	}
}

func runAPICommand(configPath, method, path string, body []byte) {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	endpoint := fmt.Sprintf("http://%s:%d%s", cfg.Server.Bind, cfg.Server.Port, path)
	req, err := http.NewRequestWithContext(context.Background(), method, endpoint, bytes.NewReader(body))
	if err != nil {
		log.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("request %s %s: %v", method, endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("read response: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Fatalf("request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var payload any
	if err := json.Unmarshal(respBody, &payload); err != nil {
		fmt.Println(strings.TrimSpace(string(respBody)))
		return
	}
	writePrettyJSON(payload)
}

type bodyFlags struct {
	json string
	file string
}

func addBodyFlags(fs *flag.FlagSet) *bodyFlags {
	body := &bodyFlags{}
	fs.StringVar(&body.json, "json", "", "JSON request body")
	fs.StringVar(&body.file, "file", "", "path to JSON request body file; use - for stdin")
	return body
}

func mustReadBody(flags bodyFlags) []byte {
	if strings.TrimSpace(flags.json) != "" && strings.TrimSpace(flags.file) != "" {
		log.Fatal("use either --json or --file, not both")
	}
	if strings.TrimSpace(flags.json) != "" {
		return []byte(flags.json)
	}
	if strings.TrimSpace(flags.file) == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("read stdin: %v", err)
		}
		return b
	}
	if strings.TrimSpace(flags.file) != "" {
		b, err := os.ReadFile(flags.file)
		if err != nil {
			log.Fatalf("read body file: %v", err)
		}
		return b
	}
	log.Fatal("--json or --file is required")
	return nil
}

func addQuery(values url.Values, key, value string) {
	if strings.TrimSpace(value) != "" {
		values.Set(key, strings.TrimSpace(value))
	}
}

func writePrettyJSON(v any) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("marshal json: %v", err)
	}
	fmt.Println(string(out))
}
