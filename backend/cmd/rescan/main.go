package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/google/uuid"
	"streamingplayer/internal/config"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository/sqlite"
	"streamingplayer/internal/service"
	"streamingplayer/pkg/fileparser"
)

func main() {
	// Configure logger to print nicely to console
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, opts)))

	slog.Info("Starting Standalone Library Rescan Utility...")

	cfg := config.Load()

	// 1. Connect to SQLite database
	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to open sqlite database: %v", err)
	}
	defer db.Close()

	mediaRepo := sqlite.NewMediaRepository(db)
	prefRepo := sqlite.NewPreferenceRepository(db)

	// 2. Resolve media folder path
	mediaDir := cfg.MediaDir
	pref, err := prefRepo.Get("homedir")
	if err == nil && pref != nil && pref.Value != "" {
		mediaDir = pref.Value
	}

	slog.Info("Scanning media folder...", "directory", mediaDir)

	// 3. Walk directory and process allowed media files
	var filesToProcess []string
	err = filepath.WalkDir(mediaDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Ignore hidden files and .part files
		filename := d.Name()
		if strings.HasPrefix(filename, ".") || strings.HasSuffix(filename, ".part") {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		isAllowed := false
		for _, e := range cfg.AllowedExtensions {
			if strings.EqualFold(ext, e) {
				isAllowed = true
				break
			}
		}
		if isAllowed {
			filesToProcess = append(filesToProcess, path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("WalkDir failed: %v", err)
	}

	slog.Info(fmt.Sprintf("Found %d media files on disk. Verifying database records...", len(filesToProcess)))

	tasksCreated := 0

	for _, path := range filesToProcess {
		filename := filepath.Base(path)
		m, err := mediaRepo.FindByFilePath(path)
		if err != nil {
			slog.Error("Database lookup failed for file", "path", path, "err", err)
			continue
		}

		if m == nil {
			// Ingest new file
			slog.Info("Found untracked file on disk. Cataloging...", "file", filename)
			meta := fileparser.ParseFilename(filename)
			mediaID := uuid.New().String()

			mimeType := "video/mp4"
			ext := strings.ToLower(filepath.Ext(path))
			switch ext {
			case ".mkv":
				mimeType = "video/x-matroska"
			case ".avi":
				mimeType = "video/x-msvideo"
			case ".webm":
				mimeType = "video/webm"
			case ".mov":
				mimeType = "video/quicktime"
			}

			language := meta.Language
			if language == "" {
				language = "en"
			}

			info, statErr := os.Stat(path)
			fileSize := int64(0)
			if statErr == nil {
				fileSize = info.Size()
			}

			m = &model.Media{
				ID:            mediaID,
				Title:         meta.Title,
				OriginalName:  filename,
				Year:          meta.Year,
				Quality:       meta.Quality,
				FilePath:      path,
				FileSize:      fileSize,
				Duration:      0,
				MimeType:      mimeType,
				ThumbnailPath: "",
				Status:        model.StatusProcessing,
				Source:        model.SourceScan,
				Language:      language,
			}

			if err := m.Validate(); err != nil {
				slog.Error("Media validation failed", "file", filename, "err", err)
				continue
			}

			if err := mediaRepo.Create(m); err != nil {
				slog.Error("Failed to save media record to DB", "file", filename, "err", err)
				continue
			}

			service.ProcessMediaBackground(cfg, mediaRepo, mediaID, path)
			tasksCreated++
		} else {
			// Check if thumbnails actually exist on disk
			thumbnailsMissing := false
			if m.ThumbnailPath == "" {
				thumbnailsMissing = true
			} else {
				if _, statErr := os.Stat(m.ThumbnailPath); os.IsNotExist(statErr) {
					thumbnailsMissing = true
				}
			}

			// Check if scrubber thumbnails exist (look for scrub_<mediaID>_1.jpg)
			if !thumbnailsMissing {
				scrubPath := filepath.Join(filepath.Dir(m.FilePath), ".thumbnails", fmt.Sprintf("scrub_%s_1.jpg", m.ID))
				if _, statErr := os.Stat(scrubPath); os.IsNotExist(statErr) {
					thumbnailsMissing = true
				}
			}

			if thumbnailsMissing {
				slog.Info("Thumbnails missing on disk. Creating processing task...", "title", m.Title)
				m.Status = model.StatusProcessing
				_ = mediaRepo.Update(m)

				service.ProcessMediaBackground(cfg, mediaRepo, m.ID, m.FilePath)
				tasksCreated++
			}
		}
	}

	slog.Info(fmt.Sprintf("Scanning complete. Created %d thumbnail/metadata generation tasks.", tasksCreated))

	if tasksCreated > 0 {
		slog.Info("Waiting for background processing tasks to complete...")
		for {
			time.Sleep(2 * time.Second)
			processing, err := mediaRepo.FindByStatus(model.StatusProcessing)
			if err != nil {
				slog.Error("Database query failed during task status wait", "err", err)
				break
			}
			if len(processing) == 0 {
				break
			}
			slog.Info("Background queue processing...", "remaining_tasks", len(processing))
		}
		slog.Info("All background tasks completed successfully!")
	} else {
		slog.Info("Library is fully up to date. No tasks were needed.")
	}

	slog.Info("Rescan utility finished successfully.")
}
