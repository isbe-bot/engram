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
