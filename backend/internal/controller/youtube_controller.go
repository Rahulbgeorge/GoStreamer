package controller

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"streamingplayer/internal/config"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
	"streamingplayer/internal/service"
)

type YoutubeController struct {
	cfg        *config.Config
	downloader service.YoutubeDownloader
	repo       repository.MediaRepository
	prefRepo   repository.PreferenceRepository
}

func NewYoutubeController(cfg *config.Config, downloader service.YoutubeDownloader, repo repository.MediaRepository, prefRepo repository.PreferenceRepository) *YoutubeController {
	return &YoutubeController{
		cfg:        cfg,
		downloader: downloader,
		repo:       repo,
		prefRepo:   prefRepo,
	}
}

func (ctrl *YoutubeController) getMediaDir() string {
	pref, err := ctrl.prefRepo.Get("homedir")
	if err == nil && pref != nil && pref.Value != "" {
		return pref.Value
	}
	return ctrl.cfg.MediaDir
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

	// Create a unique path in MediaDir
	destPath := filepath.Join(ctrl.getMediaDir(), filename)
	base := filepath.Base(destPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	counter := 1
	for {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		destPath = filepath.Join(ctrl.getMediaDir(), fmt.Sprintf("%s_%d%s", name, counter, ext))
		counter++
	}
	filename = filepath.Base(destPath)

	mediaID := uuid.New().String()

	// 2. Insert record as Downloading in the DB
	m := &model.Media{
		ID:            mediaID,
		Title:         title,
		OriginalName:  filename,
		Year:          time.Now().Year(),
		Quality:       "Pending",
		FilePath:      destPath,
		FileSize:      0,
		Duration:      0,
		MimeType:      "video/mp4",
		ThumbnailPath: "",
		Status:        model.StatusDownloading,
		Source:        model.SourceYoutube,
		Language:      "en",
	}

	if err := m.Validate(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate media model: " + err.Error()})
		return
	}

	if err := ctrl.repo.Create(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create media record: " + err.Error()})
		return
	}

	// 3. Start background download
	go func() {
		slog.Info("Starting youtube adaptive download", "media_id", mediaID, "url", url, "videoItag", videoItag, "audioItag", audioItag)
		err := ctrl.downloader.DownloadAdaptive(url, videoItag, audioItag, destPath)
		if err != nil {
			slog.Error("youtube download failed", "media_id", mediaID, "err", err)
			m.Status = model.StatusError
			_ = ctrl.repo.Update(m)
			return
		}

		// Update database with size
		info, statErr := os.Stat(destPath)
		if statErr == nil {
			m.FileSize = info.Size()
		}

		m.Status = model.StatusProcessing
		_ = ctrl.repo.Update(m)

		// Start probe and thumbnail extraction
		service.ProcessMediaBackground(ctrl.cfg, ctrl.repo, mediaID, destPath)
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Download started in background",
		"data":    m,
	})
}
