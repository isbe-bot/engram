package policy

import "testing"

func TestValidateClassification(t *testing.T) {
	if err := ValidateClassification("product"); err != nil {
		t.Fatalf("expected product classification to be valid: %v", err)
	}
	if err := ValidateClassification("invalid"); err == nil {
		t.Fatal("expected invalid classification error")
	}
}

func TestValidateSourceRefs(t *testing.T) {
	if err := ValidateSourceRefs([]string{"adr:0009", "chat:123"}); err != nil {
		t.Fatalf("expected source refs to be valid: %v", err)
	}
	if err := ValidateSourceRefs([]string{"unknown:123"}); err == nil {
		t.Fatal("expected source ref prefix validation error")
	}
}
