package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlitestore "github.com/isbe-bot/engram/internal/storage/sqlite"
)

func TestPortableJSONLImportExportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)
	fixturePath := filepath.Join(dir, "fixture.jsonl")
	if err := os.WriteFile(fixturePath, []byte(strings.Join([]string{
		`{"kind":"event","version":"engram.portable.v1","event":{"event_id":"portable-event-1","event_type":"portable.smoke","environment_id":"test","occurred_at":"2026-05-23T19:50:00Z","data":{"ok":true}}}`,
		`{"kind":"memory_object","version":"engram.portable.v1","memory":{"object_id":"portable-memory-1","type":"decision","schema_version":"v1","content":"Portable JSONL moves governed memory between ENGRAM instances","source_refs":["spec:portable-jsonl"],"confidence":0.88,"classification":"operations","scope":"local","status":"accepted","created_at":"2026-05-23T19:50:00Z","updated_at":"2026-05-23T19:50:00Z"},"envelope":{"actor_id":"test","mutation_id":"portable-mut-1","signature":"portable-sig"}}`,
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	runImport(cfgPath, fixturePath, true, false)
	runImport(cfgPath, fixturePath, false, false)

	store, err := sqlitestore.New(filepath.Join(dir, "engram.sqlite"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer func() { _ = store.Close() }()
	if eventCount, err := store.EventCount(context.Background()); err != nil || eventCount != 1 {
		t.Fatalf("event count=%d err=%v", eventCount, err)
	}
	if memoryCount, err := store.MemoryObjectCount(context.Background()); err != nil || memoryCount != 1 {
		t.Fatalf("memory count=%d err=%v", memoryCount, err)
	}

	exportPath := filepath.Join(dir, "export.jsonl")
	runExport(cfgPath, exportPath, true, true, 100)

	file, err := os.Open(exportPath)
	if err != nil {
		t.Fatalf("open export: %v", err)
	}
	defer func() { _ = file.Close() }()

	seen := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var rec portableRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			t.Fatalf("decode export line: %v", err)
		}
		if rec.Event != nil {
			seen[rec.Event.EventID] = true
		}
		if rec.Memory != nil {
			seen[rec.Memory.ObjectID] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan export: %v", err)
	}
	if !seen["portable-event-1"] || !seen["portable-memory-1"] {
		t.Fatalf("missing exported records: %+v", seen)
	}

	runImport(cfgPath, fixturePath, false, false)
	if eventCount, err := store.EventCount(context.Background()); err != nil || eventCount != 1 {
		t.Fatalf("duplicate import event count=%d err=%v", eventCount, err)
	}
	if memoryCount, err := store.MemoryObjectCount(context.Background()); err != nil || memoryCount != 1 {
		t.Fatalf("duplicate import memory count=%d err=%v", memoryCount, err)
	}
}

func writeTestConfig(t *testing.T, dir string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "engram.yaml")
	cfg := "server:\n" +
		"  bind: \"127.0.0.1\"\n" +
		"  port: 28806\n" +
		"storage:\n" +
		"  sqlite_path: \"" + filepath.Join(dir, "engram.sqlite") + "\"\n" +
		"  qdrant_url: \"\"\n" +
		"  qdrant_collection: \"\"\n" +
		"ingestion:\n" +
		"  max_batch_size: 200\n" +
		"  worker_count: 1\n" +
		"quality:\n" +
		"  eval_interval: \"24h\"\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return cfgPath
}
