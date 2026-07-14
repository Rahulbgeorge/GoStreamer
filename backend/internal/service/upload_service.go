package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

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
	InitUpload(filename string, totalSize int64) (string, error)
	StoreChunk(uploadID string, chunkIdx int, src io.Reader) error
	CompleteUpload(ctx context.Context, uploadID string) (*model.Media, error)
}

type uploadService struct {
	config   *config.Config
	repo     repository.MediaRepository
	sessions map[string]*UploadSession
	mu       sync.RWMutex
}

func NewUploadService(cfg *config.Config, repo repository.MediaRepository) UploadService {
	return &uploadService{
		config:   cfg,
		repo:     repo,
		sessions: make(map[string]*UploadSession),
	}
}

func (s *uploadService) InitUpload(filename string, totalSize int64) (string, error) {
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

	return uploadID, nil
}

func (s *uploadService) StoreChunk(uploadID string, chunkIdx int, src io.Reader) error {
	s.mu.RLock()
	session, exists := s.sessions[uploadID]
	s.mu.RUnlock()

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

	return nil
}

func (s *uploadService) CompleteUpload(ctx context.Context, uploadID string) (*model.Media, error) {
	s.mu.Lock()
	session, exists := s.sessions[uploadID]
	delete(s.sessions, uploadID)
	s.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("upload session not found")
	}

	defer os.RemoveAll(session.ChunkDir)

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
	destPath := getUniqueFilePath(s.config.MediaDir, session.Filename)
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
