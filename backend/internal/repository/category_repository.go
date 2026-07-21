package repository

import "streamingplayer/internal/model"

// CategoryRepository specifies database interactions for Category entities.
type CategoryRepository interface {
	FindAll() ([]model.Category, error)
	FindByID(id string) (*model.Category, error)
	Create(cat *model.Category) error
	Delete(id string) error
}
