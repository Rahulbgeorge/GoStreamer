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

	// Try to read the downloads migration as well
	downloadsMigrationPaths := []string{
		"migrations/004_create_downloads.sql",
		"../../migrations/004_create_downloads.sql",
		"../migrations/004_create_downloads.sql",
		"backend/migrations/004_create_downloads.sql",
	}
	var downloadsMigrationSQL []byte
	for _, path := range downloadsMigrationPaths {
		downloadsMigrationSQL, _ = os.ReadFile(path)
		if downloadsMigrationSQL != nil {
			break
		}
	}

	// Try to read clips & categories migration
	clipsCategoriesMigrationPaths := []string{
		"migrations/006_clips_and_categories.sql",
		"../../migrations/006_clips_and_categories.sql",
		"../migrations/006_clips_and_categories.sql",
		"backend/migrations/006_clips_and_categories.sql",
	}
	var clipsCategoriesMigrationSQL []byte
	for _, path := range clipsCategoriesMigrationPaths {
		clipsCategoriesMigrationSQL, _ = os.ReadFile(path)
		if clipsCategoriesMigrationSQL != nil {
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

	if len(downloadsMigrationSQL) > 0 {
		if _, migErr := db.Exec(string(downloadsMigrationSQL)); migErr != nil {
			slog.Debug("Downloads migration skipped (likely already applied)", "err", migErr)
		}
	}

	if len(clipsCategoriesMigrationSQL) > 0 {
		if _, migErr := db.Exec(string(clipsCategoriesMigrationSQL)); migErr != nil {
			slog.Debug("Clips & Categories migration skipped (likely already applied)", "err", migErr)
		}
	}

	// Try to read media_categories migration
	mediaCategoriesMigrationPaths := []string{
		"migrations/007_media_categories.sql",
		"../../migrations/007_media_categories.sql",
		"../migrations/007_media_categories.sql",
		"backend/migrations/007_media_categories.sql",
	}
	for _, path := range mediaCategoriesMigrationPaths {
		if sqlBytes, readErr := os.ReadFile(path); readErr == nil && len(sqlBytes) > 0 {
			if _, migErr := db.Exec(string(sqlBytes)); migErr != nil {
				slog.Debug("Media Categories migration skipped (likely already applied)", "err", migErr)
			}
			break
		}
	}

	// Try to read users & watch history migration
	usersMigrationPaths := []string{
		"migrations/008_users_and_watch_history.sql",
		"../../migrations/008_users_and_watch_history.sql",
		"../migrations/008_users_and_watch_history.sql",
		"backend/migrations/008_users_and_watch_history.sql",
	}
	for _, path := range usersMigrationPaths {
		if sqlBytes, readErr := os.ReadFile(path); readErr == nil && len(sqlBytes) > 0 {
			if _, migErr := db.Exec(string(sqlBytes)); migErr != nil {
				slog.Debug("Users & Watch history migration skipped (likely already applied)", "err", migErr)
			}
			break
		}
	}

	// Always ensure last_position and default_start_time columns exist on media table
	_, _ = db.Exec(`ALTER TABLE media ADD COLUMN last_position INTEGER NOT NULL DEFAULT 0;`)
	_, _ = db.Exec(`ALTER TABLE media ADD COLUMN default_start_time INTEGER NOT NULL DEFAULT 0;`)

	return db, nil
}
