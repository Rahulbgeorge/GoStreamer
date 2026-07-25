package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"streamingplayer/internal/config"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
	"streamingplayer/pkg/fileparser"
)

type UploadSession struct {
	ID                  string
	Filename            string
	TotalSize           int64
	ChunkDir            string
	BytesSinceLastCheck int64
	LastSpeedCheck      time.Time
}

type UploadService interface {
	CheckUpload(fingerprint string) (string, []int, bool, error)
	InitUpload(filename string, totalSize int64, fingerprint string, device string) (string, error)
	StoreChunk(uploadID string, chunkIdx int, src io.Reader) error
	CompleteUpload(ctx context.Context, uploadID string) (*model.Media, error)
}

type uploadService struct {
	config       *config.Config
	repo         repository.MediaRepository
	prefRepo     repository.PreferenceRepository
	downloadRepo repository.DownloadRepository
	sessions     map[string]*UploadSession
	storage      ChunkStorageAdapter
	mu           sync.RWMutex
}

func NewUploadService(
	cfg *config.Config,
	repo repository.MediaRepository,
	prefRepo repository.PreferenceRepository,
	downloadRepo repository.DownloadRepository,
) UploadService {
	s := &uploadService{
		config:       cfg,
		repo:         repo,
		prefRepo:     prefRepo,
		downloadRepo: downloadRepo,
		sessions:     make(map[string]*UploadSession),
	}

	// Use TTL-based In-Memory Cache Chunk Storage Adapter (TTL: 30 mins)
	// Can be swapped with NewFileChunkStorageAdapter(cfg.UploadDir, s.sessions, &s.mu) for disk-based chunking
	s.storage = NewInMemoryChunkStorageAdapter(30*time.Minute, s.sessions, &s.mu)

	return s
}

func (s *uploadService) getMediaDir() string {
	pref, err := s.prefRepo.Get("homedir")
	if err == nil && pref != nil && pref.Value != "" {
		return pref.Value
	}
	return s.config.MediaDir
}

func (s *uploadService) CheckUpload(fingerprint string) (string, []int, bool, error) {
	if fingerprint == "" {
		return "", nil, false, nil
	}

	downloads, err := s.downloadRepo.FindAll()
	if err != nil {
		return "", nil, false, err
	}

	for _, dl := range downloads {
		if dl.Type == "upload" && dl.Status == "uploading" {
			parts := strings.Split(dl.DestPath, "|")
			var itemFingerprint string
			for _, part := range parts {
				if strings.HasPrefix(part, "fingerprint:") {
					itemFingerprint = strings.TrimPrefix(part, "fingerprint:")
				}
			}
			if itemFingerprint == fingerprint {
				// Recreate memory session if missing
				s.mu.Lock()
				if _, exists := s.sessions[dl.ID]; !exists {
					s.sessions[dl.ID] = &UploadSession{
						ID:        dl.ID,
						Filename:  dl.Title,
						TotalSize: dl.TotalSize,
						ChunkDir:  filepath.Join(s.config.UploadDir, dl.ID),
					}
				}
				s.mu.Unlock()

				chunks, err := s.getUploadedChunks(dl.ID)
				if err != nil {
					return "", nil, false, err
				}
				return dl.ID, chunks, true, nil
			}
		}
	}

	return "", nil, false, nil
}

func (s *uploadService) getUploadedChunks(uploadID string) ([]int, error) {
	return s.storage.GetUploadedChunks(uploadID)
}

