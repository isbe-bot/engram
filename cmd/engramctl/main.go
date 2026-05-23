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
	"path/filepath"
	"strings"
	"time"

	"github.com/isbe-bot/engram/internal/config"
	"github.com/isbe-bot/engram/internal/embedding"
	"github.com/isbe-bot/engram/internal/index"
	"github.com/isbe-bot/engram/internal/quality"
	qdrantstore "github.com/isbe-bot/engram/internal/storage/qdrant"
	sqlitestore "github.com/isbe-bot/engram/internal/storage/sqlite"
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
	case "status", "migrate", "quality", "report", "reindex":
		_ = fs.Parse(os.Args[2:])
		runLocalCommand(cmd, *configPath)
	case "init":
		dataDir := fs.String("data-dir", "./data", "directory for ENGRAM runtime data")
		qdrantURL := fs.String("qdrant-url", "http://127.0.0.1:6333", "Qdrant URL; use empty string to disable semantic indexing")
		qdrantCollection := fs.String("qdrant-collection", "engram_memory", "Qdrant collection name")
		bind := fs.String("bind", "127.0.0.1", "server bind address")
		port := fs.Int("port", 8787, "server port")
		apiKey := fs.String("api-key", "", "optional API bearer token")
		force := fs.Bool("force", false, "overwrite existing config file")
		_ = fs.Parse(os.Args[2:])
		runInit(*configPath, initOptions{
			DataDir:          *dataDir,
			QdrantURL:        *qdrantURL,
			QdrantCollection: *qdrantCollection,
			Bind:             *bind,
			Port:             *port,
			APIKey:           *apiKey,
			Force:            *force,
		})
	case "backup":
		out := fs.String("out", "", "backup output path")
		_ = fs.Parse(os.Args[2:])
		runBackup(*configPath, *out)
	case "restore":
		from := fs.String("from", "", "backup SQLite file to restore")
		_ = fs.Parse(os.Args[2:])
		runRestore(*configPath, *from)
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
	fmt.Println("usage: engramctl <init|status|migrate|quality|report|reindex|backup|restore|health|ingest|curate|search|get|correct|deprecate|history> [--config path]")
}

type initOptions struct {
	DataDir          string
	QdrantURL        string
	QdrantCollection string
	Bind             string
	Port             int
	APIKey           string
	Force            bool
}

func runInit(configPath string, opts initOptions) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		log.Fatal("--config is required")
	}
	dataDir := strings.TrimSpace(opts.DataDir)
	if dataDir == "" {
		log.Fatal("--data-dir is required")
	}
	if _, err := os.Stat(configPath); err == nil && !opts.Force {
		log.Fatalf("config already exists: %s (use --force to overwrite)", configPath)
	} else if err != nil && !os.IsNotExist(err) {
		log.Fatalf("stat config: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "backups"), 0o755); err != nil {
		log.Fatalf("create backups dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		log.Fatalf("create config dir: %v", err)
	}

	sqlitePath := filepath.Join(dataDir, "engram.sqlite")
	qdrantCollection := strings.TrimSpace(opts.QdrantCollection)
	if strings.TrimSpace(opts.QdrantURL) == "" {
		qdrantCollection = ""
	}
	content := fmt.Sprintf(`server:
  bind: %q
  port: %d
  api_key: %q
  max_body_bytes: 1048576
  rate_limit_per_minute: 0

storage:
  sqlite_path: %q
  qdrant_url: %q
  qdrant_collection: %q

ingestion:
  max_batch_size: 200
  worker_count: 1

quality:
  eval_interval: "24h"
`, strings.TrimSpace(opts.Bind), opts.Port, strings.TrimSpace(opts.APIKey), sqlitePath, strings.TrimSpace(opts.QdrantURL), qdrantCollection)

	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		log.Fatalf("write config: %v", err)
	}
	writePrettyJSON(map[string]any{
		"status":      "ok",
		"config":      configPath,
		"data_dir":    dataDir,
		"sqlite_path": sqlitePath,
	})
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
	case "report":
		if err := store.ApplyMigrations(); err != nil {
			log.Fatalf("migrations: %v", err)
		}
		svc := quality.NewService(store)
		report, err := svc.Report(context.Background())
		if err != nil {
			log.Fatalf("quality report: %v", err)
		}
		writePrettyJSON(report)
	case "reindex":
		runReindex(cfg, store)
	}
}

func runBackup(configPath, outPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	outPath = strings.TrimSpace(outPath)
	if outPath == "" {
		stamp := time.Now().UTC().Format("20060102T150405Z")
		outPath = filepath.Join(filepath.Dir(cfg.Storage.SQLitePath), "engram-"+stamp+".sqlite.bak")
	}
	if err := copyFile(cfg.Storage.SQLitePath, outPath); err != nil {
		log.Fatalf("backup sqlite: %v", err)
	}
	writePrettyJSON(map[string]any{"status": "ok", "source": cfg.Storage.SQLitePath, "backup": outPath})
}

func runRestore(configPath, fromPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	fromPath = strings.TrimSpace(fromPath)
	if fromPath == "" {
		log.Fatal("--from is required")
	}
	if _, err := os.Stat(fromPath); err != nil {
		log.Fatalf("restore source: %v", err)
	}
	if _, err := os.Stat(cfg.Storage.SQLitePath); err == nil {
		stamp := time.Now().UTC().Format("20060102T150405Z")
		preRestore := cfg.Storage.SQLitePath + ".pre-restore-" + stamp
		if err := copyFile(cfg.Storage.SQLitePath, preRestore); err != nil {
			log.Fatalf("create pre-restore backup: %v", err)
		}
	}
	if err := copyFile(fromPath, cfg.Storage.SQLitePath); err != nil {
		log.Fatalf("restore sqlite: %v", err)
	}
	writePrettyJSON(map[string]any{"status": "ok", "restored_from": fromPath, "target": cfg.Storage.SQLitePath})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func runReindex(cfg config.Config, store *sqlitestore.Store) {
	if strings.TrimSpace(cfg.Storage.QdrantURL) == "" {
		log.Fatal("storage.qdrant_url is required for reindex")
	}
	if err := store.ApplyMigrations(); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	qdrantClient, err := qdrantstore.New(cfg.Storage.QdrantURL, cfg.Storage.QdrantCollection)
	if err != nil {
		log.Fatalf("init qdrant client: %v", err)
	}
	indexSvc := index.NewService(qdrantClient, embedding.NewHashProvider(embedding.HashVectorSize), embedding.HashVectorSize)
	if err := indexSvc.Ensure(context.Background()); err != nil {
		log.Fatalf("ensure qdrant collection: %v", err)
	}
	objects, err := store.ListMemoryObjects(context.Background(), 10000)
	if err != nil {
		log.Fatalf("list memory objects: %v", err)
	}
	for _, obj := range objects {
		if err := indexSvc.IndexMemory(context.Background(), obj); err != nil {
			log.Fatalf("index memory object %s: %v", obj.ObjectID, err)
		}
	}
	writePrettyJSON(map[string]any{"status": "ok", "indexed_count": len(objects), "collection": cfg.Storage.QdrantCollection})
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
	if strings.TrimSpace(cfg.Server.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.Server.APIKey))
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
