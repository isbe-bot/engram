package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/isbe-bot/engram/internal/policy"
)

const (
	MemoryStatusAccepted   = "accepted"
	MemoryStatusDeprecated = "deprecated"
)

type MemoryObject struct {
	ObjectID       string   `json:"object_id"`
	Type           string   `json:"type"`
	SchemaVer      string   `json:"schema_version"`
	Content        string   `json:"content"`
	SourceRefs     []string `json:"source_refs"`
	Confidence     float64  `json:"confidence"`
	Classification string   `json:"classification"`
	Scope          string   `json:"scope"`
	ProvenanceHash string   `json:"provenance_hash"`
	Status         string   `json:"status"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

func (m *MemoryObject) NormalizeAndValidate(now time.Time) error {
	m.ObjectID = strings.TrimSpace(m.ObjectID)
	m.Type = strings.TrimSpace(m.Type)
	m.SchemaVer = strings.TrimSpace(m.SchemaVer)
	m.Content = strings.TrimSpace(m.Content)
	m.Classification = strings.TrimSpace(strings.ToLower(m.Classification))
	m.Scope = strings.TrimSpace(strings.ToLower(m.Scope))
	m.ProvenanceHash = strings.TrimSpace(m.ProvenanceHash)
	m.Status = strings.TrimSpace(m.Status)

	if m.Type == "" {
		return fmt.Errorf("type is required")
	}
	m.Type = strings.TrimSpace(strings.ToLower(m.Type))
	if err := policy.ValidateType(m.Type); err != nil {
		return err
	}
	if m.Content == "" {
		return fmt.Errorf("content is required")
	}
	if err := policy.EnsureNoSecretLikeText(m.Content); err != nil {
		return err
	}

	refs := make([]string, 0, len(m.SourceRefs))
	for _, ref := range m.SourceRefs {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	m.SourceRefs = refs
	if err := policy.ValidateSourceRefs(m.SourceRefs); err != nil {
		return err
	}

	if m.SchemaVer == "" {
		m.SchemaVer = "v1"
	}
	if m.Classification == "" {
		m.Classification = "general"
	}
	if err := policy.ValidateClassification(m.Classification); err != nil {
		return err
	}
	if m.Scope == "" {
		m.Scope = "local"
	}
	if err := policy.ValidateScope(m.Scope); err != nil {
		return err
	}
	if m.ProvenanceHash == "" {
		m.ProvenanceHash = m.ComputeProvenanceHash()
	}
	if m.Status == "" {
		m.Status = MemoryStatusAccepted
	}
	if m.Confidence <= 0 || m.Confidence > 1 {
		m.Confidence = 0.7
	}

	nowTS := now.UTC().Format(time.RFC3339)
	if m.CreatedAt == "" {
		m.CreatedAt = nowTS
	}
	if m.UpdatedAt == "" {
		m.UpdatedAt = nowTS
	}

	return nil
}

func (m MemoryObject) ComputeProvenanceHash() string {
	refs := append([]string(nil), m.SourceRefs...)
	for i := range refs {
		refs[i] = strings.TrimSpace(refs[i])
	}
	raw := strings.Join([]string{
		strings.TrimSpace(strings.ToLower(m.Type)),
		strings.TrimSpace(m.SchemaVer),
		strings.TrimSpace(m.Content),
		strings.Join(refs, ","),
		strings.TrimSpace(strings.ToLower(m.Classification)),
		strings.TrimSpace(strings.ToLower(m.Scope)),
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
