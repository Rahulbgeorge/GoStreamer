package controller

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"streamingplayer/internal/config"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
	"streamingplayer/internal/service"
)

type YoutubeController struct {
	cfg            *config.Config
	downloader     service.YoutubeDownloader
	repo           repository.MediaRepository
	prefRepo       repository.PreferenceRepository
	downloadRepo   repository.DownloadRepository
	scannerService service.ScannerService
}

func NewYoutubeController(
	cfg *config.Config,
	downloader service.YoutubeDownloader,
	repo repository.MediaRepository,
	prefRepo repository.PreferenceRepository,
	downloadRepo repository.DownloadRepository,
	scannerService service.ScannerService,
) *YoutubeController {
	return &YoutubeController{
		cfg:            cfg,
		downloader:     downloader,
		repo:           repo,
		prefRepo:       prefRepo,
		downloadRepo:   downloadRepo,
		scannerService: scannerService,
	}
}

func (ctrl *YoutubeController) getYoutubeDir() string {
	pref, err := ctrl.prefRepo.Get("homedir")
	if err == nil && pref != nil && pref.Value != "" {
		return pref.Value
	}
	return ctrl.cfg.YoutubeDownloadDir
}

func (ctrl *YoutubeController) ListFormats(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	formats, title, err := ctrl.downloader.ListFormats(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Transform formats for a cleaner client representation
	type FormatItem struct {
		ItagNo       int    `json:"itag"`
		MimeType     string `json:"mime_type"`
		QualityLabel string `json:"quality_label"`
		Height       int    `json:"height"`
		AudioQuality string `json:"audio_quality"`
		Bitrate      int    `json:"bitrate"`
	}

	var list []FormatItem
	for _, f := range formats {
		list = append(list, FormatItem{
			ItagNo:       f.ItagNo,
			MimeType:     f.MimeType,
			QualityLabel: f.QualityLabel,
			Height:       f.Height,
			AudioQuality: f.AudioQuality,
			Bitrate:      f.Bitrate,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"title":   title,
		"formats": list,
	})
}

func (ctrl *YoutubeController) DownloadVideo(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	videoItagStr := c.Query("videoItag")
	audioItagStr := c.Query("audioItag")

	videoItag := 0
	if videoItagStr != "" {
		if val, err := strconv.Atoi(videoItagStr); err == nil {
			videoItag = val
		}
	}

	audioItag := 0
	if audioItagStr != "" {
		if val, err := strconv.Atoi(audioItagStr); err == nil {
			audioItag = val
		}
	}

	// 1. Fetch metadata to get a descriptive filename/title
	_, title, err := ctrl.downloader.ListFormats(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve video metadata: " + err.Error()})
		return
	}

	// clean the title for filename
	cleanTitle := title
	for _, char := range []string{"/", "\\", "?", "%", "*", ":", "|", "\"", "<", ">"} {
		cleanTitle = strings.ReplaceAll(cleanTitle, char, "_")
	}
	filename := cleanTitle + ".mp4"

	// Create a unique path in getYoutubeDir
	destDir := ctrl.getYoutubeDir()
	if err := os.MkdirAll(destDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create destination folder: " + err.Error()})
		return
	}

	destPath := filepath.Join(destDir, filename)
	base := filepath.Base(destPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	counter := 1
	for {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		destPath = filepath.Join(destDir, fmt.Sprintf("%s_%d%s", name, counter, ext))
		counter++
	}
	filename = filepath.Base(destPath)

	downloadID := uuid.New().String()

	// 2. Insert record as Downloading in the unified downloads DB
	dl := &model.Download{
		ID:            downloadID,
		Title:         title,
		Status:        model.DownloadStatusDownloading,
		Type:          model.DownloadTypeYoutube,
		Progress:      0,
		TotalSize:     0,
		CompletedSize: 0,
		DestPath:      destPath,
	}

	if err := ctrl.downloadRepo.Create(dl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create download record: " + err.Error()})
		return
	}

	// 3. Start background download
	go func() {
		slog.Info("Starting youtube adaptive download", "download_id", downloadID, "url", url, "videoItag", videoItag, "audioItag", audioItag)
		err := ctrl.downloader.DownloadAdaptive(context.Background(), downloadID, url, videoItag, audioItag, destPath)
		if err != nil {
			slog.Error("youtube download failed", "download_id", downloadID, "err", err)
			dl.Status = model.DownloadStatusFailed
			_ = ctrl.downloadRepo.Update(dl)
			return
		}

		dl.Status = model.DownloadStatusCompleted
		dl.Progress = 100.0
		info, statErr := os.Stat(destPath)
		if statErr == nil {
			dl.TotalSize = info.Size()
			dl.CompletedSize = info.Size()
		}
		_ = ctrl.downloadRepo.Update(dl)

		slog.Info("YouTube download complete, triggering directory scan", "title", dl.Title)
		go ctrl.scannerService.ScanDirectory(context.Background())
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Download started in background",
		"data":    dl,
	})
}
