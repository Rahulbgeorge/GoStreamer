package repository

import "streamingplayer/internal/model"

// PreferenceRepository defines operations for application configuration preferences.
type PreferenceRepository interface {
	Get(key string) (*model.Preference, error)
	Set(key string, value string) error
	GetAll() ([]model.Preference, error)
	Delete(key string) error
}
