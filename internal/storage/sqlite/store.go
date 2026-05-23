package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/isbe-bot/engram/internal/events"
	"github.com/isbe-bot/engram/internal/migrations"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) ApplyMigrations() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store is not initialized")
	}
	return migrations.Apply(s.db)
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store is not initialized")
	}
	return s.db.PingContext(ctx)
}

func (s *Store) InsertEvent(ctx context.Context, env events.Envelope) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store is not initialized")
	}

	dataJSON, err := json.Marshal(env.Data)
	if err != nil {
		return fmt.Errorf("marshal event data: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ingested_events (event_id, event_type, environment_id, occurred_at, data_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, env.EventID, env.EventType, env.EnvironmentID, env.OccurredAt, string(dataJSON), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("event already exists: %s", env.EventID)
		}
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (s *Store) SearchEvents(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store is not initialized")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	q := strings.TrimSpace(query)
	var (
		rows *sql.Rows
		err  error
	)
	if q == "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT event_id, event_type, environment_id, occurred_at, created_at, data_json
			FROM ingested_events
			ORDER BY occurred_at DESC
			LIMIT ?
		`, limit)
	} else {
		like := "%" + q + "%"
		rows, err = s.db.QueryContext(ctx, `
			SELECT event_id, event_type, environment_id, occurred_at, created_at, data_json
			FROM ingested_events
			WHERE event_type LIKE ? OR environment_id LIKE ? OR data_json LIKE ?
			ORDER BY occurred_at DESC
			LIMIT ?
		`, like, like, like, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("search events: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	for rows.Next() {
		var (
			eventID       string
			eventType     string
			environmentID string
			occurredAt    string
			createdAt     string
			dataStr       string
			data          map[string]any
		)
		if err := rows.Scan(&eventID, &eventType, &environmentID, &occurredAt, &createdAt, &dataStr); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		if dataStr != "" {
			_ = json.Unmarshal([]byte(dataStr), &data)
		}
		if data == nil {
			data = map[string]any{}
		}
		results = append(results, map[string]any{
			"event_id":       eventID,
			"event_type":     eventType,
			"environment_id": environmentID,
			"occurred_at":    occurredAt,
			"created_at":     createdAt,
			"data":           data,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}

	return results, nil
}

func (s *Store) EventCount(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("sqlite store is not initialized")
	}
	var c int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM ingested_events`).Scan(&c); err != nil {
		return 0, fmt.Errorf("count events: %w", err)
	}
	return c, nil
}
