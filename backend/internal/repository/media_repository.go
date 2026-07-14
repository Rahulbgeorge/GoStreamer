package repository

import "streamingplayer/internal/model"

// MediaRepository specifies database interactions for Media entities.
type MediaRepository interface {
	FindByID(id string) (*model.Media, error)
	FindAll(limit, offset int) ([]model.Media, error)
	FindByStatus(status model.MediaStatus) ([]model.Media, error)
	FindByFilePath(path string) (*model.Media, error)
	Search(query string, limit, offset int) ([]model.Media, error)
	Create(media *model.Media) error
	Update(media *model.Media) error
	Delete(id string) error
	Count() (int, error)
	TotalSize() (int64, error)
}
