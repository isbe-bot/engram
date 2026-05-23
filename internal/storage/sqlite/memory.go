package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aileun/engram/internal/models"
	"github.com/aileun/engram/internal/retrieve"
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

func (s *Store) SearchMemoryObjects(ctx context.Context, q retrieve.Query) ([]map[string]any, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store is not initialized")
	}

	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	filters := make([]string, 0)
	args := make([]any, 0)

	if text := strings.TrimSpace(q.Text); text != "" {
		like := "%" + text + "%"
		filters = append(filters, `(type LIKE ? OR content LIKE ? OR classification LIKE ? OR source_refs_json LIKE ?)`)
		args = append(args, like, like, like, like)
	}
	if status := strings.TrimSpace(q.Status); status != "" {
		filters = append(filters, `status = ?`)
		args = append(args, status)
	}
	if q.MinConfidence > 0 {
		filters = append(filters, `confidence >= ?`)
		args = append(args, q.MinConfidence)
	}

	sqlText := `
		SELECT object_id, type, schema_version, content, source_refs_json,
		       confidence, classification, status, created_at, updated_at
		FROM memory_objects
	`
	if len(filters) > 0 {
		sqlText += " WHERE " + strings.Join(filters, " AND ")
	}
	sqlText += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("search memory objects: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	for rows.Next() {
		var (
			objID          string
			typeName       string
			schemaVersion  string
			content        string
			sourceRefsJSON string
			confidence     float64
			classification string
			status         string
			createdAt      string
			updatedAt      string
			sourceRefs     []string
		)
		if err := rows.Scan(&objID, &typeName, &schemaVersion, &content, &sourceRefsJSON, &confidence, &classification, &status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan memory object result: %w", err)
		}
		if sourceRefsJSON != "" {
			_ = json.Unmarshal([]byte(sourceRefsJSON), &sourceRefs)
		}
		if sourceRefs == nil {
			sourceRefs = []string{}
		}

		citations := make([]map[string]any, 0, len(sourceRefs)+1)
		citations = append(citations, map[string]any{
			"kind": "memory_object",
			"path": "memory_objects/" + objID,
		})
		for _, ref := range sourceRefs {
			citations = append(citations, map[string]any{"kind": "source_ref", "path": ref})
		}

		results = append(results, map[string]any{
			"object_id":        objID,
			"type":             typeName,
			"schema_version":   schemaVersion,
			"content":          content,
			"source_refs":      sourceRefs,
			"confidence":       confidence,
			"classification":   classification,
			"status":           status,
			"created_at":       createdAt,
			"updated_at":       updatedAt,
			"citations":        citations,
			"retrieval_source": "memory_objects",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory object results: %w", err)
	}

	return results, nil
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
