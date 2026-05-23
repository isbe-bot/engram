package embedding

import "testing"

func TestHashProviderEmbedDeterministic(t *testing.T) {
	provider := NewHashProvider(16)
	a := provider.Embed("ENGRAM semantic memory")
	b := provider.Embed("ENGRAM semantic memory")
	if len(a) != 16 || len(b) != 16 {
		t.Fatalf("unexpected vector lengths: %d %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("expected deterministic vectors at index %d: %v vs %v", i, a[i], b[i])
		}
	}
}

func TestHashProviderEmbedEmpty(t *testing.T) {
	provider := NewHashProvider(8)
	vec := provider.Embed("")
	if len(vec) != 8 {
		t.Fatalf("unexpected vector length: %d", len(vec))
	}
	for _, v := range vec {
		if v != 0 {
			t.Fatalf("expected empty text to produce zero vector, got %+v", vec)
		}
	}
}
