package models

import (
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
