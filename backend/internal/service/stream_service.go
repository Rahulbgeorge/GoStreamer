package service

import (
	"context"
	"fmt"
	"os"

	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
)

type StreamService interface {
	GetVideoStream(ctx context.Context, mediaID string) (*os.File, *model.Media, error)
	GetThumbnailStream(ctx context.Context, mediaID string) (*os.File, error)
}

type streamService struct {
	repo repository.MediaRepository
}

func NewStreamService(repo repository.MediaRepository) StreamService {
	return &streamService{repo: repo}
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
