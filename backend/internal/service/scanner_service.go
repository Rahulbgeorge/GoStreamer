package service

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"streamingplayer/internal/config"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
	"streamingplayer/pkg/fileparser"
)

type ScannerService interface {
	Start(ctx context.Context) error
	Stop()
	ScanDirectory(ctx context.Context)
	IngestFile(ctx context.Context, path string)
}

type scannerService struct {
	config            *config.Config
	repo              repository.MediaRepository
	prefRepo          repository.PreferenceRepository
	watcher           *fsnotify.Watcher
	currentWatchedDir string
	mu                sync.Mutex
	running           bool
	stopChan          chan struct{}
}

func NewScannerService(cfg *config.Config, repo repository.MediaRepository, prefRepo repository.PreferenceRepository) ScannerService {
	return &scannerService{
		config:   cfg,
		repo:     repo,
		prefRepo: prefRepo,
		stopChan: make(chan struct{}),
	}
}

func (s *scannerService) getMediaDir() string {
	pref, err := s.prefRepo.Get("homedir")
	if err == nil && pref != nil && pref.Value != "" {
		return pref.Value
	}
	return s.config.MediaDir
}

// Start kicks off both fsnotify real-time watching and the periodic directory scanning.
func (s *scannerService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	mediaDir := s.getMediaDir()
	s.currentWatchedDir = mediaDir

	// Ensure media directory and thumbnail directory exist
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return fmt.Errorf("create media dir: %w", err)
	}
	if err := os.MkdirAll(s.config.ThumbnailDir, 0755); err != nil {
		return fmt.Errorf("create thumbnail dir: %w", err)
	}

	// Initialize file system watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("Failed to initialize fsnotify watcher", "err", err)
	} else {
		s.watcher = watcher
		// Watch media directory recursively
		s.watchDirRecursive(mediaDir)
		go s.watchLoop(ctx)
	}

	// Run initial scan immediately
	go func() {
		s.ScanDirectory(ctx)
	}()

	// Start periodic full-scan loop (every 5 minutes)
	go s.scanLoop(ctx)

	return nil
}

func (s *scannerService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopChan)
	if s.watcher != nil {
		s.watcher.Close()
	}
}

func (s *scannerService) watchDirRecursive(dir string) {
	if s.watcher == nil {
		return
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			slog.Debug("Adding folder to fsnotify watcher", "path", path)
			_ = s.watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		slog.Error("Recursive dir walking failed during watcher registration", "dir", dir, "err", err)
	}
}

func (s *scannerService) watchLoop(ctx context.Context) {
	for {
		select {
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			// Only process write, create, or rename events for video files
			if event.Op&fsnotify.Create == fsnotify.Create {
				info, err := os.Stat(event.Name)
				if err == nil && info.IsDir() {
					// Add newly created folders to watcher too
					s.watchDirRecursive(event.Name)
					continue
				}
				
				if isAllowedFile(event.Name, s.config.AllowedExtensions) {
					// Wait briefly for write completion (important for slow copies)
					go func(path string) {
						time.Sleep(5 * time.Second)
						s.IngestFile(ctx, path)
					}(event.Name)
				}
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("Fsnotify watcher received error event", "err", err)
		}
	}
}

func (s *scannerService) scanLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.ScanDirectory(ctx)
		}
	}
}

