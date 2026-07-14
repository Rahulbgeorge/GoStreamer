package sqlite

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Connect opens a SQLite connection, enforces foreign keys, enables WAL mode, and runs migrations.
func Connect(dbPath string, migrationSQL string) (*sql.DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create database directory %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %s: %w", dbPath, err)
	}

	// Optimize SQLite performance for our concurrent write operations
	// WAL mode enables concurrent reads while a write transaction is executing.
	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	_, err = db.Exec("PRAGMA foreign_keys=ON;")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// Run migrations
	if migrationSQL != "" {
		slog.Debug("Running database migrations...")
		if _, err := db.Exec(migrationSQL); err != nil {
			db.Close()
			return nil, fmt.Errorf("run schema migrations: %w", err)
		}
		slog.Info("Database migrations completed successfully")
	}

	return db, nil
}
