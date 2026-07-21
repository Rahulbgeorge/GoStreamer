package model

import (
	"errors"
	"strings"
	"time"
)

// MediaStatus represents the state of a media file ingest pipeline.
type MediaStatus string

const (
	StatusPending     MediaStatus = "pending"
	StatusDownloading MediaStatus = "downloading"
	StatusProcessing  MediaStatus = "processing"
	StatusReady       MediaStatus = "ready"
	StatusError       MediaStatus = "error"
)

// MediaSource tells us how this movie was added.
type MediaSource string

const (
	SourceTorrent MediaSource = "torrent"
	SourceUpload  MediaSource = "upload"
	SourceScan    MediaSource = "scan"
	SourceYoutube MediaSource = "youtube"
)

// Media is the central domain struct representing a movie in our database.
type Media struct {
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	OriginalName  string      `json:"original_name"`
	Year          int         `json:"year"`
	Quality       string      `json:"quality"`
	Genre         string      `json:"genre"`
	FilePath      string      `json:"file_path"`
	FileSize      int64       `json:"file_size"`
	Duration      int         `json:"duration"` // in seconds
	MimeType      string      `json:"mime_type"`
	ThumbnailPath string      `json:"thumbnail_path"`
	Status        MediaStatus `json:"status"`
	Source        MediaSource `json:"source"`
	Language         string      `json:"language"`           // e.g. "en", "hi"
	LastPosition     int         `json:"last_position"`      // last watched position in seconds
	DefaultStartTime int         `json:"default_start_time"` // default auto-playing start position in seconds
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// Validate verifies domain integrity constraints before save/update operations.
func (m *Media) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return errors.New("media ID is required")
	}
	if strings.TrimSpace(m.Title) == "" {
		return errors.New("media title is required")
	}
	if strings.TrimSpace(m.FilePath) == "" {
		return errors.New("media file path is required")
	}
	if m.Status == "" {
		m.Status = StatusPending
	}
	if m.Source == "" {
		m.Source = SourceScan
	}
	if strings.TrimSpace(m.Language) == "" {
		m.Language = "en"
	}
	return nil
}
