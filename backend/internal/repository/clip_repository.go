package repository

import "streamingplayer/internal/model"

// ClipRepository specifies database interactions for Clip entities.
type ClipRepository interface {
	FindAll() ([]model.Clip, error)
	FindByID(id string) (*model.Clip, error)
	FindByMediaID(mediaID string) ([]model.Clip, error)
	FindByCategoryID(categoryID string) ([]model.Clip, error)
	Create(clip *model.Clip) error
	Update(clip *model.Clip) error
	Delete(id string) error
}