func (s *uploadService) InitUpload(filename string, totalSize int64, fingerprint string, device string) (string, error) {
	// First check if there is an existing session
	existingID, _, exists, err := s.CheckUpload(fingerprint)
	if err == nil && exists {
		return existingID, nil
	}

	uploadID := uuid.New().String()
	chunkDir := filepath.Join(s.config.UploadDir, uploadID)

	session := &UploadSession{
		ID:                  uploadID,
		Filename:            filename,
		TotalSize:           totalSize,
		ChunkDir:            chunkDir,
		BytesSinceLastCheck: 0,
		LastSpeedCheck:      time.Now(),
	}

	s.mu.Lock()
	s.sessions[uploadID] = session
	s.mu.Unlock()

	// Save to downloads table to track task and resume it later
	deviceInfo := device
	if deviceInfo == "" {
		deviceInfo = "Unknown Device"
	}
	destPath := fmt.Sprintf("device:%s|fingerprint:%s", deviceInfo, fingerprint)

	now := time.Now()
	dl := &model.Download{
		ID:            uploadID,
		Title:         filename,
		Status:        "uploading",
		Type:          "upload",
		Progress:      0.0,
		TotalSize:     totalSize,
		CompletedSize: 0,
		DownloadSpeed: 0.0,
		ETA:           "Calculating...",
		DestPath:      destPath,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.downloadRepo.Create(dl); err != nil {
		slog.Error("Failed to save upload task to database", "err", err)
	}

	return uploadID, nil
}

func (s *uploadService) StoreChunk(uploadID string, chunkIdx int, src io.Reader) error {
	s.mu.Lock()
	session, exists := s.sessions[uploadID]
	if !exists {
		// If session was lost from memory but exists in DB, recreate it
		dl, err := s.downloadRepo.FindByID(uploadID)
		if err == nil && dl != nil && dl.Type == "upload" {
			session = &UploadSession{
				ID:                  uploadID,
				Filename:            dl.Title,
				TotalSize:           dl.TotalSize,
				ChunkDir:            filepath.Join(s.config.UploadDir, uploadID),
				BytesSinceLastCheck: 0,
				LastSpeedCheck:      time.Now(),
			}
			s.sessions[uploadID] = session
			exists = true
		}
	}
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("upload session not found")
	}

	if err := s.storage.StoreChunk(uploadID, chunkIdx, src); err != nil {
		return err
	}

	// Update progress and speed calculations
	s.mu.Lock()
	var newSpeed float64 = 0.0
	var updateSpeed = false
	if time.Since(session.LastSpeedCheck) >= 10*time.Second {
		newSpeed = float64(session.BytesSinceLastCheck) / 10.0 // bytes per second over 10s window
		session.BytesSinceLastCheck = 0
		session.LastSpeedCheck = time.Now()
		updateSpeed = true
	}
	s.mu.Unlock()

	// Update download/upload task progress in database
	chunks, err := s.getUploadedChunks(uploadID)
	if err == nil {
		dl, err := s.downloadRepo.FindByID(uploadID)
		if err == nil && dl != nil {
			var completedSize int64 = int64(len(chunks)) * 5242880 // 5MB per chunk approx
			if completedSize > dl.TotalSize && dl.TotalSize > 0 {
				completedSize = dl.TotalSize
			}

			dl.CompletedSize = completedSize
			if dl.TotalSize > 0 {
				dl.Progress = (float64(completedSize) / float64(dl.TotalSize)) * 100.0
				if dl.Progress > 100 {
					dl.Progress = 100
				}
			}
			if updateSpeed {
				dl.DownloadSpeed = newSpeed
			}
			dl.UpdatedAt = time.Now()
			_ = s.downloadRepo.Update(dl)
		}
	}

	return nil
}

func (s *uploadService) CompleteUpload(ctx context.Context, uploadID string) (*model.Media, error) {
	s.mu.Lock()
	session, exists := s.sessions[uploadID]
	if !exists {
		// Recreate session if needed
		dl, err := s.downloadRepo.FindByID(uploadID)
		if err == nil && dl != nil && dl.Type == "upload" {
			session = &UploadSession{
				ID:        uploadID,
				Filename:  dl.Title,
				TotalSize: dl.TotalSize,
				ChunkDir:  filepath.Join(s.config.UploadDir, uploadID),
			}
			exists = true
		}
	}
	delete(s.sessions, uploadID)
	s.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found")
	}

	defer s.storage.Cleanup(uploadID)

	// Remove from downloads repository database so task disappears from queue
	_ = s.downloadRepo.Delete(uploadID)

	// Create a unique destination file path in MediaDir
	destPath := getUniqueFilePath(s.getMediaDir(), session.Filename)

	// Assemble and write all chunks using storage adapter
	fileSize, err := s.storage.AssembleAndSave(ctx, uploadID, destPath)
	if err != nil {
		return nil, fmt.Errorf("assemble chunks: %w", err)
	}

	filename := filepath.Base(destPath)
	meta := fileparser.ParseFilename(filename)
	mediaID := uuid.New().String()

	// Guess MIME type synchronously
	mimeType := "video/mp4"
	ext := strings.ToLower(filepath.Ext(destPath))
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

	m := &model.Media{
		ID:            mediaID,
		Title:         meta.Title,
		OriginalName:  filename,
		Year:          meta.Year,
		Quality:       meta.Quality,
		FilePath:      destPath,
		FileSize:      fileSize,
		Duration:      0,
		MimeType:      mimeType,
		ThumbnailPath: "",
		Status:        model.StatusProcessing,
		Source:        model.SourceUpload,
		Language:      "en",
	}

	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("validate media model: %w", err)
	}

	if err := s.repo.Create(m); err != nil {
		return nil, fmt.Errorf("insert media into repo: %w", err)
	}

	// Trigger background processing (ffprobe, main thumbnail, scrubber thumbnails)
	ProcessMediaBackground(s.config, s.repo, mediaID, destPath)

	return m, nil
}
