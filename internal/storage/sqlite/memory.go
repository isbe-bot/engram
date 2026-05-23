package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aileun/engram/internal/models"
)

func (s *Store) CreateMemoryObject(ctx context.Context, m models.MemoryObject) (models.MemoryObject, error) {
	if s == nil || s.db == nil {
		return models.MemoryObject{}, fmt.Errorf("sqlite store is not initialized")
	}

	sourceRefsJSON, err := json.Marshal(m.SourceRefs)
	if err != nil {
		return models.MemoryObject{}, fmt.Errorf("marshal source refs: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO memory_objects (
			object_id, type, schema_version, content, source_refs_json,
			confidence, classification, status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ObjectID, m.Type, m.SchemaVer, m.Content, string(sourceRefsJSON), m.Confidence, m.Classification, m.Status, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return models.MemoryObject{}, fmt.Errorf("memory object already exists: %s", m.ObjectID)
		}
		return models.MemoryObject{}, fmt.Errorf("create memory object: %w", err)
	}

	if err := s.appendMemoryObjectEvent(ctx, m.ObjectID, "curated", "", map[string]any{"type": m.Type}); err != nil {
		return models.MemoryObject{}, err
	}

	return m, nil
}

func (s *Store) CorrectMemoryObject(ctx context.Context, objectID, content, reason string, sourceRefs []string) (models.MemoryObject, error) {
	obj, err := s.GetMemoryObject(ctx, objectID)
	if err != nil {
		return models.MemoryObject{}, err
	}
	if strings.TrimSpace(content) == "" {
		return models.MemoryObject{}, fmt.Errorf("content is required")
	}
	if strings.TrimSpace(reason) == "" {
		return models.MemoryObject{}, fmt.Errorf("reason is required")
	}

	obj.Content = strings.TrimSpace(content)
	if len(sourceRefs) > 0 {
		clean := make([]string, 0, len(sourceRefs))
		for _, ref := range sourceRefs {
			ref = strings.TrimSpace(ref)
			if ref != "" {
				clean = append(clean, ref)
			}
		}
		if len(clean) > 0 {
			obj.SourceRefs = clean
		}
	}
	obj.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	sourceRefsJSON, err := json.Marshal(obj.SourceRefs)
	if err != nil {
		return models.MemoryObject{}, fmt.Errorf("marshal source refs: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE memory_objects
		SET content = ?, source_refs_json = ?, updated_at = ?
		WHERE object_id = ?
	`, obj.Content, string(sourceRefsJSON), obj.UpdatedAt, obj.ObjectID)
	if err != nil {
		return models.MemoryObject{}, fmt.Errorf("correct memory object: %w", err)
	}

	if err := s.appendMemoryObjectEvent(ctx, obj.ObjectID, "corrected", reason, map[string]any{"source_refs": obj.SourceRefs}); err != nil {
		return models.MemoryObject{}, err
	}

	return obj, nil
}

func (s *Store) DeprecateMemoryObject(ctx context.Context, objectID, reason string) (models.MemoryObject, error) {
	obj, err := s.GetMemoryObject(ctx, objectID)
	if err != nil {
		return models.MemoryObject{}, err
	}
	if strings.TrimSpace(reason) == "" {
		return models.MemoryObject{}, fmt.Errorf("reason is required")
	}

	obj.Status = models.MemoryStatusDeprecated
	obj.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.ExecContext(ctx, `
		UPDATE memory_objects
		SET status = ?, updated_at = ?
		WHERE object_id = ?
	`, obj.Status, obj.UpdatedAt, obj.ObjectID)
	if err != nil {
		return models.MemoryObject{}, fmt.Errorf("deprecate memory object: %w", err)
	}

	if err := s.appendMemoryObjectEvent(ctx, obj.ObjectID, "deprecated", reason, map[string]any{}); err != nil {
		return models.MemoryObject{}, err
	}

	return obj, nil
}

func (s *Store) GetMemoryObject(ctx context.Context, objectID string) (models.MemoryObject, error) {
	if s == nil || s.db == nil {
		return models.MemoryObject{}, fmt.Errorf("sqlite store is not initialized")
	}

	objectID = strings.TrimSpace(objectID)
	if objectID == "" {
		return models.MemoryObject{}, fmt.Errorf("object_id is required")
	}

	var (
		obj            models.MemoryObject
		sourceRefsJSON string
	)
	if err := s.db.QueryRowContext(ctx, `
		SELECT object_id, type, schema_version, content, source_refs_json,
		       confidence, classification, status, created_at, updated_at
		FROM memory_objects
		WHERE object_id = ?
	`, objectID).Scan(&obj.ObjectID, &obj.Type, &obj.SchemaVer, &obj.Content, &sourceRefsJSON, &obj.Confidence, &obj.Classification, &obj.Status, &obj.CreatedAt, &obj.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return models.MemoryObject{}, fmt.Errorf("memory object not found: %s", objectID)
		}
		return models.MemoryObject{}, fmt.Errorf("get memory object: %w", err)
	}
	if sourceRefsJSON != "" {
		_ = json.Unmarshal([]byte(sourceRefsJSON), &obj.SourceRefs)
	}
	if obj.SourceRefs == nil {
		obj.SourceRefs = []string{}
	}

	return obj, nil
}

func (s *Store) MemoryObjectCount(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("sqlite store is not initialized")
	}
	var c int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM memory_objects`).Scan(&c); err != nil {
		return 0, fmt.Errorf("count memory objects: %w", err)
	}
	return c, nil
}

func (s *Store) appendMemoryObjectEvent(ctx context.Context, objectID, action, reason string, payload map[string]any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal memory object event payload: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO memory_object_events (object_id, action, reason, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, objectID, action, reason, string(payloadJSON), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert memory object event: %w", err)
	}
	return nil
}
