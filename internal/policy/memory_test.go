package policy

import (
	"strings"
	"testing"
)

func TestValidateType(t *testing.T) {
	if err := ValidateType("decision"); err != nil {
		t.Fatalf("expected decision type to be valid: %v", err)
	}
	if err := ValidateType("random_note"); err == nil {
		t.Fatal("expected invalid type error")
	}
}

func TestValidateScope(t *testing.T) {
	if err := ValidateScope("client"); err != nil {
		t.Fatalf("expected client scope to be valid: %v", err)
	}
	if err := ValidateScope("internet"); err == nil {
		t.Fatal("expected invalid scope error")
	}
}

func TestValidateClassification(t *testing.T) {
	if err := ValidateClassification("product"); err != nil {
		t.Fatalf("expected product classification to be valid: %v", err)
	}
	if err := ValidateClassification("invalid"); err == nil {
		t.Fatal("expected invalid classification error")
	}
}

func TestDetectSecretLikeText(t *testing.T) {
	githubToken := "ghp_" + strings.Repeat("a", 24)
	findings := DetectSecretLikeText("token=" + githubToken)
	if len(findings) == 0 {
		t.Fatal("expected secret-like token finding")
	}
	if err := EnsureNoSecretLikeText("ordinary product decision with no credentials"); err != nil {
		t.Fatalf("did not expect secret finding: %v", err)
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
