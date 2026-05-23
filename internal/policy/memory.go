package policy

import (
	"fmt"
	"strings"
)

var allowedClassifications = map[string]struct{}{
	"general":       {},
	"product":       {},
	"communication": {},
	"security":      {},
	"operations":    {},
}

var allowedSourceRefPrefixes = []string{
	"adr:",
	"chat:",
	"spec:",
	"meeting:",
	"task:",
	"event:",
	"doc:",
}

func ValidateClassification(classification string) error {
	c := strings.TrimSpace(strings.ToLower(classification))
	if c == "" {
		return fmt.Errorf("classification is required")
	}
	if _, ok := allowedClassifications[c]; !ok {
		return fmt.Errorf("classification is not allowed: %s", classification)
	}
	return nil
}

func ValidateSourceRefs(sourceRefs []string) error {
	if len(sourceRefs) == 0 {
		return fmt.Errorf("source_refs must include at least one reference")
	}
	for _, ref := range sourceRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return fmt.Errorf("source_refs cannot include blank entries")
		}
		ok := false
		for _, p := range allowedSourceRefPrefixes {
			if strings.HasPrefix(strings.ToLower(ref), p) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("source_ref prefix is not allowed: %s", ref)
		}
	}
	return nil
}
