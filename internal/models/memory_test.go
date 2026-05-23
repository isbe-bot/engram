package models

import (
	"strings"
	"testing"
	"time"
)

func TestMemoryObjectNormalizeAndValidateContractFields(t *testing.T) {
	obj := MemoryObject{
		ObjectID:       " mem-1 ",
		Type:           "Decision",
		Content:        "Use Go for ENGRAM core",
		SourceRefs:     []string{" adr:0009 "},
		Confidence:     0.91,
		Classification: "Product",
	}

	if err := obj.NormalizeAndValidate(time.Date(2026, 5, 23, 18, 20, 0, 0, time.UTC)); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if obj.Type != "decision" {
		t.Fatalf("expected normalized type decision, got %q", obj.Type)
	}
	if obj.Scope != "local" {
		t.Fatalf("expected default scope local, got %q", obj.Scope)
	}
	if obj.ProvenanceHash == "" {
		t.Fatal("expected provenance hash")
	}
	if obj.ProvenanceHash != obj.ComputeProvenanceHash() {
		t.Fatal("expected deterministic provenance hash")
	}
}

func TestMemoryObjectNormalizeRejectsSecretLikeContent(t *testing.T) {
	obj := MemoryObject{
		Type:           "decision",
		Content:        "api_key=" + strings.Repeat("a", 24),
		SourceRefs:     []string{"adr:0009"},
		Classification: "product",
	}
	if err := obj.NormalizeAndValidate(time.Now()); err == nil {
		t.Fatal("expected secret-like content error")
	}
}

func TestMemoryObjectNormalizeRejectsInvalidType(t *testing.T) {
	obj := MemoryObject{
		Type:           "note",
		Content:        "Unsupported type",
		SourceRefs:     []string{"adr:0009"},
		Classification: "product",
	}
	if err := obj.NormalizeAndValidate(time.Now()); err == nil {
		t.Fatal("expected invalid type error")
	}
}
