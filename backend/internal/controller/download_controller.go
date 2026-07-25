package controller

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"streamingplayer/internal/config"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
	"streamingplayer/internal/service"
)

type DownloadController struct {
	downloadRepo   repository.DownloadRepository
	torrentService service.TorrentService
	config         *config.Config
}

func NewDownloadController(
	downloadRepo repository.DownloadRepository,
	torrentService service.TorrentService,
	cfg *config.Config,
) *DownloadController {
	return &DownloadController{
		downloadRepo:   downloadRepo,
		torrentService: torrentService,
		config:         cfg,
	}
}

func (ctrl *DownloadController) ListDownloads(c *gin.Context) {
	list, err := ctrl.downloadRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve downloads: " + err.Error()})
		return
	}
	if list == nil {
		list = []model.Download{}
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func (ctrl *DownloadController) GetDownload(c *gin.Context) {
	id := c.Param("id")
	dl, err := ctrl.downloadRepo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if dl == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Download not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dl})
}

func (ctrl *DownloadController) DeleteDownload(c *gin.Context) {
	id := c.Param("id")
	dl, err := ctrl.downloadRepo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if dl == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Download not found"})
		return
	}

	if dl.Type == model.DownloadTypeTorrent {
		_ = ctrl.torrentService.CancelTorrent(id)
	}

	if dl.Type == "upload" {
		chunkDir := filepath.Join(ctrl.config.UploadDir, id)
		_ = os.RemoveAll(chunkDir)
	}

	if err := ctrl.downloadRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": true})
}