func (s *scannerService) ScanDirectory(ctx context.Context) {
	slog.Info("Starting folder scanning...")

	mediaDir := s.getMediaDir()

	// Ensure media directory exists
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		slog.Error("Failed to create/verify media directory", "dir", mediaDir, "err", err)
		return
	}

	// Re-register watcher if directory path changed
	if s.watcher != nil {
		s.mu.Lock()
		if s.currentWatchedDir != mediaDir {
			slog.Info("Media directory changed, updating fsnotify watcher", "old", s.currentWatchedDir, "new", mediaDir)
			s.watcher.Close()
			watcher, err := fsnotify.NewWatcher()
			if err == nil {
				s.watcher = watcher
				s.currentWatchedDir = mediaDir
				s.watchDirRecursive(mediaDir)
				go s.watchLoop(ctx)
			} else {
				slog.Error("Failed to recreate fsnotify watcher", "err", err)
			}
		}
		s.mu.Unlock()
	}

	// 1. Scan directory and ingest newly found files
	err := filepath.WalkDir(mediaDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !d.IsDir() && isAllowedFile(path, s.config.AllowedExtensions) {
			s.IngestFile(ctx, path)
		}
		return nil
	})
	if err != nil {
		slog.Error("WalkDir failed during file scanning", "dir", mediaDir, "err", err)
	}

	// 2. Clean up database records for files that no longer exist on disk
	allMedia, err := s.repo.FindAll(100000, 0)
	if err != nil {
		slog.Error("Failed to fetch all media from DB for cleanup check", "err", err)
		return
	}

	for _, m := range allMedia {
		if m.FilePath != "" {
			if _, statErr := os.Stat(m.FilePath); os.IsNotExist(statErr) {
				slog.Info("Removing record from DB as file no longer exists on disk", "id", m.ID, "title", m.Title, "path", m.FilePath)
				if deleteErr := s.repo.Delete(m.ID); deleteErr != nil {
					slog.Error("Failed to delete missing media record", "id", m.ID, "err", deleteErr)
				}
				// Clean up thumbnail
				if m.ThumbnailPath != "" {
					_ = os.Remove(m.ThumbnailPath)
				}
			}
		}
	}
}

// isAllowedFile checks if a file is not a half-downloaded .part file, and matches the AllowedExtensions list.
func isAllowedFile(path string, allowedExtensions []string) bool {
	// Ignore .part files
	if strings.HasSuffix(path, ".part") {
		return false
	}

	// Get file extension
	ext := strings.ToLower(filepath.Ext(path))

	// Check if the extension is in the allowed list
	if len(allowedExtensions) > 0 {
		for _, e := range allowedExtensions {
			if strings.EqualFold(ext, e) {
				return true
			}
		}
		return false
	}

	// Fallback to standard video check if no allowed list is specified
	return isVideoFile(path)
}

func (s *scannerService) IngestFile(ctx context.Context, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify if already registered
	existing, err := s.repo.FindByFilePath(path)
	if err != nil {
		slog.Error("Database lookup failed during scan ingestion", "path", path, "err", err)
		return
	}
	if existing != nil {
		return // Already imported
	}

	info, err := os.Stat(path)
	if err != nil {
		slog.Error("Could not read stats of file during ingestion", "path", path, "err", err)
		return
	}

	filename := filepath.Base(path)
	meta := fileparser.ParseFilename(filename)

	mediaID := uuid.New().String()

	// Guess MIME type synchronously for initial value
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

	m := &model.Media{
		ID:            mediaID,
		Title:         meta.Title,
		OriginalName:  filename,
		Year:          meta.Year,
		Quality:       meta.Quality,
		FilePath:      path,
		FileSize:      info.Size(),
		Duration:      0, // Will be updated by ffprobe in background
		MimeType:      mimeType,
		ThumbnailPath: "", // Will be updated by generator in background
		Status:        model.StatusProcessing,
		Source:        model.SourceScan,
		Language:      language,
	}

	if err := m.Validate(); err != nil {
		slog.Error("Parsed media model failed validation step", "path", path, "err", err)
		return
	}

	if err := s.repo.Create(m); err != nil {
		slog.Error("Failed to save cataloged movie file to database", "path", path, "err", err)
		return
	}

	slog.Info("Successfully cataloged newly discovered movie file (processing in background)", "title", m.Title, "quality", m.Quality, "year", m.Year)

	// Trigger background processing (ffprobe, main thumbnail, scrubber thumbnails)
	ProcessMediaBackground(s.config, s.repo, mediaID, path)
}
