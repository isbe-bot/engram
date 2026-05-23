package policy

import (
	"fmt"
	"strings"
)

var allowedTypes = map[string]struct{}{
	"fact":             {},
	"preference":       {},
	"decision":         {},
	"project_context":  {},
	"contact_context":  {},
	"workflow_pattern": {},
	"lesson":           {},
	"report_insight":   {},
}

var allowedScopes = map[string]struct{}{
	"local":   {},
	"agent":   {},
	"project": {},
	"client":  {},
	"global":  {},
}

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

func ValidateType(typeName string) error {
	t := strings.TrimSpace(strings.ToLower(typeName))
	if t == "" {
		return fmt.Errorf("type is required")
	}
	if _, ok := allowedTypes[t]; !ok {
		return fmt.Errorf("type is not allowed: %s", typeName)
	}
	return nil
}

func ValidateScope(scope string) error {
	s := strings.TrimSpace(strings.ToLower(scope))
	if s == "" {
		return fmt.Errorf("scope is required")
	}
	if _, ok := allowedScopes[s]; !ok {
		return fmt.Errorf("scope is not allowed: %s", scope)
	}
	return nil
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
