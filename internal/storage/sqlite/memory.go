package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/isbe-bot/engram/internal/citations"
	"github.com/isbe-bot/engram/internal/models"
	"github.com/isbe-bot/engram/internal/retrieve"
	"github.com/isbe-bot/engram/pkg/contracts"
)

func (s *Store) CreateMemoryObject(ctx context.Context, m models.MemoryObject, env contracts.MutationEnvelope) (models.MemoryObject, error) {
	if s == nil || s.db == nil {
		return models.MemoryObject{}, fmt.Errorf("sqlite store is not initialized")
	}
	if err := m.NormalizeAndValidate(time.Now().UTC()); err != nil {
		return models.MemoryObject{}, err
	}

	sourceRefsJSON, err := json.Marshal(m.SourceRefs)
	if err != nil {
		return models.MemoryObject{}, fmt.Errorf("marshal source refs: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO memory_objects (
			object_id, type, schema_version, content, source_refs_json,
			confidence, classification, scope, provenance_hash, status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ObjectID, m.Type, m.SchemaVer, m.Content, string(sourceRefsJSON), m.Confidence, m.Classification, m.Scope, m.ProvenanceHash, m.Status, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return models.MemoryObject{}, fmt.Errorf("memory object already exists: %s", m.ObjectID)
		}
		return models.MemoryObject{}, fmt.Errorf("create memory object: %w", err)
	}

	if err := s.appendMemoryObjectEvent(ctx, m.ObjectID, "curated", "", map[string]any{"type": m.Type}, env); err != nil {
		return models.MemoryObject{}, err
	}

	return m, nil
}

