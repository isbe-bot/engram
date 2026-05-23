package policy

import (
	"fmt"
	"regexp"
	"strings"
)

type SecretFinding struct {
	Kind string
}

var secretPatterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	{kind: "private_key", re: regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{kind: "github_token", re: regexp.MustCompile(`\bghp_[A-Za-z0-9_]{20,}\b`)},
	{kind: "github_fine_grained_token", re: regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`)},
	{kind: "openai_api_key", re: regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`)},
	{kind: "slack_token", re: regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{20,}\b`)},
	{kind: "aws_access_key", re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{kind: "secret_assignment", re: regexp.MustCompile(`(?i)\b(password|api[_-]?key|secret|token)\s*[:=]\s*["']?[^"'\s]{8,}`)},
}

func DetectSecretLikeText(text string) []SecretFinding {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	findings := make([]SecretFinding, 0)
	seen := map[string]struct{}{}
	for _, p := range secretPatterns {
		if p.re.MatchString(trimmed) {
			if _, ok := seen[p.kind]; ok {
				continue
			}
			seen[p.kind] = struct{}{}
			findings = append(findings, SecretFinding{Kind: p.kind})
		}
	}
	return findings
}

func EnsureNoSecretLikeText(text string) error {
	findings := DetectSecretLikeText(text)
	if len(findings) == 0 {
		return nil
	}
	kinds := make([]string, 0, len(findings))
	for _, f := range findings {
		kinds = append(kinds, f.Kind)
	}
	return fmt.Errorf("secret-like content rejected: %s", strings.Join(kinds, ","))
}
