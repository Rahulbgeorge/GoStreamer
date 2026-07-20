package model

import "time"

type DownloadStatus string

const (
	DownloadStatusPending     DownloadStatus = "pending"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusCompleted   DownloadStatus = "completed"
	DownloadStatusFailed      DownloadStatus = "failed"
	DownloadStatusCancelled   DownloadStatus = "cancelled"
)

type DownloadType string

const (
	DownloadTypeYoutube DownloadType = "youtube"
	DownloadTypeTorrent DownloadType = "torrent"
)

type Download struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Status        DownloadStatus `json:"status"`
	Type          DownloadType   `json:"type"`
	Progress      float64        `json:"progress"`
	TotalSize     int64          `json:"total_size"`
	CompletedSize int64          `json:"completed_size"`
	DownloadSpeed float64        `json:"download_speed_bps"`
	ETA           string         `json:"eta"`
	DestPath      string         `json:"dest_path"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
