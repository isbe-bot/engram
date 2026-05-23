package config

import "testing"

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load("/tmp/does-not-exist.yaml"); err == nil {
		t.Fatal("expected error for missing config")
	}
}
