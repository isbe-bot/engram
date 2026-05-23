package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"
)

//go:embed *.sql
var fs embed.FS

func Apply(db *sql.DB) error {
	entries, err := fs.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	for _, name := range files {
		if err := applyOne(db, name); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(db *sql.DB, name string) error {
	content, err := fs.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	version := strings.TrimSuffix(name, ".sql")
	checksum := fmt.Sprintf("len:%d", len(content))

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, version).Scan(&count); err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("check migration state: %w", err)
		}
	}

	if count > 0 {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit no-op migration tx: %w", err)
		}
		return nil
	}

	if _, err := tx.Exec(string(content)); err != nil {
		return fmt.Errorf("exec migration %s: %w", name, err)
	}

	if _, err := tx.Exec(`
		INSERT INTO schema_migrations (version, name, applied_at, checksum)
		VALUES (?, ?, ?, ?)
	`, version, name, time.Now().UTC().Format(time.RFC3339), checksum); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}
	return nil
}