func (s *Store) CorrectMemoryObject(ctx context.Context, objectID, content, reason string, sourceRefs []string, env contracts.MutationEnvelope) (models.MemoryObject, error) {
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
	obj.ProvenanceHash = obj.ComputeProvenanceHash()
	obj.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	sourceRefsJSON, err := json.Marshal(obj.SourceRefs)
	if err != nil {
		return models.MemoryObject{}, fmt.Errorf("marshal source refs: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE memory_objects
		SET content = ?, source_refs_json = ?, provenance_hash = ?, updated_at = ?
		WHERE object_id = ?
	`, obj.Content, string(sourceRefsJSON), obj.ProvenanceHash, obj.UpdatedAt, obj.ObjectID)
	if err != nil {
		return models.MemoryObject{}, fmt.Errorf("correct memory object: %w", err)
	}

	if err := s.appendMemoryObjectEvent(ctx, obj.ObjectID, "corrected", reason, map[string]any{"source_refs": obj.SourceRefs}, env); err != nil {
		return models.MemoryObject{}, err
	}

	return obj, nil
}

func (s *Store) DeprecateMemoryObject(ctx context.Context, objectID, reason string, env contracts.MutationEnvelope) (models.MemoryObject, error) {
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

	if err := s.appendMemoryObjectEvent(ctx, obj.ObjectID, "deprecated", reason, map[string]any{}, env); err != nil {
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
		       confidence, classification, scope, provenance_hash, status, created_at, updated_at
		FROM memory_objects
		WHERE object_id = ?
	`, objectID).Scan(&obj.ObjectID, &obj.Type, &obj.SchemaVer, &obj.Content, &sourceRefsJSON, &obj.Confidence, &obj.Classification, &obj.Scope, &obj.ProvenanceHash, &obj.Status, &obj.CreatedAt, &obj.UpdatedAt); err != nil {
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

func (s *Store) SearchMemoryObjects(ctx context.Context, q retrieve.Query) ([]map[string]any, string, error) {
	if s == nil || s.db == nil {
		return nil, "", fmt.Errorf("sqlite store is not initialized")
	}

	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := 0
	if strings.TrimSpace(q.Cursor) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(q.Cursor)); err == nil && parsed >= 0 {
			offset = parsed
		}
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
		       confidence, classification, scope, provenance_hash, status, created_at, updated_at
		FROM memory_objects
	`
	if len(filters) > 0 {
		sqlText += " WHERE " + strings.Join(filters, " AND ")
	}
	queryLimit := limit + 1
	sqlText += ` ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	args = append(args, queryLimit, offset)

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, "", fmt.Errorf("search memory objects: %w", err)
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
			scope          string
			provenanceHash string
			status         string
			createdAt      string
			updatedAt      string
			sourceRefs     []string
		)
		if err := rows.Scan(&objID, &typeName, &schemaVersion, &content, &sourceRefsJSON, &confidence, &classification, &scope, &provenanceHash, &status, &createdAt, &updatedAt); err != nil {
			return nil, "", fmt.Errorf("scan memory object result: %w", err)
		}
		if sourceRefsJSON != "" {
			_ = json.Unmarshal([]byte(sourceRefsJSON), &sourceRefs)
		}
		if sourceRefs == nil {
			sourceRefs = []string{}
		}

		citationsList := make([]map[string]any, 0, len(sourceRefs)+1)
		citationsList = append(citationsList, citations.Make("memory_object", "memory_objects/"+objID))
		for _, ref := range sourceRefs {
			citationsList = append(citationsList, citations.Make("source_ref", ref))
		}

		results = append(results, map[string]any{
			"object_id":        objID,
			"type":             typeName,
			"schema_version":   schemaVersion,
			"content":          content,
			"source_refs":      sourceRefs,
			"confidence":       confidence,
			"classification":   classification,
			"scope":            scope,
			"provenance_hash":  provenanceHash,
			"status":           status,
			"created_at":       createdAt,
			"updated_at":       updatedAt,
			"citations":        citationsList,
			"retrieval_source": "memory_objects",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate memory object results: %w", err)
	}

	nextCursor := ""
	if len(results) > limit {
		results = results[:limit]
		nextCursor = strconv.Itoa(offset + len(results))
	}

	return results, nextCursor, nil
}

func (s *Store) ListMemoryObjects(ctx context.Context, limit int) ([]models.MemoryObject, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store is not initialized")
	}
	if limit <= 0 || limit > 10000 {
		limit = 10000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT object_id, type, schema_version, content, source_refs_json,
		       confidence, classification, scope, provenance_hash, status, created_at, updated_at
		FROM memory_objects
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list memory objects: %w", err)
	}
	defer rows.Close()

	objects := make([]models.MemoryObject, 0)
	for rows.Next() {
		var obj models.MemoryObject
		var sourceRefsJSON string
		if err := rows.Scan(&obj.ObjectID, &obj.Type, &obj.SchemaVer, &obj.Content, &sourceRefsJSON, &obj.Confidence, &obj.Classification, &obj.Scope, &obj.ProvenanceHash, &obj.Status, &obj.CreatedAt, &obj.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory object: %w", err)
		}
		if sourceRefsJSON != "" {
			_ = json.Unmarshal([]byte(sourceRefsJSON), &obj.SourceRefs)
		}
		if obj.SourceRefs == nil {
			obj.SourceRefs = []string{}
		}
		objects = append(objects, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory objects: %w", err)
	}
	return objects, nil
}

func (s *Store) ListMemoryObjectEvents(ctx context.Context, objectID, action string, beforeID, limit int) ([]map[string]any, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store is not initialized")
	}
	objectID = strings.TrimSpace(objectID)
	if objectID == "" {
		return nil, fmt.Errorf("object_id is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	filters := []string{"object_id = ?"}
	args := []any{objectID}
	if strings.TrimSpace(action) != "" {
		filters = append(filters, "action = ?")
		args = append(args, strings.TrimSpace(action))
	}
	if beforeID > 0 {
		filters = append(filters, "id < ?")
		args = append(args, beforeID)
	}

	sqlText := `
		SELECT id, action, reason, payload_json, created_at, actor_id, mutation_id, signature, prev_hash, event_hash
		FROM memory_object_events
		WHERE ` + strings.Join(filters, " AND ") + `
		ORDER BY id DESC
		LIMIT ?
	`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("list memory object events: %w", err)
	}
	defer rows.Close()

	events := make([]map[string]any, 0)
	for rows.Next() {
		var (
			id         int
			actionName string
			reason     string
			payload    string
			createdAt  string
			actorID    string
			mutationID string
			signature  string
			prevHash   string
			eventHash  string
			payloadMap map[string]any
		)
		if err := rows.Scan(&id, &actionName, &reason, &payload, &createdAt, &actorID, &mutationID, &signature, &prevHash, &eventHash); err != nil {
			return nil, fmt.Errorf("scan memory object event: %w", err)
		}
		if payload != "" {
			_ = json.Unmarshal([]byte(payload), &payloadMap)
		}
		if payloadMap == nil {
			payloadMap = map[string]any{}
		}
		events = append(events, map[string]any{
			"id":          id,
			"object_id":   objectID,
			"action":      actionName,
			"reason":      reason,
			"payload":     payloadMap,
			"created_at":  createdAt,
			"actor_id":    actorID,
			"mutation_id": mutationID,
			"signature":   signature,
			"prev_hash":   prevHash,
			"event_hash":  eventHash,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory object events: %w", err)
	}

	return events, nil
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

func (s *Store) appendMemoryObjectEvent(ctx context.Context, objectID, action, reason string, payload map[string]any, env contracts.MutationEnvelope) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal memory object event payload: %w", err)
	}

	prevHash := ""
	_ = s.db.QueryRowContext(ctx, `SELECT event_hash FROM memory_object_events WHERE object_id = ? ORDER BY id DESC LIMIT 1`, objectID).Scan(&prevHash)

	createdAt := time.Now().UTC().Format(time.RFC3339)
	raw := strings.Join([]string{
		strings.TrimSpace(objectID),
		strings.TrimSpace(action),
		strings.TrimSpace(reason),
		string(payloadJSON),
		strings.TrimSpace(env.ActorID),
		strings.TrimSpace(env.MutationID),
		strings.TrimSpace(env.Signature),
		createdAt,
		strings.TrimSpace(prevHash),
	}, "|")
	h := sha256.Sum256([]byte(raw))
	eventHash := hex.EncodeToString(h[:])

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO memory_object_events (
			object_id, action, reason, payload_json, created_at,
			actor_id, mutation_id, signature, prev_hash, event_hash
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, objectID, action, reason, string(payloadJSON), createdAt, env.ActorID, env.MutationID, env.Signature, prevHash, eventHash)
	if err != nil {
		return fmt.Errorf("insert memory object event: %w", err)
	}
	return nil
}
