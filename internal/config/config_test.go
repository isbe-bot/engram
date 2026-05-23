package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load("/tmp/does-not-exist.yaml"); err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestLoadRequiresQdrantCollectionWhenURLSet(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "bad-qdrant.yaml")
	content := "server:\n  bind: '127.0.0.1'\n  port: 8787\nstorage:\n  sqlite_path: '/tmp/engram.sqlite'\n  qdrant_url: 'http://127.0.0.1:6333'\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadValidation(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte("server:\n  bind: ''\n  port: 0\nstorage:\n  sqlite_path: ''\n"), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadDefaultsAndEnvOverrides(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "minimal.yaml")
	if err := os.WriteFile(cfgPath, []byte("storage:\n  sqlite_path: './local.sqlite'\n"), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	t.Setenv("ENGRAM_SERVER_PORT", "9876")
	t.Setenv("ENGRAM_API_KEY", "test-token")
	t.Setenv("ENGRAM_MAX_BODY_BYTES", "2048")
	t.Setenv("ENGRAM_QDRANT_COLLECTION", "test_collection")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.Bind != DefaultBind {
		t.Fatalf("expected default bind %q, got %q", DefaultBind, cfg.Server.Bind)
	}
	if cfg.Server.Port != 9876 {
		t.Fatalf("expected env port override, got %d", cfg.Server.Port)
	}
	if cfg.Server.APIKey != "test-token" {
		t.Fatalf("expected env API key override")
	}
	if cfg.Server.MaxBodyBytes != 2048 {
		t.Fatalf("expected max body override, got %d", cfg.Server.MaxBodyBytes)
	}
	if cfg.Storage.QdrantCollection != "test_collection" {
		t.Fatalf("expected qdrant collection override")
	}
}
