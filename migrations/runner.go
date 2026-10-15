package migrations

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

// Run applies all migrations in order and records them in schema_migrations.
func Run(db *sqlx.DB) error {
	schemaTableQuery := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	if _, err := db.Exec(schemaTableQuery); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	files, err := getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to list migration files: %w", err)
	}

	for _, file := range files {
		var exists bool
		err := db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)", file.name)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", file.name, err)
		}

		if exists {
			continue
		}

		log.Printf("Applying migration: %s", file.name)

		tx, err := db.Beginx()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", file.name, err)
		}

		if _, err := tx.Exec(file.content); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", file.name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (filename) VALUES ($1)", file.name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", file.name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", file.name, err)
		}

		log.Printf("Successfully applied migration: %s", file.name)
	}

	return nil
}

type migrationEntry struct {
	name    string
	content string
}

func getMigrationFiles() ([]migrationEntry, error) {
	// 1. Try disk migrations folder
	candidates := []string{"migrations", "../migrations", "../../migrations"}
	var diskDir string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			diskDir = c
			break
		}
	}

	if diskDir != "" {
		entries, err := os.ReadDir(diskDir)
		if err == nil {
			var names []string
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
					names = append(names, e.Name())
				}
			}
			sort.Strings(names)

			var result []migrationEntry
			for _, name := range names {
				content, err := os.ReadFile(filepath.Join(diskDir, name))
				if err != nil {
					return nil, err
				}
				result = append(result, migrationEntry{
					name:    name,
					content: string(content),
				})
			}
			if len(result) > 0 {
				return result, nil
			}
		}
	}

	// 2. Fall back to embedded MigrationFS
	entries, err := MigrationFS.ReadDir(".")
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var result []migrationEntry
	for _, name := range names {
		content, err := MigrationFS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		result = append(result, migrationEntry{
			name:    name,
			content: string(content),
		})
	}

	return result, nil
}
