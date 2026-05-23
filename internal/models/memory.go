package models

import (
	"fmt"
	"strings"
	"time"
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
	Status         string   `json:"status"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

func (m *MemoryObject) NormalizeAndValidate(now time.Time) error {
	m.ObjectID = strings.TrimSpace(m.ObjectID)
	m.Type = strings.TrimSpace(m.Type)
	m.SchemaVer = strings.TrimSpace(m.SchemaVer)
	m.Content = strings.TrimSpace(m.Content)
	m.Classification = strings.TrimSpace(m.Classification)
	m.Status = strings.TrimSpace(m.Status)

	if m.Type == "" {
		return fmt.Errorf("type is required")
	}
	if m.Content == "" {
		return fmt.Errorf("content is required")
	}

	refs := make([]string, 0, len(m.SourceRefs))
	for _, ref := range m.SourceRefs {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	m.SourceRefs = refs
	if len(m.SourceRefs) == 0 {
		return fmt.Errorf("source_refs must include at least one reference")
	}

	if m.SchemaVer == "" {
		m.SchemaVer = "v1"
	}
	if m.Classification == "" {
		m.Classification = "general"
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
