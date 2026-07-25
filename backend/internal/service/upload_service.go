package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	ID        string
	Filename  string
	TotalSize int64
	ChunkDir  string
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
	mu           sync.RWMutex
}

func NewUploadService(
	cfg *config.Config,
	repo repository.MediaRepository,
	prefRepo repository.PreferenceRepository,
	downloadRepo repository.DownloadRepository,
) UploadService {
	return &uploadService{
		config:       cfg,
		repo:         repo,
		prefRepo:     prefRepo,
		downloadRepo: downloadRepo,
		sessions:     make(map[string]*UploadSession),
	}
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
	chunkDir := filepath.Join(s.config.UploadDir, uploadID)
	entries, err := os.ReadDir(chunkDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []int{}, nil
		}
		return nil, err
	}

	var chunks []int
	for _, entry := range entries {
		if !entry.IsDir() {
			if idx, err := strconv.Atoi(entry.Name()); err == nil {
				chunks = append(chunks, idx)
			}
		}
	}
	sort.Ints(chunks)
	return chunks, nil
}

func (s *uploadService) InitUpload(filename string, totalSize int64, fingerprint string, device string) (string, error) {
	// First check if there is an existing session
	existingID, _, exists, err := s.CheckUpload(fingerprint)
	if err == nil && exists {
		return existingID, nil
	}

	uploadID := uuid.New().String()
	chunkDir := filepath.Join(s.config.UploadDir, uploadID)

	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return "", fmt.Errorf("create chunk directory: %w", err)
	}

	session := &UploadSession{
		ID:        uploadID,
		Filename:  filename,
		TotalSize: totalSize,
		ChunkDir:  chunkDir,
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
				ID:        uploadID,
				Filename:  dl.Title,
				TotalSize: dl.TotalSize,
				ChunkDir:  filepath.Join(s.config.UploadDir, uploadID),
			}
			s.sessions[uploadID] = session
			exists = true
		}
	}
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("upload session not found")
	}

	chunkPath := filepath.Join(session.ChunkDir, strconv.Itoa(chunkIdx))
	destFile, err := os.Create(chunkPath)
	if err != nil {
		return fmt.Errorf("create chunk file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, src); err != nil {
		return fmt.Errorf("write chunk file: %w", err)
	}

	// Update download/upload task progress in database
	chunks, err := s.getUploadedChunks(uploadID)
	if err == nil {
		dl, err := s.downloadRepo.FindByID(uploadID)
		if err == nil && dl != nil {
			var completedSize int64
			for _, cIdx := range chunks {
				cPath := filepath.Join(session.ChunkDir, strconv.Itoa(cIdx))
				if stat, statErr := os.Stat(cPath); statErr == nil {
					completedSize += stat.Size()
				}
			}

			dl.CompletedSize = completedSize
			if dl.TotalSize > 0 {
				dl.Progress = (float64(completedSize) / float64(dl.TotalSize)) * 100.0
				if dl.Progress > 100 {
					dl.Progress = 100
				}
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

	defer os.RemoveAll(session.ChunkDir)

	// Remove from downloads repository database so task disappears from queue
	_ = s.downloadRepo.Delete(uploadID)

	// List all files in the chunk directory
	entries, err := os.ReadDir(session.ChunkDir)
	if err != nil {
		return nil, fmt.Errorf("read chunk directory: %w", err)
	}

	// Filter and sort chunk files numerically
	var chunkFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			if _, err := strconv.Atoi(entry.Name()); err == nil {
				chunkFiles = append(chunkFiles, entry.Name())
			}
		}
	}

	sort.Slice(chunkFiles, func(i, j int) bool {
		valI, _ := strconv.Atoi(chunkFiles[i])
		valJ, _ := strconv.Atoi(chunkFiles[j])
		return valI < valJ
	})

	if len(chunkFiles) == 0 {
		return nil, fmt.Errorf("no chunk files found to assemble")
	}

	// Create a unique destination file path in MediaDir
	destPath := getUniqueFilePath(s.getMediaDir(), session.Filename)
	destFile, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("create destination file: %w", err)
	}
	defer destFile.Close()

	// Merge all chunk files
	for _, filename := range chunkFiles {
		chunkPath := filepath.Join(session.ChunkDir, filename)
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return nil, fmt.Errorf("open chunk file %s: %w", filename, err)
		}
		_, err = io.Copy(destFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			return nil, fmt.Errorf("append chunk %s: %w", filename, err)
		}
	}

	// Now ingest the file to database
	info, err := destFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat merged file: %w", err)
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
		FileSize:      info.Size(),
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
