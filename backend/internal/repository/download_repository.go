package repository

import "streamingplayer/internal/model"

type DownloadRepository interface {
	FindByID(id string) (*model.Download, error)
	FindAll() ([]model.Download, error)
	Create(d *model.Download) error
	Update(d *model.Download) error
	Delete(id string) error
}
