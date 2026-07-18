package main

import (
	"database/sql"
	"log/slog"
	"os"

	"streamingplayer/internal/config"
	"streamingplayer/internal/repository/sqlite"
)

// setupDatabase loads and runs SQL migrations, then connects to the SQLite database.
func setupDatabase(cfg *config.Config) (*sql.DB, error) {
	// Read our migration schema
	migrationPaths := []string{
		"migrations/001_create_media.sql",
		"../../migrations/001_create_media.sql",
		"../migrations/001_create_media.sql",
		"backend/migrations/001_create_media.sql",
	}
	var migrationSQL []byte
	var err error
	for _, path := range migrationPaths {
		migrationSQL, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		slog.Warn("Migrations script not found, scanning fallback setup", "err", err)
	}

	// Try to read the genre migration as well
	genreMigrationPaths := []string{
		"migrations/002_add_genre.sql",
		"../../migrations/002_add_genre.sql",
		"../migrations/002_add_genre.sql",
		"backend/migrations/002_add_genre.sql",
	}
	var genreMigrationSQL []byte
	for _, path := range genreMigrationPaths {
		genreMigrationSQL, _ = os.ReadFile(path)
		if genreMigrationSQL != nil {
			break
		}
	}

	// Try to read the preferences migration as well
	preferencesMigrationPaths := []string{
		"migrations/003_create_preferences.sql",
		"../../migrations/003_create_preferences.sql",
		"../migrations/003_create_preferences.sql",
		"backend/migrations/003_create_preferences.sql",
	}
	var preferencesMigrationSQL []byte
	for _, path := range preferencesMigrationPaths {
		preferencesMigrationSQL, _ = os.ReadFile(path)
		if preferencesMigrationSQL != nil {
			break
		}
	}

	db, err := sqlite.Connect(cfg.DatabasePath, string(migrationSQL))
	if err != nil {
		return nil, err
	}

	// Run additional migrations (ignore errors for already-applied ones)
	if len(genreMigrationSQL) > 0 {
		if _, migErr := db.Exec(string(genreMigrationSQL)); migErr != nil {
			slog.Debug("Genre migration skipped (likely already applied)", "err", migErr)
		}
	}

	if len(preferencesMigrationSQL) > 0 {
		if _, migErr := db.Exec(string(preferencesMigrationSQL)); migErr != nil {
			slog.Debug("Preferences migration skipped (likely already applied)", "err", migErr)
		}
	}

	return db, nil
}
