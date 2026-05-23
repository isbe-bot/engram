package citations

import "testing"

func TestIDDeterministic(t *testing.T) {
	a := ID("memory_object", "memory_objects/mem-1")
	b := ID("memory_object", "memory_objects/mem-1")
	if a != b {
		t.Fatalf("expected deterministic IDs, got %s vs %s", a, b)
	}
	c := ID("source_ref", "adr:0009")
	if c == a {
		t.Fatalf("expected different IDs for different citation payloads")
	}
}
