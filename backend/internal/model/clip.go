package model

import (
	"errors"
	"strings"
	"time"
)

// Clip represents a timestamp segment (start_time to end_time) of a media file.
type Clip struct {
	ID            string     `json:"id"`
	MediaID       string     `json:"media_id"`
	Title         string     `json:"title"`
	StartTime     float64    `json:"start_time"` // start position in seconds
	EndTime       float64    `json:"end_time"`   // end position in seconds
	ThumbnailPath string     `json:"thumbnail_path"`
	CategoryIDs   []string   `json:"category_ids"`
	Categories    []Category `json:"categories,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Validate verifies domain integrity constraints for Clip.
func (c *Clip) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return errors.New("clip ID is required")
	}
	if strings.TrimSpace(c.MediaID) == "" {
		return errors.New("clip media_id is required")
	}
	if strings.TrimSpace(c.Title) == "" {
		return errors.New("clip title is required")
	}
	if c.EndTime <= c.StartTime {
		return errors.New("clip end_time must be greater than start_time")
	}
	return nil
}
