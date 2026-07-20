package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

type mediaTask struct {
	cfg      *config.Config
	repo     repository.MediaRepository
	mediaID  string
	filePath string
}

var (
	taskQueue chan mediaTask
	queueOnce sync.Once
)

// ProcessMediaBackground queues media metadata and thumbnail extraction to run sequentially.
func ProcessMediaBackground(cfg *config.Config, repo repository.MediaRepository, mediaID string, filePath string) {
	queueOnce.Do(func() {
		// Buffered channel size to handle typical folder scanning workload without blocking.
		taskQueue = make(chan mediaTask, 10000)
		go func() {
			ctx := context.Background()
			for task := range taskQueue {
				processMediaSingleTask(ctx, task)
			}
		}()
	})

	taskQueue <- mediaTask{
		cfg:      cfg,
		repo:     repo,
		mediaID:  mediaID,
		filePath: filePath,
	}
}

func processMediaSingleTask(ctx context.Context, task mediaTask) {
	slog.Info("Starting background processing for media", "mediaID", task.mediaID, "path", task.filePath)

	// 1. Probe duration + MIME type using ffprobe
	duration, mimeType, err := thumbnail.ProbeDuration(ctx, task.filePath)
	if err != nil {
		slog.Warn("Failed to probe duration in background", "mediaID", task.mediaID, "err", err)
	}

	// Update database with probed info
	m, err := task.repo.FindByID(task.mediaID)
	if err != nil {
		slog.Error("Failed to look up media for background processing", "mediaID", task.mediaID, "err", err)
		return
	}
	if m == nil {
		slog.Error("Media record not found for background processing", "mediaID", task.mediaID)
		return
	}

	m.Duration = duration
	m.MimeType = mimeType

	// Also probe dimensions!
	if w, h, dimErr := thumbnail.ProbeDimensions(ctx, task.filePath); dimErr == nil && w > 0 && h > 0 {
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

	_ = task.repo.Update(m)

	// Determine parent dir and construct .thumbnails directory path local to video file
	parentDir := filepath.Dir(task.filePath)
	thumbDir := filepath.Join(parentDir, ".thumbnails")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		slog.Error("Failed to create .thumbnails directory", "dir", thumbDir, "err", err)
	}

	// 2. Generate main thumbnail (first priority!)
	thumbPath, err := thumbnail.Generate(ctx, task.filePath, thumbDir, task.mediaID)
	if err != nil {
		slog.Warn("Failed to generate main thumbnail in background", "mediaID", task.mediaID, "err", err)
	} else {
		m.ThumbnailPath = thumbPath
		_ = task.repo.Update(m)
	}

	// 3. Generate scrubber thumbnails (second priority!)
	_, err = thumbnail.GenerateScrubberThumbnails(ctx, task.filePath, thumbDir, task.mediaID, 10)
	if err != nil {
		slog.Warn("Failed to generate scrubber thumbnails in background", "mediaID", task.mediaID, "err", err)
	}

	// 4. Mark status as ready
	m.Status = model.StatusReady
	if err := task.repo.Update(m); err != nil {
		slog.Error("Failed to update media status to ready in background", "mediaID", task.mediaID, "err", err)
	} else {
		slog.Info("Background media processing completed successfully", "mediaID", task.mediaID)
	}
}

// CleanUpThumbnails purges main and scrubber thumbnails under the file's parent folder.
func CleanUpThumbnails(mediaID string, filePath string) {
	if filePath == "" {
		return
	}
	parentDir := filepath.Dir(filePath)
	thumbDir := filepath.Join(parentDir, ".thumbnails")

	// 1. Delete main thumbnail
	mainThumb := filepath.Join(thumbDir, fmt.Sprintf("%s.jpg", mediaID))
	_ = os.Remove(mainThumb)

	// 2. Delete all scrubber thumbnails: scrub_<mediaID>_*.jpg
	entries, err := os.ReadDir(thumbDir)
	if err == nil {
		prefix := fmt.Sprintf("scrub_%s_", mediaID)
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
				_ = os.Remove(filepath.Join(thumbDir, entry.Name()))
			}
		}
	}

	// 3. Try to clean up the directory itself if it is empty
	_ = os.Remove(thumbDir)
}

