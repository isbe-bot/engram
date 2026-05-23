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
