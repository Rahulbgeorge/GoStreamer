package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
)

type StreamService interface {
	GetVideoStream(ctx context.Context, mediaID string) (*os.File, *model.Media, error)
	GetThumbnailStream(ctx context.Context, mediaID string) (*os.File, error)
	GetScrubberStatus(ctx context.Context, mediaID string) (int, error)
	GetScrubberImage(ctx context.Context, mediaID string, frame int) (*os.File, error)
	UpdateProgress(ctx context.Context, mediaID string, position int) error
}

type streamService struct {
	repo repository.MediaRepository
}

func NewStreamService(repo repository.MediaRepository) StreamService {
	return &streamService{repo: repo}
}

func (s *streamService) UpdateProgress(ctx context.Context, mediaID string, position int) error {
	return s.repo.UpdateProgress(mediaID, position)
}

// GetVideoStream resolves the media file path and returns a read-seekable handle to the file.
func (s *streamService) GetVideoStream(ctx context.Context, mediaID string) (*os.File, *model.Media, error) {
	media, err := s.repo.FindByID(mediaID)
	if err != nil {
		return nil, nil, fmt.Errorf("lookup media: %w", err)
	}
	if media == nil {
		return nil, nil, fmt.Errorf("media file not found: %s", mediaID)
	}

	file, err := os.Open(media.FilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open media file %s: %w", media.FilePath, err)
	}

	return file, media, nil
}

// GetThumbnailStream returns a file handle for the movie's thumbnail image.
func (s *streamService) GetThumbnailStream(ctx context.Context, mediaID string) (*os.File, error) {
	media, err := s.repo.FindByID(mediaID)
	if err != nil {
		return nil, fmt.Errorf("lookup media: %w", err)
	}
	if media == nil || media.ThumbnailPath == "" {
		return nil, fmt.Errorf("media thumbnail not found: %s", mediaID)
	}

	file, err := os.Open(media.ThumbnailPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open thumbnail file: %w", err)
	}

	return file, nil
}

// GetScrubberStatus counts how many scrubber thumbnail frames exist for the media.
func (s *streamService) GetScrubberStatus(ctx context.Context, mediaID string) (int, error) {
	media, err := s.repo.FindByID(mediaID)
	if err != nil {
		return 0, fmt.Errorf("lookup media for scrubber status: %w", err)
	}
	if media == nil || media.FilePath == "" {
		return 0, fmt.Errorf("media file not found: %s", mediaID)
	}

	dir := filepath.Join(filepath.Dir(media.FilePath), ".thumbnails")
	count := 0
	for {
		filePath := filepath.Join(dir, fmt.Sprintf("scrub_%s_%d.jpg", mediaID, count+1))
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		count++
	}

	return count, nil
}

// GetScrubberImage opens and returns the requested scrubber frame file handle.
func (s *streamService) GetScrubberImage(ctx context.Context, mediaID string, frame int) (*os.File, error) {
	media, err := s.repo.FindByID(mediaID)
	if err != nil {
		return nil, fmt.Errorf("lookup media for scrubber image: %w", err)
	}
	if media == nil || media.FilePath == "" {
		return nil, fmt.Errorf("media file not found: %s", mediaID)
	}

	filePath := filepath.Join(filepath.Dir(media.FilePath), ".thumbnails", fmt.Sprintf("scrub_%s_%d.jpg", mediaID, frame))
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open scrubber file: %w", err)
	}

	return file, nil
}
