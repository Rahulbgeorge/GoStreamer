package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"streamingplayer/internal/config"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
	"streamingplayer/pkg/thumbnail"
)

func getUniqueFilePath(dir, filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	dest := filepath.Join(dir, base)
	counter := 1
	for {
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			break
		}
		dest = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, counter, ext))
		counter++
	}
	return dest
}

func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	if err != nil {
		return err
	}

	input.Close()
	return os.Remove(src)
}

func isVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".ts":
		return true
	}
	return false
}

// ProcessMediaBackground is a background processor for metadata and thumbnail extraction.
func ProcessMediaBackground(cfg *config.Config, repo repository.MediaRepository, mediaID string, filePath string) {
	go func() {
		ctx := context.Background()
		slog.Info("Starting background processing for media", "mediaID", mediaID, "path", filePath)

		// 1. Probe duration + MIME type using ffprobe
		duration, mimeType, err := thumbnail.ProbeDuration(ctx, filePath)
		if err != nil {
			slog.Warn("Failed to probe duration in background", "mediaID", mediaID, "err", err)
		}

		// Update database with probed info
		m, err := repo.FindByID(mediaID)
		if err != nil {
			slog.Error("Failed to look up media for background processing", "mediaID", mediaID, "err", err)
			return
		}
		if m == nil {
			slog.Error("Media record not found for background processing", "mediaID", mediaID)
			return
		}

		m.Duration = duration
		m.MimeType = mimeType

		// Also probe dimensions!
		if w, h, dimErr := thumbnail.ProbeDimensions(ctx, filePath); dimErr == nil && w > 0 && h > 0 {
			resolution := fmt.Sprintf("%dx%d", w, h)
			if h >= 2160 {
				resolution = "4K (" + resolution + ")"
			} else if h >= 1080 {
				resolution = "1080p (" + resolution + ")"
			} else if h >= 720 {
				resolution = "720p (" + resolution + ")"
			}
			m.Quality = resolution
		}

		_ = repo.Update(m)

		// 2. Generate main thumbnail (first priority!)
		thumbPath, err := thumbnail.Generate(ctx, filePath, cfg.ThumbnailDir, mediaID)
		if err != nil {
			slog.Warn("Failed to generate main thumbnail in background", "mediaID", mediaID, "err", err)
		} else {
			m.ThumbnailPath = thumbPath
			_ = repo.Update(m)
		}

		// 3. Generate scrubber thumbnails (second priority!)
		_, err = thumbnail.GenerateScrubberThumbnails(ctx, filePath, cfg.ThumbnailDir, mediaID, 10)
		if err != nil {
			slog.Warn("Failed to generate scrubber thumbnails in background", "mediaID", mediaID, "err", err)
		}

		// 4. Mark status as ready
		m.Status = model.StatusReady
		if err := repo.Update(m); err != nil {
			slog.Error("Failed to update media status to ready in background", "mediaID", mediaID, "err", err)
		} else {
			slog.Info("Background media processing completed successfully", "mediaID", mediaID)
		}
	}()
}

