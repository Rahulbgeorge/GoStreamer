package model

import (
	"errors"
	"strings"
	"time"
)

// Category represents a media clip genre/tag (e.g., "Songs", "Highlights", "Action").
type Category struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Validate verifies domain integrity constraints for Category.
func (c *Category) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return errors.New("category ID is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("category name is required")
	}
	return nil
}
